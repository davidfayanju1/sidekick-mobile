import { router } from 'expo-router';
import {
  Banknote,
  Check,
  ChevronLeft,
  ChevronRight,
  CreditCard,
  IdCard,
} from 'lucide-react-native';
import * as React from 'react';
import { ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import Text from '@/components/UI/Text';
import { tw } from '@/lib/tw';
import { useRoleStore } from '@/store/roleStore';
import { colors } from '@/theme/palette';
import { PressableScale } from '@/components/UI/PressableScale';

type Step = {
  key: string;
  title: string;
  description: string;
  icon: typeof CreditCard;
  done: boolean;
  pending?: boolean;
  onPress: () => void;
};

export default function GetStarted() {
  const role = useRoleStore((state) => state.role);
  const paymentMethodAdded = useRoleStore((state) => state.paymentMethodAdded);
  const idVerificationStatus = useRoleStore((state) => state.idVerificationStatus);
  const bankDetailsAdded = useRoleStore((state) => state.bankDetailsAdded);

  const isSidekick = role === 'sidekick';

  const steps: Step[] = isSidekick
    ? [
        {
          key: 'id',
          title: 'Verify your ID',
          description: 'Upload a government ID so Heroes know who they can trust.',
          icon: IdCard,
          done: idVerificationStatus === 'verified',
          pending: idVerificationStatus === 'pending',
          onPress: () => router.push('/verify-id'),
        },
        {
          key: 'bank',
          title: 'Add bank details',
          description: 'Add an account so you can withdraw what you earn.',
          icon: Banknote,
          done: bankDetailsAdded,
          onPress: () => router.push('/bank-details'),
        },
      ]
    : [
        {
          key: 'payment',
          title: 'Add a payment method',
          description: 'Save a card so you can fund tasks the moment you post one.',
          icon: CreditCard,
          done: paymentMethodAdded,
          onPress: () => router.push('/payment-method'),
        },
      ];

  return (
    <SafeAreaView style={tw`flex-1 bg-white`}>
      <ScrollView
        style={tw`flex-1 px-4`}
        contentContainerStyle={{ paddingBottom: 24 }}
        showsVerticalScrollIndicator={false}>
        <PressableScale
          onPress={() => router.back()}
          hitSlop={8}
          style={tw`-ml-2 mt-2 h-10 w-10 items-center justify-center`}>
          <ChevronLeft size={26} color={colors.black} />
        </PressableScale>

        <Text variant="title" classN="mt-2 text-black">
          {isSidekick ? "Let's get you set up as a Sidekick" : "Let's get you set up as a Hero"}
        </Text>
        <Text variant="caption" classN={`mt-1.5 text-[${colors.gray700}]`}>
          A couple of quick steps before you {isSidekick ? 'start earning' : 'start posting'}. You
          can also do these later from your profile.
        </Text>

        <View style={tw`mt-6 gap-3`}>
          {steps.map(({ key, title, description, icon: Icon, done, pending, onPress }) => (
            <PressableScale
              key={key}
              onPress={onPress}
              style={[
                tw`flex-row items-center gap-3 rounded-2xl p-4`,
                { borderWidth: 1, borderColor: colors.gray200 },
              ]}>
              <View
                style={[
                  tw`h-11 w-11 items-center justify-center rounded-xl`,
                  { backgroundColor: done ? colors.greenTint : colors.gray50 },
                ]}>
                <Icon size={20} color={done ? colors.green : colors.gray800} />
              </View>
              <View style={tw`flex-1`}>
                <Text variant="body" fontWeight="medium" classN="text-black">
                  {title}
                </Text>
                <Text variant="caption" classN={`mt-0.5 text-[${colors.gray700}]`}>
                  {description}
                </Text>
              </View>
              {done ? (
                <View
                  style={[
                    tw`h-6 w-6 items-center justify-center rounded-full`,
                    { backgroundColor: colors.green },
                  ]}>
                  <Check size={13} color="white" strokeWidth={3} />
                </View>
              ) : pending ? (
                <View
                  style={[tw`rounded-full px-2.5 py-1`, { backgroundColor: colors.orangeTint }]}>
                  <Text variant="micro" fontWeight="normal" classN={`text-[${colors.orangeDark}]`}>
                    Pending
                  </Text>
                </View>
              ) : (
                <ChevronRight size={18} color={colors.gray500} />
              )}
            </PressableScale>
          ))}
        </View>

        <PressableScale
          onPress={() => router.replace('/(tabs)')}
          style={[
            tw`mt-8 items-center justify-center rounded-full py-4`,
            { backgroundColor: colors.primary },
          ]}>
          <Text variant="body" fontWeight="medium" classN="text-white">
            Continue to {isSidekick ? 'Sidekick' : 'Hero'} home
          </Text>
        </PressableScale>
        <Text variant="caption" classN={`mt-2 text-center text-[${colors.gray500}]`}>
          You can finish these anytime from your profile.
        </Text>
      </ScrollView>
    </SafeAreaView>
  );
}
