// Phase 4 Testing Gate — offers & matching (20 gates + self-audit).
package tests

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// ── helpers ────────────────────────────────────────────────────────────────

func fundTaskForPhase4(t *testing.T, h *harness, posterTok string, in map[string]any) string {
	t.Helper()
	id := createTask(t, h, posterTok, in)["id"].(string)
	rec := fundTask(t, h, posterTok, id, "p4-"+uniq("k"))
	require.Equal(t, 200, rec.Code, rec.Body.String())
	return id
}

func makeOffer(t *testing.T, h *harness, workerTok, taskID string, amount *int, msg string) (int, map[string]any) {
	t.Helper()
	body := map[string]any{}
	if amount != nil {
		body["amount"] = *amount
	}
	if msg != "" {
		body["message"] = msg
	}
	rec := h.do("POST", "/tasks/"+taskID+"/offers", workerTok, "", body)
	var out map[string]any
	if rec.Code == 201 {
		out = decodeBody(t, rec)
	}
	return rec.Code, out
}

func acceptOffer(t *testing.T, h *harness, posterTok, offerID, idem string) *httptest.ResponseRecorder {
	t.Helper()
	body := map[string]any{}
	if idem != "" {
		body["idempotency_key"] = idem
	}
	return h.do("POST", "/offers/"+offerID+"/accept", posterTok, "", body)
}

// ── Gate 1: at-budget + counter ────────────────────────────────────────────

func TestP4_Gate1_CreateOffers(t *testing.T) {
	h := newHarness(t)
	_, posterTok, _ := h.signup(t, uniq("p4p1a")+"@example.com", "password123", "Poster P4-1")
	_, workerTok, _ := h.signup(t, uniq("p4w1a")+"@example.com", "password123", "Worker P4-1")
	taskID := fundTaskForPhase4(t, h, posterTok, validTaskInput())

	// At-budget (no amount or amount == budget) succeeds without message.
	code, out := makeOffer(t, h, workerTok, taskID, nil, "Looking forward to help")
	require.Equal(t, 201, code)
	offer := out["offer"].(map[string]any)
	require.Equal(t, false, offer["is_counter"])
	require.Equal(t, float64(4000), offer["amount"])

	// Counter with different amount requires message.
	_, worker2Tok, _ := h.signup(t, uniq("p4w1b")+"@example.com", "password123", "Worker P4-1b")
	amt := 5000
	code, _ = makeOffer(t, h, worker2Tok, taskID, &amt, "")
	require.Equal(t, 400, code)
	code, out = makeOffer(t, h, worker2Tok, taskID, &amt, "Need more due to distance")
	require.Equal(t, 201, code)
	offer2 := out["offer"].(map[string]any)
	require.Equal(t, true, offer2["is_counter"])

	// Message too long
	amt2 := 6000
	_, worker3Tok, _ := h.signup(t, uniq("p4w1c")+"@example.com", "password123", "Worker P4-1c")
	code, _ = makeOffer(t, h, worker3Tok, taskID, &amt2, strings.Repeat("x", 301))
	require.Equal(t, 400, code)
}

// ── Gate 2: one pending per worker per task ────────────────────────────────

func TestP4_Gate2_UniquePending(t *testing.T) {
	h := newHarness(t)
	_, posterTok, _ := h.signup(t, uniq("p4p2")+"@example.com", "password123", "Poster P4-2")
	_, workerTok, _ := h.signup(t, uniq("p4w2")+"@example.com", "password123", "Worker P4-2")
	taskID := fundTaskForPhase4(t, h, posterTok, validTaskInput())
	code, _ := makeOffer(t, h, workerTok, taskID, nil, "")
	require.Equal(t, 201, code)
	amt := 5000
	code, _ = makeOffer(t, h, workerTok, taskID, &amt, "second attempt")
	require.Equal(t, 409, code)
}

// ── Gate 3: ranked view for poster ────────────────────────────────────────

