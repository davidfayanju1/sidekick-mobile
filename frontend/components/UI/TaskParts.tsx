import {
  BrushCleaning,
  Hammer,
  Laptop,
  Package,
  ShoppingBag,
  Sparkles,
  Truck,
  type LucideIcon,
} from 'lucide-react-native';
import * as React from 'react';
import { View } from 'react-native';

import Text from '@/components/UI/Text';
import { tw, twColor } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';

export const ACCENT_TEAL = '#489A9F';
export const CATEGORY_COLOR = '#D1573B';

export const CARD_SHADOW = {
  shadowColor: '#000',
  shadowOpacity: 0.06,
  shadowRadius: 8,
  shadowOffset: { width: 0, height: 2 },
  elevation: 2,
};

export type TaskStatus = 'matched' | 'in_progress' | 'offers' | 'scheduled' | 'dispute';

export type Task = {
  id: string;
  category: string;
  status: TaskStatus;
  statusLabel: string;
  title: string;
  meta: string;
  currentStep?: number;
  offersText?: string;
  disputeText?: string;
  payIn?: string;
  price?: string;
  sidekickName?: string;
  offerCount?: number;
  heroName?: string;
  description?: string;
  etaWindow?: string;
};

// A task nearby that a Sidekick hasn't accepted yet — shown in the swipe deck.
export type BrowsableTask = {
  id: string;
  category: string;
  title: string;
  description: string;
  budget: string;
  location: string;
  distanceLabel: string;
  postedAgo: string;
  etaWindow: string;
  heroName: string;
  heroInitial: string;
  heroRating: number;
};

export const STATUS_STYLES: Record<TaskStatus, { bg: string; text: string }> = {
  matched: { bg: '#DCEDEE', text: ACCENT_TEAL },
  in_progress: { bg: '#F5E6D3', text: '#B5762E' },
  offers: { bg: '#DCF5E3', text: '#2F9E56' },
  scheduled: { bg: '#DCE8FB', text: '#3B6FD1' },
  dispute: { bg: '#FBE2DC', text: '#D1573B' },
};

export const STEPS = ['Posted', 'Matched', 'In Progress', 'Confirm', 'Payment'];

export type CategoryMeta = { color: string; icon: LucideIcon };

export const QUICK_CATEGORIES: { name: string; label: string; color: string; icon: LucideIcon }[] =
  [
    { name: 'Cleaning', label: 'Cleaning', color: ACCENT_TEAL, icon: BrushCleaning },
    { name: 'Delivery', label: 'Delivery', color: CATEGORY_COLOR, icon: Package },
    { name: 'Tech Help', label: 'Tech Help', color: '#3B6FD1', icon: Laptop },
    { name: 'Moving & Assembly', label: 'Moving', color: '#B5762E', icon: Truck },
    { name: 'Handyman', label: 'Handyman', color: '#8B5CF6', icon: Hammer },
    { name: 'Personal Errands', label: 'Errands', color: '#2F9E56', icon: ShoppingBag },
  ];

const DEFAULT_CATEGORY_META: CategoryMeta = { color: CATEGORY_COLOR, icon: Sparkles };

const CATEGORY_META: Record<string, CategoryMeta> = QUICK_CATEGORIES.reduce(
  (acc, { name, color, icon }) => ({ ...acc, [name]: { color, icon } }),
  {} as Record<string, CategoryMeta>
);

export function getCategoryMeta(category: string): CategoryMeta {
  return CATEGORY_META[category] ?? DEFAULT_CATEGORY_META;
}

export function browsableToTask(browsable: BrowsableTask): Task {
  return {
    id: browsable.id,
    category: browsable.category,
    status: 'matched',
    statusLabel: 'Matched',
    title: browsable.title,
    meta: `For ${browsable.heroName} • ${browsable.location}`,
    currentStep: 1,
    payIn: "You'll earn",
    price: browsable.budget,
    heroName: browsable.heroName,
    description: browsable.description,
    etaWindow: browsable.etaWindow,
  };
}

export function parseNaira(value?: string) {
  if (!value) return 0;
  const digits = value.replace(/[^0-9]/g, '');
  return digits ? Number(digits) : 0;
}

export function formatNaira(value: number) {
  return `₦${value.toLocaleString('en-US')}`;
}

export function formatTimeAgo(timestamp: number) {
  const minutes = Math.floor((Date.now() - timestamp) / 60000);
  if (minutes < 1) return 'Just now';
  if (minutes < 60) return `${minutes} min${minutes === 1 ? '' : 's'} ago`;
  const hours = Math.floor(minutes / 60);
  return `${hours} hr${hours === 1 ? '' : 's'} ago`;
}

export function StatusPill({ status, label }: { status: TaskStatus; label: string }) {
  const { bg, text } = STATUS_STYLES[status];
  return (
    <View style={[tw`rounded-full px-2.5 py-1`, { backgroundColor: bg }]}>
      <Text fontSize={11} classN={`text-[${text}]`}>
        {label}
      </Text>
    </View>
  );
}

export function ProgressTracker({ currentStep }: { currentStep: number }) {
  const { colors } = useColorScheme();
  const muted = twColor(colors.mutedForeground);

  return (
    <View style={tw`mt-3`}>
      <View style={tw`flex-row items-center`}>
        {STEPS.map((_, index) => (
          <React.Fragment key={index}>
            <View
              style={[
                tw`h-5 w-5 items-center justify-center rounded-full`,
                index < currentStep
                  ? { backgroundColor: ACCENT_TEAL }
                  : index === currentStep
                    ? { borderWidth: 2, borderColor: ACCENT_TEAL, backgroundColor: colors.card }
                    : { borderWidth: 1.5, borderColor: colors.grey4, backgroundColor: colors.card },
              ]}>
              {index < currentStep ? (
                <View style={tw`h-1.5 w-1.5 rounded-full bg-white`} />
              ) : index === currentStep ? (
                <View style={[tw`h-1.5 w-1.5 rounded-full`, { backgroundColor: ACCENT_TEAL }]} />
              ) : null}
            </View>
            {index < STEPS.length - 1 && (
              <View
                style={[
                  tw`h-px flex-1`,
                  { backgroundColor: index < currentStep ? ACCENT_TEAL : colors.grey4 },
                ]}
              />
            )}
          </React.Fragment>
        ))}
      </View>
      <View style={tw`mt-1 flex-row`}>
        {STEPS.map((label) => (
          <Text key={label} fontSize={9} classN={`w-11 text-center text-[${muted}]`}>
            {label}
          </Text>
        ))}
      </View>
    </View>
  );
}
