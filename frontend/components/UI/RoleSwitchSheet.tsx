import { Pressable, View } from 'react-native';

import BottomSheet from '@/components/UI/BottomSheet';
import Text from '@/components/UI/Text';
import { tw, twColor } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';
import type { UserRole } from '@/store/roleStore';
import { colors } from '@/theme/palette';

const ROLE_LABEL: Record<UserRole, string> = { hero: 'Hero', sidekick: 'Sidekick' };

interface RoleSwitchSheetProps {
  visible: boolean;
  onClose: () => void;
  onConfirm: () => void;
  currentRole: UserRole;
  nextRole: UserRole;
}

export default function RoleSwitchSheet({
  visible,
  onClose,
  onConfirm,
  currentRole,
  nextRole,
}: RoleSwitchSheetProps) {
  const { colors: systemColors } = useColorScheme();

  return (
    <BottomSheet visible={visible} onClose={onClose}>
      <Text fontWeight="black" fontSize={20} classN={`text-[${twColor(systemColors.foreground)}]`}>
        Switch to {ROLE_LABEL[nextRole]}?
      </Text>
      <Text fontSize={13} classN={`mt-2 text-[${twColor(systemColors.mutedForeground)}]`}>
        You&rsquo;re about to switch from {ROLE_LABEL[currentRole]} to {ROLE_LABEL[nextRole]}. Your
        home screen, tasks, and available actions will change to match the {ROLE_LABEL[nextRole]}{' '}
        experience. Anything you&rsquo;ve already set up for either role is kept, and you can switch
        back at any time from your profile.
      </Text>

      <View style={tw`mt-5 gap-3`}>
        <Pressable
          onPress={onConfirm}
          style={[
            tw`items-center justify-center rounded-full py-4`,
            { backgroundColor: colors.brand },
          ]}>
          <Text fontWeight="bold" fontSize={15} classN="text-white">
            Switch to {ROLE_LABEL[nextRole]}
          </Text>
        </Pressable>
        <Pressable
          onPress={onClose}
          style={[
            tw`items-center justify-center rounded-full py-4`,
            { borderWidth: 1, borderColor: systemColors.grey4 },
          ]}>
          <Text
            fontWeight="bold"
            fontSize={15}
            classN={`text-[${twColor(systemColors.foreground)}]`}>
            Cancel
          </Text>
        </Pressable>
      </View>
    </BottomSheet>
  );
}
