import { router } from 'expo-router';
import { Apple, ChevronLeft } from 'lucide-react-native';
import React from 'react';
import { ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { GoogleIcon } from '@/components/icons/GoogleIcon';
import { Button } from '@/components/UI/Button';
import Form from '@/components/UI/Form';
import { PressableScale } from '@/components/UI/PressableScale';
import Text from '@/components/UI/Text';
import { useSendPhoneCode, useSignUp } from '@/lib/api/auth';
import { errorMessage } from '@/lib/axios';
import { signUpSchema } from '@/lib/forms/schemas';
import { useZodForm } from '@/lib/forms/useZodForm';
import { tw } from '@/lib/tw';
import { toast } from '@/store/toastStore';
import { colors } from '@/theme/palette';

function getPasswordStrength(password: string) {
  if (!password) return null;

  const isLong = password.length >= 8;
  const hasNumber = /\d/.test(password);
  const hasSymbol = /[^A-Za-z0-9]/.test(password);
  const hasUpper = /[A-Z]/.test(password);
  const score = [isLong, hasNumber, hasSymbol, hasUpper].filter(Boolean).length;

  if (score <= 1) {
    return { label: 'Weak', message: 'add numbers, symbols or make it longer' };
  }
  if (score <= 2) {
    return { label: 'Fair', message: 'add numbers or symbols' };
  }
  return { label: 'Strong', message: 'great password' };
}

export default function SignUp() {
  const signUp = useSignUp();
  const sendCode = useSendPhoneCode();

  const form = useZodForm(signUpSchema, {
    display_name: '',
    email: '',
    phone: '',
    date_of_birth: '',
    password: '',
  });

  const strength = getPasswordStrength(String(form.values.password ?? ''));

  const submit = form.handleSubmit(async (input) => {
    try {
      // The account has to exist before a code can be sent to its phone —
      // /auth/phone/send is keyed by user_id.
      const session = await signUp.mutateAsync({
        email: input.email,
        password: input.password,
        display_name: input.display_name,
        // Sent for forward-compatibility. `users` has no date_of_birth column
        // and /auth/signup ignores unknown fields, so today this is dropped —
        // the 18+ rule is enforced by the form, not the server.
        date_of_birth: input.date_of_birth,
      });

      await sendCode.mutateAsync({ user_id: session.user.id, phone: input.phone });
      router.push({ pathname: '/terms', params: { phone: input.phone } });
    } catch (error) {
      const status = (error as { status?: number }).status;
      toast.error(
        status === 409 ? 'That email is already registered' : 'Couldn’t create your account',
        {
          description:
            status === 409 ? 'Sign in instead, or use a different email.' : errorMessage(error),
          action:
            status === 409
              ? { label: 'Sign in', onPress: () => router.push('/sign-in') }
              : undefined,
        }
      );
    }
  });

  return (
    <SafeAreaView style={tw`flex-1 bg-white`} edges={['top']}>
      {/* Pinned: the back button sits outside the ScrollView so it stays put
          while the form scrolls under it. */}
      <View style={tw`px-4`}>
        <PressableScale
          onPress={() => router.back()}
          hitSlop={8}
          style={tw`-ml-2 h-10 w-10 items-center justify-center`}>
          <ChevronLeft size={26} color={colors.black} />
        </PressableScale>
      </View>

      <View style={tw`flex-1 px-4`}>
        <ScrollView
          contentContainerStyle={{ paddingBottom: 24 }}
          keyboardShouldPersistTaps="handled"
          keyboardDismissMode="on-drag"
          showsVerticalScrollIndicator={false}>
          <Text variant="headline" classN="mt-2 text-black">
            Create account
          </Text>
          <Text variant="body" classN={`mt-1 text-[${colors.gray400}]`}>
            Join thousands getting things done in Lagos
          </Text>

          <PressableScale
            style={{
              borderColor: colors.gray300,
              ...tw`mt-6 flex-row items-center justify-center rounded-full border h-[44px]`,
            }}>
            <GoogleIcon size={20} />
            <Text variant="body" fontWeight="medium" classN="ml-2 text-black">
              Continue with Google
            </Text>
          </PressableScale>

          <View style={tw`mt-6 flex-row items-center gap-3`}>
            <View style={[tw`h-px flex-1`, { backgroundColor: colors.gray600 }]} />
            <Text variant="caption" classN={`text-[${colors.gray600}]`}>
              Or sign up with Email
            </Text>
            <View style={[tw`h-px flex-1`, { backgroundColor: colors.gray600 }]} />
          </View>

          <Form
            label="Full Name"
            labelFontSize={13}
            labelStyle="text-black"
            containerStyle="mt-6"
            placeholder=""
            autoCapitalize="words"
            {...form.field('display_name')}
            textContentType="name"
          />

          <Form
            label="Email Address"
            labelFontSize={13}
            labelStyle="text-black"
            containerStyle="mt-5"
            placeholder=""
            keyboardType="email-address"
            {...form.field('email')}
            autoCapitalize="none"
            textContentType="emailAddress"
          />

          <Form
            label="Phone Number"
            labelFontSize={13}
            labelStyle="text-black"
            containerStyle="mt-5"
            placeholder=""
            keyboardType="phone-pad"
            {...form.field('phone')}
            textContentType="telephoneNumber"
          />

          <Form
            label="Date of Birth"
            labelFontSize={13}
            labelStyle="text-black"
            containerStyle="mt-5"
            placeholder="DD/MM/YYYY"
            keyboardType="number-pad"
            maxLength={10}
            {...form.field('date_of_birth')}
            onChangeText={(text) => {
              // Insert the slashes as they type, so DD/MM/YYYY never has to be
              // typed out and the field can't drift from the expected shape.
              const digits = text.replace(/[^0-9]/g, '').slice(0, 8);
              const parts = [digits.slice(0, 2), digits.slice(2, 4), digits.slice(4, 8)].filter(
                Boolean
              );
              form.setValue('date_of_birth', parts.join('/'));
            }}
          />

          <Form
            label="Password"
            labelFontSize={13}
            labelStyle="text-black"
            containerStyle="mt-5"
            placeholder="******"
            type="password"
            textContentType="password"
            {...form.field('password')}
          />
          {strength && (
            <Text variant="caption" classN={`mt-1 text-[${colors.orange}]`}>
              {`Password strength: ${strength.label} — ${strength.message}`}
            </Text>
          )}

          <Text variant="caption" classN={`mt-3 max-w-[90%] text-center text-[${colors.gray600}]`}>
            By creating an account you agree to{' '}
            <Text variant="caption" fontWeight="medium" classN="text-black">
              Our Terms Of Service
            </Text>{' '}
            and{' '}
            <Text variant="caption" fontWeight="medium" classN="text-black">
              Privacy Policy
            </Text>
          </Text>

          <Button
            label="Continue"
            onPress={submit}
            loading={signUp.isPending || sendCode.isPending}
            size="lg"
            style={tw`mt-3`}
          />

          <View style={tw`mb-2 mt-3 flex-row items-center justify-center`}>
            <Text variant="caption" classN="text-black">
              Already have an account?{' '}
            </Text>
            <PressableScale onPress={() => router.push('/sign-in')} hitSlop={8}>
              <Text variant="caption" fontWeight="medium" classN={`text-[${colors.orange}]`}>
                Sign In
              </Text>
            </PressableScale>
          </View>
        </ScrollView>
      </View>
    </SafeAreaView>
  );
}
