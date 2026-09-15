// Package admin implements Phase 11: the Admin Console backend.
// It provides user management (suspend/ban/reinstate), report actions,
// payout batch management, and admin search — all gated on is_admin.
package admin

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sidekick/backend/internal/audit"
	"github.com/sidekick/backend/internal/notify"
)

var (
	ErrNotFound       = fmt.Errorf("not found")
	ErrBadRequest     = fmt.Errorf("bad request")
	ErrConflict       = fmt.Errorf("conflict")
	ErrForbidden      = fmt.Errorf("forbidden")
	ErrAlreadyBanned  = fmt.Errorf("user already banned")
	ErrNotBanned      = fmt.Errorf("user is not banned")
	ErrNotSuspended   = fmt.Errorf("user is not suspended")
	ErrSelfAction     = fmt.Errorf("cannot perform this action on yourself")
)

// Service holds the admin operations.
type Service struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// ────────────────────────────────────────────────────────────────────────────
// 2.3 Suspend / Ban / Reinstate
// ────────────────────────────────────────────────────────────────────────────

// SuspendUser sets suspended_at and revokes all active sessions.
func (s *Service) SuspendUser(ctx context.Context, adminID, userID, reason string) error {
	if adminID == userID {
		return ErrSelfAction
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var currentSuspended, currentBanned sql.NullTime
	err = tx.QueryRow(ctx,
		`SELECT suspended_at, banned_at FROM users WHERE id=$1::uuid FOR UPDATE`, userID).
		Scan(&currentSuspended, &currentBanned)
	if err != nil {
		return ErrNotFound
	}
	if currentBanned.Valid {
		return fmt.Errorf("%w: user is banned", ErrConflict)
	}
	if currentSuspended.Valid {
		return fmt.Errorf("%w: user already suspended", ErrConflict)
	}

	_, err = tx.Exec(ctx,
		`UPDATE users SET suspended_at=now(), updated_at=now() WHERE id=$1::uuid`, userID)
	if err != nil {
		return err
	}

	// Revoke all sessions
	_, err = tx.Exec(ctx,
		`UPDATE sessions SET revoked_at=now() WHERE user_id=$1::uuid AND revoked_at IS NULL`, userID)
	if err != nil {
		return err
	}

	_ = audit.Log(ctx, s.pool, audit.Entry{
		ActorID: &adminID, Action: "user.suspend",
		EntityType: "user", EntityID: userID,
		Metadata: map[string]any{"reason": reason},
	})

	return tx.Commit(ctx)
}

// BanUser sets both suspended_at and banned_at, revokes all sessions.
func (s *Service) BanUser(ctx context.Context, adminID, userID, reason string) error {
	if adminID == userID {
		return ErrSelfAction
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var currentBanned sql.NullTime
	err = tx.QueryRow(ctx,
		`SELECT banned_at FROM users WHERE id=$1::uuid FOR UPDATE`, userID).
		Scan(&currentBanned)
	if err != nil {
		return ErrNotFound
	}
	if currentBanned.Valid {
		return ErrAlreadyBanned
	}

	_, err = tx.Exec(ctx,
		`UPDATE users SET suspended_at=COALESCE(suspended_at, now()), banned_at=now(), updated_at=now() WHERE id=$1::uuid`, userID)
	if err != nil {
		return err
	}

	// Revoke all sessions
	_, err = tx.Exec(ctx,
		`UPDATE sessions SET revoked_at=now() WHERE user_id=$1::uuid AND revoked_at IS NULL`, userID)
	if err != nil {
		return err
	}

	_ = audit.Log(ctx, s.pool, audit.Entry{
		ActorID: &adminID, Action: "user.ban",
		EntityType: "user", EntityID: userID,
		Metadata: map[string]any{"reason": reason},
	})

	return tx.Commit(ctx)
}

// ReinstateUser clears suspended_at. Rejects if banned_at is set.
func (s *Service) ReinstateUser(ctx context.Context, adminID, userID string) error {
	if adminID == userID {
		return ErrSelfAction
	}
	var currentSuspended, currentBanned sql.NullTime
	err := s.pool.QueryRow(ctx,
		`SELECT suspended_at, banned_at FROM users WHERE id=$1::uuid`, userID).
		Scan(&currentSuspended, &currentBanned)
	if err != nil {
		return ErrNotFound
	}
	if currentBanned.Valid {
		return fmt.Errorf("%w: cannot reinstate a banned user", ErrForbidden)
	}
	if !currentSuspended.Valid {
		return ErrNotSuspended
	}

	_, err = s.pool.Exec(ctx,
		`UPDATE users SET suspended_at=NULL, updated_at=now() WHERE id=$1::uuid`, userID)
	if err != nil {
		return err
	}

	_ = audit.Log(ctx, s.pool, audit.Entry{
		ActorID: &adminID, Action: "user.reinstate",
		EntityType: "user", EntityID: userID,
	})

	return nil
}

// ────────────────────────────────────────────────────────────────────────────
// 2.2 Report action (extends Phase 9's UpdateReportStatus)
// ────────────────────────────────────────────────────────────────────────────

// ActionReport applies a moderation action to a report: dismiss, warn, suspend, ban.
// This extends (does not replace) Phase 9's existing UpdateReportStatus.
func (s *Service) ActionReport(ctx context.Context, adminID, reportID, action, note string) error {
	action = strings.ToLower(strings.TrimSpace(action))
	if action != "dismiss" && action != "warn" && action != "suspend" && action != "ban" {
		return fmt.Errorf("%w: action must be dismiss, warn, suspend, or ban", ErrBadRequest)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Load report — row lock to prevent concurrent actions
	var reportStatus, reportedUserID string
	err = tx.QueryRow(ctx,
		`SELECT status, reported_user_id::text FROM reports WHERE id=$1::uuid FOR UPDATE`, reportID).
		Scan(&reportStatus, &reportedUserID)
	if err != nil {
		return ErrNotFound
	}
	if reportStatus != "open" && reportStatus != "reviewing" {
		return fmt.Errorf("%w: report not actionable (status=%s)", ErrConflict, reportStatus)
	}

	// Apply the action
	switch action {
	case "dismiss":
		_, err = tx.Exec(ctx,
			`UPDATE reports SET status='dismissed', handled_by=$1::uuid, action_note=$3 WHERE id=$2::uuid`,
			adminID, reportID, note)
		if err != nil {
			return err
		}

	case "warn":
		_, err = tx.Exec(ctx,
			`UPDATE reports SET status='actioned', handled_by=$1::uuid, action_note=$3 WHERE id=$2::uuid`,
			adminID, reportID, note)
		if err != nil {
			return err
		}
		// Send notification to reported user
		_, _ = notify.InsertNotifTx(ctx, tx, reportedUserID, "warning_received", "both",
			notify.Payload("warning_received", map[string]string{"report_id": reportID}))

	case "suspend":
		_, err = tx.Exec(ctx,
			`UPDATE reports SET status='actioned', handled_by=$1::uuid, action_note=$3 WHERE id=$2::uuid`,
			adminID, reportID, note)
		if err != nil {
			return err
		}
		// Suspend the reported user
		_, err = tx.Exec(ctx,
			`UPDATE users SET suspended_at=now(), updated_at=now() WHERE id=$1::uuid AND banned_at IS NULL AND suspended_at IS NULL`,
			reportedUserID)
		if err != nil {
			return err
		}
		// Revoke sessions
		_, _ = tx.Exec(ctx,
			`UPDATE sessions SET revoked_at=now() WHERE user_id=$1::uuid AND revoked_at IS NULL`, reportedUserID)

	case "ban":
		_, err = tx.Exec(ctx,
			`UPDATE reports SET status='actioned', handled_by=$1::uuid, action_note=$3 WHERE id=$2::uuid`,
			adminID, reportID, note)
		if err != nil {
			return err
		}
		// Ban the reported user
		_, err = tx.Exec(ctx,
			`UPDATE users SET suspended_at=COALESCE(suspended_at, now()), banned_at=now(), updated_at=now() WHERE id=$1::uuid AND banned_at IS NULL`,
			reportedUserID)
		if err != nil {
			return err
		}
		_, _ = tx.Exec(ctx,
			`UPDATE sessions SET revoked_at=now() WHERE user_id=$1::uuid AND revoked_at IS NULL`, reportedUserID)
	}

	_ = audit.Log(ctx, s.pool, audit.Entry{
		ActorID: &adminID, Action: "report.action." + action,
		EntityType: "report", EntityID: reportID,
		Metadata: map[string]any{"action": action, "note": note, "reported_user_id": reportedUserID},
	})

	return tx.Commit(ctx)
}

// ────────────────────────────────────────────────────────────────────────────
// 2.5 Payout batches
// ────────────────────────────────────────────────────────────────────────────

// CreateBatch groups all processing withdrawals as of the given date into a batch.
func (s *Service) CreateBatch(ctx context.Context, adminID, date string) (map[string]any, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Find all eligible withdrawal transactions
	rows, err := tx.Query(ctx,
		`SELECT id::text, payer_id::text, amount FROM transactions
		 WHERE type='withdrawal' AND status='processing'
		   AND created_at <= $1::timestamptz
		 ORDER BY created_at ASC`, date+"T23:59:59Z")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type eligible struct {
		ID     string
		UserID string
		Amount int
	}
	var items []eligible
	for rows.Next() {
		var e eligible
		if err := rows.Scan(&e.ID, &e.UserID, &e.Amount); err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("%w: no eligible withdrawals for date %s", ErrNotFound, date)
	}

	// Calculate total
	totalAmount := 0
	for _, item := range items {
		totalAmount += item.Amount
	}

	// Create batch
	var batchID string
	err = tx.QueryRow(ctx,
		`INSERT INTO payout_batches (status, created_by, item_count, total_amount)
		 VALUES ('open', $1::uuid, $2, $3) RETURNING id::text`,
		adminID, len(items), totalAmount).Scan(&batchID)
	if err != nil {
		return nil, err
	}

	// Create payout items
	for _, item := range items {
		_, err = tx.Exec(ctx,
			`INSERT INTO payout_items (batch_id, transaction_id, user_id, amount, status)
			 VALUES ($1::uuid, $2::uuid, $3::uuid, $4, 'pending')
			 ON CONFLICT (batch_id, transaction_id) DO NOTHING`,
			batchID, item.ID, item.UserID, item.Amount)
		if err != nil {
			return nil, err
		}
	}

	_ = audit.Log(ctx, s.pool, audit.Entry{
		ActorID: &adminID, Action: "payout_batch.create",
		EntityType: "payout_batch", EntityID: batchID,
		Metadata: map[string]any{"item_count": len(items), "total_amount": totalAmount, "date": date},
	})

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return map[string]any{
		"id": batchID, "status": "open", "item_count": len(items),
		"total_amount": totalAmount, "created_at": time.Now(),
	}, nil
}

// ConfirmBatch marks a batch as confirmed and sets items to processing.
func (s *Service) ConfirmBatch(ctx context.Context, adminID, batchID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	err = tx.QueryRow(ctx,
		`SELECT status FROM payout_batches WHERE id=$1::uuid FOR UPDATE`, batchID).
		Scan(&status)
	if err != nil {
		return ErrNotFound
	}
	if status != "open" {
		return fmt.Errorf("%w: batch not open (status=%s)", ErrConflict, status)
	}

	_, err = tx.Exec(ctx,
		`UPDATE payout_batches SET status='confirmed', processed_at=now() WHERE id=$1::uuid`, batchID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx,
		`UPDATE payout_items SET status='processing' WHERE batch_id=$1::uuid AND status='pending'`, batchID)
	if err != nil {
		return err
	}

	_ = audit.Log(ctx, s.pool, audit.Entry{
		ActorID: &adminID, Action: "payout_batch.confirm",
		EntityType: "payout_batch", EntityID: batchID,
	})

	return tx.Commit(ctx)
}

// ListBatches returns all payout batches, most recent first.
func (s *Service) ListBatches(ctx context.Context) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id::text, status, created_by::text, item_count, total_amount, processed_at, created_at
		 FROM payout_batches ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, status string
		var createdBy *string
		var itemCount, totalAmount int
		var processedAt, createdAt interface{}
		if err := rows.Scan(&id, &status, &createdBy, &itemCount, &totalAmount, &processedAt, &createdAt); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id": id, "status": status, "created_by": createdBy,
			"item_count": itemCount, "total_amount": totalAmount,
			"processed_at": processedAt, "created_at": createdAt,
		})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

// GetBatch returns a single payout batch with its items.
func (s *Service) GetBatch(ctx context.Context, batchID string) (map[string]any, error) {
	var id, status string
	var createdBy *string
	var itemCount, totalAmount int
	var processedAt, createdAt interface{}
	err := s.pool.QueryRow(ctx,
		`SELECT id::text, status, created_by::text, item_count, total_amount, processed_at, created_at
		 FROM payout_batches WHERE id=$1::uuid`, batchID).
		Scan(&id, &status, &createdBy, &itemCount, &totalAmount, &processedAt, &createdAt)
	if err != nil {
		return nil, ErrNotFound
	}

	// Fetch items
	rows, err := s.pool.Query(ctx,
		`SELECT pi.id::text, pi.transaction_id::text, pi.user_id::text, pi.amount, pi.status, pi.created_at
		 FROM payout_items pi WHERE pi.batch_id=$1::uuid ORDER BY pi.created_at`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []map[string]any
	for rows.Next() {
		var itemID, txID, userID string
		var amount int
		var itemStatus string
		var itemCreatedAt interface{}
		if err := rows.Scan(&itemID, &txID, &userID, &amount, &itemStatus, &itemCreatedAt); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{
			"id": itemID, "transaction_id": txID, "user_id": userID,
			"amount": amount, "status": itemStatus, "created_at": itemCreatedAt,
		})
	}
	if items == nil {
		items = []map[string]any{}
	}

	return map[string]any{
		"id": id, "status": status, "created_by": createdBy,
		"item_count": itemCount, "total_amount": totalAmount,
		"processed_at": processedAt, "created_at": createdAt,
		"items": items,
	}, nil
}

// ────────────────────────────────────────────────────────────────────────────
// 2.6 Admin search
// ────────────────────────────────────────────────────────────────────────────

// Search searches users or tasks by query.
func (s *Service) Search(ctx context.Context, adminID, query, typ string) (map[string]any, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("%w: query required", ErrBadRequest)
	}
	typ = strings.ToLower(strings.TrimSpace(typ))
	if typ != "user" && typ != "task" {
		return nil, fmt.Errorf("%w: type must be user or task", ErrBadRequest)
	}

	var results []map[string]any
	switch typ {
	case "user":
		rows, err := s.pool.Query(ctx,
			`SELECT id::text, email, phone, display_name, avatar_url, verification_status,
			        rating_avg, rating_count, suspended_at, banned_at, created_at
			 FROM users
			 WHERE (email ILIKE '%' || $1 || '%'
			        OR phone ILIKE '%' || $1 || '%'
			        OR display_name ILIKE '%' || $1 || '%')
			   AND deleted_at IS NULL
			 ORDER BY created_at DESC LIMIT 20`, query)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			var email *string
			var phone *string
			var displayName, verificationStatus string
			var avatarURL *string
			var ratingAvg float64
			var ratingCount int
			var suspendedAt, bannedAt sql.NullTime
			var createdAt interface{}
			if err := rows.Scan(&id, &email, &phone, &displayName, &avatarURL,
				&verificationStatus, &ratingAvg, &ratingCount,
				&suspendedAt, &bannedAt, &createdAt); err != nil {
				return nil, err
			}
			results = append(results, map[string]any{
				"id": id, "email": email, "phone": phone,
				"display_name": displayName, "avatar_url": avatarURL,
				"verification_status": verificationStatus,
				"rating_avg": ratingAvg, "rating_count": ratingCount,
				"suspended_at": suspendedAt.Valid, "banned_at": bannedAt.Valid,
				"created_at": createdAt,
			})
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}

	case "task":
		rows, err := s.pool.Query(ctx,
			`SELECT id::text, title, status, poster_id::text, budget, created_at
			 FROM tasks
			 WHERE id::text ILIKE '%' || $1 || '%'
			    OR title ILIKE '%' || $1 || '%'
			 ORDER BY created_at DESC LIMIT 20`, query)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var id, title, status, posterID string
			var budget int
			var createdAt interface{}
			if err := rows.Scan(&id, &title, &status, &posterID, &budget, &createdAt); err != nil {
				return nil, err
			}
			results = append(results, map[string]any{
				"id": id, "title": title, "status": status,
				"poster_id": posterID, "budget": budget, "created_at": createdAt,
			})
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	if results == nil {
		results = []map[string]any{}
	}

	// Audit log every search (this endpoint can surface PII)
	_ = audit.Log(ctx, s.pool, audit.Entry{
		ActorID: &adminID, Action: "admin.search",
		EntityType: typ, EntityID: query,
		Metadata: map[string]any{"query": query, "type": typ, "result_count": len(results)},
	})

	return map[string]any{"results": results, "count": len(results)}, nil
}

