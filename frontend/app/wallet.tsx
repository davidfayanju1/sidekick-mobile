import { router } from 'expo-router';
import { Check, ChevronLeft, ChevronRight, Plus, Wallet as WalletIcon } from 'lucide-react-native';
import * as React from 'react';
import { ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import BottomSheet from '@/components/UI/BottomSheet';
import Text from '@/components/UI/Text';
import { CARD_SHADOW } from '@/components/UI/TaskParts';
import { TransactionRow } from '@/app/payment-history';
import { Button } from '@/components/UI/Button';
import { EmptyState, ErrorState } from '@/components/UI/EmptyState';
import { FlatLedger } from '@/components/UI/illustrations';
import { Skeleton } from '@/components/UI/Skeleton';
import { toDisplayTransaction } from '@/lib/api/adapters';
import { usePayoutMethods, useTransactions, useWallet, useWithdraw } from '@/lib/api/finance';
import { apiError, errorMessage } from '@/lib/axios';
import { money } from '@/lib/format';
import { useAuthStore } from '@/store/authStore';
import { toast } from '@/store/toastStore';
import { tw, twColor } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';
import { useRoleStore } from '@/store/roleStore';
import { withOpacity } from '@/theme/with-opacity';
import { colors } from '@/theme/palette';
import { PressableScale } from '@/components/UI/PressableScale';
import { StaggerItem } from '@/components/UI/Stagger';

export default function Wallet() {
  const { colors: systemColors } = useColorScheme();
  const role = useRoleStore((state) => state.role);
  const bankDetailsAdded = useRoleStore((state) => state.bankDetailsAdded);
  const isSidekick = role === 'sidekick';
  const fg = twColor(systemColors.foreground);

  const [withdrawSheetVisible, setWithdrawSheetVisible] = React.useState(false);
  const [withdrawn, setWithdrawn] = React.useState(false);

  const currentUserId = useAuthStore((state) => state.user?.id);
  const verificationStatus = useAuthStore((state) => state.user?.verification_status);
  const wallet = useWallet();
  const ledger = useTransactions();
  const payoutMethods = usePayoutMethods();
  const withdraw = useWithdraw();

  const recent = React.useMemo(
    () => (ledger.data ?? []).slice(0, 3).map((item) => toDisplayTransaction(item, currentUserId)),
    [ledger.data, currentUserId]
  );

  // Balances are the server's to compute — summing the visible rows would be
  // wrong the moment the ledger is paginated or a transaction is still settling.
  const availableBalance = wallet.data?.available_balance ?? 0;
  const pendingBalance = wallet.data?.pending_balance ?? 0;

  // The Hero's headline is what they've committed, not a balance they hold.
  const totalSpent = React.useMemo(
    () =>
      (ledger.data ?? [])
        .filter((item) => item.type === 'escrow_hold' && item.status === 'succeeded')
        .reduce((sum, item) => sum + Math.abs(item.amount), 0),
    [ledger.data]
  );

  const handleWithdrawPress = () => {
    // Withdrawals are gated on ID verification server-side (FR-9.3, enforced in
    // finance.go Withdraw → 403 verification_required). Check it here so the
    // user gets sent to the ID screen instead of a dead-end error.
    if (verificationStatus !== 'verified') {
      toast.warning(
        verificationStatus === 'pending' ? 'ID check still in progress' : 'Verify your ID first',
        {
          description:
            verificationStatus === 'pending'
              ? 'You can withdraw as soon as your ID is approved.'
              : 'We have to verify who you are before sending money to a bank account.',
          action:
            verificationStatus === 'pending'
              ? undefined
              : { label: 'Verify ID', onPress: () => router.push('/verify-id') },
        }
      );
      return;
    }

    const payoutMethod =
      payoutMethods.data?.find((method) => method.is_default) ?? payoutMethods.data?.[0];
    if (!payoutMethod && !bankDetailsAdded) {
      router.push('/bank-details');
      return;
    }
    setWithdrawn(false);
    setWithdrawSheetVisible(true);
  };

  const handleConfirmWithdrawal = async () => {
    const payoutMethod =
      payoutMethods.data?.find((method) => method.is_default) ?? payoutMethods.data?.[0];

    if (!payoutMethod) {
      toast.warning('Add a bank account first', {
        action: { label: 'Add bank', onPress: () => router.push('/bank-details') },
      });
      return;
    }

    try {
      await withdraw.mutateAsync({
        amount: availableBalance,
        payoutMethodId: payoutMethod.id,
      });
      setWithdrawn(true);
    } catch (error) {
      const { status, message } = apiError(error);
      // The server re-checks verification and the balance; surface each plainly.
      toast.error(
        status === 403
          ? 'Verify your ID to withdraw'
          : status === 409
            ? 'Not enough available balance'
            : 'Withdrawal didn’t go through',
        { description: status === 403 ? 'Your ID hasn’t been approved yet.' : message }
      );
    }
  };

  return (
    <SafeAreaView
      style={[tw`flex-1`, { backgroundColor: systemColors.background }]}
      edges={['top', 'bottom']}>
      <View style={tw`flex-row items-center gap-3 px-4 pb-3 pt-2`}>
        <PressableScale onPress={() => router.back()} hitSlop={8}>
          <ChevronLeft size={24} color={systemColors.foreground} />
        </PressableScale>
        <Text variant="subhead" fontWeight="bold" classN={`text-[${fg}]`}>
          Wallet
        </Text>
      </View>

      <ScrollView
        style={tw`flex-1 px-4`}
        contentContainerStyle={{ paddingBottom: 40 }}
        showsVerticalScrollIndicator={false}>
        <View style={[tw`mt-2 rounded-3xl p-5`, { backgroundColor: colors.primary }]}>
          <View style={tw`flex-row items-center gap-2`}>
            <WalletIcon size={16} color="white" />
            <Text variant="caption" classN="text-white opacity-80">
              {isSidekick ? 'Available Balance' : 'Total Spent'}
            </Text>
          </View>
          {wallet.isPending ? (
            <View style={tw`mt-2`}>
              <Skeleton width={160} height={34} radius={10} />
            </View>
          ) : (
            <Text variant="display" classN="mt-1 text-white">
              {money(isSidekick ? availableBalance : totalSpent)}
            </Text>
          )}

          <View
            style={[
              tw`mt-3 flex-row items-center justify-between rounded-2xl px-4 py-3`,
              { backgroundColor: withOpacity(colors.white, 0.14) },
            ]}>
            <Text variant="caption" classN="text-white opacity-80">
              {isSidekick ? 'Pending Payout' : 'In Escrow'}
            </Text>
            <Text variant="body" fontWeight="medium" classN="text-white">
              {money(isSidekick ? pendingBalance : totalSpent - availableBalance)}
            </Text>
          </View>

          <PressableScale
            onPress={isSidekick ? handleWithdrawPress : () => router.push('/payment-method')}
            style={[
              tw`mt-3 flex-row items-center justify-center gap-2 rounded-full py-3.5`,
              { backgroundColor: 'white' },
            ]}>
            {!isSidekick && <Plus size={16} color={colors.primary} />}
            <Text variant="body" fontWeight="medium" classN={`text-[${twColor(colors.primary)}]`}>
              {isSidekick ? 'Withdraw to bank' : 'Add funds'}
            </Text>
          </PressableScale>
        </View>

        <View style={tw`mt-6 flex-row items-center justify-between`}>
          <Text variant="body" fontWeight="medium" classN={`text-[${fg}]`}>
            Recent Activity
          </Text>
          <PressableScale onPress={() => router.push('/payment-history')}>
            <View style={tw`flex-row items-center gap-0.5`}>
              <Text variant="caption" classN={`text-[${twColor(colors.primary)}]`}>
                See all
              </Text>
              <ChevronRight size={14} color={colors.primary} />
            </View>
          </PressableScale>
        </View>

        <View style={tw`mt-2 gap-3`}>
          {ledger.isPending ? (
            [0, 1, 2].map((index) => <Skeleton key={index} index={index} height={68} radius={16} />)
          ) : ledger.isError ? (
            <ErrorState
              compact
              body={errorMessage(ledger.error)}
              onRetry={() => ledger.refetch()}
            />
          ) : recent.length === 0 ? (
            <EmptyState
              compact
              illustration={<FlatLedger />}
              title="Nothing has moved yet"
              body={
                isSidekick
                  ? 'Complete a task and your earnings land here.'
                  : 'Fund a task and the escrow shows up here.'
              }
            />
          ) : (
            recent.map((transaction, i) => (
              <StaggerItem key={transaction.id} index={i}>
                <TransactionRow transaction={transaction} />
              </StaggerItem>
            ))
          )}
        </View>
      </ScrollView>

      <BottomSheet visible={withdrawSheetVisible} onClose={() => setWithdrawSheetVisible(false)}>
        {withdrawn ? (
          <View style={tw`items-center px-2 py-2`}>
            <View
              style={[
                tw`h-14 w-14 items-center justify-center rounded-full`,
                { backgroundColor: colors.green },
              ]}>
              <Check size={26} color="white" strokeWidth={3} />
            </View>
            <Text variant="subhead" fontWeight="black" classN="mt-3 text-black">
              Withdrawal started
            </Text>
            <Text variant="caption" classN={`mt-1.5 text-center text-[${colors.gray700}]`}>
              {money(availableBalance)} is on its way to your bank account. This usually takes up to
              24 hours.
            </Text>
            <PressableScale
              onPress={() => setWithdrawSheetVisible(false)}
              style={[
                tw`mt-5 w-full items-center justify-center rounded-full py-4`,
                { backgroundColor: colors.primary },
              ]}>
              <Text variant="body" fontWeight="medium" classN="text-white">
                Done
              </Text>
            </PressableScale>
          </View>
        ) : (
          <View style={[CARD_SHADOW]}>
            <Text variant="subhead" fontWeight="black" classN="text-black">
              Withdraw to bank
            </Text>
            <Text variant="caption" classN={`mt-1.5 text-[${colors.gray700}]`}>
              {money(availableBalance)} will be sent to your linked bank account.
            </Text>
            <View style={tw`mt-5`}>
              <Button
                label="Confirm withdrawal"
                onPress={handleConfirmWithdrawal}
                loading={withdraw.isPending}
                disabled={availableBalance <= 0}
                size="lg"
              />
            </View>
          </View>
        )}
      </BottomSheet>
    </SafeAreaView>
  );
}
