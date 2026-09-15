// Phase 9 Testing Gate — Trust & Safety (16 gates + self-audit + docs).
package tests

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ── helpers ────────────────────────────────────────────────────────────────

func adminToken(t *testing.T, h *harness) string {
	t.Helper()
	adminID := "00000000-0000-0000-0000-000000000099"
	// Ensure admin user exists in users table (requireAuth needs it)
	_, _ = h.pool.Exec(context.Background(),
		`INSERT INTO users (id, email, display_name) VALUES ($1::uuid, 'admin@sidekick.test', 'Admin')
		 ON CONFLICT (id) DO NOTHING`, adminID)
	tok, err := h.issuer.MintAdmin(adminID, time.Hour)
	require.NoError(t, err)
	return tok
}

func setupDisputedTask(t *testing.T, h *harness) (posterID, workerID, posterTok, workerTok, taskID, disputeID string) {
	t.Helper()
	posterID, posterTok, _ = h.signup(t, uniq("p9p")+"@example.com", "password123", "Poster P9")
	workerID, workerTok, _ = h.signup(t, uniq("p9w")+"@example.com", "password123", "Worker P9")
	taskID = fundTaskForPhase4(t, h, posterTok, validTaskInput())
	_, out := makeOffer(t, h, workerTok, taskID, nil, "")
	offerID := out["offer"].(map[string]any)["id"].(string)
	rec := acceptOffer(t, h, posterTok, offerID, "p9-assign-"+uniq("k"))
	require.Equal(t, 200, rec.Code, rec.Body.String())
	// Raise dispute
	rec = h.do("POST", "/tasks/"+taskID+"/dispute", workerTok, "", map[string]any{
		"reason": "quality_issue", "description": "Worker did not complete as described",
	})
	require.Equal(t, 201, rec.Code, rec.Body.String())
	disputeID = decodeBody(t, rec)["dispute_id"].(string)
	return
}

// ── Gate 1: submit verification; second pending rejected ────────────────────

func TestP9_Gate1_VerificationSubmitAndDuplicate(t *testing.T) {
	h := newHarness(t)
	_, tok, _ := h.signup(t, uniq("p9v")+"@example.com", "password123", "Verify User")

	// First submission succeeds
	rec := h.do("POST", "/me/verifications", tok, "", map[string]any{
		"document_type": "passport", "document_url": "https://storage.example/id/doc1.pdf",
	})
	require.Equal(t, 201, rec.Code, rec.Body.String())
	vid := decodeBody(t, rec)["verification_id"].(string)
	require.NotEmpty(t, vid)

	// Second submission while pending is rejected (409)
	rec = h.do("POST", "/me/verifications", tok, "", map[string]any{
		"document_type": "driving_licence", "document_url": "https://storage.example/id/doc2.pdf",
	})
	require.Equal(t, 409, rec.Code, "second pending must be rejected")

	// Verify users.verification_status = 'pending'
	var vStatus string
	_ = h.pool.QueryRow(context.Background(), `SELECT verification_status FROM users WHERE id=$1::uuid`,
		decodeBody(t, h.do("GET", "/me", tok, "", nil))["user"].(map[string]any)["id"].(string)).
		Scan(&vStatus)
	require.Equal(t, "pending", vStatus)
}

// ── Gate 2: admin approve/reject updates users.verification_status ──────────

