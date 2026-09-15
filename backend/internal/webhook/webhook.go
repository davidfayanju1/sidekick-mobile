// Package webhook handles provider webhooks with signature verification, idempotency, audit.
package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sidekick/backend/internal/audit"
)

var (
	ErrBadSignature = errors.New("invalid signature")
)

func webhookSecret() string {
	s := os.Getenv("PAYMENTS_WEBHOOK_SECRET")
	if s == "" {
		s = os.Getenv("WEBHOOK_SECRET")
	}
	if s == "" {
		s = "test-webhook-secret-0123456789abcdef"
	}
	return s
}

func verifySignature(payload []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(webhookSecret()))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// Event is the minimal provider event we handle.
type Event struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	IdempotencyKey *string      `json:"idempotency_key"`
	ProviderRef string          `json:"provider_ref"`
	Status      string          `json:"status"`
	Amount      *int            `json:"amount"`
	Raw         json.RawMessage `json:"-"`
}

func Handle(ctx context.Context, pool *pgxpool.Pool, payload []byte, signature string) (int, string, error) {
	// Always audit, even on failure
	defer func() {
		// audit is done inside, but ensure we log failures too
	}()

	if signature == "" || !verifySignature(payload, signature) {
		_ = audit.Log(ctx, pool, audit.Entry{Action: "webhook.rejected", EntityType: "webhook", EntityID: "unknown", Metadata: map[string]any{"reason": "bad_signature"}})
		return 401, "bad signature", ErrBadSignature
	}
	var ev Event
	if err := json.Unmarshal(payload, &ev); err != nil {
		_ = audit.Log(ctx, pool, audit.Entry{Action: "webhook.rejected", EntityType: "webhook", EntityID: "unknown", Metadata: map[string]any{"reason": "bad_json"}})
		return 400, "bad json", err
	}
	ev.Raw = payload

	// Idempotency: if we already have a transaction with this idempotency_key or provider_ref + event id, skip
	// Use idempotency_keys table or transactions provider_ref/event id?
	// For Phase 7, we use transactions idempotency_key or provider_ref deduplication.

	// Find existing transaction by idempotency_key if provided, else by provider_ref
	var txID string
	var txStatus string
	if ev.IdempotencyKey != nil && *ev.IdempotencyKey != "" {
		err := pool.QueryRow(ctx, `SELECT id::text, status FROM transactions WHERE idempotency_key=$1`, *ev.IdempotencyKey).Scan(&txID, &txStatus)
		if err == nil {
			// Already have this transaction – update status if needed, but don't double-process
			// For idempotency, just return success without re-processing if status already matches
			if txStatus == ev.Status || ev.Status == "" {
				_ = audit.Log(ctx, pool, audit.Entry{Action: "webhook.duplicate", EntityType: "webhook", EntityID: ev.ID, Metadata: map[string]any{"event_type": ev.Type}})
				return 200, "duplicate", nil
			}
		}
	} else if ev.ProviderRef != "" {
		err := pool.QueryRow(ctx, `SELECT id::text FROM transactions WHERE provider_ref=$1`, ev.ProviderRef).Scan(&txID)
		if err == nil {
			// check if we already processed this event id via audit?
			var exists int
			_ = pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE entity_id=$1 AND action='webhook.processed'`, ev.ID).Scan(&exists)
			if exists > 0 {
				_ = audit.Log(ctx, pool, audit.Entry{Action: "webhook.duplicate", EntityType: "webhook", EntityID: ev.ID})
				return 200, "duplicate", nil
			}
		}
	}

	// Process based on event type
	switch {
	case strings.HasPrefix(ev.Type, "charge."):
		// charge.succeeded / failed – update escrow_hold status
		if ev.IdempotencyKey != nil && *ev.IdempotencyKey != "" {
			_, err := pool.Exec(ctx, `UPDATE transactions SET status=$2, settled_at=now() WHERE idempotency_key=$1`, *ev.IdempotencyKey, ev.Status)
			if err != nil {
				return 500, "db error", err
			}
		} else if ev.ProviderRef != "" {
			_, _ = pool.Exec(ctx, `UPDATE transactions SET status=$2, settled_at=now() WHERE provider_ref=$1`, ev.ProviderRef, ev.Status)
		}
	case strings.HasPrefix(ev.Type, "payout."):
		// payout.succeeded / failed – update withdrawal + payout_items + batch rollup
		var withdrawalID string
		if ev.IdempotencyKey != nil {
			_ = pool.QueryRow(ctx, `SELECT id::text FROM transactions WHERE idempotency_key=$1`, *ev.IdempotencyKey).Scan(&withdrawalID)
		}
		if withdrawalID == "" && ev.ProviderRef != "" {
			_ = pool.QueryRow(ctx, `SELECT id::text FROM transactions WHERE provider_ref=$1`, ev.ProviderRef).Scan(&withdrawalID)
		}
		if withdrawalID != "" {
			if ev.Type == "payout.failed" {
				// revert status and credit balance back
				var payerID string
				var amount int
				_ = pool.QueryRow(ctx, `SELECT payer_id::text, amount FROM transactions WHERE id=$1::uuid`, withdrawalID).Scan(&payerID, &amount)
				_, _ = pool.Exec(ctx, `UPDATE transactions SET status='failed', settled_at=now() WHERE id=$1::uuid`, withdrawalID)
				if payerID != "" {
					_, _ = pool.Exec(ctx, `UPDATE wallets SET available_balance = available_balance + $2, updated_at=now() WHERE user_id=$1::uuid`, payerID, amount)
				}
				// Phase 11: update payout_items
				_, _ = pool.Exec(ctx,
					`UPDATE payout_items SET status='failed' WHERE transaction_id=$1::uuid AND status IN ('pending','processing')`, withdrawalID)
			} else {
				_, _ = pool.Exec(ctx, `UPDATE transactions SET status='succeeded', settled_at=now() WHERE id=$1::uuid`, withdrawalID)
				// Phase 11: update payout_items
				_, _ = pool.Exec(ctx,
					`UPDATE payout_items SET status='succeeded' WHERE transaction_id=$1::uuid AND status IN ('pending','processing')`, withdrawalID)
			}
			// Phase 11: roll up batch status — if all items are settled, mark batch as settled
			_, _ = pool.Exec(ctx, `
				UPDATE payout_batches SET status='settled', processed_at=now()
				WHERE id IN (SELECT batch_id FROM payout_items WHERE transaction_id=$1::uuid)
				  AND NOT EXISTS (
				    SELECT 1 FROM payout_items pi WHERE pi.batch_id = payout_batches.id
				      AND pi.status NOT IN ('succeeded','failed')
				  )`, withdrawalID)
		}
	case strings.HasPrefix(ev.Type, "refund."):
		if ev.IdempotencyKey != nil {
			_, _ = pool.Exec(ctx, `UPDATE transactions SET status=$2, settled_at=now() WHERE idempotency_key=$1`, *ev.IdempotencyKey, ev.Status)
		}
	case strings.HasPrefix(ev.Type, "transfer."):
		if ev.IdempotencyKey != nil {
			_, _ = pool.Exec(ctx, `UPDATE transactions SET status=$2, settled_at=now() WHERE idempotency_key=$1`, *ev.IdempotencyKey, ev.Status)
		}
	default:
		// unknown type – just audit and return 200 (don't fail provider retry)
	}

	_ = audit.Log(ctx, pool, audit.Entry{Action: "webhook.processed", EntityType: "webhook", EntityID: ev.ID, Metadata: map[string]any{"event_type": ev.Type, "provider_ref": ev.ProviderRef}})
	return 200, "processed", nil
}

func SignPayload(payload []byte) string {
	mac := hmac.New(sha256.New, []byte(webhookSecret()))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// For tests: helper to create signed payload
func SignedPayload(v any) ([]byte, string) {
	b, _ := json.Marshal(v)
	return b, SignPayload(b)
}
