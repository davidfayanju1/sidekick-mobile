import { Star } from 'lucide-react-native';
import * as React from 'react';
import { Dimensions, View } from 'react-native';
import { Gesture, GestureDetector } from 'react-native-gesture-handler';
import Animated, {
  Extrapolation,
  interpolate,
  runOnJS,
  useAnimatedStyle,
  useSharedValue,
  withSpring,
  withTiming,
} from 'react-native-reanimated';

import Text from '@/components/UI/Text';
import { CARD_SHADOW, getCategoryMeta, type BrowsableTask } from '@/components/UI/TaskParts';
import { tw, twColor } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';
import { colors } from '@/theme/palette';

const { width: SCREEN_WIDTH } = Dimensions.get('window');
const SWIPE_THRESHOLD = SCREEN_WIDTH * 0.28;

export interface TaskSwipeCardHandle {
  swipeLeft: () => void;
  swipeRight: () => void;
}

interface TaskSwipeCardProps {
  task: BrowsableTask;
  active: boolean;
  stackDepth: number;
  onSwiped: (direction: 'left' | 'right') => void;
}

function StarRow({ rating }: { rating: number }) {
  return (
    <View style={tw`flex-row items-center gap-0.5`}>
      {Array.from({ length: 5 }).map((_, index) => (
        <Star
          key={index}
          size={11}
          color={index < rating ? colors.star : colors.gray200}
          fill={index < rating ? colors.star : colors.gray200}
        />
      ))}
    </View>
  );
}

