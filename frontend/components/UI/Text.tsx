import React from 'react';
import { Text as RNText } from 'react-native';
import { moderateScale } from 'react-native-size-matters';

import { tw } from '@/lib/tw';
import {
  fontSize as SIZES,
  letterSpacing as TRACKING,
  lineHeight as LEADING,
  variantWeight,
  type FontWeight,
  type TextVariant,
} from '@/theme/typography';

const FONT_FAMILY: Record<FontWeight, string> = {
  light: 'Satoshi-Light',
  normal: 'Satoshi-Regular',
  medium: 'Satoshi-Medium',
  bold: 'Satoshi-Bold',
  black: 'Satoshi-Black',
};

interface TextProps {
  children: string | React.ReactNode;
  classN?: string;
  /** Preferred API: sets size, weight, line height and tracking together. */
  variant?: TextVariant;
  /** Escape hatch — overrides the variant's size. */
  fontSize?: number;
  /** Overrides the variant's default weight. */
  fontWeight?: FontWeight;
  numberOfLines?: number;
  style?: string;
  elipsizeMode?: 'head' | 'middle' | 'tail';
}

const Text = ({
  children,
  classN = '',
  variant = 'body',
  fontSize,
  fontWeight,
  numberOfLines,
  style,
  elipsizeMode,
  ...props
}: TextProps) => {
  const size = fontSize ?? SIZES[variant];
  const weight = fontWeight ?? variantWeight[variant];
  const scaled = moderateScale(size);

  return (
    <RNText
      ellipsizeMode={elipsizeMode}
      numberOfLines={numberOfLines}
      allowFontScaling={false}
      style={[
        tw`${classN}`,
        {
          fontFamily: FONT_FAMILY[weight],
          fontSize: scaled,
          lineHeight: scaled * LEADING[variant],
          letterSpacing: TRACKING[variant],
        },
      ]}
      {...props}>
      {children}
    </RNText>
  );
};

export default Text;
