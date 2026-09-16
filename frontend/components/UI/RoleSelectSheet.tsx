import { Check, ClipboardList, HandCoins } from 'lucide-react-native';
import * as React from 'react';
import { Pressable, View } from 'react-native';

import BottomSheet from '@/components/UI/BottomSheet';
import Text from '@/components/UI/Text';
import { tw, twColor } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';
import type { UserRole } from '@/store/roleStore';
import { withOpacity } from '@/theme/with-opacity';
import { colors } from '@/theme/palette';

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
}

export default function RoleSelectSheet({ visible, onSelect }: RoleSelectSheetProps) {
  const { colors: systemColors } = useColorScheme();
  const [selected, setSelected] = React.useState<UserRole | null>(null);

  const fg = twColor(systemColors.foreground);
  const muted = twColor(systemColors.mutedForeground);

  return (
    <BottomSheet visible={visible} onClose={() => {}}>
      <Text fontWeight="black" fontSize={20} classN={`text-[${fg}]`}>
        How will you use Sidekick?
      </Text>
      <Text fontSize={13} classN={`mt-1.5 text-[${muted}]`}>
        Choose how you&rsquo;d like to start. You can switch anytime from your profile.
      </Text>

      <View style={tw`mt-5 gap-3`}>
        {OPTIONS.map(({ role, title, description, icon: Icon }) => {
          const isSelected = selected === role;
          return (
            <Pressable
              key={role}
              onPress={() => setSelected(role)}
              style={[
                tw`flex-row items-center gap-3 rounded-2xl p-4`,
                {
                  borderWidth: isSelected ? 2 : 1,
                  borderColor: isSelected ? colors.brand : systemColors.grey5,
                  backgroundColor: isSelected ? withOpacity(colors.brand, 0.12) : systemColors.card,
                },
              ]}>
              <View
                style={[
                  tw`h-11 w-11 items-center justify-center rounded-xl`,
                  { backgroundColor: isSelected ? colors.brand : systemColors.grey6 },
                ]}>
                <Icon size={20} color={isSelected ? 'white' : systemColors.mutedForeground} />
              </View>
              <View style={tw`flex-1`}>
                <Text fontWeight="bold" fontSize={15} classN={`text-[${fg}]`}>
                  {title}
                </Text>
                <Text fontSize={12} classN={`mt-0.5 text-[${muted}]`}>
                  {description}
                </Text>
              </View>
              <View
                style={[
                  tw`h-5 w-5 items-center justify-center rounded-full`,
                  {
                    borderWidth: 1.5,
                    borderColor: isSelected ? colors.brand : systemColors.grey3,
                    backgroundColor: isSelected ? colors.brand : 'transparent',
                  },
                ]}>
                {isSelected && <Check size={12} color="white" strokeWidth={3} />}
              </View>
            </Pressable>
          );
        })}
      </View>

      <Pressable
        disabled={!selected}
        onPress={() => selected && onSelect(selected)}
        style={[
          tw`mt-6 items-center justify-center rounded-full py-4`,
          { backgroundColor: colors.brand, opacity: selected ? 1 : 0.5 },
        ]}>
        <Text fontWeight="bold" fontSize={15} classN="text-white">
          Continue
        </Text>
      </Pressable>
    </BottomSheet>
  );
}