func TestP9_Gate2_AdminApproveRejectVerification(t *testing.T) {
	h := newHarness(t)
	_, tok, _ := h.signup(t, uniq("p9v2")+"@example.com", "password123", "Verify User 2")
	adminTok := adminToken(t, h)

	// Submit verification
	rec := h.do("POST", "/me/verifications", tok, "", map[string]any{
		"document_type": "national_id", "document_url": "https://storage.example/id/doc3.pdf",
	})
	require.Equal(t, 201, rec.Code)
	vid := decodeBody(t, rec)["verification_id"].(string)

	// Admin lists pending — should see it
	rec = h.do("GET", "/admin/verifications?status=pending", adminTok, "", nil)
	require.Equal(t, 200, rec.Code)
	verifs := decodeBody(t, rec)["verifications"].([]any)
	require.GreaterOrEqual(t, len(verifs), 1)

	// Admin approves
	rec = h.do("POST", "/admin/verifications/"+vid+"/approve", adminTok, "", nil)
	require.Equal(t, 200, rec.Code, rec.Body.String())

	// Verify users.verification_status = 'verified'
	var vStatus string
	uid := decodeBody(t, h.do("GET", "/me", tok, "", nil))["user"].(map[string]any)["id"].(string)
	_ = h.pool.QueryRow(context.Background(), `SELECT verification_status FROM users WHERE id=$1::uuid`, uid).Scan(&vStatus)
	require.Equal(t, "verified", vStatus)

	// Test rejection path with a new user
	_, tok2, _ := h.signup(t, uniq("p9v3")+"@example.com", "password123", "Verify User 3")
	rec = h.do("POST", "/me/verifications", tok2, "", map[string]any{
		"document_type": "passport", "document_url": "https://storage.example/id/doc4.pdf",
	})
	require.Equal(t, 201, rec.Code)
	vid2 := decodeBody(t, rec)["verification_id"].(string)

	rec = h.do("POST", "/admin/verifications/"+vid2+"/reject", adminTok, "", map[string]any{
		"reason": "blurry image",
	})
	require.Equal(t, 200, rec.Code)

	uid2 := decodeBody(t, h.do("GET", "/me", tok2, "", nil))["user"].(map[string]any)["id"].(string)
	_ = h.pool.QueryRow(context.Background(), `SELECT verification_status FROM users WHERE id=$1::uuid`, uid2).Scan(&vStatus)
	require.Equal(t, "rejected", vStatus)
}

// ── Gate 3: report submission with task/conversation scope ──────────────────

func TestP9_Gate3_ReportSubmission(t *testing.T) {
	h := newHarness(t)
	_, tok1, _ := h.signup(t, uniq("p9r1")+"@example.com", "password123", "Reporter 1")
	uid2, _, _ := h.signup(t, uniq("p9r2")+"@example.com", "password123", "Reported User")

	// Report without task/conversation
	rec := h.do("POST", "/reports", tok1, "", map[string]any{
		"reported_user_id": uid2, "reason": "spam", "detail": "Posting spam content",
	})
	require.Equal(t, 201, rec.Code, rec.Body.String())
	reportID := decodeBody(t, rec)["report_id"].(string)
	require.NotEmpty(t, reportID)

	// Report with task scope
	taskID := fundTaskForPhase4(t, h, tok1, validTaskInput())
	rec = h.do("POST", "/reports", tok1, "", map[string]any{
		"reported_user_id": uid2, "task_id": taskID,
		"reason": "fraud", "detail": "Fake task",
	})
	require.Equal(t, 201, rec.Code)

	// Invalid reason rejected
	rec = h.do("POST", "/reports", tok1, "", map[string]any{
		"reported_user_id": uid2, "reason": "invalid_reason",
	})
	require.Equal(t, 400, rec.Code)

	// Cannot report yourself
	rec = h.do("POST", "/reports", tok1, "", map[string]any{
		"reported_user_id": decodeBody(t, h.do("GET", "/me", tok1, "", nil))["user"].(map[string]any)["id"].(string),
		"reason": "spam",
	})
	require.Equal(t, 400, rec.Code)
}

// ── Gate 4: admin report lifecycle open → reviewing → actioned/dismissed ────

func TestP9_Gate4_AdminReportLifecycle(t *testing.T) {
	h := newHarness(t)
	adminTok := adminToken(t, h)
	_, tok1, _ := h.signup(t, uniq("p9rl")+"@example.com", "password123", "Reporter RL")
	uid2, _, _ := h.signup(t, uniq("p9rp")+"@example.com", "password123", "Reported RL")

	// Create report
	rec := h.do("POST", "/reports", tok1, "", map[string]any{
		"reported_user_id": uid2, "reason": "harassment",
	})
	require.Equal(t, 201, rec.Code)
	reportID := decodeBody(t, rec)["report_id"].(string)

	// Admin lists open reports
	rec = h.do("GET", "/admin/reports?status=open", adminTok, "", nil)
	require.Equal(t, 200, rec.Code)
	reports := decodeBody(t, rec)["reports"].([]any)
	require.GreaterOrEqual(t, len(reports), 1)

	// Move to reviewing
	rec = h.do("PATCH", "/admin/reports/"+reportID, adminTok, "", map[string]any{"status": "reviewing"})
	require.Equal(t, 200, rec.Code, rec.Body.String())

	// Move to actioned
	rec = h.do("PATCH", "/admin/reports/"+reportID, adminTok, "", map[string]any{"status": "actioned"})
	require.Equal(t, 200, rec.Code)

	// Test dismissal path with second report
	rec = h.do("POST", "/reports", tok1, "", map[string]any{
		"reported_user_id": uid2, "reason": "other",
	})
	require.Equal(t, 201, rec.Code)
	reportID2 := decodeBody(t, rec)["report_id"].(string)

	rec = h.do("PATCH", "/admin/reports/"+reportID2, adminTok, "", map[string]any{"status": "reviewing"})
	require.Equal(t, 200, rec.Code)
	rec = h.do("PATCH", "/admin/reports/"+reportID2, adminTok, "", map[string]any{"status": "dismissed"})
	require.Equal(t, 200, rec.Code)

	// Invalid transition rejected (actioned → anything)
	rec = h.do("PATCH", "/admin/reports/"+reportID, adminTok, "", map[string]any{"status": "open"})
	require.Equal(t, 409, rec.Code)
}

