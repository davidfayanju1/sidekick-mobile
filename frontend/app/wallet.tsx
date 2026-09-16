import { router } from 'expo-router';
import { Check, ChevronLeft, ChevronRight, Plus, Wallet as WalletIcon } from 'lucide-react-native';
import * as React from 'react';
import { Pressable, ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import BottomSheet from '@/components/UI/BottomSheet';
import Text from '@/components/UI/Text';
import { CARD_SHADOW, formatNaira } from '@/components/UI/TaskParts';
import { HERO_TRANSACTIONS, SIDEKICK_TRANSACTIONS, TransactionRow } from '@/app/payment-history';
import { tw, twColor } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';
import { useRoleStore } from '@/store/roleStore';
import { withOpacity } from '@/theme/with-opacity';
import { colors } from '@/theme/palette';

function sumBy(
  transactions: typeof HERO_TRANSACTIONS,
  predicate: (t: (typeof HERO_TRANSACTIONS)[number]) => boolean
) {
  return transactions.filter(predicate).reduce((sum, t) => sum + Math.abs(t.amount), 0);
}

export default function Wallet() {
  const { colors: systemColors } = useColorScheme();
  const role = useRoleStore((state) => state.role);
  const bankDetailsAdded = useRoleStore((state) => state.bankDetailsAdded);
  const isSidekick = role === 'sidekick';
  const fg = twColor(systemColors.foreground);

  const [withdrawSheetVisible, setWithdrawSheetVisible] = React.useState(false);
  const [withdrawn, setWithdrawn] = React.useState(false);

  const transactions = isSidekick ? SIDEKICK_TRANSACTIONS : HERO_TRANSACTIONS;
  const recent = transactions.slice(0, 3);

  const totalSpent = sumBy(transactions, (t) => t.type === 'payment' && t.status === 'completed');
  const availableBalance =
    sumBy(transactions, (t) => t.type === 'earning' && t.status === 'completed') -
    sumBy(transactions, (t) => t.type === 'withdrawal' && t.status === 'completed');

  const secondaryAmount = isSidekick
    ? sumBy(transactions, (t) => t.type === 'earning' && t.status === 'pending')
    : sumBy(transactions, (t) => t.type === 'payment' && t.status === 'pending');

  const handleWithdrawPress = () => {
    if (!bankDetailsAdded) {
      router.push('/bank-details');
      return;
    }
    setWithdrawn(false);
    setWithdrawSheetVisible(true);
  };

  return (
    <SafeAreaView
      style={[tw`flex-1`, { backgroundColor: systemColors.background }]}
      edges={['top', 'bottom']}>
      <View style={tw`flex-row items-center gap-3 px-4 pb-3 pt-2`}>
        <Pressable onPress={() => router.back()} hitSlop={8}>
          <ChevronLeft size={24} color={systemColors.foreground} />
        </Pressable>
        <Text fontWeight="bold" fontSize={16} classN={`text-[${fg}]`}>
          Wallet
        </Text>
      </View>

      <ScrollView
        style={tw`flex-1 px-4`}
        contentContainerStyle={{ paddingBottom: 40 }}
        showsVerticalScrollIndicator={false}>
        <View style={[tw`mt-2 rounded-3xl p-5`, { backgroundColor: colors.brand }]}>
          <View style={tw`flex-row items-center gap-2`}>
            <WalletIcon size={16} color="white" />
            <Text fontSize={12} classN="text-white opacity-80">
              {isSidekick ? 'Available Balance' : 'Total Spent'}
            </Text>
          </View>
          <Text fontWeight="black" fontSize={30} classN="mt-1 text-white">
            {formatNaira(isSidekick ? availableBalance : totalSpent)}
          </Text>

          <View
            style={[
              tw`mt-4 flex-row items-center justify-between rounded-2xl px-4 py-3`,
              { backgroundColor: withOpacity(colors.surfaceWhite, 0.14) },
            ]}>
            <Text fontSize={12} classN="text-white opacity-80">
              {isSidekick ? 'Pending Payout' : 'In Escrow'}
            </Text>
            <Text fontWeight="bold" fontSize={14} classN="text-white">
              {formatNaira(secondaryAmount)}
            </Text>
          </View>

          <Pressable
            onPress={isSidekick ? handleWithdrawPress : () => router.push('/payment-method')}
            style={[
              tw`mt-4 flex-row items-center justify-center gap-2 rounded-full py-3.5`,
              { backgroundColor: 'white' },
            ]}>
            {!isSidekick && <Plus size={16} color={colors.brand} />}
            <Text fontWeight="bold" fontSize={14} classN={`text-[${twColor(colors.brand)}]`}>
              {isSidekick ? 'Withdraw to bank' : 'Add funds'}
            </Text>
          </Pressable>
        </View>

        <View style={tw`mt-6 flex-row items-center justify-between`}>
          <Text fontWeight="bold" fontSize={14} classN={`text-[${fg}]`}>
            Recent Activity
          </Text>
          <Pressable onPress={() => router.push('/payment-history')}>
            <View style={tw`flex-row items-center gap-0.5`}>
              <Text fontSize={12} classN={`text-[${twColor(colors.brand)}]`}>
                See all
              </Text>
              <ChevronRight size={14} color={colors.brand} />
            </View>
          </Pressable>
        </View>

        <View style={tw`mt-3 gap-3`}>
          {recent.map((transaction) => (
            <TransactionRow key={transaction.id} transaction={transaction} />
          ))}
        </View>
      </ScrollView>

      <BottomSheet visible={withdrawSheetVisible} onClose={() => setWithdrawSheetVisible(false)}>
        {withdrawn ? (
          <View style={tw`items-center px-2 py-2`}>
            <View
              style={[
                tw`h-14 w-14 items-center justify-center rounded-full`,
                { backgroundColor: colors.success },
              ]}>
              <Check size={26} color="white" strokeWidth={3} />
            </View>
            <Text fontWeight="black" fontSize={17} classN="mt-4 text-black">
              Withdrawal started
            </Text>
            <Text fontSize={13} classN={`mt-1.5 text-center text-[${colors.textSecondary}]`}>
              {formatNaira(availableBalance)} is on its way to your bank account. This usually takes
              up to 24 hours.
            </Text>
            <Pressable
              onPress={() => setWithdrawSheetVisible(false)}
              style={[
                tw`mt-5 w-full items-center justify-center rounded-full py-4`,
                { backgroundColor: colors.brand },
              ]}>
              <Text fontWeight="bold" fontSize={15} classN="text-white">
                Done
              </Text>
            </Pressable>
          </View>
        ) : (
          <View style={[CARD_SHADOW]}>
            <Text fontWeight="black" fontSize={19} classN="text-black">
              Withdraw to bank
            </Text>
            <Text fontSize={13} classN={`mt-1.5 text-[${colors.textSecondary}]`}>
              {formatNaira(availableBalance)} will be sent to your linked bank account.
            </Text>
            <Pressable
              onPress={() => setWithdrawn(true)}
              style={[
                tw`mt-5 items-center justify-center rounded-full py-4`,
                { backgroundColor: colors.brand },
              ]}>
              <Text fontWeight="bold" fontSize={15} classN="text-white">
                Confirm withdrawal
              </Text>
            </Pressable>
          </View>
        )}
      </BottomSheet>
    </SafeAreaView>
  );
}
