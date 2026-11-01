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
  Smartphone,
  Sun,
  Wallet,
} from 'lucide-react-native';
import * as React from 'react';
import { ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import RoleSwitchSheet from '@/components/UI/RoleSwitchSheet';
import Text from '@/components/UI/Text';
import { tw, twColor } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';
import { ConfirmSheet } from '@/components/UI/ConfirmSheet';
import { VerificationBadge } from '@/components/UI/VerificationBadge';
import { useAuthStore } from '@/store/authStore';
import { Spinner } from '@/components/UI/Spinner';
import { useMe, useSetRole, useSignOut } from '@/lib/api/auth';
import { errorMessage } from '@/lib/axios';
import {
  intentFromRole,
  useOutstandingStepsCount,
  useRoleStore,
  type UserRole,
} from '@/store/roleStore';
import { toast } from '@/store/toastStore';
import type { ThemePreference } from '@/store/themeStore';
import { withOpacity } from '@/theme/with-opacity';
import { colors } from '@/theme/palette';
import { PressableScale } from '@/components/UI/PressableScale';

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
  { icon: Wallet, label: 'Wallet', onPress: () => router.push('/wallet') },
  { icon: Receipt, label: 'Payment History', onPress: () => router.push('/payment-history') },
  { icon: FileText, label: 'Terms & Conditions', onPress: () => router.push('/terms') },
  { icon: Gift, label: 'Refer a friend', onPress: () => router.push('/refer') },
];

