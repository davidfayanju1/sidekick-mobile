// Phase 2 routes: tasks, drafts, photos, fee quote, fund, escrow, edit,
// cancel. Every read renders through tasks.ProjectTask (the §3.5 choke
// point) — no handler selects location fields itself.
package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sidekick/backend/internal/blobstore"
	"github.com/sidekick/backend/internal/taskphotos"
	"github.com/sidekick/backend/internal/tasks"
)

func (s *Server) registerTaskRoutes(mux *http.ServeMux) {
	mux.Handle("POST /tasks", s.requireAuth(http.HandlerFunc(s.handleTaskCreate)))
	mux.Handle("GET /tasks/{id}", s.requireAuth(http.HandlerFunc(s.handleTaskGet)))
	mux.Handle("PATCH /tasks/{id}", s.requireAuth(http.HandlerFunc(s.handleTaskEdit)))
	mux.Handle("DELETE /tasks/{id}", s.requireAuth(http.HandlerFunc(s.handleTaskCancel)))
	mux.Handle("PUT /tasks/{id}/draft", s.requireAuth(http.HandlerFunc(s.handleDraftSave)))
	mux.Handle("GET /me/drafts", s.requireAuth(http.HandlerFunc(s.handleDraftList)))
	mux.Handle("POST /tasks/{id}/photos/grant", s.requireAuth(http.HandlerFunc(s.handlePhotoGrant)))
	mux.Handle("POST /tasks/{id}/photos/complete", s.requireAuth(http.HandlerFunc(s.handlePhotoComplete)))
	mux.Handle("GET /fees/quote", s.requireAuth(http.HandlerFunc(s.handleFeeQuote)))
	mux.Handle("POST /tasks/{id}/fund", s.requireAuth(http.HandlerFunc(s.handleFund)))
	mux.Handle("GET /tasks/{id}/escrow", s.requireAuth(http.HandlerFunc(s.handleEscrow)))
}

func taskSvc(s *Server) *tasks.Service { return tasks.New(s.pool, s.activePayments()) }

func photoSvc(s *Server) *taskphotos.Service {
	if s.photoBlobs == nil {
		s.photoBlobs = blobstore.NewMemoryBlobStore()
	}
	return taskphotos.NewService(s.signer, s.photoBlobs)
}

func taskHTTPStatus(err error) (int, string) {
	switch {
	case isErr(err, tasks.ErrNotFound):
		return 404, "not_found"
	case isErr(err, tasks.ErrForbidden):
		return 403, "forbidden"
	case isErr(err, tasks.ErrConflict):
		return 409, "conflict"
	case isErr(err, tasks.ErrPaymentFailed):
		return 502, "payment_failed"
	default:
		return 400, "bad_request"
	}
}

func isErr(err, target error) bool {
	return err != nil && (err == target ||
		strings.Contains(err.Error(), target.Error()))
}

func (s *Server) handleTaskCreate(w http.ResponseWriter, r *http.Request) {
	var in tasks.Input
	if !decode(w, r, &in) {
		return
	}
	t, err := taskSvc(s).Create(r.Context(), currentUser(r).ID, in)
	if err != nil {
		st, code := taskHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"task": tasks.ProjectTask(currentUser(r).ID, t, []tasks.Photo{})})
}

func (s *Server) handleTaskGet(w http.ResponseWriter, r *http.Request) {
	t, photos, err := taskSvc(s).GetForReader(r.Context(), currentUser(r).ID, r.PathValue("id"))
	if err != nil {
		st, code := taskHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"task": tasks.ProjectTask(currentUser(r).ID, t, photos)})
}

