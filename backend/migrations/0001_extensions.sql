-- Phase 0 — 0001 extensions.
-- Pure Go backend: only a Postgres connection string is used (no Supabase services).
-- citext / pg_trgm / earthdistance(+cube) / pgcrypto are required and must enable.
-- pg_cron is attempted but OPTIONAL: vanilla Postgres and many managed instances do
-- not ship it (needs shared_preload_libraries). When missing, later phases use the
-- Go scheduler (internal/scheduler) backed by app_config intervals instead.
-- postgis is NOT required: earthdistance is the lighter approved alternative.

CREATE EXTENSION IF NOT EXISTS "citext";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";
CREATE EXTENSION IF NOT EXISTS "cube";
CREATE EXTENSION IF NOT EXISTS "earthdistance";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

DO $$
BEGIN
  BEGIN
    EXECUTE 'CREATE EXTENSION IF NOT EXISTS "pg_cron"';
  EXCEPTION WHEN OTHERS THEN
    RAISE NOTICE 'pg_cron not available (%), Go scheduler is authoritative', SQLERRM;
  END;
END
$$;