// ────────────────────────────────────────────────────────────────────────────
// Enhanced verification listing (§2.1)
// ────────────────────────────────────────────────────────────────────────────

// ListVerificationsEnhanced returns verifications with user profiles and history.
func (s *Service) ListVerificationsEnhanced(ctx context.Context, status string) ([]map[string]any, error) {
	if status == "" {
		status = "pending"
	}
	rows, err := s.pool.Query(ctx,
		`SELECT v.id::text, v.user_id::text, v.document_url, v.document_type, v.status,
		        v.rejection_reason, v.created_at,
		        u.display_name, u.email, u.phone, u.avatar_url,
		        u.verification_status, u.rating_avg, u.rating_count
		 FROM verifications v
		 JOIN users u ON u.id = v.user_id
		 WHERE v.status=$1
		 ORDER BY v.created_at ASC`, status)
	if err != nil {
		return nil, err
	}
	type vRow struct {
		id, userID, docType, vStatus, userVStatus string
		docURL                                     string
		rejectionReason                            *string
		createdAt                                  interface{}
		displayName                                string
		email, phone, avatarURL                    *string
		ratingAvg                                  float64
		ratingCount                                int
	}
	var vRows []vRow
	for rows.Next() {
		var r vRow
		if err := rows.Scan(&r.id, &r.userID, &r.docURL, &r.docType, &r.vStatus, &r.rejectionReason, &r.createdAt,
			&r.displayName, &r.email, &r.phone, &r.avatarURL,
			&r.userVStatus, &r.ratingAvg, &r.ratingCount); err != nil {
			return nil, err
		}
		vRows = append(vRows, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	var out []map[string]any
	for _, r := range vRows {
		history := s.getRejectionHistory(ctx, r.userID)

		out = append(out, map[string]any{
			"id": r.id, "user_id": r.userID,
			"document_url": r.docURL, "document_type": r.docType,
			"status": r.vStatus, "rejection_reason": r.rejectionReason,
			"created_at": r.createdAt,
			"user": map[string]any{
				"display_name": r.displayName, "email": r.email, "phone": r.phone,
				"avatar_url": r.avatarURL, "verification_status": r.userVStatus,
				"rating_avg": r.ratingAvg, "rating_count": r.ratingCount,
			},
			"prior_rejections": history,
		})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, nil
}

func (s *Service) getRejectionHistory(ctx context.Context, userID string) []map[string]any {
	rows, err := s.pool.Query(ctx,
		`SELECT id::text, rejection_reason, created_at FROM verifications
		 WHERE user_id=$1::uuid AND status='rejected' ORDER BY created_at DESC`, userID)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()
	var history []map[string]any
	for rows.Next() {
		var id string
		var reason *string
		var createdAt interface{}
		if err := rows.Scan(&id, &reason, &createdAt); err != nil {
			continue
		}
		history = append(history, map[string]any{
			"id": id, "rejection_reason": reason, "created_at": createdAt,
		})
	}
	if history == nil {
		history = []map[string]any{}
	}
	return history
}

// ────────────────────────────────────────────────────────────────────────────
// Enhanced dispute listing (§2.4)
// ────────────────────────────────────────────────────────────────────────────

// ListDisputesEnhanced returns disputes with task, thread, evidence, escrow, profiles.
func (s *Service) ListDisputesEnhanced(ctx context.Context, status string) ([]map[string]any, error) {
	if status == "" {
		status = "open"
	}
	rows, err := s.pool.Query(ctx,
		`SELECT d.id::text, d.task_id::text, d.raised_by::text, d.reason, d.description,
		        d.evidence_urls, d.status, d.resolution,
		        d.compensation_amount, d.created_at,
		        t.title, t.budget, t.escrow_status, t.status as task_status,
		        t.poster_id::text, t.assigned_worker_id::text,
		        pu.display_name as poster_name, pu.email as poster_email,
		        pu.avatar_url as poster_avatar,
		        wu.display_name as worker_name, wu.email as worker_email,
		        wu.avatar_url as worker_avatar
		 FROM disputes d
		 JOIN tasks t ON t.id = d.task_id
		 JOIN users pu ON pu.id = t.poster_id
		 LEFT JOIN users wu ON wu.id = t.assigned_worker_id
		 WHERE d.status=$1
		 ORDER BY d.created_at ASC`, status)
	if err != nil {
		return nil, err
	}
	type dRow struct {
		id, taskID, raisedBy, reason, description, taskStatus string
		evidenceURLs                                           []string
		disputeStatus                                          string
		resolution                                             *string
		compensationAmount                                     *int
		createdAt                                              interface{}
		title                                                  string
		budget                                                 int
		escrowStatus                                           string
		posterID, posterName                                   string
		workerID                                               *string
		posterEmail, posterAvatar                              *string
		workerName, workerEmail, workerAvatar                  *string
	}
	var dRows []dRow
	for rows.Next() {
		var r dRow
		if err := rows.Scan(&r.id, &r.taskID, &r.raisedBy, &r.reason, &r.description,
			&r.evidenceURLs, &r.disputeStatus, &r.resolution,
			&r.compensationAmount, &r.createdAt,
			&r.title, &r.budget, &r.escrowStatus, &r.taskStatus,
			&r.posterID, &r.workerID,
			&r.posterName, &r.posterEmail, &r.posterAvatar,
			&r.workerName, &r.workerEmail, &r.workerAvatar); err != nil {
			return nil, err
		}
		dRows = append(dRows, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	var out []map[string]any
	for _, r := range dRows {
		thread := s.getDisputeThread(ctx, r.taskID)

		out = append(out, map[string]any{
			"id": r.id, "task_id": r.taskID, "raised_by": r.raisedBy,
			"reason": r.reason, "description": r.description,
			"evidence_urls": r.evidenceURLs, "status": r.disputeStatus,
			"resolution": r.resolution,
			"compensation_amount": r.compensationAmount,
			"created_at": r.createdAt,
			"task": map[string]any{
				"title": r.title, "budget": r.budget,
				"escrow_status": r.escrowStatus, "status": r.taskStatus,
			},
			"poster": map[string]any{
				"id": r.posterID, "display_name": r.posterName,
				"email": r.posterEmail, "avatar_url": r.posterAvatar,
			},
			"worker": map[string]any{
				"id": r.workerID, "display_name": r.workerName,
				"email": r.workerEmail, "avatar_url": r.workerAvatar,
			},
			"thread": thread,
		})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, nil
}

func (s *Service) getDisputeThread(ctx context.Context, taskID string) []map[string]any {
	var convID string
	err := s.pool.QueryRow(ctx,
		`SELECT id::text FROM conversations WHERE task_id=$1::uuid`, taskID).Scan(&convID)
	if err != nil {
		return []map[string]any{}
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id::text, sender_id::text, body, type, created_at
		 FROM messages WHERE conversation_id=$1::uuid ORDER BY created_at ASC`, convID)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()
	var thread []map[string]any
	for rows.Next() {
		var id, body, msgType string
		var senderID *string
		var createdAt interface{}
		if err := rows.Scan(&id, &senderID, &body, &msgType, &createdAt); err != nil {
			continue
		}
		thread = append(thread, map[string]any{
			"id": id, "sender_id": senderID, "body": body,
			"type": msgType, "created_at": createdAt,
		})
	}
	if thread == nil {
		thread = []map[string]any{}
	}
	return thread
}
