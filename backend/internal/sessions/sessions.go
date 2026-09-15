// Package sessions manages opaque refresh-token sessions.
//
// Access JWTs carry the session id (sid). Every authenticated request
// checks the row is still live — so sign-out (§2.5) and account deletion
// invalidate tokens immediately instead of waiting for JWT expiry.
// Suspended / soft-deleted users fail validation even with a live row.
package sessions

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const RefreshTTL = 30 * 24 * time.Hour

type Session struct {
	ID        string
	UserID    string
	ExpiresAt time.Time
}

// Create mints an opaque refresh token and stores only its hash.
// Returns the session id and the RAW refresh token (shown once).
func Create(ctx context.Context, pool *pgxpool.Pool, userID string) (Session, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return Session{}, "", fmt.Errorf("sessions: rand: %w", err)
	}
	refresh := hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(refresh))
	var s Session
	err := pool.QueryRow(ctx,
		`INSERT INTO sessions (user_id, refresh_token_hash, expires_at)
		 VALUES ($1::uuid, $2, now() + $3::interval)
		 RETURNING id::text, user_id::text, expires_at`,
		userID, hex.EncodeToString(sum[:]), RefreshTTL.String()).Scan(&s.ID, &s.UserID, &s.ExpiresAt)
	if err != nil {
		return Session{}, "", fmt.Errorf("sessions: create: %w", err)
	}
	return s, refresh, nil
}

// ValidateRefresh checks a raw refresh token and returns its session.
func ValidateRefresh(ctx context.Context, pool *pgxpool.Pool, rawRefresh string) (Session, error) {
	sum := sha256.Sum256([]byte(rawRefresh))
	var s Session
	err := pool.QueryRow(ctx,
		`SELECT s.id::text, s.user_id::text, s.expires_at
		   FROM sessions s JOIN users u ON u.id = s.user_id
		  WHERE s.refresh_token_hash = $1 AND s.revoked_at IS NULL
		    AND s.expires_at > now() AND u.deleted_at IS NULL AND u.suspended_at IS NULL`,
		hex.EncodeToString(sum[:])).Scan(&s.ID, &s.UserID, &s.ExpiresAt)
	if err != nil {
		return Session{}, fmt.Errorf("sessions: invalid refresh token")
	}
	return s, nil
}

// Alive reports whether a session id is still valid for the user.
// Used by auth middleware on every request (immediate revocation).
func Alive(ctx context.Context, pool *pgxpool.Pool, sessionID, userID string) bool {
	if sessionID == "" {
		return false
	}
	var one int
	err := pool.QueryRow(ctx,
		`SELECT 1 FROM sessions s JOIN users u ON u.id = s.user_id
		  WHERE s.id = $1::uuid AND s.user_id = $2::uuid
		    AND s.revoked_at IS NULL AND s.expires_at > now()
		    AND u.deleted_at IS NULL AND u.suspended_at IS NULL`, sessionID, userID).Scan(&one)
	return err == nil && one == 1
}

// Revoke marks one session dead (sign-out).
func Revoke(ctx context.Context, pool *pgxpool.Pool, sessionID, userID string) error {
	_, err := pool.Exec(ctx,
		`UPDATE sessions SET revoked_at = now()
		  WHERE id = $1::uuid AND user_id = $2::uuid AND revoked_at IS NULL`,
		sessionID, userID)
	return err
}

// RevokeAllForUser kills every live session (account deletion, password change).
func RevokeAllForUser(ctx context.Context, pool *pgxpool.Pool, userID string) error {
	_, err := pool.Exec(ctx,
		`UPDATE sessions SET revoked_at = now() WHERE user_id = $1::uuid AND revoked_at IS NULL`, userID)
	return err
}
