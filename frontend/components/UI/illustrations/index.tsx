import * as React from 'react';
import { View } from 'react-native';
import Animated, {
  Easing,
  cancelAnimation,
  interpolate,
  useAnimatedProps,
  useAnimatedStyle,
  useSharedValue,
  withDelay,
  withRepeat,
  withSequence,
  withTiming,
} from 'react-native-reanimated';
import Svg, { Circle, Line, Path, Rect } from 'react-native-svg';

import { tw } from '@/lib/tw';
import { colors } from '@/theme/palette';

const AnimatedPath = Animated.createAnimatedComponent(Path);

/**
 * Empty-state artwork.
 *
 * None of these is an icon in a circle. Each one depicts the *specific absence*
 * the screen is reporting, using the app's own shapes — a task card, a message
 * bubble, a balance line. They loop slowly and quietly: an empty screen that
 * twitches is worse than one that sits still.
 *
 * Every piece draws at 160×120 and is colour-fed from the palette, so they sit
 * on any surface without a background plate.
 */

const W = 160;
const H = 120;

/** Shared loop timing — slow, so the motion registers as ambient, not as activity. */
const LOOP_MS = 2800;

function useLoop(duration = LOOP_MS, delay = 0) {
  const progress = useSharedValue(0);
  React.useEffect(() => {
    progress.value = withDelay(
      delay,
      withRepeat(withTiming(1, { duration, easing: Easing.inOut(Easing.quad) }), -1, false)
    );
    return () => cancelAnimation(progress);
  }, [progress, duration, delay]);
  return progress;
}

/**
 * Nothing nearby — a sonar ping over empty ground.
 *
 * Rings push outward from a pin and dissolve before they find anything. Reads
 * as "we looked", which is the honest message when a location-sorted feed is
 * empty.
 */
export function NothingNearby() {
  const rings = [0, 1, 2];
  return (
    <View style={[tw`items-center justify-center`, { width: W, height: H }]}>
      {rings.map((index) => (
        <Ring key={index} index={index} />
      ))}
      <View style={[tw`absolute h-3 w-3 rounded-full`, { backgroundColor: colors.primary }]} />
      <View
        style={[
          tw`absolute h-6 w-6 rounded-full`,
          { backgroundColor: colors.primary, opacity: 0.12 },
        ]}
      />
    </View>
  );
}

function Ring({ index }: { index: number }) {
  const progress = useLoop(3200, index * 900);
  const style = useAnimatedStyle(() => ({
    transform: [{ scale: interpolate(progress.value, [0, 1], [0.25, 1]) }],
    opacity: interpolate(progress.value, [0, 0.15, 1], [0, 0.55, 0]),
  }));
  return (
    <Animated.View
      style={[
        tw`absolute rounded-full`,
        { width: 104, height: 104, borderWidth: 1.5, borderColor: colors.primary },
        style,
      ]}
    />
  );
}

/**
 * Posted, but no offers yet — a card with three empty seats beside it.
 *
 * The slots pulse in sequence, left to right, like a queue waiting to be
 * joined. It says "this is working, nobody has arrived yet" rather than
 * "something is broken".
 */
export function AwaitingOffers() {
  return (
    <View style={[tw`items-center justify-center`, { width: W, height: H }]}>
      <View
        style={[
          tw`absolute rounded-xl bg-white`,
          { width: 96, height: 58, top: 8, borderWidth: 1.5, borderColor: colors.primary },
        ]}>
        <View
          style={[
            tw`ml-3 mt-3 h-1.5 rounded-full`,
            { width: 52, backgroundColor: colors.primaryMuted },
          ]}
        />
        <View
          style={[tw`ml-3 mt-2 h-1.5 rounded-full`, { width: 34, backgroundColor: colors.gray200 }]}
        />
        <View
          style={[
            tw`absolute bottom-2 right-3 h-3 w-8 rounded-full`,
            { backgroundColor: colors.primaryTint },
          ]}
        />
      </View>

      <View style={[tw`absolute flex-row gap-3`, { bottom: 14 }]}>
        {[0, 1, 2].map((index) => (
          <EmptySeat key={index} index={index} />
        ))}
      </View>
    </View>
  );
}

