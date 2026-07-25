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
import { Pressable, ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import Text from '@/components/UI/Text';
import { tw } from '@/lib/tw';
import { useRoleStore } from '@/store/roleStore';

const ACCENT_TEAL = '#489A9F';
const BORDER = '#E1E4EA';
const SURFACE = '#F5F6F7';
const SUCCESS = '#2F9E56';
const PENDING_BG = '#F5E6D3';
const PENDING_TEXT = '#B5762E';

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
        <Pressable
          onPress={() => router.back()}
          hitSlop={8}
          style={tw`-ml-2 mt-3 h-10 w-10 items-center justify-center`}>
          <ChevronLeft size={26} color="#000" />
        </Pressable>

        <Text fontWeight="bold" fontSize={22} classN="mt-2 text-black">
          {isSidekick ? "Let's get you set up as a Sidekick" : "Let's get you set up as a Hero"}
        </Text>
        <Text fontSize={13} classN="mt-1.5 text-[#6B7075]">
          A couple of quick steps before you {isSidekick ? 'start earning' : 'start posting'}. You
          can also do these later from your profile.
        </Text>

        <View style={tw`mt-6 gap-3`}>
          {steps.map(({ key, title, description, icon: Icon, done, pending, onPress }) => (
            <Pressable
              key={key}
              onPress={onPress}
              style={[
                tw`flex-row items-center gap-3 rounded-2xl p-4`,
                { borderWidth: 1, borderColor: BORDER },
              ]}>
              <View
                style={[
                  tw`h-11 w-11 items-center justify-center rounded-xl`,
                  { backgroundColor: done ? '#DCF5E3' : SURFACE },
                ]}>
                <Icon size={20} color={done ? SUCCESS : '#4B5054'} />
              </View>
              <View style={tw`flex-1`}>
                <Text fontWeight="bold" fontSize={14} classN="text-black">
                  {title}
                </Text>
                <Text fontSize={12} classN="mt-0.5 text-[#6B7075]">
                  {description}
                </Text>
              </View>
              {done ? (
                <View
                  style={[
                    tw`h-6 w-6 items-center justify-center rounded-full`,
                    { backgroundColor: SUCCESS },
                  ]}>
                  <Check size={13} color="white" strokeWidth={3} />
                </View>
              ) : pending ? (
                <View style={[tw`rounded-full px-2.5 py-1`, { backgroundColor: PENDING_BG }]}>
                  <Text fontSize={11} classN={`text-[${PENDING_TEXT}]`}>
                    Pending
                  </Text>
                </View>
              ) : (
                <ChevronRight size={18} color="#9AA0A6" />
              )}
            </Pressable>
          ))}
        </View>

        <Pressable
          onPress={() => router.replace('/(tabs)')}
          style={[
            tw`mt-8 items-center justify-center rounded-full py-4`,
            { backgroundColor: ACCENT_TEAL },
          ]}>
          <Text fontWeight="bold" fontSize={15} classN="text-white">
            Continue to {isSidekick ? 'Sidekick' : 'Hero'} home
          </Text>
        </Pressable>
        <Text fontSize={12} classN="mt-3 text-center text-[#9AA0A6]">
          You can finish these anytime from your profile.
        </Text>
      </ScrollView>
    </SafeAreaView>
  );
}
