import { useColorScheme as useNativewindColorScheme } from 'nativewind';
import * as React from 'react';
import { useAppColorScheme } from 'twrnc';

import { tw } from '@/lib/tw';
import { useThemeStore, type ThemePreference } from '@/store/themeStore';
import { COLORS } from '@/theme/colors';

function useColorScheme() {
  const { colorScheme, setColorScheme } = useNativewindColorScheme();
  const [, , setTwColorScheme] = useAppColorScheme(tw);
  const preference = useThemeStore((state) => state.preference);
  const setPreference = useThemeStore((state) => state.setPreference);

  const resolvedColorScheme = colorScheme ?? 'light';

  // Keep twrnc's `dark:` variant in sync with the app's own (manually toggled) color scheme.
  // NOTE: twrnc's returned setter isn't referentially stable (and unconditionally triggers its
  // own re-render on every call), so it must stay out of this effect's deps or it loops forever.
  React.useEffect(() => {
    setTwColorScheme(resolvedColorScheme);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [resolvedColorScheme]);

  // Drive nativewind's own scheme from the user's stored preference (system/light/dark).
  React.useEffect(() => {
    setColorScheme(preference);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [preference]);

  function toggleColorScheme() {
    setPreference(resolvedColorScheme === 'light' ? 'dark' : 'light');
  }

  return {
    colorScheme: resolvedColorScheme,
    isDarkColorScheme: resolvedColorScheme === 'dark',
    preference: preference as ThemePreference,
    setPreference,
    setColorScheme,
    toggleColorScheme,
    colors: COLORS[resolvedColorScheme],
  };
}

export { useColorScheme };
