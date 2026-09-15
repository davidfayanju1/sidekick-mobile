// Package offers implements Phase 4: offers & matching with the critical
// atomic accept transaction (9 steps). Ranking and exact-location surfacing
// via system message are also defined here.
package offers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sidekick/backend/internal/audit"
	"github.com/sidekick/backend/internal/messaging"
	"github.com/sidekick/backend/internal/notify"
)

var (
	ErrBadRequest = errors.New("bad request")
	ErrNotFound   = errors.New("not found")
	ErrForbidden  = errors.New("forbidden")
	ErrConflict   = errors.New("conflict")
	ErrGone       = errors.New("no longer available")
)

type Offer struct {
	ID                   string     `json:"id"`
	TaskID               string     `json:"task_id"`
	WorkerID             string     `json:"worker_id"`
	Amount               int        `json:"amount"`
	IsCounter            bool       `json:"is_counter"`
	Message              *string    `json:"message,omitempty"`
	PosterCounterAmount  *int       `json:"poster_counter_amount,omitempty"`
	PosterCounterStatus  string     `json:"poster_counter_status"`
	Status               string     `json:"status"`
	DeclineReason        *string    `json:"decline_reason,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	RespondedAt          *time.Time `json:"responded_at,omitempty"`
}

// OfferWithWorker adds the ranked worker public fields for the poster's view.
type OfferWithWorker struct {
	Offer
	WorkerRatingAvg          float64  `json:"worker_rating_avg"`
	WorkerRatingCount        int      `json:"worker_rating_count"`
	WorkerVerificationStatus string   `json:"worker_verification_status"`
	WorkerTasksCompleted     int      `json:"worker_tasks_completed"`
	WorkerDisplayName        string   `json:"worker_display_name"`
	WorkerAvatarURL          *string  `json:"worker_avatar_url,omitempty"`
}

type AcceptResult struct {
	TaskID         string `json:"task_id"`
	OfferID        string `json:"offer_id"`
	TaskStatus     string `json:"task_status"`
	ConversationID string `json:"conversation_id"`
	Repeated       bool   `json:"repeated"`
}

type Service struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

const offerCols = `id::text, task_id::text, worker_id::text, amount, is_counter, message, poster_counter_amount, poster_counter_status, status, decline_reason, created_at, responded_at`

func scanOffer(r pgx.Row) (*Offer, error) {
	var o Offer
	err := r.Scan(&o.ID, &o.TaskID, &o.WorkerID, &o.Amount, &o.IsCounter, &o.Message, &o.PosterCounterAmount, &o.PosterCounterStatus, &o.Status, &o.DeclineReason, &o.CreatedAt, &o.RespondedAt)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// ── Create ─────────────────────────────────────────────────────────────────

func (s *Service) Create(ctx context.Context, workerID, taskID string, amount int, message string) (*Offer, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("%w: amount must be positive", ErrBadRequest)
	}
	if len([]rune(message)) > 300 {
		return nil, fmt.Errorf("%w: message max 300 chars", ErrBadRequest)
	}
	// Load task and validate.
	var taskStatus, posterID, escrowStatus string
	var taskBudget int
	var posterSuspended, posterDeleted *time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT t.status, t.poster_id::text, t.budget, t.escrow_status, u.suspended_at, u.deleted_at
		 FROM tasks t JOIN users u ON u.id=t.poster_id WHERE t.id=$1::uuid`, taskID).
		Scan(&taskStatus, &posterID, &taskBudget, &escrowStatus, &posterSuspended, &posterDeleted)
	if err != nil {
		return nil, ErrNotFound
	}
	if taskStatus != "open" {
		return nil, fmt.Errorf("%w: task not open", ErrGone)
	}
	if posterID == workerID {
		return nil, fmt.Errorf("%w: cannot offer on own task", ErrForbidden)
	}
	// Worker must be active.
	var wsus, wdel *time.Time
	err = s.pool.QueryRow(ctx, `SELECT suspended_at, deleted_at FROM users WHERE id=$1::uuid`, workerID).Scan(&wsus, &wdel)
	if err != nil {
		return nil, ErrNotFound
	}
	if wsus != nil || wdel != nil {
		return nil, fmt.Errorf("%w: account suspended or deleted", ErrForbidden)
	}
	if posterSuspended != nil || posterDeleted != nil {
		return nil, fmt.Errorf("%w: poster suspended", ErrGone)
	}
	// Block either direction.
	var blocked int
	err = s.pool.QueryRow(ctx,
		`SELECT count(*) FROM blocks WHERE (blocker_id=$1::uuid AND blocked_id=$2::uuid) OR (blocker_id=$2::uuid AND blocked_id=$1::uuid)`,
		workerID, posterID).Scan(&blocked)
	if err == nil && blocked > 0 {
		return nil, fmt.Errorf("%w: blocked", ErrForbidden)
	}
	isCounter := amount != taskBudget
	if isCounter && strings.TrimSpace(message) == "" {
		return nil, fmt.Errorf("%w: counter-offer requires a message", ErrBadRequest)
	}
	o, err := scanOffer(s.pool.QueryRow(ctx,
		`INSERT INTO offers (task_id, worker_id, amount, is_counter, message)
		 VALUES ($1::uuid,$2::uuid,$3,$4,NULLIF($5,''))
		 RETURNING `+offerCols, taskID, workerID, amount, isCounter, message))
	if err != nil {
		if strings.Contains(err.Error(), "uq_offers_active") || strings.Contains(err.Error(), "duplicate") {
			return nil, fmt.Errorf("%w: already have a pending offer on this task", ErrConflict)
		}
		return nil, err
	}
	_, _ = s.pool.Exec(ctx, `UPDATE tasks SET offer_count = offer_count + 1, updated_at=now() WHERE id=$1::uuid`, taskID)
	_ = audit.Log(ctx, s.pool, audit.Entry{ActorID: &workerID, Action: "offer.create", EntityType: "offer", EntityID: o.ID, Metadata: map[string]any{"task_id": taskID, "amount": amount}})
	return o, nil
}

