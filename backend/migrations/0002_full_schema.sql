-- Phase 0 — 0002 full schema (BACKEND.md §4.1–§4.16 + additions).
-- One initial migration with every table, so FKs are correct from the start.
-- Additions beyond the PRD list (spec §4.16 + gaps found while implementing):
--   idempotency_keys   (Phase 0 §6 generic mechanism; BACKEND.md requires it but never tabled it)
--   sessions           (Phase 1 §2.5 sign-out revocation + delete revokes sessions)
--   phone_verifications(Phase 1 §2.2 OTP: expiry, attempts, resend cooldown)
-- Money is INTEGER minor units (pence). No floats anywhere.

-- ── users (§4.1, self-contained: no auth.users FK, pure Go owns identity) ──
CREATE TABLE IF NOT EXISTS users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  auth_provider TEXT NOT NULL DEFAULT 'email' CHECK (auth_provider IN ('email','google','apple')),
  provider_sub TEXT,
  email CITEXT UNIQUE,
  password_hash TEXT,
  phone TEXT UNIQUE,
  phone_verified BOOLEAN NOT NULL DEFAULT FALSE,
  phone_verified_at TIMESTAMPTZ,
  display_name TEXT NOT NULL DEFAULT '' CHECK (char_length(display_name) <= 50),
  avatar_url TEXT,
  bio TEXT CHECK (bio IS NULL OR char_length(bio) <= 500),
  location TEXT,
  latitude DOUBLE PRECISION,
  longitude DOUBLE PRECISION,
  role_intent TEXT CHECK (role_intent IS NULL OR role_intent IN ('post','work','both')),
  verification_status TEXT NOT NULL DEFAULT 'unverified'
    CHECK (verification_status IN ('unverified','pending','verified','rejected')),
  rating_avg NUMERIC(2,1) NOT NULL DEFAULT 0,
  rating_count INTEGER NOT NULL DEFAULT 0 CHECK (rating_count >= 0),
  tasks_completed INTEGER NOT NULL DEFAULT 0 CHECK (tasks_completed >= 0),
  reliability_score NUMERIC NOT NULL DEFAULT 100,
  suspended_at TIMESTAMPTZ,
  deleted_at TIMESTAMPTZ,
  notification_prefs JSONB NOT NULL DEFAULT '{}',
  onboarding_completed BOOLEAN NOT NULL DEFAULT FALSE,
  push_primer_seen_at TIMESTAMPTZ,
  seen_safety_guidance BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (auth_provider, provider_sub)
);
CREATE INDEX IF NOT EXISTS idx_users_verification ON users (verification_status);
CREATE INDEX IF NOT EXISTS idx_users_deleted ON users (deleted_at) WHERE deleted_at IS NULL;

-- ── tasks (§4.2) ──
CREATE TABLE IF NOT EXISTS tasks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  poster_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  title TEXT NOT NULL CHECK (char_length(title) BETWEEN 10 AND 80),
  description TEXT NOT NULL CHECK (char_length(description) BETWEEN 20 AND 2000),
  category TEXT NOT NULL,
  location_approx TEXT NOT NULL,
  location_lat DOUBLE PRECISION,
  location_lng DOUBLE PRECISION,
  location_exact TEXT,
  location_exact_lat DOUBLE PRECISION,
  location_exact_lng DOUBLE PRECISION,
  timing_type TEXT NOT NULL CHECK (timing_type IN ('asap','specific_date','flexible_range')),
  scheduled_for TIMESTAMPTZ,
  flexible_from DATE,
  flexible_to DATE,
  budget INTEGER NOT NULL CHECK (budget > 0),
  platform_fee INTEGER NOT NULL DEFAULT 0 CHECK (platform_fee >= 0),
  total_charge INTEGER NOT NULL DEFAULT 0 CHECK (total_charge >= 0),
  fee_payer TEXT NOT NULL DEFAULT 'poster' CHECK (fee_payer IN ('poster','worker','split')),
  status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN (
    'draft','open','assigned','completed_pending_confirmation','completed',
    'disputed','resolved_released','resolved_refunded','resolved_split',
    'cancelled_by_poster','cancelled_by_worker','expired')),
  assigned_worker_id UUID REFERENCES users(id) ON DELETE SET NULL,
  accepted_offer_id UUID,
  accepted_at TIMESTAMPTZ,
  agreed_start_at TIMESTAMPTZ,
  marked_complete_at TIMESTAMPTZ,
  auto_release_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  cancelled_by UUID REFERENCES users(id) ON DELETE SET NULL,
  cancel_reason TEXT,
  offer_count INTEGER NOT NULL DEFAULT 0 CHECK (offer_count >= 0),
  view_count INTEGER NOT NULL DEFAULT 0 CHECK (view_count >= 0),
  expires_at TIMESTAMPTZ,
  draft_payload JSONB,
  escrow_status TEXT NOT NULL DEFAULT 'none' CHECK (escrow_status IN (
    'none','secured','pending_release','released','refunded','frozen')),
  escrow_transaction_id UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_tasks_poster_status ON tasks (poster_id, status);
