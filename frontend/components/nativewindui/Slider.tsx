import RNSlider from '@react-native-community/slider';
import { Platform } from 'react-native';

import { useColorScheme } from '@/lib/useColorScheme';
import { COLORS } from '@/theme/colors';

function Slider({
  thumbTintColor,
  minimumTrackTintColor,
  maximumTrackTintColor,
  ...props
}: React.ComponentProps<typeof RNSlider>) {
  const { colors: systemColors } = useColorScheme();
  return (
    <RNSlider
      thumbTintColor={
        (thumbTintColor ?? Platform.OS === 'ios') ? COLORS.white : systemColors.primary
      }
      minimumTrackTintColor={minimumTrackTintColor ?? systemColors.primary}
      maximumTrackTintColor={
        (maximumTrackTintColor ?? Platform.OS === 'android') ? systemColors.primary : undefined
      }
      minimumValue={0}
      maximumValue={1}
      {...props}
    />
  );
}

export { Slider };
