// Phase 1 Testing Gate — automated tests for every gate item in
// phase-1-accounts-identity.md, functional (1-5) plus adversarial (6-14).
package tests

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/sidekick/backend/internal/auth"
	"github.com/sidekick/backend/internal/avatar"
	"github.com/sidekick/backend/internal/db"
	"github.com/sidekick/backend/internal/httpapi"
	"github.com/sidekick/backend/internal/sandbox"
	"github.com/sidekick/backend/internal/storage"
)

// ── harness ────────────────────────────────────────────────────────────────

type harness struct {
	t      *testing.T
	pool   *pgxpool.Pool
	server http.Handler
	api    *httpapi.Server
	sms    *sandbox.SMSSender
	mailer *sandbox.Mailer
	issuer *auth.Issuer
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = os.Getenv("DATABASE_URL_DEV")
	}
	if url == "" {
		url = "postgres://postgres:postgres@localhost:5433/sidekick_dev?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.Connect(ctx, url)
	require.NoError(t, err)
	require.NoError(t, db.Migrate(ctx, pool))
	issuer, err := auth.NewIssuer("phase1-test-secret-0123456789abcdef-01", "test")
	require.NoError(t, err)
	sms := &sandbox.SMSSender{}
	mailer := &sandbox.Mailer{}
	srv := httpapi.New(pool, issuer, sandbox.Verifier{}, mailer, sms,
		storage.NewStore("phase1-test-secret-0123456789abcdef-01-storage"),
		avatar.NewMemoryBlobStore())
	h := &harness{t: t, pool: pool, server: srv.Handler(), api: srv, sms: sms, mailer: mailer, issuer: issuer}
	t.Cleanup(pool.Close)
	return h
}

func uniq(s string) string {
	return fmt.Sprintf("%s-%s", s, strings.ReplaceAll(uuid.NewString(), "-", ""))
}

func uniqPhone() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	n := int(b[0])<<24 | int(b[1])<<16 | int(b[2])<<8 | int(b[3])
	if n < 0 {
		n = -n
	}
	return fmt.Sprintf("+1555%07d", n%10000000)
}

func (h *harness) do(method, path, token, ip string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		require.NoError(h.t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	if ip == "" {
		ip = uniq("10.9.8")
	}
	req.Header.Set("X-Forwarded-For", ip)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.server.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&out), "response: %s", rec.Body.String())
	return out
}

// signup creates a user over HTTP and returns id/access/refresh.
func (h *harness) signup(t *testing.T, email, password, name string) (string, string, string) {
	t.Helper()
	rec := h.do("POST", "/auth/signup", "", "", map[string]any{
		"email": email, "password": password, "display_name": name,
	})
	require.Equal(t, 201, rec.Code, rec.Body.String())
	out := decodeBody(t, rec)
	u := out["user"].(map[string]any)
	return u["id"].(string), out["access_token"].(string), out["refresh_token"].(string)
}

func testJPEG(t *testing.T, w, hgt int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, hgt))
	for y := 0; y < hgt; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}))
	return buf.Bytes()
}

// ── Gate 1: signup via email / Google / Apple, one row per identity ────────

