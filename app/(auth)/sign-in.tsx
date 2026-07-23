import { router } from 'expo-router';
import { Apple, ChevronLeft } from 'lucide-react-native';
import React from 'react';
import { Pressable, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { GoogleIcon } from '@/components/icons/GoogleIcon';
import Form from '@/components/UI/Form';
import Text from '@/components/UI/Text';
import { tw } from '@/lib/tw';

const BORDER = '#D1D4D5';
const ACCENT_TEAL = '#489A9F';
const ACCENT_ORANGE = '#FF7A55';

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
          <ChevronLeft size={26} color="#000" />
        </Pressable>

        <Text fontWeight="bold" fontSize={24} classN="mt-3 text-black">
          Welcome Back
        </Text>
        <Text fontSize={14} classN="mt-1 text-[#B7B7B7]">
          Sign in to your Sidekick account
        </Text>

        <Pressable
          style={{
            borderColor: BORDER,
            ...tw`mt-6 flex-row items-center justify-center rounded-full border h-[44px]`,
          }}>
          <GoogleIcon size={20} />
          <Text fontWeight="medium" fontSize={14} classN="ml-2 text-black">
            Continue with Google
          </Text>
        </Pressable>

        <Pressable
          style={{
            borderColor: BORDER,
            ...tw`mt-6 flex-row items-center justify-center rounded-full border h-[44px]`,
          }}>
          <Apple size={20} color="#000" fill="#000" />
          <Text fontWeight="medium" fontSize={14} classN="ml-2 text-black">
            Continue with Apple
          </Text>
        </Pressable>

        <View style={tw`mt-6 flex-row items-center gap-3`}>
          <View style={[tw`h-px flex-1`, { backgroundColor: '#8C9296' }]} />
          <Text fontSize={12} classN="text-[#8C9296]">
            Or sign in with Email
          </Text>
          <View style={[tw`h-px flex-1`, { backgroundColor: '#8C9296' }]} />
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

        <Pressable style={tw`mt-6 items-center justify-center rounded-full py-4 bg-[#489A9F]`}>
          <Text fontWeight="bold" fontSize={16} classN="text-white">
            Sign In
          </Text>
        </Pressable>

        <View style={tw`mt-4 flex-row items-center justify-center`}>
          <Text fontSize={13} classN="text-black">
            New here?{' '}
          </Text>
          <Pressable onPress={() => router.push('/sign-up')} hitSlop={8}>
            <Text fontWeight="bold" fontSize={13} classN={`text-[${ACCENT_ORANGE}]`}>
              Create account
            </Text>
          </Pressable>
        </View>
      </View>
    </SafeAreaView>
  );
}
