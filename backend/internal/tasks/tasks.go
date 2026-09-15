// Package tasks implements Phase 2: task creation, drafts, photos linkage,
// fee quotes, escrow-hold funding, edit and cancel.
//
// §26 decisions (placeholders in app_config, never hardcoded):
//   Q1 fee model  → fee_payer (poster|worker|split). Poster pays total =
//     budget+fee at hold; worker/split deduct the fee at release (Phase 7),
//     so the hold here is budget+fee for poster, budget otherwise. The
//     fee_payer column exists now so the model switch is config, not redesign.
//   Q5 budgets    → min_budget / max_budget (seeded 500/50000 = £5–£500).
//   Q3 provider   → payments.Adapter; mock/sandbox now, real swap-in later.
//   Q4 categories → free text ≤60 chars until the taxonomy is decided.
//
// THE central rule (§3.5): location_exact* leaves the server ONLY through
// ProjectTask, and only for poster-or-assigned-worker on assigned-or-later
// statuses. Every read endpoint must call ProjectTask — never hand-pick
// fields. The Phase 2 self-audit test enforces this by grepping handlers.
package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sidekick/backend/internal/audit"
	"github.com/sidekick/backend/internal/ledger"
	"github.com/sidekick/backend/internal/payments"
)

var (
	ErrBadRequest   = errors.New("bad request")
	ErrNotFound     = errors.New("not found")
	ErrForbidden    = errors.New("forbidden")
	ErrConflict     = errors.New("conflict")
	ErrPaymentFailed = errors.New("payment_failed")
)

// ExactVisibleStatuses: exact location is visible to poster/assigned worker
// only from assignment onward. Draft/open reveal approx to everyone.
var ExactVisibleStatuses = map[string]bool{
	"assigned": true, "completed_pending_confirmation": true, "completed": true,
	"disputed": true, "resolved_released": true, "resolved_refunded": true, "resolved_split": true,
}

type Task struct {
	ID                 string
	PosterID           string
	Title              string
	Description        string
	Category           string
	LocationApprox     string
	LocationLat        *float64
	LocationLng        *float64
	LocationExact      *string
	LocationExactLat   *float64
	LocationExactLng   *float64
	TimingType         string
	ScheduledFor       *time.Time
	FlexibleFrom       *string
	FlexibleTo         *string
	Budget             int
	PlatformFee        int
	TotalCharge        int
	FeePayer           string
	Status             string
	AssignedWorkerID   *string
	OfferCount         int
	ExpiresAt          *time.Time
	DraftPayload       *string
	EscrowStatus       string
	EscrowTxID         *string
	CreatedAt          time.Time
}

type Photo struct {
	ID   string `json:"id"`
	URL  string `json:"url"`
	Type string `json:"type"`
}

// Input for create/edit.
type Input struct {
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Category       string   `json:"category"`
	LocationApprox string   `json:"location_approx"`
	LocationLat    *float64 `json:"location_lat"`
	LocationLng    *float64 `json:"location_lng"`
	LocationExact  string   `json:"location_exact"`
	LocationExactLat *float64 `json:"location_exact_lat"`
	LocationExactLng *float64 `json:"location_exact_lng"`
	TimingType     string   `json:"timing_type"`
	ScheduledFor   *time.Time `json:"scheduled_for"`
	FlexibleFrom   *string  `json:"flexible_from"`
	FlexibleTo     *string  `json:"flexible_to"`
	Budget         int      `json:"budget"`
}

type Service struct {
	pool     *pgxpool.Pool
	payments payments.Adapter
}

func New(pool *pgxpool.Pool, pay payments.Adapter) *Service {
	return &Service{pool: pool, payments: pay}
}

const taskCols = `id::text, poster_id::text, title, description, category,
	location_approx, location_lat, location_lng, location_exact,
	location_exact_lat, location_exact_lng, timing_type, scheduled_for,
	flexible_from::text, flexible_to::text, budget, platform_fee, total_charge,
	fee_payer, status, assigned_worker_id::text, offer_count, expires_at,
	draft_payload::text, escrow_status, escrow_transaction_id::text, created_at`

