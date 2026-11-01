import { router } from 'expo-router';
import { Apple, ChevronLeft } from 'lucide-react-native';
import React from 'react';
import { View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { GoogleIcon } from '@/components/icons/GoogleIcon';
import { Button } from '@/components/UI/Button';
import Form from '@/components/UI/Form';
import { PressableScale } from '@/components/UI/PressableScale';
import Text from '@/components/UI/Text';
import { useSignIn } from '@/lib/api/auth';
import { apiError, errorMessage } from '@/lib/axios';
import { signInSchema } from '@/lib/forms/schemas';
import { useZodForm } from '@/lib/forms/useZodForm';
import { tw } from '@/lib/tw';
import { toast } from '@/store/toastStore';
import { colors } from '@/theme/palette';

export default function SignIn() {
  const signIn = useSignIn();
  const form = useZodForm(signInSchema, {
    email: 'davidfayanju01@gmail.com',
    password: 'appTestPassword23@',
  });

  const submit = form.handleSubmit(async (credentials) => {
    try {
      await signIn.mutateAsync(credentials);
      router.replace('/(tabs)');
    } catch (error) {
      const { status, code, message } = apiError(error);
      if (__DEV__) console.log('[auth] sign-in failed ←', { status, code, message });
      toast.error('Couldn’t sign you in', {
        description: status === 401 ? message : errorMessage(error),
      });
    }
  });

  return (
    <SafeAreaView style={tw`flex-1 bg-white`}>
      <View style={tw`flex-1 px-4`}>
        <PressableScale
          onPress={() => router.back()}
          hitSlop={8}
          style={tw`-ml-2 mt-2 h-10 w-10 items-center justify-center`}>
          <ChevronLeft size={26} color={colors.black} />
        </PressableScale>

        <Text variant="headline" classN="mt-2 text-black">
          Welcome Back
        </Text>
        <Text variant="body" classN={`mt-1 text-[${colors.gray400}]`}>
          Sign in to your Sidekick account
        </Text>

        <PressableScale
          style={{
            borderColor: colors.gray300,
            ...tw`mt-6 h-[44px] flex-row items-center justify-center rounded-full border`,
          }}>
          <GoogleIcon size={20} />
          <Text variant="body" fontWeight="medium" classN="ml-2 text-black">
            Continue with Google
          </Text>
        </PressableScale>

        <View style={tw`mt-6 flex-row items-center gap-3`}>
          <View style={[tw`h-px flex-1`, { backgroundColor: colors.gray600 }]} />
          <Text variant="caption" classN={`text-[${colors.gray600}]`}>
            Or sign in with Email
          </Text>
          <View style={[tw`h-px flex-1`, { backgroundColor: colors.gray600 }]} />
        </View>

        <Form
          label="Email Address"
          labelFontSize={13}
          labelStyle="text-black"
          containerStyle="mt-6"
          placeholder=""
          keyboardType="email-address"
          autoCapitalize="none"
          textContentType="emailAddress"
          returnKeyType="next"
          {...form.field('email')}
        />

        <Form
          label="Password"
          labelFontSize={13}
          labelStyle="text-black"
          containerStyle="mt-5"
          placeholder="******"
          type="password"
          textContentType="password"
          returnKeyType="go"
          onSubmitEditing={submit}
          {...form.field('password')}
        />

        <PressableScale style={tw`mt-2 self-end`} hitSlop={8}>
          <Text variant="caption" fontWeight="medium" classN="text-black">
            Forgot password?
          </Text>
        </PressableScale>

        <Button
          label="Sign In"
          onPress={submit}
          loading={signIn.isPending}
          succeeded={signIn.isSuccess}
          size="lg"
          style={tw`mt-6`}
        />

        <View style={tw`mt-3 flex-row items-center justify-center`}>
          <Text variant="caption" classN="text-black">
            New here?{' '}
          </Text>
          <PressableScale onPress={() => router.push('/sign-up')} hitSlop={8}>
            <Text variant="caption" fontWeight="medium" classN={`text-[${colors.orange}]`}>
              Create account
            </Text>
          </PressableScale>
        </View>
      </View>
    </SafeAreaView>
  );
}
