-- Phase 0 — 0003 RLS default deny (Testing Gate #5 is the most important test).
-- Pure-Go reading: Postgres is reachable ONLY via connection string; no client SDK
-- talks to it directly. The Go server connects as the table owner (bypasses RLS).
-- Enabling RLS with ZERO policies means any other role (anon / leaked / future
-- direct-access role) gets zero rows on every table. Later phases add policies only
-- if direct DB access is ever introduced; application authz lives in Go middleware.
-- Never disable RLS "temporarily".

DO $$
DECLARE t TEXT;
BEGIN
  FOREACH t IN ARRAY ARRAY[
    'users','tasks','task_photos','offers','conversations','messages',
    'transactions','wallets','payout_methods','payment_methods','reviews',
    'verifications','reports','blocks','disputes','notifications',
    'push_tokens','payout_batches','payout_items','audit_log','categories',
    'app_config','idempotency_keys','sessions','phone_verifications'
  ] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
  END LOOP;
END
$$;