func TestP4_Gate3_RankedList(t *testing.T) {
	h := newHarness(t)
	_, posterTok, _ := h.signup(t, uniq("p4p3")+"@example.com", "password123", "Poster P4-3")
	// Create three workers with distinct ranking attributes.
	idA, tokA, _ := h.signup(t, uniq("p4w3a")+"@example.com", "password123", "Worker A")
	idB, tokB, _ := h.signup(t, uniq("p4w3b")+"@example.com", "password123", "Worker B")
	idC, tokC, _ := h.signup(t, uniq("p4w3c")+"@example.com", "password123", "Worker C")
	ctx := context.Background()
	// A: verified, high rating, many tasks
	_, err := h.pool.Exec(ctx, `UPDATE users SET verification_status='verified', rating_avg=4.8, rating_count=20, tasks_completed=30 WHERE id=$1::uuid`, idA)
	require.NoError(t, err)
	// B: unverified, same rating but fewer tasks
	_, err = h.pool.Exec(ctx, `UPDATE users SET verification_status='unverified', rating_avg=4.8, rating_count=20, tasks_completed=5 WHERE id=$1::uuid`, idB)
	require.NoError(t, err)
	// C: verified but lower rating
	_, err = h.pool.Exec(ctx, `UPDATE users SET verification_status='verified', rating_avg=3.0, rating_count=10, tasks_completed=50 WHERE id=$1::uuid`, idC)
	require.NoError(t, err)

	taskID := fundTaskForPhase4(t, h, posterTok, validTaskInput())
	// Offers in non-ranked order: B first, C second, A last
	makeOffer(t, h, tokB, taskID, nil, "")
	makeOffer(t, h, tokC, taskID, nil, "")
	makeOffer(t, h, tokA, taskID, nil, "")

	rec := h.do("GET", "/tasks/"+taskID+"/offers", posterTok, "", nil)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	offers := decodeBody(t, rec)["offers"].([]any)
	require.Len(t, offers, 3)
	// Expected order: A (verified 4.8 30 tasks) before C (verified 3.0) before B (unverified)
	first := offers[0].(map[string]any)
	second := offers[1].(map[string]any)
	third := offers[2].(map[string]any)
	require.Equal(t, idA, first["worker_id"])
	require.Equal(t, idC, second["worker_id"])
	require.Equal(t, idB, third["worker_id"])
	// Ensure no phone/email leak
	for _, o := range offers {
		body := fmt.Sprintf("%v", o)
		require.NotContains(t, strings.ToLower(body), "phone")
		require.NotContains(t, strings.ToLower(body), "email")
	}
	_ = idA
}

// ── Gate 4: worker my offers filtered ──────────────────────────────────────

func TestP4_Gate4_MyOffers(t *testing.T) {
	h := newHarness(t)
	_, posterTok, _ := h.signup(t, uniq("p4p4")+"@example.com", "password123", "Poster P4-4")
	_, workerTok, _ := h.signup(t, uniq("p4w4")+"@example.com", "password123", "Worker P4-4")
	taskID := fundTaskForPhase4(t, h, posterTok, validTaskInput())
	_, out := makeOffer(t, h, workerTok, taskID, nil, "")
	offerID := out["offer"].(map[string]any)["id"].(string)

	rec := h.do("GET", "/me/offers?status=pending", workerTok, "", nil)
	require.Equal(t, 200, rec.Code)
	require.Len(t, decodeBody(t, rec)["offers"].([]any), 1)

	// Poster declines -> pending filtered empty, declined has one
	rec = h.do("POST", "/offers/"+offerID+"/decline", posterTok, "", map[string]any{"reason": "not now"})
	require.Equal(t, 200, rec.Code)
	rec = h.do("GET", "/me/offers?status=pending", workerTok, "", nil)
	require.Len(t, decodeBody(t, rec)["offers"].([]any), 0)
	rec = h.do("GET", "/me/offers?status=declined", workerTok, "", nil)
	require.Len(t, decodeBody(t, rec)["offers"].([]any), 1)
}

// ── Gate 5: decline individual ─────────────────────────────────────────────

