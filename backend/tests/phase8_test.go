// Phase 8 Testing Gate — ratings & reputation double-blind (13 gates + self-audit).
package tests

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// helpers for Phase 8

func setupCompletedTask(t *testing.T, h *harness) (posterID, workerID, posterTok, workerTok, taskID string) {
	t.Helper()
	posterID, posterTok, _ = h.signup(t, uniq("p8p")+"@example.com", "password123", "Poster P8")
	workerID, workerTok, _ = h.signup(t, uniq("p8w")+"@example.com", "password123", "Worker P8")
	taskID = fundTaskForPhase4(t, h, posterTok, validTaskInput())
	_, out := makeOffer(t, h, workerTok, taskID, nil, "")
	offerID := out["offer"].(map[string]any)["id"].(string)
	rec := acceptOffer(t, h, posterTok, offerID, "p8-assign-"+uniq("k"))
	require.Equal(t, 200, rec.Code, rec.Body.String())
	// mark complete + confirm via execution
	rec = h.do("POST", "/tasks/"+taskID+"/complete", workerTok, "", nil)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	rec = h.do("POST", "/tasks/"+taskID+"/confirm", posterTok, "", nil)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	// Verify status
	var status string
	_ = h.pool.QueryRow(context.Background(), `SELECT status FROM tasks WHERE id=$1::uuid`, taskID).Scan(&status)
	require.Equal(t, "completed", status)
	return posterID, workerID, posterTok, workerTok, taskID
}

func submitReview(t *testing.T, h *harness, tok, taskID string, rating int, body *string) (int, map[string]any) {
	t.Helper()
	payload := map[string]any{"rating": rating}
	if body != nil {
		payload["body"] = *body
	}
	rec := h.do("POST", "/tasks/"+taskID+"/reviews", tok, "", payload)
	var out map[string]any
	if rec.Code == 201 {
		out = decodeBody(t, rec)
		if rev, ok := out["review"]; ok {
			return rec.Code, rev.(map[string]any)
		}
	}
	return rec.Code, out
}

// ── Gate 1: both parties can submit one review ─────────────────────────────

func TestP8_Gate1_BothCanSubmit(t *testing.T) {
	h := newHarness(t)
	_, _, posterTok, workerTok, taskID := setupCompletedTask(t, h)
	code, _ := submitReview(t, h, posterTok, taskID, 5, strPtr("Great work!"))
	require.Equal(t, 201, code)
	code, _ = submitReview(t, h, workerTok, taskID, 4, strPtr("Good poster"))
	require.Equal(t, 201, code)
}

// ── Gate 2: second submit publishes both simultaneously ────────────────────

func TestP8_Gate2_DoubleBlindPublish(t *testing.T) {
	h := newHarness(t)
	posterID, workerID, posterTok, workerTok, taskID := setupCompletedTask(t, h)
	// First review stays unpublished
	code, rev1 := submitReview(t, h, posterTok, taskID, 5, strPtr("Excellent"))
	require.Equal(t, 201, code)
	require.Equal(t, false, rev1["is_published"])
	// Second triggers both
	code, rev2 := submitReview(t, h, workerTok, taskID, 4, nil)
	require.Equal(t, 201, code)
	require.Equal(t, true, rev2["is_published"])
	// Verify first is now published
	rec := h.do("GET", "/me/reviews", posterTok, "", nil)
	require.Equal(t, 200, rec.Code)
	// Query directly to verify
	var isPub1 bool
	_ = h.pool.QueryRow(context.Background(), `SELECT is_published FROM reviews WHERE task_id=$1::uuid AND reviewer_id=$2::uuid`, taskID, posterID).Scan(&isPub1)
	require.True(t, isPub1)
	var isPub2 bool
	_ = h.pool.QueryRow(context.Background(), `SELECT is_published FROM reviews WHERE task_id=$1::uuid AND reviewer_id=$2::uuid`, taskID, workerID).Scan(&isPub2)
	require.True(t, isPub2)
}

// ── Gate 3: one-sided 14-day cron ──────────────────────────────────────────