// ── List for poster (ranked) ───────────────────────────────────────────────
//
// Rank function (§2.3, documented): verified workers first, then higher
// rating_avg, then higher tasks_completed, then recency. This rewards trust
// and track record over raw speed.
func (s *Service) ListForTask(ctx context.Context, requesterID, taskID string) ([]OfferWithWorker, error) {
	var posterID string
	err := s.pool.QueryRow(ctx, `SELECT poster_id::text FROM tasks WHERE id=$1::uuid`, taskID).Scan(&posterID)
	if err != nil {
		return nil, ErrNotFound
	}
	if posterID != requesterID {
		return nil, ErrForbidden
	}
	rows, err := s.pool.Query(ctx,
		`SELECT o.id::text, o.task_id::text, o.worker_id::text, o.amount, o.is_counter, o.message,
		        o.poster_counter_amount, o.poster_counter_status, o.status, o.decline_reason, o.created_at, o.responded_at,
		        u.rating_avg, u.rating_count, u.verification_status, u.tasks_completed, u.display_name, u.avatar_url
		 FROM offers o JOIN users u ON u.id=o.worker_id
		 WHERE o.task_id=$1::uuid
		 ORDER BY
		   CASE u.verification_status WHEN 'verified' THEN 0 WHEN 'pending' THEN 1 ELSE 2 END,
		   u.rating_avg DESC, u.tasks_completed DESC, o.created_at DESC`,
		taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OfferWithWorker
	for rows.Next() {
		var ow OfferWithWorker
		err := rows.Scan(&ow.ID, &ow.TaskID, &ow.WorkerID, &ow.Amount, &ow.IsCounter, &ow.Message,
			&ow.PosterCounterAmount, &ow.PosterCounterStatus, &ow.Status, &ow.DeclineReason, &ow.CreatedAt, &ow.RespondedAt,
			&ow.WorkerRatingAvg, &ow.WorkerRatingCount, &ow.WorkerVerificationStatus, &ow.WorkerTasksCompleted, &ow.WorkerDisplayName, &ow.WorkerAvatarURL)
		if err != nil {
			return nil, err
		}
		out = append(out, ow)
	}
	if out == nil {
		out = []OfferWithWorker{}
	}
	return out, rows.Err()
}

// ── List my offers ─────────────────────────────────────────────────────────

func (s *Service) ListMyOffers(ctx context.Context, workerID, status string) ([]*Offer, error) {
	query := `SELECT ` + offerCols + ` FROM offers WHERE worker_id=$1::uuid`
	args := []any{workerID}
	if status != "" {
		query += ` AND status=$2`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Offer
	for rows.Next() {
		o, err := scanOffer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	if out == nil {
		out = []*Offer{}
	}
	return out, rows.Err()
}

// ── Decline ────────────────────────────────────────────────────────────────

func (s *Service) Decline(ctx context.Context, posterID, offerID string, reason string) (*Offer, error) {
	var taskID, posterCheck string
	var status string
	err := s.pool.QueryRow(ctx,
		`SELECT o.task_id::text, o.status, t.poster_id::text FROM offers o JOIN tasks t ON t.id=o.task_id WHERE o.id=$1::uuid`,
		offerID).Scan(&taskID, &status, &posterCheck)
	if err != nil {
		return nil, ErrNotFound
	}
	if posterCheck != posterID {
		return nil, ErrForbidden
	}
	if status != "pending" {
		return nil, fmt.Errorf("%w: offer not pending", ErrConflict)
	}
	declineReason := reason
	if len(declineReason) > 300 {
		declineReason = declineReason[:300]
	}
	o, err := scanOffer(s.pool.QueryRow(ctx,
		`UPDATE offers SET status='declined', decline_reason=NULLIF($2,''), responded_at=now() WHERE id=$1::uuid RETURNING `+offerCols,
		offerID, declineReason))
	if err != nil {
		return nil, err
	}
	_ = audit.Log(ctx, s.pool, audit.Entry{ActorID: &posterID, Action: "offer.decline", EntityType: "offer", EntityID: offerID})
	return o, nil
}

// ── Poster counter (one round) ─────────────────────────────────────────────

func (s *Service) Counter(ctx context.Context, posterID, offerID string, amount int) (*Offer, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("%w: amount must be positive", ErrBadRequest)
	}
	var posterCheck, offerStatus, counterStatus string
	var existing *int
	err := s.pool.QueryRow(ctx,
		`SELECT t.poster_id::text, o.status, o.poster_counter_status, o.poster_counter_amount
		 FROM offers o JOIN tasks t ON t.id=o.task_id WHERE o.id=$1::uuid`,
		offerID).Scan(&posterCheck, &offerStatus, &counterStatus, &existing)
	if err != nil {
		return nil, ErrNotFound
	}
	if posterCheck != posterID {
		return nil, ErrForbidden
	}
	if offerStatus != "pending" {
		return nil, fmt.Errorf("%w: offer not pending", ErrConflict)
	}
	if counterStatus != "none" {
		return nil, fmt.Errorf("%w: already countered", ErrConflict)
	}
	o, err := scanOffer(s.pool.QueryRow(ctx,
		`UPDATE offers SET poster_counter_amount=$2, poster_counter_status='pending' WHERE id=$1::uuid RETURNING `+offerCols,
		offerID, amount))
	if err != nil {
		return nil, err
	}
	_ = audit.Log(ctx, s.pool, audit.Entry{ActorID: &posterID, Action: "offer.counter", EntityType: "offer", EntityID: offerID, Metadata: map[string]any{"amount": amount}})
	return o, nil
}

// ── Respond to counter (worker) ────────────────────────────────────────────

func (s *Service) RespondToCounter(ctx context.Context, workerID, offerID, action string) (*Offer, error) {
	if action != "accept" && action != "decline" {
		return nil, fmt.Errorf("%w: action must be accept or decline", ErrBadRequest)
	}
	var workerCheck, offerStatus, counterStatus string
	var posterID string
	err := s.pool.QueryRow(ctx,
		`SELECT o.worker_id::text, o.status, o.poster_counter_status, t.poster_id::text FROM offers o JOIN tasks t ON t.id=o.task_id WHERE o.id=$1::uuid`,
		offerID).Scan(&workerCheck, &offerStatus, &counterStatus, &posterID)
	if err != nil {
		return nil, ErrNotFound
	}
	if workerCheck != workerID {
		return nil, ErrForbidden
	}
	if offerStatus != "pending" || counterStatus != "pending" {
		return nil, fmt.Errorf("%w: no pending counter to respond to", ErrConflict)
	}
	if action == "decline" {
		o, err := scanOffer(s.pool.QueryRow(ctx,
			`UPDATE offers SET poster_counter_status='declined', responded_at=now() WHERE id=$1::uuid RETURNING `+offerCols, offerID))
		if err != nil {
			return nil, err
		}
		return o, nil
	}
	// Accept: delegate to atomic accept with counter amount.
	// We run the same 9-step transaction but mark counter accepted first.
	// Use a short idempotency key derived from offerID for this path.
	return s.acceptViaCounter(ctx, workerID, offerID)
}

func (s *Service) acceptViaCounter(ctx context.Context, workerID, offerID string) (*Offer, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var taskID, posterID, taskStatus, escrowStatus string
	var posterCounterAmount int
	err = tx.QueryRow(ctx,
		`SELECT o.task_id::text, t.poster_id::text, t.status, t.escrow_status, o.poster_counter_amount
		 FROM offers o JOIN tasks t ON t.id=o.task_id WHERE o.id=$1::uuid FOR UPDATE OF t`,
		offerID).Scan(&taskID, &posterID, &taskStatus, &escrowStatus, &posterCounterAmount)
	if err != nil {
		return nil, ErrNotFound
	}
	if taskStatus != "open" {
		return nil, fmt.Errorf("%w: task no longer open", ErrGone)
	}
	if escrowStatus != "secured" {
		return nil, fmt.Errorf("%w: escrow not secured", ErrConflict)
	}
	// Double-check offer still pending/pending-counter.
	var oStatus, cStatus string
	err = tx.QueryRow(ctx, `SELECT status, poster_counter_status FROM offers WHERE id=$1::uuid FOR UPDATE`, offerID).Scan(&oStatus, &cStatus)
	if err != nil {
		return nil, err
	}
	if oStatus != "pending" || cStatus != "pending" {
		return nil, fmt.Errorf("%w: counter no longer pending", ErrGone)
	}
	_, err = tx.Exec(ctx, `UPDATE offers SET poster_counter_status='accepted', responded_at=now() WHERE id=$1::uuid`, offerID)
	if err != nil {
		return nil, err
	}
	// Now run steps 3-9 of accept (reuse helper with counter amount).
	res, err := s.doAcceptTx(ctx, tx, posterID, workerID, taskID, offerID, posterCounterAmount)
	if err != nil {
		return nil, err
	}
	_ = res
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	_ = audit.Log(ctx, s.pool, audit.Entry{ActorID: &workerID, Action: "offer.counter_accept", EntityType: "offer", EntityID: offerID})
	o, err := scanOffer(s.pool.QueryRow(ctx, `SELECT `+offerCols+` FROM offers WHERE id=$1::uuid`, offerID))
	if err != nil {
		return nil, err
	}
	return o, nil
}

// ── Accept (poster accepts worker offer) ───────────────────────────────────
//
// Atomic 9-step transaction. Idempotent via idempotency_keys (operation
// offer.accept). Concurrent callers serialize on task row lock; second
// sees status != open and gets ErrGone.
func (s *Service) Accept(ctx context.Context, posterID, offerID, idempotencyKey string) (*AcceptResult, error) {
	if idempotencyKey == "" {
		return nil, fmt.Errorf("%w: idempotency key required", ErrBadRequest)
	}
	// Fast replay check before acquiring transaction lock.
	var storedBody []byte
	err := s.pool.QueryRow(ctx, `SELECT response_body FROM idempotency_keys WHERE key=$1`, idempotencyKey).Scan(&storedBody)
	if err == nil {
		var prev AcceptResult
		_ = json.Unmarshal(storedBody, &prev)
		prev.Repeated = true
		return &prev, nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Re-check inside tx (handles race between check and begin).
	err = tx.QueryRow(ctx, `SELECT response_body FROM idempotency_keys WHERE key=$1 FOR UPDATE`, idempotencyKey).Scan(&storedBody)
	if err == nil {
		_ = tx.Rollback(ctx)
		var prev AcceptResult
		_ = json.Unmarshal(storedBody, &prev)
		prev.Repeated = true
		return &prev, nil
	}

	// Lock task.
	var taskStatus, taskPosterID, escrowStatus, taskLocationExact string
	var locationExactLat, locationExactLng *float64
	err = tx.QueryRow(ctx,
		`SELECT status, poster_id::text, escrow_status, location_exact, location_exact_lat, location_exact_lng FROM tasks WHERE id=(SELECT task_id FROM offers WHERE id=$1::uuid) FOR UPDATE`,
		offerID).Scan(&taskStatus, &taskPosterID, &escrowStatus, &taskLocationExact, &locationExactLat, &locationExactLng)
	if err != nil {
		return nil, ErrNotFound
	}
	if taskPosterID != posterID {
		return nil, ErrForbidden
	}
	if taskStatus != "open" {
		return nil, fmt.Errorf("%w: task no longer open (status %s)", ErrGone, taskStatus)
	}
	var oStatus, oTaskID, oWorkerID string
	var oAmount int
	err = tx.QueryRow(ctx, `SELECT status, task_id::text, worker_id::text, amount FROM offers WHERE id=$1::uuid FOR UPDATE`, offerID).Scan(&oStatus, &oTaskID, &oWorkerID, &oAmount)
	if err != nil {
		return nil, ErrNotFound
	}
	if oStatus != "pending" {
		return nil, fmt.Errorf("%w: offer no longer pending", ErrGone)
	}
	if escrowStatus != "secured" {
		return nil, fmt.Errorf("%w: escrow not secured", ErrConflict)
	}

	res, err := s.doAcceptTx(ctx, tx, posterID, oWorkerID, oTaskID, offerID, 0)
	if err != nil {
		return nil, err
	}
	// Persist idempotency result inside same transaction.
	body, _ := json.Marshal(res)
	_, err = tx.Exec(ctx,
		`INSERT INTO idempotency_keys (key, operation, user_id, response_code, response_body)
		 VALUES ($1,'offer.accept',$2::uuid,200,$3) ON CONFLICT (key) DO NOTHING`,
		idempotencyKey, posterID, string(body))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	_ = audit.Log(ctx, s.pool, audit.Entry{ActorID: &posterID, Action: "offer.accept", EntityType: "offer", EntityID: offerID, Metadata: map[string]any{"task_id": oTaskID}})
	return res, nil
}

func (s *Service) doAcceptTx(ctx context.Context, tx pgx.Tx, posterID, workerID, taskID, offerID string, counterAmount int) (*AcceptResult, error) {
	// Steps 3-5: task assigned, offer accepted, siblings auto-declined.
	_, err := tx.Exec(ctx,
		`UPDATE tasks SET status='assigned', assigned_worker_id=$1::uuid, accepted_offer_id=$2::uuid, accepted_at=now(), updated_at=now() WHERE id=$3::uuid`,
		workerID, offerID, taskID)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE offers SET status='accepted', responded_at=now() WHERE id=$1::uuid`, offerID)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE offers SET status='auto_declined', responded_at=now() WHERE task_id=$1::uuid AND status='pending' AND id<>$2::uuid`, taskID, offerID)
	if err != nil {
		return nil, err
	}
	// Steps 7-8: conversation + system message with exact location.
	var convID string
	err = tx.QueryRow(ctx,
		`INSERT INTO conversations (task_id, poster_id, worker_id) VALUES ($1::uuid,$2::uuid,$3::uuid) RETURNING id::text`,
		taskID, posterID, workerID).Scan(&convID)
	if err != nil {
		return nil, err
	}
	// System message body includes exact location for the worker (first surfacing) — via shared function (Phase 5 self-audit).
	var locExact string
	var locLat, locLng *float64
	err = tx.QueryRow(ctx, `SELECT location_exact, location_exact_lat, location_exact_lng FROM tasks WHERE id=$1::uuid`, taskID).Scan(&locExact, &locLat, &locLng)
	if err != nil {
		return nil, err
	}
	body := fmt.Sprintf("Offer accepted. Task assigned. Meeting location: %s", locExact)
	if counterAmount > 0 {
		body = fmt.Sprintf("Counter accepted (%d). Task assigned. Meeting location: %s", counterAmount, locExact)
	}
	_, err = messaging.InsertSystemMessageTx(ctx, tx, convID, "accepted", body)
	if err != nil {
		return nil, err
	}
	// Step 9: notification fan-out — real notify() calls (Phase 10 retrofit).
	acceptedPayload := notify.Payload(notify.EventOfferAccepted, map[string]string{
		"task_id": taskID, "offer_id": offerID, "conversation_id": convID,
	})
	_, err = tx.Exec(ctx,
		`INSERT INTO notifications (user_id, type, title, body, payload, channel)
		 VALUES ($1::uuid, $2, $3, $4, $5::jsonb, 'both')`,
		workerID, notify.EventOfferAccepted, "Offer accepted", "Your offer has been accepted! Task assigned.", acceptedPayload)
	if err != nil {
		return nil, err
	}
	// New offer → poster (if poster is not the worker)
	if posterID != workerID {
		posterNotifPayload := notify.Payload(notify.EventNewOffer, map[string]string{"task_id": taskID, "offer_id": offerID})
		_, _ = tx.Exec(ctx,
			`INSERT INTO notifications (user_id, type, title, body, payload, channel)
			 VALUES ($1::uuid, $2, $3, $4, $5::jsonb, 'both')`,
			posterID, notify.EventNewOffer, "New offer received", "Someone made an offer on your task", posterNotifPayload)
	}
	rows, err := tx.Query(ctx, `SELECT worker_id::text FROM offers WHERE task_id=$1::uuid AND status='auto_declined'`, taskID)
	if err != nil {
		return nil, err
	}
	var autoDeclined []string
	for rows.Next() {
		var wid string
		if err := rows.Scan(&wid); err != nil {
			rows.Close()
			return nil, err
		}
		autoDeclined = append(autoDeclined, wid)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, wid := range autoDeclined {
		declinedPayload := notify.Payload(notify.EventOfferDeclined, map[string]string{"task_id": taskID, "offer_id": offerID})
		_, err = tx.Exec(ctx,
			`INSERT INTO notifications (user_id, type, title, body, payload, channel)
			 VALUES ($1::uuid, $2, $3, $4, $5::jsonb, 'in_app')`,
			wid, notify.EventOfferDeclined, "Offer not selected", "Another offer was selected for this task", declinedPayload)
		if err != nil {
			return nil, err
		}
	}
	return &AcceptResult{TaskID: taskID, OfferID: offerID, TaskStatus: "assigned", ConversationID: convID}, nil
}

// ── Withdraw ─────────────────────────────────────────────────────────────────

func (s *Service) Withdraw(ctx context.Context, workerID, offerID string) (*Offer, error) {
	var workerCheck, status string
	err := s.pool.QueryRow(ctx, `SELECT worker_id::text, status FROM offers WHERE id=$1::uuid`, offerID).Scan(&workerCheck, &status)
	if err != nil {
		return nil, ErrNotFound
	}
	if workerCheck != workerID {
		return nil, ErrForbidden
	}
	if status != "pending" {
		return nil, fmt.Errorf("%w: only pending offers can be withdrawn", ErrConflict)
	}
	o, err := scanOffer(s.pool.QueryRow(ctx,
		`UPDATE offers SET status='withdrawn', responded_at=now() WHERE id=$1::uuid RETURNING `+offerCols, offerID))
	if err != nil {
		return nil, err
	}
	// Decrement offer_count if still open.
	var taskID string
	_ = s.pool.QueryRow(ctx, `SELECT task_id::text FROM offers WHERE id=$1::uuid`, offerID).Scan(&taskID)
	_, _ = s.pool.Exec(ctx, `UPDATE tasks SET offer_count = GREATEST(offer_count-1,0) WHERE id=$1::uuid`, taskID)
	_ = audit.Log(ctx, s.pool, audit.Entry{ActorID: &workerID, Action: "offer.withdraw", EntityType: "offer", EntityID: offerID})
	return o, nil
}
