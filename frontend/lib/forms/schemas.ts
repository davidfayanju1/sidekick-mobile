import { z } from 'zod';

import { documentTypeSchema, reportReasonSchema, timingTypeSchema } from '@/lib/api/schemas';
import { locale, stripPhone, toE164 } from '@/lib/locale';

/**
 * Form-input validation. Unlike the entity schemas in `lib/api/schemas.ts`,
 * these are strict — they are the last gate before a request leaves the device,
 * and their messages are shown verbatim under the field.
 *
 * Write messages the way a person would say them out loud. "Enter your email
 * address", not "email: Required".
 */

const password = z
  .string()
  .min(8, 'Use at least 8 characters')
  .regex(/[a-zA-Z]/, 'Include at least one letter')
  .regex(/[0-9]/, 'Include at least one number');

/** Mobile number for the active market — see `lib/locale.ts`. Stored as E.164. */
const phone = z
  .string()
  .trim()
  .transform(stripPhone)
  .refine(
    (value) => locale.phone.pattern.test(value),
    `Enter a valid mobile number, e.g. ${locale.phone.example}`
  )
  .transform(toE164);

const email = z.string().trim().toLowerCase().pipe(z.email('Enter a valid email address'));

/** Standalone, for verifying a number after sign-up rather than during it. */
export const phoneOnlySchema = z.object({ phone });

/** Sidekick handles payments and in-person work, so accounts are adults only. */
export const MINIMUM_AGE = 18;

/** Whole years elapsed, counting a birthday that hasn't come round yet as not had. */
function yearsSince(date: Date): number {
  const now = new Date();
  let age = now.getFullYear() - date.getFullYear();
  const month = now.getMonth() - date.getMonth();
  if (month < 0 || (month === 0 && now.getDate() < date.getDate())) age -= 1;
  return age;
}

/**
 * Date of birth as typed, DD/MM/YYYY → ISO `YYYY-MM-DD`.
 *
 * Parsed by hand rather than with `new Date(string)`: that accepts 31/02 and
 * rolls it into March, which would let an impossible date through. Building
 * the date from parts and checking the pieces survived catches it.
 */
const dateOfBirth = z
  .string()
  .trim()
  .regex(/^\d{2}\/\d{2}\/\d{4}$/, 'Use DD/MM/YYYY')
  .refine((value) => {
    const [day, month, year] = value.split('/').map(Number);
    const parsed = new Date(year, month - 1, day);
    return (
      parsed.getFullYear() === year && parsed.getMonth() === month - 1 && parsed.getDate() === day
    );
  }, 'That date doesn’t exist')
  .refine((value) => {
    const [day, month, year] = value.split('/').map(Number);
    return new Date(year, month - 1, day).getTime() <= Date.now();
  }, 'That date is in the future')
  .refine((value) => {
    const [day, month, year] = value.split('/').map(Number);
    return yearsSince(new Date(year, month - 1, day)) >= MINIMUM_AGE;
  }, `You must be ${MINIMUM_AGE} or over to use Sidekick`)
  .transform((value) => {
    const [day, month, year] = value.split('/');
    return `${year}-${month}-${day}`;
  });

// ── Auth ─────────────────────────────────────────────────────────────────────

export const signInSchema = z.object({
  email,
  password: z.string().min(1, 'Enter your password'),
});

export const signUpSchema = z.object({
  display_name: z
    .string()
    .trim()
    .min(2, 'Enter your name')
    .max(60, 'That name is too long')
    .refine((value) => value.includes(' '), 'Enter your first and last name'),
  email,
  phone,
  date_of_birth: dateOfBirth,
  password,
});

export const otpSchema = z.object({
  code: z
    .string()
    .trim()
    .regex(/^\d{6}$/, 'Enter the 6-digit code'),
});

export const resetRequestSchema = z.object({ email });

// ── Profile ──────────────────────────────────────────────────────────────────

export const profileSchema = z.object({
  display_name: z.string().trim().min(2, 'Enter your name').max(60, 'That name is too long'),
  bio: z.string().trim().max(500, 'Keep your bio under 500 characters').optional(),
  location: z.string().trim().max(120, 'That location is too long').optional(),
});

// ── Tasks ────────────────────────────────────────────────────────────────────

/**
 * An amount as typed ("50,000", "45.50") → integer minor units, which is what
 * the API stores. Strips the currency symbol and thousands separators first.
 */
export const toMinorUnits = z
  .string()
  .trim()
  .transform((value) => value.replace(new RegExp(`[${locale.currency},\\s]`, 'g'), ''))
  .refine((value) => /^\d+(\.\d{1,2})?$/.test(value), 'Enter an amount')
  .transform((value) => Math.round(parseFloat(value) * locale.minorUnits));

/** Smallest amount the product accepts anywhere — 5 major units. */
const MINIMUM = 5 * locale.minorUnits;

