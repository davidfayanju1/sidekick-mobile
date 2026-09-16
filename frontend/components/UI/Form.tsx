import { View, TextInput, TouchableOpacity, KeyboardTypeOptions } from 'react-native';
import React, { useState } from 'react';
import Feather from '@expo/vector-icons/Feather';
import { Ionicons } from '@expo/vector-icons';

import { tw } from '@/lib/tw';

import Text from './Text';
import { colors } from '@/theme/palette';

interface FormProps {
  placeholder: string;
  value?: string;
  containerStyle?: string;
  label?: string;
  icon?: boolean;
  formStyle?: string;
  type?: string;
  onChangeText?: (text: string) => void;
  labelFontSize?: number;
  labelStyle?: string;
  inputStyle?: string;
  error?: string;
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
  keyboardType = 'default',
  autoCapitalize = 'none',
  textContentType,
}: FormProps) => {
  const [secureTextEntry, setSecureTextEntry] = useState(type === 'password');

  const toggleSecureEntry = () => {
    setSecureTextEntry(!secureTextEntry);
  };

  return (
    <View style={tw`${containerStyle}`}>
      {label && (
        <View>
          <Text fontWeight="medium" classN={labelStyle} fontSize={labelFontSize}>
            {label}
          </Text>
        </View>
      )}
      <View
        style={tw`border-solid ${
          icon || type === 'password' ? 'flex-row items-center' : ''
        } mt-2 px-4 border-[1px] border-[${colors.border}] rounded-full ${formStyle} w-full`}>
        {icon && (
          <View style={tw`mr-2`}>
            <Feather name="search" size={24} color="gray" />
          </View>
        )}
        <TextInput
          placeholder={placeholder}
          placeholderTextColor={colors.textPlaceholder}
          style={tw`flex-1 text-[.9rem] text-black ${inputStyle}`}
          value={value}
          onChangeText={onChangeText}
          secureTextEntry={type === 'password' ? secureTextEntry : false}
          keyboardType={keyboardType}
          autoCapitalize={autoCapitalize}
          autoCorrect={false}
          textContentType={textContentType ?? (type === 'password' ? 'none' : undefined)}
        />
        {type === 'password' && (
          <TouchableOpacity onPress={toggleSecureEntry}>
            <Ionicons
              name={secureTextEntry ? 'eye-off-outline' : 'eye-outline'}
              size={20}
              color={colors.iconForm}
            />
          </TouchableOpacity>
        )}
      </View>

      {error && (
        <Text classN="text-red-600" fontSize={12}>
          {error}
        </Text>
      )}
    </View>
  );
};

export default Form;
