import { z } from 'zod';

/**
 * Mirrors `components.schemas` in backend/internal/httpapi/openapi.yaml.
 *
 * These are deliberately lenient about *extra* fields and generous with
 * `.nullish()` — a backend that grows a column should never blank a screen.
 * Entity schemas exist mainly to derive types; the strict parsing happens on
 * the way *out*, in the form schemas at the bottom of this file.
 *
 * Money is always an integer in minor units (pence). Never a float.
 */

// ── Shared ───────────────────────────────────────────────────────────────────

const uuid = z.string();
const timestamp = z.string();
const money = z.number().int();

export const verificationStatusSchema = z.enum(['unverified', 'pending', 'verified', 'rejected']);

/** The API's role vocabulary. The UI calls these Hero (poster) and Sidekick (worker). */
export const roleIntentSchema = z.enum(['post', 'work', 'both']);

/**
 * Mirrors the CHECK constraint on `tasks.status` in
 * migrations/0002_full_schema.sql — which is the real contract.
 *
 * NOTE: openapi.yaml disagrees with the database here. It lists
 * `pending_confirm`, `cancelled` and `released`, none of which the schema
 * permits or the Go code ever writes. The migration wins.
 */
export const taskStatusSchema = z.enum([
  'draft',
  'open',
  'assigned',
  'completed_pending_confirmation',
  'completed',
  'disputed',
  'resolved_released',
  'resolved_refunded',
  'resolved_split',
  'cancelled_by_poster',
  'cancelled_by_worker',
  'expired',
]);

export const escrowStatusSchema = z.enum([
  'none',
  'secured',
  'pending_release',
  'released',
  'refunded',
  'frozen',
]);

export const timingTypeSchema = z.enum(['asap', 'specific_date', 'flexible_range']);

// ── Accounts ─────────────────────────────────────────────────────────────────

export const userSchema = z.object({
  id: uuid,
  auth_provider: z.enum(['email', 'google', 'apple']).nullish(),
  email: z.string().nullish(),
  phone: z.string().nullish(),
  phone_verified: z.boolean().nullish(),
  display_name: z.string(),
  avatar_url: z.string().nullish(),
  bio: z.string().nullish(),
  location: z.string().nullish(),
  latitude: z.number().nullish(),
  longitude: z.number().nullish(),
  role_intent: roleIntentSchema.nullish(),
  verification_status: verificationStatusSchema,
  rating_avg: z.number().nullish(),
  rating_count: z.number().nullish(),
  tasks_completed: z.number().nullish(),
  onboarding_completed: z.boolean().nullish(),
  suspended_at: timestamp.nullish(),
  banned_at: timestamp.nullish(),
  created_at: timestamp,
});

export const publicProfileSchema = z.object({
  id: uuid,
  display_name: z.string(),
  avatar_url: z.string().nullish(),
  bio: z.string().nullish(),
  location: z.string().nullish(),
  verification_status: verificationStatusSchema,
  rating_avg: z.number().nullish(),
  rating_count: z.number().nullish(),
  tasks_completed: z.number().nullish(),
  member_since: timestamp.nullish(),
});

export const authTokensSchema = z.object({
  access_token: z.string(),
  refresh_token: z.string(),
  user: userSchema,
});

// ── Tasks ────────────────────────────────────────────────────────────────────

export const taskPhotoSchema = z.object({
  id: uuid,
  url: z.string(),
  type: z.enum(['listing', 'completion']),
});

export const taskSchema = z.object({
  id: uuid,
  poster_id: uuid,
  title: z.string(),
  description: z.string().nullish(),
  category: z.string().nullish(),
  location_approx: z.string().nullish(),
  location_lat: z.number().nullish(),
  location_lng: z.number().nullish(),
  /** Released only to the poster and the assigned worker. */
  location_exact: z.string().nullish(),
  location_exact_lat: z.number().nullish(),
  location_exact_lng: z.number().nullish(),
  timing_type: timingTypeSchema.nullish(),
  scheduled_for: timestamp.nullish(),
  flexible_from: z.string().nullish(),
  flexible_to: z.string().nullish(),
  budget: money,
  platform_fee: money.nullish(),
  total_charge: money.nullish(),
  status: taskStatusSchema,
  offer_count: z.number().nullish(),
  escrow_status: escrowStatusSchema.nullish(),
  draft_payload: z.record(z.string(), z.unknown()).nullish(),
  photos: z.array(taskPhotoSchema).nullish(),
  created_at: timestamp,
  updated_at: timestamp.nullish(),
});