func (s *Server) handleTaskEdit(w http.ResponseWriter, r *http.Request) {
	var in tasks.Input
	if !decode(w, r, &in) {
		return
	}
	t, err := taskSvc(s).Edit(r.Context(), currentUser(r).ID, r.PathValue("id"), in)
	if err != nil {
		st, code := taskHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	photos, _ := taskSvc(s).ListPhotos(r.Context(), t.ID)
	writeJSON(w, 200, map[string]any{"task": tasks.ProjectTask(currentUser(r).ID, t, photos)})
}

func (s *Server) handleTaskCancel(w http.ResponseWriter, r *http.Request) {
	t, err := taskSvc(s).Cancel(r.Context(), currentUser(r).ID, r.PathValue("id"))
	if err != nil {
		st, code := taskHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	photos, _ := taskSvc(s).ListPhotos(r.Context(), t.ID)
	writeJSON(w, 200, map[string]any{"task": tasks.ProjectTask(currentUser(r).ID, t, photos)})
}

func (s *Server) handleDraftSave(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Draft json.RawMessage `json:"draft"`
	}
	if !decode(w, r, &in) {
		return
	}
	// Contract: draft is a JSON OBJECT (step payload), not a string.
	// Storing a pre-encoded string would double-encode it in JSONB.
	var probe map[string]any
	if len(in.Draft) == 0 || json.Unmarshal(in.Draft, &probe) != nil {
		writeErr(w, 400, "bad_request", "draft must be a JSON object")
		return
	}
	if err := taskSvc(s).SaveDraft(r.Context(), currentUser(r).ID, r.PathValue("id"), string(in.Draft)); err != nil {
		st, code := taskHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleDraftList(w http.ResponseWriter, r *http.Request) {
	list, err := taskSvc(s).ListDrafts(r.Context(), currentUser(r).ID)
	if err != nil {
		writeErr(w, 500, "server_error", "list failed")
		return
	}
	out := make([]any, 0, len(list))
	for _, t := range list {
		out = append(out, tasks.ProjectTask(currentUser(r).ID, t, []tasks.Photo{}))
	}
	writeJSON(w, 200, map[string]any{"drafts": out})
}

func (s *Server) handlePhotoGrant(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ContentType string `json:"content_type"`
	}
	if !decode(w, r, &in) {
		return
	}
	g, err := photoSvc(s).GrantUpload(r.PathValue("id"), in.ContentType)
	if err != nil {
		writeErr(w, 400, "bad_request", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{
		"object": g.Object, "expires_at": g.ExpiresAt, "signature": g.Signature,
	})
}

func (s *Server) handlePhotoComplete(w http.ResponseWriter, r *http.Request) {
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
	key, err := photoSvc(s).Complete(taskphotos.UploadGrant{
		Object: in.Object, ExpiresAt: exp, Signature: in.Signature,
	}, data)
	if err != nil {
		writeErr(w, 400, "bad_request", err.Error())
		return
	}
	p, err := taskSvc(s).AttachPhoto(r.Context(), currentUser(r).ID, r.PathValue("id"), key, "listing")
	if err != nil {
		st, code := taskHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"photo": p})
}

func (s *Server) handleFeeQuote(w http.ResponseWriter, r *http.Request) {
	// Decision (documented): quote requires login. Pre-signup quoting would
	// leak fee tuning to scrapers and the client always quotes post-auth.
	budget, err := strconv.Atoi(r.URL.Query().Get("budget"))
	if err != nil || budget <= 0 {
		writeErr(w, 400, "bad_request", "budget query param required")
		return
	}
	q, err := taskSvc(s).Quote(r.Context(), budget)
	if err != nil {
		st, code := taskHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"quote": q})
}

func (s *Server) handleFund(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PaymentMethodRef string `json:"payment_method_ref"`
		IdempotencyKey   string `json:"idempotency_key"`
	}
	if !decode(w, r, &in) {
		return
	}
	key := in.IdempotencyKey
	if key == "" {
		// Header fallback: Idempotency-Key.
		key = r.Header.Get("Idempotency-Key")
	}
	res, err := taskSvc(s).Fund(r.Context(), currentUser(r).ID, r.PathValue("id"), in.PaymentMethodRef, key)
	if err != nil {
		st, code := taskHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"fund": res})
}

func (s *Server) handleEscrow(w http.ResponseWriter, r *http.Request) {
	v, err := taskSvc(s).EscrowView(r.Context(), currentUser(r).ID, r.PathValue("id"))
	if err != nil {
		st, code := taskHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"escrow": v})
}
