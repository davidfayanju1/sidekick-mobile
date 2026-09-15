import React from 'react';
import {
  Animated,
  Dimensions,
  KeyboardAvoidingView,
  Modal,
  Platform,
  Pressable,
  View,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { tw } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';

const SCREEN_HEIGHT = Dimensions.get('window').height;

interface BottomSheetProps {
  visible: boolean;
  onClose: () => void;
  children: React.ReactNode;
}

export default function BottomSheet({ visible, onClose, children }: BottomSheetProps) {
  const { colors } = useColorScheme();
  const translateY = React.useRef(new Animated.Value(SCREEN_HEIGHT)).current;
  const backdropOpacity = React.useRef(new Animated.Value(0)).current;
  const [rendered, setRendered] = React.useState(visible);

  React.useEffect(() => {
    if (visible) {
      setRendered(true);
      Animated.parallel([
        Animated.timing(backdropOpacity, { toValue: 1, duration: 220, useNativeDriver: true }),
        Animated.spring(translateY, {
          toValue: 0,
          useNativeDriver: true,
          damping: 22,
          stiffness: 220,
          mass: 0.9,
        }),
      ]).start();
    } else {
      Animated.parallel([
        Animated.timing(backdropOpacity, { toValue: 0, duration: 180, useNativeDriver: true }),
        Animated.timing(translateY, {
          toValue: SCREEN_HEIGHT,
          duration: 200,
          useNativeDriver: true,
        }),
      ]).start(({ finished }) => {
        if (finished) setRendered(false);
      });
    }
  }, [visible, translateY, backdropOpacity]);

  if (!rendered) return null;

  return (
    <Modal transparent visible animationType="none" statusBarTranslucent onRequestClose={onClose}>
      <View style={tw`flex-1`}>
        <Pressable style={tw`absolute top-0 bottom-0 left-0 right-0`} onPress={onClose}>
          <Animated.View
            style={[
              tw`flex-1`,
              { backgroundColor: 'rgba(15,20,24,0.55)', opacity: backdropOpacity },
            ]}
          />
        </Pressable>

        <KeyboardAvoidingView
          behavior={Platform.OS === 'ios' ? 'padding' : undefined}
          style={tw`mt-auto`}
          pointerEvents="box-none">
          <Animated.View style={{ transform: [{ translateY }] }}>
            <View style={[tw`rounded-t-[28px] px-5 pt-3`, { backgroundColor: colors.card }]}>
              <View
                style={[tw`h-1 w-10 self-center rounded-full`, { backgroundColor: colors.grey4 }]}
              />
              <View style={tw`pb-2 pt-4`}>{children}</View>
            </View>
            <SafeAreaView edges={['bottom']} style={{ backgroundColor: colors.card }} />
          </Animated.View>
        </KeyboardAvoidingView>
      </View>
    </Modal>
  );
}
