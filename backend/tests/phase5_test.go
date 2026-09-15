// Phase 5 Testing Gate — messaging + trust gate (blocks) + scalar docs.
package tests

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// ── helpers for Phase 5 ────────────────────────────────────────────────────

func setupAssignedTask(t *testing.T, h *harness) (posterID, workerID, posterTok, workerTok, taskID, convID string) {
	t.Helper()
	var posterTokTmp, workerTokTmp string
	posterID, posterTokTmp, _ = h.signup(t, uniq("p5p")+"@example.com", "password123", "Poster P5")
	workerID, workerTokTmp, _ = h.signup(t, uniq("p5w")+"@example.com", "password123", "Worker P5")
	taskID = fundTaskForPhase4(t, h, posterTokTmp, validTaskInput())
	_, out := makeOffer(t, h, workerTokTmp, taskID, nil, "")
	offerID := out["offer"].(map[string]any)["id"].(string)
	rec := acceptOffer(t, h, posterTokTmp, offerID, "p5-assign-"+uniq("k"))
	require.Equal(t, 200, rec.Code, rec.Body.String())
	convID = decodeBody(t, rec)["accept"].(map[string]any)["conversation_id"].(string)
	return posterID, workerID, posterTokTmp, workerTokTmp, taskID, convID
}

func getConvMessages(t *testing.T, h *harness, tok, convID string) []any {
	t.Helper()
	rec := h.do("GET", "/conversations/"+convID+"/messages", tok, "", nil)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	return decodeBody(t, rec)["messages"].([]any)
}

// ── Gate 1: list conversations + system message ───────────────────────────

func TestP5_Gate1_ListConversationsAndSystem(t *testing.T) {
	h := newHarness(t)
	_, _, posterTok, workerTok, _, convID := setupAssignedTask(t, h)

	// Both parties can list and see the same conversation
	for _, tok := range []string{posterTok, workerTok} {
		rec := h.do("GET", "/conversations", tok, "", nil)
		require.Equal(t, 200, rec.Code)
		convs := decodeBody(t, rec)["conversations"].([]any)
		found := false
		for _, c := range convs {
			if c.(map[string]any)["id"] == convID {
				found = true
			}
		}
		require.True(t, found, "conversation should be listed")
		msgs := getConvMessages(t, h, tok, convID)
		require.GreaterOrEqual(t, len(msgs), 1)
		first := msgs[len(msgs)-1].(map[string]any) // most-recent-first, so last is oldest? Actually most-recent-first, system is first? Check: system is first created, but ordering is DESC, so system will be last if only one? But we have at least 1, check contains accepted
		foundSys := false
		for _, m := range msgs {
			if m.(map[string]any)["type"] == "system" {
				foundSys = true
				require.Contains(t, m.(map[string]any)["body"].(string), "14 Rye Lane")
				require.Equal(t, "accepted", m.(map[string]any)["system_event"])
			}
		}
		require.True(t, foundSys)
		_ = first
	}
}

// ── Gate 2: send text appears for other ────────────────────────────────────

func TestP5_Gate2_SendText(t *testing.T) {
	h := newHarness(t)
	_, _, posterTok, workerTok, _, convID := setupAssignedTask(t, h)

	rec := h.do("POST", "/conversations/"+convID+"/messages", posterTok, "", map[string]any{"body": "Hello from poster", "type": "text"})
	require.Equal(t, 201, rec.Code, rec.Body.String())
	msgID := decodeBody(t, rec)["message"].(map[string]any)["id"].(string)

	msgs := getConvMessages(t, h, workerTok, convID)
	found := false
	for _, m := range msgs {
		if m.(map[string]any)["id"] == msgID {
			found = true
			require.Equal(t, "Hello from poster", m.(map[string]any)["body"])
		}
	}
	require.True(t, found)

	// Worker replies
	rec = h.do("POST", "/conversations/"+convID+"/messages", workerTok, "", map[string]any{"body": "Hi poster"})
	require.Equal(t, 201, rec.Code)
}

// ── Gate 3: delivery/read receipts only by recipient ───────────────────────