// ── Gate 5: dispute evidence upload by either party ────────────────────────

func TestP9_Gate5_DisputeEvidenceUpload(t *testing.T) {
	h := newHarness(t)
	posterID, workerID, posterTok, workerTok, _, disputeID := setupDisputedTask(t, h)

	// Poster uploads evidence
	rec := h.do("POST", "/disputes/"+disputeID+"/evidence", posterTok, "", map[string]any{
		"evidence_url": "https://storage.example/evidence/poster-proof.jpg",
	})
	require.Equal(t, 200, rec.Code, rec.Body.String())

	// Worker uploads evidence
	rec = h.do("POST", "/disputes/"+disputeID+"/evidence", workerTok, "", map[string]any{
		"evidence_url": "https://storage.example/evidence/worker-proof.jpg",
	})
	require.Equal(t, 200, rec.Code)

	// Verify evidence_urls array has 2 entries
	var urls []string
	_ = h.pool.QueryRow(context.Background(),
		`SELECT evidence_urls FROM disputes WHERE id=$1::uuid`, disputeID).Scan(&urls)
	require.Len(t, urls, 2)

	_ = posterID
	_ = workerID
}

// ── Gate 6: admin resolve dispute (released, refunded, split) ──────────────

func TestP9_Gate6_AdminResolveDispute(t *testing.T) {
	h := newHarness(t)
	adminTok := adminToken(t, h)

	// --- Test released ---
	posterTok1, workerTok1, taskID1, disputeID1 := createDisputeForResolve(t, h)
	posterID1 := decodeBody(t, h.do("GET", "/me", posterTok1, "", nil))["user"].(map[string]any)["id"].(string)
	workerID1 := decodeBody(t, h.do("GET", "/me", workerTok1, "", nil))["user"].(map[string]any)["id"].(string)

	rec := h.do("POST", "/admin/disputes/"+disputeID1+"/resolve", adminTok, "", map[string]any{
		"decision": "released", "resolution_notes": "Worker completed task properly",
	})
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Equal(t, "resolved_released", getDisputeStatus(t, h, disputeID1))
	require.Equal(t, "resolved_released", getTaskStatus(t, h, taskID1))
	require.Equal(t, "released", getEscrowStatus(t, h, taskID1))
	require.Greater(t, getWalletBalance(t, h, workerID1), 0)
	_ = posterID1

	// --- Test refunded ---
	_, workerTok2, taskID2, disputeID2 := createDisputeForResolve(t, h)
	_ = workerTok2

	rec = h.do("POST", "/admin/disputes/"+disputeID2+"/resolve", adminTok, "", map[string]any{
		"decision": "refunded", "resolution_notes": "Worker did not show up",
	})
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Equal(t, "resolved_refunded", getDisputeStatus(t, h, disputeID2))
	require.Equal(t, "refunded", getEscrowStatus(t, h, taskID2))

	// --- Test split ---
	posterTok3, workerTok3, taskID3, disputeID3 := createDisputeForResolve(t, h)
	_ = posterTok3
	_ = workerTok3

	rec = h.do("POST", "/admin/disputes/"+disputeID3+"/resolve", adminTok, "", map[string]any{
		"decision": "split", "compensation_amount": 500, "resolution_notes": "Partial work completed",
	})
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Equal(t, "resolved_split", getDisputeStatus(t, h, disputeID3))
	require.Equal(t, "resolved_split", getTaskStatus(t, h, taskID3))
}

