# Sidekick — Backend Build Specification (Derived from PRD v0.1 MVP)

> Source: `Sidekick_PRD_v0.1.pdf` — v0.1 MVP, Status: Draft for design, Owner: Adebayo Fayanju, Last updated 24 July 2026
> Pitch: “Post a task, get it done by someone nearby, pay safely.”
> This file contains **everything that must be built in the backend** for MVP. Frontend screens (§9) are out of scope except where they drive API/storage/realtime contracts.

---

## Table of Contents

1. [Backend Scope — In / Out](#1-backend-scope--in--out)
2. [Architecture Overview](#2-architecture-overview)
3. [Core Concepts & Glossary](#3-core-concepts--glossary)
4. [Data Model — Full Schema](#4-data-model--full-schema)
5. [Task Status State Machine (Backend Enforcement)](#5-task-status-state-machine-backend-enforcement)
6. [E1 — Accounts & Identity Backend](#6-e1--accounts--identity-backend)
7. [E2 — Task Creation Backend](#7-e2--task-creation-backend)
8. [E3 — Discovery / Feed / Search Backend](#8-e3--discovery--feed--search-backend)
9. [E4 — Offers & Matching Backend](#9-e4--offers--matching-backend)
10. [E5 — Messaging Backend](#10-e5--messaging-backend)
11. [E6 — Task Execution & Completion Backend](#11-e6--task-execution--completion-backend)
12. [E7 — Payments, Escrow & Wallet Backend](#12-e7--payments-escrow--wallet-backend)
13. [E8 — Ratings & Reputation Backend](#13-e8--ratings--reputation-backend)
14. [E9 — Trust & Safety Backend](#14-e9--trust--safety-backend)
15. [E10 — Notifications Backend](#15-e10--notifications-backend)
16. [E11 — Admin Backend](#16-e11--admin-backend)
17. [Storage Buckets](#17-storage-buckets)
18. [Realtime Channels](#18-realtime-channels)
19. [Background Jobs / Cron / Edge Functions](#19-background-jobs--cron--edge-functions)
20. [API Surface (Supabase + Edge Functions)](#20-api-surface-supabase--edge-functions)
21. [Row Level Security (RLS) Policy Matrix](#21-row-level-security-rls-policy-matrix)
22. [Validation, Business Rules & Invariants](#22-validation-business-rules--invariants)
23. [Privacy, Security & Compliance Backend](#23-privacy-security--compliance-backend)
24. [Non-Functional Requirements — Backend Implications](#24-non-functional-requirements--backend-implications)
25. [Observability, Audit & Analytics Hooks](#25-observability-audit--analytics-hooks)
26. [Open Decisions Blocking Backend](#26-open-decisions-blocking-backend)
27. [Build Order (Mapped to 6-Week Plan)](#27-build-order-mapped-to-6-week-plan)
28. [Explicit Non-Goals — Do Not Build](#28-explicit-non-goals--do-not-build)

---

## 1. Backend Scope — In / Out

### 1.1 In scope (must build backend for)

- Accounts & identity (email/password, Google, Apple, phone OTP, profile, role intent, password reset, sign-out, delete)
- Task CRUD + drafts + lifecycle + status machine + approximate/exact location split + photos
- Discovery feed (geo-sorted, filtered, paginated, exclusion rules) + text search
- Offers (at-budget, counter, pitch, ranking data, accept/decline/withdraw, poster counter one-round, auto-decline)
- 1:1 task-scoped realtime chat (text, delivery/read, image P1, system messages, read-only freeze, report/block entry)
- Execution: mark-complete, confirm/dispute, auto-release 72h, cancellations per §7.3, dispute freeze
- Payments: card funding at post time, escrow hold/capture/transfer/refund, platform fee calc, wallet (available/pending/history), bank payout methods, withdrawals (manual batch for MVP), refunds, receipts
- Ratings & reviews (1–5, 500-char body, double-blind P1, aggregates, no-double-rate)
- Trust & safety: ID upload + verification states, reports, blocks P1, disputes with evidence + freeze, safety guidance flag
- Notifications: 15 events, push + in-app centre, read/unread, deep-link payloads, preferences P1, permission primer state
- Minimal admin console backend: verification queue, reports queue, dispute queue + manual release/refund, payout batch trigger/confirm, user/task search P1
- Supporting infra: Postgres schema for all 14 entities, RLS, Storage, Realtime, Edge Functions, cron, push, audit logs

### 1.2 Deferred — do NOT build backend for

Scheduled/recurring tasks, multi-worker/team jobs, in-app tipping, worker shifts/availability calendars, business/company accounts, insurance/background checks beyond ID, transactional web app, referrals/loyalty, in-app arbitration (manual admin only), in-app calling, photo-based verification, worker skill tags, saved searches/alerts (Phase 2), task templates, map view P2 (no geo-tile backend needed beyond lat/lng + approx circle).

---

## 2. Architecture Overview

**Client:** React Native via Expo (iOS 15+, Android 8+). Requires dev build (`eas build --profile development`, `--profile preview` internal distribution). Expo Go insufficient.

**Backend (per PRD §14):**

```
Mobile App (Expo)
  ├─ Supabase Auth (email, Google, Apple, phone OTP)
  ├─ Supabase Postgres + RLS (all domain tables)
  ├─ Supabase Realtime (conversations/messages, offers, task status, notifications)
  ├─ Supabase Storage (avatars, task_photos, completion_photos, id_documents, chat_images, dispute_evidence)
  ├─ Edge Functions (escrow orchestration, auto-release cron, double-blind publisher, no-offer nudge, payout batch, push fan-out, fee calc, search)
  ├─ Cron / pg_cron / Scheduled Functions (72h auto-release, 24h warning, 14d review publish, 7d chat freeze, 24h no-offer check, payout batch)
  ├─ Payments Provider (TBD — must support hold/capture/transfer/refund + webhooks) — tokenised cards only
  └─ Push: Expo Notifications (Expo push tokens table + deep-link payloads)
Admin Web Console (minimal, not polished) → same Supabase project, service_role / admin claims
```

**Key backend principles:**

1. Card data never touches Sidekick servers — provider-tokenised only. Store only `provider_ref`, `last4`, `bank_ref`.
2. Exact address released only post-acceptance, enforced in RLS + API, never in feed payloads.
3. Phone numbers never exposed to other users via API.
4. Escrow state must be visible to both parties (`secured` / `pending` / `released` / `refunded` / `frozen`).
5. Chat system messages are first-class audit trail for disputes.
6. All money movements are append-only `transactions` rows + wallet balance updates in a DB transaction.
7. Every state transition in §12 must be server-validated; client cannot force status.

---

## 3. Core Concepts & Glossary

| Term | Backend meaning |
|------|-----------------|
| Poster | `users.id` = `tasks.poster_id`, funds task |
| Worker / Sidekick | `users.id` = `offers.worker_id` / `tasks.assigned_worker_id`, earns payout |
| Offer | Row in `offers` (at-budget or counter + message + pitch ≤300 chars) |
| Escrow | `transactions` rows `type=escrow_hold` (captured at funding) → `release` / `refund`; `status` tracks provider state |
| Auto-release | Cron releases escrow 72h after `completed_pending_confirmation` if no dispute |
| Available balance | `wallets.available_balance` — cleared, withdrawable |
| Pending balance | `wallets.pending_balance` — earned but not cleared / held |
| Double-blind | Both `reviews` rows `published_at=NULL` until both submit or 14d elapse |
| Conversation | One row per `task_id` (poster + assigned worker), created on accept |

---

## 4. Data Model — Full Schema

All tables get `id UUID PK DEFAULT gen_random_uuid()`, `created_at TIMESTAMPTZ DEFAULT now()`. Use `CITEXT` for email where case-insensitive. Enable `pg_trgm` + `postgis` (or `earthdistance`) for search + geo. All monetary values: `INTEGER` minor units (pence) — never float.

### 4.1 `users`

Extends `auth.users` (Supabase Auth). `users.id` FK → `auth.users.id` on delete cascade.

| Column | Type | Constraints / Notes |
|--------|------|---------------------|
| `id` | UUID PK | FK auth.users |
| `auth_provider` | TEXT | `email \| google \| apple` — last used; user may link multiples |
| `provider_sub` | TEXT | **Stable provider subject ID, NOT email.** Unique per (`auth_provider`, `provider_sub`). Required for Google/Apple (handles Apple relay + email changes). |
| `email` | CITEXT | Unique, nullable for Apple relay edge; never key joins on this |
| `phone` | TEXT | E.164, unique, nullable until verified; **never expose to other users** |
| `phone_verified` | BOOL DEFAULT false | Trust badge signal |
| `display_name` | TEXT NOT NULL | 2–50 chars |
| `avatar_url` | TEXT | Storage `avatars/` URL |
| `bio` | TEXT | ≤500 chars, nullable |
| `location` | TEXT | Free-text home area + `location_lat/lng` if permission granted (add `latitude DOUBLE`, `longitude DOUBLE`, nullable) |
| `role_intent` | TEXT | `post \| work \| both` — FR-1.6, reversible; drives default home tab + role switcher |
| `verification_status` | TEXT DEFAULT 'unverified' | `unverified \| pending \| verified \| rejected` — denormalised from `verifications` latest |
| `phone_verified_at` | TIMESTAMPTZ | nullable |
| `rating_avg` | NUMERIC(2,1) DEFAULT 0 | Denormalised aggregate, recomputed on review publish |
| `rating_count` | INT DEFAULT 0 | Count of published reviews received |
| `tasks_completed` | INT DEFAULT 0 | Increment on task `completed` (both poster & worker? track separately if needed: `tasks_posted_completed`, `tasks_worked_completed`) |
| `reliability_score` | NUMERIC | Optional: penalise no-show/withdraw — required by §7.3 worker penalised logic |
| `suspended_at` | TIMESTAMPTZ | NULL = active; set by admin FR-11.3 |
| `deleted_at` | TIMESTAMPTZ | Soft-delete; PII purge job on hard delete |
| `expo_push_tokens` | TEXT[] or separate table | See §15 — prefer `push_tokens` table (user_id, token, platform, created_at) |
| `notification_prefs` | JSONB | `{new_offer:true, ...}` — FR-10.4 P1, defaults all true |
| `onboarding_completed` | BOOL DEFAULT false | role + profile + phone done |

Indexes: `UNIQUE(auth_provider, provider_sub)`, `UNIQUE(email)`, `UNIQUE(phone)`, `INDEX(verification_status)`, `INDEX(deleted_at) WHERE deleted_at IS NULL`.

### 4.2 `tasks`

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `poster_id` | UUID NOT NULL FK users | |
| `title` | TEXT NOT NULL | 10–80 chars (enforce) |
| `description` | TEXT NOT NULL | 20–2000 chars |
| `category` | TEXT NOT NULL | Fixed taxonomy TBD (§18 Q4) — enforce CHECK against seed list once decided; currently free TEXT |
| `location_approx` | TEXT NOT NULL | Neighbourhood name shown publicly |
| `location_lat` / `location_lng` | DOUBLE | Approx centre; feed sorting + 500m circle |
| `location_exact` | TEXT | Full address — **RLS: only poster + assigned worker can read** |
| `location_exact_lat/lng` | DOUBLE | nullable |
| `timing_type` | TEXT NOT NULL | `asap \| specific_date \| flexible_range` |
| `scheduled_for` | TIMESTAMPTZ | for `specific_date` |
| `flexible_from` / `flexible_to` | DATE | for `flexible_range` |
| `budget` | INT NOT NULL | Minor units; enforce min/max once decided (§18 Q5) |
| `platform_fee` | INT NOT NULL | Snapshot at funding; calc per fee model (§18 Q1) |
| `total_charge` | INT NOT NULL | `budget + fee` or `budget` depending on model — store both + `fee_payer` enum |
| `status` | TEXT NOT NULL DEFAULT 'draft' | See §5 state machine |
| `assigned_worker_id` | UUID FK users | Set on accept; NULL otherwise |
| `accepted_offer_id` | UUID FK offers | Set on accept |
| `accepted_at` | TIMESTAMPTZ | |
| `agreed_start_at` | TIMESTAMPTZ | For no-show grace calc (§7.3) |
| `marked_complete_at` | TIMESTAMPTZ | When worker marks complete → starts 72h clock |
| `auto_release_at` | TIMESTAMPTZ | `marked_complete_at + 72h` — cron target |
| `completed_at` | TIMESTAMPTZ | Final completion |
| `cancelled_by` / `cancel_reason` | UUID / TEXT | Audit |
| `offer_count` | INT DEFAULT 0 | Denormalised for card |
| `view_count` | INT DEFAULT 0 | Optional analytics |
| `expires_at` | TIMESTAMPTZ | For `expired` (no offers + no action) |
| `draft_payload` | JSONB | Autosave per step FR-2.6 P1 |
| `escrow_status` | TEXT | `none \| secured \| pending_release \| released \| refunded \| frozen` — denormalised from transactions for fast UI |
| `escrow_transaction_id` | UUID FK transactions | Link to hold row |

Indexes: `INDEX(poster_id, status)`, `INDEX(status) WHERE status='open'`, `GIST/earthdistance` on lat/lng, `INDEX(created_at DESC)`, `INDEX(category, status)`, full-text `GIN(to_tsvector(title \|\| description))` for FR-3.3.

### 4.3 `task_photos`

| Column | Type | Notes |
|--------|------|-------|
| `id` UUID PK | | |
| `task_id` FK tasks ON DELETE CASCADE | | |
| `url` TEXT NOT NULL | Storage `task-photos/` | |
| `type` TEXT NOT NULL | `listing \| completion` — FR-6.2 completion photos | |
| `uploaded_by` UUID FK users | | |
| `created_at` | | |

Constraint: max 5 `listing` per task (enforce in function, not just client). Completion photos: allow 0–5.

### 4.4 `offers`

| Column | Type | Notes |
|--------|------|-------|
| `id` UUID PK | | |
| `task_id` FK tasks CASCADE NOT NULL | | |
| `worker_id` FK users NOT NULL | | |
| `amount` INT NOT NULL | At-budget (= task.budget) or counter amount | |
| `is_counter` BOOL DEFAULT false | True if `amount != task.budget at creation` | |
| `message` TEXT | Required if `is_counter` (FR-4.2); optional pitch otherwise ≤300 chars (FR-4.3) | |
| `poster_counter_amount` INT | FR-4.7 P1 — one round only; NULL until poster counters | |
| `poster_counter_status` TEXT | `none \| pending \| accepted \| declined` | |
| `status` TEXT DEFAULT 'pending' | `pending \| accepted \| declined \| withdrawn \| auto_declined \| expired` | |
| `decline_reason` TEXT | FR-4.6 P1 optional | |
| `created_at` / `responded_at` | | |

Constraints: `UNIQUE(task_id, worker_id) WHERE status IN ('pending')` — one active offer per worker per task (feed excludes already-offered FR-3.5). CHECK `amount > 0`. CHECK `char_length(message) <= 300` (or 500 for poster counter?). Indexes: `INDEX(task_id, status)`, `INDEX(worker_id, status)`.

### 4.5 `conversations`

One per task after acceptance.

| Column | Type | Notes |
|--------|------|-------|
| `id` UUID PK | | |
| `task_id` UUID UNIQUE FK tasks NOT NULL | One thread per task | |
| `poster_id` FK users NOT NULL | | |
| `worker_id` FK users NOT NULL | = assigned worker | |
| `last_message_at` TIMESTAMPTZ | For list sorting | |
| `last_message_preview` TEXT | Denormalised | |
| `read_only` BOOL DEFAULT false | True 7d after completion (FR-5.6 P1) — enforced server-side | |
| `read_only_at` TIMESTAMPTZ | When it becomes read-only | |

### 4.6 `messages`

| Column | Type | Notes |
|--------|------|-------|
| `id` UUID PK | | |
| `conversation_id` FK conversations CASCADE NOT NULL | | |
| `sender_id` FK users | NULL for system messages | |
| `type` TEXT NOT NULL | `text \| image \| system` | |
| `body` TEXT NOT NULL | Text content or system event copy; system has i18n key + params (no hardcoded strings) | |
| `system_event` TEXT | For system: `accepted \| marked_complete \| payment_released \| cancelled \| dispute_raised \| ...` | |
| `attachment_url` TEXT | For `image` — Storage `chat-images/` | |
| `read_at` TIMESTAMPTZ | Read receipt (FR-5.3); per-recipient read state needs `message_reads` table if both sides tracked — MVP: `read_at` + `delivered_at` | |
| `delivered_at` TIMESTAMPTZ | | |
| `created_at` | | |

Indexes: `INDEX(conversation_id, created_at DESC)`. For full read receipts per user, add `message_reads(message_id, user_id, read_at)` if needed — keep simple for MVP unless double-tick per user required.

System messages (FR-5.5 P0) must be inserted by server functions on: accept, mark-complete, confirm, release, cancel, dispute raise/resolve, refund, auto-release. They are the dispute audit trail — immutable (no update/delete except admin).

### 4.7 `transactions` (append-only ledger)

| Column | Type | Notes |
|--------|------|-------|
| `id` UUID PK | | |
| `task_id` FK tasks | NULL for pure wallet withdrawals? Keep task link where relevant | |
| `payer_id` FK users | Poster for hold, platform for release, etc. | |
| `payee_id` FK users | Worker for release, poster for refund | |
| `amount` INT NOT NULL | Minor units, always positive; direction via `type` | |
| `platform_fee` INT DEFAULT 0 | Snapshot | |
| `fee_payer` TEXT | `poster \| worker \| split` — per §18 Q1 | |
| `type` TEXT NOT NULL | `escrow_hold \| release \| refund \| withdrawal \| cancellation_compensation` | |
| `status` TEXT NOT NULL | `pending \| secured \| processing \| succeeded \| failed \| reversed` — visible as “secured” to both parties when held | |
| `provider_ref` TEXT | Payment provider ID (charge/transfer ID) | |
| `idempotency_key` TEXT UNIQUE | **Required** — prevents double-charge on retry | |
| `failure_reason` TEXT | | |
| `receipt_url` / `receipt_data` JSONB | For FR-7.8 P1 receipt view | |
| `created_at` / `settled_at` | | |

All inserts via Edge Function in DB transaction with wallet updates. Never update `amount` post-insert; reversals are new rows.

### 4.8 `wallets`

One per user (lazy-create on first earn/fund).

| Column | Type | Notes |
|--------|------|-------|
| `id` UUID PK | | |
| `user_id` UUID UNIQUE FK users NOT NULL | | |
| `available_balance` INT DEFAULT 0 | Cleared, withdrawable | |
| `pending_balance` INT DEFAULT 0 | Earned but not cleared / escrow-secured for poster view? Clarify: worker pending = marked-complete but not confirmed | |
| `currency` CHAR(3) DEFAULT 'GBP' | Single currency MVP, but no hardcoded symbols — store ISO | |
| `updated_at` TIMESTAMPTZ | | |

Constraint: both balances `>= 0`. All balance mutations via `SELECT ... FOR UPDATE` in transaction.

### 4.9 `payout_methods`

| Column | Type | Notes |
|--------|------|-------|
| `id` UUID PK | | |
| `user_id` FK users NOT NULL | | |
| `bank_ref` TEXT NOT NULL | Provider token, NOT raw account number | |
| `last4` CHAR(4) | Display only | |
| `bank_name` TEXT | Display | |
| `is_default` BOOL DEFAULT false | One default per user (partial unique index) | |
| `verified_at` TIMESTAMPTZ | Bank verification if provider requires | |
| `created_at` | | |

Plus `saved_cards` — PRD FR-7.9 P1 “saved cards in settings”: need `payment_methods(id, user_id, provider_ref, last4, brand, exp_month, exp_year, is_default)` — provider-tokenised only. Build as separate table (not in §11 but required).

### 4.10 `reviews`

| Column | Type | Notes |
|--------|------|-------|
| `id` UUID PK | | |
| `task_id` FK tasks NOT NULL | | |
| `reviewer_id` FK users NOT NULL | | |
| `reviewee_id` FK users NOT NULL | | |
| `rating` SMALLINT NOT NULL CHECK 1–5 | FR-8.1 | |
| `body` TEXT CHECK char_length ≤500 | FR-8.2 optional | |
| `is_published` BOOL DEFAULT false | False until double-blind release | |
| `published_at` TIMESTAMPTZ | Set when both submit or 14d elapse (FR-8.3 P1) | |
| `created_at` | | |

Constraints: `UNIQUE(task_id, reviewer_id)` — FR-8.6 cannot rate same task twice. Only allow insert when task `completed` (or `resolved_*`? decide: allow after completed only). Publish logic in cron/function recomputes `users.rating_avg/count`.

### 4.11 `verifications`

| Column | Type | Notes |
|--------|------|-------|
| `id` UUID PK | | |
| `user_id` FK users NOT NULL | | |
| `document_url` TEXT NOT NULL | Storage `id-documents/` — encrypted at rest, restricted access | |
| `document_type` TEXT | `passport \| driving_licence \| national_id` — add, needed for admin | |
| `status` TEXT DEFAULT 'pending' | `pending \| verified \| rejected` (user-level `unverified` = no row) | |
| `reviewed_by` UUID FK users (admin) | | |
| `reviewed_at` TIMESTAMPTZ | | |
| `rejection_reason` TEXT | | |
| `created_at` | | |

On approve/reject, update `users.verification_status` in same transaction + notify user.

### 4.12 `reports`

| Column | Type | Notes |
|--------|------|-------|
| `id` UUID PK | | |
| `reporter_id` FK users NOT NULL | | |
| `reported_user_id` FK users NOT NULL | | |
| `task_id` FK tasks | Nullable, link if task-scoped | |
| `conversation_id` FK conversations | Nullable, if from thread | |
| `reason` TEXT NOT NULL | Category enum: `spam \| fraud \| harassment \| safety \| off_platform_payment \| other` | |
| `detail` TEXT | Free-text (FR-9.4) | |
| `status` TEXT DEFAULT 'open' | `open \| reviewing \| actioned \| dismissed` | |
| `handled_by` UUID (admin) | | |
| `created_at` | | |

### 4.13 `blocks` (implied by FR-9.5, not in §11 — must add)

| Column | Type | Notes |
|--------|------|-------|
| `blocker_id` FK users NOT NULL | | |
| `blocked_id` FK users NOT NULL | | |
| `created_at` | | |
| PK (`blocker_id`, `blocked_id`) | | |

Backend enforces: blocked users’ tasks hidden from feed, offers prevented (check on offer insert), messages blocked.

### 4.14 `disputes`

| Column | Type | Notes |
|--------|------|-------|
| `id` UUID PK | | |
| `task_id` FK tasks NOT NULL | One open dispute per task (`UNIQUE(task_id) WHERE status='open'`) | |
| `raised_by` FK users NOT NULL | Either party (FR-6.6) | |
| `reason` TEXT NOT NULL | Category + free text | |
| `description` TEXT NOT NULL | Detail | |
| `evidence_urls` TEXT[] | Storage `dispute-evidence/` — photos/docs | |
| `status` TEXT DEFAULT 'open' | `open \| under_review \| resolved_released \| resolved_refunded \| resolved_split \| dismissed` | |
| `resolution` TEXT | Admin notes | |
| `compensation_amount` INT | For split/partial (poster-cancel compensation §7.3) | |
| `resolved_by` UUID FK (admin) | | |
| `resolved_at` TIMESTAMPTZ | | |
| `created_at` | | |

Raising dispute freezes escrow: set `tasks.escrow_status='frozen'` + block auto-release cron (skip if dispute open).

### 4.15 `notifications`

| Column | Type | Notes |
|--------|------|-------|
| `id` UUID PK | | |
| `user_id` FK users NOT NULL | Recipient | |
| `type` TEXT NOT NULL | 15 event keys from §10 (e.g. `new_offer`, `offer_accepted`, ...) | |
| `title` / `body` TEXT | Localised at send time; store rendered + i18n key | |
| `payload` JSONB NOT NULL | `{task_id, offer_id, conversation_id, ... , deep_link}` — drives deep link | |
| `channel` TEXT | `push \| in_app \| both` | |
| `read_at` TIMESTAMPTZ | NULL = unread (FR-10.2) | |
| `push_status` TEXT | `sent \| failed \| denied \| skipped` + provider receipt | |
| `created_at` | | |

Index: `INDEX(user_id, created_at DESC) WHERE read_at IS NULL` for unread badge.

### 4.16 Supporting tables (required but not in PRD §11)

- `push_tokens(user_id FK, token TEXT UNIQUE, platform TEXT, created_at)` — Expo push tokens, one row per device.
- `payment_methods(id, user_id FK, provider_ref TEXT, last4, brand, exp_month, exp_year, is_default BOOL)` — FR-7.9.
- `payout_batches(id, status, created_by admin, created_at, processed_at, item_count, total_amount)` + `payout_items(id, batch_id FK, transaction_id FK, user_id, amount, status)` — FR-11.5 manual batch.
- `audit_log(id, actor_id, action, entity_type, entity_id, metadata JSONB, created_at)` — admin actions, escrow moves, suspensions.
- `categories(id, slug UNIQUE, name, icon, sort_order, active)` — once taxonomy decided.
- `app_config(key PK, value JSONB)` — fee %, grace period, auto-release hours (72), no-offer nudge hours (24), chat freeze days (7), review publish days (14).

---

## 5. Task Status State Machine (Backend Enforcement)

```
draft
 └─> open ──────────────> cancelled_by_poster (no accepted offer → full refund, FR-2.8)
 │
 ├─> assigned ─────> cancelled_by_poster (with compensation rules §7.3)
 │ │
 │ ├──────────> cancelled_by_worker ──> open (re-list, poster notified, reliability hit)
 │ │
 │ └─> completed_pending_confirmation (marked_complete_at set, auto_release_at = +72h)
 │   │
 │   ├─> completed ──> (escrow released to wallet)
 │   │
 │   └─> disputed (escrow frozen) ──> resolved_released
 │                                  └─> resolved_refunded (+ resolved_split if needed)
 │
 └─> expired (no offers, poster took no action — cron)
```

**Backend rules:**

| Transition | Allowed caller | Server checks |
|------------|----------------|---------------|
| `draft → open` | Poster | Payment funded + escrow_hold succeeded (FR-7.1). If payment fails → stay `draft`, save `draft_payload`, return retry ( §7.3). |
| `open → cancelled_by_poster` | Poster | Only if no accepted offer. Refund escrow → `refund` transaction, notify offerers (task cancelled). |
| `open → assigned` | Poster (via accept offer) | Offer must be `pending`, task `open`. Atomic: set task assigned, offer accepted, others auto-declined, create conversation, insert system message, release exact location, confirm escrow captured, fan-out notifications within 30s (FR-4.5). |
| `assigned → cancelled_by_poster` | Poster | After agreed start + grace? §7.3: 1h free window, beyond → partial compensation (`cancellation_compensation` transaction). Full refund vs split decided by `agreed_start_at + grace`. Notify worker. |
| `assigned → cancelled_by_worker` (→ `open`) | Worker | Task returns to `open`, `assigned_worker_id` cleared, conversation kept (read-only?), poster notified, worker `reliability_score` decrement. |
| `assigned → completed_pending_confirmation` | Assigned worker only | Sets `marked_complete_at`, `auto_release_at`, optional completion photos, system message, push poster (confirm completion) |
| `completed_pending_confirmation → completed` | Poster confirm OR cron auto-release 72h | Release escrow → wallet `available` (or pending→available), system message, push worker (payment released), increment `tasks_completed` both sides, open review window |
| `* → disputed` | Either party (assigned task) | Freeze escrow (`frozen`), block auto-release, system message, notify other party + admin queue |
| `disputed → resolved_*` | Admin only (FR-11.4) | Manual release/refund/split + resolution note + notify both + unfreeze |
| `open → expired` | Cron | No offers after X + poster no action; notify poster with raise-budget/widen-radius prompt |

All transitions: single `transition_task()` Postgres function or Edge Function with row lock, audit log insert, system message insert, notification fan-out. Reject illegal transitions with 422 + machine-readable code.

---

## 6. E1 — Accounts & Identity Backend

**FR-1.1→1.8 all P0.**

- **Auth providers:** Supabase Auth email/password (FR-1.1), Google (FR-1.2), Apple (FR-1.3 mandatory on iOS). Key on `(auth_provider, provider_sub)` — never email. Handle Apple `Hide My Email` relay: store relay as email, still require OTP step.
- **Phone OTP (FR-1.4 P0):** Supabase phone auth / Verify API via Edge Function. Endpoints: `POST /auth/phone/send`, `POST /auth/phone/verify`. Set `phone_verified=true`, badge signal. Rate-limit sends, 5-min code TTL, max 5 attempts, resend cooldown 60s.
- **Profile (FR-1.5 P0):** `PATCH /me` (display_name, avatar upload via signed URL, bio, location). Avatar → `avatars/{user_id}/...` with image transform (resize).
- **Role intent (FR-1.6 P0):** `POST /me/role {intent: post|work|both}`. Persist `users.role_intent`. Acceptance: `work` → default feed; `both` → role switcher (client reads this field on launch; backend just stores + returns).
- **Reset (FR-1.7), sign-out + delete (FR-1.8):** Supabase built-in reset email; delete = Edge Function `DELETE /me` → soft-delete, purge PII (phone, email, ID docs, exact locations in threads?) per privacy, retain ledger rows anonymised, revoke sessions/tokens. Require confirmation + consequences copy (screen 50).
- **Sessions:** Supabase JWT; RLS uses `auth.uid()`. Admin role via custom claim `is_admin` (set by service_role only).
- **Edge cases:** social sign-in with no phone → still force OTP; email change → keep same `provider_sub` link; duplicate phone → 409.

---

## 7. E2 — Task Creation Backend

**FR-2.1–2.5 P0, FR-2.6–2.7 P1, FR-2.8 P0.**

- **Create:** `POST /tasks` {title, description, category, location_approx + lat/lng, location_exact + lat/lng, timing_type + scheduled_for/flexible range, budget, photo_ids}. Validate lengths, category in taxonomy, budget min/max (config), timing coherence.
- **Photos (FR-2.2):** upload via signed URLs to `task-photos/` (max 5 listing). `POST /tasks/:id/photos` links rows. Enforce count server-side; virus-scan / size-limit (e.g. 10MB, jpg/png/webp).
- **Location split (FR-2.3 P0, acceptance-critical):** store both; **never return `location_exact*` unless `requester = poster OR assigned_worker` AND status in (`assigned`, `completed_pending_confirmation`, `completed`, `disputed`, `resolved_*`)**. Feed/detail for others returns only `location_approx` + 500m-radius circle centre (fuzz lat/lng to ≥500m). Reveal exact in chat thread payload only after acceptance (system message with address, not earlier).
- **Timing (FR-2.4):** `asap | specific_date | flexible_range`. Validate required date fields per type.
- **Budget + fee (FR-2.5):** `GET /fees/quote?budget=` returns `{budget, fee, total, fee_payer}` using `app_config.fee_*`. Displayed transparently before funding; snapshot on task row at funding time (fee changes don’t retro-mutate funded tasks).
- **Draft autosave (FR-2.6 P1):** `PUT /tasks/:id/draft` upserts `draft_payload` + `status='draft'` at every step; `GET /tasks/drafts` resume.
- **Edit while open + no offers (FR-2.7 P1):** `PATCH /tasks/:id` allowed only if `status='open' AND offer_count=0 AND requester=poster`. Any edit bumps `updated_at`, no escrow change (budget edit pre-funding only; post-funding budget lock — decide: require cancel+repost).
- **Cancel/delete open + no accepted offer (FR-2.8 P0):** `DELETE /tasks/:id` or `POST /tasks/:id/cancel` → `cancelled_by_poster` + refund if funded + notify.
- **Funding coupling:** creation + funding atomic in `POST /tasks/:id/fund` (see §12). Payment fail → stay `draft`, task not lost, return `payment_failed` + retry token (§7.3).

---

## 8. E3 — Discovery / Feed / Search Backend

**FR-3.1, 3.2, 3.5, 3.6 P0; FR-3.3 P1; FR-3.4 P2 (skip map tiles).**

- **Feed (FR-3.1 P0):** `GET /feed?lat=&lng=&radius_km=&category=&min_budget=&max_budget=&timing=&cursor=&limit=` — default sort distance ASC, then `created_at DESC`. Uses PostGIS `ST_DWithin` / earthdistance. Returns card fields: `id, title, budget, distance_km, timing_type/scheduled_for, category + icon, offer_count, poster {rating_avg, rating_count, verification_status}, created_at (time-since)`, `location_approx` only. No exact address, no phone.
- **Filters (FR-3.2 P0):** category, budget range, distance radius, timing — all server-side WHERE. Validate radius max (e.g. 50km).
- **Search (FR-3.3 P1):** `GET /search?q=` — `pg_trgm` / FTS across title+description, same card shape + rank. Rate-limit.
- **Map view (FR-3.4 P2):** no dedicated backend; reuse feed lat/lng approx points if later needed.
- **Exclusions (FR-3.5 P0):** `WHERE poster_id != me AND id NOT IN (SELECT task_id FROM offers WHERE worker_id=me) AND me NOT IN blocks`. Also exclude `suspended` posters, non-`open` statuses, expired.
- **Pagination (FR-3.6 P0):** keyset cursor (`created_at, id` or distance), `limit` 20, pull-to-refresh = `cursor=null`. Return `next_cursor`. Cache feed (Redis/CDN or Postgres materialised) for <1.5s FMP on 4G; graceful offline handled client-side with cached payload.

---

## 9. E4 — Offers & Matching Backend

**FR-4.1–4.5, 4.8, 4.9 P0; FR-4.6–4.7 P1.**

- **Make offer at budget (FR-4.1):** `POST /tasks/:id/offers {amount=budget, message?}` — one tap. Checks: task `open`, not own task, not already offered, not blocked, worker not suspended.
- **Counter-offer (FR-4.2):** same endpoint with `amount != budget` + `message` required (400 if missing). `is_counter=true`.
- **Pitch (FR-4.3):** `message` ≤300 chars, sanitised.
- **List offers (poster view, FR-4.4):** `GET /tasks/:id/offers` (poster only) — ranked (server sort: verified first? rating × completed? recency? — define rank function, e.g. `verification boost + rating_avg DESC, tasks_completed DESC`). Each item: worker rating, completed count, verification badges, amount, message, time.
- **My offers (worker view):** `GET /me/offers?tab=pending|accepted|declined` — for screens 28.
- **Accept (FR-4.5 P0, acceptance-critical):** `POST /offers/:id/accept` (poster, task open, offer pending) — **atomic transaction:** 1) lock task, 2) set `tasks.status='assigned'`, `assigned_worker_id`, `accepted_offer_id`, 3) offer → `accepted`, others → `auto_declined`, 4) confirm escrow captured (`transactions` hold `secured`), 5) create `conversations` row, 6) insert system `accepted` message (includes exact location release), 7) fan-out push+in-app to all offerers within 30s. Idempotent via `Idempotency-Key`.
- **Decline individual (FR-4.6 P1):** `POST /offers/:id/decline {reason?}` — offer → `declined`, notify worker.
- **Poster counter (FR-4.7 P1, one round only):** `POST /offers/:id/counter {amount}` — sets `poster_counter_amount`, `status` stays pending; enforce max one poster-counter per offer (409 on second). Worker accept/decline of poster counter via `POST /offers/:id/respond-to-counter`.
- **Withdraw (FR-4.8 P0):** `POST /offers/:id/withdraw` (worker, pending only) → `withdrawn`.
- **Accept → assigned + chat (FR-4.9 P0):** covered above; chat creation is server-side, not client.

---

## 10. E5 — Messaging Backend

**FR-5.1–5.3, 5.5, 5.7 P0; FR-5.4, 5.6 P1.**

- **1:1 task-scoped (FR-5.1):** `conversations` 1:1 per task. `GET /conversations`, `GET /conversations/:id/messages?cursor=`. RLS: only poster/worker.
- **Gate (FR-5.2):** no conversation exists before acceptance; `POST` messages before `assigned` → 403. Conversation auto-created on accept.
- **Text + delivery/read (FR-5.3):** Supabase Realtime `postgres_changes` on `messages` for instant delivery. `POST /conversations/:id/messages {body}` → broadcast; `POST /messages/:id/delivered`, `POST /messages/:id/read` sets timestamps; Realtime updates ticks. Sanitize HTML, length cap (e.g. 2000 chars), rate-limit (e.g. 30/min).
- **Images (FR-5.4 P1):** `POST /conversations/:id/images` signed upload to `chat-images/`, then message `type=image` with `attachment_url`. Image viewer (screen 40) reads same URL with signed expiry.
- **System messages (FR-5.5 P0, first-class):** inserted only by server functions with `sender_id=NULL`, `type=system`, `system_event` enum. Distinct payload so client styles separately. Events: `accepted, marked_complete, payment_released, cancelled, dispute_raised/resolved, refunded, auto_release_scheduled`. Immutable.
- **Read-only freeze (FR-5.6 P1):** cron sets `read_only=true` 7d after `completed_at`; `POST messages` when read_only → 403 with code `chat_frozen`.
- **Report/block in thread (FR-5.7 P0):** `POST /conversations/:id/report`, `POST /users/:id/block` — same as §14; block also freezes thread for blocker.

---

## 11. E6 — Task Execution & Completion Backend

**All P0 except FR-6.2 P1.**

- **Mark complete (FR-6.1):** `POST /tasks/:id/complete` (assigned worker only, status `assigned`) → `completed_pending_confirmation`, set `marked_complete_at`, `auto_release_at = now()+72h`, system message, push poster (confirm completion deep link).
- **Completion photos (FR-6.2 P1):** optional `task_photos type=completion` upload before/with complete call.
- **Confirm/dispute (FR-6.3):** `POST /tasks/:id/confirm` (poster) → `completed` + release (see §12); `POST /tasks/:id/dispute` → `disputed` + freeze (see §14).
- **Auto-release 72h (FR-6.4 P0, critical):** cron `auto_release_worker` every 5–15min: `WHERE status='completed_pending_confirmation' AND auto_release_at <= now() AND NOT EXISTS (open dispute)` → release escrow, `completed`, system message, push both. Plus 24h warning push to poster (see §15).
- **Cancel rules (FR-6.5 → §7.3):** implement per §5 table: no-show (poster cancel after agreed start + grace → full refund + worker penalty), poster-cancel 1h free else partial compensation, worker-withdraw → reopen. All via `POST /tasks/:id/cancel {reason}` with server-side branch on timing + role.
- **Raise dispute freezes escrow (FR-6.6):** same as disputes insert; escrow cron must skip frozen tasks.

---

## 12. E7 — Payments, Escrow & Wallet Backend

**FR-7.1–7.7 P0; FR-7.8–7.9 P1. Highest-risk area — do not cut corners.**

- **Fund at posting (FR-7.1):** `POST /tasks/:id/fund {payment_method_ref, idempotency_key}` → provider `hold/capture` (auth + capture or separate hold). On success: insert `transactions(type=escrow_hold, status=secured)`, set `tasks.status='open'`, `escrow_status='secured'`. On fail: stay `draft`, return retry, task saved (§7.3).
- **Escrow visibility (FR-7.2):** `GET /tasks/:id/escrow` (both parties) → `{status: secured|frozen|released|..., amount, fee, provider_ref_masked}`. Both sides see “secured” badge while held.
- **Fee calc (FR-7.3):** server-side `quote_fee(budget)` from `app_config`; returned + displayed before confirm; snapshot on task. **Blocked on fee model decision (§18 Q1)** — implement `fee_payer` param now so copy doesn’t force redesign.
- **Release on confirm (FR-7.4):** `release_escrow(task_id)` in transaction: `transactions(type=release, status=succeeded)` + `wallets` credit worker `available_balance` (debit pending if used) + `tasks.escrow_status='released'` + system message + push. Called by confirm endpoint, auto-release cron, or admin resolve.
- **Wallet (FR-7.5):** `GET /me/wallet` → `{available_balance, pending_balance, currency, recent_activity[]}`; `GET /me/transactions?type=&cursor=` filterable history; empty state “No earnings yet” driven by zero balances + no rows.
- **Bank + withdraw (FR-7.6):** `POST /me/payout-methods {bank_ref_token}` (provider-tokenised), `POST /me/withdrawals {amount, payout_method_id, idempotency_key}` → checks: `available >= amount`, **ID verification `verified` required (FR-9.3)** else 403 `verification_required`, creates `transactions(type=withdrawal, status=processing)`, debits available, credits? No — external. MVP manual batch: rows queue into `payout_batches` for admin daily trigger (FR-11.5); worker UI shows “arrives in 1–3 working days”, never exposes manual. Webhook `payout.succeeded/failed` updates status + receipt.
- **Refunds (FR-7.7):** `refund_escrow(task_id, reason)` → `transactions(type=refund)` to original method via provider `refund`, `escrow_status='refunded'`, receipt.
- **Receipts (FR-7.8 P1):** `GET /transactions/:id/receipt` → full ledger row + provider ref + timestamps + fee breakdown.
- **Saved cards (FR-7.9 P1):** `GET/POST/DELETE /me/payment-methods` — tokens only, default handling.
- **Webhooks (provider):** `POST /webhooks/payments` (signed) — handle `charge.succeeded/failed`, `transfer.succeeded/failed`, `refund.*`, `payout.*`. Verify signature, idempotent upserts, audit log.
- **Invariants:** all money ops idempotent, ledger append-only, balances non-negative, escrow release exactly once per task (unique partial index on `transactions(task_id) WHERE type='release' AND status='succeeded'`).

---

## 13. E8 — Ratings & Reputation Backend

**FR-8.1–8.2, 8.4–8.6 P0; FR-8.3 P1 (build now — hard to retrofit).**

- **Submit (FR-8.1/8.2):** `POST /tasks/:id/reviews {rating 1–5, body? ≤500}` — one per (task, reviewer) (FR-8.6, unique index). Only after `completed` (or resolved? lock to completed for MVP), only participants.
- **Double-blind (FR-8.3 P1):** store with `is_published=false`, `published_at=NULL`. Publish when **both** reviews exist OR 14d since `completed_at` (cron `publish_reviews`). Until published, `GET reviews` hides body/rating from other party; aggregate `users.rating_*` updates only on publish. Return `my_review_submitted:true, other_submitted:false` so UI shows “waiting” state.
- **Profile aggregates (FR-8.4):** `users.rating_avg/count/tasks_completed/member_since(created_at)` — recomputed in publish transaction. `GET /users/:id/profile` public view returns these + verification badge + bio/avatar (no phone, no exact address).
- **Reviews list (FR-8.5):** `GET /users/:id/reviews?cursor=` most-recent-first, only published.
- **Anti-gaming:** no self-review (reviewer != reviewee check), no edit after publish (or single edit window? MVP: no edit), no second row per task.

---

## 14. E9 — Trust & Safety Backend

**FR-9.1–9.4, 9.6 P0; FR-9.5, 9.7 P1.**

- **ID upload (FR-9.1):** `POST /verifications {document_type}` → signed upload URL to `id-documents/{user_id}/...` (private bucket, SSE, no public read). On upload complete → `verifications(status=pending)` + `users.verification_status='pending'` + enqueue admin.
- **States + badge (FR-9.2):** `unverified (no row) | pending | verified | rejected` — badge comes from `users.verification_status`; `GET /me/verification` returns full state + `rejection_reason` if rejected; public profile returns badge only (no doc URL ever).
- **Withdraw gate (FR-9.3):** enforce in withdrawal function: `IF users.verification_status != 'verified' → 403`. (Open Q6: whether posting also gates — currently withdraw-only; keep flag `app_config.require_verification_to_post` default false for easy flip.)
- **Report (FR-9.4):** `POST /reports {reported_user_id, task_id?, conversation_id?, reason_category, detail?}` → `reports(status=open)` + admin queue + optional auto-flag thresholds (e.g. ≥3 reports → priority).
- **Block (FR-9.5 P1):** `POST /users/:id/block` / `DELETE` — upsert `blocks`. Enforce in feed (hide), offers (403 `blocked`), messages (403), search.
- **Dispute flow (FR-9.6 P0):** `POST /tasks/:id/disputes {reason, description, evidence_ids[]}` → `disputes(status=open)` + `tasks.status='disputed'` + `escrow_status='frozen'` + system message + push other party (dispute detail deep link) + admin queue. Evidence uploads to `dispute-evidence/` (private, admin + parties read). Only one open dispute per task.
- **Safety guidance (FR-9.7 P1):** `GET /me/safety-flags` → `{seen_safety_guidance: bool}`; `POST /me/safety-flags` marks seen. Client shows contextual sheet before first in-person task; backend just persists flag.

---

## 15. E10 — Notifications Backend

**FR-10.1–10.3, 10.5 P0; FR-10.4 P1.**

### 15.1 Event matrix → backend trigger

| Event | Recipient | Channel | Deep link payload | Server trigger point |
|-------|-----------|---------|-------------------|----------------------|
| New offer received | Poster | Push + in-app | `offers_list(task_id)` | `offers` insert |
| Offer accepted | Worker | Push + in-app | `chat(conversation_id)` | accept transaction |
| Offer declined / not selected | Worker | In-app (no push storm) | `feed` | decline / auto-decline fan-out |
| Counter-offer received | Both (other side) | Push + in-app | `offer_detail(offer_id)` | counter insert |
| New message | Recipient | Push + in-app | `chat(conversation_id)` | `messages` insert (debounce/group) |
| Task marked complete | Poster | Push + in-app | `confirm_completion(task_id)` | mark-complete |
| Completion confirmed | Worker | Push + in-app | `wallet` | confirm / release |
| Auto-release warning (24h left) | Poster | Push | `confirm_completion(task_id)` | cron 24h before `auto_release_at` |
| Payment released | Worker | Push + in-app | `wallet` | release transaction |
| Withdrawal processed | Worker | Push + in-app | `transaction_detail(tx_id)` | payout webhook |
| Task cancelled | Other party | Push + in-app | `task_detail(task_id)` | cancel transaction |
| Dispute raised | Other party (+admin) | Push + in-app | `dispute_detail(dispute_id)` | dispute insert |
| ID verification result | User | Push + in-app | `profile` | admin approve/reject |
| Rating received | User | In-app only | `profile` | review publish |
| No offers after 24h | Poster | Push | `task_detail_edit(task_id)` | cron 24h after open with offer_count=0 |

### 15.2 Infra to build

- `notifications` table writes on every trigger (in same DB transaction where possible) + `push_tokens` registry (`POST /me/push-tokens {token, platform}` on login, DELETE on logout).
- Push fan-out Edge Function: reads unread prefs, sends via Expo Push API, stores `push_status`, retries once, handles `DeviceNotRegistered` → prune token.
- In-app centre: `GET /me/notifications?cursor=` with `read/unread`, `POST /me/notifications/:id/read`, `POST /me/notifications/read-all` (FR-10.2). Badge count endpoint for tab.
- Deep links: every `payload` includes `{type, task_id?, offer_id?, conversation_id?, dispute_id?, transaction_id?}` so client routes (FR-10.3).
- Prefs (FR-10.4 P1): `PATCH /me/notification-prefs {event_key: bool}` — fan-out respects prefs (except security/fraud).
- Primer (FR-10.5 P0): backend stores `users.push_primer_seen_at` / `push_permission` status; client shows primer (e.g. after first offer) before OS prompt. `POST /me/push-primer-seen`.

---

## 16. E11 — Admin Backend

**FR-11.1–11.5 P0; FR-11.6 P1. Web console minimal; backend must be complete.**

All admin endpoints require `is_admin` claim (service_role check), audit-logged.

- **Verifications (FR-11.1):** `GET /admin/verifications?status=pending` queue (doc signed URLs, user profile, history) → `POST /admin/verifications/:id/approve` or `/reject {rejection_reason}` → updates `verifications` + `users.verification_status` + notifies user (push+in-app → profile).
- **Reports (FR-11.2):** `GET /admin/reports?status=` → `POST /admin/reports/:id/action {action: dismiss|warn|suspend|ban, note}`.
- **Suspend/ban (FR-11.3):** `POST /admin/users/:id/suspend {reason}` sets `suspended_at`, revokes sessions, blocks login/offers/tasks; `POST /admin/users/:id/reinstate`. Banned = suspend + `deleted` flag? Keep separate `banned_at` if needed.
- **Disputes + manual escrow (FR-11.4):** `GET /admin/disputes?status=open` (task thread, system messages, evidence, escrow state, both profiles) → `POST /admin/disputes/:id/resolve {outcome: release|refund|split, compensation_amount?, resolution_note}` → runs release/refund/split ledger moves + sets `tasks.status=resolved_*` + system message + notifies both.
- **Payout batches (FR-11.5):** `POST /admin/payouts/batch {date}` creates `payout_batches` from `transactions(type=withdrawal, status=processing)` → `POST /admin/payouts/batch/:id/confirm` triggers provider transfers (or manual bank run + mark) → webhooks settle. List/detail endpoints for ops.
- **Search (FR-11.6 P1):** `GET /admin/search?q=&type=user|task` — by email/phone/display_name/task title/id.

---

## 17. Storage Buckets

| Bucket | Public? | Contents | Backend work |
|--------|---------|----------|--------------|
| `avatars` | Public read (or signed) | Profile photos | Signed upload, resize transform, 5MB cap |
| `task-photos` | Public read (listing) | FR-2.2 ≤5 listing photos | Count enforce, signed upload, thumbnails |
| `completion-photos` or same `task-photos` with `type` | Parties-only (signed) | FR-6.2 proof | Signed read for poster/worker/admin only |
| `chat-images` | Parties-only (signed, expiring) | FR-5.4 P1 | Signed upload + expiring read URLs |
| `id-documents` | **Private, encrypted at rest, restricted** | FR-9.1 gov ID | No public access; signed admin-only URLs, short TTL, access logged; retention/purge policy |
| `dispute-evidence` | Parties + admin (signed) | FR-9.6 evidence | Same hardening as ID docs but parties can read own dispute |

All uploads: content-type whitelist, size caps, antivirus scan hook (provider), EXIF strip for privacy.

---

## 18. Realtime Channels

Enable Supabase Realtime (`postgres_changes`) on:

- `messages` (per `conversation_id`) — chat thread live updates, delivery/read ticks.
- `conversations` (per user) — list preview + `read_only` flips.
- `offers` (per `task_id` for poster; per `worker_id` for worker tabs) — new/counter/accept/decline instantly.
- `tasks` (per `poster_id` + assigned worker) — status transitions.
- `notifications` (per `user_id`) — badge + centre live.
- `wallets` / `transactions` (per user) — balance + escrow state live.
- `verifications` (per user) — badge flip.

RLS must scope each channel so users only subscribe to rows they’re party to (see §21).

---

## 19. Background Jobs / Cron / Edge Functions

| Job | Schedule | What it does |
|-----|----------|--------------|
| `auto_release_worker` | Every 5–15 min | Release escrow where `completed_pending_confirmation && auto_release_at<=now() && no open dispute` (FR-6.4) |
| `auto_release_warning` | Same tick | Push poster 24h before `auto_release_at` if still pending (matrix row) |
| `no_offer_nudge` | Hourly | `open && created_at<=now()-24h && offer_count=0` → push poster raise-budget/widen-radius (matrix + §7.3) |
| `expire_tasks` | Daily | `open && expires_at<=now()` → `expired` + notify |
| `freeze_chats` | Daily | `completed_at<=now()-7d && !read_only` → `read_only=true` (FR-5.6) |
| `publish_reviews` | Hourly/daily | Publish double-blind where both submitted OR 14d elapsed; recompute aggregates (FR-8.3) |
| `payout_batch_scheduler` | Daily (manual trigger MVP) | Group `withdrawal/processing` into `payout_batches` for admin confirm (FR-11.5) |
| `push_fanout` | On-demand (DB trigger → queue) | Send Expo pushes, prune dead tokens |
| `fee_quote` | On-demand | Pure function, no schedule |
| `fraud_signal` (light) | On message/report insert | Detect contact-detail exchange (phone/email regex) for off-platform leakage (§17) → flag, don’t block MVP |
| `pii_purge` | Daily | Hard-delete expired soft-deleted users’ PII |

Implement as Supabase Edge Functions + `pg_cron` / Scheduled Functions. All jobs idempotent + logged.

---

## 20. API Surface (Supabase + Edge Functions)

Supabase auto-CRUD covers simple reads (with RLS); all writes with side effects go via Edge Functions for atomicity. `Auth: Bearer JWT` everywhere unless noted.

**Auth:** `POST /auth/signup|signin|oauth/google|oauth/apple`, `POST /auth/phone/send|verify`, `POST /auth/reset`, `DELETE /me`, `PATCH /me`, `POST /me/role`, `POST /me/push-tokens`, `POST /me/push-primer-seen`, `PATCH /me/notification-prefs`

**Tasks:** `POST /tasks`, `GET /tasks/:id` (role-aware location masking), `PATCH /tasks/:id` (open+no-offers only), `DELETE /tasks/:id`, `PUT /tasks/:id/draft`, `GET /tasks/drafts`, `GET /me/tasks?tab=open|in_progress|completed`, `POST /tasks/:id/fund`, `GET /tasks/:id/escrow`, `POST /tasks/:id/cancel`, `POST /tasks/:id/complete`, `POST /tasks/:id/confirm`, `POST /tasks/:id/photos`

**Discovery:** `GET /feed`, `GET /search`, `GET /fees/quote`

**Offers:** `POST /tasks/:id/offers`, `GET /tasks/:id/offers` (poster), `GET /me/offers?tab=`, `POST /offers/:id/accept|decline|withdraw|counter|respond-to-counter`

**Chat:** `GET /conversations`, `GET /conversations/:id/messages`, `POST /conversations/:id/messages|images`, `POST /messages/:id/delivered|read`, `POST /conversations/:id/report`

**Wallet/payments:** `GET /me/wallet`, `GET /me/transactions`, `GET /transactions/:id/receipt`, `GET|POST|DELETE /me/payment-methods`, `POST /me/payout-methods`, `GET /me/payout-methods`, `POST /me/withdrawals`, `POST /webhooks/payments` (provider-signed, no JWT)

**Reviews:** `POST /tasks/:id/reviews`, `GET /users/:id/reviews`, `GET /users/:id/profile`

**Trust:** `POST /verifications`, `GET /me/verification`, `POST /reports`, `POST /users/:id/block`, `DELETE /users/:id/block`, `POST /tasks/:id/disputes`, `GET /disputes/:id`, `POST /me/safety-flags`, `GET /me/safety-flags`

**Notifications:** `GET /me/notifications`, `POST /me/notifications/:id/read`, `POST /me/notifications/read-all`, `GET /me/notifications/unread-count`

**Admin (all `is_admin`):** `GET /admin/verifications`, `POST /admin/verifications/:id/approve|reject`, `GET /admin/reports`, `POST /admin/reports/:id/action`, `POST /admin/users/:id/suspend|reinstate`, `GET /admin/disputes`, `POST /admin/disputes/:id/resolve`, `POST /admin/payouts/batch`, `POST /admin/payouts/batch/:id/confirm`, `GET /admin/payouts/batches`, `GET /admin/search`

Error contract: `{error: {code: SNAKE_CASE, message: human, details?}}` with codes like `payment_failed`, `verification_required`, `chat_frozen`, `blocked`, `illegal_transition`, `already_offered`, `budget_locked`. Never leak provider raw errors or PII.

---

## 21. Row Level Security (RLS) Policy Matrix

Enable RLS on **all** tables. Representative policies (Postgres):

- `users`: anyone authenticated can read non-PII public profile columns via `SECURITY DEFINER` view (`public_profiles` — no phone/email/exact location); full row only `auth.uid()=id` or `is_admin`. No direct update of `rating_*`, `verification_status` (server-only).
- `tasks`: `open` rows readable by all authenticated **but** `location_exact*` stripped — enforce via view `tasks_public` (approx only) vs `tasks_private` (poster/assigned/admin). Insert: authenticated, `poster_id=auth.uid()`. Update: poster for draft/open-no-offers; status changes only via server function (`SECURITY DEFINER`), never direct.
- `task_photos`: read per parent task visibility; insert only poster (listing) or assigned worker (completion).
- `offers`: read: poster of task + own offers + admin. Insert: worker, not own task, not blocked. Update status: only via server functions.
- `conversations/messages`: `auth.uid() IN (poster_id, worker_id)` or admin. System messages insert: server only.
- `transactions/wallets/payout_methods/payment_methods`: owner + admin only. No client-side balance edits — server functions only.
- `reviews`: insert if participant + task completed + no existing row; read published only (or own unpublished); publish flag server-only.
- `verifications/reports/disputes/blocks/notifications/push_tokens`: owner (+counterparty where noted) + admin; `id-documents` storage bucket: owner-write, admin-read only.
- Storage RLS mirrors table RLS per bucket above.

---

## 22. Validation, Business Rules & Invariants

- Title 10–80, description 20–2000, message/pitch ≤300, review body ≤500, bio ≤500.
- Budget >0 + within `app_config.min/max_budget` (TBD Q5); `amount>0` on offers; counter requires message.
- Photos: listing ≤5/task; MIME jpg/png/webp; ≤10MB; completion 0–5.
- Timing: `asap` needs no date; `specific_date` needs `scheduled_for > now()`; `flexible_range` needs `from<=to`, `to>=today`.
- Location: approx required + fuzz ≥500m radius (FR-2.3 acceptance); exact required but hidden pre-accept.
- Offers: one active per (task, worker); cannot offer own task; cannot offer when blocked/suspended/expired/assigned; poster counter max once per offer.
- Chat: no send when `read_only` or pre-accept or blocked; lengths capped; no HTML.
- Money: idempotency keys on fund/withdraw/accept; ledger append-only; single successful release per task; refunds only from secured holds; withdrawals need `available>=amount` + `verified` ID.
- Reviews: one per (task, reviewer); rating 1–5; only after completion; publish only via server.
- Disputes: one open per task; freezes escrow; blocks auto-release.
- Notifications: every state change in §5 + matrix must emit a row; accept fan-out ≤30s (FR-4.5).
- No hardcoded strings/currency — all copy via i18n keys, currency via ISO code (NFR localisation).

---

## 23. Privacy, Security & Compliance Backend

- TLS everywhere; provider-tokenised cards/bank (never raw PAN/account); ID docs encrypted at rest, access-logged, short-TTL signed URLs, admin-only read.
- Exact addresses post-accept only; phone numbers never in any other-user payload; account deletion purges PII but retains anonymised ledger for financial records.
- Supabase Auth JWT rotation; rate-limit OTP/phone/fund/search/message endpoints; idempotency + replay protection on webhooks (verify signatures).
- Apple Sign-In present (App Store rule); privacy disclosures for ID + location + payments before collection.
- Contact-detail leak detection (regex on messages) flagged for admin — mitigates off-platform payment risk without blocking MVP chat.

---

## 24. Non-Functional Requirements — Backend Implications

| Area (PRD §13) | Backend work |
|----------------|--------------|
| Performance: feed FMP <1.5s on 4G, interactions <100ms | Geo + FTS indexes, card-field-only select, keyset pagination, feed cache, connection pooling, Realtime (not polling), CDN for public photos |
| Offline: cached feed + banner | List endpoints support `If-Modified-Since`/ETag + stable ordering so client cache works; mutations queue-safe via idempotency keys |
| Accessibility WCAG 2.2 AA | Backend: no emoji/image-only status — every status has text label copy in API (`status_label`); system messages localised plain-text |
| Security | See §23 |
| Privacy | See §23 |
| Localisation | `app_config.locale='en-GB'`, `currency='GBP'` (confirm Q2); all templates/payloads use keys, no hardcoded £ |
| Platforms iOS 15+, Android 8+ | Push payloads compatible with both; token platform stored per device |

---

## 25. Observability, Audit & Analytics Hooks

- `audit_log` on: auth link, verification decisions, suspensions, dispute resolves, escrow release/refund, payout batch create/confirm, RLS bypass uses.
- Structured logs on every Edge Function (task_id, user_id, idempotency_key, latency, provider_ref).
- Metrics for PRD §5 success targets — emit events/views: `task_posted`, `task_funded`, `first_offer_at` (fill rate ≥60%, time-to-first <4h), `accepted→completed` (≥85%), `post→fund conversion` (≥70%), `dispute_rate` (<5%), repeat poster + D7 worker retention cohorts. Build as SQL views over existing tables (no extra tracking infra for MVP).
- Alert on: payment webhook failures, auto-release failures, push fan-out >30s, payout batch stuck `processing`.

---

## 26. Open Decisions Blocking Backend

1. **Fee model (Q1)** — poster/worker/split? Blocks `quote_fee`, funding copy, wallet math. *Backend mitigation:* implement `fee_payer` enum + config now.
2. **Launch market/currency (Q2)** — determines provider + ID vendor + compliance. *Default:* `en-GB/GBP` in `app_config`, single-locale code paths.
3. **Payment provider (Q3)** — must support hold/capture/transfer/refund to third parties. Confirm before wallet/payout final build (§9.4). *Mitigation:* isolate provider behind `payments_adapter` interface.
4. **Category taxonomy (Q4)** — seed `categories` table; fixed vs free-text changes validation.
5. **Min/max budgets (Q5)** — `app_config` values + CHECK constraints.
6. **ID to post vs withdraw-only (Q6)** — currently withdraw-only; add `require_verification_to_post` flag (default false).
7. **Cancellation compensation % (Q7)** — `app_config.cancellation_compensation_*` consumed by cancel function.
8. **Brand/role naming (Q8)** — no backend impact except i18n keys for “Sidekick” noun.

---

## 27. Build Order (Mapped to 6-Week Plan)

| Week | Source plan | PRD epics | Backend order |
|------|-------------|-----------|---------------|
| 1 | Setup & Auth | E1 | Auth (email/Google/Apple/OTP), `users`, `push_tokens`, role intent, RLS base, full schema migrate (all §11 entities, not just users/tasks), Storage buckets |
| 2 | Task System | E2, E3 | Tasks + drafts + location split + photos + fee quote + feed/filters/pagination + search P1 + geo indexes |
| 3 | Matching | E4 | Offers + counters + accept atomic + auto-decline + withdraw + My Offers + conversation auto-create |
| 4 | Chat & Trust | E5, E8 | Realtime chat + delivery/read + system messages + read-only freeze + reviews double-blind + aggregates |
| 5 | Payments | E7 | Funding/hold, escrow ledger, release/refund, wallet, payout methods, withdrawals + manual batch, webhooks, receipts P1 |
| 6 | Safety & Launch | E9, E10, E11 | Verifications, reports/blocks, disputes + freeze/resolve, notifications matrix + push fan-out + prefs, admin queues + search, cron jobs, audit/metrics |

**Do not slip:** escrow, ID verification gate, dispute handling. **May slip first (per PRD):** search (FR-3.3), chat images (FR-5.4), poster counter round (FR-4.7), blocking (FR-9.5).

---

## 28. Explicit Non-Goals — Do Not Build

No tables/endpoints/jobs for: recurring/scheduled tasks, multi-worker assignment, tipping, shifts/calendars, company accounts, insurance/background checks, transactional web app, referrals, auto-arbitration, calling, skill tags, saved searches, templates, map tiles. If requested, reject with pointer to Phase 2/3 roadmap.

---

*End of backend build spec. Next step: confirm §26 Q1–Q3, then generate Supabase migration (tables + RLS + indexes + functions + cron) directly from §4–§5.*
