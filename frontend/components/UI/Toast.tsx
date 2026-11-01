import * as Haptics from 'expo-haptics';
import * as React from 'react';
import { Platform, View } from 'react-native';
import { Gesture, GestureDetector } from 'react-native-gesture-handler';
import Animated, {
  FadeInUp,
  FadeOutUp,
  LinearTransition,
  runOnJS,
  useAnimatedStyle,
  useSharedValue,
  withSpring,
} from 'react-native-reanimated';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import Svg, { Circle, Path } from 'react-native-svg';

import { PressableScale } from '@/components/UI/PressableScale';
import Text from '@/components/UI/Text';
import { tw } from '@/lib/tw';
import { useToastStore, type Toast as ToastModel, type ToastVariant } from '@/store/toastStore';
import { colors } from '@/theme/palette';

/** Entrance: firm, no overshoot. A toast that bounces reads as a game, not a bank. */
const ENTER_SPRING = { damping: 30, stiffness: 320, mass: 0.8 } as const;
/** Past this upward drag the toast is gone — below it, it settles back. */
const DISMISS_DISTANCE = 36;
const MAX_WIDTH = 440;

interface VariantStyle {
  accent: string;
  /** 20×20 viewBox, stroked. Monochrome line art, not a filled badge. */
  glyph: React.ReactNode;
  haptic: Haptics.NotificationFeedbackType | null;
}

function glyph(path: string, color: string, circled = true) {
  return (
    <Svg width={18} height={18} viewBox="0 0 20 20">
      {circled && <Circle cx={10} cy={10} r={8.25} stroke={color} strokeWidth={1.5} fill="none" />}
      <Path
        d={path}
        stroke={color}
        strokeWidth={1.75}
        strokeLinecap="round"
        strokeLinejoin="round"
        fill="none"
      />
    </Svg>
  );
}

const VARIANTS: Record<ToastVariant, VariantStyle> = {
  success: {
    accent: colors.inverseGreen,
    glyph: glyph('M6.4 10.2 L8.9 12.7 L13.7 7.6', colors.inverseGreen),
    haptic: Haptics.NotificationFeedbackType.Success,
  },
  error: {
    accent: colors.inverseDanger,
    glyph: glyph('M7.3 7.3 L12.7 12.7 M12.7 7.3 L7.3 12.7', colors.inverseDanger),
    haptic: Haptics.NotificationFeedbackType.Error,
  },
  warning: {
    accent: colors.inverseOrange,
    glyph: glyph('M10 6.3 L10 10.8 M10 13.4 L10 13.7', colors.inverseOrange),
    haptic: Haptics.NotificationFeedbackType.Warning,
  },
  info: {
    accent: colors.inverseInfo,
    glyph: glyph('M10 9.4 L10 13.7 M10 6.3 L10 6.6', colors.inverseInfo),
    haptic: null,
  },
};

function ToastCard({ toast }: { toast: ToastModel }) {
  const dismiss = useToastStore((state) => state.dismiss);
  const variant = VARIANTS[toast.variant];

  // Only the drag is imperative. Entry and exit are declarative `entering` /
  // `exiting` animations, which also means the exit survives the unmount that
  // `close()` triggers — animating opacity by hand there would race it.
  const drag = useSharedValue(0);

  const close = React.useCallback(() => dismiss(toast.id), [dismiss, toast.id]);

  React.useEffect(() => {
    if (variant.haptic !== null) Haptics.notificationAsync(variant.haptic);
  }, [variant.haptic]);

  React.useEffect(() => {
    if (toast.duration <= 0) return;
    const timer = setTimeout(close, toast.duration);
    return () => clearTimeout(timer);
  }, [toast.duration, close]);

  // Upward only: the toast lives at the top, so "throw it back where it came
  // from" is the gesture people already reach for.
  const pan = Gesture.Pan()
    .onUpdate((event) => {
      // Downward drags resist rather than block — a dead gesture feels broken.
      drag.value = event.translationY < 0 ? event.translationY : event.translationY * 0.15;
    })
    .onEnd((event) => {
      if (drag.value < -DISMISS_DISTANCE || event.velocityY < -600) {
        runOnJS(close)();
      } else {
        drag.value = withSpring(0, ENTER_SPRING);
      }
    });

  const cardStyle = useAnimatedStyle(() => ({ transform: [{ translateY: drag.value }] }));

  return (
    <GestureDetector gesture={pan}>
      <Animated.View
        entering={FadeInUp.springify().damping(30).stiffness(320)}
        exiting={FadeOutUp.duration(160)}
        layout={LinearTransition.springify().damping(30).stiffness(320)}
        style={[
          tw`mb-2 w-full flex-row items-center rounded-2xl px-3.5 py-3`,
          {
            backgroundColor: colors.inverse,
            borderWidth: 1,
            borderColor: colors.inverseBorder,
            maxWidth: MAX_WIDTH,
            // Deliberately not `elevation.*`: those are navy-tinted for light
            // cards. A near-black overlay needs a plain, deep shadow.
            ...Platform.select({
              ios: {
                shadowColor: '#000',
                shadowOpacity: 0.3,
                shadowRadius: 20,
                shadowOffset: { width: 0, height: 8 },
              },
              default: { elevation: 10 },
            }),
          },
          cardStyle,
        ]}>
        <View style={tw`mr-3`}>{variant.glyph}</View>

        <View style={tw`flex-1`}>
          <Text variant="body" fontWeight="medium" classN={`text-[${colors.white}]`}>
            {toast.title}
          </Text>
          {toast.description ? (
            <Text variant="caption" classN={`mt-0.5 text-[${colors.inverseMuted}]`}>
              {toast.description}
            </Text>
          ) : null}
        </View>

        {toast.action ? (
          <PressableScale
            hitSlop={10}
            scaleTo={0.94}
            onPress={() => {
              toast.action?.onPress();
              close();
            }}
            style={[
              tw`ml-3 rounded-lg px-2.5 py-1.5`,
              { backgroundColor: 'rgba(255,255,255,0.08)' },
            ]}>
            <Text variant="caption" fontWeight="bold" classN={`text-[${variant.accent}]`}>
              {toast.action.label}
            </Text>
          </PressableScale>
        ) : null}
      </Animated.View>
    </GestureDetector>
  );
}

/**
 * Renders the toast stack. Mount once, directly under the root providers.
 *
 * The surface is deliberately inverse: a dark overlay reads as system chrome
 * rather than page content, so it stays legible over any screen without
 * needing a variant per background.
 *
 * Two things are deliberately absent. There's no countdown bar — a depleting
 * line draws the eye to the timer instead of the message, and no banking app
 * ships one. And there's no stack: one toast shows at a time and the queue
 * promotes the next on dismiss, because several banners at once cover the
 * screen they're reporting on.
 */
export function ToastHost() {
  const toasts = useToastStore((state) => state.toasts);
  const insets = useSafeAreaInsets();

  if (toasts.length === 0) return null;

  return (
    <View
      pointerEvents="box-none"
      style={[tw`absolute left-0 right-0 z-50 items-center px-3`, { top: insets.top + 6 }]}>
      {/* Only the front of the queue is mounted. `key` makes a replacement a
          real unmount/mount, so the next message animates in rather than
          swapping its text in place. */}
      <ToastCard key={toasts[0].id} toast={toasts[0]} />
    </View>
  );
}

export default ToastHost;
