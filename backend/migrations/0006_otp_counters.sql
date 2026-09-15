-- Phase 1 — OTP send counters (per-phone hourly cap, DB-backed so it
-- survives restarts and works across instances; per-IP limiting is an
-- in-memory window in the OTP service, documented there).

ALTER TABLE phone_verifications
  ADD COLUMN IF NOT EXISTS sent_count INTEGER NOT NULL DEFAULT 1 CHECK (sent_count >= 0),
  ADD COLUMN IF NOT EXISTS window_started_at TIMESTAMPTZ NOT NULL DEFAULT now();