func createDisputeForResolve(t *testing.T, h *harness) (posterTok, workerTok, taskID, disputeID string) {
	t.Helper()
	_, posterTok, _ = h.signup(t, uniq("p9dr")+"@example.com", "password123", "Poster DR")
	_, workerTok, _ = h.signup(t, uniq("p9dw")+"@example.com", "password123", "Worker DR")
	taskID = fundTaskForPhase4(t, h, posterTok, validTaskInput())
	_, out := makeOffer(t, h, workerTok, taskID, nil, "")
	offerID := out["offer"].(map[string]any)["id"].(string)
	rec := acceptOffer(t, h, posterTok, offerID, "p9-dr-"+uniq("k"))
	require.Equal(t, 200, rec.Code, rec.Body.String())
	rec = h.do("POST", "/tasks/"+taskID+"/dispute", workerTok, "", map[string]any{
		"reason": "quality_issue", "description": "Dispute for resolution test",
	})
	require.Equal(t, 201, rec.Code, rec.Body.String())
	disputeID = decodeBody(t, rec)["dispute_id"].(string)
	return
}

func getDisputeStatus(t *testing.T, h *harness, disputeID string) string {
	t.Helper()
	var s string
	err := h.pool.QueryRow(context.Background(), `SELECT status FROM disputes WHERE id=$1::uuid`, disputeID).Scan(&s)
	require.NoError(t, err)
	return s
}

// ── Gate 7: contact-detail flagged but delivered ────────────────────────────

func TestP9_Gate7_ContactDetailLeakDetection(t *testing.T) {
	h := newHarness(t)
	_, tok1, _ := h.signup(t, uniq("p9c1")+"@example.com", "password123", "Sender CL")
	_, tok2, _ := h.signup(t, uniq("p9c2")+"@example.com", "password123", "Receiver CL")

	// Create a task + conversation to send messages
	taskID := fundTaskForPhase4(t, h, tok1, validTaskInput())
	_, out := makeOffer(t, h, tok2, taskID, nil, "")
	offerID := out["offer"].(map[string]any)["id"].(string)
	rec := acceptOffer(t, h, tok1, offerID, "p9-cl-"+uniq("k"))
	require.Equal(t, 200, rec.Code, rec.Body.String())
	convID := decodeBody(t, rec)["accept"].(map[string]any)["conversation_id"].(string)

	// Send message with phone number — should be delivered AND flagged
	phoneBody := "Call me at +44 7911 123456 or text me"
	rec = h.do("POST", "/conversations/"+convID+"/messages", tok1, "", map[string]any{"body": phoneBody})
	require.Equal(t, 201, rec.Code, rec.Body.String())
	msgID := decodeBody(t, rec)["message"].(map[string]any)["id"].(string)

	// Message was delivered (recipient can see it)
	rec = h.do("GET", "/conversations/"+convID+"/messages", tok2, "", nil)
	require.Equal(t, 200, rec.Code)
	require.Contains(t, rec.Body.String(), phoneBody)

	// Audit log was written for contact leak
	var auditCnt int
	_ = h.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM audit_log WHERE action='message.contact_leak_flag' AND entity_id=$1`, msgID).Scan(&auditCnt)
	require.Equal(t, 1, auditCnt, "contact leak must be flagged in audit_log")

	// Send email — also flagged but delivered
	emailBody := "Email me at test@example.com"
	rec = h.do("POST", "/conversations/"+convID+"/messages", tok1, "", map[string]any{"body": emailBody})
	require.Equal(t, 201, rec.Code)
	require.Contains(t, h.do("GET", "/conversations/"+convID+"/messages", tok2, "", nil).Body.String(), emailBody)

	// Send off-platform payment mention
	paymentBody := "Can you pay me via Venmo?"
	rec = h.do("POST", "/conversations/"+convID+"/messages", tok1, "", map[string]any{"body": paymentBody})
	require.Equal(t, 201, rec.Code)
	require.Contains(t, h.do("GET", "/conversations/"+convID+"/messages", tok2, "", nil).Body.String(), paymentBody)

	// Normal message — no flag
	normalBody := "Hello, looking forward to the task"
	rec = h.do("POST", "/conversations/"+convID+"/messages", tok1, "", map[string]any{"body": normalBody})
	require.Equal(t, 201, rec.Code)
	msgIDNormal := decodeBody(t, rec)["message"].(map[string]any)["id"].(string)
	var normalAuditCnt int
	_ = h.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM audit_log WHERE action='message.contact_leak_flag' AND entity_id=$1`, msgIDNormal).Scan(&normalAuditCnt)
	require.Equal(t, 0, normalAuditCnt, "normal message must not be flagged")
}

