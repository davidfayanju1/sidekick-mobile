import * as React from 'react';
import { View } from 'react-native';

import BottomSheet from '@/components/UI/BottomSheet';
import { Button } from '@/components/UI/Button';
import Text from '@/components/UI/Text';
import { tw, twColor } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';

export interface ConfirmSheetProps {
  visible: boolean;
  onClose: () => void;
  onConfirm: () => void;
  title: string;
  /** One or two sentences on what happens. Say the consequence, not the obvious. */
  message: string;
  /** Name the action — "Sign out" beats "OK", which makes people re-read the title. */
  confirmLabel: string;
  cancelLabel?: string;
  /** Red confirm button, for anything that loses work or access. */
  destructive?: boolean;
  /** Keeps the sheet open and spins the confirm button while the work runs. */
  loading?: boolean;
}

/**
 * The app's confirmation dialog.
 *
 * Reach for this before anything the user can't casually undo — signing out,
 * deleting an account, cancelling a funded task. It deliberately offers two
 * labelled buttons rather than a yes/no: the confirm button restates the
 * action, so a mis-tap on the sheet isn't a mis-tap on the decision.
 *
 * Dismissing by backdrop or Cancel is always the safe outcome.
 */
export function ConfirmSheet({
  visible,
  onClose,
  onConfirm,
  title,
  message,
  confirmLabel,
  cancelLabel = 'Cancel',
  destructive = false,
  loading = false,
}: ConfirmSheetProps) {
  const { colors: systemColors } = useColorScheme();

  return (
    <BottomSheet visible={visible} onClose={loading ? () => {} : onClose}>
      <Text
        variant="title"
        fontWeight="black"
        classN={`text-[${twColor(systemColors.foreground)}]`}>
        {title}
      </Text>
      <Text variant="body" classN={`mt-2 text-[${twColor(systemColors.mutedForeground)}]`}>
        {message}
      </Text>

      <View style={tw`mt-5 gap-2.5`}>
        <Button
          label={confirmLabel}
          onPress={onConfirm}
          loading={loading}
          variant={destructive ? 'danger' : 'primary'}
          size="lg"
        />
        <Button
          label={cancelLabel}
          onPress={onClose}
          disabled={loading}
          variant="outline"
          size="lg"
        />
      </View>
    </BottomSheet>
  );
}

export default ConfirmSheet;
