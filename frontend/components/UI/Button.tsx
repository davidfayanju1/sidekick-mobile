import * as React from 'react';
import { View, type LayoutChangeEvent, type StyleProp, type ViewStyle } from 'react-native';
import Animated, {
  Easing,
  interpolate,
  useAnimatedProps,
  useAnimatedStyle,
  useDerivedValue,
  withDelay,
  withSequence,
  withSpring,
  withTiming,
} from 'react-native-reanimated';
import Svg, { Path } from 'react-native-svg';

import { PressableScale } from '@/components/UI/PressableScale';
import { Spinner } from '@/components/UI/Spinner';
import Text from '@/components/UI/Text';
import { tw } from '@/lib/tw';
import { elevation } from '@/theme/elevation';
import { colors } from '@/theme/palette';

const AnimatedPath = Animated.createAnimatedComponent(Path);

export type ButtonVariant = 'primary' | 'secondary' | 'outline' | 'ghost' | 'danger';
export type ButtonSize = 'sm' | 'md' | 'lg';

interface Palette {
  background: string;
  label: string;
  border?: string;
  raised?: boolean;
}

const VARIANTS: Record<ButtonVariant, Palette> = {
  primary: { background: colors.primary, label: colors.white, raised: true },
  secondary: { background: colors.primaryTint, label: colors.primary },
  outline: { background: 'transparent', label: colors.primary, border: colors.gray300 },
  ghost: { background: 'transparent', label: colors.primary },
  danger: { background: colors.danger, label: colors.white, raised: true },
};

const HEIGHTS: Record<ButtonSize, number> = { sm: 38, md: 48, lg: 54 };
const SPINNER: Record<ButtonSize, number> = { sm: 15, md: 18, lg: 20 };

const CHECK_PATH = 'M4 11.5 L9 16.5 L19 6';
const CHECK_LENGTH = 26;

/** How long the tick holds before the button reopens. */
const SUCCESS_HOLD_MS = 900;

/**
 * The collapse. Low stiffness and high damping so the pill *draws in* rather
 * than snapping — a fast collapse reads as a glitch, not a transition.
 */
const COLLAPSE_SPRING = { damping: 26, stiffness: 180, mass: 0.9 } as const;

export interface ButtonProps {
  label: string;
  onPress?: () => void;
  variant?: ButtonVariant;
  size?: ButtonSize;
  /** Collapses the button to a circle and spins. */
  loading?: boolean;
  disabled?: boolean;
  /** Plays a drawn tick inside the circle, then reopens to the label. */
  succeeded?: boolean;
  /** Rendered left of the label. Hidden while loading. */
  icon?: React.ReactNode;
  fullWidth?: boolean;
  /** Keeps the full width and just swaps in a spinner, without collapsing. */
  collapseOnLoad?: boolean;
  /** Applied to the wrapper, so margins hold their place while the pill shrinks. */
  style?: StyleProp<ViewStyle>;
}

/**
 * The app's CTA.
 *
 * Loading collapses the pill down to a circle the diameter of its own height,
 * with the spinner at the centre — the button becomes the loader rather than
 * containing one. Three things make it read as one motion instead of three:
 *
 *  1. **The label leaves first.** It's fully faded by the time the width has
 *     travelled a third of the way, so nothing is ever seen being squeezed.
 *  2. **The wrapper holds the space.** The pill shrinks inside a full-width
 *     wrapper, so neighbouring rows never shift.
 *  3. **Success is drawn, not popped.** The tick strokes itself inside the
 *     circle, holds, and only then does the pill reopen.
 */
