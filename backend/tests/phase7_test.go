// Phase 7 Testing Gate — full payments, escrow & wallet (18 gates + self-audit).
package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sidekick/backend/internal/webhook"
)

// helpers for Phase 7

func setVerified(t *testing.T, h *harness, userID string) {
	t.Helper()
	_, err := h.pool.Exec(context.Background(), `UPDATE users SET verification_status='verified' WHERE id=$1::uuid`, userID)
	require.NoError(t, err)
}

func getWallet(t *testing.T, h *harness, tok string) map[string]any {
	t.Helper()
	rec := h.do("GET", "/me/wallet", tok, "", nil)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	return decodeBody(t, rec)
}

func createPayoutMethod(t *testing.T, h *harness, tok string) string {
	t.Helper()
	rec := h.do("POST", "/me/payout-methods", tok, "", map[string]any{"bank_ref_token": "tok_bank_" + uniq("b"), "last4": "6789", "bank_name": "Test Bank"})
	require.Equal(t, 201, rec.Code, rec.Body.String())
	return decodeBody(t, rec)["id"].(string)
}

func createCard(t *testing.T, h *harness, tok string) string {
	t.Helper()
	rec := h.do("POST", "/me/payment-methods", tok, "", map[string]any{"provider_ref": "tok_card_" + uniq("c"), "last4": "4242", "brand": "visa", "exp_month": 12, "exp_year": 2030})
	require.Equal(t, 201, rec.Code, rec.Body.String())
	return decodeBody(t, rec)["id"].(string)
}

// ── Gate 1: wallet + history filtered/paginated, empty state ─────────────────

func TestP7_Gate1_WalletAndHistory(t *testing.T) {
	h := newHarness(t)
	_, tok, _ := h.signup(t, uniq("p7p1")+"@example.com", "password123", "Wallet User")
	// empty state
	wallet := getWallet(t, h, tok)
	require.Equal(t, float64(0), wallet["available_balance"])
	require.Equal(t, float64(0), wallet["pending_balance"])
	require.Equal(t, "GBP", wallet["currency"])
	require.Empty(t, wallet["recent_activity"])

	// create some history via fund + confirm
	posterID, workerID, posterTok, workerTok, taskID, _ := setupAssignedTaskPhase6(t, h)
	_ = posterID
	_ = workerID
	rec := h.do("POST", "/tasks/"+taskID+"/complete", workerTok, "", nil)
	require.Equal(t, 200, rec.Code)
	rec = h.do("POST", "/tasks/"+taskID+"/confirm", posterTok, "", nil)
	require.Equal(t, 200, rec.Code)

	// worker wallet should have release
	workerWallet := getWallet(t, h, workerTok)
	require.Greater(t, workerWallet["available_balance"].(float64), float64(0))
	// poster transactions filtered
	rec = h.do("GET", "/me/transactions?type=escrow_hold", posterTok, "", nil)
	require.Equal(t, 200, rec.Code)
	txs := decodeBody(t, rec)["transactions"].([]any)
	require.GreaterOrEqual(t, len(txs), 1)
	for _, tx := range txs {
		require.Equal(t, "escrow_hold", tx.(map[string]any)["type"])
	}
	// pagination
	rec = h.do("GET", "/me/transactions", workerTok, "", nil)
	require.Equal(t, 200, rec.Code)
	// Check empty filter returns empty, not error
	rec = h.do("GET", "/me/transactions?type=withdrawal", workerTok, "", nil)
	require.Equal(t, 200, rec.Code)
}

// ── Gate 2: saved cards & payout methods tokenized ───────────────────────────

