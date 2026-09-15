// Package reviews implements Phase 8: ratings & reputation with double-blind.
//
// Decision (Phase 8 §0): Only tasks with status = 'completed' are reviewable.
// Disputed/resolved tasks (resolved_released etc. from Phase 9) are NOT
// reviewable for MVP. This keeps the signal as "uncomplicated job well done"
// and avoids mixing dispute outcomes into reputation. Phase 9 can extend
// eligibility with a small change to isTaskEligible if needed.

package reviews

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sidekick/backend/internal/notify"
)

var (
	ErrBadRequest = errors.New("bad request")
	ErrNotFound   = errors.New("not found")
	ErrForbidden  = errors.New("forbidden")
	ErrConflict   = errors.New("conflict")
)

type Review struct {
	ID          string     `json:"id"`
	TaskID      string     `json:"task_id"`
	ReviewerID  string     `json:"reviewer_id"`
	RevieweeID  string     `json:"reviewee_id"`
	Rating      int        `json:"rating"`
	Body        *string    `json:"body,omitempty"`
	IsPublished bool       `json:"is_published"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type Service struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// isTaskEligible checks status == 'completed' (MVP decision). Documented above.
func isTaskEligible(status string) bool {
	return status == "completed"
}

// Submit creates a review with is_published=false, always. Calls tryPublish.
func (s *Service) Submit(ctx context.Context, reviewerID, taskID string, rating int, body *string) (*Review, error) {
	if rating < 1 || rating > 5 {
		return nil, fmt.Errorf("%w: rating must be 1-5", ErrBadRequest)
	}
	if body != nil && len([]rune(*body)) > 500 {
		return nil, fmt.Errorf("%w: body max 500 chars", ErrBadRequest)
	}
	// Load task and validate eligibility + party membership
	var status, posterID, workerID string
	var workerIDPtr *string
	err := s.pool.QueryRow(ctx, `SELECT status, poster_id::text, assigned_worker_id::text FROM tasks WHERE id=$1::uuid`, taskID).Scan(&status, &posterID, &workerIDPtr)
	if err != nil {
		return nil, ErrNotFound
	}
	if workerIDPtr != nil {
		workerID = *workerIDPtr
	}
	if !isTaskEligible(status) {
		return nil, fmt.Errorf("%w: task not eligible for review (status %s)", ErrBadRequest, status)
	}
	if reviewerID != posterID && reviewerID != workerID {
		return nil, ErrForbidden
	}
	if workerID == "" {
		return nil, fmt.Errorf("%w: task has no assigned worker", ErrBadRequest)
	}
	// Determine reviewee is the other party
	var revieweeID string
	if reviewerID == posterID {
		revieweeID = workerID
	} else {
		revieweeID = posterID
	}
	if revieweeID == "" || reviewerID == revieweeID {
		return nil, fmt.Errorf("%w: invalid reviewee", ErrBadRequest)
	}
	// Insert; UNIQUE(task_id, reviewer_id) prevents double review
	var r Review
	err = s.pool.QueryRow(ctx,
		`INSERT INTO reviews (task_id, reviewer_id, reviewee_id, rating, body, is_published, published_at)
		 VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, false, NULL)
		 RETURNING id::text, task_id::text, reviewer_id::text, reviewee_id::text, rating, body, is_published, published_at, created_at`,
		taskID, reviewerID, revieweeID, rating, body).Scan(&r.ID, &r.TaskID, &r.ReviewerID, &r.RevieweeID, &r.Rating, &r.Body, &r.IsPublished, &r.PublishedAt, &r.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			return nil, fmt.Errorf("%w: already reviewed this task", ErrConflict)
		}
		if strings.Contains(err.Error(), "reviewer_id <> reviewee_id") {
			return nil, fmt.Errorf("%w: cannot review yourself", ErrBadRequest)
		}
		return nil, err
	}
	// Try to publish double-blind immediately (second review triggers both)
	_ = s.tryPublishReviews(ctx, taskID)
	// Return fresh (may now be published if this was second)
	_ = s.pool.QueryRow(ctx, `SELECT is_published, published_at FROM reviews WHERE id=$1::uuid`, r.ID).Scan(&r.IsPublished, &r.PublishedAt)
	return &r, nil
}

// tryPublishReviews is the SINGLE double-blind publish function.
// Called after each submit and from the 14-day cron. Idempotent.
// Logic: if both poster->worker and worker->poster reviews exist and are unpublished, publish both atomically and recompute aggregates.
// If called from cron for one-sided case, see PublishOneSided.
func (s *Service) tryPublishReviews(ctx context.Context, taskID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Lock reviews for this task
	rows, err := tx.Query(ctx, `SELECT id::text, reviewer_id::text, reviewee_id::text FROM reviews WHERE task_id=$1::uuid AND is_published=false FOR UPDATE`, taskID)
	if err != nil {
		return err
	}
	var unpublished []struct{ id, reviewer, reviewee string }
	for rows.Next() {
		var id, rev, ree string
		if err := rows.Scan(&id, &rev, &ree); err != nil {
			rows.Close()
			return err
		}
		unpublished = append(unpublished, struct{ id, reviewer, reviewee string }{id, rev, ree})
	}
	rows.Close()
	if len(unpublished) != 2 {
		// Need exactly 2 unpublished (one per direction) to double-blind publish
		return tx.Commit(ctx)
	}
	// Verify they are opposite directions (poster<->worker)
	// There should be exactly 2, reviewer IDs should be distinct and cover both parties
	if unpublished[0].reviewer == unpublished[1].reviewer {
		return tx.Commit(ctx)
	}
	// Publish both
	_, err = tx.Exec(ctx, `UPDATE reviews SET is_published=true, published_at=now() WHERE task_id=$1::uuid AND is_published=false`, taskID)
	if err != nil {
		return err
	}
	// Recompute aggregates for both reviewees
	reviewees := map[string]bool{unpublished[0].reviewee: true, unpublished[1].reviewee: true}
	for rid := range reviewees {
		if err := s.recomputeAggregateTx(ctx, tx, rid); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Service) recomputeAggregateTx(ctx context.Context, tx pgx.Tx, userID string) error {
	var avg *float64
	var cnt int
	err := tx.QueryRow(ctx, `SELECT AVG(rating)::float, COUNT(*) FROM reviews WHERE reviewee_id=$1::uuid AND is_published=true`, userID).Scan(&avg, &cnt)
	if err != nil {
		return err
	}
	val := 0.0
	if avg != nil {
		val = *avg
	}
	_, err = tx.Exec(ctx, `UPDATE users SET rating_avg=$2, rating_count=$3, updated_at=now() WHERE id=$1::uuid`, userID, val, cnt)
	return err
}

// PublishOneSided publishes a single review after 14 days (cron). Idempotent.
func (s *Service) publishOneSidedTx(ctx context.Context, tx pgx.Tx, reviewID, revieweeID string) error {
	_, err := tx.Exec(ctx, `UPDATE reviews SET is_published=true, published_at=now() WHERE id=$1::uuid AND is_published=false`, reviewID)
	if err != nil {
		return err
	}
	// Phase 10 retrofit: notify reviewee of published rating
	var taskID string
	_ = tx.QueryRow(ctx, `SELECT task_id::text FROM reviews WHERE id=$1::uuid`, reviewID).Scan(&taskID)
	_, _ = notify.InsertNotifTx(ctx, tx, revieweeID, notify.EventRatingReceived, "in_app",
		notify.Payload(notify.EventRatingReceived, map[string]string{"task_id": taskID, "review_id": reviewID}))
	return s.recomputeAggregateTx(ctx, tx, revieweeID)
}

// PublishCron finds tasks completed 14+ days ago with at least one unpublished review and publishes them.
func (s *Service) PublishCron(ctx context.Context) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Find unpublished reviews where task completed_at <= now - 14 days
	rows, err := tx.Query(ctx, `
		SELECT r.id::text, r.reviewee_id::text, r.task_id::text
		FROM reviews r
		JOIN tasks t ON t.id = r.task_id
		WHERE r.is_published=false AND t.status='completed' AND t.completed_at <= now() - interval '14 days'
		FOR UPDATE OF r`)
	if err != nil {
		return 0, err
	}
	var toPublish []struct{ id, reviewee, taskID string }
	for rows.Next() {
		var id, ree, tid string
		if err := rows.Scan(&id, &ree, &tid); err != nil {
			rows.Close()
			return 0, err
		}
		toPublish = append(toPublish, struct{ id, reviewee, taskID string }{id, ree, tid})
	}
	rows.Close()
	if len(toPublish) == 0 {
		_ = tx.Commit(ctx)
		return 0, nil
	}
	// For each, publish and recompute. Group by reviewee to avoid double recompute? Just do per review.
	published := 0
	for _, p := range toPublish {
		// Check still unpublished (might have been published by tryPublish in same txn? but we already locked)
		var isPub bool
		_ = tx.QueryRow(ctx, `SELECT is_published FROM reviews WHERE id=$1::uuid`, p.id).Scan(&isPub)
		if isPub {
			continue
		}
		if err := s.publishOneSidedTx(ctx, tx, p.id, p.reviewee); err != nil {
			return published, err
		}
		published++
	}
	if err := tx.Commit(ctx); err != nil {
		return published, err
	}
	return published, nil
}

// ListPublic returns only published reviews for a user as reviewee, most recent first.
func (s *Service) ListPublic(ctx context.Context, userID string, limit int) ([]Review, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id::text, task_id::text, reviewer_id::text, reviewee_id::text, rating, body, is_published, published_at, created_at
		 FROM reviews WHERE reviewee_id=$1::uuid AND is_published=true ORDER BY published_at DESC, created_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Review
	for rows.Next() {
		var r Review
		if err := rows.Scan(&r.ID, &r.TaskID, &r.ReviewerID, &r.RevieweeID, &r.Rating, &r.Body, &r.IsPublished, &r.PublishedAt, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if out == nil {
		out = []Review{}
	}
	return out, rows.Err()
}

// ListOwn returns reviews given and received for the requesting user.
// For received unpublished reviews, hides rating/body, shows pending flag.
func (s *Service) ListOwn(ctx context.Context, userID string) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id::text, task_id::text, reviewer_id::text, reviewee_id::text, rating, body, is_published, published_at, created_at
		 FROM reviews WHERE reviewer_id=$1::uuid OR reviewee_id=$1::uuid
		 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var r Review
		if err := rows.Scan(&r.ID, &r.TaskID, &r.ReviewerID, &r.RevieweeID, &r.Rating, &r.Body, &r.IsPublished, &r.PublishedAt, &r.CreatedAt); err != nil {
			return nil, err
		}
		m := map[string]any{
			"id": r.ID, "task_id": r.TaskID, "reviewer_id": r.ReviewerID, "reviewee_id": r.RevieweeID,
			"is_published": r.IsPublished, "published_at": r.PublishedAt, "created_at": r.CreatedAt,
		}
		// If this review is about the requester and unpublished, hide rating/body
		if r.RevieweeID == userID && !r.IsPublished {
			m["rating"] = nil
			m["body"] = nil
			m["pending"] = true
		} else {
			m["rating"] = r.Rating
			m["body"] = r.Body
			m["pending"] = false
		}
		out = append(out, m)
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

// GetOwnPendingStatus is helper for tests to see if other party has reviewed.
func (s *Service) GetOwnPendingStatus(ctx context.Context, userID, taskID string) (mySubmitted bool, otherSubmitted bool, err error) {
	err = s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM reviews WHERE task_id=$1::uuid AND reviewer_id=$2::uuid)`, taskID, userID).Scan(&mySubmitted)
	if err != nil {
		return
	}
	// Find other party
	var posterID, workerID *string
	_ = s.pool.QueryRow(ctx, `SELECT poster_id::text, assigned_worker_id::text FROM tasks WHERE id=$1::uuid`, taskID).Scan(&posterID, &workerID)
	var otherID string
	if posterID != nil && *posterID == userID {
		if workerID != nil {
			otherID = *workerID
		}
	} else if workerID != nil && *workerID == userID {
		if posterID != nil {
			otherID = *posterID
		}
	}
	if otherID != "" {
		_ = s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM reviews WHERE task_id=$1::uuid AND reviewer_id=$2::uuid)`, taskID, otherID).Scan(&otherSubmitted)
	}
	return
}