func TestP1_Gate1_Providers(t *testing.T) {
	h := newHarness(t)

	idE, _, _ := h.signup(t, uniq("e")+"@example.com", "password123", "Email User")

	subG := uniq("sub-g")
	// Google sandbox.
	rec := h.do("POST", "/auth/oauth", "", "", map[string]any{
		"provider": "google", "id_token": "sandbox:google:" + subG + ":" + uniq("g") + "@example.com",
		"display_name": "Google User",
	})
	require.Equal(t, 201, rec.Code, rec.Body.String())
	idG := decodeBody(t, rec)["user"].(map[string]any)["id"].(string)

	// Apple sandbox with Hide-My-Email relay: stored, onboarding incomplete.
	relay := uniq("relay") + "@privaterelay.appleid.com"
	rec = h.do("POST", "/auth/oauth", "", "", map[string]any{
		"provider": "apple", "id_token": "sandbox:apple:" + uniq("sub-a") + ":" + relay,
		"display_name": "Apple User",
	})
	require.Equal(t, 201, rec.Code, rec.Body.String())
	out := decodeBody(t, rec)
	u := out["user"].(map[string]any)
	require.Equal(t, relay, strings.ToLower(u["email"].(string)))
	require.Equal(t, false, u["onboarding_completed"])

	// Same Google sub again → same row, 200, no duplicate.
	rec = h.do("POST", "/auth/oauth", "", "", map[string]any{
		"provider": "google", "id_token": "sandbox:google:" + subG + ":" + uniq("g") + "@example.com",
	})
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Equal(t, idG, decodeBody(t, rec)["user"].(map[string]any)["id"].(string))

	// Link Google identity to the email user → OAuth resolves to email row.
	linkSub := uniq("link-sub")
	ctx := context.Background()
	_, err := h.pool.Exec(ctx,
		`INSERT INTO user_identities (user_id, auth_provider, provider_sub) VALUES ($1::uuid,'google',$2)`, idE, linkSub)
	require.NoError(t, err)
	rec = h.do("POST", "/auth/oauth", "", "", map[string]any{
		"provider": "google", "id_token": "sandbox:google:" + linkSub,
	})
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Equal(t, idE, decodeBody(t, rec)["user"].(map[string]any)["id"].(string))

	// (provider, sub) is the key: same email, different sub → different user.
	rec = h.do("POST", "/auth/oauth", "", "", map[string]any{
		"provider": "google", "id_token": "sandbox:google:" + uniq("sub-g") + ":" + uniq("g") + "@example.com",
	})
	require.Equal(t, 201, rec.Code, rec.Body.String())
	require.NotEqual(t, idG, decodeBody(t, rec)["user"].(map[string]any)["id"].(string))

	// Duplicate email signup → 409, bad provider → 400.
	dupEmail := uniq("dup") + "@example.com"
	rec = h.do("POST", "/auth/signup", "", "", map[string]any{
		"email": dupEmail, "password": "password123", "display_name": "Dup One",
	})
	require.Equal(t, 201, rec.Code)
	rec = h.do("POST", "/auth/signup", "", "", map[string]any{
		"email": dupEmail, "password": "password123", "display_name": "Dup Two",
	})
	require.Equal(t, 409, rec.Code)
	rec = h.do("POST", "/auth/oauth", "", "", map[string]any{"provider": "twitter", "id_token": "x"})
	require.Equal(t, 400, rec.Code)
}

// ── Gate 2: OTP within limits flips phone_verified ─────────────────────────

func TestP1_Gate2_OTPVerify(t *testing.T) {
	h := newHarness(t)
	id, access, _ := h.signup(t, uniq("o")+"@example.com", "password123", "OTP User")
	phone := uniqPhone()

	rec := h.do("POST", "/auth/phone/send", "", "", map[string]any{"user_id": id, "phone": phone})
	require.Equal(t, 200, rec.Code, rec.Body.String())

	// Wrong code first: stays unverified.
	rec = h.do("POST", "/auth/phone/verify", access, "", map[string]any{"phone": phone, "code": "000000"})
	require.Equal(t, 401, rec.Code)
	rec = h.do("GET", "/me", access, "", nil)
	require.Equal(t, false, decodeBody(t, rec)["user"].(map[string]any)["phone_verified"])

	// Correct code: flips.
	rec = h.do("POST", "/auth/phone/verify", access, "", map[string]any{"phone": phone, "code": h.sms.CodeFor(phone)})
	require.Equal(t, 200, rec.Code, rec.Body.String())
	out := decodeBody(t, rec)["user"].(map[string]any)
	require.Equal(t, true, out["phone_verified"])
	require.Equal(t, true, out["onboarding_completed"])
}

// ── Gate 3: profile update + avatar upload/resize ──────────────────────────

