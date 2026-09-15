// Package execution implements Phase 6: mark complete, confirm, dispute,
// release_escrow (shared), auto-release cron/warning, cancellations.
package execution

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sidekick/backend/internal/audit"
	"github.com/sidekick/backend/internal/ledger"
	"github.com/sidekick/backend/internal/messaging"
	"github.com/sidekick/backend/internal/notify"
	"github.com/sidekick/backend/internal/payments"
)

var (
	ErrBadRequest = errors.New("bad request")
	ErrNotFound   = errors.New("not found")
	ErrForbidden  = errors.New("forbidden")
	ErrConflict   = errors.New("conflict")
	ErrGone       = errors.New("gone")
)

// Service holds dependencies.
type Service struct {
	pool     *pgxpool.Pool
	payments payments.Adapter
}

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool, payments: payments.NewMock()} }
func NewWithPayments(pool *pgxpool.Pool, p payments.Adapter) *Service {
	if p == nil {
		p = payments.NewMock()
	}
	return &Service{pool: pool, payments: p}
}

func cfgInt(ctx context.Context, pool *pgxpool.Pool, key string, fallback int) int {
	var raw string
	if err := pool.QueryRow(ctx, `SELECT value::text FROM app_config WHERE key=$1`, key).Scan(&raw); err != nil {
		return fallback
	}
	raw = strings.Trim(raw, `"`)
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}

// ensureWallet creates wallet if missing.
func ensureWallet(ctx context.Context, tx pgx.Tx, userID string) error {
	_, err := tx.Exec(ctx, `INSERT INTO wallets (user_id, available_balance, pending_balance) VALUES ($1::uuid,0,0) ON CONFLICT (user_id) DO NOTHING`, userID)
	return err
}
func ensureWalletPool(ctx context.Context, pool *pgxpool.Pool, userID string) error {
	_, err := pool.Exec(ctx, `INSERT INTO wallets (user_id, available_balance, pending_balance) VALUES ($1::uuid,0,0) ON CONFLICT (user_id) DO NOTHING`, userID)
	return err
}

// releaseEscrowTx is the shared core: insert release transaction, credit wallet, set escrow released.
// Must be called inside a transaction that has already locked the task row.
// It enforces exactly-once via partial unique index (type=release,status=succeeded).
func releaseEscrowTx(ctx context.Context, tx pgx.Tx, taskID, workerID string, amount, fee int) (string, error) {
	if err := ensureWallet(ctx, tx, workerID); err != nil {
		return "", err
	}
	// amount is total_charge - fee? caller decides. Here we just use amount as given (worker payout).
	var txID string
	err := tx.QueryRow(ctx,
		`INSERT INTO transactions (task_id, payer_id, payee_id, amount, platform_fee, type, status, provider_ref)
		 VALUES ($1::uuid, NULL, $2::uuid, $3, $4, 'release', 'succeeded', $5)
		 RETURNING id::text`,
		taskID, workerID, amount, fee, "release-"+taskID).Scan(&txID)
	if err != nil {
		if strings.Contains(err.Error(), "uq_tx_release_once") || strings.Contains(err.Error(), "duplicate") {
			return "", fmt.Errorf("%w: release already exists", ErrConflict)
		}
		return "", err
	}
	_, err = tx.Exec(ctx, `UPDATE wallets SET available_balance = available_balance + $2, updated_at=now() WHERE user_id=$1::uuid`, workerID, amount)
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, `UPDATE tasks SET escrow_status='released', updated_at=now() WHERE id=$1::uuid`, taskID)
	if err != nil {
		return "", err
	}
	return txID, nil
}

