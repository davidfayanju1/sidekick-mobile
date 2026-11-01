import { router, useLocalSearchParams } from 'expo-router';
import { ChevronLeft } from 'lucide-react-native';
import React from 'react';
import { View } from 'react-native';
import OTPTextInput from 'react-native-otp-textinput';
import { SafeAreaView } from 'react-native-safe-area-context';

import { Button } from '@/components/UI/Button';
import Form from '@/components/UI/Form';
import { PressableScale } from '@/components/UI/PressableScale';
import Text from '@/components/UI/Text';
import { useSendPhoneCode, useVerifyPhone } from '@/lib/api/auth';
import { apiError } from '@/lib/axios';
import { phoneOnlySchema } from '@/lib/forms/schemas';
import { useZodForm } from '@/lib/forms/useZodForm';
import { tw } from '@/lib/tw';
import { useAuthStore } from '@/store/authStore';
import { toast } from '@/store/toastStore';
import { colors } from '@/theme/palette';

/**
 * Must not be shorter than the server's `otp_resend_cooldown_seconds`
 * (default 60 in internal/otp), or Resend enables early and returns 429.
 * It isn't in `/config/public`, so this mirrors the server default by hand.
 */
const RESEND_SECONDS = 60;
/** The API mints six-digit codes (`%06d` in internal/otp) — this must match it. */
const CODE_LENGTH = 6;

function formatCountdown(seconds: number) {
  const mm = String(Math.floor(seconds / 60)).padStart(2, '0');
  const ss = String(seconds % 60).padStart(2, '0');
  return `${mm}:${ss}`;
}

/**
 * Phone verification, reachable two ways.
 *
 * Sign-up arrives with a `phone` param and a code already sent, so it opens on
 * the code step. Anyone verifying later from their profile arrives with
 * nothing and enters a number first — an account created by signing in, or by
 * OAuth, has no phone on file at all (`phone: null`).
 */
