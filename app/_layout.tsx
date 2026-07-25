import '../global.css';
import 'expo-dev-client';
import { ThemeProvider as NavThemeProvider } from '@react-navigation/native';
import { ActionSheetProvider } from '@expo/react-native-action-sheet';
import { useFonts } from 'expo-font';
import { Stack } from 'expo-router';
import * as SplashScreen from 'expo-splash-screen';
import { StatusBar } from 'expo-status-bar';
import * as React from 'react';
import { useDeviceContext } from 'twrnc';
import { tw } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';
import { NAV_THEME } from '@/theme';
import { useEffect } from 'react';

export {
  // Catch any errors thrown by the Layout component.
  ErrorBoundary,
} from 'expo-router';

SplashScreen.preventAutoHideAsync();

// Flip to true to skip straight to the tabs home screen, bypassing onboarding/auth.
const SKIP_ONBOARDING = true;

export default function RootLayout() {
  useDeviceContext(tw, { observeDeviceColorSchemeChanges: false, initialColorScheme: 'light' });
  const { colorScheme, isDarkColorScheme } = useColorScheme();
  const [fontsLoaded] = useFonts({
    'Satoshi-Light': require('../assets/fonts/satoshi/Satoshi-Light.otf'),
    'Satoshi-Regular': require('../assets/fonts/satoshi/Satoshi-Regular.otf'),
    'Satoshi-Medium': require('../assets/fonts/satoshi/Satoshi-Medium.otf'),
    'Satoshi-Bold': require('../assets/fonts/satoshi/Satoshi-Bold.otf'),
    'Satoshi-Black': require('../assets/fonts/satoshi/Satoshi-Black.otf'),
  });

  useEffect(() => {
    if (fontsLoaded) SplashScreen.hideAsync();
  }, [fontsLoaded]);

  if (!fontsLoaded) return null;

  return (
    <>
      <StatusBar
        style={isDarkColorScheme ? 'light' : 'dark'}
        backgroundColor="transparent"
        translucent={true}
      />
      <ActionSheetProvider>
        <NavThemeProvider value={NAV_THEME[colorScheme]}>
          <Stack screenOptions={{ animation: 'ios_from_right' }}>
            <Stack.Screen name="(tabs)" options={{ headerShown: false }} />
            <Stack.Screen name="chat/[id]" options={{ headerShown: false }} />
            <Stack.Screen name="get-started" options={{ headerShown: false }} />
            <Stack.Screen name="payment-method" options={{ headerShown: false }} />
            <Stack.Screen name="verify-id" options={{ headerShown: false }} />
            <Stack.Screen name="bank-details" options={{ headerShown: false }} />
            <Stack.Protected guard={!SKIP_ONBOARDING}>
              <Stack.Screen name="index" options={{ headerShown: false }} />
              <Stack.Screen name="(onboarding)" options={{ headerShown: false }} />
              <Stack.Screen name="(auth)" options={{ headerShown: false }} />
            </Stack.Protected>
            <Stack.Screen name="+not-found" options={{ headerShown: false }} />
          </Stack>
        </NavThemeProvider>
      </ActionSheetProvider>

      {/* </ExampleProvider> */}
    </>
  );
}
