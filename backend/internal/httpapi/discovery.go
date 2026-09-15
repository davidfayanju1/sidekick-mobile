// Phase 3 routes: GET /feed and GET /search (both authenticated).
package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/sidekick/backend/internal/discovery"
)

func (s *Server) registerDiscoveryRoutes(mux *http.ServeMux) {
	mux.Handle("GET /feed", s.requireAuth(http.HandlerFunc(s.handleFeed)))
	mux.Handle("GET /search", s.requireAuth(http.HandlerFunc(s.handleSearch)))
}

func discSvc(s *Server) *discovery.Service {
	if s.discovery == nil {
		s.discovery = discovery.New(s.pool)
	}
	return s.discovery
}

func queryFloat(r *http.Request, key string) *float64 {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil
	}
	return &v
}

func queryInt(r *http.Request, key string) *int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return nil
	}
	return &v
}

func (s *Server) handleFeed(w http.ResponseWriter, r *http.Request) {
	lat, lng := queryFloat(r, "lat"), queryFloat(r, "lng")
	if (lat == nil) != (lng == nil) {
		writeErr(w, 400, "bad_request", "lat and lng must be set together")
		return
	}
	var radius float64
	if raw := r.URL.Query().Get("radius_km"); raw != "" {
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || v <= 0 {
			writeErr(w, 400, "bad_request", "radius_km must be a positive number")
			return
		}
		radius = v
	}
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 {
			writeErr(w, 400, "bad_request", "limit must be a positive integer")
			return
		}
		limit = v
	}
	page, err := discSvc(s).Feed(r.Context(), currentUser(r).ID, discovery.Filters{
		Lat: lat, Lng: lng, RadiusKm: radius,
		Category:  r.URL.Query().Get("category"),
		MinBudget: queryInt(r, "min_budget"), MaxBudget: queryInt(r, "max_budget"),
		Timing: r.URL.Query().Get("timing"),
		Cursor: r.URL.Query().Get("cursor"), Limit: limit,
	})
	if err != nil {
		if strings.Contains(err.Error(), discovery.ErrRateLimited.Error()) {
			writeErr(w, 429, "rate_limited", err.Error())
			return
		}
		st, code := taskHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{
		"tasks": page.Cards, "next_cursor": page.NextCursor,
		"radius_km_applied": page.RadiusApplied,
	})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 {
			writeErr(w, 400, "bad_request", "limit must be a positive integer")
			return
		}
		limit = v
	}
	page, err := discSvc(s).Search(r.Context(), currentUser(r).ID, discovery.Filters{
		Query:     r.URL.Query().Get("q"),
		Category:  r.URL.Query().Get("category"),
		MinBudget: queryInt(r, "min_budget"), MaxBudget: queryInt(r, "max_budget"),
		Timing: r.URL.Query().Get("timing"),
		Cursor: r.URL.Query().Get("cursor"), Limit: limit,
	})
	if err != nil {
		if strings.Contains(err.Error(), discovery.ErrRateLimited.Error()) {
			writeErr(w, 429, "rate_limited", err.Error())
			return
		}
		st, code := taskHTTPStatus(err)
		writeErr(w, st, code, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"results": page.Cards, "next_cursor": page.NextCursor})
}
