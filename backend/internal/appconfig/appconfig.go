// Package appconfig reads placeholder-seeded rows from app_config.
// Phase 0 §10: values live in the DB from day one — never as hardcoded
// constants in application code. Get returns the raw JSON value.
package appconfig

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RequiredKeys are the rows every later phase needs (Testing Gate #11).
var RequiredKeys = []string{
	"fee_percent", "fee_payer",
	"min_budget", "max_budget",
	"auto_release_hours", "auto_release_warning_hours",
	"no_offer_nudge_hours", "chat_freeze_days", "review_publish_days",
	"cancellation_free_window_minutes", "cancellation_compensation_percent",
	"locale", "currency",
	"otp_ttl_seconds", "otp_max_attempts", "otp_resend_cooldown_seconds",
	"require_verification_to_post",
}

// Get fetches one key. Missing rows are an error — callers must never
// invent inline defaults.
func Get(ctx context.Context, pool *pgxpool.Pool, key string) (string, error) {
	var raw string
	err := pool.QueryRow(ctx, `SELECT value::text FROM app_config WHERE key = $1`, key).Scan(&raw)
	if err != nil {
		return "", fmt.Errorf("appconfig: missing key %q: %w", key, err)
	}
	return raw, nil
}

// All loads the full map (used by the seed-completeness test).
func All(ctx context.Context, pool *pgxpool.Pool) (map[string]string, error) {
	rows, err := pool.Query(ctx, `SELECT key, value::text FROM app_config`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}
