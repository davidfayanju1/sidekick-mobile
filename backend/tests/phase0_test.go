// Phase 0 Testing Gate — one test per gate item in phase-0-foundations.md.
// Pure-Go translation: Postgres is reachable ONLY via connection string
// (no Supabase Anon/Service keys exist). "Anonymous / fresh-authenticated
// with no policies" maps to a restricted Postgres role with grants but
// zero RLS policies: every query must return zero rows (RLS deny).
//
// Requires a Postgres database. Resolution order:
//   TEST_DATABASE_URL > DATABASE_URL_DEV > localhost:5433/sidekick_dev
package tests

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/sidekick/backend/internal/appconfig"
	"github.com/sidekick/backend/internal/audit"
	"github.com/sidekick/backend/internal/auth"
	"github.com/sidekick/backend/internal/config"
	"github.com/sidekick/backend/internal/db"
	"github.com/sidekick/backend/internal/idempotency"
	"github.com/sidekick/backend/internal/payments"
	"github.com/sidekick/backend/internal/scheduler"
	"github.com/sidekick/backend/internal/storage"
)

var allTables = []string{
	"users", "tasks", "task_photos", "offers", "conversations", "messages",
	"transactions", "wallets", "payout_methods", "payment_methods", "reviews",
	"verifications", "reports", "blocks", "disputes", "notifications",
	"push_tokens", "payout_batches", "payout_items", "audit_log", "categories",
	"app_config", "idempotency_keys", "sessions", "phone_verifications",
}

func testDatabaseURL(t *testing.T) string {
	t.Helper()
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	if v := os.Getenv("DATABASE_URL_DEV"); v != "" {
		return v
	}
	return "postgres://postgres:postgres@localhost:5433/sidekick_dev?sslmode=disable"
}

func setupPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.Connect(ctx, testDatabaseURL(t))
	require.NoError(t, err, "connect to test database (start sidekick-pg or set TEST_DATABASE_URL)")
	require.NoError(t, db.Migrate(ctx, pool), "apply migrations")
	t.Cleanup(pool.Close)
	return pool
}

// withRole runs fn on a dedicated connection acting as role.
func withRole(ctx context.Context, pool *pgxpool.Pool, t *testing.T, role string, fn func(ctx context.Context, conn *pgxpool.Conn)) {
	t.Helper()
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, fmt.Sprintf("SET ROLE %s", role))
	require.NoError(t, err)
	defer func() {
		_, _ = conn.Exec(context.Background(), "RESET ROLE")
	}()
	fn(ctx, conn)
}

// ── Gate 1: dev/staging/prod genuinely separate ────────────────────────────
func TestGate1_EnvironmentsSeparate(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("DATABASE_URL_DEV", "postgres://dev-user:dev-pass@dev-host:5432/sidekick_dev")
	t.Setenv("DATABASE_URL_STAGING", "postgres://stg-user:stg-pass@stg-host:5432/sidekick_staging")
	t.Setenv("DATABASE_URL_PROD", "postgres://prd-user:prd-pass@prd-host:5432/sidekick_prod")
	t.Setenv("JWT_SECRET_DEV", "dev-secret-0123456789abcdef-0123456789")
	t.Setenv("JWT_SECRET_STAGING", "staging-secret-0123456789abcdef-01234567")
	t.Setenv("JWT_SECRET_PROD", "prod-secret-0123456789abcdef-01234567890")

	os.Setenv("APP_ENV", "dev")
	devCfg, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, config.EnvDev, devCfg.Env)
	require.Contains(t, devCfg.DatabaseURL, "dev-host")

	os.Setenv("APP_ENV", "staging")
	stgCfg, err := config.Load()
	require.NoError(t, err)
	require.Contains(t, stgCfg.DatabaseURL, "stg-host")
	require.NotEqual(t, devCfg.DatabaseURL, stgCfg.DatabaseURL, "dev credential must not touch staging")
	require.NotEqual(t, devCfg.JWTSecret, stgCfg.JWTSecret)

	// A token minted with the dev secret must NOT verify under staging.
	devIss, err := auth.NewIssuer(devCfg.JWTSecret, "dev")
	require.NoError(t, err)
	stgIss, err := auth.NewIssuer(stgCfg.JWTSecret, "staging")
	require.NoError(t, err)
	tok, err := devIss.MintUser("user-1", time.Hour)
	require.NoError(t, err)
	_, err = stgIss.Verify(tok)
	require.Error(t, err, "dev token must not authenticate against staging")
}

