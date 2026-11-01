/**
 * Everything that changes when the app ships to a different market.
 *
 * The backend stores money as an integer in minor units and doesn't care which
 * currency that is, so the symbol, the phone format and the bank-account rules
 * are all client-side presentation. Keeping them here means adding a market is
 * one entry in this file, not a grep across forty screens.
 *
 * Select with EXPO_PUBLIC_LOCALE=NG|GB. Nothing else in the app should hardcode
 * a currency symbol or a phone pattern.
 */

export interface LocaleConfig {
  key: 'NG' | 'GB';
  /** Symbol shown to users. */
  currency: string;
  /** ISO code, for anything that needs to talk to a payment provider. */
  currencyCode: string;
  /** BCP-47 tag used for number and date formatting. */
  intlLocale: string;
  /** How many minor units make one major unit. 100 everywhere we ship so far. */
  minorUnits: number;
  phone: {
    /** Accepts local and international forms, spaces already stripped. */
    pattern: RegExp;
    /** E.164 country prefix, without the plus. */
    dialCode: string;
    /** Shown in the validation message. */
    example: string;
  };
  bank: {
    /** Digits in a domestic account number. */
    accountLength: number;
    /** What people call it locally — appears in the error message. */
    label: string;
  };
}

const LOCALES: Record<LocaleConfig['key'], LocaleConfig> = {
  NG: {
    key: 'NG',
    currency: '₦',
    currencyCode: 'NGN',
    intlLocale: 'en-NG',
    minorUnits: 100,
    phone: {
      // 0803…, 234803…, +234803… — MTN/Airtel/Glo/9mobile all start 7, 8 or 9.
      pattern: /^(\+?234|0)[789]\d{9}$/,
      dialCode: '234',
      example: '0803 123 4567',
    },
    bank: { accountLength: 10, label: 'NUBAN account numbers are 10 digits' },
  },
  GB: {
    key: 'GB',
    currency: '£',
    currencyCode: 'GBP',
    intlLocale: 'en-GB',
    minorUnits: 100,
    phone: {
      pattern: /^(\+?44|0)7\d{9}$/,
      dialCode: '44',
      example: '07700 900123',
    },
    bank: { accountLength: 8, label: 'Account numbers are 8 digits' },
  },
};

function resolve(): LocaleConfig {
  const requested = process.env.EXPO_PUBLIC_LOCALE as LocaleConfig['key'] | undefined;
  return LOCALES[requested ?? 'NG'] ?? LOCALES.NG;
}

export const locale = resolve();

/** Strips the formatting people type into a phone field. */
export function stripPhone(value: string): string {
  return value.replace(/[\s()-]/g, '');
}

/** Local or international input → E.164, e.g. "0803…" → "+234803…". */
export function toE164(value: string): string {
  const digits = stripPhone(value);
  if (digits.startsWith('0')) return `+${locale.phone.dialCode}${digits.slice(1)}`;
  return `+${digits.replace(/^\+/, '')}`;
}
