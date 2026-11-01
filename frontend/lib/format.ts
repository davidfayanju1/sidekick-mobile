import { locale } from '@/lib/locale';

/**
 * Display formatting. Kept in one file so an amount never renders as "45.00" on
 * one screen and "4500" on another.
 *
 * The currency symbol and number locale come from `lib/locale.ts` — never
 * hardcode either at a call site.
 */

/**
 * Minor units → display string, e.g. 5000000 → "₦50,000", 4550 → "£45.50".
 * Whole amounts drop the decimals; fractional ones keep them.
 */
export function money(minor: number | null | undefined): string {
  if (minor == null) return `${locale.currency}0`;
  const major = minor / locale.minorUnits;
  const fractional = minor % locale.minorUnits !== 0;
  return `${locale.currency}${major.toLocaleString(locale.intlLocale, {
    minimumFractionDigits: fractional ? 2 : 0,
    maximumFractionDigits: 2,
  })}`;
}

/** Always two decimals — for ledgers and receipts, where columns must line up. */
export function moneyExact(minor: number | null | undefined): string {
  return `${locale.currency}${((minor ?? 0) / locale.minorUnits).toLocaleString(locale.intlLocale, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })}`;
}

const MINUTE = 60_000;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;
const WEEK = 7 * DAY;

/** "Just now", "12 mins ago", "3 days ago", then a date. */
export function timeAgo(iso: string | null | undefined): string {
  if (!iso) return '';
  const elapsed = Date.now() - new Date(iso).getTime();

  if (elapsed < MINUTE) return 'Just now';
  if (elapsed < HOUR) {
    const mins = Math.floor(elapsed / MINUTE);
    return `${mins} min${mins === 1 ? '' : 's'} ago`;
  }
  if (elapsed < DAY) {
    const hours = Math.floor(elapsed / HOUR);
    return `${hours} hour${hours === 1 ? '' : 's'} ago`;
  }
  if (elapsed < WEEK) {
    const days = Math.floor(elapsed / DAY);
    return `${days} day${days === 1 ? '' : 's'} ago`;
  }
  return new Date(iso).toLocaleDateString(locale.intlLocale, { day: 'numeric', month: 'short' });
}

/** Chat timestamps: time today, weekday this week, date beyond that. */
export function messageTime(iso: string | null | undefined): string {
  if (!iso) return '';
  const date = new Date(iso);
  const elapsed = Date.now() - date.getTime();

  if (elapsed < DAY && date.getDate() === new Date().getDate()) {
    return date.toLocaleTimeString(locale.intlLocale, { hour: '2-digit', minute: '2-digit' });
  }
  if (elapsed < WEEK) return date.toLocaleDateString(locale.intlLocale, { weekday: 'short' });
  return date.toLocaleDateString(locale.intlLocale, { day: 'numeric', month: 'short' });
}

/** "2.4 km away", "350 m away". */
export function distance(km: number | null | undefined): string {
  if (km == null) return '';
  if (km < 1) return `${Math.round(km * 1000)} m away`;
  return `${km.toFixed(1)} km away`;
}

/** Turns a task's timing fields into one human line. */
export function timing(task: {
  timing_type?: string | null;
  scheduled_for?: string | null;
  flexible_from?: string | null;
  flexible_to?: string | null;
}): string {
  if (task.timing_type === 'asap') return 'As soon as possible';
  if (task.timing_type === 'specific_date' && task.scheduled_for) {
    return new Date(task.scheduled_for).toLocaleString(locale.intlLocale, {
      weekday: 'short',
      day: 'numeric',
      month: 'short',
      hour: '2-digit',
      minute: '2-digit',
    });
  }
  if (task.timing_type === 'flexible_range' && task.flexible_from && task.flexible_to) {
    const format = (value: string) =>
      new Date(value).toLocaleDateString(locale.intlLocale, { day: 'numeric', month: 'short' });
    return `${format(task.flexible_from)} – ${format(task.flexible_to)}`;
  }
  return 'Flexible';
}