export default function VerifyPhone() {
  const params = useLocalSearchParams<{ phone?: string }>();
  const user = useAuthStore((state) => state.user);

  // From the sign-up handoff, or already on the account.
  const knownPhone = params.phone || user?.phone || '';
  const [phone, setPhone] = React.useState(knownPhone);
  const [step, setStep] = React.useState<'phone' | 'code'>(knownPhone ? 'code' : 'phone');

  const [otp, setOtp] = React.useState('');
  const [secondsLeft, setSecondsLeft] = React.useState(knownPhone ? RESEND_SECONDS : 0);
  const otpRef = React.useRef<OTPTextInput>(null);

  const verify = useVerifyPhone();
  const sendCode = useSendPhoneCode();
  const phoneForm = useZodForm(phoneOnlySchema, { phone: knownPhone });

  React.useEffect(() => {
    if (secondsLeft <= 0) return;
    const interval = setInterval(() => {
      setSecondsLeft((value) => Math.max(0, value - 1));
    }, 1000);
    return () => clearInterval(interval);
  }, [secondsLeft]);

  /** Shared by the first send and every resend. Returns whether it landed. */
  async function send(to: string) {
    if (!user?.id) {
      toast.error('Sign in to verify a number');
      return false;
    }
    try {
      await sendCode.mutateAsync({ user_id: user.id, phone: to });
      setSecondsLeft(RESEND_SECONDS);
      otpRef.current?.clear();
      setOtp('');
      return true;
    } catch (error) {
      const { status, message } = apiError(error);
      toast.error(
        status === 429
          ? 'Too many requests'
          : status === 409
            ? 'That number is already in use'
            : 'Couldn’t send the code',
        {
          description:
            status === 429
              ? 'Wait a moment before asking for another code.'
              : status === 409
                ? 'Another account has already verified this number.'
                : message,
        }
      );
      if (status === 429) setSecondsLeft(RESEND_SECONDS);
      return false;
    }
  }

  // `handleSubmit` is invoked on press rather than during render: building the
  // handler at render time hands the compiler a closure that touches `otpRef`,
  // and it can't tell the closure won't run until the button is tapped.
  async function submitPhone() {
    await phoneForm.handleSubmit(async ({ phone: entered }) => {
      if (await send(entered)) {
        setPhone(entered);
        setStep('code');
        toast.success('Code sent', { description: `We texted a 6-digit code to ${entered}.` });
      }
    })();
  }

  async function handleResend() {
    if (secondsLeft > 0 || !phone) return;
    if (await send(phone)) {
      toast.success('Code sent', { description: `We texted a new code to ${phone}.` });
    }
  }

  async function handleVerify() {
    if (!phone) return;
    try {
      await verify.mutateAsync({ phone, code: otp });
      toast.success('Phone verified');
      // Sign-up lands in the app; verifying from the profile goes back where it
      // came from, so the user keeps their place.
      if (params.phone) router.replace('/(tabs)');
      else router.back();
    } catch (error) {
      // Wrong codes are the common case and shouldn't feel like a failure —
      // clear the boxes so the next attempt starts clean.
      otpRef.current?.clear();
      setOtp('');

      // The status codes come from httpStatus() in internal/httpapi/server.go.
      // A wrong code is a 401, not a 400 — these are distinct outcomes and each
      // one needs a different next step from the user.
      const { status, message } = apiError(error);
      const { title, description } =
        status === 401
          ? { title: 'That code didn’t work', description: 'Check the digits and try again.' }
          : status === 410
            ? {
                title: 'That code expired',
                description: 'Codes last 5 minutes — request a new one.',
              }
            : status === 423
              ? {
                  title: 'Too many attempts',
                  description: 'This code is locked. Request a new one to start over.',
                }
              : status === 409
                ? {
                    title: 'That number is already in use',
                    description: 'Another account has verified this phone number.',
                  }
                : { title: 'Couldn’t verify your number', description: message };

      toast.error(title, { description });
    }
  }

  return (
    <SafeAreaView style={tw`flex-1 bg-white`} edges={['top']}>
      <View style={tw`flex-1 px-4`}>
        <PressableScale
          // Stepping back to the number is more useful than leaving the screen,
          // unless sign-up sent us here with the number already settled.
          onPress={() => (step === 'code' && !params.phone ? setStep('phone') : router.back())}
          hitSlop={8}
          style={tw`-ml-2 h-10 w-10 items-center justify-center`}>
          <ChevronLeft size={26} color={colors.black} />
        </PressableScale>

        <Text variant="headline" classN="mt-2 text-black">
          Verify Phone
        </Text>
        <Text variant="body" classN={`mt-1 text-[${colors.gray400}]`}>
          {step === 'phone'
            ? 'We’ll text you a 6-digit code to confirm it’s really you.'
            : `We sent a 6-digit code to ${phone}`}
        </Text>

        {step === 'phone' ? (
          <>
            <Form
              label="Phone Number"
              labelFontSize={13}
              labelStyle="text-black"
              containerStyle="mt-6"
              placeholder="0803 123 4567"
              keyboardType="phone-pad"
              textContentType="telephoneNumber"
              returnKeyType="go"
              onSubmitEditing={submitPhone}
              {...phoneForm.field('phone')}
            />
            <Button
              label="Send code"
              onPress={submitPhone}
              loading={sendCode.isPending}
              size="lg"
              style={tw`mt-6`}
            />
          </>
        ) : (
          <>
            <View style={tw`mt-8 items-center`}>
              <OTPTextInput
                ref={otpRef}
                inputCount={CODE_LENGTH}
                tintColor={colors.primary}
                offTintColor={colors.gray300}
                handleTextChange={setOtp}
                containerStyle={{ width: 320 }}
                // `textInputStyle` is mistyped as ViewStyle upstream even though it targets a TextInput
                textInputStyle={
                  {
                    width: 46,
                    height: 56,
                    borderWidth: 1.5,
                    borderBottomWidth: 1.5,
                    borderRadius: 14,
                    fontSize: 20,
                    fontWeight: '600',
                    color: colors.black,
                  } as any
                }
              />

              <Text variant="caption" classN="mt-3 text-black">
                {formatCountdown(secondsLeft)}
              </Text>

              <View style={tw`mt-1 flex-row items-center`}>
                <Text variant="caption" classN={`text-[${colors.gray600}]`}>
                  Didn&apos;t Receive Code?{' '}
                </Text>
                <PressableScale
                  onPress={handleResend}
                  disabled={secondsLeft > 0 || sendCode.isPending}
                  hitSlop={8}>
                  <Text
                    variant="caption"
                    fontWeight="medium"
                    classN={
                      secondsLeft > 0 ? `text-[${colors.gray400}]` : `text-[${colors.primary}]`
                    }>
                    {sendCode.isPending ? 'Sending…' : 'Resend'}
                  </Text>
                </PressableScale>
              </View>
            </View>

            <Button
              label="Verify"
              onPress={handleVerify}
              disabled={otp.length < CODE_LENGTH}
              loading={verify.isPending}
              succeeded={verify.isSuccess}
              size="lg"
              style={tw`mt-10`}
            />
          </>
        )}
      </View>
    </SafeAreaView>
  );
}
