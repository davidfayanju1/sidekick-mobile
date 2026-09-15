// Package blocks implements user blocking. A block is bidirectional for
// messaging, feed and offers: either direction hides the other's tasks.
package blocks

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sidekick/backend/internal/audit"
)

var (
	ErrBadRequest = errors.New("bad request")
	ErrNotFound   = errors.New("not found")
	ErrForbidden  = errors.New("forbidden")
)

type Service struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

func (s *Service) Block(ctx context.Context, blockerID, blockedID string) error {
	if blockerID == blockedID {
		return fmt.Errorf("%w: cannot block yourself", ErrBadRequest)
	}
	var exists int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE id=$1::uuid AND deleted_at IS NULL AND suspended_at IS NULL`, blockedID).Scan(&exists)
	if err != nil || exists == 0 {
		return fmt.Errorf("%w: user not found", ErrNotFound)
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO blocks (blocker_id, blocked_id) VALUES ($1::uuid,$2::uuid) ON CONFLICT DO NOTHING`, blockerID, blockedID)
	if err != nil {
		return err
	}
	_ = audit.Log(ctx, s.pool, audit.Entry{ActorID: &blockerID, Action: "block.create", EntityType: "block", EntityID: blockedID, Metadata: map[string]any{"blocked_id": blockedID}})
	return nil
}

func (s *Service) Unblock(ctx context.Context, blockerID, blockedID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM blocks WHERE blocker_id=$1::uuid AND blocked_id=$2::uuid`, blockerID, blockedID)
	return err
}

func (s *Service) List(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT blocked_id::text FROM blocks WHERE blocker_id=$1::uuid ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	if out == nil {
		out = []string{}
	}
	return out, rows.Err()
}

func (s *Service) IsBlocked(ctx context.Context, a, b string) (bool, error) {
	var cnt int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM blocks WHERE (blocker_id=$1::uuid AND blocked_id=$2::uuid) OR (blocker_id=$2::uuid AND blocked_id=$1::uuid)`, a, b).Scan(&cnt)
	if err != nil {
		return false, err
	}
	return cnt > 0, nil
}
