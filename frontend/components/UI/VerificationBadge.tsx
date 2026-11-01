import { BadgeCheck, Clock, ShieldAlert, ShieldOff } from 'lucide-react-native';
import * as React from 'react';
import { View } from 'react-native';

import { PressableScale } from '@/components/UI/PressableScale';
import Text from '@/components/UI/Text';
import type { VerificationStatus } from '@/lib/api/schemas';
import { tw } from '@/lib/tw';
import { colors } from '@/theme/palette';

interface Treatment {
  label: string;
  /** Shown under the badge where there's room to explain. */
  detail: string;
  icon: typeof BadgeCheck;
  fg: string;
  bg: string;
  /** Whether tapping should take the user somewhere to act. */
  actionable: boolean;
}

/**
 * The four states of `users.verification_status`.
 *
 * `unverified` means no document has been submitted at all — it's the default,
 * not a failure, so it's styled neutrally rather than in red. Only `rejected`
 * earns alarm.
 */
const TREATMENTS: Record<VerificationStatus, Treatment> = {
  verified: {
    label: 'Verified',
    detail: 'Your ID has been checked.',
    icon: BadgeCheck,
    fg: colors.green,
    bg: colors.greenTint,
    actionable: false,
  },
  pending: {
    label: 'Verification pending',
    detail: 'We’re reviewing your ID — usually within a day.',
    icon: Clock,
    fg: colors.orangeDark,
    bg: colors.orangeTint,
    actionable: false,
  },
  rejected: {
    label: 'Verification failed',
    detail: 'We couldn’t read your document. Tap to try again.',
    icon: ShieldAlert,
    fg: colors.danger,
    bg: colors.dangerTint,
    actionable: true,
  },
  unverified: {
    label: 'Not verified',
    detail: 'Verify your ID to build trust and withdraw earnings.',
    icon: ShieldOff,
    fg: colors.gray700,
    bg: colors.gray100,
    actionable: true,
  },
};

export interface VerificationBadgeProps {
  status: VerificationStatus;
  /** Called when the status is one the user can do something about. */
  onPress?: () => void;
  /** `pill` sits next to a name; `row` is a full-width line with an explanation. */
  variant?: 'pill' | 'row';
}

/**
 * Shows where a user stands on ID verification.
 *
 * This is the signal other people judge them by — the backend denormalises it
 * onto `users.verification_status` precisely so it can be read without a join
 * — so it earns a place next to the name rather than buried in settings.
 *
 * Note it is *not* the same as `phone_verified`. A verified phone proves the
 * number is reachable; this proves who they are, and only this one gates
 * withdrawals (FR-9.3).
 */
export function VerificationBadge({ status, onPress, variant = 'pill' }: VerificationBadgeProps) {
  const treatment = TREATMENTS[status] ?? TREATMENTS.unverified;
  const Icon = treatment.icon;
  const interactive = treatment.actionable && !!onPress;

  if (variant === 'pill') {
    const pill = (
      <View
        style={[
          tw`flex-row items-center gap-1 self-start rounded-full px-2 py-0.5`,
          { backgroundColor: treatment.bg },
        ]}>
        <Icon size={11} color={treatment.fg} />
        <Text variant="micro" fontWeight="medium" classN={`text-[${treatment.fg}]`}>
          {treatment.label}
        </Text>
      </View>
    );

    // Only wrap in a pressable when there's somewhere to go — a badge that
    // depresses and does nothing reads as broken.
    return interactive ? (
      <PressableScale onPress={onPress} hitSlop={6}>
        {pill}
      </PressableScale>
    ) : (
      pill
    );
  }

  const row = (
    <View
      style={[
        tw`flex-row items-center gap-3 rounded-2xl p-3.5`,
        { backgroundColor: treatment.bg },
      ]}>
      <Icon size={18} color={treatment.fg} />
      <View style={tw`flex-1`}>
        <Text variant="caption" fontWeight="bold" classN={`text-[${treatment.fg}]`}>
          {treatment.label}
        </Text>
        <Text variant="micro" fontWeight="normal" classN={`mt-0.5 text-[${treatment.fg}]`}>
          {treatment.detail}
        </Text>
      </View>
    </View>
  );

  return interactive ? <PressableScale onPress={onPress}>{row}</PressableScale> : row;
}

export default VerificationBadge;
