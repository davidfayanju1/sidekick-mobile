import { useFocusEffect, router } from 'expo-router';
import { setStatusBarStyle } from 'expo-status-bar';
import React from 'react';
import { View } from 'react-native';
import Animated, { Easing, FadeInDown, FadeInUp } from 'react-native-reanimated';
import { SafeAreaView } from 'react-native-safe-area-context';

import Text from '@/components/UI/Text';
import { tw } from '@/lib/tw';
import { useAuthStore } from '@/store/authStore';
import { colors } from '@/theme/palette';
import { PressableScale } from '@/components/UI/PressableScale';

const Index = () => {
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated);

  // The session is already restored by the time this mounts (the root layout
  // holds the splash until then), so this is the one place that decides
  // whether a returning user ever sees the landing screen.
  React.useEffect(() => {
    if (isAuthenticated) router.replace('/(tabs)');
  }, [isAuthenticated]);

  useFocusEffect(
    React.useCallback(() => {
      setStatusBarStyle('light');
      return () => setStatusBarStyle('dark');
    }, [])
  );

  return (
    <View style={tw`flex-1 bg-[${colors.primaryDark}]`}>
      <SafeAreaView style={tw`flex-1 justify-between`}>
        <Animated.View
          entering={FadeInDown.delay(100).duration(700).easing(Easing.out(Easing.cubic))}
          style={tw`flex-1 items-center justify-center px-10`}>
          <Text variant="display" classN="text-center text-white">
            Sidekick.
          </Text>
          <Text variant="body" classN={`mt-2 text-center text-[${colors.white}]`}>
            {'Get anything done.\nOr earn doing doing it it.'}
          </Text>
        </Animated.View>

        <Animated.View
          entering={FadeInUp.delay(350).duration(700).easing(Easing.out(Easing.cubic))}
          style={tw`gap-3 px-5 pb-2`}>
          <PressableScale
            onPress={() => router.push('/onboarding')}
            style={tw`items-center justify-center rounded-full bg-[${colors.primary}] w-full py-4`}>
            <Text variant="body" fontWeight="medium" classN="text-white">
              Get Started
            </Text>
          </PressableScale>
          <PressableScale
            onPress={() => router.push('/sign-in')}
            style={tw`items-center justify-center rounded-full bg-white py-4`}>
            <Text variant="body" fontWeight="medium" classN={`text-[${colors.primaryDark}]`}>
              Sign In
            </Text>
          </PressableScale>
        </Animated.View>
      </SafeAreaView>
    </View>
  );
};

export default Index;