func TestP5_Gate3_Receipts(t *testing.T) {
	h := newHarness(t)
	_, _, posterTok, workerTok, _, convID := setupAssignedTask(t, h)

	rec := h.do("POST", "/conversations/"+convID+"/messages", posterTok, "", map[string]any{"body": "Need receipt"})
	require.Equal(t, 201, rec.Code)
	msgID := decodeBody(t, rec)["message"].(map[string]any)["id"].(string)

	// Sender cannot mark own
	rec = h.do("POST", "/messages/"+msgID+"/delivered", posterTok, "", nil)
	require.Equal(t, 403, rec.Code)

	// Recipient can mark delivered and read
	rec = h.do("POST", "/messages/"+msgID+"/delivered", workerTok, "", nil)
	require.Equal(t, 200, rec.Code)
	rec = h.do("POST", "/messages/"+msgID+"/read", workerTok, "", nil)
	require.Equal(t, 200, rec.Code)

	// Verify via list
	msgs := getConvMessages(t, h, posterTok, convID)
	for _, m := range msgs {
		if m.(map[string]any)["id"] == msgID {
			require.NotNil(t, m.(map[string]any)["delivered_at"])
			require.NotNil(t, m.(map[string]any)["read_at"])
		}
	}

	// Sender cannot mark read either
	rec = h.do("POST", "/messages/"+msgID+"/read", posterTok, "", nil)
	require.Equal(t, 403, rec.Code)
}

// ── Gate 4: image signed URL ───────────────────────────────────────────────

func TestP5_Gate4_ImageSigned(t *testing.T) {
	h := newHarness(t)
	_, _, posterTok, workerTok, _, convID := setupAssignedTask(t, h)

	// Grant
	rec := h.do("POST", "/conversations/"+convID+"/images/grant", posterTok, "", map[string]any{"content_type": "image/jpeg"})
	require.Equal(t, 200, rec.Code, rec.Body.String())
	grant := decodeBody(t, rec)
	object := grant["object"].(string)
	sig := grant["signature"].(string)
	expires := grant["expires_at"].(string)

	img := testJPEG(t, 100, 100)
	b64 := base64.StdEncoding.EncodeToString(img)
	rec = h.do("POST", "/conversations/"+convID+"/images", posterTok, "", map[string]any{
		"object": object, "signature": sig, "expires_at": expires, "image_base64": b64,
	})
	require.Equal(t, 201, rec.Code, rec.Body.String())
	msg := decodeBody(t, rec)["message"].(map[string]any)
	require.Equal(t, "image", msg["type"])
	attachment := msg["attachment_url"].(string)
	require.Contains(t, attachment, "sig=")
	require.Contains(t, attachment, "expires=")

	// Fetch via signed URL should succeed for participant
	// attachment is like /chat-images/convID/uuid.jpg?expires=...&sig=...
	rec = h.do("GET", attachment, workerTok, "", nil)
	require.Equal(t, 200, rec.Code)
	require.Equal(t, "image/jpeg", rec.Header().Get("Content-Type"))

	// Without sig should be 403
	pathOnly := strings.Split(attachment, "?")[0]
	rec = h.do("GET", pathOnly, workerTok, "", nil)
	require.Equal(t, 403, rec.Code)

	// Non-participant cannot fetch even with sig
	_, strangerTok, _ := h.signup(t, uniq("p5s4")+"@example.com", "password123", "Stranger P5-4")
	rec = h.do("GET", attachment, strangerTok, "", nil)
	require.Equal(t, 403, rec.Code)

	// Invalid content-type grant should be rejected
	rec = h.do("POST", "/conversations/"+convID+"/images/grant", posterTok, "", map[string]any{"content_type": "application/pdf"})
	require.Equal(t, 400, rec.Code)
}

// ── Gate 5: block/unblock ──────────────────────────────────────────────────

func TestP5_Gate5_BlockUnblock(t *testing.T) {
	h := newHarness(t)
	idA, tokA, _ := h.signup(t, uniq("p5a5")+"@example.com", "password123", "User A5")
	idB, _, _ := h.signup(t, uniq("p5b5")+"@example.com", "password123", "User B5")

	rec := h.do("POST", "/users/"+idB+"/block", tokA, "", nil)
	require.Equal(t, 200, rec.Code)
	rec = h.do("GET", "/blocks", tokA, "", nil)
	require.Equal(t, 200, rec.Code)
	blocks := decodeBody(t, rec)["blocks"].([]any)
	require.Contains(t, fmt.Sprint(blocks), idB)

	rec = h.do("DELETE", "/users/"+idB+"/block", tokA, "", nil)
	require.Equal(t, 200, rec.Code)
	rec = h.do("GET", "/blocks", tokA, "", nil)
	blocks = decodeBody(t, rec)["blocks"].([]any)
	require.NotContains(t, fmt.Sprint(blocks), idB)

	// Cannot block self
	rec = h.do("POST", "/users/"+idA+"/block", tokA, "", nil)
	require.Equal(t, 400, rec.Code)
}

