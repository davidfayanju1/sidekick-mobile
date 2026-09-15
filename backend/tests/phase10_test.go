package tests

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Phase 10 — Notifications Backend (12 gate tests)
// ═══════════════════════════════════════════════════════════════════════════════

func intPtr(n int) *int { return &n }

func p10TaskInput() map[string]any {
	return map[string]any{
		"title":              "Test notification task",
		"description":        "This is a test task for notifications with enough chars",
		"category":           "cleaning",
		"location_approx":    "Peckham",
		"location_lat":       51.4741,
		"location_lng":       -0.0697,
		"location_exact":     "14 Rye Lane, London SE15",
		"location_exact_lat": 51.4749,
		"location_exact_lng": -0.0685,
		"timing_type":        "asap",
		"budget":             2500,
	}
}

func TestP10_01_NotificationPrefs_Default(t *testing.T) {
	h := newHarness(t)
	_, tok, _ := h.signup(t, uniq("p10e1")+"@example.com", "password123", "Prefs Default User")

	resp := h.do("GET", "/me/notification-prefs", tok, "", nil)
	require.Equal(t, 200, resp.Code, resp.Body.String())
	body := decodeBody(t, resp)
	prefs, _ := body["preferences"].(map[string]any)
	if prefs == nil {
		prefs = map[string]any{}
	}
	require.Empty(t, prefs, "expected empty prefs by default")
}

func TestP10_02_SetNotificationPrefs(t *testing.T) {
	h := newHarness(t)
	_, tok, _ := h.signup(t, uniq("p10e2")+"@example.com", "password123", "Set Prefs User")

	resp := h.do("PATCH", "/me/notification-prefs", tok, "", map[string]any{
		"new_offer":      true,
		"offer_accepted": true,
		"new_message":    false,
	})
	require.Equal(t, 200, resp.Code, resp.Body.String())

	resp2 := h.do("GET", "/me/notification-prefs", tok, "", nil)
	require.Equal(t, 200, resp2.Code)
	body := decodeBody(t, resp2)
	prefs := body["preferences"].(map[string]any)
	require.Equal(t, true, prefs["new_offer"])
	require.Equal(t, true, prefs["offer_accepted"])
	require.Equal(t, false, prefs["new_message"])
}

func TestP10_03_InlineNotification_Created(t *testing.T) {
	h := newHarness(t)
	_, posterTok, _ := h.signup(t, uniq("p10p3")+"@example.com", "password123", "Notif Poster")
	_, workerTok, _ := h.signup(t, uniq("p10w3")+"@example.com", "password123", "Notif Worker")

	taskID := fundTaskForPhase4(t, h, posterTok, p10TaskInput())

	code, out := makeOffer(t, h, workerTok, taskID, intPtr(1500), "I can do it")
	require.Equal(t, 201, code)
	offerID := out["offer"].(map[string]any)["id"].(string)

	acceptResp := acceptOffer(t, h, posterTok, offerID, "idem-p10-3-"+uniq(""))
	require.Equal(t, 200, acceptResp.Code, acceptResp.Body.String())

	notifResp := h.do("GET", "/me/notifications", workerTok, "", nil)
	require.Equal(t, 200, notifResp.Code)
	notifBody := decodeBody(t, notifResp)
	notifications := notifBody["notifications"].([]any)
	found := false
	for _, raw := range notifications {
		n := raw.(map[string]any)
		if n["type"] == "offer_accepted" {
			found = true
			break
		}
	}
	require.True(t, found, "expected offer_accepted notification for worker")
}