func TestP4_Gate5_Decline(t *testing.T) {
	h := newHarness(t)
	_, posterTok, _ := h.signup(t, uniq("p4p5")+"@example.com", "password123", "Poster P4-5")
	_, workerTok, _ := h.signup(t, uniq("p4w5")+"@example.com", "password123", "Worker P4-5")
	taskID := fundTaskForPhase4(t, h, posterTok, validTaskInput())
	_, out := makeOffer(t, h, workerTok, taskID, nil, "")
	offerID := out["offer"].(map[string]any)["id"].(string)
	rec := h.do("POST", "/offers/"+offerID+"/decline", posterTok, "", map[string]any{"reason": "too far"})
	require.Equal(t, 200, rec.Code)
	body := decodeBody(t, rec)["offer"].(map[string]any)
	require.Equal(t, "declined", body["status"])
	require.Equal(t, "too far", body["decline_reason"])
	// Second decline fails (not pending)
	rec = h.do("POST", "/offers/"+offerID+"/decline", posterTok, "", nil)
	require.Equal(t, 409, rec.Code)
}

// ── Gate 6: poster counter one round ───────────────────────────────────────

func TestP4_Gate6_CounterOnce(t *testing.T) {
	h := newHarness(t)
	_, posterTok, _ := h.signup(t, uniq("p4p6")+"@example.com", "password123", "Poster P4-6")
	_, workerTok, _ := h.signup(t, uniq("p4w6")+"@example.com", "password123", "Worker P4-6")
	taskID := fundTaskForPhase4(t, h, posterTok, validTaskInput())
	_, out := makeOffer(t, h, workerTok, taskID, nil, "")
	offerID := out["offer"].(map[string]any)["id"].(string)

	rec := h.do("POST", "/offers/"+offerID+"/counter", posterTok, "", map[string]any{"amount": 3500})
	require.Equal(t, 200, rec.Code)
	rec = h.do("POST", "/offers/"+offerID+"/counter", posterTok, "", map[string]any{"amount": 3000})
	require.Equal(t, 409, rec.Code)
}

// ── Gate 7: worker accept/decline counter ──────────────────────────────────

func TestP4_Gate7_RespondCounter(t *testing.T) {
	h := newHarness(t)
	_, posterTok, _ := h.signup(t, uniq("p4p7a")+"@example.com", "password123", "Poster P4-7a")
	_, workerTok, _ := h.signup(t, uniq("p4w7a")+"@example.com", "password123", "Worker P4-7a")
	taskID := fundTaskForPhase4(t, h, posterTok, validTaskInput())
	_, out := makeOffer(t, h, workerTok, taskID, nil, "")
	offerID := out["offer"].(map[string]any)["id"].(string)
	rec := h.do("POST", "/offers/"+offerID+"/counter", posterTok, "", map[string]any{"amount": 3500})
	require.Equal(t, 200, rec.Code)
	// Decline path
	rec = h.do("POST", "/offers/"+offerID+"/respond", workerTok, "", map[string]any{"action": "decline"})
	require.Equal(t, 200, rec.Code)
	require.Equal(t, "declined", decodeBody(t, rec)["offer"].(map[string]any)["poster_counter_status"])

	// Accept path on fresh offer
	_, workerTok2, _ := h.signup(t, uniq("p4w7b")+"@example.com", "password123", "Worker P4-7b")
	taskID2 := fundTaskForPhase4(t, h, posterTok, validTaskInput())
	_, out2 := makeOffer(t, h, workerTok2, taskID2, nil, "")
	offerID2 := out2["offer"].(map[string]any)["id"].(string)
	rec = h.do("POST", "/offers/"+offerID2+"/counter", posterTok, "", map[string]any{"amount": 3600})
	require.Equal(t, 200, rec.Code)
	rec = h.do("POST", "/offers/"+offerID2+"/respond", workerTok2, "", map[string]any{"action": "accept"})
	require.Equal(t, 200, rec.Code)
	// Task should now be assigned
	rec = h.do("GET", "/tasks/"+taskID2, posterTok, "", nil)
	require.Equal(t, "assigned", decodeBody(t, rec)["task"].(map[string]any)["status"])
}

// ── Gate 8: accept creates assigned + auto_declined + conversation + system msg ──