export const taskDraftSchema = z
  .object({
    title: z
      .string()
      .trim()
      .min(10, 'Give your task a clearer title — at least 10 characters')
      .max(80, 'Keep the title under 80 characters'),
    category: z.string().min(1, 'Pick a category'),
    description: z
      .string()
      .trim()
      .min(20, 'Add a little more detail — at least 20 characters')
      .max(2000, 'Keep the description under 2000 characters'),
    budget: toMinorUnits.refine(
      (amount) => amount >= MINIMUM,
      `Budgets start at ${locale.currency}5`
    ),
    location_approx: z.string().trim().min(1, 'Add the area this task is in'),
    location_exact: z.string().trim().min(1, 'Add the full address'),
    timing_type: timingTypeSchema,
    scheduled_for: z.string().nullish(),
    flexible_from: z.string().nullish(),
    flexible_to: z.string().nullish(),
  })
  // The API enforces these pairings too; catching them here means the user
  // never round-trips to the server to be told they missed a date.
  .refine((task) => task.timing_type !== 'specific_date' || !!task.scheduled_for, {
    error: 'Pick a date and time',
    path: ['scheduled_for'],
  })
  .refine(
    (task) => task.timing_type !== 'flexible_range' || (!!task.flexible_from && !!task.flexible_to),
    { error: 'Pick a start and end date', path: ['flexible_from'] }
  );

// ── Offers ───────────────────────────────────────────────────────────────────

/**
 * An offer. `message` is an optional pitch at the poster's budget, but becomes
 * mandatory on a counter — `offers.go` rejects a counter with an empty message
 * as a 400, so catching it here saves a pointless round trip.
 */
export const offerSchema = z
  .object({
    amount: toMinorUnits.refine(
      (amount) => amount >= MINIMUM,
      `Offers start at ${locale.currency}5`
    ),
    message: z.string().trim().max(300, 'Keep your pitch under 300 characters').optional(),
    /** The poster's asking price, so we can tell a counter from an at-budget offer. */
    budget: z.number().int().optional(),
  })
  .refine((offer) => offer.budget == null || offer.amount === offer.budget || !!offer.message, {
    error: 'Explain why you’re offering a different price',
    path: ['message'],
  });

export const counterSchema = z.object({
  amount: toMinorUnits.refine(
    (amount) => amount >= MINIMUM,
    `Counters start at ${locale.currency}5`
  ),
});

// ── Messaging ────────────────────────────────────────────────────────────────

export const messageSchema = z.object({
  body: z.string().trim().min(1, 'Type a message').max(2000, 'That message is too long'),
});

// ── Money ────────────────────────────────────────────────────────────────────

/** Luhn check — catches a mistyped digit before the payment provider does. */
function luhn(digits: string): boolean {
  let sum = 0;
  let double = false;
  for (let i = digits.length - 1; i >= 0; i--) {
    let digit = Number(digits[i]);
    if (double) {
      digit *= 2;
      if (digit > 9) digit -= 9;
    }
    sum += digit;
    double = !double;
  }
  return sum % 10 === 0;
}

export const cardSchema = z.object({
  number: z
    .string()
    .transform((value) => value.replace(/\s/g, ''))
    .refine((value) => /^\d{13,19}$/.test(value), 'Enter a valid card number')
    .refine(luhn, 'That card number doesn’t look right'),
  name: z.string().trim().min(2, 'Enter the name on the card'),
  expiry: z
    .string()
    .trim()
    .regex(/^(0[1-9]|1[0-2])\/\d{2}$/, 'Use MM/YY')
    .refine((value) => {
      const [month, year] = value.split('/').map(Number);
      const expiry = new Date(2000 + year, month, 0, 23, 59, 59);
      return expiry.getTime() > Date.now();
    }, 'That card has expired'),
  cvv: z.string().regex(/^\d{3,4}$/, 'Enter the 3-digit code on the back'),
});

export const bankDetailsSchema = z.object({
  bank_name: z.string().min(1, 'Choose your bank'),
  account_number: z
    .string()
    .trim()
    .refine(
      (value) => new RegExp(`^\\d{${locale.bank.accountLength}}$`).test(value),
      locale.bank.label
    ),
  account_name: z.string().trim().min(2, 'Enter the name on the account'),
});

export const withdrawalSchema = z.object({
  amount: toMinorUnits.refine(
    (amount) => amount >= locale.minorUnits,
    `Withdraw at least ${locale.currency}1`
  ),
});

// ── Trust & safety ───────────────────────────────────────────────────────────

export const verificationSchema = z.object({
  document_type: documentTypeSchema,
  document_url: z.string().min(1, 'Upload a photo of your document'),
});

export const reportSchema = z.object({
  reason: reportReasonSchema,
  detail: z.string().trim().max(1000, 'Keep this under 1000 characters').optional(),
});

export const reviewSchema = z.object({
  rating: z.number().int().min(1, 'Pick a rating').max(5),
  body: z.string().trim().max(500, 'Keep your review under 500 characters').optional(),
});

export const disputeSchema = z.object({
  reason: z.string().min(1, 'Choose a reason'),
  description: z
    .string()
    .trim()
    .min(20, 'Tell us what happened — at least 20 characters')
    .max(2000, 'Keep this under 2000 characters'),
});

export type SignInInput = z.input<typeof signInSchema>;
export type SignUpInput = z.input<typeof signUpSchema>;
export type TaskDraftInput = z.input<typeof taskDraftSchema>;
export type CardInput = z.input<typeof cardSchema>;
export type BankDetailsInput = z.input<typeof bankDetailsSchema>;