// ── Gate 6: report from conversation ───────────────────────────────────────

func TestP5_Gate6_Report(t *testing.T) {
	h := newHarness(t)
	_, _, posterTok, _, _, convID := setupAssignedTask(t, h)
	rec := h.do("POST", "/conversations/"+convID+"/report", posterTok, "", map[string]any{"reason": "spam", "detail": "spammy user"})
	require.Equal(t, 201, rec.Code)
	require.NotEmpty(t, decodeBody(t, rec)["report_id"])

	// Invalid reason
	rec = h.do("POST", "/conversations/"+convID+"/report", posterTok, "", map[string]any{"reason": "invalid"})
	require.Equal(t, 400, rec.Code)

	// Non-participant cannot report
	_, strangerTok, _ := h.signup(t, uniq("p5s6")+"@example.com", "password123", "Stranger P5-6")
	rec = h.do("POST", "/conversations/"+convID+"/report", strangerTok, "", map[string]any{"reason": "spam"})
	require.Equal(t, 403, rec.Code)
}

// ── Gate 7: no conversation before accept ──────────────────────────────────

func TestP5_Gate7_NoConversationBeforeAccept(t *testing.T) {
	h := newHarness(t)
	_, posterTok, _ := h.signup(t, uniq("p5p7")+"@example.com", "password123", "Poster P5-7")
	_, workerTok, _ := h.signup(t, uniq("p5w7")+"@example.com", "password123", "Worker P5-7")
	taskID := fundTaskForPhase4(t, h, posterTok, validTaskInput())
	_, _ = makeOffer(t, h, workerTok, taskID, nil, "")

	// No conversation should exist for this task yet
	rec := h.do("GET", "/tasks/"+taskID+"/conversation", posterTok, "", nil)
	require.Equal(t, 404, rec.Code)

	// Trying to list messages on a non-existent conversation id should 404
	fakeConv := "00000000-0000-0000-0000-000000000001"
	rec = h.do("GET", "/conversations/"+fakeConv+"/messages", posterTok, "", nil)
	require.Equal(t, 404, rec.Code)

	// Trying to send to non-existent conversation
	rec = h.do("POST", "/conversations/"+fakeConv+"/messages", posterTok, "", map[string]any{"body": "hello"})
	require.Equal(t, 404, rec.Code)
}

// ── Gate 8: non-participant cannot read/send ────────────────────────────────

func TestP5_Gate8_NonParticipant(t *testing.T) {
	h := newHarness(t)
	_, _, posterTok, _, _, convID := setupAssignedTask(t, h)
	_, strangerTok, _ := h.signup(t, uniq("p5s8")+"@example.com", "password123", "Stranger P5-8")

	rec := h.do("GET", "/conversations/"+convID+"/messages", strangerTok, "", nil)
	require.Equal(t, 403, rec.Code)

	rec = h.do("POST", "/conversations/"+convID+"/messages", strangerTok, "", map[string]any{"body": "hacked"})
	require.Equal(t, 403, rec.Code)

	// Also cannot list conversations and see it
	rec = h.do("GET", "/conversations", strangerTok, "", nil)
	require.Equal(t, 200, rec.Code)
	convs := decodeBody(t, rec)["conversations"].([]any)
	for _, c := range convs {
		require.NotEqual(t, convID, c.(map[string]any)["id"])
	}
	_ = posterTok
}

// ── Gate 9: read_only frozen ───────────────────────────────────────────────

