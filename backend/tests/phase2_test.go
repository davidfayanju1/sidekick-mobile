// Phase 2 Testing Gate — task creation + minimal escrow hold.
// Covers all 18 gates plus the §3.5 self-audit (shared projection).
package tests

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

		"github.com/sidekick/backend/internal/payments"
	"github.com/sidekick/backend/internal/tasks"
)

// ── helpers ────────────────────────────────────────────────────────────────

func validTaskInput() map[string]any {
	return map[string]any{
		"title":           "Assemble my flat-pack wardrobe quickly",
		"description":     "IKEA PAX wardrobe, two doors, needs assembly in the bedroom. Tools provided, parking available outside.",
		"category":        "assembly",
		"location_approx": "Peckham",
		"location_lat":    51.4741, "location_lng": -0.0697,
		"location_exact":  "14 Rye Lane, London SE15",
		"location_exact_lat": 51.4749, "location_exact_lng": -0.0685,
		"timing_type":     "asap",
		"budget":          4000,
	}
}

func createTask(t *testing.T, h *harness, token string, in map[string]any) map[string]any {
	t.Helper()
	rec := h.do("POST", "/tasks", token, "", in)
	require.Equal(t, 201, rec.Code, rec.Body.String())
	return decodeBody(t, rec)["task"].(map[string]any)
}

func fundTask(t *testing.T, h *harness, token, taskID, key string) *httptest.ResponseRecorder {
	t.Helper()
	return h.do("POST", "/tasks/"+taskID+"/fund", token, "", map[string]any{
		"payment_method_ref": "tok_test_visa_4242", "idempotency_key": key,
	})
}

func uploadPhoto(t *testing.T, h *harness, token, taskID string, img []byte) *httptest.ResponseRecorder {
	t.Helper()
	rec := h.do("POST", "/tasks/"+taskID+"/photos/grant", token, "", map[string]any{"content_type": "image/jpeg"})
	require.Equal(t, 200, rec.Code, rec.Body.String())
	g := decodeBody(t, rec)
	return h.do("POST", "/tasks/"+taskID+"/photos/complete", token, "", map[string]any{
		"object": g["object"], "signature": g["signature"],
		"expires_at": g["expires_at"], "image_base64": base64.StdEncoding.EncodeToString(img),
	})
}

// ── Gate 1: draft create / save / resume / isolation ───────────────────────

func TestP2_Gate1_Drafts(t *testing.T) {
	h := newHarness(t)
	_, tokA, _ := h.signup(t, uniq("p2a1")+"@example.com", "password123", "Poster A")
	_, tokB, _ := h.signup(t, uniq("p2b1")+"@example.com", "password123", "Poster B")

	created := createTask(t, h, tokA, validTaskInput())
	taskID := created["id"].(string)
	require.Equal(t, "draft", created["status"])

	payload := map[string]any{"step": "details", "title": "Wardrobe job"}
	rec := h.do("PUT", "/tasks/"+taskID+"/draft", tokA, "", map[string]any{"draft": payload})
	require.Equal(t, 200, rec.Code, rec.Body.String())
	// A pre-encoded string is rejected (would double-encode in JSONB).
	rec = h.do("PUT", "/tasks/"+taskID+"/draft", tokA, "", map[string]any{"draft": `{"step":"x"}`})
	require.Equal(t, 400, rec.Code)

	// Resume: poster sees payload; drafts list shows it; B sees neither.
	rec = h.do("GET", "/tasks/"+taskID, tokA, "", nil)
	require.Equal(t, 200, rec.Code)
	gotPayload := recBody(t, rec)["task"].(map[string]any)["draft_payload"]
	require.Equal(t, payload, gotPayload)
	rec = h.do("GET", "/me/drafts", tokA, "", nil)
	require.Contains(t, rec.Body.String(), taskID)
	rec = h.do("GET", "/me/drafts", tokB, "", nil)
	require.NotContains(t, rec.Body.String(), taskID)
	rec = h.do("GET", "/tasks/"+taskID, tokB, "", nil)
	require.Equal(t, 404, rec.Code, "stranger must not enumerate drafts")
}

func recBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	return decodeBody(t, rec)
}

// ── Gate 2: 5 listing photos ok, 6th rejected ──────────────────────────────

