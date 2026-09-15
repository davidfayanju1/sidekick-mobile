// Phase 3 Testing Gate — discovery feed + search (13 gates + self-audit).
// Tasks are isolated per test via unique categories; the shared dev DB
// holds rows from all phases, so assertions are scoped, never absolute.
package tests

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// ── helpers ────────────────────────────────────────────────────────────────

// seedPoster creates a funded-task-capable user.
func seedPoster(t *testing.T, h *harness, tag string) (string, string) {
	t.Helper()
	id, tok, _ := h.signup(t, uniq("p3"+tag)+"@example.com", "password123", "P3 "+tag)
	return id, tok
}

// geoInput builds a valid task input at the given coords/category/budget.
func geoInput(lat, lng float64, category string, budget int, timing string) map[string]any {
	in := validTaskInput()
	in["location_lat"], in["location_lng"] = lat, lng
	in["category"] = category
	in["budget"] = budget
	in["timing_type"] = timing
	if timing == "specific_date" {
		in["scheduled_for"] = "2030-01-01T10:00:00Z"
	}
	return in
}

// fundOpen creates + funds a task, returning its id.
func fundOpen(t *testing.T, h *harness, tok string, in map[string]any) string {
	t.Helper()
	id := createTask(t, h, tok, in)["id"].(string)
	rec := fundTask(t, h, tok, id, "p3-"+uniq("k"))
	require.Equal(t, 200, rec.Code, rec.Body.String())
	return id
}

func getFeed(t *testing.T, h *harness, tok, query string) (int, map[string]any) {
	t.Helper()
	rec := h.do("GET", "/feed"+query, tok, "", nil)
	var out map[string]any
	if rec.Code == 200 {
		out = decodeBody(t, rec)
	}
	return rec.Code, out
}

func getSearch(t *testing.T, h *harness, tok string, params map[string]string) (int, map[string]any) {
	t.Helper()
	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	rec := h.do("GET", "/search?"+q.Encode(), tok, "", nil)
	var out map[string]any
	if rec.Code == 200 {
		out = decodeBody(t, rec)
	}
	return rec.Code, out
}

func cardIDs(cards []any) []string {
	ids := make([]string, 0, len(cards))
	for _, c := range cards {
		ids = append(ids, c.(map[string]any)["id"].(string))
	}
	return ids
}

// ── Gate 1: geo sort + radius ──────────────────────────────────────────────

func TestP3_Gate1_FeedGeo(t *testing.T) {
	h := newHarness(t)
	_, tokP := seedPoster(t, h, "geo-p")
	_, tokS := seedPoster(t, h, "geo-s")
	cat := uniq("geocat")

	baseLat, baseLng := 51.5000, -0.1200
	near := fundOpen(t, h, tokP, geoInput(baseLat+0.005, baseLng, cat, 2000, "asap"))   // ~0.5km
	mid := fundOpen(t, h, tokP, geoInput(baseLat+0.02, baseLng, cat, 2000, "asap"))     // ~2.2km
	far := fundOpen(t, h, tokP, geoInput(baseLat+0.09, baseLng, cat, 2000, "asap"))     // ~10km
	_ = fundOpen(t, h, tokP, geoInput(baseLat+0.5, baseLng, cat, 2000, "asap"))         // ~55km, out

	code, out := getFeed(t, h, tokS, fmt.Sprintf("?lat=%f&lng=%f&radius_km=20&category=%s",
		baseLat, baseLng, cat))
	require.Equal(t, 200, code)
	ids := cardIDs(out["tasks"].([]any))
	require.Equal(t, []string{near, mid, far}, ids, "distance ASC")
	require.Nil(t, out["next_cursor"])
	require.Equal(t, float64(20), out["radius_km_applied"])

	// Tight radius drops far.
	_, out = getFeed(t, h, tokS, fmt.Sprintf("?lat=%f&lng=%f&radius_km=3&category=%s",
		baseLat, baseLng, cat))
	require.Equal(t, []string{near, mid}, cardIDs(out["tasks"].([]any)))

	// Same-spot tiebreak: newer first.
	tieCat := uniq("tiecat")
	a := fundOpen(t, h, tokP, geoInput(baseLat, baseLng, tieCat, 2000, "asap"))
	b := fundOpen(t, h, tokP, geoInput(baseLat, baseLng, tieCat, 2000, "asap"))
	_, out = getFeed(t, h, tokS, fmt.Sprintf("?lat=%f&lng=%f&radius_km=5&category=%s",
		baseLat, baseLng, tieCat))
	require.Equal(t, []string{b, a}, cardIDs(out["tasks"].([]any)), "recency tiebreak")
}

