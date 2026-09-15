import { VariantProps, cva } from 'class-variance-authority';
import * as React from 'react';
import type { StyleProp, TextStyle } from 'react-native';
import { UITextView } from 'react-native-uitextview';

import { tw } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';
import { withOpacity } from '@/theme/with-opacity';

const textVariants = cva('', {
  variants: {
    variant: {
      largeTitle: 'text-4xl',
      title1: 'text-2xl',
      title2: 'text-[22px] leading-7',
      title3: 'text-xl',
      heading: 'text-[17px] leading-6 font-semibold',
      body: 'text-[17px] leading-6',
      callout: 'text-base',
      subhead: 'text-[15px] leading-6',
      footnote: 'text-[13px] leading-5',
      caption1: 'text-xs',
      caption2: 'text-[11px] leading-4',
    },
  },
  defaultVariants: {
    variant: 'body',
  },
});

type TextColor = 'primary' | 'secondary' | 'tertiary' | 'quarternary';

const TextClassContext = React.createContext<StyleProp<TextStyle> | undefined>(undefined);

function Text({
  style,
  variant,
  color = 'primary',
  ...props
}: React.ComponentProps<typeof UITextView> &
  VariantProps<typeof textVariants> & { color?: TextColor }) {
  const contextStyle = React.useContext(TextClassContext);
  const { colors } = useColorScheme();

  const colorStyle: TextStyle = {
    color:
      color === 'secondary'
        ? withOpacity(colors.secondaryForeground, 0.9)
        : color === 'tertiary'
          ? withOpacity(colors.mutedForeground, 0.9)
          : color === 'quarternary'
            ? withOpacity(colors.mutedForeground, 0.5)
            : colors.foreground,
  };

  return (
    <UITextView
      style={[tw.style(textVariants({ variant })), colorStyle, contextStyle, style]}
      {...props}
    />
  );
}

export { Text, TextClassContext, textVariants };