func TestP10_04_Notification_MarkRead(t *testing.T) {
	h := newHarness(t)
	_, posterTok, _ := h.signup(t, uniq("p10p4")+"@example.com", "password123", "Read Poster")
	_, workerTok, _ := h.signup(t, uniq("p10w4")+"@example.com", "password123", "Read Worker")

	taskID := fundTaskForPhase4(t, h, posterTok, p10TaskInput())

	code, out := makeOffer(t, h, workerTok, taskID, intPtr(1500), "offer for read")
	require.Equal(t, 201, code)
	offerID := out["offer"].(map[string]any)["id"].(string)
	acceptResp := acceptOffer(t, h, posterTok, offerID, "idem-p10-4-"+uniq(""))
	require.Equal(t, 200, acceptResp.Code)

	notifResp := h.do("GET", "/me/notifications", workerTok, "", nil)
	require.Equal(t, 200, notifResp.Code)
	notifBody := decodeBody(t, notifResp)
	notifications := notifBody["notifications"].([]any)
	var notifID string
	for _, raw := range notifications {
		n := raw.(map[string]any)
		if n["type"] == "offer_accepted" {
			notifID = n["id"].(string)
			break
		}
	}
	require.NotEmpty(t, notifID, "no offer_accepted notification found")

	readResp := h.do("POST", fmt.Sprintf("/me/notifications/%s/read", notifID), workerTok, "", nil)
	require.Equal(t, 200, readResp.Code, readResp.Body.String())

	countResp := h.do("GET", "/me/notifications/unread-count", workerTok, "", nil)
	require.Equal(t, 200, countResp.Code)
	countBody := decodeBody(t, countResp)
	require.Equal(t, float64(0), countBody["unread_count"], "expected 0 unread after mark-read")
}

func TestP10_05_Notification_MarkAllRead(t *testing.T) {
	h := newHarness(t)
	_, posterTok, _ := h.signup(t, uniq("p10p5")+"@example.com", "password123", "AllRead Poster")
	_, workerTok, _ := h.signup(t, uniq("p10w5")+"@example.com", "password123", "AllRead Worker")

	taskID := fundTaskForPhase4(t, h, posterTok, p10TaskInput())

	code, out := makeOffer(t, h, workerTok, taskID, intPtr(1500), "offer for allread")
	require.Equal(t, 201, code)
	offerID := out["offer"].(map[string]any)["id"].(string)
	acceptResp := acceptOffer(t, h, posterTok, offerID, "idem-p10-5-"+uniq(""))
	require.Equal(t, 200, acceptResp.Code)

	readAllResp := h.do("POST", "/me/notifications/read-all", workerTok, "", nil)
	require.Equal(t, 200, readAllResp.Code, readAllResp.Body.String())

	countResp := h.do("GET", "/me/notifications/unread-count", workerTok, "", nil)
	require.Equal(t, 200, countResp.Code)
	countBody := decodeBody(t, countResp)
	require.Equal(t, float64(0), countBody["unread_count"], "expected 0 unread after mark-all-read")
}

func TestP10_06_MarkRead_OtherUsers_Notif(t *testing.T) {
	h := newHarness(t)
	_, posterTok, _ := h.signup(t, uniq("p10p6")+"@example.com", "password123", "Other Poster")
	_, workerTok, _ := h.signup(t, uniq("p10w6")+"@example.com", "password123", "Other Worker")

	taskID := fundTaskForPhase4(t, h, posterTok, p10TaskInput())

	code, out := makeOffer(t, h, workerTok, taskID, intPtr(1500), "offer")
	require.Equal(t, 201, code)
	offerID := out["offer"].(map[string]any)["id"].(string)
	acceptResp := acceptOffer(t, h, posterTok, offerID, "idem-p10-6-"+uniq(""))
	require.Equal(t, 200, acceptResp.Code)

	notifResp := h.do("GET", "/me/notifications", workerTok, "", nil)
	require.Equal(t, 200, notifResp.Code)
	notifBody := decodeBody(t, notifResp)
	notifications := notifBody["notifications"].([]any)
	var notifID string
	for _, raw := range notifications {
		n := raw.(map[string]any)
		if n["type"] == "offer_accepted" {
			notifID = n["id"].(string)
			break
		}
	}

	resp := h.do("POST", fmt.Sprintf("/me/notifications/%s/read", notifID), posterTok, "", nil)
	require.Equal(t, 404, resp.Code, "poster should not be able to mark worker's notification as read")
}

