// Package httpapi — Phase 5 messaging, blocks, reports, chat-images.
package httpapi

import (
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/sidekick/backend/internal/blobstore"
	"github.com/sidekick/backend/internal/blocks"
	"github.com/sidekick/backend/internal/messaging"
	"github.com/sidekick/backend/internal/reports"
	"github.com/sidekick/backend/internal/storage"
)

func (s *Server) registerMessagingRoutes(mux *http.ServeMux) {
	mux.Handle("GET /conversations", s.requireAuth(http.HandlerFunc(s.handleListConversations)))
	mux.Handle("GET /conversations/{id}/messages", s.requireAuth(http.HandlerFunc(s.handleListMessagesNew)))
	mux.Handle("POST /conversations/{id}/messages", s.requireAuth(http.HandlerFunc(s.handleSendMessage)))
	mux.Handle("POST /conversations/{id}/images/grant", s.requireAuth(http.HandlerFunc(s.handleGrantChatImage)))
	mux.Handle("POST /conversations/{id}/images", s.requireAuth(http.HandlerFunc(s.handleSendChatImage)))
	mux.Handle("POST /messages/{id}/delivered", s.requireAuth(http.HandlerFunc(s.handleMarkDelivered)))
	mux.Handle("POST /messages/{id}/read", s.requireAuth(http.HandlerFunc(s.handleMarkRead)))
	mux.Handle("GET /chat-images/", s.requireAuth(http.HandlerFunc(s.handleGetChatImage)))
	// Blocks
	mux.Handle("POST /users/{id}/block", s.requireAuth(http.HandlerFunc(s.handleBlockUser)))
	mux.Handle("DELETE /users/{id}/block", s.requireAuth(http.HandlerFunc(s.handleUnblockUser)))
	mux.Handle("GET /blocks", s.requireAuth(http.HandlerFunc(s.handleListBlocks)))
	// Reports
	mux.Handle("POST /conversations/{id}/report", s.requireAuth(http.HandlerFunc(s.handleReportConversation)))
}

func msgSvc(s *Server) *messaging.Service {
	if s.messaging == nil {
		s.messaging = messaging.New(s.pool, s.signer, s.chatBlobs)
	}
	return s.messaging
}

func blockSvc(s *Server) *blocks.Service { return blocks.New(s.pool) }
func reportSvc(s *Server) *reports.Service { return reports.New(s.pool) }

func (s *Server) handleListConversations(w http.ResponseWriter, r *http.Request) {
	list, err := msgSvc(s).ListConversations(r.Context(), currentUser(r).ID)
	if err != nil {
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"conversations": list})
}

func (s *Server) handleListMessagesNew(w http.ResponseWriter, r *http.Request) {
	convID := r.PathValue("id")
	cursor := r.URL.Query().Get("cursor")
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		// ignore parse error -> default
	}
	msgs, next, err := msgSvc(s).ListMessages(r.Context(), currentUser(r).ID, convID, cursor, limit)
	if err != nil {
		st, code := msgHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"messages": msgs, "next_cursor": next})
}

func (s *Server) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Body string `json:"body"`
		Type string `json:"type"`
	}
	if !decode(w, r, &in) {
		return
	}
	// Reject any attempt to send system type directly
	if in.Type == "system" {
		writeErr(w, 403, "forbidden", "cannot send system message")
		return
	}
	if in.Type != "" && in.Type != "text" {
		writeErr(w, 400, "bad_request", "type must be text or omitted")
		return
	}
	convID := r.PathValue("id")
	m, err := msgSvc(s).SendText(r.Context(), currentUser(r).ID, convID, in.Body)
	if err != nil {
		st, code := msgHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	// Phase 9 §2.8: contact-detail leak detection (flag, don't block).
	// Runs after successful send so the message is delivered regardless.
	trustSvc(s).CheckContactLeak(r.Context(), currentUser(r).ID, m.ID, in.Body)
	writeJSON(w, 201, map[string]any{"message": m})
}

func (s *Server) handleGrantChatImage(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ContentType string `json:"content_type"`
	}
	if !decode(w, r, &in) {
		return
	}
	convID := r.PathValue("id")
	if err := msgSvc(s).CheckParticipant(r.Context(), currentUser(r).ID, convID); err != nil {
		st, code := msgHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	signed, err := msgSvc(s).GrantImageUpload(convID, in.ContentType)
	if err != nil {
		writeErr(w, 400, "bad_request", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{
		"object": signed.Object, "expires_at": signed.ExpiresAt.Format(time.RFC3339), "signature": signed.Signature, "bucket": string(signed.Bucket),
	})
}

func (s *Server) handleSendChatImage(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Object    string `json:"object"`
		Signature string `json:"signature"`
		ExpiresAt string `json:"expires_at"`
		ImageBase64 string `json:"image_base64"`
	}
	if !decode(w, r, &in) {
		return
	}
	convID := r.PathValue("id")
	exp, err := time.Parse(time.RFC3339, in.ExpiresAt)
	if err != nil {
		writeErr(w, 400, "bad_request", "invalid expires_at")
		return
	}
	data, err := base64.StdEncoding.DecodeString(in.ImageBase64)
	if err != nil {
		writeErr(w, 400, "bad_request", "invalid image_base64")
		return
	}
	signed := storage.SignedURL{Bucket: storage.BucketChatImages, Object: in.Object, ExpiresAt: exp, Signature: in.Signature}
	m, err := msgSvc(s).SendImage(r.Context(), currentUser(r).ID, convID, signed, data)
	if err != nil {
		st, code := msgHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"message": m})
}

