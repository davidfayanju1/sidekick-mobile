import { useColorScheme as useNativewindColorScheme } from 'nativewind';
import * as React from 'react';
import { useAppColorScheme } from 'twrnc';

import { tw } from '@/lib/tw';
import { COLORS } from '@/theme/colors';

function useColorScheme() {
  const { colorScheme, setColorScheme } = useNativewindColorScheme();
  const [, , setTwColorScheme] = useAppColorScheme(tw);

  const resolvedColorScheme = colorScheme ?? 'light';

  // Keep twrnc's `dark:` variant in sync with the app's own (manually toggled) color scheme.
  // NOTE: twrnc's returned setter isn't referentially stable (and unconditionally triggers its
  // own re-render on every call), so it must stay out of this effect's deps or it loops forever.
  React.useEffect(() => {
    setTwColorScheme(resolvedColorScheme);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [resolvedColorScheme]);

  function toggleColorScheme() {
    return setColorScheme(resolvedColorScheme === 'light' ? 'dark' : 'light');
  }

  return {
    colorScheme: resolvedColorScheme,
    isDarkColorScheme: resolvedColorScheme === 'dark',
    setColorScheme,
    toggleColorScheme,
    colors: COLORS[resolvedColorScheme],
  };
}

export { useColorScheme };
