// Package audit provides the single reusable helper for writing audit rows.
// Phase 0 §5: every privileged action (money, admin, verification,
// suspensions, escrow moves) calls Log. Built now so later phases retrofit nothing.
package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Entry struct {
	ActorID    *string
	Action     string
	EntityType string
	EntityID   string
	Metadata   map[string]any
}

// Log inserts one audit_log row. Action/entity fields are mandatory;
// metadata may be nil. Runs in the caller's transaction when one is open
// (pass the tx pool/conn through), otherwise a direct insert.
func Log(ctx context.Context, pool *pgxpool.Pool, e Entry) error {
	if e.Action == "" || e.EntityType == "" || e.EntityID == "" {
		return fmt.Errorf("audit: action, entity_type and entity_id are required")
	}
	meta, err := json.Marshal(e.Metadata)
	if err != nil {
		return fmt.Errorf("audit: marshal metadata: %w", err)
	}
	_, err = pool.Exec(ctx,
		`INSERT INTO audit_log (actor_id, action, entity_type, entity_id, metadata)
		 VALUES (NULLIF($1,'')::uuid, $2, $3, $4, $5)`,
		nullIfEmpty(e.ActorID), e.Action, e.EntityType, e.EntityID, string(meta))
	if err != nil {
		return fmt.Errorf("audit: insert: %w", err)
	}
	return nil
}

func nullIfEmpty(s *string) any {
	if s == nil || *s == "" {
		return nil
	}
	return *s
}
