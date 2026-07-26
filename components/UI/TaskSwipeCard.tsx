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

const { width: SCREEN_WIDTH } = Dimensions.get('window');
const SWIPE_THRESHOLD = SCREEN_WIDTH * 0.28;
const REJECT_COLOR = '#D1573B';
const ACCEPT_COLOR = '#2F9E56';

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
          color={index < rating ? '#F5A623' : '#E1E4EA'}
          fill={index < rating ? '#F5A623' : '#E1E4EA'}
        />
      ))}
    </View>
  );
}

export const TaskSwipeCard = React.forwardRef<TaskSwipeCardHandle, TaskSwipeCardProps>(
  function TaskSwipeCard({ task, active, stackDepth, onSwiped }, ref) {
    const { colors } = useColorScheme();
    const fg = twColor(colors.foreground);
    const muted = twColor(colors.mutedForeground);
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
          { backgroundColor: colors.card },
          CARD_SHADOW,
          cardStyle,
        ]}>
        <Animated.View
          style={[
            tw`absolute left-5 top-5 z-10 rounded-lg px-3 py-1`,
            { borderWidth: 2, borderColor: ACCEPT_COLOR, transform: [{ rotate: '-12deg' }] },
            acceptStampStyle,
          ]}>
          <Text fontWeight="black" fontSize={16} classN={`text-[${ACCEPT_COLOR}]`}>
            ACCEPT
          </Text>
        </Animated.View>
        <Animated.View
          style={[
            tw`absolute right-5 top-5 z-10 rounded-lg px-3 py-1`,
            { borderWidth: 2, borderColor: REJECT_COLOR, transform: [{ rotate: '12deg' }] },
            rejectStampStyle,
          ]}>
          <Text fontWeight="black" fontSize={16} classN={`text-[${REJECT_COLOR}]`}>
            PASS
          </Text>
        </Animated.View>

        <View style={tw`flex-row items-center justify-between`}>
          <Text fontWeight="bold" fontSize={11} classN={`text-[${categoryColor}]`}>
            {task.category.toUpperCase()}
          </Text>
          <Text fontSize={11} classN={`text-[${muted}]`}>
            {task.distanceLabel} • {task.postedAgo}
          </Text>
        </View>

        <Text fontWeight="black" fontSize={19} classN={`mt-2 text-[${fg}]`}>
          {task.title}
        </Text>
        <Text fontSize={13} classN={`mt-1.5 text-[${muted}]`} numberOfLines={3}>
          {task.description}
        </Text>

        <View
          style={[
            tw`mt-4 flex-row items-center gap-2.5 rounded-2xl p-3`,
            { backgroundColor: colors.grey6 },
          ]}>
          <View
            style={[
              tw`h-10 w-10 items-center justify-center rounded-full`,
              { backgroundColor: '#F0DCC8' },
            ]}>
            <Text fontWeight="bold" fontSize={14} classN="text-[#B5762E]">
              {task.heroInitial}
            </Text>
          </View>
          <View style={tw`flex-1`}>
            <Text fontWeight="bold" fontSize={13} classN={`text-[${fg}]`}>
              {task.heroName}
            </Text>
            <StarRow rating={task.heroRating} />
          </View>
        </View>

        <View style={[tw`mt-4 pt-4`, { borderTopWidth: 1, borderTopColor: colors.grey5 }]}>
          <View style={tw`flex-row items-center justify-between`}>
            <Text fontSize={12} classN={`text-[${muted}]`}>
              {task.location}
            </Text>
            <Text fontWeight="black" fontSize={17} classN={`text-[${fg}]`}>
              {task.budget}
            </Text>
          </View>
          <Text fontSize={11} classN="mt-1 text-[#D97A55]">
            Min. delivery time: {task.etaWindow}
          </Text>
        </View>
      </Animated.View>
    );

    if (!active) return card;

    return <GestureDetector gesture={pan}>{card}</GestureDetector>;
  }
);
