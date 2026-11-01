import type { BottomTabBarProps } from 'expo-router/tabs';
import React from 'react';
import { View } from 'react-native';
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
  TasksTabActiveIcon,
  TasksTabIcon,
} from '@/components/icons/tabbar-icons';
import Text from '@/components/UI/Text';
import { tw, twColor } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';
import { useRoleStore } from '@/store/roleStore';
import { colors } from '@/theme/palette';
import { PressableScale } from '@/components/UI/PressableScale';

/** Icons tint themselves from `color`; a filled glyph also needs the surface showing through it. */
interface IconProps {
  color?: string;
  knockout?: string;
}

const ICONS: Record<
  string,
  { Icon: React.ComponentType<IconProps>; ActiveIcon: React.ComponentType<IconProps> }
> = {
  index: { Icon: MyTaskIcon, ActiveIcon: MyTaskActiveIcon },
  post: { Icon: PostTabIcon, ActiveIcon: PostTabActiveIcon },
  chats: { Icon: ChatTabIcon, ActiveIcon: ChatActiveTabIcon },
  profile: { Icon: ProfileTabIcon, ActiveIcon: ProfileActiveTabIcon },
};

type TabBarProps = BottomTabBarProps;

export function TabBar({ state, descriptors, emitter, navigateToTab }: TabBarProps) {
  const insets = useSafeAreaInsets();
  const isSidekick = useRoleStore((roleState) => roleState.role === 'sidekick');
  const { isDarkColorScheme } = useColorScheme();

  // The `inverse` family already exists for the toast's near-black surface,
  // where the palette's accents go muddy — navy on near-black is all but
  // invisible — so the bar reuses those lifted colours rather than inventing
  // a second set of dark values.
  const surface = isDarkColorScheme ? colors.inverse : colors.white;
  const activeTint = isDarkColorScheme ? colors.inverseInfo : colors.primary;
  const restTint = isDarkColorScheme ? colors.inverseMuted : colors.gray600;
  const activeLabel = isDarkColorScheme ? colors.white : colors.black;
  const restLabel = isDarkColorScheme ? colors.inverseMuted : colors.gray500;

  return (
    <View style={[tw`absolute inset-x-4`, { bottom: insets.bottom + -12 }]}>
      <View
        style={[
          tw`h-[70px] flex-row items-center justify-between rounded-full px-4`,
          {
            backgroundColor: surface,
            shadowColor: colors.black,
            shadowOpacity: 0.08,
            shadowRadius: 12,
            shadowOffset: { width: 0, height: 4 },
            elevation: 6,
          },
          // A black shadow does nothing against a dark background, so the pill
          // needs an edge instead to sit off the screen behind it.
          isDarkColorScheme && { borderWidth: 1, borderColor: colors.inverseBorder },
        ]}>
        {state.routes
          .filter((route) => route.name !== 'chats')
          .map((route) => {
            const index = state.routes.indexOf(route);
            const isFocused = state.index === index;
            const isPostTab = route.name === 'post';
            const { Icon, ActiveIcon } =
              isPostTab && isSidekick
                ? { Icon: TasksTabIcon, ActiveIcon: TasksTabActiveIcon }
                : (ICONS[route.name] ?? { Icon: MyTaskIcon, ActiveIcon: MyTaskActiveIcon });
            const label =
              isPostTab && isSidekick
                ? 'Tasks'
                : (descriptors[route.key]?.options.title ?? route.name);

            function onPress() {
              // SDK 58 reworked the navigator core: the tab bar is handed an
              // `emitter` and a `navigateToTab` instead of a `navigation`
              // object, and tabs are addressed by route key rather than name.
              const event = emitter.emit({
                type: 'tabPress',
                target: route.key,
                canPreventDefault: true,
              });
              if (!isFocused && !event.defaultPrevented) {
                navigateToTab(route.key);
              }
            }

            return (
              <PressableScale
                key={route.key}
                onPress={onPress}
                style={tw`flex-1 items-center gap-1`}>
                <View style={tw`items-center justify-center`}>
                  {isFocused ? (
                    <ActiveIcon color={activeTint} knockout={surface} />
                  ) : (
                    <Icon color={restTint} />
                  )}
                </View>
                <Text
                  variant="caption"
                  classN={`text-[${twColor(isFocused ? activeLabel : restLabel)}]`}>
                  {label}
                </Text>
              </PressableScale>
            );
          })}
      </View>
    </View>
  );
}