func TestP5_Gate9_ReadOnlyFrozen(t *testing.T) {
	h := newHarness(t)
	_, _, posterTok, workerTok, _, convID := setupAssignedTask(t, h)

	// Flip read_only manually (simulating cron)
	_, err := h.pool.Exec(context.Background(), `UPDATE conversations SET read_only=true, read_only_at=now() WHERE id=$1::uuid`, convID)
	require.NoError(t, err)

	for _, tok := range []string{posterTok, workerTok} {
		rec := h.do("POST", "/conversations/"+convID+"/messages", tok, "", map[string]any{"body": "after freeze"})
		require.Equal(t, 403, rec.Code)
		require.Contains(t, rec.Body.String(), "chat_frozen")
	}
	// Ensure no message was created
	rec := h.do("GET", "/conversations/"+convID+"/messages", posterTok, "", nil)
	msgs := decodeBody(t, rec)["messages"].([]any)
	for _, m := range msgs {
		require.NotEqual(t, "after freeze", m.(map[string]any)["body"])
	}

	// Image also blocked
	rec = h.do("POST", "/conversations/"+convID+"/images/grant", posterTok, "", map[string]any{"content_type": "image/jpeg"})
	// Grant itself should succeed (it's just signing), but send should be blocked
	if rec.Code == 200 {
		grant := decodeBody(t, rec)
		img := testJPEG(t, 50, 50)
		b64 := base64.StdEncoding.EncodeToString(img)
		rec = h.do("POST", "/conversations/"+convID+"/images", posterTok, "", map[string]any{
			"object": grant["object"], "signature": grant["signature"], "expires_at": grant["expires_at"], "image_base64": b64,
		})
		require.Equal(t, 403, rec.Code)
		require.Contains(t, rec.Body.String(), "chat_frozen")
	}
}

// ── Gate 10: blocked cannot send ───────────────────────────────────────────

func TestP5_Gate10_BlockedCannotSend(t *testing.T) {
	h := newHarness(t)
	idP, posterTok, _ := h.signup(t, uniq("p5p10")+"@example.com", "password123", "Poster P5-10")
	idW, workerTok, _ := h.signup(t, uniq("p5w10")+"@example.com", "password123", "Worker P5-10")
	taskID := fundTaskForPhase4(t, h, posterTok, validTaskInput())
	_, out := makeOffer(t, h, workerTok, taskID, nil, "")
	offerID := out["offer"].(map[string]any)["id"].(string)
	rec := acceptOffer(t, h, posterTok, offerID, "p5-10-"+uniq("k"))
	convID := decodeBody(t, rec)["accept"].(map[string]any)["conversation_id"].(string)

	// Poster blocks worker
	rec = h.do("POST", "/users/"+idW+"/block", posterTok, "", nil)
	require.Equal(t, 200, rec.Code)

	// Neither can send
	for _, tok := range []string{posterTok, workerTok} {
		rec = h.do("POST", "/conversations/"+convID+"/messages", tok, "", map[string]any{"body": "blocked hi"})
		require.Equal(t, 403, rec.Code)
	}

	// Reverse direction also (worker blocks poster) should also block, test with new pair
	h2 := newHarness(t)
	idP2, posterTok2, _ := h2.signup(t, uniq("p5p10b")+"@example.com", "password123", "Poster P5-10b")
	idW2, workerTok2, _ := h2.signup(t, uniq("p5w10b")+"@example.com", "password123", "Worker P5-10b")
	taskID2 := fundTaskForPhase4(t, h2, posterTok2, validTaskInput())
	_, out2 := makeOffer(t, h2, workerTok2, taskID2, nil, "")
	rec = acceptOffer(t, h2, posterTok2, out2["offer"].(map[string]any)["id"].(string), "p5-10b-"+uniq("k"))
	convID2 := decodeBody(t, rec)["accept"].(map[string]any)["conversation_id"].(string)
	rec = h2.do("POST", "/users/"+idP2+"/block", workerTok2, "", nil)
	require.Equal(t, 200, rec.Code)
	rec = h2.do("POST", "/conversations/"+convID2+"/messages", workerTok2, "", map[string]any{"body": "hi"})
	require.Equal(t, 403, rec.Code)
	rec = h2.do("POST", "/conversations/"+convID2+"/messages", posterTok2, "", map[string]any{"body": "hi"})
	require.Equal(t, 403, rec.Code)
	_ = idP
	_ = idW2
}

// ── Gate 11: no client system message ──────────────────────────────────────

func TestP5_Gate11_NoSystemFromClient(t *testing.T) {
	h := newHarness(t)
	_, _, posterTok, _, _, convID := setupAssignedTask(t, h)

	// Try to send with type system
	rec := h.do("POST", "/conversations/"+convID+"/messages", posterTok, "", map[string]any{"body": "fake system", "type": "system"})
	require.Equal(t, 403, rec.Code)

	// Try direct DB insert as restricted role should be rejected (RLS)
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
	_, err = conn.Exec(ctx, `INSERT INTO messages (conversation_id, sender_id, type, body, system_event) VALUES ($1::uuid, NULL, 'system', 'hacked', 'accepted')`, convID)
	require.Error(t, err)
}

