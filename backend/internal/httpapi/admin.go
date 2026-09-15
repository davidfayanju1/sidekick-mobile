// Package httpapi — Phase 11 admin console HTTP handlers.
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/sidekick/backend/internal/admin"
)

func (s *Server) registerAdminRoutes(mux *http.ServeMux) {
	// User management
	mux.Handle("POST /admin/users/{id}/suspend", s.requireAuth(http.HandlerFunc(s.handleSuspendUser)))
	mux.Handle("POST /admin/users/{id}/ban", s.requireAuth(http.HandlerFunc(s.handleBanUser)))
	mux.Handle("POST /admin/users/{id}/reinstate", s.requireAuth(http.HandlerFunc(s.handleReinstateUser)))

	// Report action
	mux.Handle("POST /admin/reports/{id}/action", s.requireAuth(http.HandlerFunc(s.handleReportAction)))

	// Payout batches
	mux.Handle("POST /admin/payouts/batch", s.requireAuth(http.HandlerFunc(s.handleCreateBatch)))
	mux.Handle("POST /admin/payouts/batch/{id}/confirm", s.requireAuth(http.HandlerFunc(s.handleConfirmBatch)))
	mux.Handle("GET /admin/payouts/batches", s.requireAuth(http.HandlerFunc(s.handleListBatches)))
	mux.Handle("GET /admin/payouts/batch/{id}", s.requireAuth(http.HandlerFunc(s.handleGetBatch)))

	// Admin search
	mux.Handle("GET /admin/search", s.requireAuth(http.HandlerFunc(s.handleAdminSearch)))

	// Enhanced verification queue
	mux.Handle("GET /admin/verifications", s.requireAuth(http.HandlerFunc(s.handleListVerificationsEnhanced)))

	// Enhanced disputes queue
	mux.Handle("GET /admin/disputes", s.requireAuth(http.HandlerFunc(s.handleListDisputesEnhanced)))
}

func adminSvc(s *Server) *admin.Service {
	return admin.New(s.pool)
}

// ── User management ────────────────────────────────────────────────────────

func (s *Server) handleSuspendUser(w http.ResponseWriter, r *http.Request) {
	adminClaims, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	adminID := adminClaims.UserID
	userID := r.PathValue("id")
	var in struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	if err := adminSvc(s).SuspendUser(r.Context(), adminID, userID, in.Reason); err != nil {
		st, code := adminHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleBanUser(w http.ResponseWriter, r *http.Request) {
	adminClaims, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	adminID := adminClaims.UserID
	userID := r.PathValue("id")
	var in struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	if err := adminSvc(s).BanUser(r.Context(), adminID, userID, in.Reason); err != nil {
		st, code := adminHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleReinstateUser(w http.ResponseWriter, r *http.Request) {
	adminClaims, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	adminID := adminClaims.UserID
	userID := r.PathValue("id")
	if err := adminSvc(s).ReinstateUser(r.Context(), adminID, userID); err != nil {
		st, code := adminHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// ── Report action ──────────────────────────────────────────────────────────

func (s *Server) handleReportAction(w http.ResponseWriter, r *http.Request) {
	adminClaims, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	adminID := adminClaims.UserID
	reportID := r.PathValue("id")
	var in struct {
		Action string `json:"action"`
		Note   string `json:"note"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	if err := adminSvc(s).ActionReport(r.Context(), adminID, reportID, in.Action, in.Note); err != nil {
		st, code := adminHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// ── Payout batches ─────────────────────────────────────────────────────────

func (s *Server) handleCreateBatch(w http.ResponseWriter, r *http.Request) {
	adminClaims, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	adminID := adminClaims.UserID
	var in struct {
		Date string `json:"date"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	batch, err := adminSvc(s).CreateBatch(r.Context(), adminID, in.Date)
	if err != nil {
		st, code := adminHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"batch": batch})
}

func (s *Server) handleConfirmBatch(w http.ResponseWriter, r *http.Request) {
	adminClaims, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	adminID := adminClaims.UserID
	batchID := r.PathValue("id")
	if err := adminSvc(s).ConfirmBatch(r.Context(), adminID, batchID); err != nil {
		st, code := adminHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleListBatches(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	batches, err := adminSvc(s).ListBatches(r.Context())
	if err != nil {
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"batches": batches})
}

func (s *Server) handleGetBatch(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	batchID := r.PathValue("id")
	batch, err := adminSvc(s).GetBatch(r.Context(), batchID)
	if err != nil {
		st, code := adminHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"batch": batch})
}

// ── Admin search ───────────────────────────────────────────────────────────

func (s *Server) handleAdminSearch(w http.ResponseWriter, r *http.Request) {
	adminClaims, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	adminID := adminClaims.UserID
	query := r.URL.Query().Get("q")
	typ := r.URL.Query().Get("type")
	result, err := adminSvc(s).Search(r.Context(), adminID, query, typ)
	if err != nil {
		st, code := adminHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, result)
}

// ── Enhanced verification queue ────────────────────────────────────────────

func (s *Server) handleListVerificationsEnhanced(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	status := r.URL.Query().Get("status")
	list, err := adminSvc(s).ListVerificationsEnhanced(r.Context(), status)
	if err != nil {
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"verifications": list})
}

// ── Enhanced disputes queue ────────────────────────────────────────────────

func (s *Server) handleListDisputesEnhanced(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	status := r.URL.Query().Get("status")
	list, err := adminSvc(s).ListDisputesEnhanced(r.Context(), status)
	if err != nil {
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"disputes": list})
}

// ── Error mapping ──────────────────────────────────────────────────────────

func adminHTTPStatus(err error) (int, string) {
	switch {
	case errors.Is(err, admin.ErrNotFound):
		return 404, "not_found"
	case errors.Is(err, admin.ErrForbidden):
		return 403, "forbidden"
	case errors.Is(err, admin.ErrConflict) || errors.Is(err, admin.ErrAlreadyBanned) || errors.Is(err, admin.ErrNotSuspended) || errors.Is(err, admin.ErrNotBanned):
		return 409, "conflict"
	case errors.Is(err, admin.ErrSelfAction):
		return 400, "bad_request"
	case errors.Is(err, admin.ErrBadRequest):
		return 400, "bad_request"
	default:
		return 500, "server_error"
	}
}