CREATE INDEX IF NOT EXISTS idx_tasks_open ON tasks (status) WHERE status = 'open';
CREATE INDEX IF NOT EXISTS idx_tasks_created ON tasks (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tasks_category ON tasks (category, status);
CREATE INDEX IF NOT EXISTS idx_tasks_auto_release ON tasks (auto_release_at)
  WHERE status = 'completed_pending_confirmation';

-- ── task_photos (§4.3) ──
CREATE TABLE IF NOT EXISTS task_photos (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  url TEXT NOT NULL,
  type TEXT NOT NULL CHECK (type IN ('listing','completion')),
  uploaded_by UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_task_photos_task ON task_photos (task_id);

-- ── offers (§4.4) ──
CREATE TABLE IF NOT EXISTS offers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  worker_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  amount INTEGER NOT NULL CHECK (amount > 0),
  is_counter BOOLEAN NOT NULL DEFAULT FALSE,
  message TEXT CHECK (message IS NULL OR char_length(message) <= 300),
  poster_counter_amount INTEGER CHECK (poster_counter_amount IS NULL OR poster_counter_amount > 0),
  poster_counter_status TEXT NOT NULL DEFAULT 'none'
    CHECK (poster_counter_status IN ('none','pending','accepted','declined')),
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN (
    'pending','accepted','declined','withdrawn','auto_declined','expired')),
  decline_reason TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  responded_at TIMESTAMPTZ,
  UNIQUE (task_id, worker_id, status)
);
-- One ACTIVE offer per (task, worker): partial unique index is the real guard.
DROP INDEX IF EXISTS uq_offers_active;
CREATE UNIQUE INDEX IF NOT EXISTS uq_offers_active ON offers (task_id, worker_id)
  WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_offers_task ON offers (task_id, status);
CREATE INDEX IF NOT EXISTS idx_offers_worker ON offers (worker_id, status);

-- ── conversations (§4.5) ──
CREATE TABLE IF NOT EXISTS conversations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id UUID NOT NULL UNIQUE REFERENCES tasks(id) ON DELETE CASCADE,
  poster_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  worker_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  last_message_at TIMESTAMPTZ,
  last_message_preview TEXT,
  read_only BOOLEAN NOT NULL DEFAULT FALSE,
  read_only_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_conversations_users ON conversations (poster_id, worker_id);