export function Button({
  label,
  onPress,
  variant = 'primary',
  size = 'md',
  loading = false,
  disabled = false,
  succeeded = false,
  icon,
  fullWidth = true,
  collapseOnLoad = true,
  style,
}: ButtonProps) {
  const palette = VARIANTS[variant];
  const height = HEIGHTS[size];
  const isBusy = loading || succeeded;
  const isInert = disabled || isBusy;

  // The pill's resting width, measured from the wrapper rather than the pill.
  // Measuring the pill itself would feed its own animated width back in.
  const [restWidth, setRestWidth] = React.useState<number | null>(null);

  const onWrapperLayout = React.useCallback((event: LayoutChangeEvent) => {
    const { width } = event.nativeEvent.layout;
    // Sub-pixel jitter on rotation shouldn't retrigger a render.
    setRestWidth((current) =>
      current !== null && Math.abs(current - width) < 1 ? current : width
    );
  }, []);

  const canCollapse = collapseOnLoad && restWidth !== null && restWidth > height;

  // 0 = full pill with label, 1 = circle with spinner.
  const collapse = useDerivedValue(() =>
    withSpring(isBusy && canCollapse ? 1 : 0, COLLAPSE_SPRING)
  );

  // Kept separate from `collapse` so a non-collapsing button still cross-fades.
  const busy = useDerivedValue(() =>
    withSpring(isBusy ? 1 : 0, { damping: 20, stiffness: 240, mass: 0.6 })
  );

  const check = useDerivedValue(() =>
    succeeded
      ? withSequence(
          withTiming(1, { duration: 340, easing: Easing.bezier(0.2, 0.8, 0.2, 1) }),
          withDelay(SUCCESS_HOLD_MS, withTiming(1, { duration: 0 }))
        )
      : withTiming(0, { duration: 140 })
  );

  const shapeStyle = useAnimatedStyle(() => {
    if (restWidth === null) return {};
    return {
      width: canCollapse
        ? interpolate(collapse.value, [0, 1], [restWidth, height], 'clamp')
        : restWidth,
    };
  });

  // Gone by a third of the way in, so the text is never seen being crushed.
  const labelStyle = useAnimatedStyle(() => ({
    opacity: interpolate(canCollapse ? collapse.value : busy.value, [0, 0.3], [1, 0], 'clamp'),
  }));

  // Arrives only once there's a circle to sit in.
  const overlayStyle = useAnimatedStyle(() => {
    const progress = canCollapse ? collapse.value : busy.value;
    return {
      opacity: interpolate(progress, [0.45, 1], [0, 1], 'clamp'),
      transform: [{ scale: interpolate(progress, [0.45, 1], [0.6, 1], 'clamp') }],
    };
  });

  const spinnerStyle = useAnimatedStyle(() => ({ opacity: succeeded ? 0 : 1 }));
  const tickStyle = useAnimatedStyle(() => ({ opacity: succeeded ? 1 : 0, position: 'absolute' }));

  const tickProps = useAnimatedProps(() => ({
    strokeDashoffset: CHECK_LENGTH * (1 - check.value),
  }));

  // Dim only the surface, so a disabled button greys out without dulling the
  // spinner that may be sitting on top of it.
  const surfaceStyle = useAnimatedStyle(() => ({
    opacity: withTiming(disabled && !isBusy ? 0.45 : 1, { duration: 160 }),
  }));

  return (
    <View
      onLayout={onWrapperLayout}
      style={[fullWidth ? tw`w-full` : tw`self-start`, { minHeight: height }, style]}>
      <PressableScale
        onPress={onPress}
        disabled={isInert}
        scaleTo={0.985}
        dim={false}
        haptic={isInert ? 'none' : 'medium'}
        style={[
          tw`flex-row items-center justify-center self-center overflow-hidden rounded-full px-5`,
          { height, backgroundColor: palette.background },
          // Before the first measurement the pill fills the wrapper; after it,
          // the animated width takes over.
          restWidth === null ? tw`w-full` : undefined,
          palette.border ? { borderWidth: 1, borderColor: palette.border } : undefined,
          palette.raised && !isInert ? elevation.md : undefined,
          shapeStyle,
        ]}>
        <Animated.View
          pointerEvents="none"
          style={[
            tw`absolute inset-0 rounded-full`,
            { backgroundColor: palette.background },
            surfaceStyle,
          ]}
        />

        <Animated.View
          pointerEvents="none"
          style={[tw`absolute left-0 right-0 flex-row items-center justify-center`, labelStyle]}>
          {icon ? <View style={tw`mr-2`}>{icon}</View> : null}
          <Text
            variant={size === 'sm' ? 'body' : 'subhead'}
            fontWeight="bold"
            numberOfLines={1}
            classN={`text-[${palette.label}]`}>
            {label}
          </Text>
        </Animated.View>

        <Animated.View style={[tw`absolute items-center justify-center`, overlayStyle]}>
          <Animated.View style={spinnerStyle}>
            <Spinner size={SPINNER[size]} color={palette.label} />
          </Animated.View>
          <Animated.View style={tickStyle}>
            <Svg width={SPINNER[size] + 6} height={SPINNER[size] + 6} viewBox="0 0 24 24">
              <AnimatedPath
                d={CHECK_PATH}
                stroke={palette.label}
                strokeWidth={2.6}
                strokeLinecap="round"
                strokeLinejoin="round"
                fill="none"
                strokeDasharray={CHECK_LENGTH}
                animatedProps={tickProps}
              />
            </Svg>
          </Animated.View>
        </Animated.View>
      </PressableScale>
    </View>
  );
}

export default Button;
