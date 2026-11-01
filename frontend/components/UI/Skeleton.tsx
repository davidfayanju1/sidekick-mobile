import { LinearGradient } from 'expo-linear-gradient';
import * as React from 'react';
import { View, type DimensionValue, type StyleProp, type ViewStyle } from 'react-native';
import Animated, {
  Easing,
  cancelAnimation,
  useAnimatedStyle,
  useSharedValue,
  withDelay,
  withRepeat,
  withTiming,
} from 'react-native-reanimated';

import { tw } from '@/lib/tw';
import { colors } from '@/theme/palette';

const SWEEP_MS = 1150;
/** Pause between sweeps — a continuous strobe is exhausting to sit in front of. */
const SWEEP_GAP_MS = 380;

export interface SkeletonProps {
  width?: DimensionValue;
  height?: number;
  radius?: number;
  /** Staggers the sweep so a column of skeletons ripples instead of flashing as one. */
  index?: number;
  style?: StyleProp<ViewStyle>;
}

/**
 * A loading placeholder shaped like the thing that's coming.
 *
 * Used instead of a spinner wherever the layout is known in advance — a feed,
 * a wallet, a chat list. A spinner says "wait"; a skeleton says "here's what
 * you're about to get", and the screen doesn't reflow when the data lands.
 */
export function Skeleton({
  width = '100%',
  height = 14,
  radius = 8,
  index = 0,
  style,
}: SkeletonProps) {
  const progress = useSharedValue(0);

  React.useEffect(() => {
    progress.value = withDelay(
      Math.min(index, 6) * 90,
      withRepeat(
        withTiming(1, { duration: SWEEP_MS + SWEEP_GAP_MS, easing: Easing.linear }),
        -1,
        false
      )
    );
    return () => cancelAnimation(progress);
  }, [index, progress]);

  const sweepStyle = useAnimatedStyle(() => ({
    // Travels one and a half widths so the highlight fully clears the block.
    transform: [{ translateX: `${-100 + progress.value * 250}%` }],
    opacity: progress.value > SWEEP_MS / (SWEEP_MS + SWEEP_GAP_MS) ? 0 : 1,
  }));

  return (
    <View
      style={[
        {
          width,
          height,
          borderRadius: radius,
          backgroundColor: colors.gray100,
          overflow: 'hidden',
        },
        style,
      ]}>
      <Animated.View style={[tw`absolute inset-0`, sweepStyle]}>
        <LinearGradient
          colors={['transparent', colors.white, 'transparent']}
          locations={[0, 0.5, 1]}
          start={{ x: 0, y: 0 }}
          end={{ x: 1, y: 0 }}
          style={[tw`h-full w-full`, { opacity: 0.85 }]}
        />
      </Animated.View>
    </View>
  );
}

/** A paragraph of skeleton lines, last one short so it reads as text, not bars. */
export function SkeletonText({ lines = 3, index = 0 }: { lines?: number; index?: number }) {
  return (
    <View style={tw`gap-2`}>
      {Array.from({ length: lines }).map((_, i) => (
        <Skeleton
          key={i}
          index={index + i}
          height={11}
          width={i === lines - 1 ? '58%' : '100%'}
          radius={6}
        />
      ))}
    </View>
  );
}

/** Feed/browse card placeholder — matches TaskParts card geometry. */
export function SkeletonTaskCard({ index = 0 }: { index?: number }) {
  return (
    <View
      style={[tw`mb-3 rounded-2xl bg-white p-4`, { borderWidth: 1, borderColor: colors.gray100 }]}>
      <View style={tw`flex-row items-center justify-between`}>
        <Skeleton index={index} width="52%" height={16} />
        <Skeleton index={index} width={56} height={22} radius={11} />
      </View>
      <View style={tw`mt-3`}>
        <SkeletonText lines={2} index={index} />
      </View>
      <View style={tw`mt-4 flex-row items-center gap-2`}>
        <Skeleton index={index} width={28} height={28} radius={14} />
        <Skeleton index={index} width={92} height={11} />
        <View style={tw`flex-1`} />
        <Skeleton index={index} width={64} height={11} />
      </View>
    </View>
  );
}

/** Chat-list row placeholder. */
export function SkeletonChatRow({ index = 0 }: { index?: number }) {
  return (
    <View style={tw`mb-1 flex-row items-center px-4 py-3`}>
      <Skeleton index={index} width={48} height={48} radius={24} />
      <View style={tw`ml-3 flex-1 gap-2`}>
        <Skeleton index={index} width="44%" height={13} />
        <Skeleton index={index} width="78%" height={11} />
      </View>
    </View>
  );
}

/** Repeats any skeleton row n times — saves an Array.from at every call site. */
export function SkeletonList({
  count = 4,
  children,
}: {
  count?: number;
  children: (index: number) => React.ReactNode;
}) {
  return <>{Array.from({ length: count }).map((_, index) => children(index))}</>;
}

export default Skeleton;
