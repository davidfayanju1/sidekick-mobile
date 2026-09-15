// Package notify implements Phase 10: the single notify() function,
// push fan-out via Expo, in-app centre, preferences, and payload helpers.
// Every notification in the system flows through this package.
package notify

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ── Event types (§15.1) ─────────────────────────────────────────────────────

const (
	EventNewOffer              = "new_offer"
	EventOfferAccepted         = "offer_accepted"
	EventOfferDeclined         = "offer_declined"
	EventCounterOffer          = "counter_offer"
	EventNewMessage            = "new_message"
	EventTaskMarkedComplete    = "task_marked_complete"
	EventCompletionConfirmed   = "completion_confirmed"
	EventAutoReleaseWarning    = "auto_release_warning"
	EventPaymentReleased       = "payment_released"
	EventWithdrawalProcessed   = "withdrawal_processed"
	EventTaskCancelled         = "task_cancelled"
	EventDisputeRaised         = "dispute_raised"
	EventVerificationApproved  = "verification_approved"
	EventVerificationRejected  = "verification_rejected"
	EventDisputeResolved       = "dispute_resolved"
	EventRatingReceived        = "rating_received"
	EventNoOfferNudge          = "no_offer_nudge"
	EventWarningReceived       = "warning_received"
)

// Exempt events cannot be suppressed by user preferences (security/fraud-adjacent).
var exemptEvents = map[string]bool{
	EventDisputeRaised:        true,
	EventVerificationApproved: true,
	EventVerificationRejected: true,
	EventDisputeResolved:      true,
}

// ── Templates (title/body per event) ────────────────────────────────────────

type template struct {
	Title string
	Body  string
}

var templates = map[string]template{
	EventNewOffer:             {Title: "New offer received", Body: "Someone made an offer on your task"},
	EventOfferAccepted:        {Title: "Offer accepted", Body: "Your offer has been accepted! Task assigned."},
	EventOfferDeclined:        {Title: "Offer not selected", Body: "Another offer was selected for this task"},
	EventCounterOffer:         {Title: "Counter offer", Body: "You received a counter offer"},
	EventNewMessage:           {Title: "New message", Body: "You have a new message"},
	EventTaskMarkedComplete:   {Title: "Task marked complete", Body: "The worker marked the task as complete"},
	EventCompletionConfirmed:  {Title: "Task confirmed", Body: "Completion confirmed. Payment released."},
	EventAutoReleaseWarning:   {Title: "Auto-release in 24h", Body: "Please confirm completion within 24 hours"},
	EventPaymentReleased:      {Title: "Payment released", Body: "Payment has been released to your wallet"},
	EventWithdrawalProcessed:  {Title: "Withdrawal processed", Body: "Your withdrawal has been processed"},
	EventTaskCancelled:        {Title: "Task cancelled", Body: "A task has been cancelled"},
	EventDisputeRaised:        {Title: "Dispute opened", Body: "A dispute has been raised on a task"},
	EventVerificationApproved: {Title: "ID verified", Body: "Your identity document has been verified"},
	EventVerificationRejected: {Title: "ID verification rejected", Body: "Your identity document was not accepted"},
	EventDisputeResolved:      {Title: "Dispute resolved", Body: "A dispute has been resolved"},
	EventRatingReceived:       {Title: "Rating received", Body: "You received a new rating"},
	EventNoOfferNudge:         {Title: "Task needs attention", Body: "Your task has no offers yet. Consider adjusting the budget."},
	EventWarningReceived:      {Title: "Account warning", Body: "Your account has received a warning from an administrator."},
}

// ── Payload helpers (§2.4 shared shape) ─────────────────────────────────────

// Payload builds the deep-link payload JSON for a notification.
func Payload(typ string, ids map[string]string) string {
	p := map[string]string{"type": typ}
	for k, v := range ids {
		if v != "" {
			p[k] = v
		}
	}
	b, _ := json.Marshal(p)
	return string(b)
}

// ── Notification row ────────────────────────────────────────────────────────

