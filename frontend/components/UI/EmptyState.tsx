import * as React from 'react';
import { View, type StyleProp, type ViewStyle } from 'react-native';
import Animated, { FadeIn, FadeInDown } from 'react-native-reanimated';

import { Button } from '@/components/UI/Button';
import Text from '@/components/UI/Text';
import { tw } from '@/lib/tw';
import { colors } from '@/theme/palette';

export interface EmptyStateProps {
  /** An animated scene from `components/UI/illustrations`. */
  illustration?: React.ReactNode;
  title: string;
  /** One or two sentences. Say what to do next, not that there's nothing here. */
  body?: string;
  action?: { label: string; onPress: () => void };
  /** A quieter second option, e.g. "Learn more". */
  secondaryAction?: { label: string; onPress: () => void };
  compact?: boolean;
  style?: StyleProp<ViewStyle>;
}

/**
 * The shell every empty state uses.
 *
 * Copy rule: the title states the situation in the user's terms and the body
 * gives them the next move. "No tasks nearby yet" beats "Empty"; "Widen your
 * radius or check back shortly" beats "Try again later".
 *
 * The scene fades in first and the words follow it up — the eye lands on the
 * illustration, then reads.
 */
export function EmptyState({
  illustration,
  title,
  body,
  action,
  secondaryAction,
  compact = false,
  style,
}: EmptyStateProps) {
  return (
    <View
      style={[tw`items-center justify-center px-8`, compact ? tw`py-8` : tw`flex-1 py-16`, style]}>
      {illustration ? (
        <Animated.View entering={FadeIn.duration(420)} style={tw`mb-5`}>
          {illustration}
        </Animated.View>
      ) : null}

      <Animated.View entering={FadeInDown.delay(120).duration(360).springify().damping(20)}>
        <Text variant="subhead" fontWeight="bold" classN={`text-center text-[${colors.gray900}]`}>
          {title}
        </Text>
      </Animated.View>

      {body ? (
        <Animated.View entering={FadeInDown.delay(190).duration(360).springify().damping(20)}>
          <Text variant="body" classN={`mt-2 text-center text-[${colors.gray600}]`}>
            {body}
          </Text>
        </Animated.View>
      ) : null}

      {action ? (
        <Animated.View
          entering={FadeInDown.delay(260).duration(360).springify().damping(20)}
          style={tw`mt-6 w-full max-w-[240px]`}>
          <Button label={action.label} onPress={action.onPress} size="md" />
        </Animated.View>
      ) : null}

      {secondaryAction ? (
        <Animated.View
          entering={FadeInDown.delay(320).duration(360).springify().damping(20)}
          style={tw`mt-1 w-full max-w-[240px]`}>
          <Button
            label={secondaryAction.label}
            onPress={secondaryAction.onPress}
            variant="ghost"
            size="sm"
          />
        </Animated.View>
      ) : null}
    </View>
  );
}

export interface ErrorStateProps {
  title?: string;
  body?: string;
  onRetry?: () => void;
  retrying?: boolean;
  compact?: boolean;
}

/**
 * What a screen shows when its query failed.
 *
 * Kept visually distinct from an empty state — a broken thing and an empty
 * thing are different problems, and only one of them has a retry button.
 */
export function ErrorState({
  title = 'That didn’t load',
  body = 'Check your connection and try again.',
  onRetry,
  retrying = false,
  compact = false,
}: ErrorStateProps) {
  return (
    <View style={[tw`items-center justify-center px-8`, compact ? tw`py-10` : tw`flex-1 py-16`]}>
      <Animated.View entering={FadeIn.duration(300)}>
        <View
          style={[
            tw`mb-5 h-14 w-14 items-center justify-center rounded-full`,
            { backgroundColor: colors.dangerTint },
          ]}>
          <View style={[tw`h-6 w-0.5 rounded-full`, { backgroundColor: colors.danger }]} />
          <View
            style={[
              tw`absolute bottom-3.5 h-1 w-1 rounded-full`,
              { backgroundColor: colors.danger },
            ]}
          />
        </View>
      </Animated.View>

      <Text variant="subhead" fontWeight="bold" classN={`text-center text-[${colors.gray900}]`}>
        {title}
      </Text>
      <Text variant="body" classN={`mt-2 text-center text-[${colors.gray600}]`}>
        {body}
      </Text>

      {onRetry ? (
        <View style={tw`mt-6 w-full max-w-[200px]`}>
          <Button
            label="Try again"
            onPress={onRetry}
            loading={retrying}
            variant="outline"
            size="md"
          />
        </View>
      ) : null}
    </View>
  );
}

export default EmptyState;
