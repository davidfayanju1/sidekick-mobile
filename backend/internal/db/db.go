// Package db owns the Postgres connection (single connection string —
// no Supabase services) and ordered SQL migrations from /migrations.
package db

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	dbmigrations "github.com/sidekick/backend/migrations"
)

// Connect opens a pgx pool. Caller passes the env-specific URL from config.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// Migrate applies embedded migrations in filename order inside one
// transaction per file. Idempotent for CREATE IF NOT EXISTS / DO blocks.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	entries, err := fs.ReadDir(dbmigrations.FS, ".")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, n := range names {
		raw, err := dbmigrations.FS.ReadFile(n)
		if err != nil {
			return fmt.Errorf("read %s: %w", n, err)
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin %s: %w", n, err)
		}
		if _, err := tx.Exec(ctx, string(raw)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply %s: %w", n, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit %s: %w", n, err)
		}
	}
	return nil
}