// Columns exposes the canonical task select list so other packages
// (discovery joins) reuse it instead of duplicating field selection.
func Columns() string { return taskCols }

// ColumnsPrefixed is Columns with every item qualified by the table alias,
// for join queries where bare names would be ambiguous.
func ColumnsPrefixed(alias string) string {
	parts := strings.Split(taskCols, ",")
	for i := range parts {
		parts[i] = alias + "." + strings.TrimSpace(parts[i])
	}
	return strings.Join(parts, ", ")
}

// ScanDest returns scan targets in Columns() order. Join queries select
// these first, then append their own extra columns.
func (t *Task) ScanDest() []any {
	return []any{&t.ID, &t.PosterID, &t.Title, &t.Description, &t.Category,
		&t.LocationApprox, &t.LocationLat, &t.LocationLng, &t.LocationExact,
		&t.LocationExactLat, &t.LocationExactLng, &t.TimingType, &t.ScheduledFor,
		&t.FlexibleFrom, &t.FlexibleTo, &t.Budget, &t.PlatformFee, &t.TotalCharge,
		&t.FeePayer, &t.Status, &t.AssignedWorkerID, &t.OfferCount, &t.ExpiresAt,
		&t.DraftPayload, &t.EscrowStatus, &t.EscrowTxID, &t.CreatedAt}
}

