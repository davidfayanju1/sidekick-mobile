// Phase 4 routes: offers & matching (atomic accept with 9 steps, ranked list).
package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/sidekick/backend/internal/offers"
)

func (s *Server) registerOfferRoutes(mux *http.ServeMux) {
	mux.Handle("POST /tasks/{id}/offers", s.requireAuth(http.HandlerFunc(s.handleCreateOffer)))
	mux.Handle("GET /tasks/{id}/offers", s.requireAuth(http.HandlerFunc(s.handleListOffers)))
	mux.Handle("GET /me/offers", s.requireAuth(http.HandlerFunc(s.handleMyOffers)))
	mux.Handle("POST /offers/{id}/decline", s.requireAuth(http.HandlerFunc(s.handleDeclineOffer)))
	mux.Handle("POST /offers/{id}/counter", s.requireAuth(http.HandlerFunc(s.handleCounterOffer)))
	mux.Handle("POST /offers/{id}/respond", s.requireAuth(http.HandlerFunc(s.handleRespondCounter)))
	mux.Handle("POST /offers/{id}/accept", s.requireAuth(http.HandlerFunc(s.handleAcceptOffer)))
	mux.Handle("POST /offers/{id}/withdraw", s.requireAuth(http.HandlerFunc(s.handleWithdrawOffer)))
	// Conversation read for Phase 4 verification (exact-location via system message).
	mux.Handle("GET /tasks/{id}/conversation", s.requireAuth(http.HandlerFunc(s.handleGetConversation)))
}

func offerSvc(s *Server) *offers.Service { return offers.New(s.pool) }

func offerHTTPStatus(err error) (int, string) {
	switch {
	case isErr(err, offers.ErrNotFound):
		return 404, "not_found"
	case isErr(err, offers.ErrForbidden):
		return 403, "forbidden"
	case isErr(err, offers.ErrGone):
		return 409, "no_longer_available"
	case isErr(err, offers.ErrConflict):
		return 409, "conflict"
	default:
		return 400, "bad_request"
	}
}

func (s *Server) handleCreateOffer(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Amount  *int   `json:"amount"`
		Message string `json:"message"`
	}
	if !decode(w, r, &in) {
		return
	}
	taskID := r.PathValue("id")
	// Default amount to task budget if not provided; fetch budget to determine.
	var budget int
	err := s.pool.QueryRow(r.Context(), `SELECT budget FROM tasks WHERE id=$1::uuid`, taskID).Scan(&budget)
	if err != nil {
		writeErr(w, 404, "not_found", "task not found")
		return
	}
	amount := budget
	if in.Amount != nil {
		amount = *in.Amount
	}
	o, err := offerSvc(s).Create(r.Context(), currentUser(r).ID, taskID, amount, in.Message)
	if err != nil {
		st, code := offerHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"offer": o})
}

func (s *Server) handleListOffers(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	list, err := offerSvc(s).ListForTask(r.Context(), currentUser(r).ID, taskID)
	if err != nil {
		st, code := offerHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"offers": list})
}

func (s *Server) handleMyOffers(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status != "" {
		valid := map[string]bool{"pending": true, "accepted": true, "declined": true, "withdrawn": true, "auto_declined": true, "expired": true}
		if !valid[status] {
			writeErr(w, 400, "bad_request", "invalid status filter")
			return
		}
	}
	list, err := offerSvc(s).ListMyOffers(r.Context(), currentUser(r).ID, status)
	if err != nil {
		writeErr(w, 400, "bad_request", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"offers": list})
}

func (s *Server) handleDeclineOffer(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Reason string `json:"reason"`
	}
	// reason is optional, body may be empty
	_ = json.NewDecoder(r.Body).Decode(&in)
	offerID := r.PathValue("id")
	o, err := offerSvc(s).Decline(r.Context(), currentUser(r).ID, offerID, in.Reason)
	if err != nil {
		st, code := offerHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"offer": o})
}