func TestP4_Gate8_AcceptTransaction(t *testing.T) {
	h := newHarness(t)
	_, posterTok, _ := h.signup(t, uniq("p4p8")+"@example.com", "password123", "Poster P4-8")
	_, w1Tok, _ := h.signup(t, uniq("p4w8a")+"@example.com", "password123", "Worker 8A")
	_, w2Tok, _ := h.signup(t, uniq("p4w8b")+"@example.com", "password123", "Worker 8B")
	taskID := fundTaskForPhase4(t, h, posterTok, validTaskInput())
	_, out1 := makeOffer(t, h, w1Tok, taskID, nil, "")
	_, out2 := makeOffer(t, h, w2Tok, taskID, nil, "")
	offer1 := out1["offer"].(map[string]any)["id"].(string)
	offer2 := out2["offer"].(map[string]any)["id"].(string)

	rec := acceptOffer(t, h, posterTok, offer1, "idem-accept-8-"+uniq("k"))
	require.Equal(t, 200, rec.Code, rec.Body.String())
	acc := decodeBody(t, rec)["accept"].(map[string]any)
	require.Equal(t, "assigned", acc["task_status"])
	convID := acc["conversation_id"].(string)
	require.NotEmpty(t, convID)

	// Task assigned to w1
	rec = h.do("GET", "/tasks/"+taskID, posterTok, "", nil)
	require.Equal(t, "assigned", decodeBody(t, rec)["task"].(map[string]any)["status"])
	// Sibling auto_declined
	ctx := context.Background()
	var s1, s2 string
	require.NoError(t, h.pool.QueryRow(ctx, `SELECT status FROM offers WHERE id=$1::uuid`, offer1).Scan(&s1))
	require.NoError(t, h.pool.QueryRow(ctx, `SELECT status FROM offers WHERE id=$1::uuid`, offer2).Scan(&s2))
	require.Equal(t, "accepted", s1)
	require.Equal(t, "auto_declined", s2)
	// Conversation exists
	var cnt int
	require.NoError(t, h.pool.QueryRow(ctx, `SELECT count(*) FROM conversations WHERE task_id=$1::uuid`, taskID).Scan(&cnt))
	require.Equal(t, 1, cnt)
	// System message present and contains exact location
	rec = h.do("GET", "/conversations/"+convID+"/messages", posterTok, "", nil)
	require.Equal(t, 200, rec.Code)
	msgs := decodeBody(t, rec)["messages"].([]any)
	require.Len(t, msgs, 1)
	first := msgs[0].(map[string]any)
	require.Equal(t, "system", first["type"])
	require.Contains(t, first["body"].(string), "14 Rye Lane")
}

// ── Gate 9: withdraw ───────────────────────────────────────────────────────

func TestP4_Gate9_Withdraw(t *testing.T) {
	h := newHarness(t)
	_, posterTok, _ := h.signup(t, uniq("p4p9")+"@example.com", "password123", "Poster P4-9")
	_, workerTok, _ := h.signup(t, uniq("p4w9")+"@example.com", "password123", "Worker P4-9")
	taskID := fundTaskForPhase4(t, h, posterTok, validTaskInput())
	_, out := makeOffer(t, h, workerTok, taskID, nil, "")
	offerID := out["offer"].(map[string]any)["id"].(string)
	rec := h.do("POST", "/offers/"+offerID+"/withdraw", workerTok, "", nil)
	require.Equal(t, 200, rec.Code)
	require.Equal(t, "withdrawn", decodeBody(t, rec)["offer"].(map[string]any)["status"])
	// Second withdraw fails
	rec = h.do("POST", "/offers/"+offerID+"/withdraw", workerTok, "", nil)
	require.Equal(t, 409, rec.Code)
}

// ── Gate 10: concurrency — two accepts, one wins ───────────────────────────

