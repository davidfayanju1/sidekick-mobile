-- Phase 9 — Trust & Safety additions.
-- Partial unique index on verifications to enforce one pending per user.

-- ── verifications: at most one pending per user ──
CREATE UNIQUE INDEX IF NOT EXISTS uq_verification_pending ON verifications (user_id)
  WHERE status = 'pending';
