// Package reports implements minimal report creation from a conversation.
package reports

import (
	"context"
	"errors"
	"strings"

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

var validReasons = map[string]bool{
	"spam": true, "fraud": true, "harassment": true, "safety": true, "off_platform_payment": true, "other": true,
}

// Create creates a report scoped to a conversation/task.
func (s *Service) Create(ctx context.Context, reporterID, conversationID, reason, detail string) (string, error) {
	reason = strings.TrimSpace(strings.ToLower(reason))
	if !validReasons[reason] {
		return "", ErrBadRequest
	}
	var posterID, workerID, taskID string
	err := s.pool.QueryRow(ctx,
		`SELECT poster_id::text, worker_id::text, task_id::text FROM conversations WHERE id=$1::uuid`, conversationID).Scan(&posterID, &workerID, &taskID)
	if err != nil {
		return "", ErrNotFound
	}
	if reporterID != posterID && reporterID != workerID {
		return "", ErrForbidden
	}
	reportedID := posterID
	if reporterID == posterID {
		reportedID = workerID
	}
	var id string
	err = s.pool.QueryRow(ctx,
		`INSERT INTO reports (reporter_id, reported_user_id, task_id, conversation_id, reason, detail, status)
		 VALUES ($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5,$6,'open') RETURNING id::text`,
		reporterID, reportedID, taskID, conversationID, reason, detail).Scan(&id)
	if err != nil {
		return "", err
	}
	_ = audit.Log(ctx, s.pool, audit.Entry{ActorID: &reporterID, Action: "report.create", EntityType: "report", EntityID: id})
	return id, nil
}