-- ── messages (§4.6) ──
CREATE TABLE IF NOT EXISTS messages (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  sender_id UUID REFERENCES users(id) ON DELETE SET NULL,
  type TEXT NOT NULL CHECK (type IN ('text','image','system')),
  body TEXT NOT NULL CHECK (char_length(body) BETWEEN 1 AND 2000),
  system_event TEXT,
  attachment_url TEXT,
  delivered_at TIMESTAMPTZ,
  read_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_messages_conv ON messages (conversation_id, created_at DESC);

-- ── transactions (§4.7 append-only ledger) ──
CREATE TABLE IF NOT EXISTS transactions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
  payer_id UUID REFERENCES users(id) ON DELETE SET NULL,
  payee_id UUID REFERENCES users(id) ON DELETE SET NULL,
  amount INTEGER NOT NULL CHECK (amount > 0),
  platform_fee INTEGER NOT NULL DEFAULT 0 CHECK (platform_fee >= 0),
  fee_payer TEXT CHECK (fee_payer IS NULL OR fee_payer IN ('poster','worker','split')),
  type TEXT NOT NULL CHECK (type IN (
    'escrow_hold','release','refund','withdrawal','cancellation_compensation')),
  status TEXT NOT NULL CHECK (status IN (
    'pending','secured','processing','succeeded','failed','reversed')),
  provider_ref TEXT,
  idempotency_key TEXT UNIQUE,
  failure_reason TEXT,
  receipt_data JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  settled_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_tx_task ON transactions (task_id);
CREATE INDEX IF NOT EXISTS idx_tx_payee ON transactions (payee_id, created_at DESC);
-- Exactly one successful release per task.
CREATE UNIQUE INDEX IF NOT EXISTS uq_tx_release_once ON transactions (task_id)
  WHERE type = 'release' AND status = 'succeeded';

-- ── wallets (§4.8) ──
CREATE TABLE IF NOT EXISTS wallets (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
  available_balance INTEGER NOT NULL DEFAULT 0 CHECK (available_balance >= 0),
  pending_balance INTEGER NOT NULL DEFAULT 0 CHECK (pending_balance >= 0),
  currency CHAR(3) NOT NULL DEFAULT 'GBP',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ── payout_methods = bank accounts (§4.9) ──
CREATE TABLE IF NOT EXISTS payout_methods (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  bank_ref TEXT NOT NULL,
  last4 CHAR(4),
  bank_name TEXT,
  is_default BOOLEAN NOT NULL DEFAULT FALSE,
  verified_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_payout_default ON payout_methods (user_id)
  WHERE is_default;

-- ── payment_methods = saved cards (FR-7.9, provider tokens only) ──
CREATE TABLE IF NOT EXISTS payment_methods (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider_ref TEXT NOT NULL,
  last4 CHAR(4),
  brand TEXT,
  exp_month SMALLINT,
  exp_year SMALLINT,
  is_default BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_card_default ON payment_methods (user_id)
  WHERE is_default;

-- ── reviews (§4.10) ──
CREATE TABLE IF NOT EXISTS reviews (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  reviewer_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  reviewee_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  rating SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
  body TEXT CHECK (body IS NULL OR char_length(body) <= 500),
  is_published BOOLEAN NOT NULL DEFAULT FALSE,
  published_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (task_id, reviewer_id),
  CHECK (reviewer_id <> reviewee_id)
);
CREATE INDEX IF NOT EXISTS idx_reviews_reviewee ON reviews (reviewee_id, published_at DESC)
  WHERE is_published;

-- ── verifications (§4.11) ──
CREATE TABLE IF NOT EXISTS verifications (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  document_url TEXT NOT NULL,
  document_type TEXT CHECK (document_type IS NULL OR document_type IN (
    'passport','driving_licence','national_id')),
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','verified','rejected')),
  reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
  reviewed_at TIMESTAMPTZ,
  rejection_reason TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_verifications_user ON verifications (user_id, created_at DESC);

-- ── reports (§4.12) ──
CREATE TABLE IF NOT EXISTS reports (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  reporter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  reported_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
  conversation_id UUID REFERENCES conversations(id) ON DELETE SET NULL,
  reason TEXT NOT NULL CHECK (reason IN (
    'spam','fraud','harassment','safety','off_platform_payment','other')),
  detail TEXT,
  status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','reviewing','actioned','dismissed')),
  handled_by UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (reporter_id <> reported_user_id)
);
CREATE INDEX IF NOT EXISTS idx_reports_status ON reports (status, created_at DESC);

-- ── blocks (§4.13) ──
CREATE TABLE IF NOT EXISTS blocks (
  blocker_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  blocked_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (blocker_id, blocked_id),
  CHECK (blocker_id <> blocked_id)
);

-- ── disputes (§4.14) ──
CREATE TABLE IF NOT EXISTS disputes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  raised_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  reason TEXT NOT NULL,
  description TEXT NOT NULL,
  evidence_urls TEXT[] NOT NULL DEFAULT '{}',
  status TEXT NOT NULL DEFAULT 'open' CHECK (status IN (
    'open','under_review','resolved_released','resolved_refunded','resolved_split','dismissed')),
  resolution TEXT,
  compensation_amount INTEGER CHECK (compensation_amount IS NULL OR compensation_amount >= 0),
  resolved_by UUID REFERENCES users(id) ON DELETE SET NULL,
  resolved_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dispute_open ON disputes (task_id)
  WHERE status IN ('open','under_review');

-- ── notifications (§4.15) ──
CREATE TABLE IF NOT EXISTS notifications (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  type TEXT NOT NULL,
  title TEXT,
  body TEXT,
  payload JSONB NOT NULL DEFAULT '{}',
  channel TEXT NOT NULL DEFAULT 'in_app' CHECK (channel IN ('push','in_app','both')),
  read_at TIMESTAMPTZ,
  push_status TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_notif_unread ON notifications (user_id, created_at DESC)
  WHERE read_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_notif_user ON notifications (user_id, created_at DESC);

-- ── §4.16 supporting tables ──
CREATE TABLE IF NOT EXISTS push_tokens (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token TEXT NOT NULL UNIQUE,
  platform TEXT NOT NULL CHECK (platform IN ('ios','android','web')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_push_user ON push_tokens (user_id);

CREATE TABLE IF NOT EXISTS payout_batches (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','confirmed','processing','settled','failed')),
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  processed_at TIMESTAMPTZ,
  item_count INTEGER NOT NULL DEFAULT 0 CHECK (item_count >= 0),
  total_amount INTEGER NOT NULL DEFAULT 0 CHECK (total_amount >= 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS payout_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  batch_id UUID NOT NULL REFERENCES payout_batches(id) ON DELETE CASCADE,
  transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE RESTRICT,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  amount INTEGER NOT NULL CHECK (amount > 0),
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','processing','succeeded','failed')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (batch_id, transaction_id)
);

CREATE TABLE IF NOT EXISTS audit_log (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
  action TEXT NOT NULL,
  entity_type TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_audit_entity ON audit_log (entity_type, entity_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_actor ON audit_log (actor_id, created_at DESC);

CREATE TABLE IF NOT EXISTS categories (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  icon TEXT,
  sort_order INTEGER NOT NULL DEFAULT 0,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS app_config (
  key TEXT PRIMARY KEY,
  value JSONB NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ── Additions found while implementing (not tabled in BACKEND.md) ──
CREATE TABLE IF NOT EXISTS idempotency_keys (
  key TEXT PRIMARY KEY,
  operation TEXT NOT NULL,
  user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  response_code INTEGER NOT NULL,
  response_body JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL DEFAULT now() + INTERVAL '24 hours'
);
CREATE INDEX IF NOT EXISTS idx_idem_expires ON idempotency_keys (expires_at);

CREATE TABLE IF NOT EXISTS sessions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  refresh_token_hash TEXT NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions (user_id);

CREATE TABLE IF NOT EXISTS phone_verifications (
  phone TEXT PRIMARY KEY,
  code_hash TEXT NOT NULL,
  attempts INTEGER NOT NULL DEFAULT 0,
  last_sent_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL,
  verified_at TIMESTAMPTZ
);

-- FK added after offers exists (avoids forward reference).
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_tasks_accepted_offer') THEN
    ALTER TABLE tasks ADD CONSTRAINT fk_tasks_accepted_offer
      FOREIGN KEY (accepted_offer_id) REFERENCES offers(id) ON DELETE SET NULL;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_tasks_escrow_tx') THEN
    ALTER TABLE tasks ADD CONSTRAINT fk_tasks_escrow_tx
      FOREIGN KEY (escrow_transaction_id) REFERENCES transactions(id) ON DELETE SET NULL;
  END IF;
END
$$;
