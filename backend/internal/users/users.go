// Package users implements Phase 1 §2.1–§2.6: providers, profile,
// role intent, password reset, sign-out support and account deletion.
//
// Identity rule (§4.1): the key is (auth_provider, provider_sub) — never
// email. A second provider links to the same row via user_identities.
// Apple relay addresses are stored as email; onboarding completes only
// after the phone OTP step.
package users

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/sidekick/backend/internal/audit"
)

var (
	ErrConflict    = errors.New("conflict")
	ErrNotFound    = errors.New("not found")
	ErrUnauthorized = errors.New("unauthorized")
	ErrGone        = errors.New("account deleted or suspended")
	ErrBadRequest  = errors.New("bad request")
)

// Server-owned fields: no client request may ever write these (self-audit).
var serverOwnedFields = []string{
	"verification_status", "rating_avg", "rating_count",
	"tasks_completed", "suspended_at", "reliability_score",
}

// User mirrors the users row.
type User struct {
	ID                  string
	AuthProvider        string
	ProviderSub         *string
	Email               *string
	Phone               *string
	PhoneVerified       bool
	PhoneVerifiedAt     *time.Time
	DisplayName         string
	AvatarURL           *string
	Bio                 *string
	Location            *string
	Latitude            *float64
	Longitude           *float64
	RoleIntent          *string
	VerificationStatus  string
	RatingAvg           float64
	RatingCount         int
	TasksCompleted      int
	SuspendedAt         *time.Time
	DeletedAt           *time.Time
	OnboardingCompleted bool
	CreatedAt           time.Time
}

// PublicProfile is the restricted projection any authenticated user may
// read: NO phone, NO email, NO exact location. Ever.
type PublicProfile struct {
	ID                 string  `json:"id"`
	DisplayName        string  `json:"display_name"`
	AvatarURL          *string `json:"avatar_url,omitempty"`
	Bio                *string `json:"bio,omitempty"`
	Location           *string `json:"location,omitempty"`
	VerificationStatus string  `json:"verification_status"`
	RatingAvg          float64 `json:"rating_avg"`
	RatingCount        int     `json:"rating_count"`
	TasksCompleted     int     `json:"tasks_completed"`
	MemberSince        time.Time `json:"member_since"`
}

// Mailer delivers password-reset emails. TestMailer captures in tests.
type Mailer interface {
	SendPasswordReset(ctx context.Context, toEmail, rawToken string) error
}

// OAuthVerifier validates a provider token in sandbox/test mode and
// returns the stable subject + email. Production wires real Google/Apple
// verification behind this same boundary.
type OAuthVerifier interface {
	Verify(ctx context.Context, provider, idToken string) (providerSub string, email string, err error)
}

type Service struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

const userCols = `id::text, auth_provider, provider_sub, email::text, phone, phone_verified,
	phone_verified_at, display_name, avatar_url, bio, location, latitude, longitude,
	role_intent, verification_status, rating_avg, rating_count, tasks_completed,
	suspended_at, deleted_at, onboarding_completed, created_at`

