import DateTimePicker, { type DateTimePickerEvent } from '@react-native-community/datetimepicker';
import * as React from 'react';
import type { StyleProp, ViewStyle } from 'react-native';

type DatePickerProps = {
  value: Date;
  mode: 'date' | 'time' | 'datetime';
  minimumDate?: Date;
  maximumDate?: Date;
  onValueChange?: (event: DateTimePickerEvent, date?: Date) => void;
  onChange?: (event: DateTimePickerEvent, date?: Date) => void;
  materialDateStyle?: StyleProp<ViewStyle>;
  materialDateLabel?: string;
  materialDateLabelStyle?: StyleProp<ViewStyle>;
  materialTimeStyle?: StyleProp<ViewStyle>;
  materialTimeLabel?: string;
  materialTimeLabelStyle?: StyleProp<ViewStyle>;
};

export function DatePicker({
  onValueChange,
  onChange,
  materialDateStyle: _materialDateStyle,
  materialDateLabel: _materialDateLabel,
  materialDateLabelStyle: _materialDateLabelStyle,
  materialTimeStyle: _materialTimeStyle,
  materialTimeLabel: _materialTimeLabel,
  materialTimeLabelStyle: _materialTimeLabelStyle,
  ...props
}: DatePickerProps) {
  return (
    <DateTimePicker
      {...props}
      onChange={(event, date) => {
        onValueChange?.(event, date);
        onChange?.(event, date);
      }}
    />
  );
}
