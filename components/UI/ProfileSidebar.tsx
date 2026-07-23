import { ChevronLeft, FileText, Receipt, UserRound, Users, Wallet } from 'lucide-react-native';
import React from 'react';
import { Animated, Dimensions, Modal, Pressable, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import Text from '@/components/UI/Text';
import { tw } from '@/lib/tw';

const SCREEN_WIDTH = Dimensions.get('window').width;
const PANEL_WIDTH = Math.round(SCREEN_WIDTH * 0.78);
const ACCENT_ORANGE = '#D1704F';

const PANEL_SHADOW = {
  shadowColor: '#000',
  shadowOpacity: 0.15,
  shadowRadius: 16,
  shadowOffset: { width: 8, height: 0 },
  elevation: 12,
};

const MENU_ITEMS = [
  { icon: Wallet, label: 'Wallet' },
  { icon: Receipt, label: 'Payment History' },
  { icon: FileText, label: 'Terms & Conditions' },
  { icon: Users, label: 'Refer' },
];

interface ProfileSidebarProps {
  visible: boolean;
  onClose: () => void;
}

export default function ProfileSidebar({ visible, onClose }: ProfileSidebarProps) {
  const translateX = React.useRef(new Animated.Value(-PANEL_WIDTH)).current;
  const backdropOpacity = React.useRef(new Animated.Value(0)).current;
  const [rendered, setRendered] = React.useState(visible);

  React.useEffect(() => {
    if (visible) {
      setRendered(true);
      Animated.parallel([
        Animated.timing(backdropOpacity, { toValue: 1, duration: 220, useNativeDriver: true }),
        Animated.spring(translateX, {
          toValue: 0,
          useNativeDriver: true,
          damping: 24,
          stiffness: 220,
          mass: 0.9,
        }),
      ]).start();
    } else {
      Animated.parallel([
        Animated.timing(backdropOpacity, { toValue: 0, duration: 180, useNativeDriver: true }),
        Animated.timing(translateX, { toValue: -PANEL_WIDTH, duration: 220, useNativeDriver: true }),
      ]).start(({ finished }) => {
        if (finished) setRendered(false);
      });
    }
  }, [visible, translateX, backdropOpacity]);

  if (!rendered) return null;

  return (
    <Modal transparent visible animationType="none" statusBarTranslucent onRequestClose={onClose}>
      <View style={tw`flex-1`}>
        <Pressable style={tw`absolute top-0 bottom-0 left-0 right-0`} onPress={onClose}>
          <Animated.View
            style={[tw`flex-1`, { backgroundColor: 'rgba(15,20,24,0.35)', opacity: backdropOpacity }]}
          />
        </Pressable>

        <Animated.View
          style={[
            tw`absolute bottom-0 left-0 top-0`,
            { width: PANEL_WIDTH, transform: [{ translateX }] },
          ]}>
          <SafeAreaView
            edges={['top', 'bottom']}
            style={[
              tw`flex-1 bg-white`,
              { borderTopRightRadius: 28, borderBottomRightRadius: 28 },
              PANEL_SHADOW,
            ]}>
            <Pressable onPress={onClose} hitSlop={8} style={tw`px-5 pt-3`}>
              <ChevronLeft size={22} color="#16181A" />
            </Pressable>

            <View style={tw`mt-4 flex-row items-center gap-3 px-5`}>
              <View
                style={[
                  tw`h-12 w-12 items-center justify-center rounded-full`,
                  { backgroundColor: '#F0DCC8' },
                ]}>
                <Text fontWeight="bold" fontSize={16} classN="text-[#B5762E]">
                  B
                </Text>
              </View>
              <View>
                <Text fontWeight="bold" fontSize={16} classN="text-black">
                  Bisola Soks
                </Text>
                <Text fontSize={13} classN="text-[#9AA0A6]">
                  Hero
                </Text>
              </View>
            </View>

            <View style={tw`mt-6 px-5`}>
              {MENU_ITEMS.map(({ icon: Icon, label }) => (
                <Pressable key={label} style={tw`flex-row items-center gap-3 py-3.5`}>
                  <Icon size={20} color="#4B5054" />
                  <Text fontSize={15} classN="text-[#2B2F31]">
                    {label}
                  </Text>
                </Pressable>
              ))}
            </View>

            <Pressable
              style={[
                tw`mx-5 mt-4 flex-row items-center gap-3 rounded-full py-2 pl-2 pr-5`,
                { backgroundColor: ACCENT_ORANGE },
              ]}>
              <View
                style={[
                  tw`h-9 w-9 items-center justify-center rounded-full`,
                  { backgroundColor: '#F0DCC8' },
                ]}>
                <UserRound size={18} color="#B5762E" />
              </View>
              <Text fontWeight="bold" fontSize={15} classN="text-white">
                Sidekick
              </Text>
            </Pressable>
          </SafeAreaView>
        </Animated.View>
      </View>
    </Modal>
  );
}
