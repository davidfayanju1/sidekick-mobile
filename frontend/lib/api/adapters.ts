import type {
  BrowsableTask,
  Task as DisplayTask,
  TaskStatus as DisplayStatus,
} from '@/components/UI/TaskParts';
import type {
  Transaction as DisplayTransaction,
  TransactionType as DisplayTransactionType,
} from '@/app/payment-history';
import type {
  Conversation,
  Task,
  TaskCard,
  TaskStatus,
  Transaction as ApiTransaction,
} from '@/lib/api/schemas';
import { distance, money, timeAgo, timing } from '@/lib/format';
import { locale } from '@/lib/locale';

/**
 * API shapes → the display models the screens were built against.
 *
 * The UI's `Task` carries presentation (`statusLabel`, `meta`, `payIn`) that
 * the API deliberately doesn't. Rather than push formatting into forty screens
 * or reshape the API, the translation lives here — one file to change when
 * either side moves.
 */

/**
 * The database tracks twelve task states; the UI has five visual treatments.
 * Several states collapse onto one pill because they look identical to a user —
 * a task cancelled by either side is just "cancelled" on a card.
 *
 * These keys are exhaustive against `taskStatusSchema`, so TypeScript will
 * break this file if the backend adds a state — which is the point. A missing
 * key used to fall through and print raw snake_case at the user.
 */
const STATUS_MAP: Record<TaskStatus, DisplayStatus> = {
  draft: 'scheduled',
  open: 'offers',
  assigned: 'in_progress',
  completed_pending_confirmation: 'in_progress',
  completed: 'in_progress',
  resolved_released: 'in_progress',
  resolved_refunded: 'scheduled',
  resolved_split: 'scheduled',
  cancelled_by_poster: 'scheduled',
  cancelled_by_worker: 'scheduled',
  expired: 'scheduled',
  disputed: 'dispute',
};

const STATUS_LABEL: Record<TaskStatus, string> = {
  draft: 'Draft',
  open: 'Open',
  assigned: 'Matched',
  completed_pending_confirmation: 'Awaiting Confirmation',
  completed: 'Completed',
  resolved_released: 'Paid',
  resolved_refunded: 'Refunded',
  resolved_split: 'Partially Refunded',
  cancelled_by_poster: 'Cancelled',
  cancelled_by_worker: 'Cancelled by Sidekick',
  expired: 'Expired',
  disputed: 'Disputed',
};

/** Which step of the five-stage tracker a status sits on. */
const STATUS_STEP: Record<TaskStatus, number> = {
  draft: 1,
  open: 1,
  assigned: 2,
  completed_pending_confirmation: 4,
  completed: 4,
  resolved_released: 5,
  resolved_refunded: 5,
  resolved_split: 5,
  cancelled_by_poster: 1,
  cancelled_by_worker: 1,
  expired: 1,
  disputed: 4,
};

/** Terminal states — the task is over, however it ended. */
const FINISHED: readonly TaskStatus[] = [
  'completed',
  'resolved_released',
  'resolved_refunded',
  'resolved_split',
  'cancelled_by_poster',
  'cancelled_by_worker',
  'expired',
];

/** The one-line summary under a task's title, built from whatever the API gave us. */
function metaLine(task: Task): string {
  const parts: string[] = [];
  if (task.status === 'open') {
    const count = task.offer_count ?? 0;
    parts.push(count === 0 ? 'No offers yet' : `${count} offer${count === 1 ? '' : 's'}`);
  }
  if (task.created_at) parts.push(`Posted ${timeAgo(task.created_at)}`);
  if (task.location_approx) parts.push(task.location_approx);
  return parts.join(' • ');
}

