// Package idempotency implements the generic check-and-store mechanism
// required by Phase 0 §6. One implementation reused by task funding
// (Phase 2), offer acceptance (Phase 4) and all payment ops (Phase 7).
//
// Contract: the handler computes nothing before calling Do. Do runs fn
// at most once per key: on a repeat key it returns the stored response
// without re-executing fn.
package idempotency

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Result struct {
	// Repeated is true when the stored response was replayed.
	Repeated     bool
	ResponseCode int
	ResponseBody json.RawMessage
}

// Do executes fn once per (key, operation). fn must return the HTTP-style
// status code and a JSON-marshalable body.
func Do(ctx context.Context, pool *pgxpool.Pool, key, operation, userID string, fn func() (int, any, error)) (Result, error) {
	if key == "" || operation == "" {
		return Result{}, fmt.Errorf("idempotency: key and operation are required")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("idempotency: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var code int
	var bodyJSON []byte
	err = tx.QueryRow(ctx,
		`SELECT response_code, response_body FROM idempotency_keys WHERE key = $1`,
		key).Scan(&code, &bodyJSON)
	if err == nil {
		_ = tx.Rollback(ctx)
		return Result{Repeated: true, ResponseCode: code, ResponseBody: bodyJSON}, nil
	}
	if err != pgx.ErrNoRows {
		return Result{}, fmt.Errorf("idempotency: lookup: %w", err)
	}

	status, payload, err := fn()
	if err != nil {
		return Result{}, err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return Result{}, fmt.Errorf("idempotency: marshal response: %w", err)
	}
	var uid any
	if userID != "" {
		uid = userID
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO idempotency_keys (key, operation, user_id, response_code, response_body)
		 VALUES ($1, $2, NULLIF($3,'')::uuid, $4, $5)
		 ON CONFLICT (key) DO NOTHING`,
		key, operation, uid, status, string(raw)); err != nil {
		return Result{}, fmt.Errorf("idempotency: store: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, fmt.Errorf("idempotency: commit: %w", err)
	}
	return Result{Repeated: false, ResponseCode: status, ResponseBody: raw}, nil
}
