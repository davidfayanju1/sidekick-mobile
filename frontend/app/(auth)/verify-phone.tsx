import { router, useLocalSearchParams } from 'expo-router';
import { ChevronLeft } from 'lucide-react-native';
import React from 'react';
import { Pressable, View } from 'react-native';
import OTPTextInput from 'react-native-otp-textinput';
import { SafeAreaView } from 'react-native-safe-area-context';

import Text from '@/components/UI/Text';
import { tw } from '@/lib/tw';
import { colors } from '@/theme/palette';

const RESEND_SECONDS = 54;

function formatCountdown(seconds: number) {
  const mm = String(Math.floor(seconds / 60)).padStart(2, '0');
  const ss = String(seconds % 60).padStart(2, '0');
  return `${mm}:${ss}`;
}

export default function VerifyPhone() {
  const { phone } = useLocalSearchParams<{ phone?: string }>();
  const [otp, setOtp] = React.useState('');
  const [secondsLeft, setSecondsLeft] = React.useState(RESEND_SECONDS);
  const otpRef = React.useRef<OTPTextInput>(null);

  React.useEffect(() => {
    if (secondsLeft <= 0) return;
    const interval = setInterval(() => {
      setSecondsLeft((value) => Math.max(0, value - 1));
    }, 1000);
    return () => clearInterval(interval);
  }, [secondsLeft]);

  function handleResend() {
    if (secondsLeft > 0) return;
    setSecondsLeft(RESEND_SECONDS);
    otpRef.current?.clear();
  }

  function handleVerify() {
    router.replace('/(tabs)');
  }

  return (
    <SafeAreaView style={tw`flex-1 bg-white`}>
      <View style={tw`flex-1 px-4`}>
        <Pressable
          onPress={() => router.back()}
          hitSlop={8}
          style={tw`-ml-2 h-10 w-10 items-center justify-center`}>
          <ChevronLeft size={26} color={colors.iconPrimary} />
        </Pressable>

        <Text fontWeight="bold" fontSize={24} classN="mt-3 text-black">
          Verify Phone
        </Text>
        <Text fontSize={14} classN={`mt-1 text-[${colors.textSubtle}]`}>
          {phone ? `We sent a 4-digit code to ${phone}` : 'We sent a 4-digit code to your phone'}
        </Text>

        <View style={tw`mt-8 items-center`}>
          <OTPTextInput
            ref={otpRef}
            inputCount={4}
            tintColor={colors.brand}
            offTintColor={colors.borderStrong}
            handleTextChange={setOtp}
            containerStyle={{ width: 280 }}
            // `textInputStyle` is mistyped as ViewStyle upstream even though it targets a TextInput
            textInputStyle={
              {
                width: 60,
                height: 60,
                borderWidth: 1.5,
                borderBottomWidth: 1.5,
                borderRadius: 16,
                fontSize: 22,
                fontWeight: '600',
                color: colors.iconPrimary,
              } as any
            }
          />

          <Text fontSize={13} classN="mt-4 text-black">
            {formatCountdown(secondsLeft)}
          </Text>

          <View style={tw`mt-1 flex-row items-center`}>
            <Text fontSize={13} classN={`text-[${colors.textMutedAlt}]`}>
              Didn&apos;t Receive Code?{' '}
            </Text>
            <Pressable onPress={handleResend} disabled={secondsLeft > 0} hitSlop={8}>
              <Text
                fontWeight="medium"
                fontSize={13}
                classN={
                  secondsLeft > 0 ? `text-[${colors.textSubtle}]` : `text-[${colors.textLink}]`
                }>
                Resend
              </Text>
            </Pressable>
          </View>
        </View>

        <Pressable
          onPress={handleVerify}
          disabled={otp.length < 4}
          style={{
            backgroundColor: otp.length < 4 ? colors.brandDisabled : colors.brand,
            ...tw`mt-10 items-center justify-center rounded-full py-4`,
          }}>
          <Text fontWeight="bold" fontSize={16} classN="text-white">
            Verify
          </Text>
        </Pressable>
      </View>
    </SafeAreaView>
  );
}