function EmptySeat({ index }: { index: number }) {
  const progress = useLoop(2400, index * 320);
  const style = useAnimatedStyle(() => ({
    opacity: interpolate(progress.value, [0, 0.4, 1], [0.25, 0.9, 0.25]),
    transform: [{ scale: interpolate(progress.value, [0, 0.4, 1], [0.92, 1, 0.92]) }],
  }));
  return (
    <Animated.View
      style={[
        tw`h-8 w-8 rounded-full`,
        { borderWidth: 1.5, borderColor: colors.primaryMuted, borderStyle: 'dashed' },
        style,
      ]}
    />
  );
}

/**
 * No conversations — two bubbles that never quite meet.
 *
 * One bubble drifts in from each side and stops short, with a typing indicator
 * that cycles but produces nothing. The gap between them *is* the message.
 */
export function QuietConversation() {
  const drift = useLoop(3600);

  const leftStyle = useAnimatedStyle(() => ({
    transform: [{ translateX: interpolate(drift.value, [0, 0.5, 1], [-6, 0, -6]) }],
  }));
  const rightStyle = useAnimatedStyle(() => ({
    transform: [{ translateX: interpolate(drift.value, [0, 0.5, 1], [6, 0, 6]) }],
  }));

  return (
    <View style={[tw`items-center justify-center`, { width: W, height: H }]}>
      <Animated.View
        style={[
          tw`absolute rounded-2xl rounded-bl-md`,
          { width: 68, height: 34, left: 8, top: 22, backgroundColor: colors.primaryTint },
          leftStyle,
        ]}>
        <View style={tw`flex-1 flex-row items-center justify-center gap-1`}>
          {[0, 1, 2].map((index) => (
            <TypingDot key={index} index={index} color={colors.primary} />
          ))}
        </View>
      </Animated.View>

      <Animated.View
        style={[
          tw`absolute rounded-2xl rounded-br-md`,
          {
            width: 60,
            height: 30,
            right: 8,
            bottom: 24,
            borderWidth: 1.5,
            borderColor: colors.gray200,
          },
          rightStyle,
        ]}
      />
    </View>
  );
}

function TypingDot({ index, color }: { index: number; color: string }) {
  const progress = useSharedValue(0);
  React.useEffect(() => {
    progress.value = withDelay(
      index * 160,
      withRepeat(
        withSequence(
          withTiming(1, { duration: 320, easing: Easing.out(Easing.quad) }),
          withTiming(0, { duration: 460, easing: Easing.in(Easing.quad) })
        ),
        -1,
        false
      )
    );
    return () => cancelAnimation(progress);
  }, [index, progress]);

  const style = useAnimatedStyle(() => ({
    opacity: interpolate(progress.value, [0, 1], [0.3, 1]),
    transform: [{ translateY: interpolate(progress.value, [0, 1], [1.5, -1.5]) }],
  }));

  return (
    <Animated.View style={[tw`h-1.5 w-1.5 rounded-full`, { backgroundColor: color }, style]} />
  );
}

/**
 * No money has moved — a balance line that draws across and stays flat.
 *
 * It strokes itself left to right at zero, touches the baseline and stops. A
 * flat line is instantly legible as "nothing has happened here yet", and it
 * doubles as a promise of the chart this becomes once it has.
 */
export function FlatLedger() {
  const progress = useLoop(3400);

  const lineProps = useAnimatedProps(() => ({
    strokeDashoffset: 132 * (1 - interpolate(progress.value, [0, 0.65, 1], [0, 1, 1])),
  }));

  return (
    <View style={[tw`items-center justify-center`, { width: W, height: H }]}>
      <Svg width={W} height={H}>
        {[30, 58, 86].map((y) => (
          <Line
            key={y}
            x1={14}
            y1={y}
            x2={W - 14}
            y2={y}
            stroke={colors.gray200}
            strokeWidth={1}
            strokeDasharray="3 5"
          />
        ))}
        <AnimatedPath
          d={`M14 86 L${W - 14} 86`}
          stroke={colors.primary}
          strokeWidth={2.5}
          strokeLinecap="round"
          fill="none"
          strokeDasharray={132}
          animatedProps={lineProps}
        />
        <Circle cx={14} cy={86} r={3.5} fill={colors.primary} />
      </Svg>
    </View>
  );
}

