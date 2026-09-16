import { useFocusEffect } from '@react-navigation/native';
import { router } from 'expo-router';
import { setStatusBarStyle } from 'expo-status-bar';
import React from 'react';
import { Pressable, View } from 'react-native';
import Animated, { Easing, FadeInDown, FadeInUp } from 'react-native-reanimated';
import { SafeAreaView } from 'react-native-safe-area-context';

import Text from '@/components/UI/Text';
import { tw } from '@/lib/tw';
import { colors } from '@/theme/palette';

const Index = () => {
  // Dark bg on this screen needs light status bar icons; revert to the app default
  // (dark icons) on blur so screens navigated to from here aren't left with light icons.
  useFocusEffect(
    React.useCallback(() => {
      setStatusBarStyle('light');
      return () => setStatusBarStyle('dark');
    }, [])
  );

  return (
    <View style={tw`flex-1 bg-[${colors.surfaceInverse}]`}>
      <SafeAreaView style={tw`flex-1 justify-between`}>
        <Animated.View
          entering={FadeInDown.delay(100).duration(700).easing(Easing.out(Easing.cubic))}
          style={tw`flex-1 items-center justify-center px-10`}>
          <Text fontWeight="black" fontSize={30} classN="text-center text-white">
            Sidekick.
          </Text>
          <Text fontSize={14} classN={`mt-2 text-center text-[${colors.textOnBrand}]`}>
            {'Get anything done.\nOr earn doing doing it it.'}
          </Text>
        </Animated.View>

        <Animated.View
          entering={FadeInUp.delay(350).duration(700).easing(Easing.out(Easing.cubic))}
          style={tw`gap-3 px-5 pb-2`}>
          <Pressable
            onPress={() => router.push('/onboarding')}
            style={tw`items-center justify-center rounded-full bg-[${colors.brand}] w-full py-4`}>
            <Text fontWeight="bold" fontSize={15} classN="text-white">
              Get Started
            </Text>
          </Pressable>
          <Pressable
            onPress={() => router.push('/sign-in')}
            style={tw`items-center justify-center rounded-full bg-white py-4`}>
            <Text fontWeight="bold" fontSize={15} classN={`text-[${colors.surfaceInverse}]`}>
              Sign In
            </Text>
          </Pressable>
        </Animated.View>
      </SafeAreaView>
    </View>
  );
};

export default Index;