func scanUser(r pgx.Row) (*User, error) {
	var u User
	err := r.Scan(&u.ID, &u.AuthProvider, &u.ProviderSub, &u.Email, &u.Phone,
		&u.PhoneVerified, &u.PhoneVerifiedAt, &u.DisplayName, &u.AvatarURL,
		&u.Bio, &u.Location, &u.Latitude, &u.Longitude, &u.RoleIntent,
		&u.VerificationStatus, &u.RatingAvg, &u.RatingCount, &u.TasksCompleted,
		&u.SuspendedAt, &u.DeletedAt, &u.OnboardingCompleted, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func normalizeEmail(e string) string {
	return strings.ToLower(strings.TrimSpace(e))
}

// ── email/password ─────────────────────────────────────────────────────────

func (s *Service) SignupEmail(ctx context.Context, email, password, displayName string) (*User, error) {
	email = normalizeEmail(email)
	if email == "" || !strings.Contains(email, "@") {
		return nil, fmt.Errorf("%w: invalid email", ErrBadRequest)
	}
	if len(password) < 8 {
		return nil, fmt.Errorf("%w: password must be at least 8 chars", ErrBadRequest)
	}
	if l := len([]rune(displayName)); l < 2 || l > 50 {
		return nil, fmt.Errorf("%w: display_name must be 2-50 chars", ErrBadRequest)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u, err := scanUser(s.pool.QueryRow(ctx,
		`INSERT INTO users (auth_provider, email, password_hash, display_name)
		 VALUES ('email', $1::citext, $2, $3)
		 RETURNING `+userCols, email, string(hash), displayName))
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			return nil, fmt.Errorf("%w: email already registered", ErrConflict)
		}
		return nil, err
	}
	_ = audit.Log(ctx, s.pool, audit.Entry{ActorID: &u.ID, Action: "user.signup", EntityType: "user", EntityID: u.ID})
	return u, nil
}

func (s *Service) SigninEmail(ctx context.Context, email, password string) (*User, error) {
	email = normalizeEmail(email)
	var u *User
	var hash *string
	var err error
	u, err = scanUser(s.pool.QueryRow(ctx,
		`SELECT `+userCols+` FROM users WHERE email = $1::citext`, email))
	if err != nil {
		return nil, fmt.Errorf("%w: invalid credentials", ErrUnauthorized)
	}
	err = s.pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1::uuid`, u.ID).Scan(&hash)
	if err != nil || hash == nil {
		return nil, fmt.Errorf("%w: invalid credentials", ErrUnauthorized)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*hash), []byte(password)); err != nil {
		return nil, fmt.Errorf("%w: invalid credentials", ErrUnauthorized)
	}
	if u.DeletedAt != nil || u.SuspendedAt != nil {
		return nil, ErrGone
	}
	return u, nil
}

// ── OAuth (Google / Apple sandbox) ─────────────────────────────────────────

func (s *Service) OAuthSignin(ctx context.Context, v OAuthVerifier, provider, idToken, displayName string) (*User, bool, error) {
	if provider != "google" && provider != "apple" {
		return nil, false, fmt.Errorf("%w: provider must be google or apple", ErrBadRequest)
	}
	sub, email, err := v.Verify(ctx, provider, idToken)
	if err != nil || sub == "" {
		return nil, false, fmt.Errorf("%w: invalid provider token", ErrUnauthorized)
	}
	// 1) primary identity on users.
	if u, err := scanUser(s.pool.QueryRow(ctx,
		`SELECT `+userCols+` FROM users WHERE auth_provider = $1 AND provider_sub = $2`, provider, sub)); err == nil {
		if u.DeletedAt != nil || u.SuspendedAt != nil {
			return nil, false, ErrGone
		}
		return u, false, nil
	}
	// 2) linked identity.
	var userID string
	if err := s.pool.QueryRow(ctx,
		`SELECT user_id::text FROM user_identities WHERE auth_provider = $1 AND provider_sub = $2`,
		provider, sub).Scan(&userID); err == nil {
		u, err := s.GetByID(ctx, userID)
		if err != nil {
			return nil, false, err
		}
		if u.DeletedAt != nil || u.SuspendedAt != nil {
			return nil, false, ErrGone
		}
		return u, false, nil
	}
	// 3) new user. Apple relay addresses stored as-is; OTP still required.
	if displayName == "" {
		displayName = strings.Split(email, "@")[0]
		if displayName == "" {
			displayName = "Sidekick"
		}
	}
	var emailArg any
	if normalizeEmail(email) != "" {
		emailArg = normalizeEmail(email)
	}
	u, err := scanUser(s.pool.QueryRow(ctx,
		`INSERT INTO users (auth_provider, provider_sub, email, display_name)
		 VALUES ($1, $2, $3::citext, $4) RETURNING `+userCols,
		provider, sub, emailArg, displayName))
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			return nil, false, fmt.Errorf("%w: identity already registered", ErrConflict)
		}
		return nil, false, err
	}
	_ = audit.Log(ctx, s.pool, audit.Entry{ActorID: &u.ID, Action: "user.oauth_signup",
		EntityType: "user", EntityID: u.ID, Metadata: map[string]any{"provider": provider}})
	return u, true, nil
}

// LinkProvider attaches a second provider to an existing account.
func (s *Service) LinkProvider(ctx context.Context, userID, provider, sub string) error {
	if provider != "google" && provider != "apple" {
		return fmt.Errorf("%w: provider must be google or apple", ErrBadRequest)
	}
	if sub == "" {
		return fmt.Errorf("%w: provider_sub required", ErrBadRequest)
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO user_identities (user_id, auth_provider, provider_sub)
		 VALUES ($1::uuid, $2, $3)`, userID, provider, sub)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			return fmt.Errorf("%w: provider already linked elsewhere", ErrConflict)
		}
		return err
	}
	_ = audit.Log(ctx, s.pool, audit.Entry{ActorID: &userID, Action: "user.link_provider",
		EntityType: "user", EntityID: userID, Metadata: map[string]any{"provider": provider}})
	return nil
}

