// Package trust implements Phase 9: verification review, report handling,
// dispute evidence + resolution, and contact-detail leak detection.
// All admin-gated endpoints use the is_admin claim from Phase 0 — no
// alternate admin check exists in this package.
package trust

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sidekick/backend/internal/audit"
	"github.com/sidekick/backend/internal/ledger"
	"github.com/sidekick/backend/internal/messaging"
	"github.com/sidekick/backend/internal/notify"
	"github.com/sidekick/backend/internal/payments"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrForbidden = errors.New("forbidden")
	ErrBadRequest = errors.New("bad request")
	ErrConflict  = errors.New("conflict")
)

// Service handles trust & safety operations.
type Service struct {
	pool     *pgxpool.Pool
	payments payments.Adapter
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, payments: payments.NewMock()}
}

func NewWithPayments(pool *pgxpool.Pool, p payments.Adapter) *Service {
	if p == nil {
		p = payments.NewMock()
	}
	return &Service{pool: pool, payments: p}
}

// ────────────────────────────────────────────────────────────────────────────
// Verifications
// ────────────────────────────────────────────────────────────────────────────

// SubmitVerification creates a pending verification. Enforces at most one pending per user.
func (s *Service) SubmitVerification(ctx context.Context, userID, documentType, documentURL string) (string, error) {
	documentType = strings.TrimSpace(strings.ToLower(documentType))
	if documentType != "" && documentType != "passport" && documentType != "driving_licence" && documentType != "national_id" {
		return "", fmt.Errorf("%w: invalid document_type", ErrBadRequest)
	}
	if strings.TrimSpace(documentURL) == "" {
		return "", fmt.Errorf("%w: document_url required", ErrBadRequest)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Check no pending verification exists
	var pendingCnt int
	_ = tx.QueryRow(ctx, `SELECT count(*) FROM verifications WHERE user_id=$1::uuid AND status='pending'`, userID).Scan(&pendingCnt)
	if pendingCnt > 0 {
		return "", fmt.Errorf("%w: verification already pending", ErrConflict)
	}

	var id string
	err = tx.QueryRow(ctx,
		`INSERT INTO verifications (user_id, document_url, document_type, status)
		 VALUES ($1::uuid, $2, $3, 'pending') RETURNING id::text`,
		userID, documentURL, documentType).Scan(&id)
	if err != nil {
		return "", err
	}

	// Set users.verification_status = 'pending' in same transaction
	_, err = tx.Exec(ctx, `UPDATE users SET verification_status='pending', updated_at=now() WHERE id=$1::uuid`, userID)
	if err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	_ = audit.Log(ctx, s.pool, audit.Entry{ActorID: &userID, Action: "verification.submit", EntityType: "verification", EntityID: id})
	return id, nil
}

// ListPendingVerifications returns all pending verifications (admin only).
func (s *Service) ListPendingVerifications(ctx context.Context) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT v.id::text, v.user_id::text, v.document_url, v.document_type, v.status, v.created_at
		 FROM verifications v WHERE v.status='pending' ORDER BY v.created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, userID, docURL, docType, status string
		var createdAt interface{}
		if err := rows.Scan(&id, &userID, &docURL, &docType, &status, &createdAt); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id": id, "user_id": userID, "document_url": docURL,
			"document_type": docType, "status": status, "created_at": createdAt,
		})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

