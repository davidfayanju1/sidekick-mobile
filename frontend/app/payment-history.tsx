import { router } from 'expo-router';
import { ArrowDownLeft, ArrowUpRight, ChevronLeft, Landmark } from 'lucide-react-native';
import * as React from 'react';
import { ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import Text from '@/components/UI/Text';
import { CARD_SHADOW } from '@/components/UI/TaskParts';
import { tw, twColor } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';
import { EmptyState, ErrorState } from '@/components/UI/EmptyState';
import { FlatLedger } from '@/components/UI/illustrations';
import { Skeleton } from '@/components/UI/Skeleton';
import { toDisplayTransaction } from '@/lib/api/adapters';
import { useTransactions } from '@/lib/api/finance';
import { errorMessage } from '@/lib/axios';
import { money } from '@/lib/format';
import { useAuthStore } from '@/store/authStore';
import { useRoleStore } from '@/store/roleStore';
import { colors } from '@/theme/palette';
import { PressableScale } from '@/components/UI/PressableScale';
import { StaggerItem } from '@/components/UI/Stagger';

export type TransactionType = 'payment' | 'refund' | 'earning' | 'withdrawal';

export type Transaction = {
  id: string;
  type: TransactionType;
  title: string;
  date: string;
  amount: number;
  status: 'completed' | 'pending' | 'failed';
};

const TYPE_META: Record<TransactionType, { icon: typeof ArrowUpRight; color: string }> = {
  payment: { icon: ArrowUpRight, color: colors.danger },
  refund: { icon: ArrowDownLeft, color: colors.green },
  earning: { icon: ArrowDownLeft, color: colors.green },
  withdrawal: { icon: Landmark, color: colors.orangeDark },
};

const HERO_FILTERS: { key: 'all' | TransactionType; label: string }[] = [
  { key: 'all', label: 'All' },
  { key: 'payment', label: 'Payments' },
  { key: 'refund', label: 'Refunds' },
];

const SIDEKICK_FILTERS: { key: 'all' | TransactionType; label: string }[] = [
  { key: 'all', label: 'All' },
  { key: 'earning', label: 'Earnings' },
  { key: 'withdrawal', label: 'Withdrawals' },
];

export function TransactionRow({ transaction }: { transaction: Transaction }) {
  const { colors: systemColors } = useColorScheme();
  const fg = twColor(systemColors.foreground);
  const muted = twColor(systemColors.mutedForeground);
  const { icon: Icon, color } = TYPE_META[transaction.type];
  const isCredit = transaction.amount > 0;

  return (
    <View
      style={[
        tw`flex-row items-center gap-3 rounded-2xl p-3.5`,
        { backgroundColor: systemColors.card },
        CARD_SHADOW,
      ]}>
      <View
        style={[
          tw`h-10 w-10 items-center justify-center rounded-full`,
          { backgroundColor: `${color}1F` },
        ]}>
        <Icon size={17} color={color} />
      </View>
      <View style={tw`flex-1`}>
        <Text variant="caption" fontWeight="medium" classN={`text-[${fg}]`} numberOfLines={1}>
          {transaction.title}
        </Text>
        <Text variant="micro" fontWeight="normal" classN={`mt-0.5 text-[${muted}]`}>
          {transaction.date}
          {transaction.status === 'pending' ? ' • Pending' : ''}
          {transaction.status === 'failed' ? ' • Failed' : ''}
        </Text>
      </View>
      <Text
        variant="body"
        fontWeight="medium"

        classN={`text-[${isCredit ? twColor(colors.green) : fg}]`}>
        {isCredit ? '+' : '-'}
        {money(Math.abs(transaction.amount))}
      </Text>
    </View>
  );
}

export default function PaymentHistory() {
  const { colors: systemColors } = useColorScheme();
  const role = useRoleStore((state) => state.role);
  const isSidekick = role === 'sidekick';
  const fg = twColor(systemColors.foreground);
  const muted = twColor(systemColors.mutedForeground);

  const currentUserId = useAuthStore((state) => state.user?.id);
  const ledger = useTransactions();

  const transactions = React.useMemo(
    () => (ledger.data ?? []).map((item) => toDisplayTransaction(item, currentUserId)),
    [ledger.data, currentUserId]
  );

  const filters = isSidekick ? SIDEKICK_FILTERS : HERO_FILTERS;
  const [filter, setFilter] = React.useState<'all' | TransactionType>('all');

  const visible =
    filter === 'all' ? transactions : transactions.filter((item) => item.type === filter);

  return (
    <SafeAreaView
      style={[tw`flex-1`, { backgroundColor: systemColors.background }]}
      edges={['top', 'bottom']}>
      <View style={tw`flex-row items-center gap-3 px-4 pb-3 pt-2`}>
        <PressableScale onPress={() => router.back()} hitSlop={8}>
          <ChevronLeft size={24} color={systemColors.foreground} />
        </PressableScale>
        <Text variant="subhead" fontWeight="bold" classN={`text-[${fg}]`}>
          Payment History
        </Text>
      </View>

      <View style={tw`flex-row gap-2 px-4 pb-3`}>
        {filters.map(({ key, label }) => {
          const isActive = filter === key;
          return (
            <PressableScale
              key={key}
              onPress={() => setFilter(key)}
              style={[
                tw`rounded-full px-4 py-2`,
                isActive
                  ? { backgroundColor: colors.primary }
                  : { borderWidth: 1, borderColor: systemColors.grey5 },
              ]}>
              <Text
                fontWeight="medium"
                fontSize={12.5}
                classN={isActive ? 'text-white' : `text-[${muted}]`}>
                {label}
              </Text>
            </PressableScale>
          );
        })}
      </View>

      <ScrollView
        style={tw`flex-1 px-4`}
        contentContainerStyle={{ paddingBottom: 40 }}
        showsVerticalScrollIndicator={false}>
        {ledger.isPending ? (
          <View style={tw`gap-3`}>
            {[0, 1, 2, 3, 4].map((index) => (
              <Skeleton key={index} index={index} height={68} radius={16} />
            ))}
          </View>
        ) : ledger.isError ? (
          <ErrorState body={errorMessage(ledger.error)} onRetry={() => ledger.refetch()} />
        ) : visible.length === 0 ? (
          <EmptyState
            illustration={<FlatLedger />}
            title={filter === 'all' ? 'No money has moved yet' : 'Nothing under this filter'}
            body={
              filter === 'all'
                ? isSidekick
                  ? 'Your earnings will appear here once you complete your first task.'
                  : 'Payments show up here as soon as you fund your first task.'
                : 'Try a different filter to see the rest of your history.'
            }
          />
        ) : (
          <View style={tw`gap-3`}>
            {visible.map((transaction, i) => (
              <StaggerItem key={transaction.id} index={i}>
                <TransactionRow transaction={transaction} />
              </StaggerItem>
            ))}
          </View>
        )}
      </ScrollView>
    </SafeAreaView>
  );
}