// ── reads ──────────────────────────────────────────────────────────────────

func (s *Service) GetByID(ctx context.Context, id string) (*User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE id = $1::uuid`, id))
	if err != nil {
		return nil, ErrNotFound
	}
	return u, nil
}

func (s *Service) GetPublicProfile(ctx context.Context, id string) (*PublicProfile, error) {
	var p PublicProfile
	err := s.pool.QueryRow(ctx,
		`SELECT id::text, display_name, avatar_url, bio, location,
		        verification_status, rating_avg, rating_count, tasks_completed, created_at
		   FROM users WHERE id = $1::uuid AND deleted_at IS NULL AND suspended_at IS NULL`, id).
		Scan(&p.ID, &p.DisplayName, &p.AvatarURL, &p.Bio, &p.Location,
			&p.VerificationStatus, &p.RatingAvg, &p.RatingCount, &p.TasksCompleted, &p.MemberSince)
	if err != nil {
		return nil, ErrNotFound
	}
	return &p, nil
}

// ── profile update (strict allowlist) ──────────────────────────────────────

type ProfilePatch struct {
	DisplayName *string  `json:"display_name"`
	Bio         *string  `json:"bio"`
	Location    *string  `json:"location"`
	Latitude    *float64 `json:"latitude"`
	Longitude   *float64 `json:"longitude"`
	AvatarURL   *string  `json:"avatar_url"`
}

func (s *Service) UpdateProfile(ctx context.Context, userID string, p ProfilePatch) (*User, error) {
	u, err := s.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u.DeletedAt != nil || u.SuspendedAt != nil {
		return nil, ErrGone
	}
	if p.DisplayName != nil {
		if l := len([]rune(*p.DisplayName)); l < 2 || l > 50 {
			return nil, fmt.Errorf("%w: display_name must be 2-50 chars", ErrBadRequest)
		}
		u.DisplayName = *p.DisplayName
	}
	if p.Bio != nil {
		if len([]rune(*p.Bio)) > 500 {
			return nil, fmt.Errorf("%w: bio must be <=500 chars", ErrBadRequest)
		}
		u.Bio = p.Bio
	}
	if p.Location != nil {
		u.Location = p.Location
	}
	if p.Latitude != nil {
		u.Latitude = p.Latitude
	}
	if p.Longitude != nil {
		u.Longitude = p.Longitude
	}
	if p.AvatarURL != nil {
		u.AvatarURL = p.AvatarURL
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE users SET display_name=$1, bio=$2, location=$3, latitude=$4, longitude=$5,
		 avatar_url=$6, updated_at=now() WHERE id=$7::uuid`,
		u.DisplayName, u.Bio, u.Location, u.Latitude, u.Longitude, u.AvatarURL, userID)
	if err != nil {
		return nil, err
	}
	_ = s.refreshOnboarding(ctx, userID)
	return s.GetByID(ctx, userID)
}