// ── Gate 2: filters narrow server-side ─────────────────────────────────────

func TestP3_Gate2_Filters(t *testing.T) {
	h := newHarness(t)
	_, tokP := seedPoster(t, h, "flt-p")
	_, tokS := seedPoster(t, h, "flt-s")
	cat := uniq("fltcat")

	fundOpen(t, h, tokP, geoInput(51.5, -0.12, cat, 1000, "asap"))
	fundOpen(t, h, tokP, geoInput(51.5, -0.12, cat, 9000, "specific_date"))
	fundOpen(t, h, tokP, geoInput(51.5, -0.12, "other-"+cat, 1000, "asap"))

	_, out := getFeed(t, h, tokS, "?category="+cat)
	require.Len(t, out["tasks"].([]any), 2)
	_, out = getFeed(t, h, tokS, "?category="+cat+"&min_budget=5000")
	require.Len(t, out["tasks"].([]any), 1)
	_, out = getFeed(t, h, tokS, "?category="+cat+"&max_budget=2000")
	require.Len(t, out["tasks"].([]any), 1)
	_, out = getFeed(t, h, tokS, "?category="+cat+"&timing=specific_date")
	got := out["tasks"].([]any)
	require.Len(t, got, 1)
	require.Equal(t, "specific_date", got[0].(map[string]any)["timing_type"])
	// Same filters apply to search.
	_, sout := getSearch(t, h, tokS, map[string]string{"q": "assembly", "category": cat})
	require.Len(t, sout["results"].([]any), 2)
	_, sout = getSearch(t, h, tokS, map[string]string{"q": "assembly", "category": cat, "timing": "asap"})
	require.Len(t, sout["results"].([]any), 1)
}

// ── Gate 3: search relevance ───────────────────────────────────────────────

func TestP3_Gate3_SearchRank(t *testing.T) {
	h := newHarness(t)
	_, tokP := seedPoster(t, h, "srk-p")
	_, tokS := seedPoster(t, h, "srk-s")
	cat := uniq("srkcat")

	inA := geoInput(51.5, -0.12, cat, 2000, "asap")
	inA["title"] = "Assemble my flat-pack wardrobe quickly"
	idA := fundOpen(t, h, tokP, inA)
	inB := geoInput(51.5, -0.12, cat, 2000, "asap")
	inB["title"] = "Help me move several heavy boxes"
	inB["description"] = "Moving flats this weekend, and one wardrobe needs assembly too. Van provided for the trip."
	idB := fundOpen(t, h, tokP, inB)
	inC := geoInput(51.5, -0.12, cat, 2000, "asap")
	inC["title"] = "Walk my dog in the park daily"
	inC["description"] = "Friendly spaniel needs a lunchtime walk around the neighbourhood park area."
	fundOpen(t, h, tokP, inC)

	code, out := getSearch(t, h, tokS, map[string]string{"q": "wardrobe", "category": cat})
	require.Equal(t, 200, code)
	ids := cardIDs(out["results"].([]any))
	require.Equal(t, []string{idA, idB}, ids, "title match outranks description-only")
	require.Empty(t, out["next_cursor"])

	code, _ = getSearch(t, h, tokS, map[string]string{"q": ""})
	require.Equal(t, 400, code, "empty query rejected")
}

// ── Gate 4: keyset pagination, no dup/gap under concurrent insert ──────────