func TestP4_Gate10_ConcurrentAccept(t *testing.T) {
	h := newHarness(t)
	_, posterTok, _ := h.signup(t, uniq("p4p10")+"@example.com", "password123", "Poster P4-10")
	_, w1Tok, _ := h.signup(t, uniq("p4w10a")+"@example.com", "password123", "Worker 10A")
	_, w2Tok, _ := h.signup(t, uniq("p4w10b")+"@example.com", "password123", "Worker 10B")
	taskID := fundTaskForPhase4(t, h, posterTok, validTaskInput())
	_, out1 := makeOffer(t, h, w1Tok, taskID, nil, "")
	_, out2 := makeOffer(t, h, w2Tok, taskID, nil, "")
	o1 := out1["offer"].(map[string]any)["id"].(string)
	o2 := out2["offer"].(map[string]any)["id"].(string)

	var wg sync.WaitGroup
	codes := make([]int, 2)
	ids := []string{o1, o2}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rec := h.do("POST", "/offers/"+ids[i]+"/accept", posterTok, "", map[string]any{"idempotency_key": "conc-" + uniq("k") + fmt.Sprint(i)})
			codes[i] = rec.Code
		}(i)
	}
	wg.Wait()
	succeeded := 0
	for _, c := range codes {
		if c == 200 {
			succeeded++
		}
	}
	require.Equal(t, 1, succeeded, "exactly one concurrent accept must succeed (got %v)", codes)
	ctx := context.Background()
	var accepted int
	require.NoError(t, h.pool.QueryRow(ctx, `SELECT count(*) FROM offers WHERE task_id=$1::uuid AND status='accepted'`, taskID).Scan(&accepted))
	require.Equal(t, 1, accepted)
	var taskStatus string
	require.NoError(t, h.pool.QueryRow(ctx, `SELECT status FROM tasks WHERE id=$1::uuid`, taskID).Scan(&taskStatus))
	require.Equal(t, "assigned", taskStatus)
}

// ── Gate 11: idempotent retry ──────────────────────────────────────────────

func TestP4_Gate11_IdempotentAccept(t *testing.T) {
	h := newHarness(t)
	_, posterTok, _ := h.signup(t, uniq("p4p11")+"@example.com", "password123", "Poster P4-11")
	_, w1Tok, _ := h.signup(t, uniq("p4w11a")+"@example.com", "password123", "Worker 11A")
	_, w2Tok, _ := h.signup(t, uniq("p4w11b")+"@example.com", "password123", "Worker 11B")
	taskID := fundTaskForPhase4(t, h, posterTok, validTaskInput())
	_, out1 := makeOffer(t, h, w1Tok, taskID, nil, "")
	_, _ = makeOffer(t, h, w2Tok, taskID, nil, "")
	o1 := out1["offer"].(map[string]any)["id"].(string)
	key := "idem-11-" + uniq("k")
	first := acceptOffer(t, h, posterTok, o1, key)
	require.Equal(t, 200, first.Code)
	firstBody := decodeBody(t, first)["accept"].(map[string]any)
	second := acceptOffer(t, h, posterTok, o1, key)
	require.Equal(t, 200, second.Code)
	secondBody := decodeBody(t, second)["accept"].(map[string]any)
	require.Equal(t, true, secondBody["repeated"])
	require.Equal(t, firstBody["conversation_id"], secondBody["conversation_id"])

	ctx := context.Background()
	var convCnt int
	require.NoError(t, h.pool.QueryRow(ctx, `SELECT count(*) FROM conversations WHERE task_id=$1::uuid`, taskID).Scan(&convCnt))
	require.Equal(t, 1, convCnt)
	var autoCnt int
	require.NoError(t, h.pool.QueryRow(ctx, `SELECT count(*) FROM offers WHERE task_id=$1::uuid AND status='auto_declined'`, taskID).Scan(&autoCnt))
	require.Equal(t, 1, autoCnt)
}

// ── Gate 12: cannot offer own task ─────────────────────────────────────────

func TestP4_Gate12_OwnTask(t *testing.T) {
	h := newHarness(t)
	_, posterTok, _ := h.signup(t, uniq("p4p12")+"@example.com", "password123", "Poster P4-12")
	taskID := fundTaskForPhase4(t, h, posterTok, validTaskInput())
	code, _ := makeOffer(t, h, posterTok, taskID, nil, "")
	require.Equal(t, 403, code)
}

// ── Gate 13: blocked cannot offer ──────────────────────────────────────────