// ── Gate 12: system message immutable ──────────────────────────────────────

func TestP5_Gate12_SystemImmutable(t *testing.T) {
	h := newHarness(t)
	_, _, posterTok, _, _, convID := setupAssignedTask(t, h)
	rec := h.do("GET", "/conversations/"+convID+"/messages", posterTok, "", nil)
	msgs := decodeBody(t, rec)["messages"].([]any)
	var sysID string
	for _, m := range msgs {
		if m.(map[string]any)["type"] == "system" {
			sysID = m.(map[string]any)["id"].(string)
		}
	}
	require.NotEmpty(t, sysID)

	// No HTTP endpoint to update/delete messages, so PUT should 404
	rec = h.do("PUT", "/messages/"+sysID, posterTok, "", map[string]any{"body": "edited"})
	require.Equal(t, 404, rec.Code)

	rec = h.do("DELETE", "/messages/"+sysID, posterTok, "", nil)
	require.Equal(t, 404, rec.Code)

	// Direct DB update as restricted should be denied
	ctx := context.Background()
	conn, err := h.pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, `SET ROLE phase0_restricted`)
	require.NoError(t, err)
	defer func() { _, _ = conn.Exec(context.Background(), `RESET ROLE`) }()
	tag, err := conn.Exec(ctx, `UPDATE messages SET body='hacked' WHERE id=$1::uuid`, sysID)
	require.NoError(t, err)
	require.Equal(t, int64(0), tag.RowsAffected())
	tag, err = conn.Exec(ctx, `DELETE FROM messages WHERE id=$1::uuid`, sysID)
	require.NoError(t, err)
	require.Equal(t, int64(0), tag.RowsAffected())
}

// ── Gate 13: rate limit 30/min ─────────────────────────────────────────────

func TestP5_Gate13_RateLimit(t *testing.T) {
	h := newHarness(t)
	_, _, posterTok, _, _, convID := setupAssignedTask(t, h)

	// Send 30 should succeed, 31st should be 429
	for i := 0; i < 30; i++ {
		rec := h.do("POST", "/conversations/"+convID+"/messages", posterTok, "", map[string]any{"body": fmt.Sprintf("msg %d", i)})
		require.Equal(t, 201, rec.Code, "msg %d should succeed", i)
	}
	rec := h.do("POST", "/conversations/"+convID+"/messages", posterTok, "", map[string]any{"body": "msg 30"})
	require.Equal(t, 429, rec.Code)
}

// ── Gate 14: HTML sanitized ────────────────────────────────────────────────

func TestP5_Gate14_Sanitize(t *testing.T) {
	h := newHarness(t)
	_, _, posterTok, workerTok, _, convID := setupAssignedTask(t, h)

	payload := `<b>hello</b> <script>alert(1)</script> world <img src=x onerror=alert(2)>`
	rec := h.do("POST", "/conversations/"+convID+"/messages", posterTok, "", map[string]any{"body": payload})
	require.Equal(t, 201, rec.Code)
	body := decodeBody(t, rec)["message"].(map[string]any)["body"].(string)
	require.NotContains(t, body, "<")
	require.NotContains(t, body, ">")
	require.NotContains(t, body, "<script")
	require.Contains(t, body, "hello")
	require.Contains(t, body, "world")

	// Verify other party sees same sanitized
	msgs := getConvMessages(t, h, workerTok, convID)
	found := false
	for _, m := range msgs {
		if m.(map[string]any)["body"] == body {
			found = true
			require.NotContains(t, m.(map[string]any)["body"].(string), "<")
		}
	}
	require.True(t, found)

	// Length cap 2000 after sanitization
	long := strings.Repeat("a", 2001)
	rec = h.do("POST", "/conversations/"+convID+"/messages", posterTok, "", map[string]any{"body": long})
	require.Equal(t, 400, rec.Code)
}

// ── Gate 15: re-run Phase 3/4 block checks via real endpoint ───────────────