func TestP2_Gate2_PhotoCap(t *testing.T) {
	h := newHarness(t)
	_, tok, _ := h.signup(t, uniq("p2a2")+"@example.com", "password123", "Photo User")
	taskID := createTask(t, h, tok, validTaskInput())["id"].(string)
	img := testJPEG(t, 100, 100)
	for i := 0; i < 5; i++ {
		require.Equal(t, 201, uploadPhoto(t, h, tok, taskID, img).Code, "photo %d", i+1)
	}
	rec := uploadPhoto(t, h, tok, taskID, img)
	require.Equal(t, 403, rec.Code, "6th listing photo must be rejected server-side")
	// Bad type at grant; oversize at complete.
	rec = h.do("POST", "/tasks/"+taskID+"/photos/grant", tok, "", map[string]any{"content_type": "application/pdf"})
	require.Equal(t, 400, rec.Code)
}

// ── Gate 3: timing coherence ───────────────────────────────────────────────

func TestP2_Gate3_Timing(t *testing.T) {
	h := newHarness(t)
	_, tok, _ := h.signup(t, uniq("p2a3")+"@example.com", "password123", "Timing User")

	past := validTaskInput()
	past["timing_type"] = "specific_date"
	past["scheduled_for"] = time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	require.Equal(t, 400, h.do("POST", "/tasks", tok, "", past).Code)

	bad := validTaskInput()
	bad["timing_type"] = "flexible_range"
	from, to := "2026-12-10", "2026-12-01"
	bad["flexible_from"], bad["flexible_to"] = from, to
	require.Equal(t, 400, h.do("POST", "/tasks", tok, "", bad).Code)

	asapBad := validTaskInput()
	asapBad["scheduled_for"] = time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	require.Equal(t, 400, h.do("POST", "/tasks", tok, "", asapBad).Code)

	future := time.Now().Add(48 * time.Hour).Format(time.RFC3339)
	okDate := validTaskInput()
	okDate["timing_type"] = "specific_date"
	okDate["scheduled_for"] = future
	require.Equal(t, 201, h.do("POST", "/tasks", tok, "", okDate).Code)
	okFlex := validTaskInput()
	okFlex["timing_type"] = "flexible_range"
	f, to2 := "2026-12-01", "2026-12-10"
	okFlex["flexible_from"], okFlex["flexible_to"] = f, to2
	require.Equal(t, 201, h.do("POST", "/tasks", tok, "", okFlex).Code)

	// Title/description length guards.
	short := validTaskInput()
	short["title"] = "Too short"
	require.Equal(t, 400, h.do("POST", "/tasks", tok, "", short).Code)
}

// ── Gate 4: budget bounds ──────────────────────────────────────────────────

func TestP2_Gate4_Budget(t *testing.T) {
	h := newHarness(t)
	_, tok, _ := h.signup(t, uniq("p2a4")+"@example.com", "password123", "Budget User")
	lo := validTaskInput()
	lo["budget"] = 100 // below £5 min
	require.Equal(t, 400, h.do("POST", "/tasks", tok, "", lo).Code)
	hi := validTaskInput()
	hi["budget"] = 99999999
	require.Equal(t, 400, h.do("POST", "/tasks", tok, "", hi).Code)
	rec := h.do("GET", "/fees/quote?budget=10", tok, "", nil)
	require.Equal(t, 400, rec.Code)
}

// ── Gate 5: fee quote correctness + consistency ────────────────────────────

func TestP2_Gate5_Quote(t *testing.T) {
	h := newHarness(t)
	_, tok, _ := h.signup(t, uniq("p2a5")+"@example.com", "password123", "Quote User")
	rec := h.do("GET", "/fees/quote?budget=5000", tok, "", nil)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	q := decodeBody(t, rec)["quote"].(map[string]any)
	// fee_percent=8 poster-side: fee 400, total 5400.
	require.Equal(t, float64(5000), q["budget"])
	require.Equal(t, float64(400), q["fee"])
	require.Equal(t, float64(5400), q["total"])
	require.Equal(t, "poster", q["fee_payer"])

	// Consistency: funded task snapshots the same numbers.
	in := validTaskInput()
	in["budget"] = 5000
	taskID := createTask(t, h, tok, in)["id"].(string)
	require.Equal(t, 200, fundTask(t, h, tok, taskID, "q-"+uniq("k")).Code)
	rec = h.do("GET", "/tasks/"+taskID, tok, "", nil)
	got := decodeBody(t, rec)["task"].(map[string]any)
	require.Equal(t, float64(400), got["platform_fee"])
	require.Equal(t, float64(5400), got["total_charge"])
}

// ── Gate 6: fund success → open/secured + one ledger row ───────────────────

