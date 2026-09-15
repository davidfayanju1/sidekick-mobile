// Package otp implements Phase 1 §2.2 phone verification.
//
// Limits (from app_config, seeded in Phase 0):
//   - code TTL 5 min (otp_ttl_seconds)
//   - max 5 verification attempts per code, then locked (otp_max_attempts)
//   - 60 s cooldown between sends per phone (otp_resend_cooldown_seconds)
//   - hourly cap per phone (DB-backed sent_count window)
//   - per-IP sliding window (in-memory; single-instance. A Redis window
//     replaces this when the API runs multi-instance — documented, not hidden.)
// A phone already held by another account is rejected with 409.
package otp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sidekick/backend/internal/audit"
)

var (
	ErrCooldown   = errors.New("resend cooldown active")
	ErrRateLimit  = errors.New("rate limit exceeded")
	ErrLocked     = errors.New("code locked after too many attempts")
	ErrExpired    = errors.New("code expired")
	ErrInvalid    = errors.New("invalid code")
	ErrConflict   = errors.New("phone already in use")
	ErrBadRequest = errors.New("bad request")
)

// Sender transmits the code. TestSender captures for tests; production
// wires Twilio/Vonage behind this same boundary.
type Sender interface {
	SendSMS(ctx context.Context, phone, code string) error
}

type Service struct {
	pool   *pgxpool.Pool
	sender Sender

	mu sync.Mutex
	// ipHits maps IP -> recent send timestamps (sliding 1h window, cap 20).
	ipHits map[string][]time.Time
}

func New(pool *pgxpool.Pool, sender Sender) *Service {
	return &Service{pool: pool, sender: sender, ipHits: map[string][]time.Time{}}
}

func (s *Service) cfg(ctx context.Context, key string, fallback int) int {
	var raw string
	if err := s.pool.QueryRow(ctx, `SELECT value::text FROM app_config WHERE key=$1`, key).Scan(&raw); err != nil {
		return fallback
	}
	raw = strings.Trim(raw, `"`)
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func validPhone(phone string) bool {
	phone = strings.TrimSpace(phone)
	if !strings.HasPrefix(phone, "+") || len(phone) < 8 || len(phone) > 16 {
		return false
	}
	for _, c := range phone[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func newCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()+100000), nil
}

func hashCode(code string) string {
	sum := sha256.Sum256([]byte("otp:" + code))
	return hex.EncodeToString(sum[:])
}

// checkIP enforces the per-IP sliding window (cap 20 sends/hour).
func (s *Service) checkIP(ip string) error {
	if ip == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	cut := now.Add(-time.Hour)
	hits := s.ipHits[ip][:0]
	for _, t := range s.ipHits[ip] {
		if t.After(cut) {
			hits = append(hits, t)
		}
	}
	if len(hits) >= 20 {
		s.ipHits[ip] = hits
		return ErrRateLimit
	}
	s.ipHits[ip] = append(hits, now)
	return nil
}

// Send issues a code to phone for userID.
func (s *Service) Send(ctx context.Context, userID, phone, ip string) error {
	phone = strings.TrimSpace(phone)
	if !validPhone(phone) {
		return fmt.Errorf("%w: phone must be E.164", ErrBadRequest)
	}
	// 409: phone belongs to a different account (verified or not).
	var other string
	err := s.pool.QueryRow(ctx,
		`SELECT id::text FROM users WHERE phone = $1 AND id <> $2::uuid`, phone, userID).Scan(&other)
	if err == nil {
		return ErrConflict
	}
	if err := s.checkIP(ip); err != nil {
		return err
	}
	cooldown := s.cfg(ctx, "otp_resend_cooldown_seconds", 60)
	ttl := s.cfg(ctx, "otp_ttl_seconds", 300)

	var lastSent time.Time
	var sentCount int
	var windowStart time.Time
	var has bool
	err = s.pool.QueryRow(ctx,
		`SELECT last_sent_at, sent_count, window_started_at FROM phone_verifications WHERE phone=$1`,
		phone).Scan(&lastSent, &sentCount, &windowStart)
	has = err == nil
	now := time.Now()
	if has {
		if now.Sub(lastSent) < time.Duration(cooldown)*time.Second {
			return ErrCooldown
		}
		if now.Sub(windowStart) > time.Hour {
			sentCount, windowStart = 0, now
		}
		if sentCount >= 5 {
			return fmt.Errorf("%w: too many sends for this number", ErrRateLimit)
		}
	}
	code, err := newCode()
	if err != nil {
		return err
	}
	if s.sender != nil {
		if err := s.sender.SendSMS(ctx, phone, code); err != nil {
			return err
		}
	}
	if has {
		_, err = s.pool.Exec(ctx,
			`UPDATE phone_verifications SET code_hash=$1, attempts=0, last_sent_at=now(),
			 expires_at=now()+($2::text||' seconds')::interval,
			 sent_count=$3, window_started_at=$4, verified_at=NULL WHERE phone=$5`,
			hashCode(code), strconv.Itoa(ttl), sentCount+1, windowStart, phone)
	} else {
		_, err = s.pool.Exec(ctx,
			`INSERT INTO phone_verifications (phone, code_hash, expires_at, sent_count, window_started_at)
			 VALUES ($1, $2, now()+($3::text||' seconds')::interval, 1, now())`,
			phone, hashCode(code), strconv.Itoa(ttl))
	}
	return err
}

// Verify checks the code and binds the phone to the user.
func (s *Service) Verify(ctx context.Context, userID, phone, code string) error {
	phone = strings.TrimSpace(phone)
	maxAttempts := s.cfg(ctx, "otp_max_attempts", 5)
	var codeHash string
	var attempts int
	var expires time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT code_hash, attempts, expires_at FROM phone_verifications WHERE phone=$1`, phone).
		Scan(&codeHash, &attempts, &expires)
	if err != nil {
		return ErrInvalid
	}
	if attempts >= maxAttempts {
		return ErrLocked
	}
	if time.Now().After(expires) {
		return ErrExpired
	}
	if subtle.ConstantTimeCompare([]byte(codeHash), []byte(hashCode(strings.TrimSpace(code)))) != 1 {
		_, _ = s.pool.Exec(ctx, `UPDATE phone_verifications SET attempts = attempts + 1 WHERE phone=$1`, phone)
		// Re-read: the increment may have hit the cap — report locked then.
		if attempts+1 >= maxAttempts {
			return ErrLocked
		}
		return ErrInvalid
	}
	// 409 re-check at bind time (race-safe via users.phone unique).
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var other string
	err = tx.QueryRow(ctx, `SELECT id::text FROM users WHERE phone=$1 AND id<>$2::uuid`, phone, userID).Scan(&other)
	if err == nil {
		return ErrConflict
	}
	if _, err := tx.Exec(ctx,
		`UPDATE users SET phone=$1, phone_verified=TRUE, phone_verified_at=now(), updated_at=now()
		  WHERE id=$2::uuid AND deleted_at IS NULL`, phone, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE phone_verifications SET verified_at=now() WHERE phone=$1`, phone); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE users SET onboarding_completed=TRUE, updated_at=now()
		  WHERE id=$1::uuid AND char_length(display_name) >= 2`, userID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			return ErrConflict
		}
		return err
	}
	_ = audit.Log(ctx, s.pool, audit.Entry{ActorID: &userID, Action: "user.phone_verified",
		EntityType: "user", EntityID: userID})
	return nil
}
