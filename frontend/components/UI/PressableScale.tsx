import * as Haptics from 'expo-haptics';
import React from 'react';
import { Pressable, type PressableProps, type ViewStyle, type StyleProp } from 'react-native';
import Animated, {
  useAnimatedStyle,
  useSharedValue,
  withSpring,
  withTiming,
} from 'react-native-reanimated';

const AnimatedPressable = Animated.createAnimatedComponent(Pressable);

const SPRING = { damping: 18, stiffness: 320, mass: 0.5 };

type Haptic = 'light' | 'medium' | 'heavy' | 'selection' | 'none';

const HAPTIC: Record<Exclude<Haptic, 'none'>, () => void> = {
  light: () => Haptics.impactAsync(Haptics.ImpactFeedbackStyle.Light),
  medium: () => Haptics.impactAsync(Haptics.ImpactFeedbackStyle.Medium),
  heavy: () => Haptics.impactAsync(Haptics.ImpactFeedbackStyle.Heavy),
  selection: () => Haptics.selectionAsync(),
};

export interface PressableScaleProps extends Omit<PressableProps, 'style'> {
  /** How far to shrink on press. 1 disables the scale. */
  scaleTo?: number;
  /** Dim slightly while held. */
  dim?: boolean;
  /** Haptic fired on press-in. `none` to stay silent. */
  haptic?: Haptic;
  style?: StyleProp<ViewStyle>;
  children?: React.ReactNode;
}

/**
 * A Pressable that physically responds — shrinks, dims and taps back.
 *
 * Drop-in replacement for `Pressable` on anything tappable: cards, buttons,
 * list rows, icon targets.
 */
export function PressableScale({
  scaleTo = 0.97,
  dim = true,
  haptic = 'light',
  style,
  children,
  onPressIn,
  onPressOut,
  disabled,
  ...props
}: PressableScaleProps) {
  const pressed = useSharedValue(0);

  const animatedStyle = useAnimatedStyle(() => ({
    transform: [{ scale: withSpring(1 - pressed.value * (1 - scaleTo), SPRING) }],
    opacity: dim ? withTiming(1 - pressed.value * 0.12, { duration: 90 }) : 1,
  }));

  return (
    <AnimatedPressable
      disabled={disabled}
      style={[style, animatedStyle]}
      onPressIn={(e) => {
        pressed.value = 1;
        if (!disabled && haptic !== 'none') HAPTIC[haptic]();
        onPressIn?.(e);
      }}
      onPressOut={(e) => {
        pressed.value = 0;
        onPressOut?.(e);
      }}
      {...props}>
      {children}
    </AnimatedPressable>
  );
}

export default PressableScale;