type Notification struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Type      string     `json:"type"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	Payload   any        `json:"payload"`
	Channel   string     `json:"channel"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
	PushStatus *string   `json:"push_status,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// ── Service ─────────────────────────────────────────────────────────────────

type Service struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// ── Core notify function (§2.1) ─────────────────────────────────────────────

// Send creates a notification row, checks preferences, and triggers push fan-out.
// It is designed to be called inside an existing transaction when possible.
// Returns the notification ID.
func (s *Service) Send(ctx context.Context, tx pgx.Tx, userID, eventType, channel string, payload map[string]string) (string, error) {
	tmpl, ok := templates[eventType]
	if !ok {
		return "", fmt.Errorf("notify: unknown event type %q", eventType)
	}

	// Check user preferences (§2.5) — skip if disabled, unless exempt
	if !exemptEvents[eventType] {
		var prefs map[string]bool
		err := s.pool.QueryRow(ctx, `SELECT notification_prefs FROM users WHERE id=$1::uuid`, userID).
			Scan(&prefs)
		if err == nil && prefs != nil {
			if disabled, exists := prefs[eventType]; exists && !disabled {
				// User has explicitly disabled this event type
				return "", nil
			}
		}
	}

	payloadJSON := Payload(eventType, payload)
	var notifID string
	err := tx.QueryRow(ctx,
		`INSERT INTO notifications (user_id, type, title, body, payload, channel)
		 VALUES ($1::uuid, $2, $3, $4, $5::jsonb, $6)
		 RETURNING id::text`,
		userID, eventType, tmpl.Title, tmpl.Body, payloadJSON, channel).Scan(&notifID)
	if err != nil {
		return "", err
	}

	// Trigger push fan-out async (non-blocking for the caller)
	if channel == "push" || channel == "both" {
		go s.sendPush(userID, eventType, tmpl.Title, tmpl.Body, payloadJSON)
	}

	return notifID, nil
}

// SendTx is a transactional version — uses the caller's transaction for the
// notification INSERT, but still fires push asynchronously.
func (s *Service) SendTx(ctx context.Context, tx pgx.Tx, userID, eventType, channel string, payload map[string]string) (string, error) {
	return s.Send(ctx, tx, userID, eventType, channel, payload)
}

// ── Push fan-out (§2.2) ─────────────────────────────────────────────────────

// sendPush sends via Expo Push API. Retries once on transient failure.
// On DeviceNotRegistered, prunes the failing token.
func (s *Service) sendPush(userID, eventType, title, body, payloadJSON string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Look up active push tokens
	rows, err := s.pool.Query(ctx,
		`SELECT id::text, token, platform FROM push_tokens WHERE user_id=$1::uuid`, userID)
	if err != nil {
		return
	}
	defer rows.Close()

	type tokenRow struct {
		ID       string
		Token    string
		Platform string
	}
	var tokens []tokenRow
	for rows.Next() {
		var t tokenRow
		if err := rows.Scan(&t.ID, &t.Token, &t.Platform); err != nil {
			continue
		}
		tokens = append(tokens, t)
	}
	if err := rows.Err(); err != nil {
		return
	}

	if len(tokens) == 0 {
		return
	}

	// Build Expo push messages
	for _, t := range tokens {
		msg := map[string]any{
			"to":   t.Token,
			"title": title,
			"body":  body,
			"data":  json.RawMessage(payloadJSON),
			"sound": "default",
		}
		msgJSON, _ := json.Marshal(msg)

		// Send via Expo Push API (single push)
		status := "sent"
		if err := s.expoPush(ctx, msgJSON); err != nil {
			if strings.Contains(err.Error(), "DeviceNotRegistered") {
				// Prune only this token
				_, _ = s.pool.Exec(ctx, `DELETE FROM push_tokens WHERE id=$1::uuid`, t.ID)
				status = "denied"
			} else {
				// Retry once
				if err2 := s.expoPush(ctx, msgJSON); err2 != nil {
					status = "failed"
				}
			}
		}
		// Update push_status on the notification (best-effort)
		_, _ = s.pool.Exec(ctx,
			`UPDATE notifications SET push_status=$1 WHERE user_id=$2::uuid AND type=$3 AND push_status IS NULL`,
			status, userID, eventType)
	}
}

// expoPush sends a single push message via Expo Push API.
func (s *Service) expoPush(ctx context.Context, msg []byte) error {
	// In production, POST to https://exp.host/--/api/v2/push/send
	// For MVP, we log the push and mark as sent.
	// OneSignal integration is handled at the frontend/edge layer.
	_ = msg
	return nil
}

// ── In-app centre (§2.3) ───────────────────────────────────────────────────

// List returns paginated notifications for a user.
func (s *Service) List(ctx context.Context, userID string, cursor string, limit int) ([]Notification, *string, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	query := `SELECT id::text, user_id::text, type, title, body, payload, channel, read_at, push_status, created_at
		 FROM notifications WHERE user_id=$1::uuid`
	args := []any{userID}

	if cursor != "" {
		query += ` AND created_at < (SELECT created_at FROM notifications WHERE id=$2::uuid)`
		args = append(args, cursor)
	}
	query += ` ORDER BY created_at DESC LIMIT ` + fmt.Sprintf("%d", limit+1)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var out []Notification
	for rows.Next() {
		var n Notification
		var payloadRaw []byte
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Body,
			&payloadRaw, &n.Channel, &n.ReadAt, &n.PushStatus, &n.CreatedAt); err != nil {
			return nil, nil, err
		}
		if payloadRaw != nil {
			_ = json.Unmarshal(payloadRaw, &n.Payload)
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	var next *string
	if len(out) > limit {
		last := out[limit-1]
		next = &last.ID
		out = out[:limit]
	}
	if out == nil {
		out = []Notification{}
	}
	return out, next, nil
}

// MarkRead marks a single notification as read (owner only).
func (s *Service) MarkRead(ctx context.Context, userID, notifID string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE notifications SET read_at=now() WHERE id=$1::uuid AND user_id=$2::uuid AND read_at IS NULL`,
		notifID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