func scanTask(r pgx.Row) (*Task, error) {
	var t Task
	err := r.Scan(&t.ID, &t.PosterID, &t.Title, &t.Description, &t.Category,
		&t.LocationApprox, &t.LocationLat, &t.LocationLng, &t.LocationExact,
		&t.LocationExactLat, &t.LocationExactLng, &t.TimingType, &t.ScheduledFor,
		&t.FlexibleFrom, &t.FlexibleTo, &t.Budget, &t.PlatformFee, &t.TotalCharge,
		&t.FeePayer, &t.Status, &t.AssignedWorkerID, &t.OfferCount, &t.ExpiresAt,
		&t.DraftPayload, &t.EscrowStatus, &t.EscrowTxID, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Service) cfgInt(ctx context.Context, key string, fallback int) int {
	var raw string
	if err := s.pool.QueryRow(ctx, `SELECT value::text FROM app_config WHERE key=$1`, key).Scan(&raw); err != nil {
		return fallback
	}
	n, err := strconv.Atoi(strings.Trim(raw, `"`))
	if err != nil {
		return fallback
	}
	return n
}

func (s *Service) cfgStr(ctx context.Context, key, fallback string) string {
	var raw string
	if err := s.pool.QueryRow(ctx, `SELECT value::text FROM app_config WHERE key=$1`, key).Scan(&raw); err != nil {
		return fallback
	}
	return strings.Trim(raw, `"`)
}

func validateInput(in Input, minBudget, maxBudget int) error {
	if l := len([]rune(strings.TrimSpace(in.Title))); l < 10 || l > 80 {
		return fmt.Errorf("%w: title must be 10-80 chars", ErrBadRequest)
	}
	if l := len([]rune(strings.TrimSpace(in.Description))); l < 20 || l > 2000 {
		return fmt.Errorf("%w: description must be 20-2000 chars", ErrBadRequest)
	}
	if strings.TrimSpace(in.Category) == "" || len([]rune(in.Category)) > 60 {
		return fmt.Errorf("%w: category required, max 60 chars", ErrBadRequest)
	}
	if strings.TrimSpace(in.LocationApprox) == "" {
		return fmt.Errorf("%w: location_approx required", ErrBadRequest)
	}
	if strings.TrimSpace(in.LocationExact) == "" {
		return fmt.Errorf("%w: location_exact required", ErrBadRequest)
	}
	if (in.LocationLat == nil) != (in.LocationLng == nil) {
		return fmt.Errorf("%w: location_lat/lng must be set together", ErrBadRequest)
	}
	switch in.TimingType {
	case "asap":
		if in.ScheduledFor != nil {
			return fmt.Errorf("%w: asap takes no scheduled_for", ErrBadRequest)
		}
	case "specific_date":
		if in.ScheduledFor == nil || !in.ScheduledFor.After(time.Now()) {
			return fmt.Errorf("%w: specific_date needs a future scheduled_for", ErrBadRequest)
		}
	case "flexible_range":
		if in.FlexibleFrom == nil || in.FlexibleTo == nil {
			return fmt.Errorf("%w: flexible_range needs from and to", ErrBadRequest)
		}
		if *in.FlexibleFrom > *in.FlexibleTo {
			return fmt.Errorf("%w: flexible from must be <= to", ErrBadRequest)
		}
		if *in.FlexibleTo < time.Now().Format("2006-01-02") {
			return fmt.Errorf("%w: flexible to must be >= today", ErrBadRequest)
		}
	default:
		return fmt.Errorf("%w: timing_type must be asap, specific_date or flexible_range", ErrBadRequest)
	}
	if in.Budget < minBudget || in.Budget > maxBudget {
		return fmt.Errorf("%w: budget must be between %d and %d", ErrBadRequest, minBudget, maxBudget)
	}
	return nil
}

// ── creation (draft) ───────────────────────────────────────────────────────

func (s *Service) Create(ctx context.Context, posterID string, in Input) (*Task, error) {
	minB := s.cfgInt(ctx, "min_budget", 500)
	maxB := s.cfgInt(ctx, "max_budget", 50000)
	if err := validateInput(in, minB, maxB); err != nil {
		return nil, err
	}
	t, err := scanTask(s.pool.QueryRow(ctx,
		`INSERT INTO tasks (poster_id, title, description, category, location_approx,
		  location_lat, location_lng, location_exact, location_exact_lat, location_exact_lng,
		  timing_type, scheduled_for, flexible_from, flexible_to, budget)
		 VALUES ($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,
		   NULLIF($13,'')::date, NULLIF($14,'')::date,$15)
		 RETURNING `+taskCols,
		posterID, strings.TrimSpace(in.Title), strings.TrimSpace(in.Description),
		strings.TrimSpace(in.Category), strings.TrimSpace(in.LocationApprox),
		in.LocationLat, in.LocationLng, strings.TrimSpace(in.LocationExact),
		in.LocationExactLat, in.LocationExactLng, in.TimingType, in.ScheduledFor,
		strOrEmpty(in.FlexibleFrom), strOrEmpty(in.FlexibleTo), in.Budget))
	if err != nil {
		return nil, err
	}
	_ = audit.Log(ctx, s.pool, audit.Entry{ActorID: &posterID, Action: "task.create",
		EntityType: "task", EntityID: t.ID})
	return t, nil
}

func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ── drafts ─────────────────────────────────────────────────────────────────

func (s *Service) SaveDraft(ctx context.Context, posterID, taskID string, payload string) error {
	// Incomplete allowed: only ownership + status gate, no field validation.
	ct, err := s.pool.Exec(ctx,
		`UPDATE tasks SET draft_payload=$1::jsonb, updated_at=now()
		  WHERE id=$2::uuid AND poster_id=$3::uuid AND status='draft'`,
		payload, taskID, posterID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("%w: draft not found or not editable", ErrNotFound)
	}
	return nil
}

func (s *Service) ListDrafts(ctx context.Context, posterID string) ([]*Task, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+taskCols+` FROM tasks WHERE poster_id=$1::uuid AND status='draft' ORDER BY created_at DESC`, posterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ── THE projection: every read goes through here ───────────────────────────

// canSeeExact implements §3.5 as clarified by the Phase 2 gate 12 note:
// the poster ALWAYS sees their own exact location (it is their own address,
// needed to edit/resume drafts and manage open tasks); the assigned worker
// sees it only from assignment onward. Strangers never see it, in any status.
func canSeeExact(requesterID string, t *Task) bool {
	if requesterID == t.PosterID {
		return true
	}
	if !ExactVisibleStatuses[t.Status] {
		return false
	}
	return t.AssignedWorkerID != nil && requesterID == *t.AssignedWorkerID
}

// ProjectTask is the SINGLE function that renders a task for any requester.
// Handlers must never hand-pick location fields.
// Split rule (see canSeeExact): poster always sees exact; assigned worker
// from assignment on; strangers never. Draft payloads surface to the poster
// only, so resume never leaks through another reader.
func ProjectTask(requesterID string, t *Task, photos []Photo) map[string]any {
	out := map[string]any{
		"id": t.ID, "poster_id": t.PosterID, "title": t.Title,
		"description": t.Description, "category": t.Category,
		"location_approx": t.LocationApprox,
		"location_lat": t.LocationLat, "location_lng": t.LocationLng,
		"timing_type": t.TimingType, "scheduled_for": t.ScheduledFor,
		"flexible_from": t.FlexibleFrom, "flexible_to": t.FlexibleTo,
		"budget": t.Budget, "platform_fee": t.PlatformFee, "total_charge": t.TotalCharge,
		"status": t.Status, "offer_count": t.OfferCount,
		"escrow_status": t.EscrowStatus, "created_at": t.CreatedAt,
		"photos": photos,
	}
	if canSeeExact(requesterID, t) {
		out["location_exact"] = t.LocationExact
		out["location_exact_lat"] = t.LocationExactLat
		out["location_exact_lng"] = t.LocationExactLng
	}
	if t.Status == "draft" && requesterID == t.PosterID && t.DraftPayload != nil {
		var obj any
		if err := json.Unmarshal([]byte(*t.DraftPayload), &obj); err == nil {
			out["draft_payload"] = obj // rendered as an object, never double-encoded
		}
	}
	return out
}

// GetForReader loads a task with read visibility: drafts visible to poster
// only (404 otherwise — no enumeration); others via the projection.
func (s *Service) GetForReader(ctx context.Context, requesterID, taskID string) (*Task, []Photo, error) {
	t, err := scanTask(s.pool.QueryRow(ctx, `SELECT `+taskCols+` FROM tasks WHERE id=$1::uuid`, taskID))
	if err != nil {
		return nil, nil, ErrNotFound
	}
	if t.Status == "draft" && requesterID != t.PosterID {
		return nil, nil, ErrNotFound
	}
	photos, _ := s.ListPhotos(ctx, taskID)
	return t, photos, nil
}

// ── photos linkage ─────────────────────────────────────────────────────────

func (s *Service) ListPhotos(ctx context.Context, taskID string) ([]Photo, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id::text, url, type FROM task_photos WHERE task_id=$1::uuid ORDER BY created_at`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Photo
	for rows.Next() {
		var p Photo
		if err := rows.Scan(&p.ID, &p.URL, &p.Type); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if out == nil {
		out = []Photo{}
	}
	return out, rows.Err()
}

func (s *Service) AttachPhoto(ctx context.Context, posterID, taskID, url, typ string) (*Photo, error) {
	var p Photo
	err := s.pool.QueryRow(ctx,
		`WITH parent AS (SELECT poster_id, status FROM tasks WHERE id=$1::uuid),
		 cnt AS (SELECT count(*) AS n FROM task_photos WHERE task_id=$1::uuid AND type='listing')
		 INSERT INTO task_photos (task_id, url, type, uploaded_by)
		 SELECT $1::uuid, $2, $3, $4::uuid FROM parent JOIN cnt ON TRUE
		 WHERE parent.poster_id=$4::uuid AND parent.status IN ('draft','open')
		   AND (($3='listing' AND cnt.n < 5) OR $3='completion')
		 RETURNING id::text, url, type`, taskID, url, typ, posterID).
		Scan(&p.ID, &p.URL, &p.Type)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("%w: cannot attach photo (owner, status or 5-photo cap)", ErrForbidden)
		}
		return nil, err
	}
	return &p, nil
}

// ── fee quote (pure) ───────────────────────────────────────────────────────

type Quote struct {
	Budget   int    `json:"budget"`
	Fee      int    `json:"fee"`
	Total    int    `json:"total"`
	FeePayer string `json:"fee_payer"`
}

func (s *Service) Quote(ctx context.Context, budget int) (Quote, error) {
	minB := s.cfgInt(ctx, "min_budget", 500)
	maxB := s.cfgInt(ctx, "max_budget", 50000)
	if budget < minB || budget > maxB {
		return Quote{}, fmt.Errorf("%w: budget must be between %d and %d", ErrBadRequest, minB, maxB)
	}
	pct := s.cfgInt(ctx, "fee_percent", 8)
	payer := s.cfgStr(ctx, "fee_payer", "poster")
	fee := budget * pct / 100
	total := budget
	if payer == "poster" {
		total = budget + fee
	}
	return Quote{Budget: budget, Fee: fee, Total: total, FeePayer: payer}, nil
}

// ── funding (escrow-hold slice) ────────────────────────────────────────────

type FundResult struct {
	TaskID        string `json:"task_id"`
	Status        string `json:"status"`
	EscrowStatus  string `json:"escrow_status"`
	TransactionID string `json:"transaction_id"`
	Repeated      bool   `json:"repeated"`
}

// Fund moves draft→open with a provider hold. Atomic: hold + ledger row +
// task update + idempotency record commit together — never half-funded.
// Failure leaves draft + draft_payload intact with a retryable error.
func (s *Service) Fund(ctx context.Context, posterID, taskID, paymentRef, idemKey string) (*FundResult, error) {
	if idemKey == "" {
		return nil, fmt.Errorf("%w: idempotency key required", ErrBadRequest)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Replay: same key returns the original result, no second charge.
	var storedCode int
	var storedBody []byte
	err = tx.QueryRow(ctx, `SELECT response_code, response_body FROM idempotency_keys WHERE key=$1`, idemKey).
		Scan(&storedCode, &storedBody)
	if err == nil {
		_ = tx.Rollback(ctx)
		var prev FundResult
		_ = json.Unmarshal(storedBody, &prev)
		prev.Repeated = true
		return &prev, nil
	}

	var t Task
	err = tx.QueryRow(ctx, `SELECT `+taskCols+` FROM tasks WHERE id=$1::uuid FOR UPDATE`, taskID).
		Scan(t.ScanDest()...)
	if err != nil {
		return nil, ErrNotFound
	}
	if t.PosterID != posterID {
		return nil, ErrForbidden
	}
	if t.Status != "draft" {
		return nil, fmt.Errorf("%w: only draft tasks can be funded (status %s)", ErrConflict, t.Status)
	}

	q, err := s.quoteTx(ctx, tx, t.Budget)
	if err != nil {
		return nil, err
	}
	hold, err := s.payments.Hold(ctx, payments.Request{
		IdempotencyKey: idemKey, TaskID: taskID,
		Amount: int64(q.Total), PlatformFee: int64(q.Fee), FeePayer: q.FeePayer,
		ProviderRef: paymentRef,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPaymentFailed, err)
	}
	var txID string
	err = tx.QueryRow(ctx,
		`INSERT INTO transactions (task_id, payer_id, amount, platform_fee, fee_payer,
		  type, status, provider_ref, idempotency_key)
		 VALUES ($1::uuid,$2::uuid,$3,$4,$5,'escrow_hold','secured',$6,$7)
		 RETURNING id::text`,
		taskID, posterID, q.Total, q.Fee, q.FeePayer, hold.ProviderRef, idemKey).Scan(&txID)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE tasks SET status='open', escrow_status='secured', escrow_transaction_id=$1::uuid,
		  platform_fee=$2, total_charge=$3, fee_payer=$4, updated_at=now() WHERE id=$5::uuid`,
		txID, q.Fee, q.Total, q.FeePayer, taskID); err != nil {
		return nil, err
	}
	res := &FundResult{TaskID: taskID, Status: "open", EscrowStatus: "secured", TransactionID: txID}
	body, _ := json.Marshal(res)
	if _, err := tx.Exec(ctx,
		`INSERT INTO idempotency_keys (key, operation, user_id, response_code, response_body)
		 VALUES ($1,'task.fund',$2::uuid,200,$3) ON CONFLICT (key) DO NOTHING`,
		idemKey, posterID, string(body)); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	_ = audit.Log(ctx, s.pool, audit.Entry{ActorID: &posterID, Action: "task.fund",
		EntityType: "task", EntityID: taskID,
		Metadata: map[string]any{"transaction_id": txID, "total": q.Total}})
	return res, nil
}

// ── edit ───────────────────────────────────────────────────────────────────

func (s *Service) Edit(ctx context.Context, posterID, taskID string, in Input) (*Task, error) {
	t, err := scanTask(s.pool.QueryRow(ctx, `SELECT `+taskCols+` FROM tasks WHERE id=$1::uuid`, taskID))
	if err != nil {
		return nil, ErrNotFound
	}
	if t.PosterID != posterID {
		return nil, ErrForbidden
	}
	editable := t.Status == "draft" ||
		(t.Status == "open" && t.OfferCount == 0)
	if !editable {
		return nil, fmt.Errorf("%w: task not editable in status %s (offers %d)", ErrConflict, t.Status, t.OfferCount)
	}
	minB := s.cfgInt(ctx, "min_budget", 500)
	maxB := s.cfgInt(ctx, "max_budget", 50000)
	if err := validateInput(in, minB, maxB); err != nil {
		return nil, err
	}
	if t.EscrowStatus == "secured" && in.Budget != t.Budget {
		return nil, fmt.Errorf("%w: budget locked after funding — cancel and repost", ErrConflict)
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE tasks SET title=$1, description=$2, category=$3, location_approx=$4,
		  location_lat=$5, location_lng=$6, location_exact=$7, location_exact_lat=$8,
		  location_exact_lng=$9, timing_type=$10, scheduled_for=$11,
		  flexible_from=NULLIF($12,'')::date, flexible_to=NULLIF($13,'')::date,
		  budget=$14, updated_at=now() WHERE id=$15::uuid`,
		strings.TrimSpace(in.Title), strings.TrimSpace(in.Description), strings.TrimSpace(in.Category),
		strings.TrimSpace(in.LocationApprox), in.LocationLat, in.LocationLng,
		strings.TrimSpace(in.LocationExact), in.LocationExactLat, in.LocationExactLng,
		in.TimingType, in.ScheduledFor, strOrEmpty(in.FlexibleFrom), strOrEmpty(in.FlexibleTo),
		in.Budget, taskID)
	if err != nil {
		return nil, err
	}
	_ = audit.Log(ctx, s.pool, audit.Entry{ActorID: &posterID, Action: "task.edit",
		EntityType: "task", EntityID: taskID})
	return scanTask(s.pool.QueryRow(ctx, `SELECT `+taskCols+` FROM tasks WHERE id=$1::uuid`, taskID))
}

// ── cancel (narrow: full refund of unaccepted funded task) ──────────────────

func (s *Service) Cancel(ctx context.Context, posterID, taskID string) (*Task, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var t Task
	err = tx.QueryRow(ctx, `SELECT `+taskCols+` FROM tasks WHERE id=$1::uuid FOR UPDATE`, taskID).
		Scan(t.ScanDest()...)
	if err != nil {
		return nil, ErrNotFound
	}
	if t.PosterID != posterID {
		return nil, ErrForbidden
	}
	if t.Status != "draft" && t.Status != "open" {
		return nil, fmt.Errorf("%w: cannot cancel task in status %s", ErrConflict, t.Status)
	}
	if t.AssignedWorkerID != nil {
		return nil, fmt.Errorf("%w: task already accepted", ErrConflict)
	}
	if t.EscrowStatus == "secured" && t.EscrowTxID != nil {
		var convID *string
		// open tasks have no conversation, so convID remains nil (ledger will skip system msg)
		_ = tx.QueryRow(ctx, `SELECT id::text FROM conversations WHERE task_id=$1::uuid`, taskID).Scan(&convID)
		// refund_escrow is the single implementation; keep amount = total_charge
		if _, err := ledger.RefundEscrowTx(ctx, tx, s.payments, taskID, posterID, t.TotalCharge, t.PlatformFee, "cancel open task", "refund-"+taskID, convID); err != nil {
			return nil, fmt.Errorf("%w: refund failed: %v", ErrPaymentFailed, err)
		}
	}
	if _, err := tx.Exec(ctx,
		`UPDATE tasks SET status='cancelled_by_poster', cancelled_by=$1::uuid,
		  escrow_status=CASE WHEN escrow_status='secured' THEN 'refunded' ELSE escrow_status END,
		  updated_at=now() WHERE id=$2::uuid`, posterID, taskID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	_ = audit.Log(ctx, s.pool, audit.Entry{ActorID: &posterID, Action: "task.cancel",
		EntityType: "task", EntityID: taskID})
	return scanTask(s.pool.QueryRow(ctx, `SELECT `+taskCols+` FROM tasks WHERE id=$1::uuid`, taskID))
}

// ── escrow read (poster only in this phase; masked ref) ────────────────────

type EscrowView struct {
	Status      string `json:"status"`
	Amount      int    `json:"amount"`
	Fee         int    `json:"fee"`
	ProviderRef string `json:"provider_ref_masked"`
}

func maskRef(ref string) string {
	if len(ref) <= 4 {
		return "••••"
	}
	return "…" + ref[len(ref)-4:]
}

func (s *Service) EscrowView(ctx context.Context, posterID, taskID string) (*EscrowView, error) {
	t, err := scanTask(s.pool.QueryRow(ctx, `SELECT `+taskCols+` FROM tasks WHERE id=$1::uuid`, taskID))
	if err != nil {
		return nil, ErrNotFound
	}
	if t.PosterID != posterID {
		return nil, ErrForbidden
	}
	var ref *string
	_ = s.pool.QueryRow(ctx, `SELECT provider_ref FROM transactions WHERE id=$1::uuid`,
		nullStr(t.EscrowTxID)).Scan(&ref)
	masked := "••••"
	if ref != nil {
		masked = maskRef(*ref)
	}
	return &EscrowView{Status: t.EscrowStatus, Amount: t.TotalCharge, Fee: t.PlatformFee, ProviderRef: masked}, nil
}

func nullStr(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

// ── internals ──────────────────────────────────────────────────────────────

func (s *Service) quoteTx(ctx context.Context, tx pgx.Tx, budget int) (Quote, error) {
	var feePct, minB, maxB int
	var payer string
	_ = tx.QueryRow(ctx, `SELECT value::int FROM app_config WHERE key='fee_percent'`).Scan(&feePct)
	_ = tx.QueryRow(ctx, `SELECT value::int FROM app_config WHERE key='min_budget'`).Scan(&minB)
	_ = tx.QueryRow(ctx, `SELECT value::int FROM app_config WHERE key='max_budget'`).Scan(&maxB)
	_ = tx.QueryRow(ctx, `SELECT trim(both '"' from value::text) FROM app_config WHERE key='fee_payer'`).Scan(&payer)
	if feePct == 0 {
		feePct = 8
	}
	if minB == 0 {
		minB = 500
	}
	if maxB == 0 {
		maxB = 50000
	}
	if payer == "" {
		payer = "poster"
	}
	if budget < minB || budget > maxB {
		return Quote{}, fmt.Errorf("%w: budget out of range", ErrBadRequest)
	}
	fee := budget * feePct / 100
	total := budget
	if payer == "poster" {
		total = budget + fee
	}
	return Quote{Budget: budget, Fee: fee, Total: total, FeePayer: payer}, nil
}


