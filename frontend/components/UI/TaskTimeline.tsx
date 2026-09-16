import { Check } from 'lucide-react-native';
import * as React from 'react';
import { View } from 'react-native';
import Animated, {
  useAnimatedStyle,
  useSharedValue,
  withRepeat,
  withTiming,
} from 'react-native-reanimated';

import Text from '@/components/UI/Text';
import { ACCENT_TEAL } from '@/components/UI/TaskParts';
import { tw, twColor } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';

function usePulseStyle(maxScale: number) {
  const scale = useSharedValue(1);
  const opacity = useSharedValue(0.7);

  React.useEffect(() => {
    scale.value = withRepeat(withTiming(maxScale, { duration: 1400 }), -1, false);
    opacity.value = withRepeat(withTiming(0, { duration: 1400 }), -1, false);
  }, [maxScale, opacity, scale]);

  return useAnimatedStyle(() => ({
    transform: [{ scale: scale.value }],
    opacity: opacity.value,
  }));
}

/** A small pulsing "this is live" dot, meant to sit beside a status pill. */
export function LiveDot() {
  const pulseStyle = usePulseStyle(2.2);

  return (
    <View style={tw`h-2.5 w-2.5 items-center justify-center`}>
      <Animated.View
        style={[
          tw`absolute h-2.5 w-2.5 rounded-full`,
          { backgroundColor: ACCENT_TEAL },
          pulseStyle,
        ]}
      />
      <View style={[tw`h-2 w-2 rounded-full`, { backgroundColor: ACCENT_TEAL }]} />
    </View>
  );
}

function PulsingRing() {
  const pulseStyle = usePulseStyle(1.8);
  return (
    <Animated.View
      style={[tw`absolute h-8 w-8 rounded-full`, { backgroundColor: ACCENT_TEAL }, pulseStyle]}
    />
  );
}

export type TimelineStep = {
  title: string;
  description: string;
  timestamp?: string;
};

export function TaskTimeline({
  steps,
  currentStep,
}: {
  steps: TimelineStep[];
  currentStep: number;
}) {
  const { colors: systemColors } = useColorScheme();
  const fg = twColor(systemColors.foreground);
  const muted = twColor(systemColors.mutedForeground);

  return (
    <View style={tw`mt-1`}>
      {steps.map((step, index) => {
        const isDone = index < currentStep;
        const isCurrent = index === currentStep;
        const isLast = index === steps.length - 1;
        const isReached = isDone || isCurrent;

        return (
          <View key={step.title} style={tw`flex-row`}>
            <View style={[tw`items-center`, { width: 32 }]}>
              <View style={tw`h-8 w-8 items-center justify-center`}>
                {isCurrent && <PulsingRing />}
                <View
                  style={[
                    tw`h-8 w-8 items-center justify-center rounded-full`,
                    isReached
                      ? { backgroundColor: ACCENT_TEAL }
                      : {
                          borderWidth: 1.5,
                          borderColor: systemColors.grey4,
                          backgroundColor: systemColors.card,
                        },
                  ]}>
                  {isDone ? (
                    <Check size={15} color="white" strokeWidth={3} />
                  ) : (
                    <View
                      style={[
                        tw`h-2 w-2 rounded-full`,
                        { backgroundColor: isCurrent ? 'white' : systemColors.grey4 },
                      ]}
                    />
                  )}
                </View>
              </View>
              {!isLast && (
                <View
                  style={[
                    tw`w-px flex-1`,
                    { backgroundColor: isDone ? ACCENT_TEAL : systemColors.grey4, minHeight: 22 },
                  ]}
                />
              )}
            </View>

            <View style={tw`flex-1 pb-5 pl-3`}>
              <Text
                fontWeight={isCurrent ? 'bold' : 'medium'}
                fontSize={14}
                classN={`text-[${isReached ? fg : muted}]`}>
                {step.title}
              </Text>
              {isReached && (
                <Text fontSize={12} classN={`mt-0.5 text-[${muted}]`}>
                  {step.description}
                </Text>
              )}
              {step.timestamp && (
                <Text
                  fontWeight="medium"
                  fontSize={11}
                  classN={`mt-1 text-[${twColor(ACCENT_TEAL)}]`}>
                  {step.timestamp}
                </Text>
              )}
            </View>
          </View>
        );
      })}
    </View>
  );
}
