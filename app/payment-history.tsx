import { router } from 'expo-router';
import { ArrowDownLeft, ArrowUpRight, ChevronLeft, Landmark } from 'lucide-react-native';
import * as React from 'react';
import { Pressable, ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import Text from '@/components/UI/Text';
import { CARD_SHADOW, formatNaira } from '@/components/UI/TaskParts';
import { tw, twColor } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';
import { useRoleStore } from '@/store/roleStore';

const ACCENT_TEAL = '#489A9F';
const SUCCESS = '#2F9E56';

export type TransactionType = 'payment' | 'refund' | 'earning' | 'withdrawal';

export type Transaction = {
  id: string;
  type: TransactionType;
  title: string;
  date: string;
  amount: number;
  status: 'completed' | 'pending';
};

export const HERO_TRANSACTIONS: Transaction[] = [
  {
    id: 'h1',
    type: 'payment',
    title: 'Deep clean 2-bedroom apartment',
    date: 'Jul 24, 2026',
    amount: -50000,
    status: 'pending',
  },
  {
    id: 'h2',
    type: 'payment',
    title: 'Laptop battery repair',
    date: 'Jul 20, 2026',
    amount: -50000,
    status: 'pending',
  },
  {
    id: 'h3',
    type: 'refund',
    title: 'Pick up groceries — offer cancelled',
    date: 'Jul 15, 2026',
    amount: 12000,
    status: 'completed',
  },
  {
    id: 'h4',
    type: 'payment',
    title: 'Home WiFi setup',
    date: 'Jul 10, 2026',
    amount: -8000,
    status: 'completed',
  },
];

export const SIDEKICK_TRANSACTIONS: Transaction[] = [
  {
    id: 's1',
    type: 'earning',
    title: 'Weekly apartment cleaning',
    date: 'Jul 24, 2026',
    amount: 18000,
    status: 'pending',
  },
  {
    id: 's2',
    type: 'withdrawal',
    title: 'Withdrawal to GTBank ****1234',
    date: 'Jul 18, 2026',
    amount: -45000,
    status: 'completed',
  },
  {
    id: 's3',
    type: 'earning',
    title: 'Laptop battery repair',
    date: 'Jul 12, 2026',
    amount: 9800,
    status: 'completed',
  },
  {
    id: 's4',
    type: 'earning',
    title: 'Grocery delivery',
    date: 'Jul 5, 2026',
    amount: 6500,
    status: 'completed',
  },
  {
    id: 's5',
    type: 'earning',
    title: 'Furniture assembly',
    date: 'Jul 1, 2026',
    amount: 32000,
    status: 'completed',
  },
];

const TYPE_META: Record<TransactionType, { icon: typeof ArrowUpRight; color: string }> = {
  payment: { icon: ArrowUpRight, color: '#D1573B' },
  refund: { icon: ArrowDownLeft, color: SUCCESS },
  earning: { icon: ArrowDownLeft, color: SUCCESS },
  withdrawal: { icon: Landmark, color: '#B5762E' },
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
  const { colors } = useColorScheme();
  const fg = twColor(colors.foreground);
  const muted = twColor(colors.mutedForeground);
  const { icon: Icon, color } = TYPE_META[transaction.type];
  const isCredit = transaction.amount > 0;

  return (
    <View
      style={[
        tw`flex-row items-center gap-3 rounded-2xl p-3.5`,
        { backgroundColor: colors.card },
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
        <Text fontWeight="bold" fontSize={13} classN={`text-[${fg}]`} numberOfLines={1}>
          {transaction.title}
        </Text>
        <Text fontSize={11} classN={`mt-0.5 text-[${muted}]`}>
          {transaction.date}
          {transaction.status === 'pending' ? ' • Pending' : ''}
        </Text>
      </View>
      <Text fontWeight="bold" fontSize={14} classN={`text-[${isCredit ? twColor(SUCCESS) : fg}]`}>
        {isCredit ? '+' : '-'}
        {formatNaira(Math.abs(transaction.amount))}
      </Text>
    </View>
  );
}

export default function PaymentHistory() {
  const { colors } = useColorScheme();
  const role = useRoleStore((state) => state.role);
  const isSidekick = role === 'sidekick';
  const fg = twColor(colors.foreground);
  const muted = twColor(colors.mutedForeground);

  const transactions = isSidekick ? SIDEKICK_TRANSACTIONS : HERO_TRANSACTIONS;
  const filters = isSidekick ? SIDEKICK_FILTERS : HERO_FILTERS;
  const [filter, setFilter] = React.useState<'all' | TransactionType>('all');

  const visible =
    filter === 'all' ? transactions : transactions.filter((item) => item.type === filter);

  return (
    <SafeAreaView
      style={[tw`flex-1`, { backgroundColor: colors.background }]}
      edges={['top', 'bottom']}>
      <View style={tw`flex-row items-center gap-3 px-4 pb-3 pt-2`}>
        <Pressable onPress={() => router.back()} hitSlop={8}>
          <ChevronLeft size={24} color={colors.foreground} />
        </Pressable>
        <Text fontWeight="bold" fontSize={16} classN={`text-[${fg}]`}>
          Payment History
        </Text>
      </View>

      <View style={tw`flex-row gap-2 px-4 pb-3`}>
        {filters.map(({ key, label }) => {
          const isActive = filter === key;
          return (
            <Pressable
              key={key}
              onPress={() => setFilter(key)}
              style={[
                tw`rounded-full px-4 py-2`,
                isActive
                  ? { backgroundColor: ACCENT_TEAL }
                  : { borderWidth: 1, borderColor: colors.grey5 },
              ]}>
              <Text
                fontWeight="medium"
                fontSize={12.5}
                classN={isActive ? 'text-white' : `text-[${muted}]`}>
                {label}
              </Text>
            </Pressable>
          );
        })}
      </View>

      <ScrollView
        style={tw`flex-1 px-4`}
        contentContainerStyle={{ paddingBottom: 40 }}
        showsVerticalScrollIndicator={false}>
        {visible.length === 0 ? (
          <View style={tw`mt-16 items-center`}>
            <Text fontSize={13} classN={`text-[${muted}]`}>
              Nothing here yet
            </Text>
          </View>
        ) : (
          <View style={tw`gap-3`}>
            {visible.map((transaction) => (
              <TransactionRow key={transaction.id} transaction={transaction} />
            ))}
          </View>
        )}
      </ScrollView>
    </SafeAreaView>
  );
}