func TestP3_Gate4_Pagination(t *testing.T) {
	h := newHarness(t)
	_, tokP := seedPoster(t, h, "pgn-p")
	_, tokS := seedPoster(t, h, "pgn-s")
	cat := uniq("pgncat")
	for i := 0; i < 5; i++ {
		fundOpen(t, h, tokP, geoInput(51.5, -0.12, cat, 2000, "asap"))
	}
	collect := func(cursor string) (ids []string, next string) {
		q := "?category=" + cat + "&limit=2"
		if cursor != "" {
			q += "&cursor=" + url.QueryEscape(cursor)
		}
		code, out := getFeed(t, h, tokS, q)
		require.Equal(t, 200, code)
		ids = cardIDs(out["tasks"].([]any))
		if out["next_cursor"] != nil {
			next = out["next_cursor"].(string)
		}
		return ids, next
	}
	p1, c1 := collect("")
	require.Len(t, p1, 2)
	// Concurrent insert between pages (newest — sits before the cursor).
	extra := fundOpen(t, h, tokP, geoInput(51.5, -0.12, cat, 2000, "asap"))
	p2, c2 := collect(c1)
	require.Len(t, p2, 2)
	p3, c3 := collect(c2)
	require.Empty(t, c3)
	seen := map[string]int{}
	for _, id := range append(append(p1, p2...), p3...) {
		seen[id]++
	}
	for id, n := range seen {
		require.Equal(t, 1, n, "duplicate %s across pages", id)
	}
	for _, id := range p1 {
		require.Contains(t, seen, id)
	}
	require.Len(t, seen, 5, "all pre-existing rows exactly once (newcomer %s may sit before cursor)", extra)
}

// ── Gate 5: limit capped ───────────────────────────────────────────────────

func TestP3_Gate5_LimitCap(t *testing.T) {
	h := newHarness(t)
	_, tokP := seedPoster(t, h, "lim-p")
	_, tokS := seedPoster(t, h, "lim-s")
	cat := uniq("limcat")
	for i := 0; i < 55; i++ {
		fundOpen(t, h, tokP, geoInput(51.5, -0.12, cat, 2000, "asap"))
	}
	code, out := getFeed(t, h, tokS, "?category="+cat+"&limit=10000")
	require.Equal(t, 200, code)
	require.Len(t, out["tasks"].([]any), 50, "limit capped at 50 server-side")
	require.NotNil(t, out["next_cursor"])
}

// ── Gate 6: no exact/phone/email anywhere in feed/search ───────────────────

func TestP3_Gate6_NoLeak(t *testing.T) {
	h := newHarness(t)
	_, tokP := seedPoster(t, h, "lek-p")
	_, tokS := seedPoster(t, h, "lek-s")
	cat := uniq("lekcat")
	fundOpen(t, h, tokP, geoInput(51.5, -0.12, cat, 2000, "asap"))

	for _, path := range []string{"?category=" + cat, "?lat=51.5&lng=-0.12&radius_km=10&category=" + cat} {
		rec := h.do("GET", "/feed"+path, tokS, "", nil)
		require.Equal(t, 200, rec.Code)
		body := rec.Body.String()
		for _, k := range []string{`"location_exact"`, `"phone"`, `"email"`, `"password`} {
			require.NotContains(t, body, k, "feed %s leaks %s", path, k)
		}
	}
	rec := h.do("GET", "/search?q=assembly&category="+cat, tokS, "", nil)
	require.Equal(t, 200, rec.Code)
	for _, k := range []string{`"location_exact"`, `"phone"`, `"email"`, `"password`} {
		require.NotContains(t, rec.Body.String(), k)
	}
}

// ── Gate 7: own tasks excluded ─────────────────────────────────────────────

func TestP3_Gate7_OwnExcluded(t *testing.T) {
	h := newHarness(t)
	_, tokP := seedPoster(t, h, "own-p")
	_, tokS := seedPoster(t, h, "own-s")
	cat := uniq("owncat")
	mine := fundOpen(t, h, tokP, geoInput(51.5, -0.12, cat, 2000, "asap"))

	_, out := getFeed(t, h, tokP, "?category="+cat)
	require.NotContains(t, cardIDs(out["tasks"].([]any)), mine)
	_, out = getFeed(t, h, tokS, "?category="+cat)
	require.Contains(t, cardIDs(out["tasks"].([]any)), mine)
}

// ── Gate 8: pending-offer tasks excluded ────────────────────────────────────

func TestP3_Gate8_OfferExcluded(t *testing.T) {
	h := newHarness(t)
	_, tokP := seedPoster(t, h, "ofr-p")
	idW, tokW := seedPoster(t, h, "ofr-w")
	_, tokO := seedPoster(t, h, "ofr-o")
	cat := uniq("ofrcat")
	taskID := fundOpen(t, h, tokP, geoInput(51.5, -0.12, cat, 2000, "asap"))

	ctx := context.Background()
	_, err := h.pool.Exec(ctx,
		`INSERT INTO offers (task_id, worker_id, amount, status) VALUES ($1::uuid, $2::uuid, 2000, 'pending')`,
		taskID, idW)
	require.NoError(t, err)

	_, out := getFeed(t, h, tokW, "?category="+cat)
	require.NotContains(t, cardIDs(out["tasks"].([]any)), taskID)
	_, out = getFeed(t, h, tokO, "?category="+cat)
	require.Contains(t, cardIDs(out["tasks"].([]any)), taskID)
	_, sout := getSearch(t, h, tokW, map[string]string{"q": "assembly", "category": cat})
	require.NotContains(t, cardIDs(sout["results"].([]any)), taskID)
}