// ReleaseEscrow is the public shared function used by confirm and cron. It wraps releaseEscrowTx with task lookup and wallet handling.
// Documented choice: credits available_balance directly (no pending/clearing delay).
func (s *Service) ReleaseEscrow(ctx context.Context, taskID string) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var workerID string
	var total, fee int
	var status, escrowStatus string
	err = tx.QueryRow(ctx, `SELECT assigned_worker_id::text, total_charge, platform_fee, status, escrow_status FROM tasks WHERE id=$1::uuid FOR UPDATE`, taskID).Scan(&workerID, &total, &fee, &status, &escrowStatus)
	if err != nil {
		return "", ErrNotFound
	}
	if workerID == "" {
		return "", fmt.Errorf("%w: no assigned worker", ErrConflict)
	}
	if escrowStatus != "secured" && escrowStatus != "frozen" {
		// for auto-release we expect secured; for disputed we block earlier.
		// Allow only if secured.
		return "", fmt.Errorf("%w: escrow not secured", ErrConflict)
	}
	// check dispute exists
	var disputeCnt int
	_ = tx.QueryRow(ctx, `SELECT count(*) FROM disputes WHERE task_id=$1::uuid AND status IN ('open','under_review')`, taskID).Scan(&disputeCnt)
	if disputeCnt > 0 {
		return "", fmt.Errorf("%w: disputed", ErrConflict)
	}
	amount := total - fee
	if amount <= 0 {
		amount = total
	}
	txID, err := releaseEscrowTx(ctx, tx, taskID, workerID, amount, fee)
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return txID, nil
}

