import {
  DateTimePickerAndroid,
  type DateTimePickerEvent,
} from '@react-native-community/datetimepicker';
import * as React from 'react';
import { View, type StyleProp, type ViewStyle } from 'react-native';

import { Button } from '@/components/nativewindui/Button';
import { Text } from '@/components/nativewindui/Text';
import { tw } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';
import { withOpacity } from '@/theme/with-opacity';

type DatePickerProps = {
  value: Date;
  mode: 'date' | 'time' | 'datetime';
  minimumDate?: Date;
  maximumDate?: Date;
  onValueChange?: (event: DateTimePickerEvent, date?: Date) => void;
  onChange?: (event: DateTimePickerEvent, date?: Date) => void;
  onDismiss?: (event: DateTimePickerEvent) => void;
  onNeutralButtonPress?: (event: DateTimePickerEvent) => void;
  materialDateStyle?: StyleProp<ViewStyle>;
  materialDateLabel?: string;
  materialDateLabelStyle?: StyleProp<ViewStyle>;
  materialTimeStyle?: StyleProp<ViewStyle>;
  materialTimeLabel?: string;
  materialTimeLabelStyle?: StyleProp<ViewStyle>;
};

export function DatePicker(props: DatePickerProps) {
  const { colors } = useColorScheme();

  const show = (currentMode: 'time' | 'date') => () => {
    DateTimePickerAndroid.open({
      value: props.value,
      mode: currentMode,
      minimumDate: props.minimumDate,
      maximumDate: props.maximumDate,
      onChange: (event, selectedDate) => {
        if (event.type === 'dismissed') {
          props.onDismiss?.(event);
          return;
        }
        if (event.type === 'neutralButtonPressed') {
          props.onNeutralButtonPress?.(event);
          return;
        }
        props.onValueChange?.(event, selectedDate);
        props.onChange?.(event, selectedDate);
      },
    });
  };

  return (
    <View style={tw`flex-row gap-2.5`}>
      {props.mode.includes('date') && (
        <View style={[tw`relative pt-1.5`, props.materialDateStyle]}>
          <Button
            variant="plain"
            androidRootStyle={tw`rounded-none`}
            onPress={show('date')}
            style={({ pressed }) => [
              tw`rounded px-2.5 py-3`,
              { borderWidth: 1, borderColor: withOpacity(colors.foreground, 0.3) },
              pressed && tw`opacity-80`,
            ]}>
            <Text style={tw`py-px text-sm`}>
              {new Intl.DateTimeFormat('en-US', {
                dateStyle: 'medium',
              }).format(props.value)}
            </Text>
          </Button>
          <View
            style={[
              tw`absolute left-2 top-0 px-1`,
              { backgroundColor: colors.card },
              props.materialDateLabelStyle,
            ]}>
            <Text variant="caption2" style={tw`text-[10px] opacity-60`}>
              {props.materialDateLabel ?? 'Date'}
            </Text>
          </View>
        </View>
      )}
      {props.mode.includes('time') && (
        <View style={[tw`relative pt-1.5`, props.materialTimeStyle]}>
          <Button
            variant="plain"
            androidRootStyle={tw`rounded-none`}
            onPress={show('time')}
            style={({ pressed }) => [
              tw`rounded px-2.5 py-3`,
              { borderWidth: 1, borderColor: withOpacity(colors.foreground, 0.3) },
              pressed && tw`opacity-80`,
            ]}>
            <Text style={tw`py-px text-sm`}>
              {new Intl.DateTimeFormat('en-US', {
                timeStyle: 'short',
              }).format(props.value)}
            </Text>
          </Button>
          <View
            style={[
              tw`absolute left-2 top-0 px-1`,
              { backgroundColor: colors.card },
              props.materialTimeLabelStyle,
            ]}>
            <Text variant="caption2" style={tw`text-[10px] opacity-60`}>
              {props.materialTimeLabel ?? 'Time'}
            </Text>
          </View>
        </View>
      )}
    </View>
  );
}
