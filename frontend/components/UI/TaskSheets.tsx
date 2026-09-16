import { BadgeCheck, Check, Image as ImageIcon, Star } from 'lucide-react-native';
import React from 'react';
import { Pressable, ScrollView, TextInput, View } from 'react-native';

import BottomSheet from '@/components/UI/BottomSheet';
import Text from '@/components/UI/Text';
import { tw } from '@/lib/tw';
import { colors } from '@/theme/palette';

function PrimaryButton({ label, onPress }: { label: string; onPress?: () => void }) {
  return (
    <Pressable
      onPress={onPress}
      style={[
        tw`items-center justify-center rounded-full py-4`,
        { backgroundColor: colors.brand },
      ]}>
      <Text fontWeight="bold" fontSize={15} classN="text-white">
        {label}
      </Text>
    </Pressable>
  );
}

function OutlineButton({
  label,
  onPress,
  color = colors.textPrimary,
}: {
  label: string;
  onPress?: () => void;
  color?: string;
}) {
  return (
    <Pressable
      onPress={onPress}
      style={[
        tw`items-center justify-center rounded-full py-4`,
        { borderWidth: 1, borderColor: colors.borderLight, backgroundColor: 'white' },
      ]}>
      <Text fontWeight="bold" fontSize={15} classN={`text-[${color}]`}>
        {label}
      </Text>
    </Pressable>
  );
}

interface ConfirmTaskSheetProps {
  visible: boolean;
  onClose: () => void;
  onReportDispute: () => void;
  sidekickName: string;
  price: string;
  photoCount?: number;
}

export function ConfirmTaskSheet({
  visible,
  onClose,
  onReportDispute,
  sidekickName,
  price,
  photoCount = 2,
}: ConfirmTaskSheetProps) {
  return (
    <BottomSheet visible={visible} onClose={onClose}>
      <Text fontWeight="black" fontSize={20} classN="text-black">
        Confirm Task done?
      </Text>
      <Text fontSize={13} classN={`mt-1.5 text-[${colors.textSecondary}]`}>
        {sidekickName} has marked this complete. Confirm to release payment.
      </Text>

      <Pressable
        style={[
          tw`mt-4 flex-row items-center gap-3 rounded-2xl p-3`,
          { borderWidth: 1, borderColor: colors.borderLight },
        ]}>
        <View
          style={[
            tw`h-11 w-11 items-center justify-center rounded-xl`,
            { backgroundColor: colors.surface },
          ]}>
          <ImageIcon size={20} color={colors.textSecondary} />
        </View>
        <View style={tw`flex-1`}>
          <Text fontWeight="bold" fontSize={13} classN="text-black">
            {photoCount} photos attached as proof
          </Text>
          <Text fontSize={11} classN={`mt-0.5 text-[${colors.textMuted}]`}>
            Sent by {sidekickName.split(' ')[0]} • Tap to view
          </Text>
        </View>
      </Pressable>

      <View style={tw`mt-5 gap-3`}>
        <PrimaryButton label={`Confirm and pay ${price}`} onPress={onClose} />
        <OutlineButton label="Report a dispute" onPress={onReportDispute} color={colors.danger} />
      </View>
    </BottomSheet>
  );
}

const DISPUTE_REASONS = [
  {
    id: 'not_completed',
    title: 'Task not completed',
    description: "Sidekick didn't finish the work",
  },
  {
    id: 'poor_quality',
    title: 'Work quality is poor',
    description: 'Job done but not to standard',
  },
  { id: 'no_show', title: 'Sidekick no-showed', description: 'Accepted but never arrived' },
  {
    id: 'safety',
    title: 'Safety Concern',
    description: 'Report aggressive or suspicious behaviour',
  },
] as const;

interface DisputeSheetProps {
  visible: boolean;
  onClose: () => void;
  onSubmit: () => void;
}