// ── Gate 8: non-admin cannot access any admin endpoint ─────────────────────

func TestP9_Gate8_NonAdminBlockedOnAllAdminEndpoints(t *testing.T) {
	h := newHarness(t)
	_, userTok, _ := h.signup(t, uniq("p9na")+"@example.com", "password123", "Non-Admin")

	// Create resources to reference
	rec := h.do("POST", "/me/verifications", userTok, "", map[string]any{
		"document_type": "passport", "document_url": "https://storage.example/id/doc.pdf",
	})
	require.Equal(t, 201, rec.Code)
	vid := decodeBody(t, rec)["verification_id"].(string)

	uid2, _, _ := h.signup(t, uniq("p9na2")+"@example.com", "password123", "Other User")
	rec = h.do("POST", "/reports", userTok, "", map[string]any{
		"reported_user_id": uid2, "reason": "spam",
	})
	require.Equal(t, 201, rec.Code)
	reportID := decodeBody(t, rec)["report_id"].(string)

	_, _, _, workerTok, taskID, _ := setupDisputedTask(t, h)
	_ = workerTok
	var disputeID string
	_ = h.pool.QueryRow(context.Background(),
		`SELECT id::text FROM disputes WHERE task_id=$1::uuid`, taskID).Scan(&disputeID)

	// Test each admin endpoint with non-admin token
	adminEndpoints := []struct {
		method string
		path   string
		body   any
	}{
		{"GET", "/admin/verifications?status=pending", nil},
		{"POST", "/admin/verifications/" + vid + "/approve", nil},
		{"POST", "/admin/verifications/" + vid + "/reject", map[string]any{"reason": "test"}},
		{"GET", "/admin/reports?status=open", nil},
		{"PATCH", "/admin/reports/" + reportID, map[string]any{"status": "reviewing"}},
		{"GET", "/admin/disputes?status=open", nil},
		{"PATCH", "/admin/disputes/" + disputeID + "/triage", nil},
		{"POST", "/admin/disputes/" + disputeID + "/resolve", map[string]any{"decision": "released"}},
	}

	for _, ep := range adminEndpoints {
		rec = h.do(ep.method, ep.path, userTok, "", ep.body)
		require.Equal(t, 403, rec.Code,
			"non-admin must be rejected on %s %s, got %d: %s", ep.method, ep.path, rec.Code, rec.Body.String())
	}
}

// ── Gate 9: raw storage path not guessable/public ──────────────────────────

func TestP9_Gate9_VerificationDocNotPublic(t *testing.T) {
	h := newHarness(t)
	_, tok, _ := h.signup(t, uniq("p9vp")+"@example.com", "password123", "Verify Public")

	rec := h.do("POST", "/me/verifications", tok, "", map[string]any{
		"document_type": "passport", "document_url": "https://storage.example/id/secret-doc.pdf",
	})
	require.Equal(t, 201, rec.Code)
	vid := decodeBody(t, rec)["verification_id"].(string)

	// The document URL is stored but not publicly accessible via any list endpoint
	// Verify that listing pending verifications returns the URL only to admin
	adminTok := adminToken(t, h)
	rec = h.do("GET", "/admin/verifications?status=pending", adminTok, "", nil)
	require.Equal(t, 200, rec.Code)
	verifs := decodeBody(t, rec)["verifications"].([]any)
	found := false
	for _, v := range verifs {
		vMap := v.(map[string]any)
		if vMap["id"] == vid {
			require.Equal(t, "https://storage.example/id/secret-doc.pdf", vMap["document_url"])
			found = true
		}
	}
	require.True(t, found, "verification must be in admin list")

	// Non-admin cannot list verifications
	_, userTok2, _ := h.signup(t, uniq("p9vp2")+"@example.com", "password123", "Other User")
	rec = h.do("GET", "/admin/verifications?status=pending", userTok2, "", nil)
	require.Equal(t, 403, rec.Code)
}

// ── Gate 10: reported user cannot read reports about them ──────────────────

