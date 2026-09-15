// Package ledger consolidates all money movements to a single implementation per type.
// Phase 7's invariant: exactly one refund_escrow, one release, one withdrawal, etc.
// No other package may directly INSERT into transactions for those types.
package ledger

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/sidekick/backend/internal/messaging"
	"github.com/sidekick/backend/internal/payments"
)

// RefundEscrowTx is the single refund implementation for the entire codebase.
// It must be called inside a transaction that already holds the task row lock.
// It inserts a refund transaction (idempotent via idempotency_key), sets escrow to refunded,
// inserts a system message, and calls the provider's Refund.
// amount is the refund amount (total_charge for full, or remainder for partial).
// fee is platform_fee snapshot.
// reason is for system message / audit.
// provider may be nil in tests (mock); if nil, it just inserts DB row.
func RefundEscrowTx(ctx context.Context, tx pgx.Tx, adapter payments.Adapter, taskID, posterID string, amount, fee int, reason, idempotencyKey string, convID *string) (string, error) {
	if amount <= 0 {
		return "", fmt.Errorf("refund amount must be positive")
	}
	// Call provider first (if adapter provided) – but we still need idempotency.
	var providerRef string
	if adapter != nil {
		res, err := adapter.Refund(ctx, payments.Request{
			IdempotencyKey: idempotencyKey,
			TaskID:         taskID,
			Amount:         int64(amount),
			PlatformFee:    int64(fee),
		})
		if err != nil {
			return "", err
		}
		providerRef = res.ProviderRef
	} else {
		providerRef = "refund-" + taskID + "-" + idempotencyKey
	}
	var txID string
	err := tx.QueryRow(ctx,
		`INSERT INTO transactions (task_id, payer_id, payee_id, amount, platform_fee, type, status, provider_ref, idempotency_key)
		 VALUES ($1::uuid, NULL, $2::uuid, $3, $4, 'refund', 'succeeded', $5, $6)
		 ON CONFLICT (idempotency_key) DO UPDATE SET amount=EXCLUDED.amount RETURNING id::text`,
		taskID, posterID, amount, fee, providerRef, idempotencyKey).Scan(&txID)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			// fetch existing
			_ = tx.QueryRow(ctx, `SELECT id::text FROM transactions WHERE idempotency_key=$1`, idempotencyKey).Scan(&txID)
			return txID, nil
		}
		return "", err
	}
	// Only set escrow to refunded if currently secured/frozen – but caller decides.
	// We do not overwrite if already refunded.
	_, _ = tx.Exec(ctx, `UPDATE tasks SET escrow_status='refunded', updated_at=now() WHERE id=$1::uuid AND escrow_status IN ('secured','frozen')`, taskID)
	if convID != nil && *convID != "" {
		_, _ = messaging.InsertSystemMessageTx(ctx, tx, *convID, "refunded", "Refund issued: "+reason)
	}
	return txID, nil
}

// ReleaseTx is the shared release (called by execution Confirm, cron, and dispute resolution).
// amount is the worker payout (total_charge - fee for full release, or a partial amount for split).
// fee is the platform fee snapshot. Caller must have locked the task row.
func ReleaseTx(ctx context.Context, tx pgx.Tx, taskID, workerID string, amount, fee int) (string, error) {
	if amount <= 0 {
		return "", fmt.Errorf("release amount must be positive")
	}
	// ensure wallet exists
	_, _ = tx.Exec(ctx, `INSERT INTO wallets (user_id, available_balance, pending_balance) VALUES ($1::uuid,0,0) ON CONFLICT (user_id) DO NOTHING`, workerID)
	var txID string
	err := tx.QueryRow(ctx,
		`INSERT INTO transactions (task_id, payer_id, payee_id, amount, platform_fee, type, status, provider_ref)
		 VALUES ($1::uuid, NULL, $2::uuid, $3, $4, 'release', 'succeeded', $5) RETURNING id::text`,
		taskID, workerID, amount, fee, "release-"+taskID).Scan(&txID)
	if err != nil {
		if strings.Contains(err.Error(), "uq_tx_release_once") {
			return "", fmt.Errorf("release already exists")
		}
		return "", err
	}
	_, err = tx.Exec(ctx, `UPDATE wallets SET available_balance = available_balance + $2, updated_at=now() WHERE user_id=$1::uuid`, workerID, amount)
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, `UPDATE tasks SET escrow_status='released', updated_at=now() WHERE id=$1::uuid`, taskID)
	return txID, err
}