// ── Gate 2: no secret in version-controlled history ────────────────────────
func TestGate2_NoSecretsInRepo(t *testing.T) {
	root, err := filepath.Abs("..")
	require.NoError(t, err)
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
		return string(out)
	}
	tracked := run("ls-files")
	require.NotContains(t, strings.Split(tracked, "\n"), ".env", ".env must never be committed")
	for _, line := range strings.Split(tracked, "\n") {
		require.NotRegexp(t, `^\.env(\.|$)`, strings.TrimSpace(line))
	}
	require.Empty(t, strings.TrimSpace(run("log", "--all", "--oneline", "--", ".env")),
		".env must never appear in history")

	// Staging/prod example values must be placeholders, never real secrets.
	raw, err := os.ReadFile(filepath.Join(root, ".env.example"))
	require.NoError(t, err)
	require.Contains(t, string(raw), "REPLACE_ME", "staging/prod secrets must be placeholders")
}

// ── Gate 3: required extensions queryable ──────────────────────────────────
func TestGate3_Extensions(t *testing.T) {
	pool := setupPool(t)
	ctx := context.Background()
	for _, ext := range []string{"citext", "pg_trgm", "earthdistance"} {
		var one int
		err := pool.QueryRow(ctx, `SELECT 1 FROM pg_extension WHERE extname = $1`, ext).Scan(&one)
		require.NoError(t, err, "extension %s must be enabled", ext)
	}
	// pg_cron is optional (vanilla images lack it): either it is installed,
	// or the Go scheduler fallback is present and functional.
	var cronPresent bool
	err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname='pg_cron')`).Scan(&cronPresent)
	require.NoError(t, err)
	if !cronPresent {
		s := scheduler.New()
		s.Register(scheduler.Job{Name: "probe", Interval: time.Hour, Run: func(ctx context.Context) error { return nil }})
		require.NoError(t, s.RunOnce(ctx, "probe"))
		require.Error(t, s.RunOnce(ctx, "no-such-job"))
	}
	t.Logf("pg_cron present: %v (Go scheduler fallback covers absence)", cronPresent)
}

// ── Gate 4: every table exists ─────────────────────────────────────────────
func TestGate4_AllTablesExist(t *testing.T) {
	pool := setupPool(t)
	ctx := context.Background()
	for _, tbl := range allTables {
		var one int
		err := pool.QueryRow(ctx,
			`SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name=$1`, tbl).Scan(&one)
		require.NoError(t, err, "table %s must exist", tbl)
	}
}

// ── Gate 5 (MOST IMPORTANT): RLS default deny on every table ───────────────
func TestGate5_RLSDefaultDeny(t *testing.T) {
	pool := setupPool(t)
	ctx := context.Background()

	// RLS must be enabled on every table.
	for _, tbl := range allTables {
		var enabled bool
		err := pool.QueryRow(ctx, `SELECT rowsecurity FROM pg_tables WHERE schemaname='public' AND tablename=$1`, tbl).Scan(&enabled)
		require.NoError(t, err)
		require.True(t, enabled, "RLS must be enabled on %s", tbl)
		// Zero policies is the correct starting state.
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM pg_policies WHERE schemaname='public' AND tablename=$1`, tbl).Scan(&n))
		require.Zero(t, n, "table %s must have zero policies in Phase 0", tbl)
	}

	// Restricted role: full grants (so only RLS can deny), no policies.
	_, err := pool.Exec(ctx, `DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='phase0_restricted') THEN
			CREATE ROLE phase0_restricted NOLOGIN;
		END IF;
	END $$`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `GRANT USAGE ON SCHEMA public TO phase0_restricted`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO phase0_restricted`)
	require.NoError(t, err)

	// Seed one row as owner; owner bypasses RLS and must see it.
	seedEmail := fmt.Sprintf("rls-probe-%s@example.com", uuid.NewString())
	var seedID string
	err = pool.QueryRow(ctx,
		`INSERT INTO users (auth_provider, email, display_name) VALUES ('email',$1,'probe') RETURNING id::text`,
		seedEmail).Scan(&seedID)
	require.NoError(t, err)
	var ownerCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE id=$1::uuid`, seedID).Scan(&ownerCount))
	require.Equal(t, 1, ownerCount)

	// As the restricted role every table must yield zero rows — never data.
	withRole(ctx, pool, t, "phase0_restricted", func(ctx context.Context, conn *pgxpool.Conn) {
		for _, tbl := range allTables {
			var n int
			err := conn.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s`, tbl)).Scan(&n)
			require.NoError(t, err, "restricted SELECT on %s must succeed-with-zero-rows (RLS deny), not error", tbl)
			require.Zero(t, n, "table %s leaked rows to restricted role", tbl)
		}
		// Writes must also be denied without a policy.
		_, err := conn.Exec(ctx, `INSERT INTO users (auth_provider, email, display_name) VALUES ('email','blocked@example.com','x')`)
		require.Error(t, err, "restricted INSERT must be denied by RLS")
	})

	_, err = pool.Exec(ctx, `DELETE FROM users WHERE id=$1::uuid`, seedID)
	require.NoError(t, err)
}

// ── Gate 6: audit helper row visible to owner only ─────────────────────────
func TestGate6_AuditLogHelper(t *testing.T) {
	pool := setupPool(t)
	ctx := context.Background()
	var actor string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (auth_provider, email, display_name) VALUES ('email',$1,'audit-probe') RETURNING id::text`,
		fmt.Sprintf("audit-probe-%s@example.com", uuid.NewString())).Scan(&actor))
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1::uuid`, actor)
	})
	require.NoError(t, audit.Log(ctx, pool, audit.Entry{
		ActorID: &actor, Action: "phase0.probe", EntityType: "probe",
		EntityID: uuid.NewString(), Metadata: map[string]any{"k": "v"},
	}))
	var n int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM audit_log WHERE action='phase0.probe' AND actor_id=$1::uuid`, actor).Scan(&n))
	require.Equal(t, 1, n, "owner must read its audit row")

	withRole(ctx, pool, t, "phase0_restricted", func(ctx context.Context, conn *pgxpool.Conn) {
		var rn int
		require.NoError(t, conn.QueryRow(ctx, `SELECT count(*) FROM audit_log`).Scan(&rn))
		require.Zero(t, rn, "restricted role must see zero audit rows")
	})
	_, err := pool.Exec(ctx, `DELETE FROM audit_log WHERE action='phase0.probe' AND actor_id=$1::uuid`, actor)
	require.NoError(t, err)
}

