import { BadgeCheck, Check, Image as ImageIcon, Star } from 'lucide-react-native';
import React from 'react';
import { ScrollView, TextInput, View } from 'react-native';

import BottomSheet from '@/components/UI/BottomSheet';
import { Button } from '@/components/UI/Button';
import { EmptyState, ErrorState } from '@/components/UI/EmptyState';
import { AwaitingOffers } from '@/components/UI/illustrations';
import { Skeleton } from '@/components/UI/Skeleton';
import Text from '@/components/UI/Text';
import { useAcceptOffer, useCounterOffer, useDeclineOffer, useTaskOffers } from '@/lib/api/offers';
import type { OfferWithWorker } from '@/lib/api/schemas';
import { errorMessage } from '@/lib/axios';
import { money } from '@/lib/format';
import { counterSchema } from '@/lib/forms/schemas';
import { locale } from '@/lib/locale';
import { tw } from '@/lib/tw';
import { toast } from '@/store/toastStore';
import { colors } from '@/theme/palette';
import { PressableScale } from '@/components/UI/PressableScale';

function PrimaryButton({
  label,
  onPress,
  loading,
  disabled,
}: {
  label: string;
  onPress?: () => void;
  loading?: boolean;
  disabled?: boolean;
}) {
  return <Button label={label} onPress={onPress} loading={loading} disabled={disabled} size="lg" />;
}

function OutlineButton({
  label,
  onPress,
  color = colors.gray900,
}: {
  label: string;
  onPress?: () => void;
  color?: string;
}) {
  return (
    <PressableScale
      onPress={onPress}
      style={[
        tw`items-center justify-center rounded-full py-4`,
        { borderWidth: 1, borderColor: colors.gray200, backgroundColor: 'white' },
      ]}>
      <Text variant="body" fontWeight="medium" classN={`text-[${color}]`}>
        {label}
      </Text>
    </PressableScale>
  );
}

interface ConfirmTaskSheetProps {
  visible: boolean;
  onClose: () => void;
  onReportDispute: () => void;
  /** Releases escrow to the Sidekick. Irreversible, so it confirms here first. */
  onConfirm: () => void;
  confirming?: boolean;
  sidekickName: string;
  price: string;
  photoCount?: number;
}

