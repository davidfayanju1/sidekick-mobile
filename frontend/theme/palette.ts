/**
 * Single source of truth for the app's colour palette.
 *
 * Every colour used in `app/` and `components/` lives here. Do not hardcode hex
 * values in screens or components — add a token here and reference it instead.
 *
 * Naming is by role, not by hue, so a value can change without its name lying.
 *
 * NOTE: this palette is light-mode only, mirroring the current design. Tokens are
 * referenced through `colors.*` at every call site, so a dark set can be layered in
 * later (see `theme/colors.ts` for the platform/system palette used by nav chrome)
 * without touching the screens.
 */

export const colors = {
  // ---------------------------------------------------------------------------
  // Brand — the teal family
  // ---------------------------------------------------------------------------
  /** Primary brand teal: CTAs, active states, links. */
  brand: '#489A9F',
  /** Darker teal, for text that sits on a light teal tint. */
  brandDark: '#3F7A7D',
  /** Teal border, e.g. outlined checkboxes. */
  brandBorder: '#8FBFC2',
  /** Brand teal at rest/disabled. */
  brandDisabled: '#CFE3E4',
  /** Teal tint background. */
  brandTint: '#DCEEEF',
  /** Teal tint for "matched" status chips. */
  brandTintAlt: '#DCEDEE',
  /** Faintest teal tint, e.g. summary cards. */
  brandTintSubtle: '#EAF5F5',
  /** Teal-tinted input background, e.g. chat search. */
  brandTintMuted: '#EFF7F6',
  /** Outgoing chat bubble. */
  brandBubble: '#DDF2F1',

  // ---------------------------------------------------------------------------
  // Text
  // ---------------------------------------------------------------------------
  /** Primary text and dark icons. */
  textPrimary: '#16181A',
  /** Long-form body copy. */
  textBody: '#4B4F54',
  /** Secondary/supporting copy. */
  textSecondary: '#6B7075',
  /** Muted labels and captions. */
  textMuted: '#9AA0A6',
  /** Muted copy on auth screens. */
  textMutedAlt: '#8C9296',
  /** Lowest-emphasis copy, e.g. inactive resend timer. */
  textSubtle: '#B7B7B7',
  /** TextInput placeholder. */
  textPlaceholder: '#A4A0A0',
  /** Inactive step/checklist label. */
  textDisabled: '#4B5054',
  /** Text on a brand-coloured surface. */
  textOnBrand: '#FFFFFF',
  /** Interactive link, e.g. "Resend code". */
  textLink: '#3B82F6',
  /** Link on the not-found screen. */
  textLinkAlt: '#2E78B7',

  // ---------------------------------------------------------------------------
  // Icons
  // ---------------------------------------------------------------------------
  /** Default back-chevron / header icon. */
  iconPrimary: '#000',
  /** Inactive tab bar glyph. */
  iconTabInactive: '#8C9397',
  /** Form affordance icon, e.g. password eye. */
  iconForm: '#575555',
  /** Empty-state icon, e.g. image placeholder. */
  iconPlaceholder: '#D9DCE0',

  // ---------------------------------------------------------------------------
  // Surfaces
  // ---------------------------------------------------------------------------
  /** Default raised surface / selected row. */
  surface: '#F5F6F7',
  /** Recessed surface and hairline dividers. */
  surfaceSunken: '#F0F1F2',
  /** Pure white cards. */
  surfaceWhite: '#FFFFFF',
  /** Dark surface, e.g. splash screen. */
  surfaceInverse: '#19262E',

  // ---------------------------------------------------------------------------
  // Borders
  // ---------------------------------------------------------------------------
  /** Default input/card border. */
  border: '#DADADA',
  /** Lighter border, also the empty-star fill. */
  borderLight: '#E1E4EA',
  /** Higher-contrast border, used on auth inputs. */
  borderStrong: '#D1D4D5',
  /** Horizontal rule. */
  borderDivider: '#E3E5E8',
  /** Card outline. */
  borderCard: '#EDEEF0',
  /** Unselected radio ring. */
  borderRadio: '#C7CBCF',

  // ---------------------------------------------------------------------------
  // Status
  // ---------------------------------------------------------------------------
  success: '#2F9E56',
  successBg: '#DCF5E3',
  warning: '#D97A55',
  /** Destructive action, e.g. "Delete account". */
  danger: '#C94B34',
  /** Declined/negative text and amounts. */
  dangerText: '#D1573B',
  disputeBg: '#FBE2DC',
  pendingText: '#B5762E',
  pendingBg: '#F5E6D3',
  scheduledText: '#3B6FD1',
  scheduledBg: '#DCE8FB',
  /** Filled rating star. */
  star: '#F5A623',
  /** Unread message badge. */
  unread: '#E5484D',

  // ---------------------------------------------------------------------------
  // Accents
  // ---------------------------------------------------------------------------
  accentOrange: '#FF7A55',
  accentOrangeDeep: '#D1704F',
  /** Sand/tan avatar and chip background. */
  accentSand: '#F0DCC8',
  /** Category accent, e.g. Handyman. */
  accentPurple: '#8B5CF6',
} as const;

export type ColorToken = keyof typeof colors;