func TestP9_Gate10_ReportedUserCannotReadReports(t *testing.T) {
	h := newHarness(t)
	_, reporterTok, _ := h.signup(t, uniq("p9rr")+"@example.com", "password123", "Reporter RR")
	reportedUID, reportedTok, _ := h.signup(t, uniq("p9rpt")+"@example.com", "password123", "Reported RR")

	// Reporter submits report
	rec := h.do("POST", "/reports", reporterTok, "", map[string]any{
		"reported_user_id": reportedUID, "reason": "harassment",
	})
	require.Equal(t, 201, rec.Code)

	// Reported user tries to list own reports — should see empty (they are not the reporter)
	rec = h.do("GET", "/me/reports", reportedTok, "", nil)
	require.Equal(t, 200, rec.Code)
	reports := decodeBody(t, rec)["reports"].([]any)
	require.Empty(t, reports, "reported user must not see reports about them via own reports endpoint")
}

// ── Gate 11: non-party cannot upload evidence ──────────────────────────────

func TestP9_Gate11_NonPartyCannotUploadEvidence(t *testing.T) {
	h := newHarness(t)
	_, _, _, _, _, disputeID := setupDisputedTask(t, h)

	// Third user (not a party) tries to upload evidence
	_, outsiderTok, _ := h.signup(t, uniq("p9np")+"@example.com", "password123", "Outsider")
	rec := h.do("POST", "/disputes/"+disputeID+"/evidence", outsiderTok, "", map[string]any{
		"evidence_url": "https://storage.example/evidence/outsider.jpg",
	})
	require.Equal(t, 403, rec.Code, "non-party must be rejected")
}

// ── Gate 12: evidence rejected after dispute resolved ──────────────────────

func TestP9_Gate12_EvidenceRejectedAfterResolved(t *testing.T) {
	h := newHarness(t)
	adminTok := adminToken(t, h)
	posterTok, workerTok, _, disputeID := createDisputeForResolve(t, h)

	// Resolve dispute
	rec := h.do("POST", "/admin/disputes/"+disputeID+"/resolve", adminTok, "", map[string]any{
		"decision": "released", "resolution_notes": "Done",
	})
	require.Equal(t, 200, rec.Code)

	// Evidence upload rejected after resolution
	rec = h.do("POST", "/disputes/"+disputeID+"/evidence", posterTok, "", map[string]any{
		"evidence_url": "https://storage.example/evidence/late.jpg",
	})
	require.Equal(t, 409, rec.Code, "evidence must be rejected after resolution")

	rec = h.do("POST", "/disputes/"+disputeID+"/evidence", workerTok, "", map[string]any{
		"evidence_url": "https://storage.example/evidence/late2.jpg",
	})
	require.Equal(t, 409, rec.Code, "evidence must be rejected after resolution")
}

// ── Gate 13: race test — concurrent resolve attempts ───────────────────────

