// Package discovery implements Phase 3: geo feed, filtered search, and
// full-text search over open tasks.
//
// Shared-rule guarantees (pinned by the Phase 3 self-audit test):
//   1. ONE exclusion clause builder (exclusions) serves feed AND search —
//      no hand-written copies that can drift.
//   2. Card field selection wraps tasks.ProjectTask (the Phase 2 split) —
//      the feed never re-implements location privacy.
//   3. RLS posture (§3): queries run server-side; requesters never read
//      others' blocks/offers rows directly — the exclusion join only tests
//      existence of rows involving the requester.
package discovery

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sidekick/backend/internal/tasks"
)

const (
	// MaxRadiusKm caps geo queries (§2.2). Larger requests are clamped,
	// never honored as-is; the applied radius is echoed back.
	MaxRadiusKm = 50
	// DefaultRadiusKm applies when lat/lng are given without a radius.
	DefaultRadiusKm = 25
	DefaultLimit    = 20
	MaxLimit        = 50
)

var (
	ErrBadRequest = errors.New("bad request")
	ErrRateLimited = errors.New("rate_limited")
)

// PosterCard is the only poster data a card may carry — never phone/email.
type PosterCard struct {
	RatingAvg          float64 `json:"rating_avg"`
	RatingCount        int     `json:"rating_count"`
	VerificationStatus string  `json:"verification_status"`
}

type Filters struct {
	Lat, Lng   *float64
	RadiusKm   float64
	Category   string
	MinBudget  *int
	MaxBudget  *int
	Timing     string
	Query      string // search only
	Cursor     string
	Limit      int
}

type Page struct {
	Cards         []map[string]any `json:"cards"`
	NextCursor    *string          `json:"next_cursor"`
	RadiusApplied *float64         `json:"radius_km_applied,omitempty"`
}

// exclusions is THE shared exclusion clause for feed and search.
// Removes: own tasks, pending-offer tasks (offers join — zero rows until
// Phase 4, which is fine), either-direction blocks, suspended/deleted
// posters, and anything not open. args numbering starts at nextArg.
func exclusions(requesterID string, nextArg int) (string, []any) {
	// Single join condition; referenced by both queries from this one string.
	frag := fmt.Sprintf(`t.poster_id <> $%d::uuid
	  AND t.status = 'open'
	  AND u.suspended_at IS NULL AND u.deleted_at IS NULL
	  AND NOT EXISTS (SELECT 1 FROM offers o
	    WHERE o.task_id = t.id AND o.worker_id = $%d::uuid AND o.status = 'pending')
	  AND NOT EXISTS (SELECT 1 FROM blocks b
	    WHERE (b.blocker_id = $%d::uuid AND b.blocked_id = t.poster_id)
	       OR (b.blocker_id = t.poster_id AND b.blocked_id = $%d::uuid))`,
		nextArg, nextArg, nextArg, nextArg)
	return frag, []any{requesterID}
}

// ProjectCard wraps the Phase 2 split rule, then reshapes to the card:
// id, title, budget, distance_km, timing, category, offer_count, poster,
// created_at, location_approx. Description/fees/ledger/coords are dropped;
// any exact-location keys ProjectTask granted pass through unchanged.
func ProjectCard(requesterID string, t *tasks.Task, poster PosterCard, distKm *float64) map[string]any {
	base := tasks.ProjectTask(requesterID, t, nil)
	card := map[string]any{
		"id": t.ID, "title": t.Title, "budget": t.Budget,
		"timing_type": t.TimingType, "scheduled_for": t.ScheduledFor,
		"flexible_from": t.FlexibleFrom, "flexible_to": t.FlexibleTo,
		"category": t.Category, "offer_count": t.OfferCount,
		"location_approx": base["location_approx"],
		"created_at":      base["created_at"],
		"poster":          poster,
	}
	if distKm != nil {
		card["distance_km"] = *distKm
	}
	for _, k := range []string{"location_exact", "location_exact_lat", "location_exact_lng"} {
		if v, ok := base[k]; ok {
			card[k] = v
		}
	}
	return card
}

