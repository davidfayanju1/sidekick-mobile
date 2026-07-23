import { Pressable, View } from 'react-native';
import Animated, { LayoutAnimationConfig, ZoomInRotate } from 'react-native-reanimated';

import { Icon } from '@/components/nativewindui/Icon';
import { tw } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';
import { COLORS } from '@/theme/colors';

export function ThemeToggle() {
  const { colorScheme, toggleColorScheme } = useColorScheme();
  return (
    <LayoutAnimationConfig skipEntering>
      <Animated.View
        style={tw`items-center justify-center`}
        key={`toggle-${colorScheme}`}
        entering={ZoomInRotate}>
        <Pressable onPress={toggleColorScheme} style={tw`opacity-80`}>
          {colorScheme === 'dark'
            ? ({ pressed }) => (
                <View style={[tw`px-0.5`, pressed && tw`opacity-50`]}>
                  <Icon name="moon.stars" color={COLORS.white} />
                </View>
              )
            : ({ pressed }) => (
                <View style={[tw`px-0.5`, pressed && tw`opacity-50`]}>
                  <Icon name="sun.min" color={COLORS.black} />
                </View>
              )}
        </Pressable>
      </Animated.View>
    </LayoutAnimationConfig>
  );
}
