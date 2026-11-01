/**
 * Single source of truth for type.
 *
 * Seven steps with real jumps between them. The previous design used 16 ad-hoc
 * sizes with 79% of all text crowded between 11 and 15px, so nothing read as a
 * heading. Each step here is far enough from its neighbours to be seen.
 *
 * Use `<Text variant="...">` rather than `fontSize` — the variant carries the
 * matching default weight, which is what actually creates hierarchy.
 */

export const fontSize = {
  /** Uppercase category labels, badge text. */
  micro: 11,
  /** Captions, metadata, timestamps. */
  caption: 12,
  /** Default body copy. */
  body: 14,
  /** Card titles, list-row headings, emphasised values. */
  subhead: 17,
  /** Section headings. */
  title: 22,
  /** Screen titles. */
  headline: 28,
  /** Hero figures — wallet balance, headline stats. */
  display: 34,
} as const;

export type TextVariant = keyof typeof fontSize;

export type FontWeight = 'light' | 'normal' | 'medium' | 'bold' | 'black';

/**
 * The weight each variant uses unless overridden.
 *
 * Body-level text is deliberately `normal`. Emphasis only means something while
 * it stays scarce, so bold is reserved for `subhead` and above.
 */
export const variantWeight: Record<TextVariant, FontWeight> = {
  micro: 'medium',
  caption: 'normal',
  body: 'normal',
  subhead: 'medium',
  title: 'bold',
  headline: 'bold',
  display: 'black',
};

/** Line heights as a multiplier of font size — tighter as type gets larger. */
export const lineHeight: Record<TextVariant, number> = {
  micro: 1.35,
  caption: 1.4,
  body: 1.5,
  subhead: 1.4,
  title: 1.25,
  headline: 1.2,
  display: 1.15,
};

/** Letter spacing, in points. Large type needs tightening; small caps need air. */
export const letterSpacing: Record<TextVariant, number> = {
  micro: 0.6,
  caption: 0,
  body: 0,
  subhead: -0.2,
  title: -0.4,
  headline: -0.6,
  display: -0.9,
};