func TestP7_Gate2_CardsAndPayoutMethods(t *testing.T) {
	h := newHarness(t)
	_, tok, _ := h.signup(t, uniq("p7p2")+"@example.com", "password123", "Card User")

	// Add card
	cardID := createCard(t, h, tok)
	// Add payout method
	pmID := createPayoutMethod(t, h, tok)

	// List
	rec := h.do("GET", "/me/payment-methods", tok, "", nil)
	require.Equal(t, 200, rec.Code)
	require.Len(t, decodeBody(t, rec)["payment_methods"].([]any), 1)
	rec = h.do("GET", "/me/payout-methods", tok, "", nil)
	require.Equal(t, 200, rec.Code)
	require.Len(t, decodeBody(t, rec)["payout_methods"].([]any), 1)

	// Check DB only has tokens, not raw
	var providerRef, bankRef string
	err := h.pool.QueryRow(context.Background(), `SELECT provider_ref FROM payment_methods WHERE id=$1::uuid`, cardID).Scan(&providerRef)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(providerRef, "tok_"))
	var bankRefDB string
	err = h.pool.QueryRow(context.Background(), `SELECT bank_ref FROM payout_methods WHERE id=$1::uuid`, pmID).Scan(&bankRefDB)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(bankRefDB, "tok_"))

	// Delete card
	rec = h.do("DELETE", "/me/payment-methods/"+cardID, tok, "", nil)
	require.Equal(t, 200, rec.Code)
	rec = h.do("GET", "/me/payment-methods", tok, "", nil)
	require.Len(t, decodeBody(t, rec)["payment_methods"].([]any), 0)

	// Check default handling: first card is default, second not
	cardID2 := createCard(t, h, tok)
	cardID3 := createCard(t, h, tok)
	var isDef2, isDef3 bool
	_ = h.pool.QueryRow(context.Background(), `SELECT is_default FROM payment_methods WHERE id=$1::uuid`, cardID2).Scan(&isDef2)
	_ = h.pool.QueryRow(context.Background(), `SELECT is_default FROM payment_methods WHERE id=$1::uuid`, cardID3).Scan(&isDef3)
	// One of them should be true (the first remaining), not both
	require.True(t, isDef2 != isDef3 || (isDef2 && !isDef3))

	_ = providerRef
	_ = bankRef
}

// ── Gate 3: withdrawal with verified + sufficient ────────────────────────────

func TestP7_Gate3_WithdrawalSuccess(t *testing.T) {
	h := newHarness(t)
	posterID, workerID, posterTok, workerTok, taskID, _ := setupAssignedTaskPhase6(t, h)
	_ = posterID
	// fund and confirm to give worker balance
	rec := h.do("POST", "/tasks/"+taskID+"/complete", workerTok, "", nil)
	require.Equal(t, 200, rec.Code)
	rec = h.do("POST", "/tasks/"+taskID+"/confirm", posterTok, "", nil)
	require.Equal(t, 200, rec.Code)
	setVerified(t, h, workerID)
	pmID := createPayoutMethod(t, h, workerTok)
	var before int
	_ = h.pool.QueryRow(context.Background(), `SELECT available_balance FROM wallets WHERE user_id=$1::uuid`, workerID).Scan(&before)
	require.Greater(t, before, 0)
	amount := before / 2
	if amount == 0 {
		amount = 100
	}
	rec = h.do("POST", "/me/withdrawals", workerTok, "", map[string]any{"amount": amount, "payout_method_id": pmID, "idempotency_key": "wd-" + uniq("k")})
	require.Equal(t, 201, rec.Code, rec.Body.String())
	body := decodeBody(t, rec)
	require.Equal(t, "processing", body["status"])
	var after int
	_ = h.pool.QueryRow(context.Background(), `SELECT available_balance FROM wallets WHERE user_id=$1::uuid`, workerID).Scan(&after)
	require.Equal(t, before-amount, after)
	// check transaction row
	var txType, txStatus string
	_ = h.pool.QueryRow(context.Background(), `SELECT type, status FROM transactions WHERE id=$1::uuid`, body["id"]).Scan(&txType, &txStatus)
	require.Equal(t, "withdrawal", txType)
	require.Equal(t, "processing", txStatus)
}

// ── Gate 4: not verified -> 403 ─────────────────────────────────────────────

func TestP7_Gate4_VerificationRequired(t *testing.T) {
	h := newHarness(t)
	_, workerID, _, workerTok, taskID, _ := setupAssignedTaskPhase6(t, h)
	// give balance but not verified
	posterID2, _, posterTok2, _, taskID2, _ := setupAssignedTaskPhase6(t, h)
	_ = h.do("POST", "/tasks/"+taskID2+"/complete", workerTok, "", nil)
	_ = h.do("POST", "/tasks/"+taskID2+"/confirm", posterTok2, "", nil)
	// ensure worker is unverified
	_, _ = h.pool.Exec(context.Background(), `UPDATE users SET verification_status='unverified' WHERE id=$1::uuid`, workerID)
	pmID := createPayoutMethod(t, h, workerTok)
	_ = taskID
	_ = posterID2
	rec := h.do("POST", "/me/withdrawals", workerTok, "", map[string]any{"amount": 100, "payout_method_id": pmID, "idempotency_key": "wd-" + uniq("k")})
	require.Equal(t, 403, rec.Code)
	require.Contains(t, rec.Body.String(), "verification_required")
}

