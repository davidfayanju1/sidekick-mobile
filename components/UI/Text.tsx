import React from 'react';
import { Text as RNText } from 'react-native';
import { moderateScale } from 'react-native-size-matters';

import { tw } from '@/lib/tw';

interface TextProps {
  children: string | React.ReactNode;
  classN?: string;
  fontSize?: number;
  fontWeight?: 'light' | 'normal' | 'medium' | 'bold' | 'black';
  numberOfLines?: number;
  style?: string;
  elipsizeMode?: 'head' | 'middle' | 'tail';
}

const Text = ({
  children,
  classN = '',
  fontSize,
  fontWeight = 'normal',
  numberOfLines,
  style,
  elipsizeMode,
  ...props
}: TextProps) => {
  // Map font weights to the actual font files loaded from assets/fonts/satoshi
  const getFontFamily = () => {
    switch (fontWeight) {
      case 'light':
        return 'Satoshi-Light';
      case 'medium':
        return 'Satoshi-Medium';
      case 'bold':
        return 'Satoshi-Bold';
      case 'black':
        return 'Satoshi-Black';
      case 'normal':
      default:
        return 'Satoshi-Regular';
    }
  };

  return (
    <RNText
      ellipsizeMode={elipsizeMode}
      numberOfLines={numberOfLines}
      allowFontScaling={false}
      style={[
        tw`${classN}`,
        {
          fontFamily: getFontFamily(),
          fontSize: fontSize ? moderateScale(fontSize) : undefined,
        },
        // style,
      ]}
      {...props}>
      {children}
    </RNText>
  );
};

export default Text;