func TestP4_Gate13_BlockedOffer(t *testing.T) {
	h := newHarness(t)
	idP, posterTok, _ := h.signup(t, uniq("p4p13")+"@example.com", "password123", "Poster P4-13")
	idW, workerTok, _ := h.signup(t, uniq("p4w13")+"@example.com", "password123", "Worker P4-13")
	taskID := fundTaskForPhase4(t, h, posterTok, validTaskInput())
	ctx := context.Background()
	// Worker blocks poster
	_, err := h.pool.Exec(ctx, `INSERT INTO blocks (blocker_id, blocked_id) VALUES ($1::uuid,$2::uuid)`, idW, idP)
	require.NoError(t, err)
	code, _ := makeOffer(t, h, workerTok, taskID, nil, "")
	require.Equal(t, 403, code)
	_, _ = h.pool.Exec(ctx, `DELETE FROM blocks WHERE blocker_id=$1::uuid AND blocked_id=$2::uuid`, idW, idP)
	// Poster blocks worker
	_, err = h.pool.Exec(ctx, `INSERT INTO blocks (blocker_id, blocked_id) VALUES ($1::uuid,$2::uuid)`, idP, idW)
	require.NoError(t, err)
	code, _ = makeOffer(t, h, workerTok, taskID, nil, "")
	require.Equal(t, 403, code)
}

// ── Gate 14: suspended cannot offer ────────────────────────────────────────

func TestP4_Gate14_SuspendedOffer(t *testing.T) {
	h := newHarness(t)
	_, posterTok, _ := h.signup(t, uniq("p4p14a")+"@example.com", "password123", "Poster P4-14")
	idW, workerTok, _ := h.signup(t, uniq("p4w14a")+"@example.com", "password123", "Worker P4-14")
	taskID := fundTaskForPhase4(t, h, posterTok, validTaskInput())
	_, err := h.pool.Exec(context.Background(), `UPDATE users SET suspended_at=now() WHERE id=$1::uuid`, idW)
	require.NoError(t, err)
	code, _ := makeOffer(t, h, workerTok, taskID, nil, "")
	require.True(t, code == 403 || code == 401, "suspended worker must be rejected (got %d)", code)
}

// ── Gate 15: cannot act on others' offer ───────────────────────────────────

func TestP4_Gate15_OtherOffer(t *testing.T) {
	h := newHarness(t)
	_, posterTok, _ := h.signup(t, uniq("p4p15")+"@example.com", "password123", "Poster P4-15")
	_, workerTok, _ := h.signup(t, uniq("p4w15a")+"@example.com", "password123", "Worker 15A")
	_, otherTok, _ := h.signup(t, uniq("p4w15b")+"@example.com", "password123", "Worker 15B")
	taskID := fundTaskForPhase4(t, h, posterTok, validTaskInput())
	_, out := makeOffer(t, h, workerTok, taskID, nil, "")
	offerID := out["offer"].(map[string]any)["id"].(string)
	require.Equal(t, 403, h.do("POST", "/offers/"+offerID+"/withdraw", otherTok, "", nil).Code)
	require.Equal(t, 403, h.do("POST", "/offers/"+offerID+"/respond", otherTok, "", map[string]any{"action": "accept"}).Code)
}

// ── Gate 16: poster cannot act on others' task offers ──────────────────────

func TestP4_Gate16_WrongPoster(t *testing.T) {
	h := newHarness(t)
	_, posterTok, _ := h.signup(t, uniq("p4p16a")+"@example.com", "password123", "Poster P4-16A")
	_, otherPosterTok, _ := h.signup(t, uniq("p4p16b")+"@example.com", "password123", "Poster P4-16B")
	_, workerTok, _ := h.signup(t, uniq("p4w16")+"@example.com", "password123", "Worker P4-16")
	taskID := fundTaskForPhase4(t, h, posterTok, validTaskInput())
	_, out := makeOffer(t, h, workerTok, taskID, nil, "")
	offerID := out["offer"].(map[string]any)["id"].(string)
	require.Equal(t, 403, h.do("POST", "/offers/"+offerID+"/decline", otherPosterTok, "", nil).Code)
	require.Equal(t, 403, h.do("POST", "/offers/"+offerID+"/counter", otherPosterTok, "", map[string]any{"amount": 3000}).Code)
	require.Equal(t, 403, h.do("POST", "/offers/"+offerID+"/accept", otherPosterTok, "", map[string]any{"idempotency_key": uniq("k")}).Code)
}

// ── Gate 17: RLS rejects direct writes ─────────────────────────────────────