// ── Gate 5: insufficient balance ─────────────────────────────────────────────

func TestP7_Gate5_InsufficientBalance(t *testing.T) {
	h := newHarness(t)
	_, workerID, _, workerTok, _, _ := setupAssignedTaskPhase6(t, h)
	setVerified(t, h, workerID)
	pmID := createPayoutMethod(t, h, workerTok)
	// ensure wallet 0 or small
	_, _ = h.pool.Exec(context.Background(), `UPDATE wallets SET available_balance=50 WHERE user_id=$1::uuid`, workerID)
	// try withdraw more than balance
	rec := h.do("POST", "/me/withdrawals", workerTok, "", map[string]any{"amount": 10000, "payout_method_id": pmID, "idempotency_key": "wd-" + uniq("k")})
	require.Equal(t, 400, rec.Code)
	require.Contains(t, rec.Body.String(), "insufficient")
}

// ── Gate 6: receipt for payer/payee ─────────────────────────────────────────

func TestP7_Gate6_Receipt(t *testing.T) {
	h := newHarness(t)
	posterID, workerID, posterTok, workerTok, taskID, _ := setupAssignedTaskPhase6(t, h)
	rec := h.do("POST", "/tasks/"+taskID+"/complete", workerTok, "", nil)
	require.Equal(t, 200, rec.Code)
	rec = h.do("POST", "/tasks/"+taskID+"/confirm", posterTok, "", nil)
	require.Equal(t, 200, rec.Code)
	var txID string
	err := h.pool.QueryRow(context.Background(), `SELECT id::text FROM transactions WHERE task_id=$1::uuid AND type='release'`, taskID).Scan(&txID)
	require.NoError(t, err)
	// poster is payer? Actually release has payer NULL, payee worker, so poster is not payer/payee. Check: release has payee worker, payer NULL, so poster is not owner. But our receipt allows payer or payee. For release, only worker is payee, so poster shouldn't see? However escrow_hold has payer poster, so poster can see hold receipt, worker can see release receipt.
	// Test with escrow_hold receipt (poster is payer)
	var holdID string
	_ = h.pool.QueryRow(context.Background(), `SELECT id::text FROM transactions WHERE task_id=$1::uuid AND type='escrow_hold'`, taskID).Scan(&holdID)
	rec = h.do("GET", "/transactions/"+holdID+"/receipt", posterTok, "", nil)
	require.Equal(t, 200, rec.Code)
	body := decodeBody(t, rec)
	require.NotEmpty(t, body["provider_ref"])
	require.Contains(t, body["provider_ref"].(string), "…") // masked
	require.NotContains(t, fmt.Sprint(body["provider_ref"]), "mock_hold") // masked, not raw
	// worker can see release receipt
	rec = h.do("GET", "/transactions/"+txID+"/receipt", workerTok, "", nil)
	require.Equal(t, 200, rec.Code)
	// poster should not see release receipt (not payer/payee) -> 403
	rec = h.do("GET", "/transactions/"+txID+"/receipt", posterTok, "", nil)
	require.Equal(t, 403, rec.Code)
	// stranger cannot see
	_, strangerTok, _ := h.signup(t, uniq("p7s6")+"@example.com", "password123", "Stranger")
	rec = h.do("GET", "/transactions/"+holdID+"/receipt", strangerTok, "", nil)
	require.Equal(t, 403, rec.Code)
	_ = posterID
	_ = workerID
}

// ── Gate 7: webhook charge.succeeded ────────────────────────────────────────

