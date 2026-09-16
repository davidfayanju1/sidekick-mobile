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
  const { colors: systemColors } = useColorScheme();
  return (
    <View
      style={[
        tw`ios:shadow-sm ios:shadow-black/5 rounded-md border`,
        { borderColor: systemColors.background, backgroundColor: systemColors.background },
        containerStyle,
      ]}>
      <RNPicker
        mode={mode}
        style={
          style ?? {
            backgroundColor: systemColors.root,
            borderRadius: 8,
          }
        }
        {...(Platform.OS === 'android'
          ? {
              dropdownIconColor: dropdownIconColor ?? systemColors.foreground,
              dropdownIconRippleColor: dropdownIconRippleColor ?? systemColors.foreground,
            }
          : {})}
        {...props}
      />
    </View>
  );
}

export const PickerItem = RNPicker.Item;
