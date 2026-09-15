// Package httpapi — Phase 6 execution handlers.
package httpapi

import (
	"net/http"

	"github.com/sidekick/backend/internal/execution"
)

func (s *Server) registerExecutionRoutes(mux *http.ServeMux) {
	mux.Handle("POST /tasks/{id}/complete", s.requireAuth(http.HandlerFunc(s.handleMarkComplete)))
	mux.Handle("POST /tasks/{id}/confirm", s.requireAuth(http.HandlerFunc(s.handleConfirmCompletion)))
	mux.Handle("POST /tasks/{id}/dispute", s.requireAuth(http.HandlerFunc(s.handleRaiseDispute)))
	mux.Handle("POST /tasks/{id}/cancel", s.requireAuth(http.HandlerFunc(s.handleCancelTaskPhase6)))
	// internal cron triggers for tests (also scalar testable)
	mux.Handle("POST /internal/cron/auto-release", s.requireAuth(http.HandlerFunc(s.handleAutoReleaseCron)))
	mux.Handle("POST /internal/cron/auto-release-warning", s.requireAuth(http.HandlerFunc(s.handleAutoReleaseWarningCron)))
}

func execSvc(s *Server) *execution.Service {
	return execution.New(s.pool)
}

func (s *Server) handleMarkComplete(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PhotoURLs []string `json:"photo_urls"`
	}
	// body may be empty (no photos)
	_ = decodeOptional(w, r, &in)
	taskID := r.PathValue("id")
	userID := currentUser(r).ID
	err := execSvc(s).MarkComplete(r.Context(), userID, taskID, in.PhotoURLs)
	if err != nil {
		st, code := executionHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleConfirmCompletion(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	userID := currentUser(r).ID
	err := execSvc(s).Confirm(r.Context(), userID, taskID)
	if err != nil {
		st, code := executionHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleRaiseDispute(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Reason      string `json:"reason"`
		Description string `json:"description"`
	}
	if !decode(w, r, &in) {
		return
	}
	taskID := r.PathValue("id")
	userID := currentUser(r).ID
	disputeID, err := execSvc(s).RaiseDispute(r.Context(), userID, taskID, in.Reason, in.Description)
	if err != nil {
		st, code := executionHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"dispute_id": disputeID})
}

func (s *Server) handleCancelTaskPhase6(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Reason string `json:"reason"`
	}
	_ = decodeOptional(w, r, &in)
	taskID := r.PathValue("id")
	userID := currentUser(r).ID
	// Use execution service which handles both open and assigned; fallback to tasks service for draft?
	err := execSvc(s).Cancel(r.Context(), userID, taskID, in.Reason)
	if err != nil {
		st, code := executionHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleAutoReleaseCron(w http.ResponseWriter, r *http.Request) {
	count, err := execSvc(s).AutoReleaseWorker(r.Context())
	if err != nil {
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"released": count})
}

func (s *Server) handleAutoReleaseWarningCron(w http.ResponseWriter, r *http.Request) {
	ids, err := execSvc(s).AutoReleaseWarning(r.Context())
	if err != nil {
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"warnings": ids})
}

func executionHTTPStatus(err error) (int, string) {
	switch {
	case isErr(err, execution.ErrNotFound):
		return 404, "not_found"
	case isErr(err, execution.ErrForbidden):
		return 403, "forbidden"
	case isErr(err, execution.ErrConflict), isErr(err, execution.ErrGone):
		return 409, "conflict"
	default:
		return 400, "bad_request"
	}
}

// decodeOptional is like decode but allows empty body.
func decodeOptional(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.ContentLength == 0 {
		return true
	}
	return decode(w, r, v)
}