export function toDisplayTask(task: Task): DisplayTask {
  const offerCount = task.offer_count ?? 0;

  return {
    id: task.id,
    category: task.category ?? 'Other',
    status: STATUS_MAP[task.status] ?? 'scheduled',
    statusLabel:
      task.status === 'open' && offerCount > 0
        ? `${offerCount} Offer${offerCount === 1 ? '' : 's'}`
        : (STATUS_LABEL[task.status] ?? task.status),
    title: task.title,
    meta: metaLine(task),
    currentStep: STATUS_STEP[task.status],
    offersText:
      task.status === 'open' && offerCount > 0
        ? `${offerCount} Sidekick${offerCount === 1 ? '' : 's'} made offers. Tap to view`
        : undefined,
    disputeText: task.status === 'disputed' ? 'Under review by our team' : undefined,
    price: money(task.budget),
    offerCount,
    description: task.description ?? undefined,
    etaWindow: timing(task),
  };
}

export function toBrowsableTask(card: TaskCard): BrowsableTask {
  const name = 'Hero';
  return {
    id: card.id,
    category: card.category ?? 'Other',
    title: card.title,
    description: card.description ?? '',
    budget: money(card.budget),
    location: card.location_approx ?? '',
    // The feed only exposes an approximate distance — exact location is
    // withheld until an offer is accepted.
    distanceLabel: distance(card.distance_km),
    postedAgo: timeAgo(card.created_at),
    etaWindow: timing(card),
    heroName: name,
    heroInitial: name.charAt(0),
    heroRating: card.poster?.rating_avg ?? 0,
  };
}

/**
 * A conversation row for the chats list.
 *
 * `GET /conversations` returns ids, the task title and unread counts, but not
 * the other person's name — so the row leads with the task, which is what the
 * conversation is actually about, and the counterpart's status pill comes from
 * the task those ids belong to.
 */
export function toChatPreview(
  conversation: Conversation,
  task: Task | undefined
): {
  id: string;
  taskId: string;
  title: string;
  initial: string;
  lastMessage: string;
  time: string;
  unread: number;
  statusLabel: string | undefined;
  isCompleted: boolean;
} {
  const title = conversation.task_title ?? task?.title ?? 'Task';
  const status = task?.status;

  return {
    id: conversation.id,
    taskId: conversation.task_id,
    title,
    initial: title.charAt(0).toUpperCase(),
    lastMessage: conversation.last_message_body ?? 'No messages yet',
    time: conversation.last_message_at ? timeAgo(conversation.last_message_at) : '',
    unread: conversation.unread_count ?? 0,
    statusLabel: status ? STATUS_LABEL[status] : undefined,
    isCompleted: !!status && FINISHED.includes(status),
  };
}

/**
 * A ledger entry, from the signed-in user's point of view.
 *
 * The API records a transaction once, with a payer and a payee; whether it
 * reads as money in or money out depends entirely on which side you're on.
 * `release` is the clearest case — the same row is the Hero's payment and the
 * Sidekick's earning.
 */
export function toDisplayTransaction(
  transaction: ApiTransaction,
  currentUserId: string | undefined
): DisplayTransaction {
  const isPayee = !!currentUserId && transaction.payee_id === currentUserId;

  const type: DisplayTransactionType =
    transaction.type === 'withdrawal'
      ? 'withdrawal'
      : transaction.type === 'refund'
        ? 'refund'
        : transaction.type === 'escrow_hold'
          ? 'payment'
          : isPayee
            ? 'earning'
            : 'payment';

  // Money leaving the account shows negative; money arriving shows positive.
  const outgoing = type === 'payment' || type === 'withdrawal';

  return {
    id: transaction.id,
    type,
    title: TRANSACTION_TITLE[transaction.type] ?? 'Transaction',
    date: new Date(transaction.created_at).toLocaleDateString(locale.intlLocale, {
      day: 'numeric',
      month: 'short',
      year: 'numeric',
    }),
    amount: outgoing ? -Math.abs(transaction.amount) : Math.abs(transaction.amount),
    status:
      transaction.status === 'succeeded'
        ? 'completed'
        : transaction.status === 'failed'
          ? 'failed'
          : 'pending',
  };
}

const TRANSACTION_TITLE: Record<string, string> = {
  escrow_hold: 'Escrow funded',
  release: 'Payment released',
  refund: 'Refund',
  withdrawal: 'Withdrawal',
  cancellation_compensation: 'Cancellation compensation',
};