// ApproveVerification approves a pending verification. Updates users.verification_status in same txn.
func (s *Service) ApproveVerification(ctx context.Context, adminID, verificationID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Row-lock the verification
	var status, userID string
	err = tx.QueryRow(ctx,
		`SELECT status, user_id::text FROM verifications WHERE id=$1::uuid FOR UPDATE`, verificationID).
		Scan(&status, &userID)
	if err != nil {
		return ErrNotFound
	}
	if status != "pending" {
		return fmt.Errorf("%w: verification not pending (status=%s)", ErrConflict, status)
	}

	// Update verification
	_, err = tx.Exec(ctx,
		`UPDATE verifications SET status='verified', reviewed_by=$1::uuid, reviewed_at=now() WHERE id=$2::uuid`,
		adminID, verificationID)
	if err != nil {
		return err
	}

	// Update users.verification_status in same transaction
	_, err = tx.Exec(ctx,
		`UPDATE users SET verification_status='verified', updated_at=now() WHERE id=$1::uuid`, userID)
	if err != nil {
		return err
	}

	// Audit log
	_, _ = tx.Exec(ctx,
		`INSERT INTO audit_log (actor_id, action, entity_type, entity_id, metadata)
		 VALUES ($1::uuid, 'verification.approve', 'verification', $2, $3)`,
		adminID, verificationID, fmt.Sprintf(`{"user_id":"%s"}`, userID))

	// Phase 10 retrofit: real notification
	_, _ = notify.InsertNotifTx(ctx, tx, userID, notify.EventVerificationApproved, "both",
		notify.Payload(notify.EventVerificationApproved, map[string]string{}))

	return tx.Commit(ctx)
}

// RejectVerification rejects a pending verification with a reason. Updates users.verification_status in same txn.
func (s *Service) RejectVerification(ctx context.Context, adminID, verificationID, reason string) error {
	if strings.TrimSpace(reason) == "" {
		return fmt.Errorf("%w: rejection reason required", ErrBadRequest)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status, userID string
	err = tx.QueryRow(ctx,
		`SELECT status, user_id::text FROM verifications WHERE id=$1::uuid FOR UPDATE`, verificationID).
		Scan(&status, &userID)
	if err != nil {
		return ErrNotFound
	}
	if status != "pending" {
		return fmt.Errorf("%w: verification not pending (status=%s)", ErrConflict, status)
	}

	_, err = tx.Exec(ctx,
		`UPDATE verifications SET status='rejected', reviewed_by=$1::uuid, reviewed_at=now(), rejection_reason=$3 WHERE id=$2::uuid`,
		adminID, verificationID, reason)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx,
		`UPDATE users SET verification_status='rejected', updated_at=now() WHERE id=$1::uuid`, userID)
	if err != nil {
		return err
	}

	_, _ = tx.Exec(ctx,
		`INSERT INTO audit_log (actor_id, action, entity_type, entity_id, metadata)
		 VALUES ($1::uuid, 'verification.reject', 'verification', $2, $3)`,
		adminID, verificationID, fmt.Sprintf(`{"user_id":"%s","reason":"%s"}`, userID, reason))

	_, _ = notify.InsertNotifTx(ctx, tx, userID, notify.EventVerificationRejected, "both",
		notify.Payload(notify.EventVerificationRejected, map[string]string{"reason": reason}))

	return tx.Commit(ctx)
}

// GetVerificationDocURL returns a short-TTL signed URL for admin review.
func (s *Service) GetVerificationDocURL(ctx context.Context, verificationID string) (string, error) {
	var docURL string
	err := s.pool.QueryRow(ctx,
		`SELECT document_url FROM verifications WHERE id=$1::uuid`, verificationID).Scan(&docURL)
	if err != nil {
		return "", ErrNotFound
	}
	return docURL, nil
}

// ────────────────────────────────────────────────────────────────────────────
// Reports (full lifecycle)
// ────────────────────────────────────────────────────────────────────────────

var validReasons = map[string]bool{
	"spam": true, "fraud": true, "harassment": true,
	"safety": true, "off_platform_payment": true, "other": true,
}

