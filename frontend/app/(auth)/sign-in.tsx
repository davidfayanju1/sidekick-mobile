import { router } from 'expo-router';
import { Apple, ChevronLeft } from 'lucide-react-native';
import React from 'react';
import { Pressable, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { GoogleIcon } from '@/components/icons/GoogleIcon';
import Form from '@/components/UI/Form';
import Text from '@/components/UI/Text';
import { tw } from '@/lib/tw';
import { colors } from '@/theme/palette';

export default function SignIn() {
  const [email, setEmail] = React.useState('');
  const [password, setPassword] = React.useState('');

  return (
    <SafeAreaView style={tw`flex-1 bg-white`}>
      <View style={tw`flex-1 px-4`}>
        <Pressable
          onPress={() => router.back()}
          hitSlop={8}
          style={tw`-ml-2 mt-2 h-10 w-10 items-center justify-center`}>
          <ChevronLeft size={26} color={colors.iconPrimary} />
        </Pressable>

        <Text fontWeight="bold" fontSize={24} classN="mt-3 text-black">
          Welcome Back
        </Text>
        <Text fontSize={14} classN={`mt-1 text-[${colors.textSubtle}]`}>
          Sign in to your Sidekick account
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
            ...tw`mt-6 flex-row items-center justify-center rounded-full border h-[44px]`,
          }}>
          <Apple size={20} color={colors.iconPrimary} fill={colors.iconPrimary} />
          <Text fontWeight="medium" fontSize={14} classN="ml-2 text-black">
            Continue with Apple
          </Text>
        </Pressable>

        <View style={tw`mt-6 flex-row items-center gap-3`}>
          <View style={[tw`h-px flex-1`, { backgroundColor: colors.textMutedAlt }]} />
          <Text fontSize={12} classN={`text-[${colors.textMutedAlt}]`}>
            Or sign in with Email
          </Text>
          <View style={[tw`h-px flex-1`, { backgroundColor: colors.textMutedAlt }]} />
        </View>

        <Form
          label="Email Address"
          labelFontSize={13}
          labelStyle="text-black"
          containerStyle="mt-6"
          placeholder=""
          value={email}
          onChangeText={setEmail}
          keyboardType="email-address"
          autoCapitalize="none"
          textContentType="emailAddress"
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

        <Pressable style={tw`mt-2 self-end`} hitSlop={8}>
          <Text fontWeight="medium" fontSize={13} classN="text-black">
            Forgot password?
          </Text>
        </Pressable>

        <Pressable
          style={tw`mt-6 items-center justify-center rounded-full py-4 bg-[${colors.brand}]`}>
          <Text fontWeight="bold" fontSize={16} classN="text-white">
            Sign In
          </Text>
        </Pressable>

        <View style={tw`mt-4 flex-row items-center justify-center`}>
          <Text fontSize={13} classN="text-black">
            New here?{' '}
          </Text>
          <Pressable onPress={() => router.push('/sign-up')} hitSlop={8}>
            <Text fontWeight="bold" fontSize={13} classN={`text-[${colors.accentOrange}]`}>
              Create account
            </Text>
          </Pressable>
        </View>
      </View>
    </SafeAreaView>
  );
}