func TestP2_Gate6_FundSuccess(t *testing.T) {
	h := newHarness(t)
	_, tok, _ := h.signup(t, uniq("p2a6")+"@example.com", "password123", "Fund User")
	taskID := createTask(t, h, tok, validTaskInput())["id"].(string)

	rec := fundTask(t, h, tok, taskID, "fund-"+uniq("k"))
	require.Equal(t, 200, rec.Code, rec.Body.String())
	fund := decodeBody(t, rec)["fund"].(map[string]any)
	require.Equal(t, "open", fund["status"])
	require.Equal(t, "secured", fund["escrow_status"])
	require.NotEmpty(t, fund["transaction_id"])

	rec = h.do("GET", "/tasks/"+taskID, tok, "", nil)
	require.Equal(t, "open", decodeBody(t, rec)["task"].(map[string]any)["status"])

	ctx := context.Background()
	var n int
	require.NoError(t, h.pool.QueryRow(ctx,
		`SELECT count(*) FROM transactions WHERE task_id=$1::uuid AND type='escrow_hold' AND status='secured'`,
		taskID).Scan(&n))
	require.Equal(t, 1, n, "exactly one hold row")
	var escStatus, escTx string
	require.NoError(t, h.pool.QueryRow(ctx,
		`SELECT escrow_status, escrow_transaction_id::text FROM tasks WHERE id=$1::uuid`, taskID).
		Scan(&escStatus, &escTx))
	require.Equal(t, "secured", escStatus)
	require.Equal(t, fund["transaction_id"], escTx, "denormalized fields must agree with ledger")

	// Escrow view: masked ref, correct amounts.
	rec = h.do("GET", "/tasks/"+taskID+"/escrow", tok, "", nil)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	esc := decodeBody(t, rec)["escrow"].(map[string]any)
	require.Equal(t, "secured", esc["status"])
	require.NotContains(t, esc["provider_ref_masked"].(string), "mock_hold_fund",
		"raw provider ref must never be returned")
}

// ── Gate 7: failed funding keeps draft + payload, retryable ────────────────

func TestP2_Gate7_FundFailure(t *testing.T) {
	h := newHarness(t)
	srv := h.api
	_, tok, _ := h.signup(t, uniq("p2a7")+"@example.com", "password123", "Fail User")
	taskID := createTask(t, h, tok, validTaskInput())["id"].(string)
	payload := map[string]any{"step": "budget"}
	require.Equal(t, 200, h.do("PUT", "/tasks/"+taskID+"/draft", tok, "", map[string]any{"draft": payload}).Code)

	failMock := payments.NewMock()
	failMock.FailOn = map[string]error{"hold": fmt.Errorf("card declined")}
	srv.SetPaymentsAdapter(failMock)
	rec := fundTask(t, h, tok, taskID, "fail-"+uniq("k"))
	require.Equal(t, 502, rec.Code)
	require.Contains(t, rec.Body.String(), "payment_failed")
	srv.SetPaymentsAdapter(payments.NewMock())

	rec = h.do("GET", "/tasks/"+taskID, tok, "", nil)
	got := decodeBody(t, rec)["task"].(map[string]any)
	require.Equal(t, "draft", got["status"], "failed fund must not half-open the task")
	require.Equal(t, payload, got["draft_payload"])

	var n int
	require.NoError(t, h.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM transactions WHERE task_id=$1::uuid`, taskID).Scan(&n))
	require.Zero(t, n, "no ledger row on failed fund")

	// Retry succeeds.
	require.Equal(t, 200, fundTask(t, h, tok, taskID, "fail-"+uniq("k")).Code)
}

// ── Gate 8: idempotent retry charges once ──────────────────────────────────

func TestP2_Gate8_IdempotentFund(t *testing.T) {
	h := newHarness(t)
	srv := h.api
	mock := payments.NewMock()
	srv.SetPaymentsAdapter(mock)
	_, tok, _ := h.signup(t, uniq("p2a8")+"@example.com", "password123", "Idem User")
	taskID := createTask(t, h, tok, validTaskInput())["id"].(string)
	key := "idem-" + uniq("k")

	first := fundTask(t, h, tok, taskID, key)
	require.Equal(t, 200, first.Code, first.Body.String())
	second := fundTask(t, h, tok, taskID, key)
	require.Equal(t, 200, second.Code)
	firstBody := decodeBody(t, first)["fund"].(map[string]any)
	secondBody := decodeBody(t, second)["fund"].(map[string]any)
	require.Equal(t, true, secondBody["repeated"])
	require.Equal(t, firstBody["transaction_id"], secondBody["transaction_id"])

	holds := 0
	for _, c := range mock.Calls {
		if c == "hold" {
			holds++
		}
	}
	require.Equal(t, 1, holds, "provider hold must run once")
	var n int
	require.NoError(t, h.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM transactions WHERE task_id=$1::uuid`, taskID).Scan(&n))
	require.Equal(t, 1, n)
}