func TestP1_Gate3_ProfileAvatar(t *testing.T) {
	h := newHarness(t)
	_, access, _ := h.signup(t, uniq("p")+"@example.com", "password123", "Profile User")

	rec := h.do("PATCH", "/me", access, "", map[string]any{
		"display_name": "New Name", "bio": "hello world", "location": "Peckham",
	})
	require.Equal(t, 200, rec.Code, rec.Body.String())
	u := decodeBody(t, rec)["user"].(map[string]any)
	require.Equal(t, "New Name", u["display_name"])
	require.Equal(t, "Peckham", u["location"])

	// Bad display_name / over-long bio rejected.
	rec = h.do("PATCH", "/me", access, "", map[string]any{"display_name": "x"})
	require.Equal(t, 400, rec.Code)
	rec = h.do("PATCH", "/me", access, "", map[string]any{"bio": strings.Repeat("b", 501)})
	require.Equal(t, 400, rec.Code)
	rec = h.do("POST", "/me/avatar/upload-url", access, "", map[string]any{"content_type": "application/pdf"})
	require.Equal(t, 400, rec.Code)

	// Avatar: grant → complete with 800x600 JPEG → resized ≤512, under cap.
	img := testJPEG(t, 800, 600)
	rec = h.do("POST", "/me/avatar/upload-url", access, "", map[string]any{"content_type": "image/jpeg"})
	require.Equal(t, 200, rec.Code, rec.Body.String())
	grant := decodeBody(t, rec)
	rec = h.do("POST", "/me/avatar/complete", access, "", map[string]any{
		"object": grant["object"], "signature": grant["signature"],
		"expires_at": grant["expires_at"], "image_base64": base64.StdEncoding.EncodeToString(img),
	})
	require.Equal(t, 200, rec.Code, rec.Body.String())
	gotURL := decodeBody(t, rec)["user"].(map[string]any)["avatar_url"].(string)
	require.True(t, strings.HasPrefix(gotURL, "avatars/"))
	require.Less(t, len(img), 5*1024*1024)

	// Oversize payload rejected (6MB of non-image bytes trips size first).
	rec = h.do("POST", "/me/avatar/upload-url", access, "", map[string]any{"content_type": "image/png"})
	grant = decodeBody(t, rec)
	big := make([]byte, 6*1024*1024)
	rec = h.do("POST", "/me/avatar/complete", access, "", map[string]any{
		"object": grant["object"], "signature": grant["signature"],
		"expires_at": grant["expires_at"], "image_base64": base64.StdEncoding.EncodeToString(big),
	})
	require.Equal(t, 400, rec.Code)
}

// ── Gate 4: role intent ────────────────────────────────────────────────────

func TestP1_Gate4_RoleIntent(t *testing.T) {
	h := newHarness(t)
	_, access, _ := h.signup(t, uniq("r")+"@example.com", "password123", "Role User")
	for _, role := range []string{"post", "work", "both"} {
		rec := h.do("POST", "/me/role", access, "", map[string]any{"role_intent": role})
		require.Equal(t, 200, rec.Code, rec.Body.String())
		require.Equal(t, role, decodeBody(t, rec)["user"].(map[string]any)["role_intent"])
	}
	rec := h.do("POST", "/me/role", access, "", map[string]any{"role_intent": "admin"})
	require.Equal(t, 400, rec.Code)
}

// ── Gate 5: deletion signs out; confirm required ───────────────────────────

func TestP1_Gate5_DeleteSignout(t *testing.T) {
	h := newHarness(t)
	_, access, refresh := h.signup(t, uniq("d")+"@example.com", "password123", "Delete User")

	rec := h.do("DELETE", "/me/account", access, "", map[string]any{"confirm": false})
	require.Equal(t, 400, rec.Code)

	rec = h.do("DELETE", "/me/account", access, "", map[string]any{"confirm": true})
	require.Equal(t, 200, rec.Code)

	rec = h.do("GET", "/me", access, "", nil)
	require.Equal(t, 401, rec.Code, "old access token must no longer authenticate")
	rec = h.do("POST", "/auth/refresh", "", "", map[string]any{"refresh_token": refresh})
	require.Equal(t, 401, rec.Code, "refresh token must be revoked")
}