func TestP5_Gate15_BlockViaEndpointAffectsFeedAndOffers(t *testing.T) {
	// Feed exclusion via real block endpoint
	h := newHarness(t)
	idPoster, posterTok, _ := h.signup(t, uniq("p5p15a")+"@example.com", "password123", "Poster P5-15a")
	_, otherTok, _ := h.signup(t, uniq("p5o15")+"@example.com", "password123", "Other P5-15")
	cat := uniq("p5cat15")
	taskID := fundTaskForPhase4(t, h, posterTok, geoInput(51.5, -0.12, cat, 2000, "asap"))

	// Other can see task before block
	code, out := getFeed(t, h, otherTok, "?category="+cat)
	require.Equal(t, 200, code)
	require.Contains(t, fmt.Sprint(out["tasks"]), taskID)

	// Create real block via endpoint (other blocks poster)
	otherID := getUserID(t, h, otherTok)
	rec := h.do("POST", "/users/"+idPoster+"/block", otherTok, "", nil)
	require.Equal(t, 200, rec.Code)

	// Now feed should hide
	code, out = getFeed(t, h, otherTok, "?category="+cat)
	require.Equal(t, 200, code)
	require.NotContains(t, fmt.Sprint(out["tasks"]), taskID)

	// Offer creation via real block should also be rejected
	_, err := h.pool.Exec(context.Background(), `DELETE FROM blocks WHERE blocker_id=$1::uuid AND blocked_id=$2::uuid`, otherID, idPoster)
	require.NoError(t, err)
	// Need to test offer block via endpoint: poster blocks worker
	idWorker, workerTok, _ := h.signup(t, uniq("p5w15")+"@example.com", "password123", "Worker P5-15")
	// Worker tries to offer before block should succeed
	taskID2 := fundTaskForPhase4(t, h, posterTok, geoInput(51.5, -0.12, cat, 2000, "asap"))
	codeOffer, _ := makeOffer(t, h, workerTok, taskID2, nil, "")
	require.Equal(t, 201, codeOffer)

	// Now poster blocks worker via endpoint
	rec = h.do("POST", "/users/"+idWorker+"/block", posterTok, "", nil)
	require.Equal(t, 200, rec.Code)

	// New task from same poster, worker should be blocked
	taskID3 := fundTaskForPhase4(t, h, posterTok, geoInput(51.5, -0.12, cat, 2000, "asap"))
	codeOffer, _ = makeOffer(t, h, workerTok, taskID3, nil, "")
	require.Equal(t, 403, codeOffer)

	// Also messaging block
	_, _, _, _, _, convID := setupAssignedTaskWithIDs(t, h)
	// Use the same poster/worker from setup
	// Instead create new assigned task for block messaging test
	h2 := newHarness(t)
	_, pTok2, _ := h2.signup(t, uniq("p5p15c")+"@example.com", "password123", "Poster P5-15c")
	_, wTok2, _ := h2.signup(t, uniq("p5w15c")+"@example.com", "password123", "Worker P5-15c")
	tid := fundTaskForPhase4(t, h2, pTok2, validTaskInput())
	_, out = makeOffer(t, h2, wTok2, tid, nil, "")
	rec = acceptOffer(t, h2, pTok2, out["offer"].(map[string]any)["id"].(string), "p5-15-"+uniq("k"))
	convID = decodeBody(t, rec)["accept"].(map[string]any)["conversation_id"].(string)
	// Get IDs
	pID2 := getUserID(t, h2, pTok2)
	wID2 := getUserID(t, h2, wTok2)
	rec = h2.do("POST", "/users/"+wID2+"/block", pTok2, "", nil)
	require.Equal(t, 200, rec.Code)
	rec = h2.do("POST", "/conversations/"+convID+"/messages", pTok2, "", map[string]any{"body": "blocked"})
	require.Equal(t, 403, rec.Code)
	rec = h2.do("POST", "/conversations/"+convID+"/messages", wTok2, "", map[string]any{"body": "blocked"})
	require.Equal(t, 403, rec.Code)
	_ = pID2
}

func getUserID(t *testing.T, h *harness, tok string) string {
	t.Helper()
	rec := h.do("GET", "/me", tok, "", nil)
	require.Equal(t, 200, rec.Code)
	return decodeBody(t, rec)["user"].(map[string]any)["id"].(string)
}

func setupAssignedTaskWithIDs(t *testing.T, h *harness) (string, string, string, string, string, string) {
	return setupAssignedTask(t, h)
}

// ── Gate 16: only own blocks visible ───────────────────────────────────────

