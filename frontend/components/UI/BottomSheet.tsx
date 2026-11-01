import React from 'react';
import { Animated, Dimensions, KeyboardAvoidingView, Modal, Platform, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { tw } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';
import { PressableScale } from '@/components/UI/PressableScale';

const SCREEN_HEIGHT = Dimensions.get('window').height;

interface BottomSheetProps {
  visible: boolean;
  onClose: () => void;
  children: React.ReactNode;
}

export default function BottomSheet({ visible, onClose, children }: BottomSheetProps) {
  const { colors: systemColors } = useColorScheme();
  // `useState` with an initialiser, not `useRef(...).current`: reading a ref
  // during render is what the React Compiler rules forbid, and this gives the
  // same "create once, keep forever" behaviour without the violation.
  const [translateY] = React.useState(() => new Animated.Value(SCREEN_HEIGHT));
  const [backdropOpacity] = React.useState(() => new Animated.Value(0));
  const [rendered, setRendered] = React.useState(visible);

  // Mount during render rather than in an effect. Setting state inside the
  // effect would queue a second render pass before the sheet ever paints, so
  // the entrance animation could start against a stale frame.
  if (visible && !rendered) setRendered(true);

  React.useEffect(() => {
    if (visible) {
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
        <PressableScale style={tw`absolute top-0 bottom-0 left-0 right-0`} onPress={onClose}>
          <Animated.View
            style={[
              tw`flex-1`,
              { backgroundColor: 'rgba(15,20,24,0.55)', opacity: backdropOpacity },
            ]}
          />
        </PressableScale>

        <KeyboardAvoidingView
          behavior={Platform.OS === 'ios' ? 'padding' : undefined}
          style={tw`mt-auto`}
          pointerEvents="box-none">
          <Animated.View style={{ transform: [{ translateY }] }}>
            <View style={[tw`rounded-t-[28px] px-5 pt-3`, { backgroundColor: systemColors.card }]}>
              <View
                style={[
                  tw`h-1 w-10 self-center rounded-full`,
                  { backgroundColor: systemColors.grey4 },
                ]}
              />
              <View style={tw`pb-2 pt-4`}>{children}</View>
            </View>
            <SafeAreaView edges={['bottom']} style={{ backgroundColor: systemColors.card }} />
          </Animated.View>
        </KeyboardAvoidingView>
      </View>
    </Modal>
  );
}
