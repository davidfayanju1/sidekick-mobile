// Package httpapi — Phase 10 notification endpoints.
package httpapi

import (
	"net/http"
	"strconv"

	"github.com/sidekick/backend/internal/notify"
)

func (s *Server) registerNotifyRoutes(mux *http.ServeMux) {
	mux.Handle("GET /me/notifications", s.requireAuth(http.HandlerFunc(s.handleListNotifications)))
	mux.Handle("GET /me/notifications/unread-count", s.requireAuth(http.HandlerFunc(s.handleUnreadCount)))
	mux.Handle("GET /me/notification-prefs", s.requireAuth(http.HandlerFunc(s.handleGetNotificationPrefs)))
	mux.Handle("POST /me/notifications/read-all", s.requireAuth(http.HandlerFunc(s.handleMarkAllRead)))
	mux.Handle("POST /me/notifications/{id}/read", s.requireAuth(http.HandlerFunc(s.handleMarkNotifRead)))
	mux.Handle("PATCH /me/notification-prefs", s.requireAuth(http.HandlerFunc(s.handleNotificationPrefs)))
	mux.Handle("POST /me/push-primer-seen", s.requireAuth(http.HandlerFunc(s.handlePushPrimerSeen)))
	mux.Handle("POST /me/push-permission", s.requireAuth(http.HandlerFunc(s.handlePushPermission)))
}

func notifySvc(s *Server) *notify.Service {
	return notify.New(s.pool)
}

func (s *Server) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	cursor := r.URL.Query().Get("cursor")
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	list, _, err := notifySvc(s).List(r.Context(), currentUser(r).ID, cursor, limit)
	if err != nil {
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"notifications": list})
}

func (s *Server) handleMarkNotifRead(w http.ResponseWriter, r *http.Request) {
	notifID := r.PathValue("id")
	if err := notifySvc(s).MarkRead(r.Context(), currentUser(r).ID, notifID); err != nil {
		writeErr(w, 404, "not_found", "notification not found")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleMarkAllRead(w http.ResponseWriter, r *http.Request) {
	if err := notifySvc(s).MarkAllRead(r.Context(), currentUser(r).ID); err != nil {
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleUnreadCount(w http.ResponseWriter, r *http.Request) {
	cnt, err := notifySvc(s).UnreadCount(r.Context(), currentUser(r).ID)
	if err != nil {
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"unread_count": cnt})
}

func (s *Server) handleNotificationPrefs(w http.ResponseWriter, r *http.Request) {
	var in map[string]any
	if !decode(w, r, &in) {
		return
	}
	for key, val := range in {
		enabled, ok := val.(bool)
		if !ok {
			continue
		}
		if err := notifySvc(s).SetPreference(r.Context(), currentUser(r).ID, key, enabled); err != nil {
			writeErr(w, 400, "bad_request", err.Error())
			return
		}
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleGetNotificationPrefs(w http.ResponseWriter, r *http.Request) {
	prefs, err := notifySvc(s).GetPreferences(r.Context(), currentUser(r).ID)
	if err != nil {
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"preferences": prefs})
}

func (s *Server) handlePushPrimerSeen(w http.ResponseWriter, r *http.Request) {
	if err := notifySvc(s).MarkPushPrimerSeen(r.Context(), currentUser(r).ID); err != nil {
		writeErr(w, 500, "server_error", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handlePushPermission(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Permission string `json:"permission"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := notifySvc(s).SetPushPermission(r.Context(), currentUser(r).ID, in.Permission); err != nil {
		writeErr(w, 400, "bad_request", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