// ── Gates 9/15: edit rules ────────────────────────────────────────────────

func TestP2_Gate9_EditRules(t *testing.T) {
	h := newHarness(t)
	_, tokA, _ := h.signup(t, uniq("p2a9")+"@example.com", "password123", "Editor A")
	_, tokB, _ := h.signup(t, uniq("p2b9")+"@example.com", "password123", "Editor B")

	// Draft editable (incl. budget).
	draftID := createTask(t, h, tokA, validTaskInput())["id"].(string)
	edit := validTaskInput()
	edit["title"] = "Updated wardrobe assembly task title"
	edit["budget"] = 4500
	rec := h.do("PATCH", "/tasks/"+draftID, tokA, "", edit)
	require.Equal(t, 200, rec.Code, rec.Body.String())

	// Fund → open/0 offers: title editable, budget locked.
	require.Equal(t, 200, fundTask(t, h, tokA, draftID, "edit-"+uniq("k")).Code)
	edit["budget"] = 4500 // funded value: title-only change is allowed
	rec = h.do("PATCH", "/tasks/"+draftID, tokA, "", edit)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	editLocked := validTaskInput()
	editLocked["title"] = "Trying to change the funded budget here"
	editLocked["budget"] = 9999
	rec = h.do("PATCH", "/tasks/"+draftID, tokA, "", editLocked)
	require.Equal(t, 409, rec.Code, "funded budget is locked")

	// Simulate Phase 4 offers: offer_count > 0 blocks edit.
	_, err := h.pool.Exec(context.Background(),
		`UPDATE tasks SET offer_count=1 WHERE id=$1::uuid`, draftID)
	require.NoError(t, err)
	rec = h.do("PATCH", "/tasks/"+draftID, tokA, "", edit)
	require.Equal(t, 409, rec.Code, "edit with offers must be rejected, not silent")

	// Stranger cannot edit.
	rec = h.do("PATCH", "/tasks/"+draftID, tokB, "", edit)
	require.Equal(t, 403, rec.Code)
}

// ── Gate 10: cancel funded → refund + cancelled_by_poster ──────────────────

func TestP2_Gate10_CancelRefund(t *testing.T) {
	h := newHarness(t)
	_, tok, _ := h.signup(t, uniq("p2a10")+"@example.com", "password123", "Cancel User")
	taskID := createTask(t, h, tok, validTaskInput())["id"].(string)
	require.Equal(t, 200, fundTask(t, h, tok, taskID, "cx-"+uniq("k")).Code)

	rec := h.do("DELETE", "/tasks/"+taskID, tok, "", nil)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	got := decodeBody(t, rec)["task"].(map[string]any)
	require.Equal(t, "cancelled_by_poster", got["status"])
	require.Equal(t, "refunded", got["escrow_status"])

	ctx := context.Background()
	var n int
	require.NoError(t, h.pool.QueryRow(ctx,
		`SELECT count(*) FROM transactions WHERE task_id=$1::uuid AND type='refund' AND status='succeeded'`,
		taskID).Scan(&n))
	require.Equal(t, 1, n, "exactly one refund row")

	// Cancel twice → conflict, not a second refund.
	rec = h.do("DELETE", "/tasks/"+taskID, tok, "", nil)
	require.Equal(t, 409, rec.Code)
	require.NoError(t, h.pool.QueryRow(ctx,
		`SELECT count(*) FROM transactions WHERE task_id=$1::uuid AND type='refund'`, taskID).Scan(&n))
	require.Equal(t, 1, n)

	// Unfunded draft cancel: no refund row, still cancelled.
	draftID := createTask(t, h, tok, validTaskInput())["id"].(string)
	require.Equal(t, 200, h.do("DELETE", "/tasks/"+draftID, tok, "", nil).Code)
	require.NoError(t, h.pool.QueryRow(ctx,
		`SELECT count(*) FROM transactions WHERE task_id=$1::uuid`, draftID).Scan(&n))
	require.Zero(t, n)
}

