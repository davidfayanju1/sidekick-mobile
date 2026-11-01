import { Check, ClipboardList, HandCoins } from 'lucide-react-native';
import * as React from 'react';
import { View } from 'react-native';

import BottomSheet from '@/components/UI/BottomSheet';
import { Button } from '@/components/UI/Button';
import Text from '@/components/UI/Text';
import { tw, twColor } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';
import type { UserRole } from '@/store/roleStore';
import { withOpacity } from '@/theme/with-opacity';
import { colors } from '@/theme/palette';
import { PressableScale } from '@/components/UI/PressableScale';

const OPTIONS: {
  role: UserRole;
  title: string;
  description: string;
  icon: typeof ClipboardList;
}[] = [
  {
    role: 'hero',
    title: 'Hero',
    description: 'Post tasks and get help from someone nearby',
    icon: ClipboardList,
  },
  {
    role: 'sidekick',
    title: 'Sidekick',
    description: 'Browse tasks nearby and earn on your schedule',
    icon: HandCoins,
  },
];

interface RoleSelectSheetProps {
  visible: boolean;
  onSelect: (role: UserRole) => void;
  /** True while the choice is being saved to the server. */
  loading?: boolean;
}

export default function RoleSelectSheet({
  visible,
  onSelect,
  loading = false,
}: RoleSelectSheetProps) {
  const { colors: systemColors } = useColorScheme();
  const [selected, setSelected] = React.useState<UserRole | null>(null);

  const fg = twColor(systemColors.foreground);
  const muted = twColor(systemColors.mutedForeground);

  return (
    <BottomSheet visible={visible} onClose={() => {}}>
      <Text variant="title" fontWeight="black" classN={`text-[${fg}]`}>
        How will you use Sidekick?
      </Text>
      <Text variant="caption" classN={`mt-1.5 text-[${muted}]`}>
        Choose how you&rsquo;d like to start. You can switch anytime from your profile.
      </Text>

      <View style={tw`mt-5 gap-3`}>
        {OPTIONS.map(({ role, title, description, icon: Icon }) => {
          const isSelected = selected === role;
          return (
            <PressableScale
              key={role}
              onPress={() => setSelected(role)}
              style={[
                tw`flex-row items-center gap-3 rounded-2xl p-4`,
                {
                  borderWidth: isSelected ? 2 : 1,
                  borderColor: isSelected ? colors.primary : systemColors.grey5,
                  backgroundColor: isSelected
                    ? withOpacity(colors.primary, 0.12)
                    : systemColors.card,
                },
              ]}>
              <View
                style={[
                  tw`h-11 w-11 items-center justify-center rounded-xl`,
                  { backgroundColor: isSelected ? colors.primary : systemColors.grey6 },
                ]}>
                <Icon size={20} color={isSelected ? 'white' : systemColors.mutedForeground} />
              </View>
              <View style={tw`flex-1`}>
                <Text variant="body" fontWeight="medium" classN={`text-[${fg}]`}>
                  {title}
                </Text>
                <Text variant="caption" classN={`mt-0.5 text-[${muted}]`}>
                  {description}
                </Text>
              </View>
              <View
                style={[
                  tw`h-5 w-5 items-center justify-center rounded-full`,
                  {
                    borderWidth: 1.5,
                    borderColor: isSelected ? colors.primary : systemColors.grey3,
                    backgroundColor: isSelected ? colors.primary : 'transparent',
                  },
                ]}>
                {isSelected && <Check size={12} color="white" strokeWidth={3} />}
              </View>
            </PressableScale>
          );
        })}
      </View>

      <Button
        label="Continue"
        onPress={() => selected && onSelect(selected)}
        disabled={!selected}
        loading={loading}
        size="lg"
        style={tw`mt-6`}
      />
    </BottomSheet>
  );
}
