// Package httpapi wires Phase 1 endpoints (stdlib ServeMux).
//
// Auth rule (§2.6): every endpoint requires a valid session EXCEPT
// signup, signin, oauth, phone-send, password-reset request/confirm and
// refresh. RLS decision (§3, documented): app_config is
// authenticated-readable via GET /config/public (non-sensitive keys only);
// everything else is server-side. Public profiles never contain phone,
// email or exact location — enforced by the projection, not by trust.
package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sidekick/backend/internal/auth"
	"github.com/sidekick/backend/internal/avatar"
	"github.com/sidekick/backend/internal/blobstore"
	"github.com/sidekick/backend/internal/discovery"
	"github.com/sidekick/backend/internal/messaging"
	"github.com/sidekick/backend/internal/otp"
	"github.com/sidekick/backend/internal/payments"
	"github.com/sidekick/backend/internal/pushtokens"
	"github.com/sidekick/backend/internal/sessions"
	"github.com/sidekick/backend/internal/storage"
	"github.com/sidekick/backend/internal/users"
)

type ctxKey string

const userKey ctxKey = "user"

type Server struct {
	pool       *pgxpool.Pool
	issuer     *auth.Issuer
	users      *users.Service
	otp        *otp.Service
	avatars    *avatar.Service
	oauth      users.OAuthVerifier
	mailer     users.Mailer
	payments   payments.Adapter
	signer     *storage.Store
	photoBlobs *blobstore.MemoryBlobStore
	discovery  *discovery.Service
	messaging  *messaging.Service
	chatBlobs  *blobstore.MemoryBlobStore
	// payOverride lets tests inject a failing adapter without rebuilding.
	payOverride payments.Adapter
}

// SetPaymentsAdapter swaps the payments backend (tests use it to simulate
// provider failure; production wires the real adapter in main).
func (s *Server) SetPaymentsAdapter(p payments.Adapter) { s.payOverride = p }

func (s *Server) activePayments() payments.Adapter {
	if s.payOverride != nil {
		return s.payOverride
	}
	if s.payments != nil {
		return s.payments
	}
	return payments.NewMock()
}

func New(pool *pgxpool.Pool, issuer *auth.Issuer, oauth users.OAuthVerifier,
	mailer users.Mailer, sms otp.Sender, signer *storage.Store, blobs avatar.BlobStore) *Server {
	var mBlob *blobstore.MemoryBlobStore
	if b, ok := blobs.(*blobstore.MemoryBlobStore); ok {
		mBlob = b
	} else {
		mBlob = blobstore.NewMemoryBlobStore()
	}
	return &Server{
		pool: pool, issuer: issuer,
		users: users.New(pool), otp: otp.New(pool, sms),
		avatars: avatar.NewService(signer, blobs),
		oauth:   oauth, mailer: mailer,
		payments: payments.NewMock(), signer: signer,
		photoBlobs: mBlob, chatBlobs: mBlob,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	// public
	mux.HandleFunc("POST /auth/signup", s.handleSignup)
	mux.HandleFunc("POST /auth/signin", s.handleSignin)
	mux.HandleFunc("POST /auth/oauth", s.handleOAuth)
	mux.HandleFunc("POST /auth/phone/send", s.handlePhoneSend)
	mux.HandleFunc("POST /auth/password-reset/request", s.handleResetRequest)
	mux.HandleFunc("POST /auth/password-reset/confirm", s.handleResetConfirm)
	mux.HandleFunc("POST /auth/refresh", s.handleRefresh)
	// authed
	mux.Handle("GET /me", s.requireAuth(http.HandlerFunc(s.handleMe)))
	mux.Handle("PATCH /me", s.requireAuth(http.HandlerFunc(s.handlePatchMe)))
	mux.Handle("POST /me/role", s.requireAuth(http.HandlerFunc(s.handleRole)))
	mux.Handle("POST /auth/signout", s.requireAuth(http.HandlerFunc(s.handleSignout)))
	mux.Handle("DELETE /me/account", s.requireAuth(http.HandlerFunc(s.handleDeleteAccount)))
	mux.Handle("POST /auth/phone/verify", s.requireAuth(http.HandlerFunc(s.handlePhoneVerify)))
	mux.Handle("GET /me/push-tokens", s.requireAuth(http.HandlerFunc(s.handleListTokens)))
	mux.Handle("POST /me/push-tokens", s.requireAuth(http.HandlerFunc(s.handleRegisterToken)))
	mux.Handle("DELETE /me/push-tokens", s.requireAuth(http.HandlerFunc(s.handleRemoveToken)))
	mux.Handle("GET /users/{id}", s.requireAuth(http.HandlerFunc(s.handlePublicProfile)))
	mux.Handle("POST /me/avatar/upload-url", s.requireAuth(http.HandlerFunc(s.handleAvatarURL)))
	mux.Handle("POST /me/avatar/complete", s.requireAuth(http.HandlerFunc(s.handleAvatarComplete)))
	mux.Handle("GET /config/public", s.requireAuth(http.HandlerFunc(s.handlePublicConfig)))
	// Admin probe enforces RequireAdmin itself: admin subjects are not
	// necessarily users rows, so the user-loading middleware must not run first.
	mux.Handle("GET /admin/probe", http.HandlerFunc(s.handleAdminProbe))
	s.registerTaskRoutes(mux)
	s.registerDiscoveryRoutes(mux)
	s.registerOfferRoutes(mux)
	s.registerMessagingRoutes(mux)
	s.registerExecutionRoutes(mux)
	s.registerFinanceRoutes(mux)
	s.registerReviewsRoutes(mux)
	s.registerTrustRoutes(mux)
	s.registerAdminRoutes(mux)
	s.registerNotifyRoutes(mux)
	s.registerDocsRoutes(mux)
	return mux
}

// ── helpers ────────────────────────────────────────────────────────────────

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code string, msg string) {
	writeJSON(w, status, apiError{Code: code, Message: msg})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeErr(w, 400, "bad_json", "invalid JSON body")
		return false
	}
	return true
}