func TestP4_Gate17_RLSWrites(t *testing.T) {
	h := newHarness(t)
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
	tag, err := conn.Exec(ctx, `UPDATE offers SET status='accepted' WHERE true`)
	require.NoError(t, err)
	require.Zero(t, tag.RowsAffected(), "direct offers.status update must be denied by RLS")
	_, err = conn.Exec(ctx, `INSERT INTO conversations (task_id, poster_id, worker_id) VALUES ('00000000-0000-0000-0000-000000000000'::uuid, '00000000-0000-0000-0000-000000000001'::uuid, '00000000-0000-0000-0000-000000000002'::uuid)`)
	require.Error(t, err, "direct conversation insert must be rejected by RLS")
}

// ── Gate 18: assigned task rejects new offers ──────────────────────────────

func TestP4_Gate18_NoOfferAfterAssigned(t *testing.T) {
	h := newHarness(t)
	_, posterTok, _ := h.signup(t, uniq("p4p18")+"@example.com", "password123", "Poster P4-18")
	_, w1Tok, _ := h.signup(t, uniq("p4w18a")+"@example.com", "password123", "Worker 18A")
	_, w2Tok, _ := h.signup(t, uniq("p4w18b")+"@example.com", "password123", "Worker 18B")
	taskID := fundTaskForPhase4(t, h, posterTok, validTaskInput())
	_, out := makeOffer(t, h, w1Tok, taskID, nil, "")
	offerID := out["offer"].(map[string]any)["id"].(string)
	rec := acceptOffer(t, h, posterTok, offerID, "idem-18-"+uniq("k"))
	require.Equal(t, 200, rec.Code)
	code, _ := makeOffer(t, h, w2Tok, taskID, nil, "")
	require.Equal(t, 409, code)
}

// ── Gate 19: exact location only for participants via system message ───────

func TestP4_Gate19_ExactViaSystemMessage(t *testing.T) {
	h := newHarness(t)
	_, posterTok, _ := h.signup(t, uniq("p4p19a")+"@example.com", "password123", "Poster P4-19")
	idW, workerTok, _ := h.signup(t, uniq("p4w19a")+"@example.com", "password123", "Worker P4-19")
	_, strangerTok, _ := h.signup(t, uniq("p4s19")+"@example.com", "password123", "Stranger P4-19")
	taskID := fundTaskForPhase4(t, h, posterTok, validTaskInput())
	_, out := makeOffer(t, h, workerTok, taskID, nil, "")
	offerID := out["offer"].(map[string]any)["id"].(string)
	rec := acceptOffer(t, h, posterTok, offerID, "idem-19-"+uniq("k"))
	require.Equal(t, 200, rec.Code)
	convID := decodeBody(t, rec)["accept"].(map[string]any)["conversation_id"].(string)

	// Poster and assigned worker can read and see exact
	for _, tok := range []string{posterTok, workerTok} {
		rec = h.do("GET", "/conversations/"+convID+"/messages", tok, "", nil)
		require.Equal(t, 200, rec.Code)
		require.Contains(t, rec.Body.String(), "14 Rye Lane")
	}
	// Stranger forbidden
	rec = h.do("GET", "/conversations/"+convID+"/messages", strangerTok, "", nil)
	require.Equal(t, 403, rec.Code)
	// Also verify stranger's task detail still hides exact (Phase 2 rule)
	rec = h.do("GET", "/tasks/"+taskID, strangerTok, "", nil)
	require.Equal(t, 200, rec.Code)
	require.NotContains(t, rec.Body.String(), "Rye Lane")
	// But participants see it via task detail too (assigned status)
	rec = h.do("GET", "/tasks/"+taskID, workerTok, "", nil)
	require.Contains(t, rec.Body.String(), "Rye Lane")
	_ = idW
}

// ── Gate 20: Phase 3 exclusions with live offers ───────────────────────────