export function DisputeSheet({ visible, onClose, onSubmit }: DisputeSheetProps) {
  const [selected, setSelected] = React.useState<string>(DISPUTE_REASONS[0].id);
  const [note, setNote] = React.useState('');

  return (
    <BottomSheet visible={visible} onClose={onClose}>
      <Text fontWeight="black" fontSize={20} classN="text-black">
        Raise a Dispute
      </Text>
      <Text fontSize={13} classN={`mt-1.5 text-[${colors.textSecondary}]`}>
        Select the issue. Our team reviews within 24h.
      </Text>

      <View style={tw`mt-1`}>
        {DISPUTE_REASONS.map((reason) => {
          const isSelected = selected === reason.id;
          return (
            <Pressable
              key={reason.id}
              onPress={() => setSelected(reason.id)}
              style={[
                tw`mt-3 flex-row items-start gap-3 rounded-2xl p-4`,
                { backgroundColor: colors.surface },
              ]}>
              <View
                style={[
                  tw`mt-0.5 h-5 w-5 items-center justify-center rounded-full`,
                  {
                    borderWidth: 2,
                    borderColor: isSelected ? colors.textPrimary : colors.borderRadio,
                  },
                ]}>
                {isSelected && (
                  <View style={tw`h-2.5 w-2.5 rounded-full bg-[${colors.textPrimary}]`} />
                )}
              </View>
              <View style={tw`flex-1`}>
                <Text fontWeight="bold" fontSize={14} classN="text-black">
                  {reason.title}
                </Text>
                <Text fontSize={12} classN={`mt-0.5 text-[${colors.textMuted}]`}>
                  {reason.description}
                </Text>
              </View>
            </Pressable>
          );
        })}
      </View>

      <Text fontWeight="bold" fontSize={13} classN="mt-4 text-black">
        Additional Information
      </Text>
      <TextInput
        multiline
        value={note}
        onChangeText={setNote}
        placeholder=""
        placeholderTextColor={colors.textMuted}
        style={[
          tw`mt-2 rounded-2xl p-4 text-black`,
          {
            borderWidth: 1,
            borderColor: colors.borderLight,
            minHeight: 96,
            textAlignVertical: 'top',
          },
        ]}
      />

      <View style={tw`mt-5 gap-3`}>
        <Pressable
          onPress={onSubmit}
          style={[
            tw`items-center justify-center rounded-full py-4`,
            { backgroundColor: colors.danger },
          ]}>
          <Text fontWeight="bold" fontSize={15} classN="text-white">
            Submit Dispute
          </Text>
        </Pressable>
        <OutlineButton label="Cancel" onPress={onClose} />
      </View>
    </BottomSheet>
  );
}

type OfferActionType = 'review' | 'start';

type Offer = {
  id: string;
  name: string;
  initial: string;
  avatarBg: string;
  avatarText: string;
  rating: number;
  tasksCompleted: number;
  price: string;
  priceLabel: string;
  reason?: string;
  actionType: OfferActionType;
};

const OFFERS: Offer[] = [
  {
    id: '1',
    name: 'Tunde A.',
    initial: 'T',
    avatarBg: colors.accentSand,
    avatarText: colors.pendingText,
    rating: 4,
    tasksCompleted: 45,
    price: '₦12,000',
    priceLabel: 'Counter-offer',
    reason: 'Traffic is heavy today, need extra time and fuel',
    actionType: 'review',
  },
  {
    id: '2',
    name: 'Pelumi Daniels',
    initial: 'P',
    avatarBg: colors.scheduledBg,
    avatarText: colors.scheduledText,
    rating: 4,
    tasksCompleted: 70,
    price: '₦15,000',
    priceLabel: 'Counter-offer',
    reason: 'Traffic is heavy today, need extra time and fuel',
    actionType: 'review',
  },
  {
    id: '3',
    name: 'John Grace',
    initial: 'J',
    avatarBg: colors.successBg,
    avatarText: colors.success,
    rating: 4,
    tasksCompleted: 45,
    price: '₦10,000',
    priceLabel: 'On budget',
    reason: 'Traffic is heavy today, need extra time and fuel',
    actionType: 'start',
  },
];

function StarRating({ rating }: { rating: number }) {
  return (
    <View style={tw`flex-row items-center gap-0.5`}>
      {Array.from({ length: 5 }).map((_, index) => (
        <Star
          key={index}
          size={10}
          color={index < rating ? colors.star : colors.borderLight}
          fill={index < rating ? colors.star : colors.borderLight}
        />
      ))}
    </View>
  );
}