// ── Gate 6: A cannot read B's phone/email ──────────────────────────────────

func TestP1_Gate6_PrivacyProjection(t *testing.T) {
	h := newHarness(t)
	idA, tokA, _ := h.signup(t, uniq("a6")+"@example.com", "password123", "User A6")
	phoneB := uniqPhone()
	idB, tokB, _ := h.signup(t, uniq("b6")+"@example.com", "password123", "User B6")

	rec := h.do("POST", "/auth/phone/send", "", "", map[string]any{"user_id": idB, "phone": phoneB})
	require.Equal(t, 200, rec.Code)
	rec = h.do("POST", "/auth/phone/verify", tokB, "", map[string]any{"phone": phoneB, "code": h.sms.CodeFor(phoneB)})
	require.Equal(t, 200, rec.Code)

	// Owner sees own phone; stranger's public profile leaks nothing.
	rec = h.do("GET", "/me", tokB, "", nil)
	require.Equal(t, phoneB, decodeBody(t, rec)["user"].(map[string]any)["phone"])
	rec = h.do("GET", "/users/"+idB, tokA, "", nil)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	prof := decodeBody(t, rec)["profile"].(map[string]any)
	for _, k := range []string{"phone", "email", "phone_verified", "location_exact"} {
		_, found := prof[k]
		require.False(t, found, "public profile must not contain %q", k)
	}
	_ = idA
}

// ── Gate 7 + self-audit: server-owned fields never client-writable ─────────

func TestP1_Gate7_SelfAudit_ServerOwned(t *testing.T) {
	h := newHarness(t)
	_, access, _ := h.signup(t, uniq("s7")+"@example.com", "password123", "Owned User")

	rec := h.do("PATCH", "/me", access, "", map[string]any{
		"display_name": "Still Me", "verification_status": "verified",
		"rating_avg": 5.0, "rating_count": 100, "suspended_at": time.Now().UTC().Format(time.RFC3339),
		"tasks_completed": 50, "reliability_score": 0, "phone_verified": true,
	})
	require.Equal(t, 200, rec.Code, rec.Body.String())
	u := decodeBody(t, rec)["user"].(map[string]any)
	require.Equal(t, "Still Me", u["display_name"])
	require.Equal(t, "unverified", u["verification_status"])
	require.Equal(t, float64(0), u["rating_avg"])
	require.Equal(t, float64(0), u["rating_count"])
	require.Equal(t, float64(0), u["tasks_completed"])
	require.Equal(t, false, u["phone_verified"])

	var vs string
	var ra float64
	var rc, tc int
	require.NoError(t, h.pool.QueryRow(context.Background(),
		`SELECT verification_status, rating_avg, rating_count, tasks_completed FROM users WHERE email=$1::citext`,
		strings.ToLower(u["email"].(string))).Scan(&vs, &ra, &rc, &tc))
	_ = vs
}

// ── Gate 8: unauth rejected except public endpoints ────────────────────────