func (s *Server) handleMarkDelivered(w http.ResponseWriter, r *http.Request) {
	msgID := r.PathValue("id")
	err := msgSvc(s).MarkDelivered(r.Context(), currentUser(r).ID, msgID)
	if err != nil {
		st, code := msgHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleMarkRead(w http.ResponseWriter, r *http.Request) {
	msgID := r.PathValue("id")
	err := msgSvc(s).MarkRead(r.Context(), currentUser(r).ID, msgID)
	if err != nil {
		st, code := msgHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleGetChatImage(w http.ResponseWriter, r *http.Request) {
	// Path is /chat-images/{object} where object may contain slashes (convID/filename)
	// Since we registered prefix /chat-images/, extract after prefix.
	prefix := "/chat-images/"
	pathObj := strings.TrimPrefix(r.URL.Path, prefix)
	if idx := strings.Index(pathObj, "?"); idx >= 0 {
		pathObj = pathObj[:idx]
	}
	// Also support ?object= fallback for clients that encode slashes
	if pathObj == "" || pathObj == "object" || !strings.Contains(pathObj, "/") {
		if qobj := r.URL.Query().Get("object"); qobj != "" {
			pathObj = qobj
		}
	}
	object := pathObj
	sig := r.URL.Query().Get("sig")
	expires := r.URL.Query().Get("expires")
	if sig == "" || expires == "" {
		writeErr(w, 403, "forbidden", "signed URL required")
		return
	}
	data, err := msgSvc(s).GetImage(r.Context(), currentUser(r).ID, object, sig, expires)
	if err != nil {
		st, code := msgHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.WriteHeader(200)
	_, _ = w.Write(data)
}

func (s *Server) handleBlockUser(w http.ResponseWriter, r *http.Request) {
	blockedID := r.PathValue("id")
	err := blockSvc(s).Block(r.Context(), currentUser(r).ID, blockedID)
	if err != nil {
		st, code := blockHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleUnblockUser(w http.ResponseWriter, r *http.Request) {
	blockedID := r.PathValue("id")
	err := blockSvc(s).Unblock(r.Context(), currentUser(r).ID, blockedID)
	if err != nil {
		writeErr(w, 400, "bad_request", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleListBlocks(w http.ResponseWriter, r *http.Request) {
	list, err := blockSvc(s).List(r.Context(), currentUser(r).ID)
	if err != nil {
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"blocks": list})
}

func (s *Server) handleReportConversation(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Reason string `json:"reason"`
		Detail string `json:"detail"`
	}
	if !decode(w, r, &in) {
		return
	}
	convID := r.PathValue("id")
	id, err := reportSvc(s).Create(r.Context(), currentUser(r).ID, convID, in.Reason, in.Detail)
	if err != nil {
		st, code := reportHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"report_id": id})
}

func msgHTTPStatus(err error) (int, string) {
	switch {
	case isErr(err, messaging.ErrNotFound):
		return 404, "not_found"
	case isErr(err, messaging.ErrForbidden):
		return 403, "forbidden"
	case isErr(err, messaging.ErrChatFrozen):
		return 403, "chat_frozen"
	case isErr(err, messaging.ErrRateLimited):
		return 429, "rate_limited"
	case isErr(err, messaging.ErrConflict):
		return 409, "conflict"
	default:
		if err != nil && strings.Contains(err.Error(), "chat_frozen") {
			return 403, "chat_frozen"
		}
		if err != nil && strings.Contains(err.Error(), "rate_limited") {
			return 429, "rate_limited"
		}
		return 400, "bad_request"
	}
}

func blockHTTPStatus(err error) (int, string) {
	switch {
	case isErr(err, blocks.ErrNotFound):
		return 404, "not_found"
	case isErr(err, blocks.ErrForbidden):
		return 403, "forbidden"
	default:
		return 400, "bad_request"
	}
}

func reportHTTPStatus(err error) (int, string) {
	switch {
	case isErr(err, reports.ErrNotFound):
		return 404, "not_found"
	case isErr(err, reports.ErrForbidden):
		return 403, "forbidden"
	default:
		return 400, "bad_request"
	}
}

// Ensure imports are used
var _ = blobstore.NewMemoryBlobStore