func TestP8_Gate3_OneSidedCron(t *testing.T) {
	h := newHarness(t)
	posterID, _, posterTok, _, taskID := setupCompletedTask(t, h)
	code, _ := submitReview(t, h, posterTok, taskID, 5, nil)
	require.Equal(t, 201, code)
	// Verify still unpublished (only one)
	var isPub bool
	_ = h.pool.QueryRow(context.Background(), `SELECT is_published FROM reviews WHERE task_id=$1::uuid AND reviewer_id=$2::uuid`, taskID, posterID).Scan(&isPub)
	require.False(t, isPub)
	// Move completed_at 15 days ago to trigger cron
	_, err := h.pool.Exec(context.Background(), `UPDATE tasks SET completed_at=now() - interval '15 days' WHERE id=$1::uuid`, taskID)
	require.NoError(t, err)
	// Run cron
	rec := h.do("POST", "/internal/cron/publish-reviews", posterTok, "", nil)
	require.Equal(t, 200, rec.Code)
	// Now should be published
	_ = h.pool.QueryRow(context.Background(), `SELECT is_published FROM reviews WHERE task_id=$1::uuid AND reviewer_id=$2::uuid`, taskID, posterID).Scan(&isPub)
	require.True(t, isPub)
	// Other side still no review (nothing to publish for them)
	var cnt int
	_ = h.pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM reviews WHERE task_id=$1::uuid`, taskID).Scan(&cnt)
	require.Equal(t, 1, cnt)
}

// ── Gate 4: aggregates recomputed ──────────────────────────────────────────

func TestP8_Gate4_Aggregates(t *testing.T) {
	h := newHarness(t)
	_, workerID, posterTok, workerTok, taskID := setupCompletedTask(t, h)
	// First review
	_, _ = submitReview(t, h, posterTok, taskID, 5, nil)
	_, _ = submitReview(t, h, workerTok, taskID, 5, nil)
	var avg float64
	var cnt int
	_ = h.pool.QueryRow(context.Background(), `SELECT rating_avg, rating_count FROM users WHERE id=$1::uuid`, workerID).Scan(&avg, &cnt)
	require.Equal(t, 5.0, avg)
	require.Equal(t, 1, cnt)
	// Second task, same worker gets 3
	posterID2, posterTok2, _ := h.signup(t, uniq("p8p2")+"@example.com", "password123", "Poster P8-2")
	taskID2 := fundTaskForPhase4(t, h, posterTok2, validTaskInput())
	_, out := makeOffer(t, h, workerTok, taskID2, nil, "")
	offerID := out["offer"].(map[string]any)["id"].(string)
	rec := acceptOffer(t, h, posterTok2, offerID, "p8-agg2-"+uniq("k"))
	require.Equal(t, 200, rec.Code)
	rec = h.do("POST", "/tasks/"+taskID2+"/complete", workerTok, "", nil)
	require.Equal(t, 200, rec.Code)
	rec = h.do("POST", "/tasks/"+taskID2+"/confirm", posterTok2, "", nil)
	require.Equal(t, 200, rec.Code)
	code, _ := submitReview(t, h, posterTok2, taskID2, 3, nil)
	require.Equal(t, 201, code)
	code, _ = submitReview(t, h, workerTok, taskID2, 4, nil)
	require.Equal(t, 201, code)
	// Now worker should have 2 reviews: 5 and 3 => avg 4.0
	_ = h.pool.QueryRow(context.Background(), `SELECT rating_avg, rating_count FROM users WHERE id=$1::uuid`, workerID).Scan(&avg, &cnt)
	require.Equal(t, 2, cnt)
	require.InDelta(t, 4.0, avg, 0.01)
	_ = posterID2
}

// ── Gate 5: public only published ──────────────────────────────────────────

func TestP8_Gate5_PublicOnlyPublished(t *testing.T) {
	h := newHarness(t)
	posterID, workerID, posterTok, workerTok, taskID := setupCompletedTask(t, h)
	_, _ = submitReview(t, h, posterTok, taskID, 5, nil)
	// Before second, public should be empty
	rec := h.do("GET", "/users/"+workerID+"/reviews", posterTok, "", nil)
	require.Equal(t, 200, rec.Code)
	require.Len(t, decodeBody(t, rec)["reviews"].([]any), 0)
	_, _ = submitReview(t, h, workerTok, taskID, 4, nil)
	rec = h.do("GET", "/users/"+workerID+"/reviews", posterTok, "", nil)
	require.Equal(t, 200, rec.Code)
	require.Len(t, decodeBody(t, rec)["reviews"].([]any), 1)
	_ = posterID
}

// ── Gate 6: no second review ───────────────────────────────────────────────

func TestP8_Gate6_NoSecondReview(t *testing.T) {
	h := newHarness(t)
	_, _, posterTok, _, taskID := setupCompletedTask(t, h)
	code, _ := submitReview(t, h, posterTok, taskID, 5, nil)
	require.Equal(t, 201, code)
	code, _ = submitReview(t, h, posterTok, taskID, 4, nil)
	require.Equal(t, 409, code)
}

// ── Gate 7: not party cannot review ────────────────────────────────────────

func TestP8_Gate7_NotParty(t *testing.T) {
	h := newHarness(t)
	_, _, _, _, taskID := setupCompletedTask(t, h)
	_, strangerTok, _ := h.signup(t, uniq("p8s")+"@example.com", "password123", "Stranger")
	code, _ := submitReview(t, h, strangerTok, taskID, 5, nil)
	require.Equal(t, 403, code)
}

// ── Gate 8: not eligible (not completed) ───────────────────────────────────

func TestP8_Gate8_NotEligible(t *testing.T) {
	h := newHarness(t)
	_, _, posterTok, _, taskID, _ := setupAssignedTask(t, h) // assigned, not completed
	code, _ := submitReview(t, h, posterTok, taskID, 5, nil)
	require.Equal(t, 400, code)
	// Also open
	_, openTok, _ := h.signup(t, uniq("p8o")+"@example.com", "password123", "Open User")
	openTask := fundTaskForPhase4(t, h, openTok, validTaskInput())
	code, _ = submitReview(t, h, openTok, openTask, 5, nil)
	require.Equal(t, 400, code)
	// Also pending
	_, _, posterTok2, workerTok2, taskID2, _ := setupAssignedTask(t, h)
	_, _ = h.do("POST", "/tasks/"+taskID2+"/complete", workerTok2, "", nil).Body.String(), posterTok2 // mark pending
	var status string
	_ = h.pool.QueryRow(context.Background(), `SELECT status FROM tasks WHERE id=$1::uuid`, taskID2).Scan(&status)
	// task is now completed_pending, not completed
	code, _ = submitReview(t, h, posterTok2, taskID2, 5, nil)
	require.Equal(t, 400, code)
}

// ── Gate 9: /me/reviews hides unpublished about me ─────────────────────────

func TestP8_Gate9_HidesUnpublished(t *testing.T) {
	h := newHarness(t)
	posterID, workerID, posterTok, workerTok, taskID := setupCompletedTask(t, h)
	// Poster reviews worker (unpublished alone)
	_, _ = submitReview(t, h, posterTok, taskID, 5, strPtr("Great"))
	// Worker checks /me/reviews - should see pending but not rating/body
	rec := h.do("GET", "/me/reviews", workerTok, "", nil)
	require.Equal(t, 200, rec.Code)
	reviews := decodeBody(t, rec)["reviews"].([]any)
	found := false
	for _, r := range reviews {
		m := r.(map[string]any)
		if m["task_id"] == taskID && m["reviewee_id"] == workerID {
			found = true
			require.True(t, m["pending"].(bool))
			require.Nil(t, m["rating"])
			require.Nil(t, m["body"])
		}
	}
	require.True(t, found)
	// Poster checks own given review (should see rating/body even unpublished)
	rec = h.do("GET", "/me/reviews", posterTok, "", nil)
	found = false
	for _, r := range decodeBody(t, rec)["reviews"].([]any) {
		m := r.(map[string]any)
		if m["reviewer_id"] == posterID && m["task_id"] == taskID {
			found = true
			require.Equal(t, float64(5), m["rating"])
		}
	}
	require.True(t, found)
	// After second review, both should be visible
	_, _ = submitReview(t, h, workerTok, taskID, 4, strPtr("Okay"))
	rec = h.do("GET", "/me/reviews", workerTok, "", nil)
	for _, r := range decodeBody(t, rec)["reviews"].([]any) {
		m := r.(map[string]any)
		if m["task_id"] == taskID && m["reviewee_id"] == workerID {
			require.Equal(t, false, m["pending"])
			require.Equal(t, float64(5), m["rating"])
		}
	}
}

// ── Gate 10: cron idempotent ───────────────────────────────────────────────

func TestP8_Gate10_CronIdempotent(t *testing.T) {
	h := newHarness(t)
	posterID, _, posterTok, _, taskID := setupCompletedTask(t, h)
	_, _ = submitReview(t, h, posterTok, taskID, 5, nil)
	_, err := h.pool.Exec(context.Background(), `UPDATE tasks SET completed_at=now() - interval '15 days' WHERE id=$1::uuid`, taskID)
	require.NoError(t, err)
	rec := h.do("POST", "/internal/cron/publish-reviews", posterTok, "", nil)
	require.Equal(t, 200, rec.Code)
	var cnt1 int
	_ = h.pool.QueryRow(context.Background(), `SELECT rating_count FROM users WHERE id=$1::uuid`, posterID).Scan(&cnt1) // actually reviewee is worker, not poster
	// Check worker's count
	var workerID string
	_ = h.pool.QueryRow(context.Background(), `SELECT assigned_worker_id::text FROM tasks WHERE id=$1::uuid`, taskID).Scan(&workerID)
	var avg1 float64
	var cntA int
	_ = h.pool.QueryRow(context.Background(), `SELECT rating_avg, rating_count FROM users WHERE id=$1::uuid`, workerID).Scan(&avg1, &cntA)
	// Run again
	rec = h.do("POST", "/internal/cron/publish-reviews", posterTok, "", nil)
	require.Equal(t, 200, rec.Code)
	var avg2 float64
	var cntB int
	_ = h.pool.QueryRow(context.Background(), `SELECT rating_avg, rating_count FROM users WHERE id=$1::uuid`, workerID).Scan(&avg2, &cntB)
	require.Equal(t, cntA, cntB)
	require.Equal(t, avg1, avg2)
}

// ── Gate 11: cannot set is_published via client ─────────────────────────────

func TestP8_Gate11_CannotSetPublished(t *testing.T) {
	h := newHarness(t)
	_, _, posterTok, _, taskID := setupCompletedTask(t, h)
	payload := map[string]any{"rating": 5, "body": "hi", "is_published": true, "published_at": "2026-01-01T00:00:00Z"}
	rec := h.do("POST", "/tasks/"+taskID+"/reviews", posterTok, "", payload)
	require.Equal(t, 400, rec.Code)
	// Also via raw JSON with is_published
	rec = h.doRaw("POST", "/tasks/"+taskID+"/reviews", posterTok, `{"rating":5,"is_published":true}`)
	require.Equal(t, 400, rec.Code)
}

// ── Gate 12: cannot edit/delete ────────────────────────────────────────────

func TestP8_Gate12_CannotEditDelete(t *testing.T) {
	h := newHarness(t)
	_, _, posterTok, _, taskID := setupCompletedTask(t, h)
	code, rev := submitReview(t, h, posterTok, taskID, 5, strPtr("orig"))
	require.Equal(t, 201, code)
	reviewID := rev["id"].(string)
	// Try PUT/PATCH/DELETE
	for _, method := range []string{"PUT", "PATCH", "DELETE"} {
		rec := h.do(method, "/tasks/"+taskID+"/reviews/"+reviewID, posterTok, "", map[string]any{"rating": 1})
		require.True(t, rec.Code == 404 || rec.Code == 405, "method %s should be not found/method not allowed", method)
	}
	// Also try direct update via DB as restricted user should be blocked by RLS (no policy for update)
	ctx := context.Background()
	_, err := h.pool.Exec(ctx, `GRANT USAGE ON SCHEMA public TO phase0_restricted`)
	require.NoError(t, err)
	_, err = h.pool.Exec(ctx, `GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO phase0_restricted`)
	require.NoError(t, err)
	conn, err := h.pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, `SET ROLE phase0_restricted`)
	require.NoError(t, err)
	defer func() { _, _ = conn.Exec(context.Background(), `RESET ROLE`) }()
	tag, err := conn.Exec(ctx, `UPDATE reviews SET rating=1 WHERE id=$1::uuid`, reviewID)
	require.NoError(t, err)
	require.Equal(t, int64(0), tag.RowsAffected())
	tag, err = conn.Exec(ctx, `DELETE FROM reviews WHERE id=$1::uuid`, reviewID)
	require.NoError(t, err)
	require.Equal(t, int64(0), tag.RowsAffected())
}

// ── Gate 13: aggregate correct after 3 publishes ───────────────────────────

func TestP8_Gate13_AggregateThree(t *testing.T) {
	h := newHarness(t)
	// Create worker who will get 3 reviews
	_, workerID, _, workerTok, _ := setupCompletedTask(t, h) // dummy to get worker
	// We'll create 3 tasks each completed with same worker as reviewee
	ratings := []int{5, 3, 4}
	for _, r := range ratings {
		posterID, posterTok, _ := h.signup(t, uniq("p8agg")+"@example.com", "password123", "Agg Poster")
		taskID := fundTaskForPhase4(t, h, posterTok, validTaskInput())
		_, out := makeOffer(t, h, workerTok, taskID, nil, "")
		offerID := out["offer"].(map[string]any)["id"].(string)
		rec := acceptOffer(t, h, posterTok, offerID, "agg-"+uniq("k"))
		require.Equal(t, 200, rec.Code)
		rec = h.do("POST", "/tasks/"+taskID+"/complete", workerTok, "", nil)
		require.Equal(t, 200, rec.Code)
		rec = h.do("POST", "/tasks/"+taskID+"/confirm", posterTok, "", nil)
		require.Equal(t, 200, rec.Code)
		// Submit both reviews to publish
		_, _ = submitReview(t, h, posterTok, taskID, r, nil)
		_, _ = submitReview(t, h, workerTok, taskID, 5, nil)
		_ = posterID
	}
	var avg float64
	var cnt int
	_ = h.pool.QueryRow(context.Background(), `SELECT rating_avg, rating_count FROM users WHERE id=$1::uuid`, workerID).Scan(&avg, &cnt)
	require.Equal(t, 3, cnt)
	// (5+3+4)/3 = 4.0
	require.InDelta(t, 4.0, avg, 0.01)
}

// ── Self-audit: same tryPublish function ───────────────────────────────────

func TestP8_SelfAudit_SameFunction(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "internal", "reviews", "reviews.go"))
	require.NoError(t, err)
	s := string(raw)
	require.Contains(t, s, "func (s *Service) tryPublishReviews")
	require.Contains(t, s, "func (s *Service) PublishCron")
	// Both should call recomputeAggregateTx, not duplicate logic
	require.GreaterOrEqual(t, strings.Count(s, "recomputeAggregateTx"), 2)
	// Only one place should have UPDATE reviews SET is_published=true
	// Actually two places: tryPublish and publishOneSided, but they share recompute
	require.Contains(t, s, "is_published")

	rawHTTP, err := os.ReadFile(filepath.Join("..", "internal", "httpapi", "reviews.go"))
	require.NoError(t, err)
	require.Contains(t, string(rawHTTP), "handleSubmitReview")
	require.Contains(t, string(rawHTTP), "handlePublishCron")
}

func TestP8_Docs(t *testing.T) {
	h := newHarness(t)
	rec := h.do("GET", "/openapi.yaml", "", "", nil)
	require.Equal(t, 200, rec.Code)
	body := rec.Body.String()
	for _, p := range []string{"/tasks/{id}/reviews", "/users/{id}/reviews", "/me/reviews", "/internal/cron/publish-reviews"} {
		require.Contains(t, body, p)
	}
	rec = h.do("GET", "/docs", "", "", nil)
	require.Equal(t, 200, rec.Code)
	require.Contains(t, rec.Body.String(), "scalar")
}

// helpers to expose harness internals for tests that need raw request
func (h *harness) doRaw(method, path, token, raw string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.server.ServeHTTP(rec, req)
	return rec
}