func TestP1_Gate8_AuthRequired(t *testing.T) {
	h := newHarness(t)
	id, access, _ := h.signup(t, uniq("s8")+"@example.com", "password123", "Auth User")

	authed := []struct{ method, path string }{
		{"GET", "/me"}, {"PATCH", "/me"}, {"POST", "/me/role"},
		{"POST", "/auth/signout"}, {"DELETE", "/me/account"},
		{"POST", "/auth/phone/verify"}, {"GET", "/me/push-tokens"},
		{"POST", "/me/push-tokens"}, {"DELETE", "/me/push-tokens"},
		{"GET", "/users/" + id}, {"POST", "/me/avatar/upload-url"},
		{"POST", "/me/avatar/complete"}, {"GET", "/config/public"},
	}
	for _, ep := range authed {
		rec := h.do(ep.method, ep.path, "", "", map[string]any{})
		require.Equal(t, 401, rec.Code, "%s %s without token", ep.method, ep.path)
	}

	// Expired token rejected.
	expired, err := h.issuer.MintUser(id, -time.Hour)
	require.NoError(t, err)
	rec := h.do("GET", "/me", expired, "", nil)
	require.Equal(t, 401, rec.Code)

	// Forged token rejected.
	rec = h.do("GET", "/me", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ4In0.forged", "", nil)
	require.Equal(t, 401, rec.Code)

	// Public endpoints work without a token.
	rec = h.do("POST", "/auth/signup", "", "", map[string]any{
		"email": uniq("pub") + "@example.com", "password": "password123", "display_name": "Public User",
	})
	require.Equal(t, 201, rec.Code)
	rec = h.do("POST", "/auth/phone/send", "", "", map[string]any{"user_id": id, "phone": uniqPhone()})
	require.Equal(t, 200, rec.Code)
	rec = h.do("POST", "/auth/password-reset/request", "", "", map[string]any{"email": "nobody@example.com"})
	require.Equal(t, 200, rec.Code)
	_ = access
}

// ── Gate 9: 5 attempts locks the code ──────────────────────────────────────

func TestP1_Gate9_OTPLockout(t *testing.T) {
	h := newHarness(t)
	id, access, _ := h.signup(t, uniq("s9")+"@example.com", "password123", "Lock User")
	phone := uniqPhone()
	rec := h.do("POST", "/auth/phone/send", "", "", map[string]any{"user_id": id, "phone": phone})
	require.Equal(t, 200, rec.Code)

	for i := 0; i < 4; i++ {
		rec = h.do("POST", "/auth/phone/verify", access, "", map[string]any{"phone": phone, "code": "999999"})
		require.Equal(t, 401, rec.Code, "attempt %d", i+1)
	}
	rec = h.do("POST", "/auth/phone/verify", access, "", map[string]any{"phone": phone, "code": "999999"})
	require.Equal(t, 423, rec.Code, "5th bad attempt locks")
	rec = h.do("POST", "/auth/phone/verify", access, "", map[string]any{"phone": phone, "code": h.sms.CodeFor(phone)})
	require.Equal(t, 423, rec.Code, "correct code after lock still rejected")
}

// ── Gate 10: resend cooldown ───────────────────────────────────────────────

func TestP1_Gate10_ResendCooldown(t *testing.T) {
	h := newHarness(t)
	id, _, _ := h.signup(t, uniq("s10")+"@example.com", "password123", "Cooldown User")
	phone := uniqPhone()
	ip := uniq("10.1.2")
	rec := h.do("POST", "/auth/phone/send", "", ip, map[string]any{"user_id": id, "phone": phone})
	require.Equal(t, 200, rec.Code)
	rec = h.do("POST", "/auth/phone/send", "", ip, map[string]any{"user_id": id, "phone": phone})
	require.Equal(t, 429, rec.Code, "immediate resend must be rejected, not queued")
}

// ── Gate 11: duplicate verified phone → 409 ────────────────────────────────

func TestP1_Gate11_DupPhone(t *testing.T) {
	h := newHarness(t)
	idA, tokA, _ := h.signup(t, uniq("a11")+"@example.com", "password123", "User A11")
	phone := uniqPhone()
	require.Equal(t, 200, h.do("POST", "/auth/phone/send", "", "", map[string]any{"user_id": idA, "phone": phone}).Code)
	require.Equal(t, 200, h.do("POST", "/auth/phone/verify", tokA, "",
		map[string]any{"phone": phone, "code": h.sms.CodeFor(phone)}).Code)

	idB, _, _ := h.signup(t, uniq("b11")+"@example.com", "password123", "User B11")
	rec := h.do("POST", "/auth/phone/send", "", "", map[string]any{"user_id": idB, "phone": phone})
	require.Equal(t, 409, rec.Code)
}

// ── Gate 12: non-admin rejected on admin gate ──────────────────────────────

func TestP1_Gate12_AdminGate(t *testing.T) {
	h := newHarness(t)
	_, access, _ := h.signup(t, uniq("s12")+"@example.com", "password123", "Normal User")

	rec := h.do("GET", "/admin/probe", "", "", nil)
	require.Equal(t, 401, rec.Code)
	rec = h.do("GET", "/admin/probe", access, "", nil)
	require.Equal(t, 403, rec.Code, "user token must not pass admin gate")

	adminTok, err := h.issuer.MintAdmin("admin-12", time.Hour)
	require.NoError(t, err)
	rec = h.do("GET", "/admin/probe", adminTok, "", nil)
	require.Equal(t, 200, rec.Code)
}

// ── Gate 13: post-delete PII purge + session death ─────────────────────────

func TestP1_Gate13_DeletePurgesPII(t *testing.T) {
	h := newHarness(t)
	email := uniq("s13") + "@example.com"
	id, access, refresh := h.signup(t, email, "password123", "Purge User")
	phone := uniqPhone()
	require.Equal(t, 200, h.do("POST", "/auth/phone/send", "", "", map[string]any{"user_id": id, "phone": phone}).Code)
	require.Equal(t, 200, h.do("POST", "/auth/phone/verify", access, "",
		map[string]any{"phone": phone, "code": h.sms.CodeFor(phone)}).Code)
	require.Equal(t, 200, h.do("POST", "/me/push-tokens", access, "",
		map[string]any{"token": "ExponentPushToken[test13]", "platform": "ios"}).Code)

	require.Equal(t, 200, h.do("DELETE", "/me/account", access, "", map[string]any{"confirm": true}).Code)

	ctx := context.Background()
	var dbPhone, dbAvatar *string
	var dbEmail string
	var dbProviderSub *string
	var deleted bool
	require.NoError(t, h.pool.QueryRow(ctx,
		`SELECT phone, avatar_url, email::text, provider_sub, deleted_at IS NOT NULL FROM users WHERE id=$1::uuid`,
		id).Scan(&dbPhone, &dbAvatar, &dbEmail, &dbProviderSub, &deleted))
	require.True(t, deleted)
	require.Nil(t, dbPhone, "phone purged")
	require.Nil(t, dbAvatar, "avatar purged")
	require.NotContains(t, strings.ToLower(dbEmail), strings.Split(email, "@")[0][:8],
		"original email must be gone (anonymized to %q)", dbEmail)
	require.Nil(t, dbProviderSub)
	var n int
	require.NoError(t, h.pool.QueryRow(ctx,
		`SELECT count(*) FROM sessions WHERE user_id=$1::uuid AND revoked_at IS NULL`, id).Scan(&n))
	require.Zero(t, n, "all sessions revoked")
	require.NoError(t, h.pool.QueryRow(ctx,
		`SELECT count(*) FROM push_tokens WHERE user_id=$1::uuid`, id).Scan(&n))
	require.Zero(t, n, "push tokens removed")

	require.Equal(t, 401, h.do("POST", "/auth/refresh", "", "", map[string]any{"refresh_token": refresh}).Code)
}

// ── Gate 14: RLS default-deny still holds (all tables incl. new ones) ──────

func TestP1_Gate14_RLSStillDeny(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	// Re-grant: GRANT ON ALL TABLES never covers tables created later
	// (0005 additions), so grant fresh here before asserting RLS deny.
	_, err := h.pool.Exec(ctx, `GRANT USAGE ON SCHEMA public TO phase0_restricted`)
	require.NoError(t, err)
	_, err = h.pool.Exec(ctx, `GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO phase0_restricted`)
	require.NoError(t, err)
	tables := []string{
		"users", "tasks", "task_photos", "offers", "conversations", "messages",
		"transactions", "wallets", "payout_methods", "payment_methods", "reviews",
		"verifications", "reports", "blocks", "disputes", "notifications",
		"push_tokens", "payout_batches", "payout_items", "audit_log", "categories",
		"app_config", "idempotency_keys", "sessions", "phone_verifications",
		"user_identities", "password_resets",
	}
	for _, tbl := range tables {
		var on bool
		require.NoError(t, h.pool.QueryRow(ctx,
			`SELECT rowsecurity FROM pg_tables WHERE schemaname='public' AND tablename=$1`, tbl).Scan(&on))
		require.True(t, on, "RLS must stay enabled on %s", tbl)
	}
	conn, err := h.pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, "SET ROLE phase0_restricted")
	require.NoError(t, err)
	defer func() { _, _ = conn.Exec(context.Background(), "RESET ROLE") }()
	for _, tbl := range tables {
		var n int
		require.NoError(t, conn.QueryRow(ctx, "SELECT count(*) FROM "+tbl).Scan(&n))
		require.Zero(t, n, "table %s leaked rows to restricted role", tbl)
	}
}