// ── Gate 7: idempotency replays without repeating the side effect ──────────
func TestGate7_Idempotency(t *testing.T) {
	pool := setupPool(t)
	ctx := context.Background()
	key := "phase0-" + uuid.NewString()
	calls := 0
	fn := func() (int, any, error) {
		calls++
		return 200, map[string]any{"ok": true, "n": calls}, nil
	}
	first, err := idempotency.Do(ctx, pool, key, "phase0.probe", "", fn)
	require.NoError(t, err)
	require.False(t, first.Repeated)
	second, err := idempotency.Do(ctx, pool, key, "phase0.probe", "", fn)
	require.NoError(t, err)
	require.True(t, second.Repeated, "second call must replay")
	require.Equal(t, 1, calls, "side effect must run exactly once")
	require.JSONEq(t, string(first.ResponseBody), string(second.ResponseBody))
	_, err = pool.Exec(ctx, `DELETE FROM idempotency_keys WHERE key=$1`, key)
	require.NoError(t, err)
}

// ── Gate 8: bucket privacy posture ─────────────────────────────────────────
func TestGate8_StorageBuckets(t *testing.T) {
	store := storage.NewStore("test-secret-0123456789abcdef-0123456789")
	for _, b := range storage.AllBuckets {
		require.NotEmpty(t, string(b))
	}
	// Private buckets reject unsigned reads.
	for _, b := range []storage.Bucket{
		storage.BucketIDDocuments, storage.BucketDisputeEvidence, storage.BucketChatImages,
	} {
		require.True(t, storage.Private(b))
		require.Error(t, store.AuthorizeRead(b, nil), "%s must reject unsigned reads", b)
		cap := store.MintDownload(b, "obj/1.jpg", time.Minute)
		require.NoError(t, store.AuthorizeRead(b, &cap))
	}
	// Public listing buckets allow unsigned reads of content.
	for _, b := range []storage.Bucket{storage.BucketAvatars, storage.BucketTaskPhotos} {
		require.True(t, storage.PublicRead(b))
		require.NoError(t, store.AuthorizeRead(b, nil))
	}
	// Forged and expired capabilities are rejected.
	good := store.MintDownload(storage.BucketIDDocuments, "a/b.jpg", time.Minute)
	bad := good
	bad.Signature = "deadbeef"
	require.Error(t, store.Verify("GET", bad))
	expiredStore := storage.NewStore("test-secret-0123456789abcdef-0123456789")
	_ = expiredStore
	short := store.MintDownload(storage.BucketIDDocuments, "a/b.jpg", -time.Minute)
	require.Error(t, store.Verify("GET", short))
}