// refreshOnboarding flips the flag once display name + verified phone exist.
func (s *Service) refreshOnboarding(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET onboarding_completed = TRUE, updated_at = now()
		  WHERE id = $1::uuid AND phone_verified AND char_length(display_name) >= 2`, userID)
	return err
}

// ── role intent ────────────────────────────────────────────────────────────

func (s *Service) SetRoleIntent(ctx context.Context, userID, intent string) (*User, error) {
	if intent != "post" && intent != "work" && intent != "both" {
		return nil, fmt.Errorf("%w: role_intent must be post, work or both", ErrBadRequest)
	}
	u, err := s.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u.DeletedAt != nil || u.SuspendedAt != nil {
		return nil, ErrGone
	}
	_, err = s.pool.Exec(ctx, `UPDATE users SET role_intent=$1, updated_at=now() WHERE id=$2::uuid`, intent, userID)
	if err != nil {
		return nil, err
	}
	_ = audit.Log(ctx, s.pool, audit.Entry{ActorID: &userID, Action: "user.set_role",
		EntityType: "user", EntityID: userID, Metadata: map[string]any{"role_intent": intent}})
	return s.GetByID(ctx, userID)
}

// ── password reset ─────────────────────────────────────────────────────────

func (s *Service) RequestPasswordReset(ctx context.Context, mailer Mailer, email string) error {
	email = normalizeEmail(email)
	var id, em string
	err := s.pool.QueryRow(ctx,
		`SELECT id::text, email::text FROM users WHERE email = $1::citext AND deleted_at IS NULL`, email).
		Scan(&id, &em)
	if err != nil {
		return nil // no enumeration: still "ok"
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	token := hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	_, err = s.pool.Exec(ctx,
		`INSERT INTO password_resets (user_id, token_hash) VALUES ($1::uuid, $2)`,
		id, hex.EncodeToString(sum[:]))
	if err != nil {
		return err
	}
	if mailer != nil {
		_ = mailer.SendPasswordReset(ctx, em, token)
	}
	_ = audit.Log(ctx, s.pool, audit.Entry{ActorID: &id, Action: "user.reset_request",
		EntityType: "user", EntityID: id})
	return nil
}

func (s *Service) ConfirmPasswordReset(ctx context.Context, rawToken, newPassword string) error {
	if len(newPassword) < 8 {
		return fmt.Errorf("%w: password must be at least 8 chars", ErrBadRequest)
	}
	sum := sha256.Sum256([]byte(rawToken))
	var userID string
	err := s.pool.QueryRow(ctx,
		`SELECT user_id::text FROM password_resets
		  WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()`,
		hex.EncodeToString(sum[:])).Scan(&userID)
	if err != nil {
		return fmt.Errorf("%w: invalid or expired reset token", ErrUnauthorized)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `UPDATE users SET password_hash=$1, updated_at=now() WHERE id=$2::uuid`,
		string(hash), userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE password_resets SET used_at=now() WHERE token_hash=$1`,
		hex.EncodeToString(sum[:])); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE sessions SET revoked_at=now() WHERE user_id=$1::uuid AND revoked_at IS NULL`,
		userID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	_ = audit.Log(ctx, s.pool, audit.Entry{ActorID: &userID, Action: "user.reset_confirm",
		EntityType: "user", EntityID: userID})
	return nil
}

// ── deletion ───────────────────────────────────────────────────────────────

// DeleteAccount soft-deletes, purges PII, revokes sessions + push tokens.
// confirm must be true (explicit confirmation step, §2.5). Ledger rows in
// later phases are retained — this phase deletes none of them.
func (s *Service) DeleteAccount(ctx context.Context, userID string, confirm bool) error {
	if !confirm {
		return fmt.Errorf("%w: deletion requires explicit confirmation", ErrBadRequest)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var exists bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1::uuid AND deleted_at IS NULL)`,
		userID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	anonEmail := fmt.Sprintf("deleted+%s@deleted.local", userID[:8])
	if _, err := tx.Exec(ctx,
		`UPDATE users SET deleted_at=now(), email=$2::citext, phone=NULL, phone_verified=FALSE,
		 phone_verified_at=NULL, avatar_url=NULL, location=NULL, latitude=NULL, longitude=NULL,
		 bio=NULL, password_hash=NULL, provider_sub=NULL, updated_at=now()
		  WHERE id=$1::uuid`, userID, anonEmail); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM user_identities WHERE user_id=$1::uuid`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE sessions SET revoked_at=now() WHERE user_id=$1::uuid AND revoked_at IS NULL`,
		userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM push_tokens WHERE user_id=$1::uuid`, userID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	_ = audit.Log(ctx, s.pool, audit.Entry{ActorID: &userID, Action: "user.delete",
		EntityType: "user", EntityID: userID})
	return nil
}