// ── Gate 9: blocks hide either direction ────────────────────────────────────

func TestP3_Gate9_Blocks(t *testing.T) {
	h := newHarness(t)
	idP, tokP := seedPoster(t, h, "blk-p")
	idB, tokB := seedPoster(t, h, "blk-b")
	cat := uniq("blkcat")
	taskID := fundOpen(t, h, tokP, geoInput(51.5, -0.12, cat, 2000, "asap"))

	ctx := context.Background()
	_, err := h.pool.Exec(ctx, `INSERT INTO blocks (blocker_id, blocked_id) VALUES ($1::uuid, $2::uuid)`, idB, idP)
	require.NoError(t, err)
	_, out := getFeed(t, h, tokB, "?category="+cat)
	require.NotContains(t, cardIDs(out["tasks"].([]any)), taskID, "blocker hides blocked poster")

	_, err = h.pool.Exec(ctx, `DELETE FROM blocks WHERE blocker_id=$1::uuid AND blocked_id=$2::uuid`, idB, idP)
	require.NoError(t, err)
	_, err = h.pool.Exec(ctx, `INSERT INTO blocks (blocker_id, blocked_id) VALUES ($1::uuid, $2::uuid)`, idP, idB)
	require.NoError(t, err)
	_, out = getFeed(t, h, tokB, "?category="+cat)
	require.NotContains(t, cardIDs(out["tasks"].([]any)), taskID, "reverse direction also hides")
}

// ── Gate 10: suspended posters invisible ────────────────────────────────────

func TestP3_Gate10_Suspended(t *testing.T) {
	h := newHarness(t)
	_, tokS := seedPoster(t, h, "sus-s")
	cat := uniq("suscat")
	// Fund as the to-be-suspended poster.
	ctx := context.Background()
	var idP, tokSuspended string
	{
		rec := h.do("POST", "/auth/signup", "", "", map[string]any{
			"email": uniq("susp") + "@example.com", "password": "password123", "display_name": "Suspended Poster",
		})
		require.Equal(t, 201, rec.Code)
		out := decodeBody(t, rec)
		idP = out["user"].(map[string]any)["id"].(string)
		tokSuspended = out["access_token"].(string)
	}
	taskID := fundOpen(t, h, tokSuspended, geoInput(51.5, -0.12, cat, 2000, "asap"))
	_, err := h.pool.Exec(ctx, `UPDATE users SET suspended_at=now() WHERE id=$1::uuid`, idP)
	require.NoError(t, err)
	_, out := getFeed(t, h, tokS, "?category="+cat)
	require.NotContains(t, cardIDs(out["tasks"].([]any)), taskID)
	_, sout := getSearch(t, h, tokS, map[string]string{"q": "assembly", "category": cat})
	require.NotContains(t, cardIDs(sout["results"].([]any)), taskID)
}

// ── Gate 11: only open appears ──────────────────────────────────────────────

func TestP3_Gate11_OnlyOpen(t *testing.T) {
	h := newHarness(t)
	_, tokP := seedPoster(t, h, "stt-p")
	_, tokS := seedPoster(t, h, "stt-s")
	cat := uniq("sttcat")
	ctx := context.Background()

	statuses := map[string]string{
		"draft": "draft", "cancelled": "cancelled_by_poster", "expired": "expired",
		"assigned": "assigned", "completed": "completed",
	}
	ids := map[string]string{}
	for name, st := range statuses {
		in := geoInput(51.5, -0.12, cat, 2000, "asap")
		in["title"] = "Status probe task " + name + " for listing"
		id := createTask(t, h, tokP, in)["id"].(string)
		if st != "draft" {
			fundTask(t, h, tokP, id, "stt-"+uniq("k"))
		}
		_, err := h.pool.Exec(ctx, `UPDATE tasks SET status=$1 WHERE id=$2::uuid`, st, id)
		require.NoError(t, err)
		ids[name] = id
	}
	openID := fundOpen(t, h, tokP, geoInput(51.5, -0.12, cat, 2000, "asap"))

	_, out := getFeed(t, h, tokS, "?category="+cat)
	got := cardIDs(out["tasks"].([]any))
	require.Equal(t, []string{openID}, got)
	_, sout := getSearch(t, h, tokS, map[string]string{"q": "assembly", "category": cat})
	require.Equal(t, []string{openID}, cardIDs(sout["results"].([]any)))
}