func TestP10_07_PushPrimerSeen(t *testing.T) {
	h := newHarness(t)
	_, tok, _ := h.signup(t, uniq("p10e7")+"@example.com", "password123", "Push Primer User")

	resp := h.do("POST", "/me/push-primer-seen", tok, "", nil)
	require.Equal(t, 200, resp.Code, resp.Body.String())
}

func TestP10_08_PushPermission_Granted(t *testing.T) {
	h := newHarness(t)
	_, tok, _ := h.signup(t, uniq("p10e8")+"@example.com", "password123", "Push Granted User")

	resp := h.do("POST", "/me/push-permission", tok, "", map[string]any{
		"permission": "granted",
	})
	require.Equal(t, 200, resp.Code, resp.Body.String())
}

func TestP10_09_PushPermission_Denied(t *testing.T) {
	h := newHarness(t)
	_, tok, _ := h.signup(t, uniq("p10e9")+"@example.com", "password123", "Push Denied User")

	resp := h.do("POST", "/me/push-permission", tok, "", map[string]any{
		"permission": "denied",
	})
	require.Equal(t, 200, resp.Code, resp.Body.String())
}

func TestP10_10_PushPermission_InvalidValue(t *testing.T) {
	h := newHarness(t)
	_, tok, _ := h.signup(t, uniq("p10e10")+"@example.com", "password123", "Push Invalid User")

	resp := h.do("POST", "/me/push-permission", tok, "", map[string]any{
		"permission": "maybe",
	})
	require.Equal(t, 400, resp.Code, "expected 400 for invalid permission value")
}

func TestP10_11_DisputeNotification_Created(t *testing.T) {
	h := newHarness(t)
	_, posterTok, _ := h.signup(t, uniq("p10p11")+"@example.com", "password123", "Dispute Poster")
	_, workerTok, _ := h.signup(t, uniq("p10w11")+"@example.com", "password123", "Dispute Worker")

	taskID := fundTaskForPhase4(t, h, posterTok, p10TaskInput())

	code, out := makeOffer(t, h, workerTok, taskID, intPtr(1500), "offer for dispute")
	require.Equal(t, 201, code)
	offerID := out["offer"].(map[string]any)["id"].(string)
	acceptResp := acceptOffer(t, h, posterTok, offerID, "idem-p10-11-"+uniq(""))
	require.Equal(t, 200, acceptResp.Code)

	mcResp := h.do("POST", fmt.Sprintf("/tasks/%s/complete", taskID), workerTok, "", nil)
	require.Equal(t, 200, mcResp.Code, mcResp.Body.String())

	dispResp := h.do("POST", fmt.Sprintf("/tasks/%s/dispute", taskID), posterTok, "", map[string]any{
		"reason":      "work not done",
		"description": "The worker did not complete the task as described",
	})
	require.Equal(t, 201, dispResp.Code, dispResp.Body.String())

	notifResp := h.do("GET", "/me/notifications", workerTok, "", nil)
	require.Equal(t, 200, notifResp.Code)
	notifBody := decodeBody(t, notifResp)
	notifications := notifBody["notifications"].([]any)
	found := false
	for _, raw := range notifications {
		n := raw.(map[string]any)
		if n["type"] == "dispute_raised" {
			found = true
			break
		}
	}
	require.True(t, found, "expected dispute_raised notification for worker")
}

func TestP10_12_MarkRead_NotFound(t *testing.T) {
	h := newHarness(t)
	_, tok, _ := h.signup(t, uniq("p10e12")+"@example.com", "password123", "Notfound User")

	resp := h.do("POST", "/me/notifications/00000000-0000-0000-0000-000000000000/read", tok, "", nil)
	require.Equal(t, 404, resp.Code, "expected 404 for nonexistent notification")
}
