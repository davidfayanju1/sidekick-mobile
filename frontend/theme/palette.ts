/**
 * Single source of truth for the app's colour palette.
 *
 * Every colour used in `app/` and `components/` lives here. Do not hardcode hex
 * values in screens or components — add a token here and reference it instead.
 *
 * Six families: primary (navy), green, orange, danger, star, and one grey ramp.
 * Reach for a family token first; only add a new one if nothing here fits.
 *
 * NOTE: light-mode only, mirroring the current design. Every call site goes through
 * `colors.*`, so a dark set can be layered in later without touching the screens.
 * `theme/colors.ts` is a separate, platform-adaptive palette used only for nav chrome.
 */

export const colors = {
  // ---------------------------------------------------------------------------
  // Primary — navy. CTAs, active states, links, the "Scheduled" status.
  // ---------------------------------------------------------------------------
  primary: '#1B3A5F',
  /** Pressed/darker navy, and the dark splash background. */
  primaryDark: '#12283F',
  /** Mid navy for outlined controls, e.g. checkbox borders. */
  primaryMuted: '#7D93AD',
  /** Navy at rest/disabled. */
  primaryDisabled: '#B8C6D6',
  /** Navy tint background: chips, chat bubbles, highlighted rows. */
  primaryTint: '#E3EAF2',
  /** Faintest navy wash, e.g. search fields and summary cards. */
  primarySurface: '#F0F4F9',

  // ---------------------------------------------------------------------------
  // Green — success and completion.
  // ---------------------------------------------------------------------------
  green: '#2F9E56',
  greenTint: '#DCF5E3',

  // ---------------------------------------------------------------------------
  // Orange — warnings, pending states, warm accents.
  // ---------------------------------------------------------------------------
  /** Bright accent orange. */
  orange: '#FF7A55',
  /** Muted orange for warnings and secondary accents. */
  orangeMuted: '#D97A55',
  /** Deep amber for "pending" text. */
  orangeDark: '#B5762E',
  /** Sand tint: pending chips, avatar backgrounds. */
  orangeTint: '#F5E6D3',

  // ---------------------------------------------------------------------------
  // Danger — destructive actions, declined states, unread badges.
  // ---------------------------------------------------------------------------
  danger: '#C94B34',
  dangerTint: '#FBE2DC',

  // ---------------------------------------------------------------------------
  // Star — rating gold. Kept separate because no other family reads as a rating.
  // ---------------------------------------------------------------------------
  star: '#F5A623',

  // ---------------------------------------------------------------------------
  // Grey ramp — surfaces, borders and text, lightest to darkest.
  // ---------------------------------------------------------------------------
  /** Default raised surface, selected rows. */
  gray50: '#F5F6F7',
  /** Recessed surface, card outlines. */
  gray100: '#F0F1F2',
  /** Light borders, dividers, empty stars. */
  gray200: '#E1E4EA',
  /** Default border, placeholder icons. */
  gray300: '#DADADA',
  /** Lowest-emphasis text. */
  gray400: '#B7B7B7',
  /** Muted labels, captions, placeholders. */
  gray500: '#9AA0A6',
  /** Muted text on auth screens, inactive tab glyphs. */
  gray600: '#8C9296',
  /** Secondary/supporting copy. */
  gray700: '#6B7075',
  /** Body copy, form icons. */
  gray800: '#4B4F54',
  /** Primary text. */
  gray900: '#16181A',

  // ---------------------------------------------------------------------------
  // Inverse surface — toasts, tooltips, anything that sits *over* the app.
  //
  // A dark overlay reads as system chrome rather than page content, which is
  // why it survives on top of any screen. The family colours above are tuned
  // for white backgrounds and go muddy here, so these accents are lifted to
  // stay legible on near-black.
  // ---------------------------------------------------------------------------
  /** Near-black overlay surface. */
  inverse: '#17191C',
  /** Hairline that defines the overlay's edge. */
  inverseBorder: 'rgba(255,255,255,0.10)',
  /** Secondary copy on the inverse surface. */
  inverseMuted: 'rgba(255,255,255,0.64)',
  inverseGreen: '#4ADE80',
  inverseDanger: '#FF8A75',
  inverseOrange: '#FBBF24',
  inverseInfo: '#9EC5FF',

  // ---------------------------------------------------------------------------
  // Absolutes
  // ---------------------------------------------------------------------------
  black: '#000',
  white: '#FFFFFF',
} as const;

export type ColorToken = keyof typeof colors;