func httpStatus(err error) (int, string) {
	switch {
	case errors.Is(err, users.ErrConflict) || errors.Is(err, otp.ErrConflict):
		return 409, "conflict"
	case errors.Is(err, users.ErrUnauthorized) || errors.Is(err, otp.ErrInvalid):
		return 401, "unauthorized"
	case errors.Is(err, users.ErrNotFound):
		return 404, "not_found"
	case errors.Is(err, users.ErrGone):
		return 403, "account_inactive"
	case errors.Is(err, otp.ErrCooldown) || errors.Is(err, otp.ErrRateLimit):
		return 429, "rate_limited"
	case errors.Is(err, otp.ErrLocked):
		return 423, "code_locked"
	case errors.Is(err, otp.ErrExpired):
		return 410, "code_expired"
	default:
		return 400, "bad_request"
	}
}

func currentUser(r *http.Request) *users.User {
	u, _ := r.Context().Value(userKey).(*users.User)
	return u
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i >= 0 {
		host = host[:i]
	}
	return host
}

// issueSession creates a sessions row + bound access JWT + refresh token.
func (s *Server) issueSession(ctx context.Context, userID string) (access, refresh string, err error) {
	sess, rawRefresh, err := sessions.Create(ctx, s.pool, userID)
	if err != nil {
		return "", "", err
	}
	access, err = s.issuer.MintUserSession(userID, sess.ID, 15*time.Minute)
	if err != nil {
		return "", "", err
	}
	return access, rawRefresh, nil
}

// requireAuth verifies Bearer JWT, live session row, active user.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		parts := strings.SplitN(h, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") || parts[1] == "" {
			writeErr(w, 401, "unauthorized", "missing bearer token")
			return
		}
		claims, err := s.issuer.Verify(parts[1])
		if err != nil {
			writeErr(w, 401, "unauthorized", "invalid or expired token")
			return
		}
		if claims.SessionID != "" && !sessions.Alive(r.Context(), s.pool, claims.SessionID, claims.UserID) {
			writeErr(w, 401, "unauthorized", "session revoked")
			return
		}
		u, err := s.users.GetByID(r.Context(), claims.UserID)
		if err != nil {
			writeErr(w, 401, "unauthorized", "unknown user")
			return
		}
		if u.DeletedAt != nil || u.SuspendedAt != nil {
			writeErr(w, 403, "account_inactive", "account deleted or suspended")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, u)))
	})
}

// ── public handlers ────────────────────────────────────────────────────────

func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if !decode(w, r, &in) {
		return
	}
	u, err := s.users.SignupEmail(r.Context(), in.Email, in.Password, in.DisplayName)
	if err != nil {
		st, code := httpStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	access, refresh, err := s.issueSession(r.Context(), u.ID)
	if err != nil {
		writeErr(w, 500, "server_error", "session failed")
		return
	}
	writeJSON(w, 201, map[string]any{"user": safeUser(u), "access_token": access, "refresh_token": refresh})
}