// ── Gate 9: is_admin claim is server-only ──────────────────────────────────
func TestGate9_AdminClaimServerOnly(t *testing.T) {
	iss, err := auth.NewIssuer("gate9-secret-0123456789abcdef-0123456789", "test")
	require.NoError(t, err)
	userTok, err := iss.MintUser("user-9", time.Hour)
	require.NoError(t, err)
	claims, err := iss.Verify(userTok)
	require.NoError(t, err)
	require.False(t, claims.IsAdmin, "user token must never carry is_admin")
	_, err = iss.RequireAdmin(userTok)
	require.Error(t, err, "user token must fail admin gate")

	adminTok, err := iss.MintAdmin("admin-9", time.Hour)
	require.NoError(t, err)
	_, err = iss.RequireAdmin(adminTok)
	require.NoError(t, err)

	// Token signed with another secret (the "client forgery" analogue) fails.
	other, err := auth.NewIssuer("other-secret-0123456789abcdef-0123456789", "test")
	require.NoError(t, err)
	forged, err := other.MintAdmin("user-9", time.Hour)
	require.NoError(t, err)
	_, err = iss.RequireAdmin(forged)
	require.Error(t, err, "foreign-signed admin claim must be rejected")

	// No Go file outside internal/auth (+ tests) may touch the claim —
	// i.e. no client-reachable setter exists.
	root, _ := filepath.Abs("..")
	var offenders []string
	err = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if strings.HasPrefix(rel, "internal"+string(os.PathSeparator)+"auth") ||
			strings.Contains(rel, "tests"+string(os.PathSeparator)) {
			return nil
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		for _, line := range strings.Split(string(raw), "\n") {
			trim := strings.TrimSpace(line)
			if strings.HasPrefix(trim, "//") {
				continue
			}
			if strings.Contains(line, "is_admin") {
				offenders = append(offenders, rel+": "+trim)
			}
		}
		return nil
	})
	require.NoError(t, err)
	require.Empty(t, offenders, "is_admin must only be minted in internal/auth")
}

// ── Gate 10: payments adapter mock, no provider coupling ───────────────────
func TestGate10_PaymentsAdapterMock(t *testing.T) {
	ctx := context.Background()
	var adapter payments.Adapter = payments.NewMock()
	for _, op := range []func(context.Context, payments.Request) (payments.Result, error){
		adapter.Hold, adapter.Capture, adapter.Release, adapter.Refund, adapter.Transfer,
	} {
		res, err := op(ctx, payments.Request{
			IdempotencyKey: "k-" + uuid.NewString(), TaskID: uuid.NewString(),
			Amount: 1500, PlatformFee: 120, FeePayer: "poster",
		})
		require.NoError(t, err)
		require.Equal(t, "succeeded", res.Status)
		require.True(t, strings.HasPrefix(res.ProviderRef, "mock_"))
	}
	// Guardrails: non-positive amount and missing key are rejected pre-provider.
	_, err := adapter.Hold(ctx, payments.Request{IdempotencyKey: "k", Amount: 0})
	require.Error(t, err)
	_, err = adapter.Hold(ctx, payments.Request{Amount: 100})
	require.Error(t, err)
}

// ── Gate 11: app_config seeded, no missing keys ────────────────────────────
func TestGate11_AppConfigSeeded(t *testing.T) {
	pool := setupPool(t)
	ctx := context.Background()
	got, err := appconfig.All(ctx, pool)
	require.NoError(t, err)
	for _, k := range appconfig.RequiredKeys {
		v, ok := got[k]
		require.True(t, ok, "app_config key %q must be seeded", k)
		require.NotEmpty(t, strings.TrimSpace(v), "app_config key %q must be non-empty", k)
	}
}
