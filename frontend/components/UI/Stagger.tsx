import React from 'react';
import { type StyleProp, type ViewStyle } from 'react-native';
import Animated, { FadeInDown } from 'react-native-reanimated';

/** Base duration and per-item offset for list entrances. */
const DURATION = 340;
const STEP = 45;

export interface StaggerItemProps {
  /** Position in the list — drives the delay. */
  index?: number;
  /** Extra delay before the whole group starts, in ms. */
  delay?: number;
  /** Gap between consecutive items, in ms. */
  step?: number;
  style?: StyleProp<ViewStyle>;
  children: React.ReactNode;
}

/**
 * Fades and lifts a list item into place, offset by its index.
 *
 * Screens felt static because content simply appeared. A short staggered
 * entrance gives the eye an order to follow and makes lists feel alive.
 *
 * Capped at 8 steps so long lists don't leave later rows visibly waiting.
 */
export function StaggerItem({
  index = 0,
  delay = 0,
  step = STEP,
  style,
  children,
}: StaggerItemProps) {
  return (
    <Animated.View
      style={style}
      entering={FadeInDown.delay(delay + Math.min(index, 8) * step)
        .duration(DURATION)
        .springify()
        .damping(20)}>
      {children}
    </Animated.View>
  );
}

export default StaggerItem;
