import { Stack } from 'expo-router';

const AuthLayout = () => {
  return (
    <Stack>
      <Stack.Screen name="sign-in" options={{ headerShown: false }} />
      <Stack.Screen name="sign-up" options={{ headerShown: false }} />
      <Stack.Screen name="terms" options={{ headerShown: false }} />
      <Stack.Screen name="verify-phone" options={{ headerShown: false }} />
    </Stack>
  );
};

export default AuthLayout;