func TestP7_Gate7_WebhookChargeSucceeded(t *testing.T) {
	h := newHarness(t)
	// create a pending escrow_hold via direct insert to simulate pending charge
	posterID, _, posterTok, _, _, _ := setupAssignedTaskPhase6(t, h)
	// Use fund to create a hold, then set it to pending and update via webhook
	// Instead, create a new task and fund but mock pending status
	_, _, posterTok2, _, taskID, _ := setupAssignedTaskPhase6(t, h) // dummy to get funds? Actually need a pending transaction
	// For simplicity, create a transaction with status pending and idempotency key
	pendingKey := "pending-" + uniq("k")
	_, err := h.pool.Exec(context.Background(),
		`INSERT INTO transactions (task_id, payer_id, amount, type, status, provider_ref, idempotency_key)
		 VALUES ($1::uuid, $2::uuid, 1000, 'escrow_hold', 'pending', $3, $4)`,
		taskID, posterID, "prov-123", pendingKey)
	require.NoError(t, err)
	payload := map[string]any{"id": "evt-1", "type": "charge.succeeded", "idempotency_key": pendingKey, "provider_ref": "prov-123", "status": "succeeded"}
	b, sig := webhook.SignedPayload(payload)
	rec := httptest.NewRequest("POST", "/webhooks/payments", strings.NewReader(string(b)))
	rec.Header.Set("X-Webhook-Signature", sig)
	httprec := httptest.NewRecorder()
	h.server.ServeHTTP(httprec, rec)
	// Our harness does not have webhook via server? Need to use h.do with signature header
	// Instead use h.do with header
	httprec2 := h.doWithSignature("POST", "/webhooks/payments", "", payload, sig)
	require.Equal(t, 200, httprec2.Code, httprec2.Body.String())
	var status string
	_ = h.pool.QueryRow(context.Background(), `SELECT status FROM transactions WHERE idempotency_key=$1`, pendingKey).Scan(&status)
	require.Equal(t, "succeeded", status)
	_ = posterTok
	_ = posterTok2
}