export const taskCardSchema = z.object({
  id: uuid,
  title: z.string(),
  description: z.string().nullish(),
  budget: money,
  distance_km: z.number().nullish(),
  timing_type: timingTypeSchema.nullish(),
  scheduled_for: timestamp.nullish(),
  flexible_from: z.string().nullish(),
  flexible_to: z.string().nullish(),
  category: z.string().nullish(),
  offer_count: z.number().nullish(),
  location_approx: z.string().nullish(),
  created_at: timestamp,
  poster: z
    .object({
      rating_avg: z.number().nullish(),
      rating_count: z.number().nullish(),
      verification_status: z.string().nullish(),
    })
    .nullish(),
});

export const quoteSchema = z.object({
  budget: money,
  fee: money,
  total: money,
  fee_payer: z.enum(['poster', 'worker']),
});

export const escrowSchema = z.object({
  status: z.string(),
  amount: money.nullish(),
  fee: money.nullish(),
  provider_ref_masked: z.string().nullish(),
});

// ── Offers ───────────────────────────────────────────────────────────────────

export const offerStatusSchema = z.enum([
  'pending',
  'accepted',
  'declined',
  'withdrawn',
  'auto_declined',
  'expired',
]);

export const offerSchema = z.object({
  id: uuid,
  task_id: uuid,
  worker_id: uuid,
  amount: money,
  is_counter: z.boolean().nullish(),
  message: z.string().nullish(),
  poster_counter_amount: money.nullish(),
  poster_counter_status: z.string().nullish(),
  status: offerStatusSchema,
  decline_reason: z.string().nullish(),
  created_at: timestamp,
  responded_at: timestamp.nullish(),
});

export const offerWithWorkerSchema = offerSchema.extend({
  worker_display_name: z.string().nullish(),
  worker_avatar_url: z.string().nullish(),
  worker_rating_avg: z.number().nullish(),
  worker_rating_count: z.number().nullish(),
  worker_verification_status: z.string().nullish(),
  worker_tasks_completed: z.number().nullish(),
});

// ── Messaging ────────────────────────────────────────────────────────────────

export const conversationSchema = z.object({
  id: uuid,
  task_id: uuid,
  poster_id: uuid,
  worker_id: uuid,
  task_title: z.string().nullish(),
  last_message_body: z.string().nullish(),
  last_message_at: timestamp.nullish(),
  unread_count: z.number().nullish(),
});

export const messageSchema = z.object({
  id: uuid,
  conversation_id: uuid,
  /** Null for system messages. */
  sender_id: uuid.nullish(),
  sender_name: z.string().nullish(),
  type: z.enum(['text', 'image', 'system']),
  body: z.string().nullish(),
  system_event: z.string().nullish(),
  attachment_url: z.string().nullish(),
  created_at: timestamp,
});

// ── Finance ──────────────────────────────────────────────────────────────────

export const transactionTypeSchema = z.enum([
  'escrow_hold',
  'release',
  'refund',
  'withdrawal',
  'cancellation_compensation',
]);

/** `secured` is an escrow hold still held; `reversed` is a backed-out entry. */
export const transactionStatusSchema = z.enum([
  'pending',
  'secured',
  'processing',
  'succeeded',
  'failed',
  'reversed',
]);

export const walletSchema = z.object({
  available_balance: money,
  pending_balance: money,
  currency: z.string().nullish(),
  recent_activity: z
    .array(
      z.object({
        id: uuid,
        type: z.string(),
        amount: money,
        fee: money.nullish(),
        status: z.string(),
        created_at: timestamp,
      })
    )
    .nullish(),
});

export const transactionSchema = z.object({
  id: uuid,
  task_id: uuid.nullish(),
  payer_id: uuid.nullish(),
  payee_id: uuid.nullish(),
  amount: money,
  platform_fee: money.nullish(),
  fee_payer: z.string().nullish(),
  type: transactionTypeSchema,
  status: transactionStatusSchema,
  provider_ref: z.string().nullish(),
  created_at: timestamp,
  settled_at: timestamp.nullish(),
});

export const paymentMethodSchema = z.object({
  id: uuid,
  provider_ref: z.string().nullish(),
  last4: z.string(),
  brand: z.string().nullish(),
  exp_month: z.number().nullish(),
  exp_year: z.number().nullish(),
  is_default: z.boolean().nullish(),
});

export const payoutMethodSchema = z.object({
  id: uuid,
  bank_ref: z.string().nullish(),
  last4: z.string(),
  bank_name: z.string().nullish(),
  is_default: z.boolean().nullish(),
});

export const receiptSchema = transactionSchema.extend({
  fee_payer: z.string().nullish(),
});