function OfferCard({ offer }: { offer: Offer }) {
  return (
    <View style={[tw`mt-4 rounded-2xl p-4`, { borderWidth: 1, borderColor: colors.borderCard }]}>
      <View style={tw`flex-row items-start justify-between`}>
        <View style={tw`flex-1 flex-row items-center gap-2.5`}>
          <View
            style={[
              tw`h-10 w-10 items-center justify-center rounded-full`,
              { backgroundColor: offer.avatarBg },
            ]}>
            <Text fontWeight="bold" fontSize={14} classN={`text-[${offer.avatarText}]`}>
              {offer.initial}
            </Text>
          </View>
          <View style={tw`flex-1`}>
            <View style={tw`flex-row items-center gap-1`}>
              <Text fontWeight="bold" fontSize={14} classN="text-black">
                {offer.name}
              </Text>
              <BadgeCheck size={13} color={colors.brand} fill={colors.brand} />
            </View>
            <View style={tw`mt-1 flex-row items-center gap-1.5`}>
              <StarRating rating={offer.rating} />
              <Text fontSize={11} classN={`text-[${colors.textMuted}]`}>
                ({offer.tasksCompleted} Tasks)
              </Text>
            </View>
          </View>
        </View>
        <View style={tw`items-end`}>
          <Text fontWeight="bold" fontSize={15} classN="text-black">
            {offer.price}
          </Text>
          <Text fontSize={11} classN={`mt-0.5 text-[${colors.textMuted}]`}>
            {offer.priceLabel}
          </Text>
        </View>
      </View>

      {offer.reason && (
        <View style={tw`mt-3`}>
          <Text fontWeight="bold" fontSize={11} classN="text-black">
            Reasons for counter offer
          </Text>
          <View style={[tw`mt-1.5 rounded-xl px-3 py-2.5`, { backgroundColor: colors.surface }]}>
            <Text fontSize={12} classN={`italic text-[${colors.textSecondary}]`}>
              &ldquo;{offer.reason}&rdquo;
            </Text>
          </View>
        </View>
      )}

      <View style={tw`mt-3 flex-row gap-2`}>
        {offer.actionType === 'review' ? (
          <>
            <Pressable
              style={[
                tw`flex-1 items-center justify-center rounded-full py-3`,
                { borderWidth: 1, borderColor: colors.borderLight },
              ]}>
              <Text fontWeight="bold" fontSize={13} classN="text-black">
                Accept
              </Text>
            </Pressable>
            <Pressable
              style={[
                tw`flex-1 items-center justify-center rounded-full py-3`,
                { backgroundColor: colors.brand },
              ]}>
              <Text fontWeight="bold" fontSize={13} classN="text-white">
                Counter
              </Text>
            </Pressable>
            <Pressable
              style={[
                tw`flex-1 items-center justify-center rounded-full py-3`,
                { borderWidth: 1, borderColor: colors.borderLight },
              ]}>
              <Text fontWeight="bold" fontSize={13} classN="text-black">
                Decline
              </Text>
            </Pressable>
          </>
        ) : (
          <>
            <Pressable
              style={[
                tw`flex-1 items-center justify-center rounded-full py-3`,
                { backgroundColor: colors.brand },
              ]}>
              <Text fontWeight="bold" fontSize={13} classN="text-white">
                Start Task
              </Text>
            </Pressable>
            <Pressable
              style={[
                tw`flex-1 items-center justify-center rounded-full py-3`,
                { borderWidth: 1, borderColor: colors.borderLight },
              ]}>
              <Text fontWeight="bold" fontSize={13} classN="text-black">
                Cancel
              </Text>
            </Pressable>
          </>
        )}
      </View>
    </View>
  );
}

interface ReviewOffersSheetProps {
  visible: boolean;
  onClose: () => void;
  taskTitle: string;
  offerCount?: number;
}

export function ReviewOffersSheet({
  visible,
  onClose,
  taskTitle,
  offerCount = 4,
}: ReviewOffersSheetProps) {
  return (
    <BottomSheet visible={visible} onClose={onClose}>
      <Text fontWeight="black" fontSize={20} classN="text-black">
        Review offers
      </Text>
      <Text fontSize={13} classN={`mt-1.5 text-[${colors.textSecondary}]`}>
        {offerCount} Sidekicks offered on &ldquo;{taskTitle}&rdquo;
      </Text>

      <ScrollView
        style={{ maxHeight: 420 }}
        showsVerticalScrollIndicator={false}
        contentContainerStyle={tw`pb-1`}>
        {OFFERS.map((offer) => (
          <OfferCard key={offer.id} offer={offer} />
        ))}
      </ScrollView>

      <View style={tw`mt-4`}>
        <OutlineButton label="Close" onPress={onClose} />
      </View>
    </BottomSheet>
  );
}