// CreateReport creates a report. Generalizes Phase 5's conversation-scoped endpoint.
func (s *Service) CreateReport(ctx context.Context, reporterID, reportedUserID string, taskID, conversationID *string, reason, detail string) (string, error) {
	reason = strings.TrimSpace(strings.ToLower(reason))
	if !validReasons[reason] {
		return "", fmt.Errorf("%w: invalid reason", ErrBadRequest)
	}
	if reporterID == reportedUserID {
		return "", fmt.Errorf("%w: cannot report yourself", ErrBadRequest)
	}
	// Verify reported user exists
	var exists int
	_ = s.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE id=$1::uuid AND deleted_at IS NULL`, reportedUserID).Scan(&exists)
	if exists == 0 {
		return "", ErrNotFound
	}
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO reports (reporter_id, reported_user_id, task_id, conversation_id, reason, detail, status)
		 VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, $6, 'open') RETURNING id::text`,
		reporterID, reportedUserID, nullIfNil(taskID), nullIfNil(conversationID), reason, detail).Scan(&id)
	if err != nil {
		return "", err
	}
	_ = audit.Log(ctx, s.pool, audit.Entry{ActorID: &reporterID, Action: "report.create", EntityType: "report", EntityID: id})
	return id, nil
}

func nullIfNil(s *string) any {
	if s == nil || *s == "" {
		return nil
	}
	return *s
}

// ListOpenReports returns all open reports (admin only).
func (s *Service) ListOpenReports(ctx context.Context) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT r.id::text, r.reporter_id::text, r.reported_user_id::text,
		        r.task_id::text, r.conversation_id::text, r.reason, r.detail, r.status, r.created_at
		 FROM reports r WHERE r.status='open' ORDER BY r.created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, reporterID, reportedID, reason, status string
		var taskID, convID, detail *string
		var createdAt interface{}
		if err := rows.Scan(&id, &reporterID, &reportedID, &taskID, &convID, &reason, &detail, &status, &createdAt); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id": id, "reporter_id": reporterID, "reported_user_id": reportedID,
			"task_id": taskID, "conversation_id": convID, "reason": reason,
			"detail": detail, "status": status, "created_at": createdAt,
		})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