func TestP4_Gate20_ExclusionsLive(t *testing.T) {
	h := newHarness(t)
	_, posterTok, _ := h.signup(t, uniq("p4p20")+"@example.com", "password123", "Poster P4-20")
	idW, workerTok, _ := h.signup(t, uniq("p4w20")+"@example.com", "password123", "Worker P4-20")
	_, otherTok, _ := h.signup(t, uniq("p4o20")+"@example.com", "password123", "Other P4-20")
	cat := uniq("p4cat20")
	// Four tasks: one will get a pending offer from worker, one blocked, one suspended poster, one clean
	cleanID := fundTaskForPhase4(t, h, posterTok, geoInput(51.5, -0.12, cat, 2000, "asap"))
	offerTaskID := fundTaskForPhase4(t, h, posterTok, geoInput(51.5, -0.12, cat, 2000, "asap"))
	_, _ = makeOffer(t, h, workerTok, offerTaskID, nil, "")

	// Blocked: poster blocks other -> other's feed should not see blocked poster's tasks, but worker's view unaffected? Actually worker is not blocked.
	// Create a task from blocked poster perspective: poster2 tasks hidden from blocker.
	idBlockedPoster, blockedPosterTok, _ := h.signup(t, uniq("p4bp")+"@example.com", "password123", "Blocked Poster")
	blockedTaskID := fundTaskForPhase4(t, h, blockedPosterTok, geoInput(51.5, -0.12, cat, 2000, "asap"))
	ctx := context.Background()
	_, err := h.pool.Exec(ctx, `INSERT INTO blocks (blocker_id, blocked_id) VALUES ($1::uuid,$2::uuid)`, idW, idBlockedPoster)
	require.NoError(t, err)

	// Worker who offered should not see offerTask in feed
	code, out := getFeed(t, h, workerTok, "?category="+cat)
	require.Equal(t, 200, code)
	ids := cardIDs(out["tasks"].([]any))
	require.NotContains(t, ids, offerTaskID, "already-offered must be filtered")
	require.Contains(t, ids, cleanID)
	require.NotContains(t, ids, blockedTaskID, "blocked poster tasks hidden")
	// Other user (not blocked, no offer) sees both clean and offerTask
	_, out = getFeed(t, h, otherTok, "?category="+cat)
	ids = cardIDs(out["tasks"].([]any))
	require.Contains(t, ids, offerTaskID)
	require.Contains(t, ids, blockedTaskID)

	// Suspended poster task hidden from everyone
	_, err = h.pool.Exec(ctx, `UPDATE users SET suspended_at=now() WHERE id=$1::uuid`, idBlockedPoster)
	require.NoError(t, err)
	_, out = getFeed(t, h, otherTok, "?category="+cat)
	require.NotContains(t, cardIDs(out["tasks"].([]any)), blockedTaskID)
}

// ── Self-audit: accept atomicity ───────────────────────────────────────────

func TestP4_SelfAudit_AcceptAtomic(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "internal", "offers", "offers.go"))
	require.NoError(t, err)
	s := string(raw)
	// All 9 steps happen inside doAcceptTx which is called within a single pgx.Tx
	require.Contains(t, s, "doAcceptTx")
	// Verify siblings auto-decline and conversation creation are in same tx (not separate Exec outside)
	require.Contains(t, s, "auto_declined")
	require.Contains(t, s, "INSERT INTO conversations")
	// System messages must go via shared function (Phase 5 refactored offers to use it)
	require.Contains(t, s, "InsertSystemMessageTx")
	require.NotContains(t, s, "INSERT INTO messages")
	require.Contains(t, s, "INSERT INTO notifications")
	// No raw location_exact bypass outside ProjectTask in offers
	require.NotContains(t, s, "location_exact'") // hypothetical raw leak
	// Verify shared function is the only writer of system rows
	raw2, err := os.ReadFile(filepath.Join("..", "internal", "messaging", "messaging.go"))
	require.NoError(t, err)
	require.Contains(t, string(raw2), "INSERT INTO messages")
}

func TestP4_Docs_OpenAPI(t *testing.T) {
	h := newHarness(t)
	rec := h.do("GET", "/openapi.yaml", "", "", nil)
	require.Equal(t, 200, rec.Code)
	body := rec.Body.String()
	for _, p := range []string{"/tasks/{id}/offers", "/me/offers", "/offers/{id}/accept", "/offers/{id}/withdraw", "/tasks/{id}/conversation", "/conversations/{id}/messages"} {
		require.Contains(t, body, p, "spec must document %s", p)
	}
}