export const withdrawalSchema = z.object({
  id: uuid,
  amount: money,
  status: z.string(),
  repeated: z.boolean().nullish(),
});

// ── Reviews ──────────────────────────────────────────────────────────────────

export const reviewSchema = z.object({
  id: uuid,
  task_id: uuid,
  reviewer_id: uuid,
  reviewee_id: uuid,
  rating: z.number().int().min(1).max(5),
  comment: z.string().nullish(),
  body: z.string().nullish(),
  is_published: z.boolean().nullish(),
  published_at: timestamp.nullish(),
  created_at: timestamp,
});

// ── Notifications ────────────────────────────────────────────────────────────

export const notificationTypeSchema = z.enum([
  'offer_received',
  'offer_accepted',
  'offer_declined',
  'task_completed',
  'task_confirmed',
  'payment_released',
  'dispute_opened',
  'dispute_resolved',
  'review_received',
  'verification_approved',
  'verification_rejected',
  'admin_warning',
  'system',
]);

export const notificationSchema = z.object({
  id: uuid,
  type: notificationTypeSchema.catch('system'),
  title: z.string(),
  body: z.string().nullish(),
  entity_type: z.string().nullish(),
  entity_id: uuid.nullish(),
  is_read: z.boolean().nullish(),
  created_at: timestamp,
});

// ── Trust & safety ───────────────────────────────────────────────────────────

export const documentTypeSchema = z.enum(['passport', 'driving_licence', 'national_id']);

export const verificationSchema = z.object({
  id: uuid,
  user_id: uuid,
  document_type: documentTypeSchema,
  document_url: z.string().nullish(),
  // The DB says 'verified', not 'approved' — openapi.yaml is wrong again.
  status: z.enum(['pending', 'verified', 'rejected']),
  rejection_reason: z.string().nullish(),
  reviewed_at: timestamp.nullish(),
  created_at: timestamp,
});

export const reportReasonSchema = z.enum([
  'spam',
  'fraud',
  'harassment',
  'safety',
  'off_platform_payment',
  'other',
]);

export const reportSchema = z.object({
  id: uuid,
  reporter_id: uuid,
  reported_user_id: uuid,
  task_id: uuid.nullish(),
  conversation_id: uuid.nullish(),
  reason: reportReasonSchema,
  detail: z.string().nullish(),
  status: z.enum(['open', 'reviewing', 'actioned', 'dismissed']),
  created_at: timestamp,
});

export const disputeSchema = z.object({
  id: uuid,
  task_id: uuid,
  raised_by: uuid,
  reason: z.string(),
  description: z.string().nullish(),
  status: z.enum([
    'open',
    'under_review',
    'resolved_released',
    'resolved_refunded',
    'resolved_split',
    'dismissed',
  ]),
  resolution: z.string().nullish(),
});

// ── Inferred types ───────────────────────────────────────────────────────────

export type User = z.infer<typeof userSchema>;
export type PublicProfile = z.infer<typeof publicProfileSchema>;
export type AuthTokens = z.infer<typeof authTokensSchema>;
export type Task = z.infer<typeof taskSchema>;
export type TaskCard = z.infer<typeof taskCardSchema>;
export type TaskPhoto = z.infer<typeof taskPhotoSchema>;
export type Quote = z.infer<typeof quoteSchema>;
export type Escrow = z.infer<typeof escrowSchema>;
export type Offer = z.infer<typeof offerSchema>;
export type OfferWithWorker = z.infer<typeof offerWithWorkerSchema>;
export type Conversation = z.infer<typeof conversationSchema>;
export type Message = z.infer<typeof messageSchema>;
export type Wallet = z.infer<typeof walletSchema>;
export type Transaction = z.infer<typeof transactionSchema>;
export type PaymentMethod = z.infer<typeof paymentMethodSchema>;
export type PayoutMethod = z.infer<typeof payoutMethodSchema>;
export type Receipt = z.infer<typeof receiptSchema>;
export type Withdrawal = z.infer<typeof withdrawalSchema>;
export type Review = z.infer<typeof reviewSchema>;
export type Notification = z.infer<typeof notificationSchema>;
export type Verification = z.infer<typeof verificationSchema>;
export type Report = z.infer<typeof reportSchema>;
export type Dispute = z.infer<typeof disputeSchema>;
export type TaskStatus = z.infer<typeof taskStatusSchema>;
export type OfferStatus = z.infer<typeof offerStatusSchema>;
export type RoleIntent = z.infer<typeof roleIntentSchema>;
export type VerificationStatus = z.infer<typeof verificationStatusSchema>;
