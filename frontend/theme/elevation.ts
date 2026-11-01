import { Platform } from 'react-native';

import { colors } from './palette';

/**
 * Shadow levels. The previous single `CARD_SHADOW` used shadowOpacity 0.06,
 * which is effectively invisible — cards sat flat against the page.
 *
 * Tint the shadow with the brand navy rather than pure black: coloured shadows
 * read as depth, black shadows read as dirt.
 */
function shadow(opacity: number, radius: number, y: number, elevation: number) {
  return Platform.select({
    ios: {
      shadowColor: colors.primaryDark,
      shadowOpacity: opacity,
      shadowRadius: radius,
      shadowOffset: { width: 0, height: y },
    },
    default: { elevation },
  }) as object;
}

export const elevation = {
  /** Hairline lift — chips, inputs at rest. */
  sm: shadow(0.06, 6, 1, 1),
  /** Default card. */
  md: shadow(0.1, 14, 4, 3),
  /** Raised card, pressed-state target, sticky headers. */
  lg: shadow(0.14, 24, 8, 6),
  /** Modals and sheets. */
  xl: shadow(0.18, 32, 12, 12),
} as const;
