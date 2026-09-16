import { Switch } from 'react-native';

import { useColorScheme } from '@/lib/useColorScheme';
import { COLORS } from '@/theme/colors';

function Toggle(props: React.ComponentProps<typeof Switch>) {
  const { colors: systemColors } = useColorScheme();
  return (
    <Switch
      trackColor={{
        true: systemColors.primary,
        false: systemColors.grey,
      }}
      thumbColor={COLORS.white}
      {...props}
    />
  );
}

export { Toggle };