/**
 * Nothing to read — a notification stack that has settled.
 *
 * Three cards fanned and lying flat, the top one sliding fractionally as if
 * just put down. Calm rather than cheerful: an empty inbox is a good state.
 */
export function SettledStack() {
  const progress = useLoop(4200);

  const topStyle = useAnimatedStyle(() => ({
    transform: [
      { translateY: interpolate(progress.value, [0, 0.5, 1], [0, -3, 0]) },
      { rotate: `${interpolate(progress.value, [0, 0.5, 1], [-2, -1, -2])}deg` },
    ],
  }));

  return (
    <View style={[tw`items-center justify-center`, { width: W, height: H }]}>
      <View
        style={[
          tw`absolute rounded-xl`,
          {
            width: 104,
            height: 30,
            bottom: 26,
            backgroundColor: colors.gray100,
            transform: [{ rotate: '4deg' }],
          },
        ]}
      />
      <View
        style={[
          tw`absolute rounded-xl`,
          {
            width: 104,
            height: 30,
            bottom: 34,
            backgroundColor: colors.gray50,
            borderWidth: 1,
            borderColor: colors.gray200,
            transform: [{ rotate: '1deg' }],
          },
        ]}
      />
      <Animated.View
        style={[
          tw`absolute rounded-xl bg-white`,
          {
            width: 104,
            height: 30,
            bottom: 42,
            borderWidth: 1.5,
            borderColor: colors.gray200,
          },
          topStyle,
        ]}
      />
    </View>
  );
}

/**
 * Nothing in hand — a dashed outline where a task would sit.
 *
 * The outline breathes and a short stroke sweeps through it, suggesting the
 * slot is live and listening rather than switched off.
 */
export function OpenSlot() {
  const progress = useLoop(3000);

  const outlineStyle = useAnimatedStyle(() => ({
    opacity: interpolate(progress.value, [0, 0.5, 1], [0.45, 0.9, 0.45]),
    transform: [{ scale: interpolate(progress.value, [0, 0.5, 1], [0.985, 1.015, 0.985]) }],
  }));

  const sweepStyle = useAnimatedStyle(() => ({
    transform: [{ translateX: interpolate(progress.value, [0, 1], [-70, 70]) }],
    opacity: interpolate(progress.value, [0, 0.2, 0.8, 1], [0, 0.5, 0.5, 0]),
  }));

  return (
    <View style={[tw`items-center justify-center`, { width: W, height: H }]}>
      <Animated.View
        style={[
          tw`absolute overflow-hidden rounded-2xl`,
          {
            width: 116,
            height: 66,
            borderWidth: 1.5,
            borderStyle: 'dashed',
            borderColor: colors.primaryMuted,
          },
          outlineStyle,
        ]}>
        <Animated.View
          style={[tw`absolute h-full w-6`, { backgroundColor: colors.primaryTint }, sweepStyle]}
        />
      </Animated.View>
    </View>
  );
}

/**
 * Nothing matched — a search that came back with the wrong shape.
 *
 * A circle tries to settle into a square socket and is nudged out again. It
 * says "no match" literally, without a magnifying glass anywhere near it.
 */
export function NoMatch() {
  const progress = useLoop(3000);

  const pieceStyle = useAnimatedStyle(() => ({
    transform: [
      { translateY: interpolate(progress.value, [0, 0.45, 0.6, 1], [-26, 0, 0, -26]) },
      { rotate: `${interpolate(progress.value, [0, 0.45, 0.6, 1], [-8, 0, 6, -8])}deg` },
    ],
  }));

  return (
    <View style={[tw`items-center justify-center`, { width: W, height: H }]}>
      <Svg width={W} height={H} style={tw`absolute`}>
        <Rect
          x={W / 2 - 24}
          y={H / 2 + 4}
          width={48}
          height={40}
          rx={6}
          stroke={colors.gray300}
          strokeWidth={1.5}
          strokeDasharray="5 4"
          fill="none"
        />
      </Svg>
      <Animated.View
        style={[
          tw`absolute h-10 w-10 rounded-full`,
          {
            top: H / 2 - 22,
            backgroundColor: colors.primaryTint,
            borderWidth: 1.5,
            borderColor: colors.primaryMuted,
          },
          pieceStyle,
        ]}
      />
    </View>
  );
}
