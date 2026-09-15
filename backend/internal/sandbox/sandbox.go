// Package sandbox holds provider sandbox doubles: OAuth verification,
// SMS capture and email capture. Production replaces each behind the same
// interface (real Google/Apple verification, Twilio/Vonage, SES) without
// touching callers.
package sandbox

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// ── OAuth ──────────────────────────────────────────────────────────────────
// Sandbox token format: "sandbox:<provider>:<sub>[:email]".
// Apple relay is simulated with "@privaterelay.appleid.com" addresses.

type Verifier struct{}

func (Verifier) Verify(_ context.Context, provider, idToken string) (string, string, error) {
	parts := strings.SplitN(idToken, ":", 4)
	if len(parts) < 3 || parts[0] != "sandbox" || parts[1] != provider || parts[2] == "" {
		return "", "", fmt.Errorf("sandbox: invalid token")
	}
	email := ""
	if len(parts) == 4 {
		email = parts[3]
	}
	return parts[2], email, nil
}

// ── SMS ────────────────────────────────────────────────────────────────────

type SMSSender struct {
	mu   sync.Mutex
	Codes map[string]string // phone -> last code
}

func (s *SMSSender) SendSMS(_ context.Context, phone, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Codes == nil {
		s.Codes = map[string]string{}
	}
	s.Codes[phone] = code
	return nil
}

func (s *SMSSender) CodeFor(phone string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Codes[phone]
}

// ── Email ──────────────────────────────────────────────────────────────────

type Mailer struct {
	mu     sync.Mutex
	Resets map[string]string // email -> last raw reset token
}

func (m *Mailer) SendPasswordReset(_ context.Context, toEmail, rawToken string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Resets == nil {
		m.Resets = map[string]string{}
	}
	m.Resets[toEmail] = rawToken
	return nil
}

func (m *Mailer) TokenFor(email string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Resets[email]
}
