import '../global.css';
import 'expo-dev-client';
import { ActionSheetProvider } from '@expo/react-native-action-sheet';
import { QueryClientProvider } from '@tanstack/react-query';
import { useFonts } from 'expo-font';
import { ThemeProvider as NavThemeProvider, Stack } from 'expo-router';
import * as SplashScreen from 'expo-splash-screen';
import { StatusBar } from 'expo-status-bar';
import * as React from 'react';
import { useEffect } from 'react';
import { GestureHandlerRootView } from 'react-native-gesture-handler';
import { useDeviceContext } from 'twrnc';

import { ToastHost } from '@/components/UI/Toast';
import { queryClient, useAppStateFocusManager } from '@/lib/queryClient';
import { tw } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';
import { useSessionGuard } from '@/lib/useSessionGuard';
import { useAuthStore } from '@/store/authStore';
import { NAV_THEME } from '@/theme';

export {
  // Catch any errors thrown by the Layout component.
  ErrorBoundary,
} from 'expo-router';

SplashScreen.preventAutoHideAsync();

/**
 * Root layout.
 *
 * Every route is registered unconditionally and navigation is explicit:
 * signing in replaces to the tabs, signing out replaces to sign-in. There used
 * to be `Stack.Protected` guards here that decided which half of the app
 * existed based on session state — they made a failed sign-in look like a
 * no-op, because a screen simply wasn't registered and nothing said why.
 * Where a screen needs a signed-in user, it asks for one.
 *
 * The one navigation that isn't a screen's own business is leaving: a session
 * can end without anyone tapping anything, so `useSessionGuard` watches for
 * that here and shows the door.
 */
export default function RootLayout() {
  useDeviceContext(tw, { observeDeviceColorSchemeChanges: false, initialColorScheme: 'light' });
  useAppStateFocusManager();
  useSessionGuard();

  const { colorScheme, isDarkColorScheme } = useColorScheme();
  const [fontsLoaded] = useFonts({
    'Satoshi-Light': require('../assets/fonts/satoshi/Satoshi-Light.otf'),
    'Satoshi-Regular': require('../assets/fonts/satoshi/Satoshi-Regular.otf'),
    'Satoshi-Medium': require('../assets/fonts/satoshi/Satoshi-Medium.otf'),
    'Satoshi-Bold': require('../assets/fonts/satoshi/Satoshi-Bold.otf'),
    'Satoshi-Black': require('../assets/fonts/satoshi/Satoshi-Black.otf'),
  });

  const hydrate = useAuthStore((state) => state.hydrate);
  const isHydrated = useAuthStore((state) => state.isHydrated);

  useEffect(() => {
    hydrate();
  }, [hydrate]);

  useEffect(() => {
    if (fontsLoaded && isHydrated) SplashScreen.hideAsync();
  }, [fontsLoaded, isHydrated]);

  // Hold the splash until the stored session is read, so the first screen can
  // decide where to send the user without a visible redirect.
  if (!fontsLoaded || !isHydrated) return null;

  return (
    <GestureHandlerRootView style={{ flex: 1 }}>
      {/* SDK 57 drops `backgroundColor`/`translucent`: Android is always
          edge-to-edge now, so the bar is transparent by default. */}
      <StatusBar style={isDarkColorScheme ? 'light' : 'dark'} />
      <QueryClientProvider client={queryClient}>
        {/* ActionSheetProvider calls React.Children.only — it takes exactly one
            child, so anything else belongs outside it, not beside the nav tree. */}
        <ActionSheetProvider>
          <NavThemeProvider value={NAV_THEME[colorScheme]}>
            <Stack screenOptions={{ animation: 'ios_from_right' }}>
              <Stack.Screen name="index" options={{ headerShown: false }} />
              <Stack.Screen name="(onboarding)" options={{ headerShown: false }} />
              <Stack.Screen name="(auth)" options={{ headerShown: false }} />
              <Stack.Screen name="(tabs)" options={{ headerShown: false }} />
              <Stack.Screen name="chat/[id]" options={{ headerShown: false }} />
              <Stack.Screen name="task/[id]" options={{ headerShown: false }} />
              <Stack.Screen name="get-started" options={{ headerShown: false }} />
              <Stack.Screen name="payment-method" options={{ headerShown: false }} />
              <Stack.Screen name="verify-id" options={{ headerShown: false }} />
              <Stack.Screen name="bank-details" options={{ headerShown: false }} />
              <Stack.Screen name="wallet" options={{ headerShown: false }} />
              <Stack.Screen name="payment-history" options={{ headerShown: false }} />
              <Stack.Screen name="refer" options={{ headerShown: false }} />
              <Stack.Screen name="+not-found" options={{ headerShown: false }} />
            </Stack>
          </NavThemeProvider>
        </ActionSheetProvider>
        {/* Last child, so the toast stack paints above every screen. */}
        <ToastHost />
      </QueryClientProvider>
    </GestureHandlerRootView>
  );
}