// MarkComplete: assigned worker -> completed_pending_confirmation
func (s *Service) MarkComplete(ctx context.Context, workerID, taskID string, photoURLs []string) error {
	if len(photoURLs) > 5 {
		return fmt.Errorf("%w: completion photos max 5", ErrBadRequest)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status, assignedID string
	var posterID string
	err = tx.QueryRow(ctx, `SELECT status, assigned_worker_id::text, poster_id::text FROM tasks WHERE id=$1::uuid FOR UPDATE`, taskID).Scan(&status, &assignedID, &posterID)
	if err != nil {
		return ErrNotFound
	}
	if status != "assigned" {
		return fmt.Errorf("%w: task not assigned", ErrConflict)
	}
	if assignedID != workerID {
		return ErrForbidden
	}
	// check suspended?
	var susp *time.Time
	_ = tx.QueryRow(ctx, `SELECT suspended_at FROM users WHERE id=$1::uuid`, workerID).Scan(&susp)
	if susp != nil {
		return ErrForbidden
	}
	// enforce 0-5 cap for completion photos (existing + new)
	if len(photoURLs) > 0 {
		var existing int
		_ = tx.QueryRow(ctx, `SELECT count(*) FROM task_photos WHERE task_id=$1::uuid AND type='completion'`, taskID).Scan(&existing)
		if existing+len(photoURLs) > 5 {
			return fmt.Errorf("%w: completion photos max 5", ErrBadRequest)
		}
	}
	hours := cfgInt(ctx, s.pool, "auto_release_hours", 72)
	_, err = tx.Exec(ctx, `UPDATE tasks SET status='completed_pending_confirmation', marked_complete_at=now(), auto_release_at=now() + ($2::text || ' hours')::interval, updated_at=now() WHERE id=$1::uuid`, taskID, strconv.Itoa(hours))
	if err != nil {
		return err
	}
	// handle completion photos if any
	for _, url := range photoURLs {
		_, err = tx.Exec(ctx, `INSERT INTO task_photos (task_id, url, type, uploaded_by) VALUES ($1::uuid,$2,'completion',$3::uuid)`, taskID, url, workerID)
		if err != nil {
			return err
		}
	}
	// need conversation id for system message
	var convID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM conversations WHERE task_id=$1::uuid`, taskID).Scan(&convID)
	if err == nil {
		body := "Task marked complete by worker"
		_, _ = messaging.InsertSystemMessageTx(ctx, tx, convID, "marked_complete", body)
		// Phase 10 retrofit: real notification to poster
		_, _ = notify.InsertNotifTx(ctx, tx, posterID, notify.EventTaskMarkedComplete, "both",
			notify.Payload(notify.EventTaskMarkedComplete, map[string]string{"task_id": taskID}))
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	_ = audit.Log(ctx, s.pool, audit.Entry{ActorID: &workerID, Action: "task.mark_complete", EntityType: "task", EntityID: taskID})
	return nil
}

// Confirm: poster -> completed + release
func (s *Service) Confirm(ctx context.Context, posterID, taskID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status, assignedID, taskPosterID string
	var total, fee int
	err = tx.QueryRow(ctx, `SELECT status, assigned_worker_id::text, poster_id::text, total_charge, platform_fee FROM tasks WHERE id=$1::uuid FOR UPDATE`, taskID).Scan(&status, &assignedID, &taskPosterID, &total, &fee)
	if err != nil {
		return ErrNotFound
	}
	if taskPosterID != posterID {
		return ErrForbidden
	}
	if status != "completed_pending_confirmation" {
		return fmt.Errorf("%w: task not pending confirmation", ErrConflict)
	}
	// check dispute
	var disputeCnt int
	_ = tx.QueryRow(ctx, `SELECT count(*) FROM disputes WHERE task_id=$1::uuid AND status IN ('open','under_review')`, taskID).Scan(&disputeCnt)
	if disputeCnt > 0 {
		return fmt.Errorf("%w: disputed", ErrConflict)
	}
	amount := total - fee
	if amount <= 0 {
		amount = total
	}
	// release escrow via shared logic (inline to keep same tx)
	var workerID = assignedID
	if err := ensureWallet(ctx, tx, workerID); err != nil {
		return err
	}
	var txID string
	err = tx.QueryRow(ctx,
		`INSERT INTO transactions (task_id, payer_id, payee_id, amount, platform_fee, type, status, provider_ref)
		 VALUES ($1::uuid, NULL, $2::uuid, $3, $4, 'release', 'succeeded', $5) RETURNING id::text`,
		taskID, workerID, amount, fee, "release-"+taskID).Scan(&txID)
	if err != nil {
		if strings.Contains(err.Error(), "uq_tx_release_once") {
			return fmt.Errorf("%w: already released", ErrConflict)
		}
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE wallets SET available_balance = available_balance + $2, updated_at=now() WHERE user_id=$1::uuid`, workerID, amount)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE tasks SET status='completed', completed_at=now(), escrow_status='released', updated_at=now() WHERE id=$1::uuid`, taskID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE users SET tasks_completed = tasks_completed + 1, updated_at=now() WHERE id = $1::uuid OR id = $2::uuid`, posterID, workerID)
	if err != nil {
		return err
	}
	var convID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM conversations WHERE task_id=$1::uuid`, taskID).Scan(&convID)
	if err == nil {
		_, _ = messaging.InsertSystemMessageTx(ctx, tx, convID, "payment_released", "Payment released to worker")
		// Phase 10 retrofit: real notifications
		_, _ = notify.InsertNotifTx(ctx, tx, workerID, notify.EventPaymentReleased, "both",
			notify.Payload(notify.EventPaymentReleased, map[string]string{"task_id": taskID}))
		_, _ = notify.InsertNotifTx(ctx, tx, posterID, notify.EventCompletionConfirmed, "both",
			notify.Payload(notify.EventCompletionConfirmed, map[string]string{"task_id": taskID}))
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	_ = audit.Log(ctx, s.pool, audit.Entry{ActorID: &posterID, Action: "task.confirm", EntityType: "task", EntityID: taskID})
	return nil
}

// RaiseDispute: either party on assigned/completed_pending
func (s *Service) RaiseDispute(ctx context.Context, userID, taskID, reason, description string) (string, error) {
	if strings.TrimSpace(reason) == "" || strings.TrimSpace(description) == "" {
		return "", fmt.Errorf("%w: reason and description required", ErrBadRequest)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status, posterID, workerID string
	err = tx.QueryRow(ctx, `SELECT status, poster_id::text, assigned_worker_id::text FROM tasks WHERE id=$1::uuid FOR UPDATE`, taskID).Scan(&status, &posterID, &workerID)
	if err != nil {
		return "", ErrNotFound
	}
	if userID != posterID && userID != workerID {
		return "", ErrForbidden
	}
	if status != "assigned" && status != "completed_pending_confirmation" {
		return "", fmt.Errorf("%w: cannot dispute in status %s", ErrConflict, status)
	}
	var disputeID string
	err = tx.QueryRow(ctx,
		`INSERT INTO disputes (task_id, raised_by, reason, description, status) VALUES ($1::uuid,$2::uuid,$3,$4,'open') RETURNING id::text`,
		taskID, userID, reason, description).Scan(&disputeID)
	if err != nil {
		if strings.Contains(err.Error(), "uq_dispute_open") {
			return "", fmt.Errorf("%w: dispute already open", ErrConflict)
		}
		return "", err
	}
	_, err = tx.Exec(ctx, `UPDATE tasks SET status='disputed', escrow_status='frozen', updated_at=now() WHERE id=$1::uuid`, taskID)
	if err != nil {
		return "", err
	}
	var convID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM conversations WHERE task_id=$1::uuid`, taskID).Scan(&convID)
	if err == nil {
		_, _ = messaging.InsertSystemMessageTx(ctx, tx, convID, "dispute_raised", "Dispute raised: "+reason)
		other := posterID
		if userID == posterID {
			other = workerID
		}
		if other != "" {
			// Phase 10 retrofit: real notification to other party
			_, _ = notify.InsertNotifTx(ctx, tx, other, notify.EventDisputeRaised, "both",
				notify.Payload(notify.EventDisputeRaised, map[string]string{"task_id": taskID, "dispute_id": disputeID}))
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	_ = audit.Log(ctx, s.pool, audit.Entry{ActorID: &userID, Action: "dispute.create", EntityType: "dispute", EntityID: disputeID})
	return disputeID, nil
}

// Cancel handles branching by role and status.
func (s *Service) Cancel(ctx context.Context, userID, taskID, reason string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status, posterID string
	var workerID *string
	var escrowStatus string
	var agreedAt *time.Time
	var total, fee int
	err = tx.QueryRow(ctx, `SELECT status, poster_id::text, assigned_worker_id::text, escrow_status, agreed_start_at, total_charge, platform_fee FROM tasks WHERE id=$1::uuid FOR UPDATE`, taskID).Scan(&status, &posterID, &workerID, &escrowStatus, &agreedAt, &total, &fee)
	if err != nil {
		return ErrNotFound
	}
	if status == "completed" || status == "disputed" || strings.HasPrefix(status, "resolved") || status == "cancelled_by_poster" || status == "cancelled_by_worker" || status == "expired" {
		return fmt.Errorf("%w: cannot cancel in status %s", ErrConflict, status)
	}
	isPoster := userID == posterID
	isWorker := workerID != nil && userID == *workerID
	if !isPoster && !isWorker {
		return ErrForbidden
	}
	// Need conversation for system messages/notifications
	var convID string
	_ = tx.QueryRow(ctx, `SELECT id::text FROM conversations WHERE task_id=$1::uuid`, taskID).Scan(&convID)

	switch {
	case status == "open":
		// only poster can cancel open (already handled via tasks.Cancel but we also support here for completeness)
		if !isPoster {
			return ErrForbidden
		}
		// reuse tasks cancel logic for open: full refund if secured
		// For Phase 6 we handle assigned cases below, so open case here is simple:
		if escrowStatus == "secured" {
			var cID *string
			if convID != "" {
				cID = &convID
			}
			if _, err := ledger.RefundEscrowTx(ctx, tx, s.payments, taskID, posterID, total, fee, "cancel open task", "refund-"+taskID+"-"+userID, cID); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `UPDATE tasks SET status='cancelled_by_poster', cancelled_by=$1::uuid, cancel_reason=$3, escrow_status=CASE WHEN escrow_status='secured' THEN 'refunded' ELSE escrow_status END, updated_at=now() WHERE id=$2::uuid`, userID, taskID, reason)
		if err != nil {
			return err
		}
		if convID != "" {
			_, _ = messaging.InsertSystemMessageTx(ctx, tx, convID, "cancelled", "Task cancelled by poster")
			// Phase 10 retrofit: notify worker
			if workerID != nil {
				_, _ = notify.InsertNotifTx(ctx, tx, *workerID, notify.EventTaskCancelled, "both",
					notify.Payload(notify.EventTaskCancelled, map[string]string{"task_id": taskID}))
			}
		}
	case status == "assigned":
		if isPoster {
			// poster cancellation: check grace
			graceMin := cfgInt(ctx, s.pool, "cancellation_free_window_minutes", 60)
			compPct := cfgInt(ctx, s.pool, "cancellation_compensation_percent", 20)
			var withinFree bool
			if agreedAt == nil {
				withinFree = true
			} else {
				withinFree = time.Now().Before(agreedAt.Add(time.Duration(graceMin) * time.Minute))
			}
			if withinFree {
				// full refund via consolidated ledger
				var cID *string
				if convID != "" {
					cID = &convID
				}
				if _, err := ledger.RefundEscrowTx(ctx, tx, s.payments, taskID, posterID, total, fee, "cancel within grace", "refund-"+taskID+"-poster-free", cID); err != nil {
					return err
				}
				_, err = tx.Exec(ctx, `UPDATE tasks SET status='cancelled_by_poster', cancelled_by=$1::uuid, cancel_reason=$3, escrow_status='refunded', updated_at=now() WHERE id=$2::uuid`, userID, taskID, reason)
				if err != nil {
					return err
				}
			} else {
				// compensation + partial refund
				compAmount := total * compPct / 100
				refundAmount := total - compAmount
				if compAmount > 0 && workerID != nil {
					if err := ensureWallet(ctx, tx, *workerID); err != nil {
						return err
					}
					compKey := "comp-" + taskID
					_, err = tx.Exec(ctx,
						`INSERT INTO transactions (task_id, payer_id, payee_id, amount, platform_fee, type, status, provider_ref, idempotency_key)
						 VALUES ($1::uuid,$2::uuid,$3::uuid,$4,0,'cancellation_compensation','succeeded',$5,$6) ON CONFLICT (idempotency_key) DO NOTHING`,
						taskID, posterID, *workerID, compAmount, "comp-"+taskID, compKey)
					if err != nil {
						return err
					}
					_, err = tx.Exec(ctx, `UPDATE wallets SET available_balance = available_balance + $2, updated_at=now() WHERE user_id=$1::uuid`, *workerID, compAmount)
					if err != nil {
						return err
					}
					if refundAmount > 0 {
						var cID *string
						if convID != "" {
							cID = &convID
						}
						if _, err := ledger.RefundEscrowTx(ctx, tx, s.payments, taskID, posterID, refundAmount, fee, "cancel partial refund", "refund-"+taskID+"-partial", cID); err != nil {
							return err
						}
					}
					_, err = tx.Exec(ctx, `UPDATE tasks SET status='cancelled_by_poster', cancelled_by=$1::uuid, cancel_reason=$3, escrow_status='refunded', updated_at=now() WHERE id=$2::uuid`, userID, taskID, reason)
					if err != nil {
						return err
					}
				}
			}
		if convID != "" && workerID != nil {
			_, _ = messaging.InsertSystemMessageTx(ctx, tx, convID, "cancelled", "Task cancelled by poster")
			// Phase 10 retrofit: notify worker
			_, _ = notify.InsertNotifTx(ctx, tx, *workerID, notify.EventTaskCancelled, "both",
				notify.Payload(notify.EventTaskCancelled, map[string]string{"task_id": taskID}))
		}
		} else if isWorker {
			// worker cancellation: reopen, clear assignment, decrement reliability, keep escrow secured
			_, err = tx.Exec(ctx, `UPDATE tasks SET status='open', assigned_worker_id=NULL, accepted_offer_id=NULL, accepted_at=NULL, agreed_start_at=NULL, escrow_status='secured', updated_at=now() WHERE id=$1::uuid`, taskID)
			if err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `UPDATE users SET reliability_score = GREATEST(reliability_score - 5, 0), updated_at=now() WHERE id=$1::uuid`, userID)
			if err != nil {
				return err
			}
		if convID != "" {
			_, _ = messaging.InsertSystemMessageTx(ctx, tx, convID, "cancelled", "Task cancelled by worker, reopened")
			// Phase 10 retrofit: notify poster
			_, _ = notify.InsertNotifTx(ctx, tx, posterID, notify.EventTaskCancelled, "both",
				notify.Payload(notify.EventTaskCancelled, map[string]string{"task_id": taskID}))
		}
		}
	case status == "completed_pending_confirmation":
		// only disputes or confirm, not cancel? But spec says cancellation per §5 only for assigned; for completed_pending we don't allow cancel via this endpoint.
		return fmt.Errorf("%w: cannot cancel in status %s", ErrConflict, status)
	default:
		return fmt.Errorf("%w: cannot cancel in status %s", ErrConflict, status)
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	_ = audit.Log(ctx, s.pool, audit.Entry{ActorID: &userID, Action: "task.cancel", EntityType: "task", EntityID: taskID})
	return nil
}

// AutoReleaseWorker runs one tick: selects due tasks, releases each via shared logic, marks completed.
func (s *Service) AutoReleaseWorker(ctx context.Context) (int, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id::text, assigned_worker_id::text, total_charge, platform_fee FROM tasks
		 WHERE status='completed_pending_confirmation' AND auto_release_at <= now()
		   AND NOT EXISTS (SELECT 1 FROM disputes WHERE disputes.task_id=tasks.id AND disputes.status IN ('open','under_review'))`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id, workerID string
		var total, fee int
		if err := rows.Scan(&id, &workerID, &total, &fee); err != nil {
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	count := 0
	for _, taskID := range ids {
		// each task in its own transaction with exactly-once via unique index
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			continue
		}
		var status, workerID string
		var total, fee int
		err = tx.QueryRow(ctx, `SELECT status, assigned_worker_id::text, total_charge, platform_fee FROM tasks WHERE id=$1::uuid FOR UPDATE`, taskID).Scan(&status, &workerID, &total, &fee)
		if err != nil {
			_ = tx.Rollback(ctx)
			continue
		}
		if status != "completed_pending_confirmation" {
			_ = tx.Rollback(ctx)
			continue
		}
		var disputeCnt int
		_ = tx.QueryRow(ctx, `SELECT count(*) FROM disputes WHERE task_id=$1::uuid AND status IN ('open','under_review')`, taskID).Scan(&disputeCnt)
		if disputeCnt > 0 {
			_ = tx.Rollback(ctx)
			continue
		}
		amount := total - fee
		if amount <= 0 {
			amount = total
		}
		if err := ensureWallet(ctx, tx, workerID); err != nil {
			_ = tx.Rollback(ctx)
			continue
		}
		var txID string
		err = tx.QueryRow(ctx,
			`INSERT INTO transactions (task_id, payer_id, payee_id, amount, platform_fee, type, status, provider_ref)
			 VALUES ($1::uuid, NULL, $2::uuid, $3, $4, 'release', 'succeeded', $5) RETURNING id::text`,
			taskID, workerID, amount, fee, "release-"+taskID).Scan(&txID)
		if err != nil {
			if strings.Contains(err.Error(), "uq_tx_release_once") {
				// already released, just mark completed
				_, _ = tx.Exec(ctx, `UPDATE tasks SET status='completed', completed_at=now(), escrow_status='released', updated_at=now() WHERE id=$1::uuid`, taskID)
				_ = tx.Commit(ctx)
				count++
				continue
			}
			_ = tx.Rollback(ctx)
			continue
		}
		_, _ = tx.Exec(ctx, `UPDATE wallets SET available_balance = available_balance + $2, updated_at=now() WHERE user_id=$1::uuid`, workerID, amount)
		_, _ = tx.Exec(ctx, `UPDATE tasks SET status='completed', completed_at=now(), escrow_status='released', updated_at=now() WHERE id=$1::uuid`, taskID)
		_, _ = tx.Exec(ctx, `UPDATE users SET tasks_completed = tasks_completed + 1, updated_at=now() WHERE id IN (SELECT poster_id FROM tasks WHERE id=$1::uuid UNION SELECT $2::uuid)`, taskID, workerID)
		var convID string
		if err := tx.QueryRow(ctx, `SELECT id::text FROM conversations WHERE task_id=$1::uuid`, taskID).Scan(&convID); err == nil {
			_, _ = messaging.InsertSystemMessageTx(ctx, tx, convID, "payment_released", "Payment auto-released to worker")
			// Phase 10 retrofit: real notifications
			var posterID string
			_ = tx.QueryRow(ctx, `SELECT poster_id::text FROM tasks WHERE id=$1::uuid`, taskID).Scan(&posterID)
			_, _ = notify.InsertNotifTx(ctx, tx, workerID, notify.EventPaymentReleased, "both",
				notify.Payload(notify.EventPaymentReleased, map[string]string{"task_id": taskID}))
			_, _ = notify.InsertNotifTx(ctx, tx, posterID, notify.EventCompletionConfirmed, "both",
				notify.Payload(notify.EventCompletionConfirmed, map[string]string{"task_id": taskID}))
		}
		_ = tx.Commit(ctx)
		count++
	}
	return count, nil
}

// AutoReleaseWarning returns tasks due for 24h warning (for tests).
func (s *Service) AutoReleaseWarning(ctx context.Context) ([]string, error) {
	warnHours := cfgInt(ctx, s.pool, "auto_release_warning_hours", 24)
	rows, err := s.pool.Query(ctx,
		`SELECT id::text FROM tasks
		 WHERE status='completed_pending_confirmation'
		   AND auto_release_at > now()
		   AND auto_release_at <= now() + ($1::text || ' hours')::interval
		   AND NOT EXISTS (SELECT 1 FROM disputes WHERE disputes.task_id=tasks.id AND disputes.status IN ('open','under_review'))`, strconv.Itoa(warnHours))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
		// Phase 10 retrofit: idempotent — only insert if no recent warning in last 20h
		var posterID string
		_ = s.pool.QueryRow(ctx, `SELECT poster_id::text FROM tasks WHERE id=$1::uuid`, id).Scan(&posterID)
		var exists int
		_ = s.pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE user_id=$1::uuid AND type='auto_release_warning' AND payload->>'task_id'=$2 AND created_at > now() - interval '20 hours'`, posterID, id).Scan(&exists)
		if exists == 0 {
			_, _ = s.pool.Exec(ctx,
				`INSERT INTO notifications (user_id, type, title, body, payload, channel)
				 VALUES ($1::uuid, $2, $3, $4, $5::jsonb, 'push')`,
				posterID, notify.EventAutoReleaseWarning, "Auto-release in 24h",
				"Please confirm completion within 24 hours",
				notify.Payload(notify.EventAutoReleaseWarning, map[string]string{"task_id": id}))
		}
	}
	return ids, rows.Err()
}

func (s *Service) ensureWalletTx(ctx context.Context, tx pgx.Tx, userID string) error {
	_, err := tx.Exec(ctx, `INSERT INTO wallets (user_id) VALUES ($1::uuid) ON CONFLICT (user_id) DO NOTHING`, userID)
	return err
}

// Helper for tests: set agreed_start_at directly
func (s *Service) SetAgreedStart(ctx context.Context, taskID string, t time.Time) error {
	_, err := s.pool.Exec(ctx, `UPDATE tasks SET agreed_start_at=$2 WHERE id=$1::uuid`, taskID, t)
	return err
}
