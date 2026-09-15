import { Link, Stack } from 'expo-router';

import { Text, View } from 'react-native';

import { tw } from '@/lib/tw';

export default function NotFoundScreen() {
  return (
    <>
      <Stack.Screen options={{ title: 'Oops!' }} />
      <View style={tw`flex-1 items-center justify-center p-5`}>
        <Text style={tw`text-xl font-bold`}>{"This screen doesn't exist."}</Text>
        <Link href="/" style={tw`mt-4 pt-4`}>
          <Text style={tw`text-base text-[#2e78b7]`}>Go to home screen!</Text>
        </Link>
      </View>
    </>
  );
}