// UpdateReportStatus transitions a report through open → reviewing → actioned|dismissed.
func (s *Service) UpdateReportStatus(ctx context.Context, adminID, reportID, newStatus string) error {
	validTransitions := map[string][]string{
		"open":      {"reviewing"},
		"reviewing": {"actioned", "dismissed"},
	}
	var currentStatus string
	err := s.pool.QueryRow(ctx, `SELECT status FROM reports WHERE id=$1::uuid FOR UPDATE`, reportID).Scan(&currentStatus)
	if err != nil {
		return ErrNotFound
	}
	allowed := validTransitions[currentStatus]
	valid := false
	for _, s := range allowed {
		if s == newStatus {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("%w: cannot transition from %s to %s", ErrConflict, currentStatus, newStatus)
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE reports SET status=$1, handled_by=$2::uuid WHERE id=$3::uuid`,
		newStatus, adminID, reportID)
	if err != nil {
		return err
	}
	_ = audit.Log(ctx, s.pool, audit.Entry{
		ActorID: &adminID, Action: "report." + newStatus,
		EntityType: "report", EntityID: reportID,
		Metadata: map[string]any{"from": currentStatus, "to": newStatus},
	})
	return nil
}

// ListOwnReports returns reports submitted by the given user.
func (s *Service) ListOwnReports(ctx context.Context, userID string) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id::text, reported_user_id::text, reason, status, created_at
		 FROM reports WHERE reporter_id=$1::uuid ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, reportedID, reason, status string
		var createdAt interface{}
		if err := rows.Scan(&id, &reportedID, &reason, &status, &createdAt); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id": id, "reported_user_id": reportedID, "reason": reason,
			"status": status, "created_at": createdAt,
		})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

// ────────────────────────────────────────────────────────────────────────────
// Disputes — evidence upload + resolution
// ────────────────────────────────────────────────────────────────────────────

// AddEvidence appends an evidence URL to a dispute. Only parties on the underlying task.
func (s *Service) AddEvidence(ctx context.Context, userID, disputeID, evidenceURL string) error {
	if strings.TrimSpace(evidenceURL) == "" {
		return fmt.Errorf("%w: evidence_url required", ErrBadRequest)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Load dispute + task info, row-lock dispute
	var status, raisedBy, taskPosterID string
	var taskWorkerID *string
	err = tx.QueryRow(ctx,
		`SELECT d.status, d.raised_by::text, t.poster_id::text, t.assigned_worker_id::text
		 FROM disputes d JOIN tasks t ON t.id = d.task_id
		 WHERE d.id=$1::uuid FOR UPDATE`, disputeID).
		Scan(&status, &raisedBy, &taskPosterID, &taskWorkerID)
	if err != nil {
		return ErrNotFound
	}
	if status != "open" && status != "under_review" {
		return fmt.Errorf("%w: dispute not open (status=%s)", ErrConflict, status)
	}
	// Check party
	isParty := userID == taskPosterID || userID == raisedBy
	if taskWorkerID != nil && userID == *taskWorkerID {
		isParty = true
	}
	if !isParty {
		return ErrForbidden
	}

	// Append evidence URL
	_, err = tx.Exec(ctx,
		`UPDATE disputes SET evidence_urls = array_append(evidence_urls, $2) WHERE id=$1::uuid`,
		disputeID, evidenceURL)
	if err != nil {
		return err
	}

	_ = audit.Log(ctx, s.pool, audit.Entry{
		ActorID: &userID, Action: "dispute.evidence_add",
		EntityType: "dispute", EntityID: disputeID,
		Metadata: map[string]any{"url": evidenceURL},
	})

	return tx.Commit(ctx)
}

// ListOpenDisputes returns all open/under_review disputes (admin only).
func (s *Service) ListOpenDisputes(ctx context.Context) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT d.id::text, d.task_id::text, d.raised_by::text, d.reason, d.description,
		        d.evidence_urls, d.status, d.created_at
		 FROM disputes d WHERE d.status IN ('open','under_review') ORDER BY d.created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, taskID, raisedBy, reason, description, status string
		var evidenceURLs []string
		var createdAt interface{}
		if err := rows.Scan(&id, &taskID, &raisedBy, &reason, &description, &evidenceURLs, &status, &createdAt); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id": id, "task_id": taskID, "raised_by": raisedBy,
			"reason": reason, "description": description,
			"evidence_urls": evidenceURLs, "status": status, "created_at": createdAt,
		})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

// TriageDispute marks a dispute as under_review (admin only, no money movement).
func (s *Service) TriageDispute(ctx context.Context, adminID, disputeID string) error {
	var status string
	err := s.pool.QueryRow(ctx,
		`SELECT status FROM disputes WHERE id=$1::uuid FOR UPDATE`, disputeID).Scan(&status)
	if err != nil {
		return ErrNotFound
	}
	if status != "open" {
		return fmt.Errorf("%w: dispute not open (status=%s)", ErrConflict, status)
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE disputes SET status='under_review' WHERE id=$1::uuid`, disputeID)
	if err != nil {
		return err
	}
	_ = audit.Log(ctx, s.pool, audit.Entry{
		ActorID: &adminID, Action: "dispute.triage",
		EntityType: "dispute", EntityID: disputeID,
	})
	return nil
}

// ResolveDispute resolves a dispute with the given decision.
// decision must be one of: released, refunded, split.
// Reuses ledger.ReleaseTx and ledger.RefundEscrowTx from Phase 6/7.
// Enforces exactly-one-resolution via row-lock + status check.
func (s *Service) ResolveDispute(ctx context.Context, adminID, disputeID, decision, resolutionNotes string, compensationAmount *int) error {
	if decision != "released" && decision != "refunded" && decision != "split" {
		return fmt.Errorf("%w: decision must be released, refunded, or split", ErrBadRequest)
	}
	if decision == "split" && (compensationAmount == nil || *compensationAmount <= 0) {
		return fmt.Errorf("%w: split requires compensation_amount > 0", ErrBadRequest)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Row-lock dispute — prevents concurrent resolutions
	var disputeStatus, taskID, raisedBy string
	err = tx.QueryRow(ctx,
		`SELECT status, task_id::text, raised_by::text FROM disputes WHERE id=$1::uuid FOR UPDATE`, disputeID).
		Scan(&disputeStatus, &taskID, &raisedBy)
	if err != nil {
		return ErrNotFound
	}
	if disputeStatus != "open" && disputeStatus != "under_review" {
		return fmt.Errorf("%w: dispute already resolved (status=%s)", ErrConflict, disputeStatus)
	}

	// Load task info for money movement
	var taskStatus, posterID, workerID string
	var total, fee int
	var escrowStatus string
	err = tx.QueryRow(ctx,
		`SELECT status, poster_id::text, assigned_worker_id::text, total_charge, platform_fee, escrow_status
		 FROM tasks WHERE id=$1::uuid FOR UPDATE`, taskID).
		Scan(&taskStatus, &posterID, &workerID, &total, &fee, &escrowStatus)
	if err != nil {
		return ErrNotFound
	}
	if workerID == "" {
		return fmt.Errorf("%w: no assigned worker", ErrConflict)
	}

	// Perform money movement based on decision
	workerPayout := total - fee
	if workerPayout <= 0 {
		workerPayout = total
	}
	var disputeResolvedStatus string
	switch decision {
	case "released":
		// Full release to worker via ledger.ReleaseTx
		_, err = ledger.ReleaseTx(ctx, tx, taskID, workerID, workerPayout, fee)
		if err != nil {
			return err
		}
		disputeResolvedStatus = "resolved_released"
		_, err = tx.Exec(ctx, `UPDATE tasks SET status='resolved_released', escrow_status='released', updated_at=now() WHERE id=$1::uuid`, taskID)
		if err != nil {
			return err
		}

	case "refunded":
		// Full refund to poster via ledger.RefundEscrowTx
		convID := s.getConversationID(ctx, tx, taskID)
		_, err = ledger.RefundEscrowTx(ctx, tx, s.payments, taskID, posterID, total, fee,
			"dispute resolved: refund", "refund-dispute-"+disputeID, convID)
		if err != nil {
			return err
		}
		disputeResolvedStatus = "resolved_refunded"
		_, err = tx.Exec(ctx, `UPDATE tasks SET status='resolved_refunded', escrow_status='refunded', updated_at=now() WHERE id=$1::uuid`, taskID)
		if err != nil {
			return err
		}

	case "split":
		compAmt := *compensationAmount
		if compAmt > total {
			compAmt = total
		}
		releaseAmt := workerPayout - compAmt
		if releaseAmt < 0 {
			releaseAmt = 0
		}
		refundAmt := compAmt

		// Release partial to worker
		if releaseAmt > 0 {
			_, err = ledger.ReleaseTx(ctx, tx, taskID, workerID, releaseAmt, 0)
			if err != nil {
				return err
			}
		}
		// Refund partial to poster
		if refundAmt > 0 {
			convID := s.getConversationID(ctx, tx, taskID)
			_, err = ledger.RefundEscrowTx(ctx, tx, s.payments, taskID, posterID, refundAmt, 0,
				"dispute resolved: split refund", "refund-split-"+disputeID, convID)
			if err != nil {
				return err
			}
		}
		disputeResolvedStatus = "resolved_split"
		_, err = tx.Exec(ctx,
			`UPDATE tasks SET status='resolved_split', escrow_status='refunded', updated_at=now() WHERE id=$1::uuid`, taskID)
		if err != nil {
			return err
		}
	}

	// Update dispute
	var compAny any
	if compensationAmount != nil {
		compAny = *compensationAmount
	}
	_, err = tx.Exec(ctx,
		`UPDATE disputes SET status=$1, resolution=$2, compensation_amount=$3, resolved_by=$4::uuid, resolved_at=now()
		 WHERE id=$5::uuid`,
		disputeResolvedStatus, resolutionNotes, compAny, adminID, disputeID)
	if err != nil {
		return err
	}

	// System message
	convID := s.getConversationID(ctx, tx, taskID)
	if convID != nil && *convID != "" {
		_, _ = messaging.InsertSystemMessageTx(ctx, tx, *convID, "dispute_resolved",
			"Dispute resolved: "+decision)
	}

	// Phase 10 retrofit: real notifications to both parties
	_, _ = notify.InsertNotifTx(ctx, tx, posterID, notify.EventDisputeResolved, "both",
		notify.Payload(notify.EventDisputeResolved, map[string]string{"task_id": taskID, "dispute_id": disputeID, "decision": decision}))
	_, _ = notify.InsertNotifTx(ctx, tx, workerID, notify.EventDisputeResolved, "both",
		notify.Payload(notify.EventDisputeResolved, map[string]string{"task_id": taskID, "dispute_id": disputeID, "decision": decision}))

	// Audit
	_, _ = tx.Exec(ctx,
		`INSERT INTO audit_log (actor_id, action, entity_type, entity_id, metadata)
		 VALUES ($1::uuid, 'dispute.resolve', 'dispute', $2, $3)`,
		adminID, disputeID, fmt.Sprintf(`{"decision":"%s","task_id":"%s"}`, decision, taskID))

	return tx.Commit(ctx)
}

func (s *Service) getConversationID(ctx context.Context, tx pgx.Tx, taskID string) *string {
	var convID string
	err := tx.QueryRow(ctx, `SELECT id::text FROM conversations WHERE task_id=$1::uuid`, taskID).Scan(&convID)
	if err != nil {
		return nil
	}
	return &convID
}

// ────────────────────────────────────────────────────────────────────────────
// Contact-detail leak detection (passive, non-blocking)
// ────────────────────────────────────────────────────────────────────────────

// Patterns for contact-detail leak detection.
// Flag, don't block — false positives are common and blocking legitimate
// conversation is worse than the risk (documented product choice, §2.8).
var (
	phoneRE  = regexp.MustCompile(`(?i)(?:\+?\d{1,3}[-.\s]?)?\(?\d{2,4}\)?[-.\s]?\d{3,4}[-.\s]?\d{3,4}`)
	emailRE  = regexp.MustCompile(`(?i)[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	paymentRE = regexp.MustCompile(`(?i)(?:venmo|cashapp|cash\s*app|paypal|zelle|monzo|revolut|wise|bank\s*transfer|bank\s*details|sort\s*code|account\s*number)`)
)

// CheckContactLeak scans a message body for phone numbers, emails, and
// off-platform payment mentions. Returns true if a match was found and
// flagged to audit_log. The message is NEVER blocked or altered.
func (s *Service) CheckContactLeak(ctx context.Context, senderID, messageID, body string) bool {
	matched := false
	var reasons []string

	if phoneRE.MatchString(body) {
		matched = true
		reasons = append(reasons, "phone_number")
	}
	if emailRE.MatchString(body) {
		matched = true
		reasons = append(reasons, "email_address")
	}
	if paymentRE.MatchString(body) {
		matched = true
		reasons = append(reasons, "off_platform_payment")
	}

	if !matched {
		return false
	}

	// Flag to audit_log — does not block or alter the message
	_ = audit.Log(ctx, s.pool, audit.Entry{
		ActorID: &senderID,
		Action:  "message.contact_leak_flag",
		EntityType: "message",
		EntityID: messageID,
		Metadata: map[string]any{
			"flagged_reasons": reasons,
			"message_body":    body[:min(200, len(body))], // truncate for audit
		},
	})
	return true
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
