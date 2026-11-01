/**
 * Every query key in the app, in one place.
 *
 * Keys are hierarchical so a mutation can invalidate a whole branch:
 * `invalidateQueries({ queryKey: qk.tasks.all })` catches the list, the detail
 * and the drafts without naming them individually.
 */
export const qk = {
  auth: {
    me: ['auth', 'me'] as const,
  },
  users: {
    profile: (id: string) => ['users', id] as const,
    reviews: (id: string) => ['users', id, 'reviews'] as const,
  },
  tasks: {
    all: ['tasks'] as const,
    detail: (id: string) => ['tasks', id] as const,
    escrow: (id: string) => ['tasks', id, 'escrow'] as const,
    drafts: ['tasks', 'drafts'] as const,
    quote: (budget: number) => ['tasks', 'quote', budget] as const,
  },
  discovery: {
    feed: (filters: unknown) => ['discovery', 'feed', filters] as const,
    search: (query: string, filters: unknown) => ['discovery', 'search', query, filters] as const,
  },
  offers: {
    forTask: (taskId: string) => ['offers', 'task', taskId] as const,
    mine: ['offers', 'mine'] as const,
  },
  messaging: {
    conversations: ['conversations'] as const,
    messages: (conversationId: string) => ['conversations', conversationId, 'messages'] as const,
    forTask: (taskId: string) => ['conversations', 'task', taskId] as const,
    blocks: ['blocks'] as const,
  },
  finance: {
    wallet: ['finance', 'wallet'] as const,
    transactions: ['finance', 'transactions'] as const,
    receipt: (id: string) => ['finance', 'receipt', id] as const,
    paymentMethods: ['finance', 'payment-methods'] as const,
    payoutMethods: ['finance', 'payout-methods'] as const,
  },
  reviews: {
    mine: ['reviews', 'mine'] as const,
  },
  notifications: {
    list: ['notifications'] as const,
    unreadCount: ['notifications', 'unread-count'] as const,
    prefs: ['notifications', 'prefs'] as const,
  },
  trust: {
    reports: ['trust', 'reports'] as const,
  },
} as const;

/**
 * Idempotency key for money-moving requests.
 *
 * The API dedupes funding and withdrawals on this, so a double-tap or a retry
 * after a dropped response can't charge twice. Not security-sensitive — it only
 * has to be unique per attempt.
 */
export function idempotencyKey(): string {
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
}
