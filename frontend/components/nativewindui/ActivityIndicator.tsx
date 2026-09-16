import { ActivityIndicator as RNActivityIndicator } from 'react-native';

import { useColorScheme } from '@/lib/useColorScheme';

function ActivityIndicator(props: React.ComponentProps<typeof RNActivityIndicator>) {
  const { colors: systemColors } = useColorScheme();
  return <RNActivityIndicator color={systemColors.primary} {...props} />;
}

export { ActivityIndicator };