// ── Gate 12: radius clamped ─────────────────────────────────────────────────

func TestP3_Gate12_RadiusClamp(t *testing.T) {
	h := newHarness(t)
	_, tokP := seedPoster(t, h, "rad-p")
	_, tokS := seedPoster(t, h, "rad-s")
	cat := uniq("radcat")
	fundOpen(t, h, tokP, geoInput(51.5, -0.12, cat, 2000, "asap"))

	code, out := getFeed(t, h, tokS, "?lat=51.5&lng=-0.12&radius_km=500&category="+cat)
	require.Equal(t, 200, code)
	require.Equal(t, float64(50), out["radius_km_applied"], "500km must be clamped to 50")
	code, _ = getFeed(t, h, tokS, "?lat=51.5&lng=-0.12&radius_km=-5&category="+cat)
	require.Equal(t, 400, code, "non-positive radius rejected")
}

// ── Gate 13: search rate limit ──────────────────────────────────────────────

func TestP3_Gate13_SearchRateLimit(t *testing.T) {
	h := newHarness(t)
	_, tok := seedPoster(t, h, "rl-s")
	limited := false
	for i := 0; i < 35; i++ {
		rec := h.do("GET", "/search?q=assembly", tok, "", nil)
		if rec.Code == 429 {
			limited = true
			break
		}
		require.Equal(t, 200, rec.Code)
	}
	require.True(t, limited, "hammering search must eventually 429")
}

// ── Self-audit: shared exclusion + shared projection + docs ─────────────────

func TestP3_SelfAudit_SharedRules(t *testing.T) {
	root, err := filepath.Abs("..")
	require.NoError(t, err)
	raw, err := os.ReadFile(filepath.Join(root, "internal", "discovery", "discovery.go"))
	require.NoError(t, err)
	// The offers exclusion join must exist exactly once (shared builder).
	require.Equal(t, 1, strings.Count(string(raw), "o.worker_id"),
		"exclusion join must live in the single shared builder")
	// Cards must wrap the Phase 2 split, never re-implement it.
	require.Contains(t, string(raw), "tasks.ProjectTask")
	hraw, err := os.ReadFile(filepath.Join(root, "internal", "httpapi", "discovery.go"))
	require.NoError(t, err)
	require.NotContains(t, string(hraw), "location_exact",
		"feed/search handlers must not touch location fields")
}

func TestP3_Docs_OpenAPIAndScalar(t *testing.T) {
	h := newHarness(t)
	rec := h.do("GET", "/openapi.yaml", "", "", nil)
	require.Equal(t, 200, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "yaml")
	var spec struct {
		OpenAPI string `yaml:"openapi"`
		Paths   map[string]map[string]struct {
			OperationID string `yaml:"operationId"`
		} `yaml:"paths"`
	}
	require.NoError(t, yaml.Unmarshal(rec.Body.Bytes(), &spec))
	require.True(t, strings.HasPrefix(spec.OpenAPI, "3."))
	for _, p := range []string{
		"/auth/signup", "/auth/signin", "/auth/oauth", "/auth/phone/send",
		"/auth/phone/verify", "/auth/refresh", "/me", "/users/{id}",
		"/tasks", "/tasks/{id}", "/tasks/{id}/fund", "/tasks/{id}/escrow",
		"/fees/quote", "/feed", "/search", "/docs", "/openapi.yaml",
	} {
		ops, ok := spec.Paths[p]
		require.True(t, ok, "spec must document %s", p)
		for method, op := range ops {
			require.NotEmpty(t, op.OperationID, "%s %s needs operationId", method, p)
		}
	}
	rec = h.do("GET", "/docs", "", "", nil)
	require.Equal(t, 200, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	require.Contains(t, strings.ToLower(rec.Body.String()), "scalar")
	require.Contains(t, rec.Body.String(), "/openapi.yaml")
}
