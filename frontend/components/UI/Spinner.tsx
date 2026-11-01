import * as React from 'react';
import { View } from 'react-native';
import Animated, {
  Easing,
  cancelAnimation,
  useAnimatedProps,
  useAnimatedStyle,
  useSharedValue,
  withRepeat,
  withSequence,
  withTiming,
} from 'react-native-reanimated';
import Svg, { Circle } from 'react-native-svg';

import { colors } from '@/theme/palette';

const AnimatedCircle = Animated.createAnimatedComponent(Circle);

/** One full rotation. Slow enough to read as considered, not frantic. */
const SPIN_MS = 1100;
/** The arc grows and shrinks over this period, offset from the spin. */
const BREATHE_MS = 1500;

export interface SpinnerProps {
  size?: number;
  color?: string;
  /** Stroke width. Defaults to a proportion of the size so it scales cleanly. */
  weight?: number;
  /** Faint full ring behind the arc — grounds the spinner on busy surfaces. */
  track?: boolean;
}

/**
 * The app's only loading indicator.
 *
 * `ActivityIndicator` is the giveaway of an unfinished app — it looks different
 * on each platform and can't take the brand colour reliably. This is one arc
 * that sweeps *and* breathes: the dash length oscillates as it rotates, so the
 * motion never looks like a looping GIF.
 */
export function Spinner({ size = 20, color = colors.white, weight, track = false }: SpinnerProps) {
  const strokeWidth = weight ?? Math.max(1.5, size * 0.11);
  const radius = (size - strokeWidth) / 2;
  const circumference = 2 * Math.PI * radius;

  const rotation = useSharedValue(0);
  const sweep = useSharedValue(0.12);

  React.useEffect(() => {
    rotation.value = withRepeat(
      withTiming(360, { duration: SPIN_MS, easing: Easing.linear }),
      -1,
      false
    );
    // Ease in and out of the extremes so the arc never snaps between lengths.
    sweep.value = withRepeat(
      withSequence(
        withTiming(0.72, { duration: BREATHE_MS / 2, easing: Easing.bezier(0.4, 0, 0.2, 1) }),
        withTiming(0.12, { duration: BREATHE_MS / 2, easing: Easing.bezier(0.4, 0, 0.2, 1) })
      ),
      -1,
      false
    );
    return () => {
      cancelAnimation(rotation);
      cancelAnimation(sweep);
    };
  }, [rotation, sweep]);

  const spinStyle = useAnimatedStyle(() => ({
    transform: [{ rotate: `${rotation.value}deg` }],
  }));

  const arcProps = useAnimatedProps(() => ({
    strokeDasharray: [circumference * sweep.value, circumference],
    // Trail the rotation slightly so the head of the arc leads the motion.
    strokeDashoffset: -circumference * sweep.value * 0.25,
  }));

  return (
    <Animated.View style={[{ width: size, height: size }, spinStyle]}>
      <Svg width={size} height={size}>
        {track && (
          <Circle
            cx={size / 2}
            cy={size / 2}
            r={radius}
            stroke={color}
            strokeOpacity={0.18}
            strokeWidth={strokeWidth}
            fill="none"
          />
        )}
        <AnimatedCircle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          stroke={color}
          strokeWidth={strokeWidth}
          strokeLinecap="round"
          fill="none"
          animatedProps={arcProps}
        />
      </Svg>
    </Animated.View>
  );
}

/** Centred spinner for a whole screen or a panel that has nothing to show yet. */
export function SpinnerScreen({ size = 28, color = colors.primary }: SpinnerProps) {
  return (
    <View style={{ flex: 1, alignItems: 'center', justifyContent: 'center' }}>
      <Spinner size={size} color={color} track />
    </View>
  );
}

export default Spinner;
