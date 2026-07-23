import { Picker as RNPicker } from '@react-native-picker/picker';
import { Platform, View } from 'react-native';

import { tw } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';

export function Picker<T>({
  mode = 'dropdown',
  style,
  dropdownIconColor,
  dropdownIconRippleColor,
  containerStyle,
  ...props
}: React.ComponentProps<typeof RNPicker<T>> & {
  containerStyle?: React.ComponentProps<typeof View>['style'];
}) {
  const { colors } = useColorScheme();
  return (
    <View
      style={[
        tw`ios:shadow-sm ios:shadow-black/5 rounded-md border`,
        { borderColor: colors.background, backgroundColor: colors.background },
        containerStyle,
      ]}>
      <RNPicker
        mode={mode}
        style={
          style ?? {
            backgroundColor: colors.root,
            borderRadius: 8,
          }
        }
        {...(Platform.OS === 'android'
          ? {
              dropdownIconColor: dropdownIconColor ?? colors.foreground,
              dropdownIconRippleColor: dropdownIconRippleColor ?? colors.foreground,
            }
          : {})}
        {...props}
      />
    </View>
  );
}

export const PickerItem = RNPicker.Item;
