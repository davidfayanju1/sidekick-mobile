-- Phase 1 — additions found while implementing (spec gaps).
-- user_identities: a user may link a SECOND provider to the same account
--   (Phase 1 §2.1: do not create a duplicate user row). users(provider_sub)
--   keeps the primary/first identity; linked ones live here.
-- password_resets: standard email reset flow (Phase 1 §2.5). Token hashes
--   only — raw tokens are emailed once and never stored.

CREATE TABLE IF NOT EXISTS user_identities (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  auth_provider TEXT NOT NULL CHECK (auth_provider IN ('email','google','apple')),
  provider_sub TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (auth_provider, provider_sub)
);
CREATE INDEX IF NOT EXISTS idx_identities_user ON user_identities (user_id);

CREATE TABLE IF NOT EXISTS password_resets (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL DEFAULT now() + INTERVAL '1 hour',
  used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_pwresets_user ON password_resets (user_id);

-- RLS default deny applies to new tables too (Phase 0 rule: never RLS-off).
ALTER TABLE user_identities ENABLE ROW LEVEL SECURITY;
ALTER TABLE password_resets ENABLE ROW LEVEL SECURITY;
