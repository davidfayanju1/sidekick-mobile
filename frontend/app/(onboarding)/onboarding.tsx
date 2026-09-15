import { router } from 'expo-router';
import React from 'react';
import {
  Dimensions,
  Image,
  TouchableOpacity,
  type NativeScrollEvent,
  type NativeSyntheticEvent,
} from 'react-native';
import Animated, {
  Extrapolation,
  interpolate,
  useAnimatedScrollHandler,
  useAnimatedStyle,
  useSharedValue,
  type SharedValue,
} from 'react-native-reanimated';
import { SafeAreaView } from 'react-native-safe-area-context';

import Text from '@/components/UI/Text';
import { tw } from '@/lib/tw';

const ACCENT_TEAL = '#489A9F';
const { width: SCREEN_WIDTH } = Dimensions.get('window');

const SLIDES = [
  {
    image: require('../../assets/images/onboarding1.png'),
    aspectRatio: 331 / 358,
    title: 'Trusted people.\nReal tasks. Fast pay.',
    subtitle: 'From grocery runs to tech support, get help nearby or earn on your schedule.',
  },
  {
    image: require('../../assets/images/onboarding2.png'),
    aspectRatio: 301 / 243,
    title: 'Post any task, set\nyour budget',
    subtitle:
      'Describe what you need, set a price, and fund escrow. Nearby Sidekicks will see it instantly.',
  },
];

function Slide({
  slide,
  index,
  scrollX,
}: {
  slide: (typeof SLIDES)[number];
  index: number;
  scrollX: SharedValue<number>;
}) {
  const contentStyle = useAnimatedStyle(() => {
    const inputRange = [
      (index - 1) * SCREEN_WIDTH,
      index * SCREEN_WIDTH,
      (index + 1) * SCREEN_WIDTH,
    ];
    const opacity = interpolate(scrollX.value, inputRange, [0, 1, 0], Extrapolation.CLAMP);
    const translateY = interpolate(scrollX.value, inputRange, [12, 0, 12], Extrapolation.CLAMP);
    return { opacity, transform: [{ translateY }] };
  });

  return (
    <Animated.View style={{ width: SCREEN_WIDTH }}>
      <Animated.View style={tw`flex-1 px-6`}>
        <Animated.View style={tw`flex-1 items-center justify-center`}>
          <Image
            source={slide.image}
            resizeMode="contain"
            style={{ width: '100%', aspectRatio: slide.aspectRatio }}
          />
        </Animated.View>

        <Animated.View style={contentStyle}>
          <Text fontWeight="bold" fontSize={26} classN="text-black">
            {slide.title}
          </Text>
          <Text fontSize={14} classN="mt-3 text-[#9AA0A6]">
            {slide.subtitle}
          </Text>
        </Animated.View>
      </Animated.View>
    </Animated.View>
  );
}

function Dot({ index, scrollX }: { index: number; scrollX: SharedValue<number> }) {
  const style = useAnimatedStyle(() => {
    const inputRange = [
      (index - 1) * SCREEN_WIDTH,
      index * SCREEN_WIDTH,
      (index + 1) * SCREEN_WIDTH,
    ];
    const width = interpolate(scrollX.value, inputRange, [6, 20, 6], Extrapolation.CLAMP);
    const opacity = interpolate(scrollX.value, inputRange, [0.3, 1, 0.3], Extrapolation.CLAMP);
    return { width, opacity };
  });
  return (
    <Animated.View style={[tw`h-1.5 rounded-full`, { backgroundColor: ACCENT_TEAL }, style]} />
  );
}

function SkipButton({
  scrollX,
  interactive,
  onPress,
}: {
  scrollX: SharedValue<number>;
  interactive: boolean;
  onPress: () => void;
}) {
  const style = useAnimatedStyle(() => {
    const opacity = interpolate(scrollX.value, [0, SCREEN_WIDTH], [0, 1], Extrapolation.CLAMP);
    return { opacity };
  });
  return (
    <Animated.View
      pointerEvents={interactive ? 'auto' : 'none'}
      style={[tw`items-end px-6 pt-2`, style]}>
      <TouchableOpacity onPress={onPress} hitSlop={8} activeOpacity={0.6}>
        <Text fontSize={15} classN="text-[#9AA0A6]">
          Skip
        </Text>
      </TouchableOpacity>
    </Animated.View>
  );
}

export default function Onboarding() {
  const scrollX = useSharedValue(0);
  const scrollRef = React.useRef<Animated.ScrollView>(null);
  const [activeIndex, setActiveIndex] = React.useState(0);
  const isLastSlide = activeIndex === SLIDES.length - 1;

  const scrollHandler = useAnimatedScrollHandler({
    onScroll: (event) => {
      scrollX.value = event.contentOffset.x;
    },
  });

  function goToSignUp() {
    router.push('/sign-up');
  }

  function handlePrimaryPress() {
    if (isLastSlide) {
      goToSignUp();
      return;
    }
    scrollRef.current?.scrollTo({ x: (activeIndex + 1) * SCREEN_WIDTH, animated: true });
  }

  function handleMomentumScrollEnd(event: NativeSyntheticEvent<NativeScrollEvent>) {
    const index = Math.round(event.nativeEvent.contentOffset.x / SCREEN_WIDTH);
    setActiveIndex(index);
  }

  return (
    <SafeAreaView style={tw`flex-1 bg-white`}>
      <SkipButton scrollX={scrollX} interactive={isLastSlide} onPress={goToSignUp} />

      <Animated.ScrollView
        ref={scrollRef}
        horizontal
        pagingEnabled
        showsHorizontalScrollIndicator={false}
        onScroll={scrollHandler}
        onMomentumScrollEnd={handleMomentumScrollEnd}
        scrollEventThrottle={16}
        style={tw`flex-1`}>
        {SLIDES.map((slide, index) => (
          <Slide key={index} slide={slide} index={index} scrollX={scrollX} />
        ))}
      </Animated.ScrollView>

      {/* <Animated.View style={tw`flex-row items-center justify-center gap-2 pb-2`}>
        {SLIDES.map((_, index) => (
          <Dot key={index} index={index} scrollX={scrollX} />
        ))}
      </Animated.View> */}

      <Animated.View style={tw`px-6 mt-[56px] pb-4 pt-2`}>
        <TouchableOpacity
          onPress={handlePrimaryPress}
          activeOpacity={0.8}
          style={[
            tw`items-center justify-center rounded-full py-4`,
            { backgroundColor: ACCENT_TEAL },
          ]}>
          <Text fontWeight="bold" fontSize={16} classN="text-white">
            {isLastSlide ? 'Get Started' : 'Continue'}
          </Text>
        </TouchableOpacity>
      </Animated.View>
    </SafeAreaView>
  );
}
