import { router } from 'expo-router';
import { Apple, ChevronLeft } from 'lucide-react-native';
import React from 'react';
import { Pressable, ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { GoogleIcon } from '@/components/icons/GoogleIcon';
import Form from '@/components/UI/Form';
import Text from '@/components/UI/Text';
import { tw } from '@/lib/tw';
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
  const [fullName, setFullName] = React.useState('');
  const [email, setEmail] = React.useState('');
  const [phone, setPhone] = React.useState('');
  const [password, setPassword] = React.useState('');

  const strength = getPasswordStrength(password);

  return (
    <ScrollView style={tw`bg-white pt-[62px]`} showsVerticalScrollIndicator={false}>
      <View style={tw`flex-1 px-4`}>
        <Pressable
          onPress={() => router.back()}
          hitSlop={8}
          style={tw`-ml-2 h-10 w-10 items-center justify-center`}>
          <ChevronLeft size={26} color={colors.iconPrimary} />
        </Pressable>

        <ScrollView
          contentContainerStyle={{ paddingBottom: 16 }}
          keyboardShouldPersistTaps="handled"
          showsVerticalScrollIndicator={false}>
          <Text fontWeight="bold" fontSize={24} classN="mt-3 text-black">
            Create account
          </Text>
          <Text fontSize={14} classN={`mt-1 text-[${colors.textSubtle}]`}>
            Join thousands getting things done in Lagos
          </Text>

          <Pressable
            style={{
              borderColor: colors.borderStrong,
              ...tw`mt-6 flex-row items-center justify-center rounded-full border h-[44px]`,
            }}>
            <GoogleIcon size={20} />
            <Text fontWeight="medium" fontSize={14} classN="ml-2 text-black">
              Continue with Google
            </Text>
          </Pressable>

          <Pressable
            style={{
              borderColor: colors.borderStrong,
              ...tw`mt-3 flex-row items-center justify-center rounded-full border h-[44px]`,
            }}>
            <Apple size={20} color={colors.iconPrimary} fill={colors.iconPrimary} />
            <Text fontWeight="medium" fontSize={14} classN="ml-2 text-black">
              Continue with Apple
            </Text>
          </Pressable>

          <View style={tw`mt-6 flex-row items-center gap-3`}>
            <View style={[tw`h-px flex-1`, { backgroundColor: colors.textMutedAlt }]} />
            <Text fontSize={12} classN={`text-[${colors.textMutedAlt}]`}>
              Or sign up with Email
            </Text>
            <View style={[tw`h-px flex-1`, { backgroundColor: colors.textMutedAlt }]} />
          </View>

          <Form
            label="Full Name"
            labelFontSize={13}
            labelStyle="text-black"
            containerStyle="mt-6"
            placeholder=""
            value={fullName}
            onChangeText={setFullName}
            autoCapitalize="words"
            textContentType="name"
          />

          <Form
            label="Email Address"
            labelFontSize={13}
            labelStyle="text-black"
            containerStyle="mt-5"
            placeholder=""
            value={email}
            onChangeText={setEmail}
            keyboardType="email-address"
            autoCapitalize="none"
            textContentType="emailAddress"
          />

          <Form
            label="Phone Number"
            labelFontSize={13}
            labelStyle="text-black"
            containerStyle="mt-5"
            placeholder=""
            value={phone}
            onChangeText={setPhone}
            keyboardType="phone-pad"
            textContentType="telephoneNumber"
          />

          <Form
            label="Password"
            labelFontSize={13}
            labelStyle="text-black"
            containerStyle="mt-5"
            placeholder="******"
            type="password"
            value={password}
            onChangeText={setPassword}
            textContentType="password"
          />
          {strength && (
            <Text fontSize={12} classN={`mt-1 text-[${colors.accentOrange}]`}>
              {`Password strength: ${strength.label} — ${strength.message}`}
            </Text>
          )}

          <Text fontSize={12} classN={`mt-4 max-w-[90%] text-center text-[${colors.textMutedAlt}]`}>
            By creating an account you agree to{' '}
            <Text fontWeight="bold" fontSize={12} classN="text-black">
              Our Terms Of Service
            </Text>{' '}
            and{' '}
            <Text fontWeight="bold" fontSize={12} classN="text-black">
              Privacy Policy
            </Text>
          </Text>

          <Pressable
            onPress={() => router.push({ pathname: '/terms', params: { phone } })}
            style={{
              backgroundColor: colors.brand,
              ...tw`mt-4 items-center justify-center rounded-full py-4`,
            }}>
            <Text fontWeight="bold" fontSize={16} classN="text-white">
              Continue
            </Text>
          </Pressable>

          <View style={tw`mb-2 mt-4 flex-row items-center justify-center`}>
            <Text fontSize={13} classN="text-black">
              Already have an account?{' '}
            </Text>
            <Pressable onPress={() => router.push('/sign-in')} hitSlop={8}>
              <Text fontWeight="bold" fontSize={13} classN={`text-[${colors.accentOrange}]`}>
                Sign In
              </Text>
            </Pressable>
          </View>
        </ScrollView>
      </View>
    </ScrollView>
  );
}
