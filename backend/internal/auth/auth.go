// Package auth mints and verifies JWT sessions with an is_admin claim.
//
// Phase 0 §8: the is_admin claim can ONLY be set by the trusted server
// process (MintAdmin, using the server-side secret). There is no
// client-reachable path — no endpoint, no request field — that grants it.
// Verification rejects any token not signed with the env-specific secret,
// so a dev credential can never authenticate against staging or prod.
package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// ClaimAdmin is the only privilege flag in the system.
	ClaimAdmin = "is_admin"
	// minSecretLen mirrors config.Load: short secrets are rejected at boot.
	minSecretLen = 32
)

// Claims is the full session payload. SessionID binds the access token to
// a row in sessions, so sign-out and account deletion revoke it immediately
// (Phase 1 §2.5). Empty for Phase-0-era callers — Verify stays compatible.
type Claims struct {
	UserID    string `json:"sub"`
	SessionID string `json:"sid,omitempty"`
	IsAdmin   bool   `json:"is_admin"`
	jwt.RegisteredClaims
}

// Issuer holds the env-specific HMAC secret. One instance per environment.
type Issuer struct {
	secret []byte
	env    string
}

func NewIssuer(secret, env string) (*Issuer, error) {
	if len(secret) < minSecretLen {
		return nil, fmt.Errorf("auth: secret for env %q must be at least %d chars", env, minSecretLen)
	}
	return &Issuer{secret: []byte(secret), env: env}, nil
}

// MintUser issues a normal session. IsAdmin is always false here by
// construction — callers cannot smuggle it in.
func (iss *Issuer) MintUser(userID string, ttl time.Duration) (string, error) {
	return iss.mint(userID, "", false, ttl)
}

// MintUserSession issues a normal session bound to a sessions row.
// Sign-out / deletion revoke the row, which invalidates this token.
func (iss *Issuer) MintUserSession(userID, sessionID string, ttl time.Duration) (string, error) {
	return iss.mint(userID, sessionID, false, ttl)
}

// MintAdmin issues a privileged session. Only server-side admin-grant flows
// (Phase 11) may call this; it is never wired to a client request field.
func (iss *Issuer) MintAdmin(userID string, ttl time.Duration) (string, error) {
	return iss.mint(userID, "", true, ttl)
}

func (iss *Issuer) mint(userID, sessionID string, admin bool, ttl time.Duration) (string, error) {
	now := time.Now()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID:    userID,
		SessionID: sessionID,
		IsAdmin:   admin,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Issuer:    "sidekick-" + iss.env,
		},
	})
	signed, err := tok.SignedString(iss.secret)
	if err != nil {
		return "", fmt.Errorf("auth: sign: %w", err)
	}
	return signed, nil
}

// Verify parses and authenticates a token. Tokens signed with a different
// env secret (or tampered claims, e.g. flipping is_admin) are rejected.
func (iss *Issuer) Verify(tokenStr string) (*Claims, error) {
	tok, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("auth: unexpected alg %q", t.Method.Alg())
		}
		return iss.secret, nil
	}, jwt.WithIssuer("sidekick-"+iss.env), jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}
	claims, ok := tok.Claims.(*Claims)
	if !ok || !tok.Valid {
		return nil, fmt.Errorf("auth: invalid token")
	}
	return claims, nil
}

// RequireAdmin verifies the token AND the admin claim for gated endpoints.
func (iss *Issuer) RequireAdmin(tokenStr string) (*Claims, error) {
	c, err := iss.Verify(tokenStr)
	if err != nil {
		return nil, err
	}
	if !c.IsAdmin {
		return nil, fmt.Errorf("auth: admin claim required")
	}
	return c, nil
}