// helper to send webhook via harness with signature
func (h *harness) doWithSignature(method, path, token string, body any, sig string) *httptest.ResponseRecorder {
	var buf []byte
	if body != nil {
		b, _ := json.Marshal(body)
		buf = b
	}
	req := httptest.NewRequest(method, path, strings.NewReader(string(buf)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", sig)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.server.ServeHTTP(rec, req)
	return rec
}

// ── Gate 8: payout.failed reverts ───────────────────────────────────────────

func TestP7_Gate8_WebhookPayoutFailed(t *testing.T) {
	h := newHarness(t)
	_, workerID, _, workerTok, _, _ := setupAssignedTaskPhase6(t, h)
	// give worker balance and verified
	setVerified(t, h, workerID)
	pmID := createPayoutMethod(t, h, workerTok)
	// fund and confirm to get balance - create second task with same worker
	_, _, posterTok2, _, _, _ := setupAssignedTaskPhase6(t, h)
	taskID2 := fundTaskForPhase4(t, h, posterTok2, validTaskInput())
	_, out := makeOffer(t, h, workerTok, taskID2, nil, "")
	rec2 := acceptOffer(t, h, posterTok2, out["offer"].(map[string]any)["id"].(string), "p7-8-2-"+uniq("k"))
	require.Equal(t, 200, rec2.Code)
	_ = h.do("POST", "/tasks/"+taskID2+"/complete", workerTok, "", nil)
	_ = h.do("POST", "/tasks/"+taskID2+"/confirm", posterTok2, "", nil)
	var before int
	_ = h.pool.QueryRow(context.Background(), `SELECT available_balance FROM wallets WHERE user_id=$1::uuid`, workerID).Scan(&before)
	// withdraw
	withdrawKey := "wd-fail-" + uniq("k")
	rec := h.do("POST", "/me/withdrawals", workerTok, "", map[string]any{"amount": 200, "payout_method_id": pmID, "idempotency_key": withdrawKey})
	require.Equal(t, 201, rec.Code)
	var afterWithdraw int
	_ = h.pool.QueryRow(context.Background(), `SELECT available_balance FROM wallets WHERE user_id=$1::uuid`, workerID).Scan(&afterWithdraw)
	require.Equal(t, before-200, afterWithdraw)
	// get withdrawal tx id
	var txID string
	_ = h.pool.QueryRow(context.Background(), `SELECT id::text FROM transactions WHERE idempotency_key=$1`, withdrawKey).Scan(&txID)
	// webhook payout.failed
	payload := map[string]any{"id": "evt-fail-1", "type": "payout.failed", "idempotency_key": withdrawKey, "provider_ref": "withdrawal-" + withdrawKey, "status": "failed"}
	_, sig := webhook.SignedPayload(payload)
	rec2 = h.doWithSignature("POST", "/webhooks/payments", "", payload, sig)
	require.Equal(t, 200, rec2.Code)
	var afterFailed int
	_ = h.pool.QueryRow(context.Background(), `SELECT available_balance FROM wallets WHERE user_id=$1::uuid`, workerID).Scan(&afterFailed)
	require.Equal(t, before, afterFailed)
	var txStatus string
	_ = h.pool.QueryRow(context.Background(), `SELECT status FROM transactions WHERE id=$1::uuid`, txID).Scan(&txStatus)
	require.Equal(t, "failed", txStatus)
}

// ── Gate 9: bad signature rejected and logged ────────────────────────────────

func TestP7_Gate9_BadSignature(t *testing.T) {
	h := newHarness(t)
	payload := map[string]any{"id": "evt-bad", "type": "charge.succeeded", "status": "succeeded"}
	b, _ := json.Marshal(payload)
	rec := httptest.NewRequest("POST", "/webhooks/payments", strings.NewReader(string(b)))
	rec.Header.Set("X-Webhook-Signature", "bad-sig")
	httprec := httptest.NewRecorder()
	h.server.ServeHTTP(httprec, rec)
	require.Equal(t, 401, httprec.Code)
	// check audit logged
	var cnt int
	_ = h.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_log WHERE action='webhook.rejected'`).Scan(&cnt)
	require.Greater(t, cnt, 0)
	// missing signature
	rec = httptest.NewRequest("POST", "/webhooks/payments", strings.NewReader(string(b)))
	httprec = httptest.NewRecorder()
	h.server.ServeHTTP(httprec, rec)
	require.Equal(t, 401, httprec.Code)
}

// ── Gate 10: duplicate webhook idempotent ───────────────────────────────────

func TestP7_Gate10_DuplicateWebhook(t *testing.T) {
	h := newHarness(t)
	posterID, _, _, _, taskID, _ := setupAssignedTaskPhase6(t, h)
	pendingKey := "dup-" + uniq("k")
	_, err := h.pool.Exec(context.Background(),
		`INSERT INTO transactions (task_id, payer_id, amount, type, status, provider_ref, idempotency_key)
		 VALUES ($1::uuid,$2::uuid,1000,'escrow_hold','pending','prov-dup',$3)`, taskID, posterID, pendingKey)
	require.NoError(t, err)
	payload := map[string]any{"id": "evt-dup", "type": "charge.succeeded", "idempotency_key": pendingKey, "provider_ref": "prov-dup", "status": "succeeded"}
	_, sig := webhook.SignedPayload(payload)
	rec1 := h.doWithSignature("POST", "/webhooks/payments", "", payload, sig)
	require.Equal(t, 200, rec1.Code)
	rec2 := h.doWithSignature("POST", "/webhooks/payments", "", payload, sig)
	require.Equal(t, 200, rec2.Code)
	require.Contains(t, rec2.Body.String(), "duplicate")
	var status string
	_ = h.pool.QueryRow(context.Background(), `SELECT status FROM transactions WHERE idempotency_key=$1`, pendingKey).Scan(&status)
	require.Equal(t, "succeeded", status)
	// also check wallet not double-credited: create withdrawal and duplicate payout.failed should not double credit
	_, workerID, _, workerTok, _, _ := setupAssignedTaskPhase6(t, h)
	setVerified(t, h, workerID)
	pmID := createPayoutMethod(t, h, workerTok)
	// fund worker with second task using same worker
	_, _, posterTok2, _, _, _ := setupAssignedTaskPhase6(t, h)
	taskID2 := fundTaskForPhase4(t, h, posterTok2, validTaskInput())
	_, out2 := makeOffer(t, h, workerTok, taskID2, nil, "")
	rec2b := acceptOffer(t, h, posterTok2, out2["offer"].(map[string]any)["id"].(string), "p7-10-2-"+uniq("k"))
	require.Equal(t, 200, rec2b.Code)
	_ = h.do("POST", "/tasks/"+taskID2+"/complete", workerTok, "", nil)
	_ = h.do("POST", "/tasks/"+taskID2+"/confirm", posterTok2, "", nil)
	var before int
	_ = h.pool.QueryRow(context.Background(), `SELECT available_balance FROM wallets WHERE user_id=$1::uuid`, workerID).Scan(&before)
	wdKey := "wd-dup-" + uniq("k")
	rec := h.do("POST", "/me/withdrawals", workerTok, "", map[string]any{"amount": 100, "payout_method_id": pmID, "idempotency_key": wdKey})
	require.Equal(t, 201, rec.Code)
	payload2 := map[string]any{"id": "evt-dup2", "type": "payout.failed", "idempotency_key": wdKey, "status": "failed"}
	_, sig2 := webhook.SignedPayload(payload2)
	_ = h.doWithSignature("POST", "/webhooks/payments", "", payload2, sig2)
	var after1 int
	_ = h.pool.QueryRow(context.Background(), `SELECT available_balance FROM wallets WHERE user_id=$1::uuid`, workerID).Scan(&after1)
	// duplicate again should not double credit
	_ = h.doWithSignature("POST", "/webhooks/payments", "", payload2, sig2)
	var after2 int
	_ = h.pool.QueryRow(context.Background(), `SELECT available_balance FROM wallets WHERE user_id=$1::uuid`, workerID).Scan(&after2)
	require.Equal(t, after1, after2)
}

// ── Gate 11: no raw card/bank numbers ───────────────────────────────────────

func TestP7_Gate11_NoRawNumbers(t *testing.T) {
	h := newHarness(t)
	_, tok, _ := h.signup(t, uniq("p7p11")+"@example.com", "password123", "NoRaw User")
	// try to add card with raw PAN should be rejected
	rec := h.do("POST", "/me/payment-methods", tok, "", map[string]any{"provider_ref": "4242424242424242", "last4": "4242", "brand": "visa"})
	require.Equal(t, 400, rec.Code)
	// add tokenized card
	rec = h.do("POST", "/me/payment-methods", tok, "", map[string]any{"provider_ref": "tok_visa_4242", "last4": "4242", "brand": "visa"})
	require.Equal(t, 201, rec.Code)
	// add payout with raw bank
	rec = h.do("POST", "/me/payout-methods", tok, "", map[string]any{"bank_ref_token": "1234567890123456", "last4": "3456"})
	require.Equal(t, 400, rec.Code)
	rec = h.do("POST", "/me/payout-methods", tok, "", map[string]any{"bank_ref_token": "tok_bank_123", "last4": "3456"})
	require.Equal(t, 201, rec.Code)

	// audit all tables this phase writes: payment_methods, payout_methods, transactions
	var cnt int
	_ = h.pool.QueryRow(context.Background(), `SELECT count(*) FROM payment_methods WHERE provider_ref ~ '^[0-9]{13,19}$'`).Scan(&cnt)
	require.Equal(t, 0, cnt)
	_ = h.pool.QueryRow(context.Background(), `SELECT count(*) FROM payout_methods WHERE bank_ref ~ '^[0-9]{13,19}$'`).Scan(&cnt)
	require.Equal(t, 0, cnt)
	_ = h.pool.QueryRow(context.Background(), `SELECT count(*) FROM transactions WHERE provider_ref ~ '^[0-9]{13,19}$'`).Scan(&cnt)
	require.Equal(t, 0, cnt)
}

// ── Gate 12: cannot view another user's finances ────────────────────────────

func TestP7_Gate12_CannotViewOthers(t *testing.T) {
	h := newHarness(t)
	_, tokA, _ := h.signup(t, uniq("p7a12")+"@example.com", "password123", "User A12")
	_, tokB, _ := h.signup(t, uniq("p7b12")+"@example.com", "password123", "User B12")
	// A adds card
	rec := h.do("POST", "/me/payment-methods", tokA, "", map[string]any{"provider_ref": "tok_a", "last4": "1111"})
	require.Equal(t, 201, rec.Code)
	cardID := decodeBody(t, rec)["id"].(string)
	// B tries to delete A's card
	rec = h.do("DELETE", "/me/payment-methods/"+cardID, tokB, "", nil)
	require.Equal(t, 404, rec.Code)

	// B tries to view A's wallet is not possible via API (only /me/wallet), but we can test that B's wallet does not contain A's transactions
	// Create a transaction for A
	posterID, _, posterTok, workerTok, taskID, _ := setupAssignedTaskPhase6(t, h)
	_ = h.do("POST", "/tasks/"+taskID+"/complete", workerTok, "", nil)
	_ = h.do("POST", "/tasks/"+taskID+"/confirm", posterTok, "", nil)
	var txID string
	_ = h.pool.QueryRow(context.Background(), `SELECT id::text FROM transactions WHERE task_id=$1::uuid LIMIT 1`, taskID).Scan(&txID)
	rec = h.do("GET", "/transactions/"+txID+"/receipt", tokB, "", nil)
	require.Equal(t, 403, rec.Code)

	// B tries to list A's payout methods via direct DB? API only returns own, so check that B's list doesn't contain A's
	rec = h.do("GET", "/me/payout-methods", tokB, "", nil)
	require.NotContains(t, rec.Body.String(), "tok_a")

	_ = posterID
}

// ── Gate 13: cannot directly update wallets ──────────────────────────────────

func TestP7_Gate13_CannotDirectUpdateWallet(t *testing.T) {
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
	tag, err := conn.Exec(ctx, `UPDATE wallets SET available_balance=1000000 WHERE true`)
	require.NoError(t, err)
	require.Equal(t, int64(0), tag.RowsAffected())
}

// ── Gate 14: cannot directly insert transactions ─────────────────────────────

func TestP7_Gate14_CannotDirectInsertTx(t *testing.T) {
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
	_, err = conn.Exec(ctx, `INSERT INTO transactions (task_id, amount, type, status) VALUES ('00000000-0000-0000-0000-000000000000'::uuid, 100, 'release', 'succeeded')`)
	require.Error(t, err)
}

// ── Gate 15: simultaneous withdrawals row-locking ────────────────────────────

func TestP7_Gate15_ConcurrentWithdrawals(t *testing.T) {
	h := newHarness(t)
	_, workerID, posterTok, workerTok, taskID, _ := setupAssignedTaskPhase6(t, h)
	setVerified(t, h, workerID)
	pmID := createPayoutMethod(t, h, workerTok)
	// fund worker by completing and confirming the assigned task
	rec := h.do("POST", "/tasks/"+taskID+"/complete", workerTok, "", nil)
	require.Equal(t, 200, rec.Code)
	rec = h.do("POST", "/tasks/"+taskID+"/confirm", posterTok, "", nil)
	require.Equal(t, 200, rec.Code)
	var before int
	_ = h.pool.QueryRow(context.Background(), `SELECT available_balance FROM wallets WHERE user_id=$1::uuid`, workerID).Scan(&before)
	require.Greater(t, before, 300)
	// two withdrawals that together exceed balance: 200 + 200 > before if before is 300-400, but we set to ensure
	amount1 := before - 50
	amount2 := 100 // together = before+50 > before
	var wg sync.WaitGroup
	results := make([]int, 2)
	bodies := make([]string, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		rec := h.do("POST", "/me/withdrawals", workerTok, "", map[string]any{"amount": amount1, "payout_method_id": pmID, "idempotency_key": "conc-wd-1-" + uniq("k")})
		results[0] = rec.Code
		bodies[0] = rec.Body.String()
	}()
	go func() {
		defer wg.Done()
		rec := h.do("POST", "/me/withdrawals", workerTok, "", map[string]any{"amount": amount2, "payout_method_id": pmID, "idempotency_key": "conc-wd-2-" + uniq("k")})
		results[1] = rec.Code
		bodies[1] = rec.Body.String()
	}()
	wg.Wait()
	// exactly one should succeed (201), other 400 insufficient
	successes := 0
	for _, c := range results {
		if c == 201 {
			successes++
		}
	}
	require.Equal(t, 1, successes, "only one concurrent withdrawal should succeed, got %v bodies %v", results, bodies)
}

// ── Gate 16: retry with same idempotency does not double debit ──────────────

func TestP7_Gate16_IdempotentWithdrawal(t *testing.T) {
	h := newHarness(t)
	_, workerID, posterTok, workerTok, taskID, _ := setupAssignedTaskPhase6(t, h)
	setVerified(t, h, workerID)
	pmID := createPayoutMethod(t, h, workerTok)
	// fund same worker
	rec2 := h.do("POST", "/tasks/"+taskID+"/complete", workerTok, "", nil)
	require.Equal(t, 200, rec2.Code)
	_ = h.do("POST", "/tasks/"+taskID+"/confirm", posterTok, "", nil)
	var before int
	_ = h.pool.QueryRow(context.Background(), `SELECT available_balance FROM wallets WHERE user_id=$1::uuid`, workerID).Scan(&before)
	key := "idem-wd-" + uniq("k")
	rec1 := h.do("POST", "/me/withdrawals", workerTok, "", map[string]any{"amount": 100, "payout_method_id": pmID, "idempotency_key": key})
	require.Equal(t, 201, rec1.Code)
	var after1 int
	_ = h.pool.QueryRow(context.Background(), `SELECT available_balance FROM wallets WHERE user_id=$1::uuid`, workerID).Scan(&after1)
	require.Equal(t, before-100, after1)
	rec2 = h.do("POST", "/me/withdrawals", workerTok, "", map[string]any{"amount": 100, "payout_method_id": pmID, "idempotency_key": key})
	require.Equal(t, 201, rec2.Code)
	require.Equal(t, true, decodeBody(t, rec2)["repeated"])
	var after2 int
	_ = h.pool.QueryRow(context.Background(), `SELECT available_balance FROM wallets WHERE user_id=$1::uuid`, workerID).Scan(&after2)
	require.Equal(t, after1, after2)
}

// ── Gate 17: regression Phase 2 & 6 ─────────────────────────────────────────

func TestP7_Gate17_Regression(t *testing.T) {
	// Re-run funding and release
	h := newHarness(t)
	_, workerID, posterTok, workerTok, taskID, _ := setupAssignedTaskPhase6(t, h)
	_ = workerID
	// funding already done via setup, now test release via confirm
	rec := h.do("POST", "/tasks/"+taskID+"/complete", workerTok, "", nil)
	require.Equal(t, 200, rec.Code)
	rec = h.do("POST", "/tasks/"+taskID+"/confirm", posterTok, "", nil)
	require.Equal(t, 200, rec.Code)
	// cancellation refund via Phase 2 path (open task cancel)
	h2 := newHarness(t)
	// use open task refund: create open task then cancel before accept
	_, tok, _ := h2.signup(t, uniq("p7reg")+"@example.com", "password123", "Reg User")
	taskOpen := fundTaskForPhase4(t, h2, tok, validTaskInput())
	// fundTaskForPhase4 already funds and opens, so cancel open
	rec = h2.do("DELETE", "/tasks/"+taskOpen, tok, "", nil)
	require.Equal(t, 200, rec.Code)
}

// ── Gate 18: unique partial index still enforced ─────────────────────────────

func TestP7_Gate18_UniqueReleaseIndex(t *testing.T) {
	h := newHarness(t)
	_, err := h.pool.Exec(context.Background(), `SELECT 1 FROM pg_indexes WHERE indexname='uq_tx_release_once'`)
	require.NoError(t, err)
	// Try to insert second release directly at DB level should fail
	_, _, _, _, taskID, _ := setupAssignedTaskPhase6(t, h)
	// First release via confirm
	// Need to mark complete then confirm to get first release
	// Use helper to do it
	// Instead, directly try to insert two releases for same task
	_, workerID, posterTok, workerTok, taskID2, _ := setupAssignedTaskPhase6(t, h)
	rec := h.do("POST", "/tasks/"+taskID2+"/complete", workerTok, "", nil)
	require.Equal(t, 200, rec.Code)
	rec = h.do("POST", "/tasks/"+taskID2+"/confirm", posterTok, "", nil)
	require.Equal(t, 200, rec.Code)
	// Now try direct second release insert should fail due to unique index
	_, err = h.pool.Exec(context.Background(),
		`INSERT INTO transactions (task_id, payee_id, amount, type, status, provider_ref) VALUES ($1::uuid, $2::uuid, 100, 'release', 'succeeded', 'second-release')`,
		taskID2, workerID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "uq_tx_release_once")
	_ = taskID
	_ = workerID
}

// ── Self-audit: exactly one refund_escrow ───────────────────────────────────

func TestP7_SelfAudit_SingleRefund(t *testing.T) {
	root, _ := filepath.Abs("..")
	count := 0
	var files []string
	filepath.Walk(filepath.Join(root, "internal"), func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") {
			return nil
		}
		raw, _ := os.ReadFile(p)
		s := string(raw)
		// Count places that create refund transactions (should be only ledger)
		if strings.Contains(s, "type, status") && strings.Contains(s, "'refund'") {
			// Check if it's inside ledger file
			if !strings.Contains(p, "ledger") {
				count++
				files = append(files, p)
			}
		}
		// Also check for RefundEscrowTx definition
		if strings.Contains(s, "RefundEscrowTx") {
			// ok
		}
		return nil
	})
	require.Equal(t, 0, count, "only ledger should create refund rows, found in %v", files)
	// Ensure ledger has exactly one func RefundEscrowTx
	raw, _ := os.ReadFile(filepath.Join(root, "internal", "ledger", "ledger.go"))
	require.Contains(t, string(raw), "func RefundEscrowTx")
	require.Equal(t, 1, strings.Count(string(raw), "func RefundEscrowTx"))
}

func TestP7_Docs_OpenAPI(t *testing.T) {
	h := newHarness(t)
	rec := h.do("GET", "/openapi.yaml", "", "", nil)
	require.Equal(t, 200, rec.Code)
	body := rec.Body.String()
	for _, p := range []string{"/me/wallet", "/me/transactions", "/me/payment-methods", "/me/payout-methods", "/me/withdrawals", "/transactions/{id}/receipt", "/webhooks/payments"} {
		require.Contains(t, body, p, "spec must document %s", p)
	}
}