func (s *Server) handleCounterOffer(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Amount int `json:"amount"`
	}
	if !decode(w, r, &in) {
		return
	}
	offerID := r.PathValue("id")
	o, err := offerSvc(s).Counter(r.Context(), currentUser(r).ID, offerID, in.Amount)
	if err != nil {
		st, code := offerHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"offer": o})
}

func (s *Server) handleRespondCounter(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Action string `json:"action"`
	}
	if !decode(w, r, &in) {
		return
	}
	offerID := r.PathValue("id")
	in.Action = strings.ToLower(strings.TrimSpace(in.Action))
	o, err := offerSvc(s).RespondToCounter(r.Context(), currentUser(r).ID, offerID, in.Action)
	if err != nil {
		st, code := offerHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"offer": o})
}

func (s *Server) handleAcceptOffer(w http.ResponseWriter, r *http.Request) {
	// Idempotency key: body or header Idempotency-Key
	var in struct {
		IdempotencyKey string `json:"idempotency_key"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	key := in.IdempotencyKey
	if key == "" {
		key = r.Header.Get("Idempotency-Key")
	}
	offerID := r.PathValue("id")
	res, err := offerSvc(s).Accept(r.Context(), currentUser(r).ID, offerID, key)
	if err != nil {
		st, code := offerHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"accept": res})
}

func (s *Server) handleWithdrawOffer(w http.ResponseWriter, r *http.Request) {
	offerID := r.PathValue("id")
	o, err := offerSvc(s).Withdraw(r.Context(), currentUser(r).ID, offerID)
	if err != nil {
		st, code := offerHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"offer": o})
}

func (s *Server) handleGetConversation(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	userID := currentUser(r).ID
	var id, posterID, workerID, tID string
	err := s.pool.QueryRow(r.Context(),
		`SELECT id::text, poster_id::text, worker_id::text, task_id::text FROM conversations WHERE task_id=$1::uuid`, taskID).Scan(&id, &posterID, &workerID, &tID)
	if err != nil {
		writeErr(w, 404, "not_found", "conversation not found")
		return
	}
	if userID != posterID && userID != workerID {
		writeErr(w, 403, "forbidden", "not a participant")
		return
	}
	writeJSON(w, 200, map[string]any{"conversation": map[string]any{"id": id, "task_id": tID, "poster_id": posterID, "worker_id": workerID}})
}

func (s *Server) handleListMessages(w http.ResponseWriter, r *http.Request) {
	convID := r.PathValue("id")
	userID := currentUser(r).ID
	var posterID, workerID string
	err := s.pool.QueryRow(r.Context(), `SELECT poster_id::text, worker_id::text FROM conversations WHERE id=$1::uuid`, convID).Scan(&posterID, &workerID)
	if err != nil {
		writeErr(w, 404, "not_found", "conversation not found")
		return
	}
	if userID != posterID && userID != workerID {
		writeErr(w, 403, "forbidden", "not a participant")
		return
	}
	rows, err := s.pool.Query(r.Context(),
		`SELECT id::text, conversation_id::text, sender_id::text, type, body, system_event, attachment_url, created_at
		 FROM messages WHERE conversation_id=$1::uuid ORDER BY created_at`, convID)
	if err != nil {
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	defer rows.Close()
	type msg struct {
		ID             string  `json:"id"`
		ConversationID string  `json:"conversation_id"`
		SenderID       *string `json:"sender_id"`
		Type           string  `json:"type"`
		Body           string  `json:"body"`
		SystemEvent    *string `json:"system_event"`
		AttachmentURL  *string `json:"attachment_url"`
		CreatedAt      string  `json:"created_at"`
	}
	var out []msg
	for rows.Next() {
		var m msg
		var sender *string
		var created time.Time
		if err := rows.Scan(&m.ID, &m.ConversationID, &sender, &m.Type, &m.Body, &m.SystemEvent, &m.AttachmentURL, &created); err != nil {
			writeErr(w, 500, "server_error", err.Error())
			return
		}
		m.SenderID = sender
		m.CreatedAt = created.Format(time.RFC3339Nano)
		out = append(out, m)
	}
	if out == nil {
		out = []msg{}
	}
	writeJSON(w, 200, map[string]any{"messages": out})
}