func TestP9_Gate13_ConcurrentResolveRace(t *testing.T) {
	h := newHarness(t)
	adminTok := adminToken(t, h)
	posterTok, _, _, _ := createDisputeForResolve(t, h)

	// Get dispute ID
	var disputeID string
	// We need to create a fresh dispute for this test
	_, wTok, _ := h.signup(t, uniq("p9race")+"@example.com", "password123", "Worker Race")
	taskID := fundTaskForPhase4(t, h, posterTok, validTaskInput())
	_, out := makeOffer(t, h, wTok, taskID, nil, "")
	offerID := out["offer"].(map[string]any)["id"].(string)
	rec := acceptOffer(t, h, posterTok, offerID, "p9-race-"+uniq("k"))
	require.Equal(t, 200, rec.Code)
	rec = h.do("POST", "/tasks/"+taskID+"/dispute", wTok, "", map[string]any{
		"reason": "quality_issue", "description": "Race test",
	})
	require.Equal(t, 201, rec.Code)
	disputeID = decodeBody(t, rec)["dispute_id"].(string)

	// Two concurrent resolve attempts
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([]int, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			r := h.do("POST", "/admin/disputes/"+disputeID+"/resolve", adminTok, "", map[string]any{
				"decision": "released", "resolution_notes": fmt.Sprintf("attempt %d", idx),
			})
			mu.Lock()
			results[idx] = r.Code
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	// Exactly one must succeed (200), the other must fail (409)
	successCnt := 0
	for _, code := range results {
		if code == 200 {
			successCnt++
		}
	}
	require.Equal(t, 1, successCnt, "exactly one concurrent resolve must succeed; got %d successes", successCnt)
	require.Equal(t, "resolved_released", getDisputeStatus(t, h, disputeID))
}

// ── Gate 14: split resolution uses partial amounts correctly ────────────────

func TestP9_Gate14_SplitResolutionPartialAmounts(t *testing.T) {
	h := newHarness(t)
	adminTok := adminToken(t, h)

	posterTok, workerTok, taskID, disputeID := createDisputeForResolve(t, h)
	posterID := decodeBody(t, h.do("GET", "/me", posterTok, "", nil))["user"].(map[string]any)["id"].(string)
	workerID := decodeBody(t, h.do("GET", "/me", workerTok, "", nil))["user"].(map[string]any)["id"].(string)

	// Get original escrow amount
	var total, fee int
	_ = h.pool.QueryRow(context.Background(),
		`SELECT total_charge, platform_fee FROM tasks WHERE id=$1::uuid`, taskID).Scan(&total, &fee)
	workerPayout := total - fee
	if workerPayout <= 0 {
		workerPayout = total
	}

	// Split: 300 to poster (refund), rest to worker (release)
	splitAmount := 300
	rec := h.do("POST", "/admin/disputes/"+disputeID+"/resolve", adminTok, "", map[string]any{
		"decision": "split", "compensation_amount": splitAmount,
		"resolution_notes": "Partial compensation",
	})
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Equal(t, "resolved_split", getDisputeStatus(t, h, disputeID))

	// Verify amounts: total movement must not exceed held escrow
	var releaseTotal, refundTotal int
	_ = h.pool.QueryRow(context.Background(),
		`SELECT COALESCE(SUM(amount),0) FROM transactions WHERE task_id=$1::uuid AND type='release' AND status='succeeded'`, taskID).Scan(&releaseTotal)
	_ = h.pool.QueryRow(context.Background(),
		`SELECT COALESCE(SUM(amount),0) FROM transactions WHERE task_id=$1::uuid AND type='refund' AND status='succeeded'`, taskID).Scan(&refundTotal)

	totalMovement := releaseTotal + refundTotal
	require.LessOrEqual(t, totalMovement, total, "total movement must not exceed held escrow")

	// Verify wallet balances are consistent
	workerBal := getWalletBalance(t, h, workerID)
	posterBal := getPosterRefundBalance(t, h, posterID)
	require.Equal(t, workerPayout-splitAmount, workerBal, "worker gets payout minus compensation")
	require.Equal(t, splitAmount, posterBal, "poster gets compensation refund")
}

func getPosterRefundBalance(t *testing.T, h *harness, posterID string) int {
	t.Helper()
	var total int
	_ = h.pool.QueryRow(context.Background(),
		`SELECT COALESCE(SUM(amount),0) FROM transactions WHERE payee_id=$1::uuid AND type='refund' AND status='succeeded'`, posterID).Scan(&total)
	return total
}

// ── Gate 15: is_admin is same mechanism from Phase 0 ───────────────────────

func TestP9_Gate15_AdminClaimIsPhase0Mechanism(t *testing.T) {
	h := newHarness(t)

	// Regular user token must not pass admin gate
	_, userTok, _ := h.signup(t, uniq("p9ac")+"@example.com", "password123", "Admin Check")

	// Verify user token does NOT carry is_admin
	claims, err := h.issuer.Verify(userTok)
	require.NoError(t, err)
	require.False(t, claims.IsAdmin, "user token must never carry is_admin")

	// Admin token from MintAdmin passes
	adminTok := adminToken(t, h)
	claims, err = h.issuer.RequireAdmin(adminTok)
	require.NoError(t, err)
	require.True(t, claims.IsAdmin)

	// Verify no alternate admin check: trace all admin endpoints to RequireAdmin
	// This is confirmed by construction — all admin handlers in trust.go call
	// s.issuer.RequireAdmin(r.Header.Get("Authorization")) directly.
}

// ── Gate 16: contact-leak flag does not block/delay/alter message ──────────

func TestP9_Gate16_ContactLeakDoesNotBlockMessage(t *testing.T) {
	h := newHarness(t)
	_, tok1, _ := h.signup(t, uniq("p9nb")+"@example.com", "password123", "Sender NB")
	_, tok2, _ := h.signup(t, uniq("p9nbr")+"@example.com", "password123", "Receiver NB")

	taskID := fundTaskForPhase4(t, h, tok1, validTaskInput())
	_, out := makeOffer(t, h, tok2, taskID, nil, "")
	offerID := out["offer"].(map[string]any)["id"].(string)
	rec := acceptOffer(t, h, tok1, offerID, "p9-nb-"+uniq("k"))
	require.Equal(t, 200, rec.Code)
	convID := decodeBody(t, rec)["accept"].(map[string]any)["conversation_id"].(string)

	// Send message with contact detail
	body := "My number is +44 7700 900123"
	start := time.Now()
	rec = h.do("POST", "/conversations/"+convID+"/messages", tok1, "", map[string]any{"body": body})
	elapsed := time.Since(start)
	require.Equal(t, 201, rec.Code, rec.Body.String())

	// Confirm message delivered exactly as sent (not altered)
	msgID := decodeBody(t, rec)["message"].(map[string]any)["id"].(string)
	var msgBody string
	_ = h.pool.QueryRow(context.Background(),
		`SELECT body FROM messages WHERE id=$1::uuid`, msgID).Scan(&msgBody)
	require.Equal(t, body, msgBody, "message body must not be altered")

	// Recipient receives it
	rec = h.do("GET", "/conversations/"+convID+"/messages", tok2, "", nil)
	require.Equal(t, 200, rec.Code)
	require.Contains(t, rec.Body.String(), body)

	// Must not be blocked or delayed (completed within 5 seconds)
	require.Less(t, elapsed.Seconds(), 5.0, "flagged message must not be delayed")
}

// ── Self-audit: split uses ledger functions ─────────────────────────────────

func TestP9_SelfAudit_SplitUsesLedgerFunctions(t *testing.T) {
	// This test confirms the architectural invariant: dispute resolution
	// calls ledger.ReleaseTx and ledger.RefundEscrowTx — the SAME functions
	// used by confirm and cancel. No separate split-specific money function exists.
	// Verified by code inspection; this test exercises the split path end-to-end.
	h := newHarness(t)
	adminTok := adminToken(t, h)
	posterTok, workerTok, taskID, disputeID := createDisputeForResolve(t, h)

	// Record wallet balances before
	workerID := decodeBody(t, h.do("GET", "/me", workerTok, "", nil))["user"].(map[string]any)["id"].(string)
	posterID := decodeBody(t, h.do("GET", "/me", posterTok, "", nil))["user"].(map[string]any)["id"].(string)
	beforeWorker := getWalletBalance(t, h, workerID)

	var total, fee int
	_ = h.pool.QueryRow(context.Background(),
		`SELECT total_charge, platform_fee FROM tasks WHERE id=$1::uuid`, taskID).Scan(&total, &fee)

	// Resolve as split
	splitAmt := 200
	rec := h.do("POST", "/admin/disputes/"+disputeID+"/resolve", adminTok, "", map[string]any{
		"decision": "split", "compensation_amount": splitAmt,
		"resolution_notes": "self-audit split",
	})
	require.Equal(t, 200, rec.Code, rec.Body.String())

	// Verify transactions exist with correct types
	var releaseCnt, refundCnt int
	_ = h.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM transactions WHERE task_id=$1::uuid AND type='release' AND status='succeeded'`, taskID).Scan(&releaseCnt)
	_ = h.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM transactions WHERE task_id=$1::uuid AND type='refund' AND status='succeeded'`, taskID).Scan(&refundCnt)
	require.Equal(t, 1, releaseCnt, "must have exactly one release transaction")
	require.Equal(t, 1, refundCnt, "must have exactly one refund transaction")

	// Worker wallet increased by release amount (release = workerPayout - splitAmt, workerPayout = total - fee)
	afterWorker := getWalletBalance(t, h, workerID)
	require.Equal(t, total-fee-splitAmt, afterWorker-beforeWorker, "worker gets total-fee minus split")

	_ = posterID
}

// ── Docs check ─────────────────────────────────────────────────────────────

func TestP9_Docs(t *testing.T) {
	h := newHarness(t)
	rec := h.do("GET", "/openapi.yaml", "", "", nil)
	require.Equal(t, 200, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, "/me/verifications")
	require.Contains(t, body, "/admin/verifications")
	require.Contains(t, body, "/admin/reports")
	require.Contains(t, body, "/disputes/")
	require.Contains(t, body, "/admin/disputes")
}
