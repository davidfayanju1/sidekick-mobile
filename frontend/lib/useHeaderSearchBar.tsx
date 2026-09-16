import { useNavigation } from 'expo-router';
import * as React from 'react';
import type { SearchBarProps } from 'react-native-screens';

import { useColorScheme } from './useColorScheme';

function useHeaderSearchBar(props: SearchBarProps = {}) {
  const { colorScheme, colors: systemColors } = useColorScheme();
  const navigation = useNavigation();
  const [search, setSearch] = React.useState('');

  React.useLayoutEffect(() => {
    navigation.setOptions({
      headerSearchBarOptions: {
        placeholder: 'Search...',
        textColor: systemColors.foreground,
        tintColor: systemColors.primary,
        headerIconColor: systemColors.foreground,
        hintTextColor: systemColors.grey,
        hideWhenScrolling: false,
        onChangeText(event) {
          setSearch(event.nativeEvent.text);
        },
        ...props,
      } satisfies SearchBarProps,
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [navigation, colorScheme]);

  return search;
}

export { useHeaderSearchBar };
