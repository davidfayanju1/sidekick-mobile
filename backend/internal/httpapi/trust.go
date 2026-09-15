// Package httpapi — Phase 9 trust & safety handlers.
package httpapi

import (
	"net/http"
	"strings"

	"github.com/sidekick/backend/internal/auth"
	"github.com/sidekick/backend/internal/trust"
)

func (s *Server) registerTrustRoutes(mux *http.ServeMux) {
	// Verifications — user
	mux.Handle("POST /me/verifications", s.requireAuth(http.HandlerFunc(s.handleSubmitVerification)))
	// Verifications — admin (approve/reject only; listing is in admin.go)
	mux.Handle("POST /admin/verifications/{id}/approve", s.requireAuth(http.HandlerFunc(s.handleApproveVerification)))
	mux.Handle("POST /admin/verifications/{id}/reject", s.requireAuth(http.HandlerFunc(s.handleRejectVerification)))
	// Reports — user
	mux.Handle("POST /reports", s.requireAuth(http.HandlerFunc(s.handleSubmitReport)))
	mux.Handle("GET /me/reports", s.requireAuth(http.HandlerFunc(s.handleListOwnReports)))
	// Reports — admin (listing + status update)
	mux.Handle("GET /admin/reports", s.requireAuth(http.HandlerFunc(s.handleListOpenReports)))
	mux.Handle("PATCH /admin/reports/{id}", s.requireAuth(http.HandlerFunc(s.handleUpdateReportStatus)))
	// Disputes — evidence (user, party only)
	mux.Handle("POST /disputes/{id}/evidence", s.requireAuth(http.HandlerFunc(s.handleAddEvidence)))
	// Disputes — admin (triage/resolve only; listing is in admin.go)
	mux.Handle("PATCH /admin/disputes/{id}/triage", s.requireAuth(http.HandlerFunc(s.handleTriageDispute)))
	mux.Handle("POST /admin/disputes/{id}/resolve", s.requireAuth(http.HandlerFunc(s.handleResolveDispute)))
}

func trustSvc(s *Server) *trust.Service {
	return trust.NewWithPayments(s.pool, s.activePayments())
}

// ── Verification handlers ──────────────────────────────────────────────────

func (s *Server) handleSubmitVerification(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DocumentType string `json:"document_type"`
		DocumentURL  string `json:"document_url"`
	}
	if !decode(w, r, &in) {
		return
	}
	id, err := trustSvc(s).SubmitVerification(r.Context(), currentUser(r).ID, in.DocumentType, in.DocumentURL)
	if err != nil {
		st, code := trustHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"verification_id": id})
}

// requireAdmin extracts the Bearer token from the Authorization header
// and verifies the admin claim. Returns claims on success, error on failure.
func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	h := r.Header.Get("Authorization")
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") || parts[1] == "" {
		writeErr(w, 401, "unauthorized", "missing bearer token")
		return nil, false
	}
	claims, err := s.issuer.RequireAdmin(parts[1])
	if err != nil {
		writeErr(w, 403, "forbidden", "admin required")
		return nil, false
	}
	return claims, true
}

func (s *Server) handleListPendingVerifications(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	list, err := trustSvc(s).ListPendingVerifications(r.Context())
	if err != nil {
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"verifications": list})
}

func (s *Server) handleApproveVerification(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	if err := trustSvc(s).ApproveVerification(r.Context(), claims.UserID, r.PathValue("id")); err != nil {
		st, code := trustHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleRejectVerification(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	var in struct {
		Reason string `json:"reason"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := trustSvc(s).RejectVerification(r.Context(), claims.UserID, r.PathValue("id"), in.Reason); err != nil {
		st, code := trustHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// ── Report handlers ────────────────────────────────────────────────────────

func (s *Server) handleSubmitReport(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ReportedUserID string  `json:"reported_user_id"`
		TaskID         *string `json:"task_id"`
		ConversationID *string `json:"conversation_id"`
		Reason         string  `json:"reason"`
		Detail         string  `json:"detail"`
	}
	if !decode(w, r, &in) {
		return
	}
	id, err := trustSvc(s).CreateReport(r.Context(), currentUser(r).ID, in.ReportedUserID,
		in.TaskID, in.ConversationID, in.Reason, in.Detail)
	if err != nil {
		st, code := trustHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"report_id": id})
}

func (s *Server) handleListOwnReports(w http.ResponseWriter, r *http.Request) {
	list, err := trustSvc(s).ListOwnReports(r.Context(), currentUser(r).ID)
	if err != nil {
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"reports": list})
}

func (s *Server) handleListOpenReports(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	list, err := trustSvc(s).ListOpenReports(r.Context())
	if err != nil {
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"reports": list})
}

func (s *Server) handleUpdateReportStatus(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	var in struct {
		Status string `json:"status"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := trustSvc(s).UpdateReportStatus(r.Context(), claims.UserID, r.PathValue("id"), in.Status); err != nil {
		st, code := trustHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// ── Dispute handlers ───────────────────────────────────────────────────────

func (s *Server) handleAddEvidence(w http.ResponseWriter, r *http.Request) {
	var in struct {
		EvidenceURL string `json:"evidence_url"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := trustSvc(s).AddEvidence(r.Context(), currentUser(r).ID, r.PathValue("id"), in.EvidenceURL); err != nil {
		st, code := trustHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleListOpenDisputes(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	list, err := trustSvc(s).ListOpenDisputes(r.Context())
	if err != nil {
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"disputes": list})
}

func (s *Server) handleTriageDispute(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	if err := trustSvc(s).TriageDispute(r.Context(), claims.UserID, r.PathValue("id")); err != nil {
		st, code := trustHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleResolveDispute(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	var in struct {
		Decision           string `json:"decision"`
		CompensationAmount *int   `json:"compensation_amount"`
		ResolutionNotes    string `json:"resolution_notes"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := trustSvc(s).ResolveDispute(r.Context(), claims.UserID, r.PathValue("id"),
		in.Decision, in.ResolutionNotes, in.CompensationAmount); err != nil {
		st, code := trustHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// ── Error mapping ──────────────────────────────────────────────────────────

func trustHTTPStatus(err error) (int, string) {
	if err == nil {
		return 200, "ok"
	}
	msg := err.Error()
	switch {
	case isErr(err, trust.ErrNotFound):
		return 404, "not_found"
	case isErr(err, trust.ErrForbidden):
		return 403, "forbidden"
	case isErr(err, trust.ErrConflict):
		return 409, "conflict"
	case isErr(err, trust.ErrBadRequest):
		return 400, "bad_request"
	default:
		if strings.Contains(msg, "not found") {
			return 404, "not_found"
		}
		if strings.Contains(msg, "forbidden") {
			return 403, "forbidden"
		}
		if strings.Contains(msg, "conflict") {
			return 409, "conflict"
		}
		return 400, "bad_request"
	}
}