function SummaryRow({ label, value }: { label: string; value: string }) {
  return (
    <View style={tw`flex-row items-center justify-between py-2`}>
      <Text fontSize={13} classN={`text-[${colors.textSecondary}]`}>
        {label}
      </Text>
      <Text fontWeight="bold" fontSize={13} classN="text-black">
        {value}
      </Text>
    </View>
  );
}

interface TaskSummarySheetProps {
  visible: boolean;
  onClose: () => void;
  onContinue: () => void;
  category: string;
  location: string;
  price: string;
  date: string;
  time: string;
}

export function TaskSummarySheet({
  visible,
  onClose,
  onContinue,
  category,
  location,
  price,
  date,
  time,
}: TaskSummarySheetProps) {
  return (
    <BottomSheet visible={visible} onClose={onClose}>
      <Text fontWeight="black" fontSize={20} classN="text-black">
        Task Summary
      </Text>

      <View style={[tw`mt-4 rounded-2xl px-4 py-1`, { backgroundColor: colors.brandTintSubtle }]}>
        <SummaryRow label="Category" value={category} />
        <SummaryRow label="Location" value={location} />
        <SummaryRow label="Price" value={price} />
        <SummaryRow label="Date" value={date} />
        <SummaryRow label="Time" value={time} />
      </View>

      <View style={tw`mt-5 gap-3`}>
        <PrimaryButton label="Continue to escrow payment" onPress={onContinue} />
        <OutlineButton label="Back" onPress={onClose} />
      </View>
    </BottomSheet>
  );
}

interface EscrowPaymentSheetProps {
  visible: boolean;
  onClose: () => void;
  onPay: () => void;
  budget: number;
  platformFee: number;
}

function formatNaira(amount: number) {
  return `₦${Math.round(amount).toLocaleString('en-US')}`;
}

export function EscrowPaymentSheet({
  visible,
  onClose,
  onPay,
  budget,
  platformFee,
}: EscrowPaymentSheetProps) {
  const total = budget + platformFee;

  return (
    <BottomSheet visible={visible} onClose={onClose}>
      <Text fontWeight="black" fontSize={20} classN="text-black">
        Fund escrow to post
      </Text>
      <Text fontSize={13} classN={`mt-1.5 text-[${colors.textSecondary}]`}>
        Held safely until you confirm the task is done. Released only on your approval.
      </Text>

      <View style={tw`mt-4`}>
        <SummaryRow label="Task Budget" value={formatNaira(budget)} />
        <SummaryRow label="Platform fee (%)" value={formatNaira(platformFee)} />
        <View
          style={[
            tw`mt-1 flex-row items-center justify-between pt-3`,
            { borderTopWidth: 1, borderTopColor: colors.borderLight },
          ]}>
          <Text fontWeight="bold" fontSize={14} classN="text-black">
            Total
          </Text>
          <Text fontWeight="bold" fontSize={14} classN="text-black">
            {formatNaira(total)}
          </Text>
        </View>
      </View>

      <View style={[tw`mt-4 rounded-xl px-3 py-2.5`, { backgroundColor: colors.brandTintSubtle }]}>
        <Text fontSize={12} classN={`text-[${colors.brandDark}]`}>
          Full refund if no Sidekick is matched within 24 hours.
        </Text>
      </View>

      <View style={tw`mt-5 gap-3`}>
        <PrimaryButton label={`Pay ${formatNaira(total)} via Paystack`} onPress={onPay} />
        <OutlineButton label="Back" onPress={onClose} />
      </View>
    </BottomSheet>
  );
}

interface PaymentSuccessSheetProps {
  visible: boolean;
  onBackHome: () => void;
}

export function PaymentSuccessSheet({ visible, onBackHome }: PaymentSuccessSheetProps) {
  return (
    <BottomSheet visible={visible} onClose={onBackHome}>
      <View style={tw`items-center px-2 py-2`}>
        <View
          style={[
            tw`h-16 w-16 items-center justify-center rounded-full`,
            { backgroundColor: colors.success },
          ]}>
          <Check size={32} color="white" strokeWidth={3} />
        </View>

        <Text fontWeight="black" fontSize={19} classN="mt-4 text-black">
          Success
        </Text>
        <Text fontSize={13} classN={`mt-1.5 text-center text-[${colors.textSecondary}]`}>
          You have successfully sent money into your Escrow Wallet
        </Text>

        <View style={tw`mt-5 w-full`}>
          <PrimaryButton label="Back Home" onPress={onBackHome} />
        </View>
      </View>
    </BottomSheet>
  );
}