// ── rate limiting ──────────────────────────────────────────────────────────

type Limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	hits   map[string][]time.Time
}

func NewLimiter(max int, window time.Duration) *Limiter {
	return &Limiter{max: max, window: window, hits: map[string][]time.Time{}}
}

func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cut := now.Add(-l.window)
	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.max {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}

// ── service ────────────────────────────────────────────────────────────────

type Service struct {
	pool        *pgxpool.Pool
	searchLimit *Limiter
	feedLimit   *Limiter
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{
		pool:        pool,
		searchLimit: NewLimiter(30, time.Minute), // scraping guard (§2.3)
		feedLimit:   NewLimiter(120, time.Minute),
	}
}

func normLimit(n int) int {
	if n <= 0 {
		return DefaultLimit
	}
	if n > MaxLimit {
		return MaxLimit // capped server-side, never honored as-is
	}
	return n
}

type feedCursor struct {
	DistM     float64 `json:"d"`
	CreatedAt string  `json:"c"`
	ID        string  `json:"i"`
}

type searchCursor struct {
	Score     float64 `json:"s"`
	CreatedAt string  `json:"c"`
	ID        string  `json:"i"`
}

func encodeCursor(v any) string {
	raw, _ := json.Marshal(v)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeFeedCursor(s string) (feedCursor, error) {
	var c feedCursor
	if s == "" {
		return c, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return c, fmt.Errorf("%w: bad cursor", ErrBadRequest)
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, fmt.Errorf("%w: bad cursor", ErrBadRequest)
	}
	return c, nil
}

func decodeSearchCursor(s string) (searchCursor, error) {
	var c searchCursor
	if s == "" {
		return c, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return c, fmt.Errorf("%w: bad cursor", ErrBadRequest)
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, fmt.Errorf("%w: bad cursor", ErrBadRequest)
	}
	return c, nil
}

// filterFrag appends category/budget/timing constraints (server-side only).
func filterFrag(f Filters, nextArg int, args []any) (string, []any) {
	var b strings.Builder
	if f.Category != "" {
		b.WriteString(fmt.Sprintf(" AND t.category = $%d", nextArg))
		args = append(args, f.Category)
		nextArg++
	}
	if f.MinBudget != nil {
		b.WriteString(fmt.Sprintf(" AND t.budget >= $%d", nextArg))
		args = append(args, *f.MinBudget)
		nextArg++
	}
	if f.MaxBudget != nil {
		b.WriteString(fmt.Sprintf(" AND t.budget <= $%d", nextArg))
		args = append(args, *f.MaxBudget)
		nextArg++
	}
	if f.Timing != "" {
		if f.Timing != "asap" && f.Timing != "specific_date" && f.Timing != "flexible_range" {
			// Invalid timing values match nothing rather than erroring the
			// whole feed — strict but non-breaking. (Empty filter = no filter
			// is handled by the caller passing "".)
			b.WriteString(" AND FALSE")
		} else {
			b.WriteString(fmt.Sprintf(" AND t.timing_type = $%d", nextArg))
			args = append(args, f.Timing)
			nextArg++
		}
	}
	return b.String(), args
}

// Feed returns open-task cards: distance ASC then recency (geo), or recency
// (no coords). Keyset cursors survive concurrent inserts without dup/gap.
func (s *Service) Feed(ctx context.Context, requesterID string, f Filters) (*Page, error) {
	if !s.feedLimit.Allow("feed:" + requesterID) {
		return nil, fmt.Errorf("%w: feed rate limit exceeded", ErrRateLimited)
	}
	limit := normLimit(f.Limit)
	geo := f.Lat != nil && f.Lng != nil

	// $1/$2 are lat/lng when geo; everything else numbers up from there.
	var args []any
	nextArg := 1
	var distSelect, distFilter string
	var radiusApplied *float64
	if geo {
		radius := f.RadiusKm
		if radius <= 0 {
			radius = DefaultRadiusKm
		}
		applied := radius
		if applied > MaxRadiusKm {
			applied = MaxRadiusKm // clamp, never honor as-is
		}
		radiusApplied = &applied
		args = append(args, *f.Lat, *f.Lng)
		nextArg = 3
		distExpr := `earth_distance(ll_to_earth($1, $2), ll_to_earth(t.location_lat, t.location_lng))`
		distSelect = `, ` + distExpr + ` AS dist_m`
		distFilter = ` AND t.location_lat IS NOT NULL AND t.location_lng IS NOT NULL AND ` +
			distExpr + fmt.Sprintf(` <= $%d`, nextArg)
		nextArg++
		_ = radius
		args = append(args, applied*1000)
	}

	excl, eargs := exclusions(requesterID, nextArg)
	args = append(args, eargs...)
	nextArg++
	frag, fargs := filterFrag(f, nextArg, args)
	args = fargs
	nextArg += strings.Count(frag, "$")

	selectCols := tasks.ColumnsPrefixed("t") +
		`, u.rating_avg, u.rating_count, u.verification_status`
	inner := `SELECT ` + selectCols + distSelect +
		` FROM tasks t JOIN users u ON u.id = t.poster_id WHERE ` + excl + frag + distFilter
	// dist_m is a SELECT alias — cursor predicates on it need the wrap.
	rowsSQL := inner
	if geo {
		rowsSQL = `SELECT * FROM (` + inner + `) ranked WHERE TRUE`
	}

	cur, err := decodeFeedCursor(f.Cursor)
	if err != nil {
		return nil, err
	}
	if f.Cursor != "" {
		if geo {
			rowsSQL += fmt.Sprintf(` AND (dist_m > $%d OR (dist_m = $%d AND (created_at < $%d OR (created_at = $%d AND id < $%d))))`,
				nextArg, nextArg, nextArg+1, nextArg+1, nextArg+2)
			args = append(args, cur.DistM, cur.CreatedAt, cur.ID)
		} else {
			rowsSQL += fmt.Sprintf(` AND (t.created_at < $%d OR (t.created_at = $%d AND t.id::text < $%d))`,
				nextArg, nextArg, nextArg+1)
			args = append(args, cur.CreatedAt, cur.ID)
		}
	}
	if geo {
		rowsSQL += ` ORDER BY dist_m ASC, created_at DESC, id DESC`
	} else {
		rowsSQL += ` ORDER BY t.created_at DESC, t.id::text DESC`
	}
	page, err := s.queryCards(ctx, requesterID, rowsSQL, args, limit, geo)
	if err != nil {
		return nil, err
	}
	page.RadiusApplied = radiusApplied
	return page, nil
}

// queryCards runs the assembled select, fetches limit+1 rows, and builds
// cards through ProjectCard with the keyset next_cursor. The cursor always
// encodes the last RETURNED row — never the lookahead.
func (s *Service) queryCards(ctx context.Context, requesterID, sql string, args []any, limit int, hasDist bool) (*Page, error) {
	sql += fmt.Sprintf(` LIMIT %d`, limit+1)
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type fetched struct {
		task    tasks.Task
		poster  PosterCard
		distKm  *float64
		distM   float64 // raw meters for the cursor (no km round-trip)
	}
	var all []fetched
	for rows.Next() {
		var t tasks.Task
		dest := t.ScanDest()
		var p PosterCard
		dest = append(dest, &p.RatingAvg, &p.RatingCount, &p.VerificationStatus)
		var f fetched
		if hasDist {
			var d float64
			dest = append(dest, &d)
			if err := rows.Scan(dest...); err != nil {
				return nil, err
			}
			km := d / 1000
			f = fetched{task: t, poster: p, distKm: &km, distM: d}
		} else {
			if err := rows.Scan(dest...); err != nil {
				return nil, err
			}
			f = fetched{task: t, poster: p}
		}
		all = append(all, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	page := &Page{Cards: []map[string]any{}}
	// Cursor meters: re-derive from the last returned card's distance_km.
	for i, f := range all {
		if i >= limit {
			break
		}
		page.Cards = append(page.Cards, ProjectCard(requesterID, &f.task, f.poster, f.distKm))
	}
	if len(all) > limit {
		last := all[limit-1]
		var cur string
		if hasDist && last.distKm != nil {
			cur = encodeCursor(feedCursor{DistM: last.distM,
				CreatedAt: last.task.CreatedAt.Format(time.RFC3339Nano), ID: last.task.ID})
		} else {
			cur = encodeCursor(feedCursor{CreatedAt: last.task.CreatedAt.Format(time.RFC3339Nano), ID: last.task.ID})
		}
		page.NextCursor = &cur
	}
	return page, nil
}

// Search runs FTS + trigram matching with relevance ranking, the same
// exclusions/filters/cards as the feed. Rate-limited per user.
func (s *Service) Search(ctx context.Context, requesterID string, f Filters) (*Page, error) {
	q := strings.TrimSpace(f.Query)
	if q == "" || len([]rune(q)) > 200 {
		return nil, fmt.Errorf("%w: query required, max 200 chars", ErrBadRequest)
	}
	if !s.searchLimit.Allow("search:" + requesterID) {
		return nil, fmt.Errorf("%w: search rate limit exceeded", ErrRateLimited)
	}
	limit := normLimit(f.Limit)

	excl, args := exclusions(requesterID, 2) // $1 reserved for query text
	nextArg := 3
	frag, args := filterFrag(f, nextArg, args)
	nextArg += strings.Count(frag, "$")
	args = append([]any{q}, args...)

	score := `(ts_rank(to_tsvector('english', t.title), plainto_tsquery('english', $1)) * 2
	  + ts_rank(to_tsvector('english', t.description), plainto_tsquery('english', $1))
	  + GREATEST(similarity(t.title, $1), similarity(t.description, $1)))`
	match := `(to_tsvector('english', t.title || ' ' || t.description) @@ plainto_tsquery('english', $1)
	  OR t.title % $1 OR t.description % $1)`
	inner := `SELECT ` + tasks.ColumnsPrefixed("t") +
		`, u.rating_avg, u.rating_count, u.verification_status, ` + score + ` AS score` +
		` FROM tasks t JOIN users u ON u.id = t.poster_id WHERE ` + excl + frag + ` AND ` + match
	// score is a SELECT alias — cursor predicates need the wrap.
	rowsSQL := `SELECT * FROM (` + inner + `) ranked WHERE TRUE`
	cur, err := decodeSearchCursor(f.Cursor)
	if err != nil {
		return nil, err
	}
	if f.Cursor != "" {
		rowsSQL += fmt.Sprintf(` AND (score < $%d OR (score = $%d AND (created_at < $%d OR (created_at = $%d AND id < $%d))))`,
			nextArg, nextArg, nextArg+1, nextArg+1, nextArg+2)
		args = append(args, cur.Score, cur.CreatedAt, cur.ID)
	}
	rowsSQL += ` ORDER BY score DESC, created_at DESC, id DESC`
	return s.querySearchCards(ctx, requesterID, rowsSQL, args, limit)
}

func (s *Service) querySearchCards(ctx context.Context, requesterID, sql string, args []any, limit int) (*Page, error) {
	sql += fmt.Sprintf(` LIMIT %d`, limit+1)
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	page := &Page{Cards: []map[string]any{}}
	var lastScore float64
	var lastCreated time.Time
	var lastID string
	n := 0
	for rows.Next() {
		var t tasks.Task
		dest := t.ScanDest()
		var p PosterCard
		var score float64
		dest = append(dest, &p.RatingAvg, &p.RatingCount, &p.VerificationStatus, &score)
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		lastScore, lastCreated, lastID = score, t.CreatedAt, t.ID
		if n < limit {
			page.Cards = append(page.Cards, ProjectCard(requesterID, &t, p, nil))
		}
		n++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if n > limit {
		cur := encodeCursor(searchCursor{Score: lastScore, CreatedAt: lastCreated.Format(time.RFC3339Nano), ID: lastID})
		page.NextCursor = &cur
	}
	return page, nil
}