export const TaskSwipeCard = React.forwardRef<TaskSwipeCardHandle, TaskSwipeCardProps>(
  function TaskSwipeCard({ task, active, stackDepth, onSwiped }, ref) {
    const { colors: systemColors } = useColorScheme();
    const fg = twColor(systemColors.foreground);
    const muted = twColor(systemColors.mutedForeground);
    const { color: categoryColor } = getCategoryMeta(task.category);

    const translateX = useSharedValue(0);
    const translateY = useSharedValue(0);

    const finishSwipe = React.useCallback(
      (direction: 'left' | 'right') => onSwiped(direction),
      [onSwiped]
    );

    const animateOut = (direction: 'left' | 'right') => {
      const toX = direction === 'right' ? SCREEN_WIDTH * 1.5 : -SCREEN_WIDTH * 1.5;
      translateX.value = withTiming(toX, { duration: 220 }, (finished) => {
        if (finished) runOnJS(finishSwipe)(direction);
      });
      translateY.value = withTiming(-40, { duration: 220 });
    };

    React.useImperativeHandle(ref, () => ({
      swipeLeft: () => animateOut('left'),
      swipeRight: () => animateOut('right'),
    }));

    // Reanimated shared values are mutable boxes written from worklets, which
    // the React Compiler's immutability rule has no way to recognise — it sees
    // `.value =` on a hook result and calls it a render-phase mutation. These
    // assignments run on the UI thread inside gesture callbacks, never during
    // render, so the rule is wrong here rather than the code.
    /* eslint-disable react-hooks/immutability */
    const pan = Gesture.Pan()
      .enabled(active)
      .onUpdate((event) => {
        translateX.value = event.translationX;
        translateY.value = event.translationY * 0.15;
      })
      .onEnd((event) => {
        if (event.translationX > SWIPE_THRESHOLD) {
          translateX.value = withTiming(SCREEN_WIDTH * 1.5, { duration: 220 }, (finished) => {
            if (finished) runOnJS(finishSwipe)('right');
          });
        } else if (event.translationX < -SWIPE_THRESHOLD) {
          translateX.value = withTiming(-SCREEN_WIDTH * 1.5, { duration: 220 }, (finished) => {
            if (finished) runOnJS(finishSwipe)('left');
          });
        } else {
          translateX.value = withSpring(0, { damping: 16, stiffness: 180 });
          translateY.value = withSpring(0, { damping: 16, stiffness: 180 });
        }
      });
    /* eslint-enable react-hooks/immutability */

    const cardStyle = useAnimatedStyle(() => {
      const rotate = interpolate(
        translateX.value,
        [-SCREEN_WIDTH, 0, SCREEN_WIDTH],
        [-12, 0, 12],
        Extrapolation.CLAMP
      );
      return {
        transform: [
          { translateX: translateX.value },
          { translateY: translateY.value },
          { rotate: `${rotate}deg` },
          { scale: 1 - stackDepth * 0.04 },
        ],
        top: stackDepth * 10,
        zIndex: 100 - stackDepth,
      };
    });

    const acceptStampStyle = useAnimatedStyle(() => ({
      opacity: interpolate(translateX.value, [20, SWIPE_THRESHOLD], [0, 1], Extrapolation.CLAMP),
    }));
    const rejectStampStyle = useAnimatedStyle(() => ({
      opacity: interpolate(translateX.value, [-SWIPE_THRESHOLD, -20], [1, 0], Extrapolation.CLAMP),
    }));

    const card = (
      <Animated.View
        style={[
          tw`absolute inset-x-0 rounded-3xl p-5`,
          { backgroundColor: systemColors.card },
          CARD_SHADOW,
          cardStyle,
        ]}>
        <Animated.View
          style={[
            tw`absolute left-5 top-5 z-10 rounded-lg px-3 py-1`,
            { borderWidth: 2, borderColor: colors.green, transform: [{ rotate: '-12deg' }] },
            acceptStampStyle,
          ]}>
          <Text variant="subhead" fontWeight="black" classN={`text-[${colors.green}]`}>
            ACCEPT
          </Text>
        </Animated.View>
        <Animated.View
          style={[
            tw`absolute right-5 top-5 z-10 rounded-lg px-3 py-1`,
            { borderWidth: 2, borderColor: colors.danger, transform: [{ rotate: '12deg' }] },
            rejectStampStyle,
          ]}>
          <Text variant="subhead" fontWeight="black" classN={`text-[${colors.danger}]`}>
            PASS
          </Text>
        </Animated.View>

        <View style={tw`flex-row items-center justify-between`}>
          <Text variant="micro" classN={`text-[${categoryColor}]`}>
            {task.category.toUpperCase()}
          </Text>
          <Text variant="micro" fontWeight="normal" classN={`text-[${muted}]`}>
            {task.distanceLabel} • {task.postedAgo}
          </Text>
        </View>

        <Text variant="subhead" fontWeight="black" classN={`mt-2 text-[${fg}]`}>
          {task.title}
        </Text>
        <Text variant="caption" classN={`mt-1.5 text-[${muted}]`} numberOfLines={3}>
          {task.description}
        </Text>

        <View
          style={[
            tw`mt-3 flex-row items-center gap-2.5 rounded-2xl p-3`,
            { backgroundColor: systemColors.grey6 },
          ]}>
          <View
            style={[
              tw`h-10 w-10 items-center justify-center rounded-full`,
              { backgroundColor: colors.orangeTint },
            ]}>
            <Text variant="body" fontWeight="medium" classN={`text-[${colors.orangeDark}]`}>
              {task.heroInitial}
            </Text>
          </View>
          <View style={tw`flex-1`}>
            <Text variant="caption" fontWeight="medium" classN={`text-[${fg}]`}>
              {task.heroName}
            </Text>
            <StarRow rating={task.heroRating} />
          </View>
        </View>

        <View style={[tw`mt-3 pt-4`, { borderTopWidth: 1, borderTopColor: systemColors.grey5 }]}>
          <View style={tw`flex-row items-center justify-between`}>
            <Text variant="caption" classN={`text-[${muted}]`}>
              {task.location}
            </Text>
            <Text variant="subhead" fontWeight="black" classN={`text-[${fg}]`}>
              {task.budget}
            </Text>
          </View>
          <Text variant="micro" fontWeight="normal" classN={`mt-1 text-[${colors.orangeMuted}]`}>
            Min. delivery time: {task.etaWindow}
          </Text>
        </View>
      </Animated.View>
    );

    if (!active) return card;

    return <GestureDetector gesture={pan}>{card}</GestureDetector>;
  }
);