export function ConfirmTaskSheet({
  visible,
  onClose,
  onReportDispute,
  onConfirm,
  confirming = false,
  sidekickName,
  price,
  photoCount = 2,
}: ConfirmTaskSheetProps) {
  return (
    <BottomSheet visible={visible} onClose={onClose}>
      <Text variant="title" fontWeight="black" classN="text-black">
        Confirm Task done?
      </Text>
      <Text variant="caption" classN={`mt-1.5 text-[${colors.gray700}]`}>
        {sidekickName} has marked this complete. Confirm to release payment.
      </Text>

      <PressableScale
        style={[
          tw`mt-3 flex-row items-center gap-3 rounded-2xl p-3`,
          { borderWidth: 1, borderColor: colors.gray200 },
        ]}>
        <View
          style={[
            tw`h-11 w-11 items-center justify-center rounded-xl`,
            { backgroundColor: colors.gray50 },
          ]}>
          <ImageIcon size={20} color={colors.gray700} />
        </View>
        <View style={tw`flex-1`}>
          <Text variant="caption" fontWeight="medium" classN="text-black">
            {photoCount} photos attached as proof
          </Text>
          <Text variant="micro" fontWeight="normal" classN={`mt-0.5 text-[${colors.gray500}]`}>
            Sent by {sidekickName.split(' ')[0]} • Tap to view
          </Text>
        </View>
      </PressableScale>

      <View style={tw`mt-5 gap-3`}>
        <PrimaryButton
          label={`Confirm and pay ${price}`}
          onPress={onConfirm}
          loading={confirming}
        />
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
  onSubmit: (reason: string, description: string) => void;
  submitting?: boolean;
}

export function DisputeSheet({
  visible,
  onClose,
  onSubmit,
  submitting = false,
}: DisputeSheetProps) {
  const [selected, setSelected] = React.useState<string>(DISPUTE_REASONS[0].id);
  const [note, setNote] = React.useState('');

  return (
    <BottomSheet visible={visible} onClose={onClose}>
      <Text variant="title" fontWeight="black" classN="text-black">
        Raise a Dispute
      </Text>
      <Text variant="caption" classN={`mt-1.5 text-[${colors.gray700}]`}>
        Select the issue. Our team reviews within 24h.
      </Text>

      <View style={tw`mt-1`}>
        {DISPUTE_REASONS.map((reason) => {
          const isSelected = selected === reason.id;
          return (
            <PressableScale
              key={reason.id}
              onPress={() => setSelected(reason.id)}
              style={[
                tw`mt-2 flex-row items-start gap-3 rounded-2xl p-4`,
                { backgroundColor: colors.gray50 },
              ]}>
              <View
                style={[
                  tw`mt-0.5 h-5 w-5 items-center justify-center rounded-full`,
                  {
                    borderWidth: 2,
                    borderColor: isSelected ? colors.gray900 : colors.gray300,
                  },
                ]}>
                {isSelected && <View style={tw`h-2.5 w-2.5 rounded-full bg-[${colors.gray900}]`} />}
              </View>
              <View style={tw`flex-1`}>
                <Text variant="body" fontWeight="medium" classN="text-black">
                  {reason.title}
                </Text>
                <Text variant="caption" classN={`mt-0.5 text-[${colors.gray500}]`}>
                  {reason.description}
                </Text>
              </View>
            </PressableScale>
          );
        })}
      </View>

      <Text variant="caption" fontWeight="medium" classN="mt-3 text-black">
        Additional Information
      </Text>
      <TextInput
        multiline
        value={note}
        onChangeText={setNote}
        placeholder=""
        placeholderTextColor={colors.gray500}
        style={[
          tw`mt-2 rounded-2xl p-4 text-black`,
          {
            borderWidth: 1,
            borderColor: colors.gray200,
            minHeight: 96,
            textAlignVertical: 'top',
          },
        ]}
      />

      <View style={tw`mt-5 gap-3`}>
        <Button
          label="Submit Dispute"
          variant="danger"
          size="lg"
          loading={submitting}
          // The API needs at least 20 characters of description; the reason
          // title stands in when the user adds nothing of their own.
          onPress={() =>
            onSubmit(
              selected,
              note.trim() ||
                DISPUTE_REASONS.find((reason) => reason.id === selected)?.description ||
                ''
            )
          }
        />
        <OutlineButton label="Cancel" onPress={onClose} />
      </View>
    </BottomSheet>
  );
}

/**
 * An offer as this sheet draws it, built from `OfferWithWorker`.
 *
 * The API sends the worker's reputation alongside the offer precisely so this
 * card can be rendered without a second request per row.
 */
function toOfferCard(offer: OfferWithWorker) {
  const name = offer.worker_display_name ?? 'Sidekick';
  return {
    id: offer.id,
    name,
    initial: name.charAt(0).toUpperCase(),
    rating: Math.round(offer.worker_rating_avg ?? 0),
    tasksCompleted: offer.worker_tasks_completed ?? 0,
    price: money(offer.amount),
    // A counter is an offer at a price the poster didn't set.
    priceLabel: offer.is_counter ? 'Counter-offer' : 'At your budget',
    reason: offer.message ?? undefined,
    isCounter: !!offer.is_counter,
  };
}

type OfferCardModel = ReturnType<typeof toOfferCard>;

/** Avatar tints, derived from the offer id so a card keeps its colour. */
const OFFER_TINTS = [
  { bg: colors.orangeTint, text: colors.orangeDark },
  { bg: colors.primaryTint, text: colors.primary },
  { bg: colors.greenTint, text: colors.green },
] as const;

function offerTint(id: string) {
  const sum = [...id].reduce((total, character) => total + character.charCodeAt(0), 0);
  return OFFER_TINTS[sum % OFFER_TINTS.length];
}

function StarRating({ rating }: { rating: number }) {
  return (
    <View style={tw`flex-row items-center gap-0.5`}>
      {Array.from({ length: 5 }).map((_, index) => (
        <Star
          key={index}
          size={10}
          color={index < rating ? colors.star : colors.gray200}
          fill={index < rating ? colors.star : colors.gray200}
        />
      ))}
    </View>
  );
}

function OfferCard({
  offer,
  onAccept,
  onCounter,
  onDecline,
  busy,
}: {
  offer: OfferCardModel;
  onAccept: () => void;
  onCounter: () => void;
  onDecline: () => void;
  busy: boolean;
}) {
  return (
    <View style={[tw`mt-3 rounded-2xl p-4`, { borderWidth: 1, borderColor: colors.gray100 }]}>
      <View style={tw`flex-row items-start justify-between`}>
        <View style={tw`flex-1 flex-row items-center gap-2.5`}>
          <View
            style={[
              tw`h-10 w-10 items-center justify-center rounded-full`,
              { backgroundColor: offerTint(offer.id).bg },
            ]}>
            <Text variant="body" fontWeight="medium" classN={`text-[${offerTint(offer.id).text}]`}>
              {offer.initial}
            </Text>
          </View>
          <View style={tw`flex-1`}>
            <View style={tw`flex-row items-center gap-1`}>
              <Text variant="body" fontWeight="medium" classN="text-black">
                {offer.name}
              </Text>
              <BadgeCheck size={13} color={colors.primary} fill={colors.primary} />
            </View>
            <View style={tw`mt-1 flex-row items-center gap-1.5`}>
              <StarRating rating={offer.rating} />
              <Text variant="micro" fontWeight="normal" classN={`text-[${colors.gray500}]`}>
                ({offer.tasksCompleted} Tasks)
              </Text>
            </View>
          </View>
        </View>
        <View style={tw`items-end`}>
          <Text variant="body" fontWeight="medium" classN="text-black">
            {offer.price}
          </Text>
          <Text variant="micro" fontWeight="normal" classN={`mt-0.5 text-[${colors.gray500}]`}>
            {offer.priceLabel}
          </Text>
        </View>
      </View>

      {offer.reason && (
        <View style={tw`mt-2`}>
          <Text variant="micro" classN="text-black">
            {offer.isCounter ? 'Reasons for counter offer' : 'Their pitch'}
          </Text>
          <View style={[tw`mt-1.5 rounded-xl px-3 py-2.5`, { backgroundColor: colors.gray50 }]}>
            <Text variant="caption" classN={`italic text-[${colors.gray700}]`}>
              &ldquo;{offer.reason}&rdquo;
            </Text>
          </View>
        </View>
      )}

      <View style={tw`mt-2 flex-row gap-2`}>
        <Button
          label="Accept"
          variant="outline"
          size="sm"
          onPress={onAccept}
          disabled={busy}
          fullWidth={false}
          style={tw`flex-1`}
        />
        <Button
          label="Counter"
          size="sm"
          onPress={onCounter}
          disabled={busy}
          fullWidth={false}
          style={tw`flex-1`}
        />
        <Button
          label="Decline"
          variant="outline"
          size="sm"
          onPress={onDecline}
          disabled={busy}
          fullWidth={false}
          style={tw`flex-1`}
        />
      </View>
    </View>
  );
}

interface ReviewOffersSheetProps {
  visible: boolean;
  onClose: () => void;
  taskTitle: string;
  taskId: string;
}

/**
 * The poster's decision point: accept an offer, counter it once, or decline.
 *
 * Accepting is the hinge of the whole marketplace — it assigns the task and
 * opens the chat — so it asks for confirmation and closes the sheet on success
 * rather than leaving a stale list on screen.
 */
export function ReviewOffersSheet({ visible, onClose, taskTitle, taskId }: ReviewOffersSheetProps) {
  const offers = useTaskOffers(visible ? taskId : undefined);
  const acceptOffer = useAcceptOffer(taskId);
  const declineOffer = useDeclineOffer(taskId);
  const counterOffer = useCounterOffer(taskId);

  const [counteringId, setCounteringId] = React.useState<string | null>(null);
  const [counterAmount, setCounterAmount] = React.useState('');

  const pending = (offers.data ?? []).filter((offer) => offer.status === 'pending');
  const busy = acceptOffer.isPending || declineOffer.isPending || counterOffer.isPending;

  const handleAccept = async (offerId: string) => {
    try {
      await acceptOffer.mutateAsync(offerId);
      toast.success('Offer accepted', {
        description: 'Your escrow is held and a chat is now open with your Sidekick.',
      });
      onClose();
    } catch (error) {
      toast.error('Couldn’t accept that offer', { description: errorMessage(error) });
    }
  };

  const handleDecline = async (offerId: string) => {
    try {
      await declineOffer.mutateAsync({ offerId });
      toast.info('Offer declined');
    } catch (error) {
      toast.error('Couldn’t decline that offer', { description: errorMessage(error) });
    }
  };

  const handleCounter = async (offerId: string) => {
    const parsed = counterSchema.safeParse({ amount: counterAmount });
    if (!parsed.success) {
      toast.warning(parsed.error.issues[0]?.message ?? 'Enter an amount');
      return;
    }
    try {
      await counterOffer.mutateAsync({ offerId, amount: parsed.data.amount });
      setCounteringId(null);
      setCounterAmount('');
      toast.success('Counter sent', { description: 'You get one counter per offer.' });
    } catch (error) {
      toast.error('Couldn’t send that counter', { description: errorMessage(error) });
    }
  };

  return (
    <BottomSheet visible={visible} onClose={onClose}>
      <Text variant="title" fontWeight="black" classN="text-black">
        Review offers
      </Text>
      <Text variant="caption" classN={`mt-1.5 text-[${colors.gray700}]`}>
        {pending.length === 0
          ? `No open offers on “${taskTitle}” yet`
          : `${pending.length} Sidekick${pending.length === 1 ? '' : 's'} offered on “${taskTitle}”`}
      </Text>

      <ScrollView
        style={{ maxHeight: 420 }}
        showsVerticalScrollIndicator={false}
        contentContainerStyle={tw`pb-1`}>
        {offers.isPending ? (
          <View style={tw`mt-3 gap-3`}>
            {[0, 1].map((index) => (
              <Skeleton key={index} index={index} height={150} radius={16} />
            ))}
          </View>
        ) : offers.isError ? (
          <ErrorState compact body={errorMessage(offers.error)} onRetry={() => offers.refetch()} />
        ) : pending.length === 0 ? (
          <EmptyState
            compact
            illustration={<AwaitingOffers />}
            title="No offers yet"
            body="Sidekicks nearby can see your task. Offers usually arrive within the hour."
          />
        ) : (
          pending.map((offer) => {
            const card = toOfferCard(offer);
            return (
              <View key={card.id}>
                <OfferCard
                  offer={card}
                  busy={busy}
                  onAccept={() => handleAccept(card.id)}
                  onDecline={() => handleDecline(card.id)}
                  onCounter={() =>
                    setCounteringId((current) => (current === card.id ? null : card.id))
                  }
                />
                {counteringId === card.id && (
                  <View style={[tw`mt-2 rounded-2xl p-3`, { backgroundColor: colors.gray50 }]}>
                    <Text variant="micro" fontWeight="medium" classN="text-black">
                      Your counter
                    </Text>
                    <TextInput
                      value={counterAmount}
                      onChangeText={setCounterAmount}
                      keyboardType="number-pad"
                      placeholder={locale.currency}
                      placeholderTextColor={colors.gray500}
                      style={[
                        tw`mt-2 rounded-full px-4 text-black`,
                        { height: 44, borderWidth: 1, borderColor: colors.gray300 },
                      ]}
                    />
                    <View style={tw`mt-2`}>
                      <Button
                        label="Send counter"
                        size="sm"
                        loading={counterOffer.isPending}
                        onPress={() => handleCounter(card.id)}
                      />
                    </View>
                  </View>
                )}
              </View>
            );
          })
        )}
      </ScrollView>

      <View style={tw`mt-3`}>
        <OutlineButton label="Close" onPress={onClose} />
      </View>
    </BottomSheet>
  );
}

function SummaryRow({ label, value }: { label: string; value: string }) {
  return (
    <View style={tw`flex-row items-center justify-between py-2`}>
      <Text variant="caption" classN={`text-[${colors.gray700}]`}>
        {label}
      </Text>
      <Text variant="caption" fontWeight="medium" classN="text-black">
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
      <Text variant="title" fontWeight="black" classN="text-black">
        Task Summary
      </Text>

      <View style={[tw`mt-3 rounded-2xl px-4 py-1`, { backgroundColor: colors.primarySurface }]}>
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
  /** Minor units, as the API stores them. */
  budget: number;
  platformFee: number;
  /** True while the task is being created and escrow funded. */
  paying?: boolean;
}

export function EscrowPaymentSheet({
  visible,
  onClose,
  onPay,
  budget,
  platformFee,
  paying = false,
}: EscrowPaymentSheetProps) {
  const total = budget + platformFee;

  return (
    <BottomSheet visible={visible} onClose={onClose}>
      <Text variant="title" fontWeight="black" classN="text-black">
        Fund escrow to post
      </Text>
      <Text variant="caption" classN={`mt-1.5 text-[${colors.gray700}]`}>
        Held safely until you confirm the task is done. Released only on your approval.
      </Text>

      <View style={tw`mt-3`}>
        <SummaryRow label="Task Budget" value={money(budget)} />
        <SummaryRow label="Platform fee" value={money(platformFee)} />
        <View
          style={[
            tw`mt-1 flex-row items-center justify-between pt-3`,
            { borderTopWidth: 1, borderTopColor: colors.gray200 },
          ]}>
          <Text variant="body" fontWeight="medium" classN="text-black">
            Total
          </Text>
          <Text variant="body" fontWeight="medium" classN="text-black">
            {money(total)}
          </Text>
        </View>
      </View>

      <View style={[tw`mt-3 rounded-xl px-3 py-2.5`, { backgroundColor: colors.primarySurface }]}>
        <Text variant="caption" classN={`text-[${colors.primaryDark}]`}>
          Full refund if no Sidekick is matched within 24 hours.
        </Text>
      </View>

      <View style={tw`mt-5 gap-3`}>
        <PrimaryButton
          label={paying ? 'Securing your payment' : `Pay ${money(total)}`}
          onPress={onPay}
          loading={paying}
        />
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
            { backgroundColor: colors.green },
          ]}>
          <Check size={32} color="white" strokeWidth={3} />
        </View>

        <Text variant="subhead" fontWeight="black" classN="mt-3 text-black">
          Success
        </Text>
        <Text variant="caption" classN={`mt-1.5 text-center text-[${colors.gray700}]`}>
          You have successfully sent money into your Escrow Wallet
        </Text>

        <View style={tw`mt-5 w-full`}>
          <PrimaryButton label="Back Home" onPress={onBackHome} />
        </View>
      </View>
    </BottomSheet>
  );
}