// ── Gates 11/12: location split — strangers never, poster sees own ─────────

func TestP2_Gate11_SplitMatrix(t *testing.T) {
	h := newHarness(t)
	_, tokPoster, _ := h.signup(t, uniq("p2pa")+"@example.com", "password123", "Split Poster")
	_, tokStranger, _ := h.signup(t, uniq("p2st")+"@example.com", "password123", "Split Stranger")

	openID := createTask(t, h, tokPoster, validTaskInput())["id"].(string)
	require.Equal(t, 200, fundTask(t, h, tokPoster, openID, "sp-"+uniq("k")).Code)
	cxID := createTask(t, h, tokPoster, validTaskInput())["id"].(string)
	require.Equal(t, 200, fundTask(t, h, tokPoster, cxID, "sp-"+uniq("k")).Code)
	require.Equal(t, 200, h.do("DELETE", "/tasks/"+cxID, tokPoster, "", nil).Code)

	for _, id := range []string{openID, cxID} {
		rec := h.do("GET", "/tasks/"+id, tokStranger, "", nil)
		require.Equal(t, 200, rec.Code)
		body := decodeBody(t, rec)["task"].(map[string]any)
		for _, k := range []string{"location_exact", "location_exact_lat", "location_exact_lng"} {
			_, found := body[k]
			require.False(t, found, "stranger must never receive %s (task %s)", k, id)
		}
		require.Equal(t, "Peckham", body["location_approx"])
	}

	// Poster sees their own exact on the open, unassigned task.
	rec := h.do("GET", "/tasks/"+openID, tokPoster, "", nil)
	body := decodeBody(t, rec)["task"].(map[string]any)
	require.Equal(t, "14 Rye Lane, London SE15", body["location_exact"])
}

// Unit-level: projection across every status × role combination.
func TestP2_SplitAllStatuses(t *testing.T) {
	mk := func(status string) *tasks.Task {
		w := "worker-id"
		return &tasks.Task{ID: "t", PosterID: "poster", Title: "x", Status: status,
			AssignedWorkerID: &w, LocationApprox: "A", LocationExact: strPtr("EXACT")}
	}
	statuses := []string{"draft", "open", "assigned", "completed_pending_confirmation",
		"completed", "disputed", "resolved_released", "resolved_refunded", "resolved_split",
		"cancelled_by_poster", "cancelled_by_worker", "expired"}
	for _, st := range statuses {
		task := mk(st)
		poster := tasks.ProjectTask("poster", task, nil)
		stranger := tasks.ProjectTask("stranger", task, nil)
		worker := tasks.ProjectTask("worker-id", task, nil)
		_, strangerHas := stranger["location_exact"]
		require.False(t, strangerHas, "stranger must never see exact (status %s)", st)
		should := tasks.ExactVisibleStatuses[st]
		_, posterHas := poster["location_exact"]
		_, workerHas := worker["location_exact"]
		// Poster always sees their own exact address (gate 12); the assigned
		// worker only from assignment on; strangers never.
		require.True(t, posterHas, "poster always sees exact (status %s)", st)
		require.Equal(t, should, workerHas, "worker exact (status %s)", st)
	}
}

func strPtr(s string) *string { return &s }

// ── Gates 13/14: RLS rejects raw status/ledger writes ─────────────────────

func TestP2_Gate13_RLSWrites(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	_, tok, _ := h.signup(t, uniq("p2a13")+"@example.com", "password123", "RLS User")
	taskID := createTask(t, h, tok, validTaskInput())["id"].(string)

	conn, err := h.pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, "SET ROLE phase0_restricted")
	require.NoError(t, err)
	defer func() { _, _ = conn.Exec(context.Background(), "RESET ROLE") }()

	// RLS semantics: filtered UPDATE/DELETE succeed with ZERO rows affected
	// (never an error) — the write is silently ineffective. INSERT without a
	// policy is a hard error. Both prove the write never lands.
	tag, err := conn.Exec(ctx, `UPDATE tasks SET status='open' WHERE id=$1::uuid`, taskID)
	require.NoError(t, err)
	require.Zero(t, tag.RowsAffected(), "raw status write must affect zero rows under RLS")
	_, err = conn.Exec(ctx,
		`INSERT INTO transactions (task_id, amount, type, status) VALUES ($1::uuid, 100, 'release', 'succeeded')`, taskID)
	require.Error(t, err, "raw ledger insert must be rejected by RLS")
	tag, err = conn.Exec(ctx, `UPDATE transactions SET status='succeeded' WHERE task_id=$1::uuid`, taskID)
	require.NoError(t, err)
	require.Zero(t, tag.RowsAffected(), "raw ledger update must affect zero rows under RLS")

	// Confirm nothing changed via the owner connection.
	var status string
	require.NoError(t, h.pool.QueryRow(ctx, `SELECT status FROM tasks WHERE id=$1::uuid`, taskID).Scan(&status))
	require.Equal(t, "draft", status)
}

