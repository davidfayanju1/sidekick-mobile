-- Phase 10 — Notifications Backend additions.
-- Add push_permission to users, no_offer_nudge_sent_at to tasks.

ALTER TABLE users ADD COLUMN IF NOT EXISTS push_permission TEXT NOT NULL DEFAULT 'undetermined'
  CHECK (push_permission IN ('undetermined','granted','denied'));

ALTER TABLE tasks ADD COLUMN IF NOT EXISTS no_offer_nudge_sent_at TIMESTAMPTZ;