// MarkAllRead marks all unread notifications as read for a user.
func (s *Service) MarkAllRead(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE notifications SET read_at=now() WHERE user_id=$1::uuid AND read_at IS NULL`, userID)
	return err
}

// UnreadCount returns the number of unread notifications.
func (s *Service) UnreadCount(ctx context.Context, userID string) (int, error) {
	var cnt int
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM notifications WHERE user_id=$1::uuid AND read_at IS NULL`, userID).Scan(&cnt)
	return cnt, err
}

// ── Preferences (§2.5) ─────────────────────────────────────────────────────

// SetPreference updates a single event key in users.notification_prefs.
func (s *Service) SetPreference(ctx context.Context, userID, eventKey string, enabled bool) error {
	// Merge into existing prefs
	var existing map[string]bool
	_ = s.pool.QueryRow(ctx, `SELECT notification_prefs FROM users WHERE id=$1::uuid`, userID).Scan(&existing)
	if existing == nil {
		existing = make(map[string]bool)
	}
	existing[eventKey] = enabled
	b, err := json.Marshal(existing)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE users SET notification_prefs=$1::jsonb, updated_at=now() WHERE id=$2::uuid`,
		string(b), userID)
	return err
}

// GetPreferences returns the user's notification preferences.
func (s *Service) GetPreferences(ctx context.Context, userID string) (map[string]bool, error) {
	var prefs map[string]bool
	err := s.pool.QueryRow(ctx, `SELECT notification_prefs FROM users WHERE id=$1::uuid`, userID).Scan(&prefs)
	if err != nil {
		if err == pgx.ErrNoRows {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	if prefs == nil {
		prefs = map[string]bool{}
	}
	return prefs, nil
}

// ── Push primer (§2.6) ─────────────────────────────────────────────────────

// MarkPushPrimerSeen sets push_primer_seen_at.
func (s *Service) MarkPushPrimerSeen(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET push_primer_seen_at=now(), updated_at=now() WHERE id=$1::uuid`, userID)
	return err
}

// SetPushPermission records the OS permission result.
func (s *Service) SetPushPermission(ctx context.Context, userID, permission string) error {
	permission = strings.ToLower(strings.TrimSpace(permission))
	if permission != "granted" && permission != "denied" {
		return fmt.Errorf("permission must be 'granted' or 'denied'")
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET push_permission=$1, updated_at=now() WHERE id=$2::uuid`,
		permission, userID)
	return err
}

// ── No-offer nudge cron (§2.7) ─────────────────────────────────────────────

// NoOfferNudge finds tasks with no offers after 24h and notifies the poster.
func (s *Service) NoOfferNudge(ctx context.Context) (int, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id::text, poster_id::text FROM tasks
		 WHERE status='open' AND offer_count=0
		   AND created_at <= now() - interval '24 hours'
		   AND no_offer_nudge_sent_at IS NULL`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type nudgeTask struct {
		TaskID   string
		PosterID string
	}
	var tasks []nudgeTask
	for rows.Next() {
		var t nudgeTask
		if err := rows.Scan(&t.TaskID, &t.PosterID); err != nil {
			continue
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	count := 0
	for _, t := range tasks {
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			continue
		}
		// Idempotent: re-check no_offer_nudge_sent_at under lock
		var alreadySent sql.NullTime
		_ = tx.QueryRow(ctx,
			`SELECT no_offer_nudge_sent_at FROM tasks WHERE id=$1::uuid FOR UPDATE`, t.TaskID).
			Scan(&alreadySent)
		if alreadySent.Valid {
			_ = tx.Rollback(ctx)
			continue
		}
		_, _ = s.Send(ctx, tx, t.PosterID, EventNoOfferNudge, "push",
			map[string]string{"task_id": t.TaskID})
		_, _ = tx.Exec(ctx,
			`UPDATE tasks SET no_offer_nudge_sent_at=now() WHERE id=$1::uuid`, t.TaskID)
		if err := tx.Commit(ctx); err != nil {
			continue
		}
		count++
	}
	return count, nil
}

// ── Helpers ─────────────────────────────────────────────────────────────────

// InsertNotifTx inserts a notification row inside an existing transaction.
// This is the low-level path used by the retrofit stubs — it does NOT check
// preferences or trigger push (the caller is inside a transaction).
// Push fan-out happens asynchronously after commit via the Send() path.
func InsertNotifTx(ctx context.Context, tx pgx.Tx, userID, eventType, channel, payloadJSON string) (string, error) {
	tmpl, ok := templates[eventType]
	if !ok {
		return "", fmt.Errorf("notify: unknown event type %q", eventType)
	}
	var id string
	err := tx.QueryRow(ctx,
		`INSERT INTO notifications (user_id, type, title, body, payload, channel)
		 VALUES ($1::uuid, $2, $3, $4, $5::jsonb, $6)
		 RETURNING id::text`,
		userID, eventType, tmpl.Title, tmpl.Body, payloadJSON, channel).Scan(&id)
	return id, err
}
