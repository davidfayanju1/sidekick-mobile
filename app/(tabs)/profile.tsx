import { router } from 'expo-router';
import {
  Banknote,
  ChevronRight,
  CreditCard,
  FileText,
  Gift,
  IdCard,
  LogOut,
  Monitor,
  Moon,
  Receipt,
  Sun,
  Wallet,
} from 'lucide-react-native';
import * as React from 'react';
import { Pressable, ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import RoleSwitchSheet from '@/components/UI/RoleSwitchSheet';
import Text from '@/components/UI/Text';
import { tw, twColor } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';
import { useOutstandingStepsCount, useRoleStore, type UserRole } from '@/store/roleStore';
import type { ThemePreference } from '@/store/themeStore';
import { withOpacity } from '@/theme/with-opacity';

const ACCENT_TEAL = '#489A9F';
const ACCENT_ORANGE = '#D1704F';
const DANGER = '#C94B34';
const SUCCESS = '#2F9E56';

const ROLE_LABEL: Record<UserRole, string> = { hero: 'Hero', sidekick: 'Sidekick' };
const ROLE_DESCRIPTION: Record<UserRole, string> = {
  hero: 'You post tasks and get help from Sidekicks nearby.',
  sidekick: 'You browse tasks nearby and earn on your schedule.',
};

const APPEARANCE_OPTIONS: { value: ThemePreference; label: string; icon: typeof Sun }[] = [
  { value: 'system', label: 'System', icon: Monitor },
  { value: 'light', label: 'Light', icon: Sun },
  { value: 'dark', label: 'Dark', icon: Moon },
];

const MENU_ITEMS: { icon: typeof Wallet; label: string; onPress?: () => void }[] = [
  { icon: Wallet, label: 'Wallet' },
  { icon: Receipt, label: 'Payment History' },
  { icon: FileText, label: 'Terms & Conditions', onPress: () => router.push('/terms') },
  { icon: Gift, label: 'Refer a friend' },
];

export default function Profile() {
  const { colors, preference, setPreference } = useColorScheme();
  const role = useRoleStore((state) => state.role);
  const setRole = useRoleStore((state) => state.setRole);
  const paymentMethodAdded = useRoleStore((state) => state.paymentMethodAdded);
  const idVerificationStatus = useRoleStore((state) => state.idVerificationStatus);
  const bankDetailsAdded = useRoleStore((state) => state.bankDetailsAdded);
  const outstandingSteps = useOutstandingStepsCount();
  const [switchSheetVisible, setSwitchSheetVisible] = React.useState(false);

  const fg = twColor(colors.foreground);
  const muted = twColor(colors.mutedForeground);

  const isSidekick = role === 'sidekick';
  const otherRole: UserRole = isSidekick ? 'hero' : 'sidekick';

  const setupSteps = isSidekick
    ? [
        {
          key: 'id',
          title: 'Verify your ID',
          icon: IdCard,
          done: idVerificationStatus === 'verified',
          pending: idVerificationStatus === 'pending',
          onPress: () => router.push('/verify-id'),
        },
        {
          key: 'bank',
          title: 'Add bank details',
          icon: Banknote,
          done: bankDetailsAdded,
          pending: false,
          onPress: () => router.push('/bank-details'),
        },
      ]
    : role === 'hero'
      ? [
          {
            key: 'payment',
            title: 'Add a payment method',
            icon: CreditCard,
            done: paymentMethodAdded,
            pending: false,
            onPress: () => router.push('/payment-method'),
          },
        ]
      : [];

  return (
    <SafeAreaView style={[tw`flex-1`, { backgroundColor: colors.background }]}>
      <ScrollView
        style={tw`flex-1 px-4`}
        contentContainerStyle={{ paddingBottom: 40 }}
        showsVerticalScrollIndicator={false}>
        <View style={tw`mt-4 flex-row items-center gap-3`}>
          <View
            style={[
              tw`h-14 w-14 items-center justify-center rounded-full`,
              { backgroundColor: '#F0DCC8' },
            ]}>
            <Text fontWeight="bold" fontSize={18} classN="text-[#B5762E]">
              B
            </Text>
          </View>
          <View>
            <Text fontWeight="bold" fontSize={17} classN={`text-[${fg}]`}>
              Bisola Soks
            </Text>
            {role && (
              <View
                style={[
                  tw`mt-1 self-start rounded-full px-2.5 py-0.5`,
                  { backgroundColor: withOpacity(ACCENT_TEAL, 0.14) },
                ]}>
                <Text fontWeight="medium" fontSize={11} classN={`text-[${twColor(ACCENT_TEAL)}]`}>
                  {ROLE_LABEL[role]}
                </Text>
              </View>
            )}
          </View>
        </View>

        <Text fontWeight="bold" fontSize={13} classN={`mt-8 text-[${fg}]`}>
          Appearance
        </Text>
        <View
          style={[
            tw`mt-2.5 flex-row rounded-2xl p-1`,
            { backgroundColor: colors.grey6, borderWidth: 1, borderColor: colors.grey5 },
          ]}>
          {APPEARANCE_OPTIONS.map(({ value, label, icon: Icon }) => {
            const isActive = preference === value;
            return (
              <Pressable
                key={value}
                onPress={() => setPreference(value)}
                style={[
                  tw`flex-1 flex-row items-center justify-center gap-1.5 rounded-xl py-2.5`,
                  isActive && { backgroundColor: ACCENT_TEAL },
                ]}>
                <Icon size={15} color={isActive ? 'white' : colors.mutedForeground} />
                <Text
                  fontWeight="medium"
                  fontSize={12}
                  classN={isActive ? 'text-white' : `text-[${muted}]`}>
                  {label}
                </Text>
              </Pressable>
            );
          })}
        </View>

        {role && (
          <>
            <Text fontWeight="bold" fontSize={13} classN={`mt-8 text-[${fg}]`}>
              Role
            </Text>
            <View
              style={[
                tw`mt-2.5 rounded-2xl p-4`,
                { backgroundColor: colors.card, borderWidth: 1, borderColor: colors.grey5 },
              ]}>
              <Text fontWeight="bold" fontSize={14} classN={`text-[${fg}]`}>
                You&rsquo;re a {ROLE_LABEL[role]}
              </Text>
              <Text fontSize={12} classN={`mt-1 text-[${muted}]`}>
                {ROLE_DESCRIPTION[role]}
              </Text>
              <Pressable
                onPress={() => setSwitchSheetVisible(true)}
                style={[
                  tw`mt-3.5 items-center justify-center rounded-full py-3`,
                  { backgroundColor: ACCENT_ORANGE },
                ]}>
                <Text fontWeight="bold" fontSize={13} classN="text-white">
                  Switch to {ROLE_LABEL[otherRole]}
                </Text>
              </Pressable>
            </View>
          </>
        )}

        {setupSteps.length > 0 && (
          <>
            <View style={tw`mt-8 flex-row items-center justify-between`}>
              <Text fontWeight="bold" fontSize={13} classN={`text-[${fg}]`}>
                Account setup
              </Text>
              {outstandingSteps > 0 && (
                <Text fontSize={11} classN={`text-[${twColor('#B5762E')}]`}>
                  {outstandingSteps} left
                </Text>
              )}
            </View>
            <View style={tw`mt-2.5 gap-2.5`}>
              {setupSteps.map(({ key, title, icon: Icon, done, pending, onPress }) => (
                <Pressable
                  key={key}
                  onPress={onPress}
                  style={[
                    tw`flex-row items-center gap-3 rounded-2xl p-3.5`,
                    { backgroundColor: colors.card, borderWidth: 1, borderColor: colors.grey5 },
                  ]}>
                  <Icon size={18} color={done ? SUCCESS : colors.mutedForeground} />
                  <Text fontWeight="medium" fontSize={13} classN={`flex-1 text-[${fg}]`}>
                    {title}
                  </Text>
                  <Text fontSize={11} classN={`text-[${muted}]`}>
                    {done ? 'Done' : pending ? 'Pending' : 'Add'}
                  </Text>
                  <ChevronRight size={16} color={colors.mutedForeground} />
                </Pressable>
              ))}
            </View>
          </>
        )}

        <Text fontWeight="bold" fontSize={13} classN={`mt-8 text-[${fg}]`}>
          General
        </Text>
        <View
          style={[
            tw`mt-2.5 rounded-2xl px-1`,
            { backgroundColor: colors.card, borderWidth: 1, borderColor: colors.grey5 },
          ]}>
          {MENU_ITEMS.map(({ icon: Icon, label, onPress }, index) => (
            <Pressable
              key={label}
              onPress={onPress}
              style={[
                tw`flex-row items-center gap-3 px-3.5 py-3.5`,
                index < MENU_ITEMS.length - 1 && {
                  borderBottomWidth: 1,
                  borderBottomColor: colors.grey5,
                },
              ]}>
              <Icon size={18} color={colors.mutedForeground} />
              <Text fontSize={14} classN={`flex-1 text-[${fg}]`}>
                {label}
              </Text>
              <ChevronRight size={16} color={colors.mutedForeground} />
            </Pressable>
          ))}
        </View>

        <Pressable
          onPress={() => router.replace('/sign-in')}
          style={[
            tw`mt-6 flex-row items-center justify-center gap-2 rounded-2xl py-3.5`,
            { borderWidth: 1, borderColor: colors.grey5 },
          ]}>
          <LogOut size={16} color={DANGER} />
          <Text fontWeight="bold" fontSize={13} classN={`text-[${twColor(DANGER)}]`}>
            Sign Out
          </Text>
        </Pressable>
      </ScrollView>

      {role && (
        <RoleSwitchSheet
          visible={switchSheetVisible}
          onClose={() => setSwitchSheetVisible(false)}
          onConfirm={() => {
            setRole(otherRole);
            setSwitchSheetVisible(false);
          }}
          currentRole={role}
          nextRole={otherRole}
        />
      )}
    </SafeAreaView>
  );
}
