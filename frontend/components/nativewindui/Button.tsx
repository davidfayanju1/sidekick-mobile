import * as Slot from '@rn-primitives/slot';
import { cva, type VariantProps } from 'class-variance-authority';
import {
  Platform,
  Pressable,
  PressableProps,
  PressableStateCallbackType,
  StyleProp,
  TextStyle,
  View,
  ViewStyle,
} from 'react-native';

import { TextClassContext } from '@/components/nativewindui/Text';
import { tw } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';
import { COLORS } from '@/theme/colors';
import { withOpacity } from '@/theme/with-opacity';

const buttonSizeVariants = cva('flex-row items-center justify-center gap-2', {
  variants: {
    size: {
      none: '',
      sm: 'py-1 px-2.5 rounded-full',
      md: 'py-2 ios:py-1.5 ios:px-3.5 px-5 rounded-full',
      lg: 'py-2.5 px-5 ios:py-2 rounded-full gap-2',
      icon: 'h-10 w-10 rounded-full',
    },
  },
  defaultVariants: {
    size: 'md',
  },
});

const androidRootVariants = cva('overflow-hidden', {
  variants: {
    size: {
      none: '',
      icon: 'rounded-full',
      sm: 'rounded-full',
      md: 'rounded-full',
      lg: 'rounded-xl',
    },
  },
  defaultVariants: {
    size: 'md',
  },
});

const buttonTextSizeVariants = cva('font-medium', {
  variants: {
    size: {
      none: '',
      icon: '',
      sm: 'text-[15px] leading-5',
      md: 'text-[17px] leading-7',
      lg: 'text-[17px] leading-7',
    },
  },
  defaultVariants: {
    size: 'md',
  },
});

const ANDROID_RIPPLE = {
  dark: {
    primary: { color: withOpacity(COLORS.dark.grey3, 0.4), borderless: false },
    secondary: { color: withOpacity(COLORS.dark.grey5, 0.8), borderless: false },
    plain: { color: withOpacity(COLORS.dark.grey5, 0.8), borderless: false },
    tonal: { color: withOpacity(COLORS.dark.grey5, 0.8), borderless: false },
  },
  light: {
    primary: { color: withOpacity(COLORS.light.grey4, 0.4), borderless: false },
    secondary: { color: withOpacity(COLORS.light.grey5, 0.4), borderless: false },
    plain: { color: withOpacity(COLORS.light.grey5, 0.4), borderless: false },
    tonal: { color: withOpacity(COLORS.light.grey6, 0.4), borderless: false },
  },
};

const BORDER_CURVE: ViewStyle = {
  borderCurve: 'continuous',
};

type ButtonVariant = 'primary' | 'secondary' | 'tonal' | 'plain';

type ButtonThemeColors = { primary: string; foreground: string };

function getButtonVariantStyle(
  variant: ButtonVariant,
  colors: ButtonThemeColors,
  isDarkColorScheme: boolean,
  pressed: boolean
): ViewStyle {
  const isIOS = Platform.OS === 'ios';
  switch (variant) {
    case 'primary':
      return {
        backgroundColor: colors.primary,
        ...(isIOS && pressed ? { opacity: 0.8 } : null),
      };
    case 'secondary':
      return {
        borderWidth: 1,
        borderColor: isIOS ? colors.primary : withOpacity(colors.foreground, 0.4),
        ...(isIOS && pressed ? { backgroundColor: withOpacity(colors.primary, 0.05) } : null),
      };
    case 'tonal': {
      const baseOpacity = isIOS ? 0.1 : isDarkColorScheme ? 0.3 : 0.15;
      const pressedOpacity = isIOS ? 0.15 : baseOpacity;
      return {
        backgroundColor: withOpacity(colors.primary, pressed ? pressedOpacity : baseOpacity),
      };
    }
    case 'plain':
      return isIOS && pressed ? { opacity: 0.7 } : {};
  }
}

function getButtonTextStyle(variant: ButtonVariant, colors: ButtonThemeColors): TextStyle {
  const isIOS = Platform.OS === 'ios';
  switch (variant) {
    case 'primary':
      return { color: COLORS.white };
    case 'secondary':
    case 'tonal':
      return { color: isIOS ? colors.primary : colors.foreground };
    case 'plain':
      return { color: colors.foreground };
  }
}

type ButtonVariantProps = Omit<VariantProps<typeof buttonSizeVariants>, never> & {
  variant?: ButtonVariant;
};

type AndroidOnlyButtonProps = {
  /**
   * ANDROID ONLY: extra style for the root responsible for hiding the ripple overflow.
   */
  androidRootStyle?: StyleProp<ViewStyle>;
};

type ButtonProps = PressableProps & ButtonVariantProps & AndroidOnlyButtonProps;

const Root = Platform.OS === 'android' ? View : Slot.Pressable;

function Button({ variant = 'primary', size, style, androidRootStyle, ...props }: ButtonProps) {
  const { colorScheme, colors: systemColors, isDarkColorScheme } = useColorScheme();

  const textStyle = tw.style(
    buttonTextSizeVariants({ size }),
    getButtonTextStyle(variant, systemColors)
  );

  return (
    <TextClassContext.Provider value={textStyle}>
      <Root
        style={
          Platform.OS === 'android'
            ? [tw.style(androidRootVariants({ size })), androidRootStyle]
            : undefined
        }>
        <Pressable
          style={(state: PressableStateCallbackType) => [
            tw.style(buttonSizeVariants({ size })),
            getButtonVariantStyle(variant, systemColors, isDarkColorScheme, state.pressed),
            props.disabled && { opacity: 0.5 },
            BORDER_CURVE,
            typeof style === 'function' ? style(state) : style,
          ]}
          android_ripple={ANDROID_RIPPLE[colorScheme][variant]}
          {...props}
        />
      </Root>
    </TextClassContext.Provider>
  );
}

export { Button, getButtonTextStyle, getButtonVariantStyle };
export type { ButtonProps };
