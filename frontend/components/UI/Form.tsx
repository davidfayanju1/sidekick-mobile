import { Ionicons } from '@expo/vector-icons';
import Feather from '@expo/vector-icons/Feather';
import React, { useState } from 'react';
import { TextInput, TouchableOpacity, View, type KeyboardTypeOptions } from 'react-native';
import Animated, {
  FadeIn,
  FadeOut,
  interpolateColor,
  useAnimatedStyle,
  useDerivedValue,
  withSpring,
  withTiming,
} from 'react-native-reanimated';

import { tw } from '@/lib/tw';
import { colors } from '@/theme/palette';

import Text from './Text';

/**
 * How much of the error message's height is absorbed by the gap that already
 * exists between fields. Roughly one caption line minus a few points of
 * breathing room, so a single-line error costs ~7px instead of ~21px.
 */
const ERROR_GUTTER_PULL = 12;

interface FormProps {
  placeholder: string;
  value?: string;
  containerStyle?: string;
  label?: string;
  icon?: boolean;
  formStyle?: string;
  type?: string;
  onChangeText?: (text: string) => void;
  onBlur?: () => void;
  onFocus?: () => void;
  labelFontSize?: number;
  labelStyle?: string;
  inputStyle?: string;
  error?: string;
  editable?: boolean;
  multiline?: boolean;
  maxLength?: number;
  returnKeyType?: 'done' | 'go' | 'next' | 'search' | 'send';
  onSubmitEditing?: () => void;
  keyboardType?: KeyboardTypeOptions;
  autoCapitalize?: 'none' | 'sentences' | 'words' | 'characters';
  textContentType?:
    | 'none'
    | 'password'
    | 'newPassword'
    | 'username'
    | 'emailAddress'
    | 'name'
    | 'telephoneNumber'
    | 'oneTimeCode';
}

const Form = ({
  error,
  inputStyle = '',
  labelStyle,
  labelFontSize = 11,
  type,
  icon = false,
  placeholder,
  value,
  containerStyle = '',
  label,
  formStyle = 'h-[3rem]',
  onChangeText,
  onBlur,
  onFocus,
  editable = true,
  multiline = false,
  maxLength,
  returnKeyType,
  onSubmitEditing,
  keyboardType = 'default',
  autoCapitalize = 'none',
  textContentType,
}: FormProps) => {
  const [secureTextEntry, setSecureTextEntry] = useState(type === 'password');
  const [focused, setFocused] = useState(false);

  // 0 resting, 1 focused, 2 errored — one value so the colours can't fight.
  const state = useDerivedValue(() => withTiming(error ? 2 : focused ? 1 : 0, { duration: 180 }));

  const borderStyle = useAnimatedStyle(() => ({
    borderColor: interpolateColor(
      state.value,
      [0, 1, 2],
      [colors.gray300, colors.primary, colors.danger]
    ),
    // A focused field lifts a hair off the page.
    transform: [{ scale: withSpring(focused ? 1.004 : 1, { damping: 20, stiffness: 300 }) }],
  }));

  return (
    <View style={tw`${containerStyle}`}>
      {label && (
        <View>
          <Text fontWeight="medium" classN={labelStyle} fontSize={labelFontSize}>
            {label}
          </Text>
        </View>
      )}

      <Animated.View
        style={[
          tw`border-solid ${
            icon || type === 'password' ? 'flex-row items-center' : ''
          } mt-2 w-full rounded-full border px-4 ${formStyle}`,
          multiline ? tw`rounded-2xl py-3` : undefined,
          !editable ? { backgroundColor: colors.gray50 } : undefined,
          borderStyle,
        ]}>
        {icon && (
          <View style={tw`mr-2`}>
            <Feather name="search" size={24} color={colors.gray500} />
          </View>
        )}

        <TextInput
          placeholder={placeholder}
          placeholderTextColor={colors.gray500}
          style={tw`flex-1 text-[.9rem] text-black ${inputStyle}`}
          value={value}
          onChangeText={onChangeText}
          onFocus={() => {
            setFocused(true);
            onFocus?.();
          }}
          onBlur={() => {
            setFocused(false);
            onBlur?.();
          }}
          editable={editable}
          multiline={multiline}
          maxLength={maxLength}
          returnKeyType={returnKeyType}
          onSubmitEditing={onSubmitEditing}
          secureTextEntry={type === 'password' ? secureTextEntry : false}
          keyboardType={keyboardType}
          autoCapitalize={autoCapitalize}
          autoCorrect={false}
          textContentType={textContentType ?? (type === 'password' ? 'none' : undefined)}
        />

        {type === 'password' && (
          <TouchableOpacity onPress={() => setSecureTextEntry((current) => !current)} hitSlop={8}>
            <Ionicons
              name={secureTextEntry ? 'eye-off-outline' : 'eye-outline'}
              size={20}
              color={colors.gray800}
            />
          </TouchableOpacity>
        )}
      </Animated.View>

      {error ? (
        <Animated.View
          entering={FadeIn.duration(160)}
          exiting={FadeOut.duration(120)}
          // The message sits *in* the gap between fields rather than adding to
          // it. Call sites space fields with mt-5/mt-6 (20–24px); without this
          // pull-up an error stacked its own height on top of that and the gap
          // more than doubled. A wrapped two-line message still pushes the next
          // field down, so this can tighten the layout but never overlap it.
          style={{ marginTop: 2, marginBottom: -ERROR_GUTTER_PULL }}>
          <Text variant="caption" classN={`ml-4 text-[${colors.danger}]`}>
            {error}
          </Text>
        </Animated.View>
      ) : null}
    </View>
  );
};

export default Form;
