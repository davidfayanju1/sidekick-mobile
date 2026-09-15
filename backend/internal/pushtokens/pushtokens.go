// Package pushtokens registers device tokens (§2.7). Nothing sends a push
// yet (Phase 10) — rows are scoped to the authenticated user only.
package pushtokens

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrBadRequest = errors.New("bad request")

func validPlatform(p string) bool {
	return p == "ios" || p == "android" || p == "web"
}

// Register upserts a token for the user (re-register from another account
// moves it — a device belongs to whoever holds it now).
func Register(ctx context.Context, pool *pgxpool.Pool, userID, token, platform string) error {
	if token == "" || !validPlatform(platform) {
		return fmt.Errorf("%w: token and platform ios|android|web required", ErrBadRequest)
	}
	_, err := pool.Exec(ctx,
		`INSERT INTO push_tokens (user_id, token, platform) VALUES ($1::uuid, $2, $3)
		 ON CONFLICT (token) DO UPDATE SET user_id = EXCLUDED.user_id, platform = EXCLUDED.platform`,
		userID, token, platform)
	return err
}

// Remove deletes one of the user's own tokens.
func Remove(ctx context.Context, pool *pgxpool.Pool, userID, token string) error {
	_, err := pool.Exec(ctx, `DELETE FROM push_tokens WHERE user_id=$1::uuid AND token=$2`, userID, token)
	return err
}

// List returns the user's tokens.
func List(ctx context.Context, pool *pgxpool.Pool, userID string) ([]string, error) {
	rows, err := pool.Query(ctx, `SELECT token FROM push_tokens WHERE user_id=$1::uuid`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var tok string
		if err := rows.Scan(&tok); err != nil {
			return nil, err
		}
		out = append(out, tok)
	}
	return out, rows.Err()
}