func TestP5_Gate16_OnlyOwnBlocks(t *testing.T) {
	h := newHarness(t)
	_, tokA, _ := h.signup(t, uniq("p5a16")+"@example.com", "password123", "User A16")
	idB, tokB, _ := h.signup(t, uniq("p5b16")+"@example.com", "password123", "User B16")
	idC, _, _ := h.signup(t, uniq("p5c16")+"@example.com", "password123", "User C16")
	_, tokD, _ := h.signup(t, uniq("p5d16")+"@example.com", "password123", "User D16")

	// A blocks B
	rec := h.do("POST", "/users/"+idB+"/block", tokA, "", nil)
	require.Equal(t, 200, rec.Code)
	// B blocks C
	rec = h.do("POST", "/users/"+idC+"/block", tokB, "", nil)
	require.Equal(t, 200, rec.Code)
	rec = h.do("GET", "/blocks", tokA, "", nil)
	require.Equal(t, 200, rec.Code)
	blocksA := decodeBody(t, rec)["blocks"].([]any)
	require.Len(t, blocksA, 1)
	require.Equal(t, idB, blocksA[0])

	rec = h.do("GET", "/blocks", tokD, "", nil)
	require.Equal(t, 200, rec.Code)
	require.Len(t, decodeBody(t, rec)["blocks"].([]any), 0)

	// Ensure B's blocks not visible to A
	// Create B2 that blocks C, verify A doesn't see it
	// Use a separate harness for B/C
	h2 := newHarness(t)
	_, tokB2, _ := h2.signup(t, uniq("p5b16c")+"@example.com", "password123", "User B16c")
	idC2, _, _ := h2.signup(t, uniq("p5c16c")+"@example.com", "password123", "User C16c")
	rec = h2.do("POST", "/users/"+idC2+"/block", tokB2, "", nil)
	require.Equal(t, 200, rec.Code)
	// A still only sees B, not C
	rec = h.do("GET", "/blocks", tokA, "", nil)
	require.Len(t, decodeBody(t, rec)["blocks"].([]any), 1)
	_ = idC
}

// ── Self-audit: only messaging writes system rows ───────────────────────────

func TestP5_SelfAudit_SystemMessageOnlyViaShared(t *testing.T) {
	// Verify Phase 4's offers.go now uses shared function, not direct insert
	rawOffers, err := os.ReadFile(filepath.Join("..", "internal", "offers", "offers.go"))
	require.NoError(t, err)
	require.Contains(t, string(rawOffers), "InsertSystemMessageTx")
	require.NotContains(t, string(rawOffers), "INSERT INTO messages")

	rawMsg, err := os.ReadFile(filepath.Join("..", "internal", "messaging", "messaging.go"))
	require.NoError(t, err)
	require.Contains(t, string(rawMsg), "INSERT INTO messages")
	require.Contains(t, string(rawMsg), "validSystemEvents")

	// Ensure httpapi does not directly insert system messages
	rawHTTP, err := os.ReadFile(filepath.Join("..", "internal", "httpapi", "messaging.go"))
	require.NoError(t, err)
	require.NotContains(t, string(rawHTTP), "system_event")
	require.NotContains(t, string(rawHTTP), "'system'")

	// Ensure no other file writes type system except messaging
	root, _ := filepath.Abs("..")
	matches := 0
	filepath.Walk(filepath.Join(root, "internal"), func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") {
			return nil
		}
		raw, _ := os.ReadFile(p)
		if strings.Contains(string(raw), "'system'") || strings.Contains(string(raw), "\"system\"") {
			// Only messaging should contain system type literal for insert
			if !strings.Contains(p, "messaging") {
				matches++
			}
		}
		return nil
	})
	// Allow offers to contain the word system in comment, but not direct insert
	require.Equal(t, 0, matches, "only messaging should write system messages")
}

func TestP5_Docs_OpenAPI(t *testing.T) {
	h := newHarness(t)
	rec := h.do("GET", "/openapi.yaml", "", "", nil)
	require.Equal(t, 200, rec.Code)
	body := rec.Body.String()
	for _, p := range []string{
		"/conversations", "/conversations/{id}/messages", "/conversations/{id}/images/grant",
		"/messages/{id}/delivered", "/messages/{id}/read",
		"/users/{id}/block", "/blocks", "/conversations/{id}/report",
	} {
		require.Contains(t, body, p, "spec must document %s", p)
	}
}

func TestP5_Docs_Scalar(t *testing.T) {
	h := newHarness(t)
	rec := h.do("GET", "/docs", "", "", nil)
	require.Equal(t, 200, rec.Code)
	require.Contains(t, rec.Body.String(), "scalar")
}
