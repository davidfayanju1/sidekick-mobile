package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/sidekick/backend/internal/reviews"
)

func decodeRaw(w http.ResponseWriter, r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeErr(w, 400, "bad_json", "invalid JSON body")
		return err
	}
	return nil
}

func (s *Server) registerReviewsRoutes(mux *http.ServeMux) {
	mux.Handle("POST /tasks/{id}/reviews", s.requireAuth(http.HandlerFunc(s.handleSubmitReview)))
	mux.Handle("GET /users/{id}/reviews", s.requireAuth(http.HandlerFunc(s.handleListPublicReviews)))
	mux.Handle("GET /me/reviews", s.requireAuth(http.HandlerFunc(s.handleListOwnReviews)))
	mux.Handle("POST /internal/cron/publish-reviews", s.requireAuth(http.HandlerFunc(s.handlePublishCron)))
}

func reviewSvc(s *Server) *reviews.Service {
	return reviews.New(s.pool)
}

func (s *Server) handleSubmitReview(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	// Reject is_published/published_at if client tries to set them
	var raw map[string]any
	if err := decodeRaw(w, r, &raw); err != nil {
		return
	}
	if _, ok := raw["is_published"]; ok {
		writeErr(w, 400, "bad_request", "is_published cannot be set by client")
		return
	}
	if _, ok := raw["published_at"]; ok {
		writeErr(w, 400, "bad_request", "published_at cannot be set by client")
		return
	}
	var in struct {
		Rating int     `json:"rating"`
		Body   *string `json:"body"`
	}
	// Re-marshal raw to struct (or decode again)
	b, _ := json.Marshal(raw)
	_ = json.Unmarshal(b, &in)
	rev, err := reviewSvc(s).Submit(r.Context(), currentUser(r).ID, taskID, in.Rating, in.Body)
	if err != nil {
		status, code := reviewHTTPStatus(err)
		writeErr(w, status, code, err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"review": rev})
}

func (s *Server) handleListPublicReviews(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	list, err := reviewSvc(s).ListPublic(r.Context(), userID, 20)
	if err != nil {
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"reviews": list})
}

func (s *Server) handleListOwnReviews(w http.ResponseWriter, r *http.Request) {
	list, err := reviewSvc(s).ListOwn(r.Context(), currentUser(r).ID)
	if err != nil {
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"reviews": list})
}

func (s *Server) handlePublishCron(w http.ResponseWriter, r *http.Request) {
	n, err := reviewSvc(s).PublishCron(r.Context())
	if err != nil {
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"published": n})
}

func reviewHTTPStatus(err error) (int, string) {
	if err == nil {
		return 200, "ok"
	}
	msg := err.Error()
	switch {
	case isErr(err, reviews.ErrNotFound):
		return 404, "not_found"
	case isErr(err, reviews.ErrForbidden):
		return 403, "forbidden"
	case isErr(err, reviews.ErrConflict):
		return 409, "conflict"
	case isErr(err, reviews.ErrBadRequest):
		return 400, "bad_request"
	default:
		if strings.Contains(msg, "not found") {
			return 404, "not_found"
		}
		if strings.Contains(msg, "forbidden") {
			return 403, "forbidden"
		}
		if strings.Contains(msg, "already reviewed") || strings.Contains(msg, "conflict") {
			return 409, "conflict"
		}
		return 400, "bad_request"
	}
}
