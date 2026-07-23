import { View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import Text from '@/components/UI/Text';
import { tw } from '@/lib/tw';

export default function Profile() {
  return (
    <SafeAreaView style={tw`flex-1 bg-white`}>
      <View style={tw`flex-1 items-center justify-center px-8`}>
        <Text fontWeight="bold" fontSize={18} classN="text-center text-black">
          Profile
        </Text>
        <Text fontSize={14} classN="mt-2 text-center text-[#9AA0A6]">
          Coming soon.
        </Text>
      </View>
    </SafeAreaView>
  );
}
