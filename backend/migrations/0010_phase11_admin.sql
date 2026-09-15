-- Phase 11 — Admin Console Backend.
-- Add banned_at to users for permanent ban (distinct from reversible suspended_at).
-- Add action_note to reports for moderation notes.

ALTER TABLE users ADD COLUMN IF NOT EXISTS banned_at TIMESTAMPTZ;
ALTER TABLE reports ADD COLUMN IF NOT EXISTS action_note TEXT;