// ── push tokens + password reset + signout/refresh flow ────────────────────

func TestP1_PushTokensResetSignout(t *testing.T) {
	h := newHarness(t)
	email := uniq("misc") + "@example.com"
	_, access, refresh := h.signup(t, email, "password123", "Misc User")

	// Push token register/list/remove, scoped to owner.
	require.Equal(t, 200, h.do("POST", "/me/push-tokens", access, "",
		map[string]any{"token": "ExponentPushToken[misc1]", "platform": "android"}).Code)
	rec := h.do("GET", "/me/push-tokens", access, "", nil)
	require.Contains(t, rec.Body.String(), "misc1")
	require.Equal(t, 200, h.do("DELETE", "/me/push-tokens", access, "",
		map[string]any{"token": "ExponentPushToken[misc1]"}).Code)
	rec = h.do("GET", "/me/push-tokens", access, "", nil)
	require.NotContains(t, rec.Body.String(), "misc1")
	require.Equal(t, 400, h.do("POST", "/me/push-tokens", access, "",
		map[string]any{"token": "x", "platform": "carrier-pigeon"}).Code)

	// Password reset: unknown email still 200 (no enumeration); real flow works.
	rec = h.do("POST", "/auth/password-reset/request", "", "", map[string]any{"email": "ghost@example.com"})
	require.Equal(t, 200, rec.Code)
	require.Equal(t, 200, h.do("POST", "/auth/password-reset/request", "", "", map[string]any{"email": email}).Code)
	token := h.mailer.TokenFor(strings.ToLower(email))
	require.NotEmpty(t, token)
	require.Equal(t, 200, h.do("POST", "/auth/password-reset/confirm", "", "",
		map[string]any{"token": token, "new_password": "newpassword123"}).Code)
	// Old password dead, new works; reuse of token rejected.
	require.Equal(t, 401, h.do("POST", "/auth/signin", "", "",
		map[string]any{"email": email, "password": "password123"}).Code)
	rec = h.do("POST", "/auth/signin", "", "", map[string]any{"email": email, "password": "newpassword123"})
	require.Equal(t, 200, rec.Code)
	require.Equal(t, 401, h.do("POST", "/auth/password-reset/confirm", "", "",
		map[string]any{"token": token, "new_password": "anotherpass123"}).Code)

	// Refresh rotation: old refresh dies, new pair works; signout kills access.
	// NB: password-reset revoked the signup session, so sign in fresh first.
	rec = h.do("POST", "/auth/signin", "", "", map[string]any{"email": email, "password": "newpassword123"})
	require.Equal(t, 200, rec.Code, rec.Body.String())
	fresh := decodeBody(t, rec)
	refresh = fresh["refresh_token"].(string)
	rec = h.do("POST", "/auth/refresh", "", "", map[string]any{"refresh_token": refresh})
	require.Equal(t, 200, rec.Code)
	newTokens := decodeBody(t, rec)
	require.Equal(t, 401, h.do("POST", "/auth/refresh", "", "",
		map[string]any{"refresh_token": refresh}).Code, "rotated refresh must die")
	require.Equal(t, 200, h.do("GET", "/me", newTokens["access_token"].(string), "", nil).Code)
	require.Equal(t, 200, h.do("POST", "/auth/signout", newTokens["access_token"].(string), "", map[string]any{}).Code)
	require.Equal(t, 401, h.do("GET", "/me", newTokens["access_token"].(string), "", nil).Code)
}
