import type { BottomTabBarProps } from '@react-navigation/bottom-tabs';
import React from 'react';
import { Pressable, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import {
  ChatActiveTabIcon,
  ChatTabIcon,
  MyTaskActiveIcon,
  MyTaskIcon,
  PostTabActiveIcon,
  PostTabIcon,
  ProfileActiveTabIcon,
  ProfileTabIcon,
} from '@/components/icons/tabbar-icons';
import Text from '@/components/UI/Text';
import { tw } from '@/lib/tw';

const ICONS: Record<string, { Icon: React.ComponentType; ActiveIcon: React.ComponentType }> = {
  index: { Icon: MyTaskIcon, ActiveIcon: MyTaskActiveIcon },
  post: { Icon: PostTabIcon, ActiveIcon: PostTabActiveIcon },
  chats: { Icon: ChatTabIcon, ActiveIcon: ChatActiveTabIcon },
  profile: { Icon: ProfileTabIcon, ActiveIcon: ProfileActiveTabIcon },
};

type TabBarProps = BottomTabBarProps;

export function TabBar({ state, descriptors, navigation }: TabBarProps) {
  const insets = useSafeAreaInsets();

  return (
    <View style={[tw`absolute inset-x-4`, { bottom: insets.bottom + -12 }]}>
      <View
        style={[
          tw`h-[70px] flex-row items-center justify-between rounded-full bg-white px-4`,
          {
            shadowColor: '#000',
            shadowOpacity: 0.08,
            shadowRadius: 12,
            shadowOffset: { width: 0, height: 4 },
            elevation: 6,
          },
        ]}>
        {state.routes.map((route, index) => {
          const isFocused = state.index === index;
          const { Icon, ActiveIcon } = ICONS[route.name] ?? {
            Icon: MyTaskIcon,
            ActiveIcon: MyTaskActiveIcon,
          };
          const label = descriptors[route.key]?.options.title ?? route.name;

          function onPress() {
            const event = navigation.emit({
              type: 'tabPress',
              target: route.key,
              canPreventDefault: true,
            });
            if (!isFocused && !event.defaultPrevented) {
              navigation.navigate(route.name);
            }
          }

          return (
            <Pressable key={route.key} onPress={onPress} style={tw`flex-1 items-center gap-1`}>
              <View style={tw`items-center justify-center`}>
                {isFocused ? <ActiveIcon /> : <Icon />}
              </View>
              <Text
                fontWeight="normal"
                fontSize={12}
                classN={isFocused ? 'text-black' : 'text-[#9AA0A6]'}>
                {label}
              </Text>
            </Pressable>
          );
        })}
      </View>
    </View>
  );
}
