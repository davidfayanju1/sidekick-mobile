import { View } from 'react-native';

import BottomSheet from '@/components/UI/BottomSheet';
import { Button } from '@/components/UI/Button';
import Text from '@/components/UI/Text';
import { tw, twColor } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';
import type { UserRole } from '@/store/roleStore';
import { colors } from '@/theme/palette';
import { PressableScale } from '@/components/UI/PressableScale';

const ROLE_LABEL: Record<UserRole, string> = { hero: 'Hero', sidekick: 'Sidekick' };

interface RoleSwitchSheetProps {
  visible: boolean;
  onClose: () => void;
  onConfirm: () => void;
  currentRole: UserRole;
  nextRole: UserRole;
  /** True while `POST /me/role` is in flight. */
  loading?: boolean;
}

export default function RoleSwitchSheet({
  visible,
  onClose,
  onConfirm,
  currentRole,
  nextRole,
  loading = false,
}: RoleSwitchSheetProps) {
  const { colors: systemColors } = useColorScheme();

  return (
    // Not dismissable mid-save: the role only changes once the server agrees.
    <BottomSheet visible={visible} onClose={loading ? () => {} : onClose}>
      <Text
        variant="title"
        fontWeight="black"
        classN={`text-[${twColor(systemColors.foreground)}]`}>
        Switch to {ROLE_LABEL[nextRole]}?
      </Text>
      <Text variant="caption" classN={`mt-2 text-[${twColor(systemColors.mutedForeground)}]`}>
        You&rsquo;re about to switch from {ROLE_LABEL[currentRole]} to {ROLE_LABEL[nextRole]}. Your
        home screen, tasks, and available actions will change to match the {ROLE_LABEL[nextRole]}{' '}
        experience. Anything you&rsquo;ve already set up for either role is kept, and you can switch
        back at any time from your profile.
      </Text>

      <View style={tw`mt-5 gap-2.5`}>
        <Button
          label={`Switch to ${ROLE_LABEL[nextRole]}`}
          onPress={onConfirm}
          loading={loading}
          size="lg"
        />
        <Button label="Cancel" onPress={onClose} disabled={loading} variant="outline" size="lg" />
      </View>
    </BottomSheet>
  );
}