// ── Gate 16: cannot fund/cancel others' tasks ──────────────────────────────

func TestP2_Gate16_Ownership(t *testing.T) {
	h := newHarness(t)
	_, tokA, _ := h.signup(t, uniq("p2a16")+"@example.com", "password123", "Owner User")
	_, tokB, _ := h.signup(t, uniq("p2b16")+"@example.com", "password123", "Other User")
	taskID := createTask(t, h, tokA, validTaskInput())["id"].(string)

	require.Equal(t, 403, fundTask(t, h, tokB, taskID, "no-"+uniq("k")).Code)
	require.Equal(t, 403, h.do("DELETE", "/tasks/"+taskID, tokB, "", nil).Code)
	require.Equal(t, 403, h.do("GET", "/tasks/"+taskID+"/escrow", tokB, "", nil).Code)
}

// ── Gate 17: concurrent funds, different keys — one hold ───────────────────

func TestP2_Gate17_ConcurrentFund(t *testing.T) {
	h := newHarness(t)
	_, tok, _ := h.signup(t, uniq("p2a17")+"@example.com", "password123", "Race User")
	taskID := createTask(t, h, tok, validTaskInput())["id"].(string)

	var wg sync.WaitGroup
	codes := make([]int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i] = fundTask(t, h, tok, taskID, "race-"+uniq("k")).Code
		}(i)
	}
	wg.Wait()
	succeeded := 0
	for _, c := range codes {
		if c == 200 {
			succeeded++
		}
	}
	require.Equal(t, 1, succeeded, "exactly one concurrent fund wins (got %v)", codes)
	var n int
	require.NoError(t, h.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM transactions WHERE task_id=$1::uuid AND type='escrow_hold'`, taskID).Scan(&n))
	require.Equal(t, 1, n)
}

// ── Gate 18: no raw card data in the database ──────────────────────────────

func TestP2_Gate18_NoRawCardData(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	// Schema must not even have raw-card columns.
	var n int
	require.NoError(t, h.pool.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.columns
		  WHERE table_schema='public' AND column_name IN
		  ('card_number','pan','cvv','cvc','card_cvv','expiry','card_expiry')`).Scan(&n))
	require.Zero(t, n, "no raw-card columns may exist")

	// Stored refs are tokenized; no 13–19 digit PAN-like runs anywhere.
	_, tok, _ := h.signup(t, uniq("p2a18")+"@example.com", "password123", "Card User")
	taskID := createTask(t, h, tok, validTaskInput())["id"].(string)
	require.Equal(t, 200, fundTask(t, h, tok, taskID, "card-"+uniq("k")).Code)
	rows, err := h.pool.Query(ctx, `SELECT provider_ref FROM transactions WHERE task_id=$1::uuid`, taskID)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var ref *string
		require.NoError(t, rows.Scan(&ref))
		require.NotNil(t, ref)
		require.True(t, strings.HasPrefix(*ref, "mock_") || strings.HasPrefix(*ref, "tok_"),
			"provider_ref must be tokenized, got %q", *ref)
		require.NotRegexp(t, `\d{13,19}`, *ref, "no PAN-like digit runs")
	}
}

// ── Self-audit: all task reads route through ProjectTask ───────────────────

func TestP2_SelfAudit_ProjectionCentralized(t *testing.T) {
	root, err := filepath.Abs("..")
	require.NoError(t, err)
	var offenders []string
	err = filepath.Walk(filepath.Join(root, "internal", "httpapi"), func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") {
			return nil
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		for _, line := range strings.Split(string(raw), "\n") {
			trim := strings.TrimSpace(line)
			if strings.HasPrefix(trim, "//") {
				continue
			}
			if strings.Contains(line, "location_exact") {
				offenders = append(offenders, filepath.Base(p)+": "+trim)
			}
		}
		return nil
	})
	require.NoError(t, err)
	require.Empty(t, offenders,
		"no httpapi handler may touch location_exact — ProjectTask owns the split")
}