func (s *Server) handleSignin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	u, err := s.users.SigninEmail(r.Context(), in.Email, in.Password)
	if err != nil {
		st, code := httpStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	access, refresh, err := s.issueSession(r.Context(), u.ID)
	if err != nil {
		writeErr(w, 500, "server_error", "session failed")
		return
	}
	writeJSON(w, 200, map[string]any{"user": safeUser(u), "access_token": access, "refresh_token": refresh})
}

func (s *Server) handleOAuth(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Provider    string `json:"provider"`
		IDToken     string `json:"id_token"`
		DisplayName string `json:"display_name"`
	}
	if !decode(w, r, &in) {
		return
	}
	u, isNew, err := s.users.OAuthSignin(r.Context(), s.oauth, in.Provider, in.IDToken, in.DisplayName)
	if err != nil {
		st, code := httpStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	access, refresh, err := s.issueSession(r.Context(), u.ID)
	if err != nil {
		writeErr(w, 500, "server_error", "session failed")
		return
	}
	st := 200
	if isNew {
		st = 201
	}
	writeJSON(w, st, map[string]any{"user": safeUser(u), "access_token": access, "refresh_token": refresh})
}

func (s *Server) handlePhoneSend(w http.ResponseWriter, r *http.Request) {
	var in struct {
		UserID string `json:"user_id"`
		Phone  string `json:"phone"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.UserID == "" {
		writeErr(w, 400, "bad_request", "user_id required")
		return
	}
	if err := s.otp.Send(r.Context(), in.UserID, in.Phone, clientIP(r)); err != nil {
		st, code := httpStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleResetRequest(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &in) {
		return
	}
	_ = s.users.RequestPasswordReset(r.Context(), s.mailer, in.Email)
	writeJSON(w, 200, map[string]any{"ok": true}) // never enumerate
}

func (s *Server) handleResetConfirm(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := s.users.ConfirmPasswordReset(r.Context(), in.Token, in.NewPassword); err != nil {
		st, code := httpStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	if !decode(w, r, &in) {
		return
	}
	sess, err := sessions.ValidateRefresh(r.Context(), s.pool, in.RefreshToken)
	if err != nil {
		writeErr(w, 401, "unauthorized", "invalid refresh token")
		return
	}
	// rotation: revoke presented token, mint a fresh pair
	_ = sessions.Revoke(r.Context(), s.pool, sess.ID, sess.UserID)
	access, refresh, err := s.issueSession(r.Context(), sess.UserID)
	if err != nil {
		writeErr(w, 500, "server_error", "session failed")
		return
	}
	writeJSON(w, 200, map[string]any{"access_token": access, "refresh_token": refresh})
}

// ── authed handlers ────────────────────────────────────────────────────────

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"user": safeUser(currentUser(r))})
}

func (s *Server) handlePatchMe(w http.ResponseWriter, r *http.Request) {
	// Strict allowlist: decode into the patch struct ONLY. Server-owned
	// fields (verification_status, rating_*, suspended_at, tasks_completed,
	// reliability_score, phone, email...) have no binding target here, so
	// even a malicious payload cannot reach them.
	var in users.ProfilePatch
	if !decode(w, r, &in) {
		return
	}
	u, err := s.users.UpdateProfile(r.Context(), currentUser(r).ID, in)
	if err != nil {
		st, code := httpStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"user": safeUser(u)})
}

func (s *Server) handleRole(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RoleIntent string `json:"role_intent"`
	}
	if !decode(w, r, &in) {
		return
	}
	u, err := s.users.SetRoleIntent(r.Context(), currentUser(r).ID, in.RoleIntent)
	if err != nil {
		st, code := httpStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"user": safeUser(u)})
}

func (s *Server) handleSignout(w http.ResponseWriter, r *http.Request) {
	h := r.Header.Get("Authorization")
	claims, _ := s.issuer.Verify(strings.SplitN(h, " ", 2)[1])
	if claims != nil && claims.SessionID != "" {
		_ = sessions.Revoke(r.Context(), s.pool, claims.SessionID, claims.UserID)
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Confirm bool `json:"confirm"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := s.users.DeleteAccount(r.Context(), currentUser(r).ID, in.Confirm); err != nil {
		st, code := httpStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handlePhoneVerify(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Phone string `json:"phone"`
		Code  string `json:"code"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := s.otp.Verify(r.Context(), currentUser(r).ID, in.Phone, in.Code); err != nil {
		st, code := httpStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	u, _ := s.users.GetByID(r.Context(), currentUser(r).ID)
	writeJSON(w, 200, map[string]any{"user": safeUser(u)})
}

func (s *Server) handleRegisterToken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token    string `json:"token"`
		Platform string `json:"platform"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := pushtokens.Register(r.Context(), s.pool, currentUser(r).ID, in.Token, in.Platform); err != nil {
		writeErr(w, 400, "bad_request", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleRemoveToken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token string `json:"token"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := pushtokens.Remove(r.Context(), s.pool, currentUser(r).ID, in.Token); err != nil {
		writeErr(w, 400, "bad_request", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleListTokens(w http.ResponseWriter, r *http.Request) {
	toks, err := pushtokens.List(r.Context(), s.pool, currentUser(r).ID)
	if err != nil {
		writeErr(w, 500, "server_error", "list failed")
		return
	}
	if toks == nil {
		toks = []string{}
	}
	writeJSON(w, 200, map[string]any{"tokens": toks})
}

func (s *Server) handlePublicProfile(w http.ResponseWriter, r *http.Request) {
	p, err := s.users.GetPublicProfile(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, 404, "not_found", "user not found")
		return
	}
	writeJSON(w, 200, map[string]any{"profile": p})
}

func (s *Server) handleAvatarURL(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ContentType string `json:"content_type"`
	}
	if !decode(w, r, &in) {
		return
	}
	grant, err := s.avatars.GrantUpload(currentUser(r).ID, in.ContentType)
	if err != nil {
		writeErr(w, 400, "bad_request", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{
		"object": grant.Object, "expires_at": grant.ExpiresAt,
		"signature": grant.Signature.Signature,
	})
}

func (s *Server) handleAvatarComplete(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Object    string `json:"object"`
		Signature string `json:"signature"`
		ExpiresAt string `json:"expires_at"`
		ImageB64  string `json:"image_base64"`
	}
	if !decode(w, r, &in) {
		return
	}
	exp, err := time.Parse(time.RFC3339, in.ExpiresAt)
	if err != nil {
		writeErr(w, 400, "bad_request", "bad expires_at")
		return
	}
	data, err := base64.StdEncoding.DecodeString(in.ImageB64)
	if err != nil {
		writeErr(w, 400, "bad_request", "bad image_base64")
		return
	}
	key, err := s.avatars.Complete(
		avatar.UploadGrant{Object: in.Object,
			Signature: storage.SignedURL{Bucket: storage.BucketAvatars, Object: in.Object, ExpiresAt: exp, Signature: in.Signature}},
		data)
	if err != nil {
		writeErr(w, 400, "bad_request", err.Error())
		return
	}
	u, err := s.users.UpdateProfile(r.Context(), currentUser(r).ID, users.ProfilePatch{AvatarURL: &key})
	if err != nil {
		writeErr(w, 400, "bad_request", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"user": safeUser(u)})
}

// Non-sensitive config only — documents the §3 decision.
var publicConfigKeys = []string{
	"fee_percent", "fee_payer", "min_budget", "max_budget",
	"auto_release_hours", "chat_freeze_days", "review_publish_days",
	"locale", "currency",
}

func (s *Server) handlePublicConfig(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{}
	for _, k := range publicConfigKeys {
		var v string
		if err := s.pool.QueryRow(r.Context(), `SELECT value::text FROM app_config WHERE key=$1`, k).Scan(&v); err == nil {
			out[k] = v
		}
	}
	writeJSON(w, 200, map[string]any{"config": out})
}

// Admin probe: proves non-admin tokens are rejected on gated endpoints.
func (s *Server) handleAdminProbe(w http.ResponseWriter, r *http.Request) {
	h := r.Header.Get("Authorization")
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 {
		writeErr(w, 401, "unauthorized", "missing bearer token")
		return
	}
	if _, err := s.issuer.RequireAdmin(parts[1]); err != nil {
		writeErr(w, 403, "forbidden", "admin required")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// safeUser is the owner-view serializer. password_hash is NEVER included;
// phone/email are included ONLY for the owner (public endpoint uses the
// projection instead).
func safeUser(u *users.User) map[string]any {
	return map[string]any{
		"id": u.ID, "auth_provider": u.AuthProvider, "email": u.Email,
		"phone": u.Phone, "phone_verified": u.PhoneVerified,
		"display_name": u.DisplayName, "avatar_url": u.AvatarURL,
		"bio": u.Bio, "location": u.Location,
		"latitude": u.Latitude, "longitude": u.Longitude,
		"role_intent": u.RoleIntent, "verification_status": u.VerificationStatus,
		"rating_avg": u.RatingAvg, "rating_count": u.RatingCount,
		"tasks_completed": u.TasksCompleted, "onboarding_completed": u.OnboardingCompleted,
		"created_at": u.CreatedAt,
	}
}
