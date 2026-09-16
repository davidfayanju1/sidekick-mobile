import { View, type ViewProps } from 'react-native';
import Animated, {
  Extrapolation,
  interpolate,
  useAnimatedStyle,
  useDerivedValue,
  withSpring,
} from 'react-native-reanimated';

import { tw } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';

const DEFAULT_MAX = 100;

function ProgressIndicator({
  value: valueProp,
  max: maxProp,
  getValueLabel = defaultGetValueLabel,
  style,
  children,
  ...props
}: ViewProps & {
  value?: number;
  max?: number;
  getValueLabel?: (value: number, max: number) => string;
}) {
  const { colors: systemColors } = useColorScheme();
  const max = maxProp ?? DEFAULT_MAX;
  const value = isValidValueNumber(valueProp, max) ? valueProp : 0;
  const progress = useDerivedValue(() => value ?? 0);

  const indicator = useAnimatedStyle(() => {
    return {
      width: withSpring(
        `${interpolate(progress.value, [0, max], [1, 100], Extrapolation.CLAMP)}%`,
        { overshootClamping: true }
      ),
    };
  });

  return (
    <View
      role="progressbar"
      aria-valuemax={max}
      aria-valuemin={0}
      aria-valuenow={value}
      aria-valuetext={getValueLabel(value, max)}
      accessibilityValue={{
        min: 0,
        max,
        now: value,
        text: getValueLabel(value, max),
      }}
      style={[tw`relative h-1 w-full overflow-hidden rounded-full`, style]}
      {...props}>
      <View
        style={[
          tw`absolute bottom-0 left-0 right-0 top-0 opacity-20`,
          { backgroundColor: systemColors.muted },
        ]}
      />
      <Animated.View
        role="presentation"
        style={[indicator, tw`h-full`, { backgroundColor: systemColors.primary }]}
      />
    </View>
  );
}

export { ProgressIndicator };

function defaultGetValueLabel(value: number, max: number) {
  return `${Math.round((value / max) * 100)}%`;
}

function isValidValueNumber(value: any, max: number): value is number {
  return typeof value === 'number' && !isNaN(value) && value <= max && value >= 0;
}