export default function Profile() {
  const { colors: systemColors, preference, setPreference } = useColorScheme();
  const role = useRoleStore((state) => state.role);
  const setRoleIntent = useSetRole();
  const me = useMe();
  const signOut = useSignOut();
  // `setSession` stores the user from the sign-in response, so the name is
  // available before /me resolves — and survives it failing. Falling back to a
  // placeholder only when we genuinely have no user avoids a signed-in profile
  // reading "Your profile".
  const storedUser = useAuthStore((state) => state.user);
  const displayName = me.data?.display_name ?? storedUser?.display_name ?? 'Your profile';
  const initial = displayName.charAt(0).toUpperCase();

  const [signOutSheetVisible, setSignOutSheetVisible] = React.useState(false);

  const handleSignOut = async () => {
    // signOut swallows server errors; the local session is cleared either way.
    // Leaving for the landing screen is `useSessionGuard`'s job — doing it here
    // as well would mean two routes out of the app that have to agree.
    await signOut.mutateAsync().catch(() => {});
    setSignOutSheetVisible(false);
  };
  const setRole = useRoleStore((state) => state.setRole);
  const paymentMethodAdded = useRoleStore((state) => state.paymentMethodAdded);
  const idVerificationStatus = useRoleStore((state) => state.idVerificationStatus);
  const bankDetailsAdded = useRoleStore((state) => state.bankDetailsAdded);
  const outstandingSteps = useOutstandingStepsCount();
  const [switchSheetVisible, setSwitchSheetVisible] = React.useState(false);

  const fg = twColor(systemColors.foreground);
  const muted = twColor(systemColors.mutedForeground);

  const isSidekick = role === 'sidekick';
  const otherRole: UserRole = isSidekick ? 'hero' : 'sidekick';

  const handleRoleSwitch = async () => {
    // Switching sides changes what the whole app shows, so it waits for the
    // server rather than assuming. The sheet stays open and spinning until the
    // account actually reflects the new role.
    try {
      await setRoleIntent.mutateAsync(intentFromRole(otherRole));
      setRole(otherRole);
      setSwitchSheetVisible(false);
      toast.success(`You're now a ${otherRole === 'hero' ? 'Hero' : 'Sidekick'}`);
    } catch (error) {
      toast.error('Couldn’t switch your role', { description: errorMessage(error) });
    }
  };

  // Phone verification is account-level, not role-specific: both sides of the
  // marketplace need a reachable number, and an account created by signing in
  // or via OAuth has none on file.
  const phoneVerified = (me.data ?? storedUser)?.phone_verified === true;
  const phoneStep = {
    key: 'phone',
    title: 'Verify your phone number',
    icon: Smartphone,
    done: phoneVerified,
    pending: false,
    onPress: () => router.push('/verify-phone'),
  };

  const setupSteps = isSidekick
    ? [
        phoneStep,
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
          phoneStep,
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
    <SafeAreaView style={[tw`flex-1`, { backgroundColor: systemColors.background }]}>
      <ScrollView
        style={tw`flex-1 px-4`}
        contentContainerStyle={{ paddingBottom: 110 }}
        showsVerticalScrollIndicator={false}>
        <View style={tw`mt-3 flex-row items-center gap-3`}>
          <View
            style={[
              tw`h-14 w-14 items-center justify-center rounded-full`,
              { backgroundColor: colors.orangeTint },
            ]}>
            <Text variant="subhead" fontWeight="bold" classN={`text-[${colors.orangeDark}]`}>
              {initial}
            </Text>
          </View>
          <View>
            <Text variant="subhead" fontWeight="bold" classN={`text-[${fg}]`}>
              {displayName}
            </Text>
            <View style={tw`mt-1 flex-row flex-wrap items-center gap-1.5`}>
              {role && (
                <View
                  style={[
                    tw`self-start rounded-full px-2.5 py-0.5`,
                    { backgroundColor: withOpacity(colors.primary, 0.14) },
                  ]}>
                  <Text variant="micro" classN={`text-[${twColor(colors.primary)}]`}>
                    {ROLE_LABEL[role]}
                  </Text>
                </View>
              )}
              <VerificationBadge
                status={idVerificationStatus}
                onPress={() => router.push('/verify-id')}
              />
            </View>
          </View>
        </View>

        <Text variant="caption" fontWeight="medium" classN={`mt-8 text-[${fg}]`}>
          Appearance
        </Text>
        <View
          style={[
            tw`mt-2.5 flex-row rounded-2xl p-1`,
            {
              backgroundColor: systemColors.grey6,
              borderWidth: 1,
              borderColor: systemColors.grey5,
            },
          ]}>
          {APPEARANCE_OPTIONS.map(({ value, label, icon: Icon }) => {
            const isActive = preference === value;
            return (
              <PressableScale
                key={value}
                onPress={() => setPreference(value)}
                style={[
                  tw`flex-1 flex-row items-center justify-center gap-1.5 rounded-xl py-2.5`,
                  isActive && { backgroundColor: colors.primary },
                ]}>
                <Icon size={15} color={isActive ? 'white' : systemColors.mutedForeground} />
                <Text
                  variant="caption"
                  fontWeight="medium"

                  classN={isActive ? 'text-white' : `text-[${muted}]`}>
                  {label}
                </Text>
              </PressableScale>
            );
          })}
        </View>

        {role && (
          <>
            <Text variant="caption" fontWeight="medium" classN={`mt-8 text-[${fg}]`}>
              Role
            </Text>
            <View
              style={[
                tw`mt-2.5 rounded-2xl p-4`,
                {
                  backgroundColor: systemColors.card,
                  borderWidth: 1,
                  borderColor: systemColors.grey5,
                },
              ]}>
              <Text variant="body" fontWeight="medium" classN={`text-[${fg}]`}>
                You&rsquo;re a {ROLE_LABEL[role]}
              </Text>
              <Text variant="caption" classN={`mt-1 text-[${muted}]`}>
                {ROLE_DESCRIPTION[role]}
              </Text>
              <PressableScale
                onPress={() => setSwitchSheetVisible(true)}
                style={[
                  tw`mt-2.5 items-center justify-center rounded-full py-3`,
                  { backgroundColor: colors.orangeMuted },
                ]}>
                <Text variant="caption" fontWeight="medium" classN="text-white">
                  Switch to {ROLE_LABEL[otherRole]}
                </Text>
              </PressableScale>
            </View>
          </>
        )}

        {setupSteps.length > 0 && (
          <>
            <View style={tw`mt-8 flex-row items-center justify-between`}>
              <Text variant="caption" fontWeight="medium" classN={`text-[${fg}]`}>
                Account setup
              </Text>
              {outstandingSteps > 0 && (
                <Text
                  variant="micro"
                  fontWeight="normal"
                  classN={`text-[${twColor('${colors.orangeDark}')}]`}>
                  {outstandingSteps} left
                </Text>
              )}
            </View>
            <View style={tw`mt-2.5 gap-2.5`}>
              {setupSteps.map(({ key, title, icon: Icon, done, pending, onPress }) => (
                <PressableScale
                  key={key}
                  onPress={onPress}
                  style={[
                    tw`flex-row items-center gap-3 rounded-2xl p-3.5`,
                    {
                      backgroundColor: systemColors.card,
                      borderWidth: 1,
                      borderColor: systemColors.grey5,
                    },
                  ]}>
                  <Icon size={18} color={done ? colors.green : systemColors.mutedForeground} />
                  <Text variant="caption" fontWeight="medium" classN={`flex-1 text-[${fg}]`}>
                    {title}
                  </Text>
                  <Text variant="micro" fontWeight="normal" classN={`text-[${muted}]`}>
                    {done ? 'Done' : pending ? 'Pending' : 'Add'}
                  </Text>
                  <ChevronRight size={16} color={systemColors.mutedForeground} />
                </PressableScale>
              ))}
            </View>
          </>
        )}

        <Text variant="caption" fontWeight="medium" classN={`mt-8 text-[${fg}]`}>
          General
        </Text>
        <View
          style={[
            tw`mt-2.5 rounded-2xl px-1`,
            { backgroundColor: systemColors.card, borderWidth: 1, borderColor: systemColors.grey5 },
          ]}>
          {MENU_ITEMS.map(({ icon: Icon, label, onPress }, index) => (
            <PressableScale
              key={label}
              onPress={onPress}
              style={[
                tw`flex-row items-center gap-3 px-3.5 py-3.5`,
                index < MENU_ITEMS.length - 1 && {
                  borderBottomWidth: 1,
                  borderBottomColor: systemColors.grey5,
                },
              ]}>
              <Icon size={18} color={systemColors.mutedForeground} />
              <Text variant="body" classN={`flex-1 text-[${fg}]`}>
                {label}
              </Text>
              <ChevronRight size={16} color={systemColors.mutedForeground} />
            </PressableScale>
          ))}
        </View>

        <PressableScale
          onPress={() => setSignOutSheetVisible(true)}
          disabled={signOut.isPending}
          style={[
            tw`mt-6 flex-row items-center justify-center gap-2 rounded-2xl py-3.5`,
            { borderWidth: 1, borderColor: systemColors.grey5 },
          ]}>
          {signOut.isPending ? (
            <Spinner size={16} color={colors.danger} />
          ) : (
            <LogOut size={16} color={colors.danger} />
          )}
          <Text variant="caption" fontWeight="medium" classN={`text-[${twColor(colors.danger)}]`}>
            Sign Out
          </Text>
        </PressableScale>
      </ScrollView>

      <ConfirmSheet
        visible={signOutSheetVisible}
        onClose={() => setSignOutSheetVisible(false)}
        onConfirm={handleSignOut}
        loading={signOut.isPending}
        destructive
        title="Sign out?"
        message="You'll need your email and password to get back in. Anything in progress stays on your account."
        confirmLabel="Sign out"
        cancelLabel="Stay signed in"
      />

      {role && (
        <RoleSwitchSheet
          visible={switchSheetVisible}
          onClose={() => setSwitchSheetVisible(false)}
          onConfirm={handleRoleSwitch}
          loading={setRoleIntent.isPending}
          currentRole={role}
          nextRole={otherRole}
        />
      )}
    </SafeAreaView>
  );
}
