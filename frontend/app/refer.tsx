import { router } from 'expo-router';
import { ChevronLeft, Copy, Gift, Users } from 'lucide-react-native';
import * as React from 'react';
import { Pressable, ScrollView, Share, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import Text from '@/components/UI/Text';
import { CARD_SHADOW } from '@/components/UI/TaskParts';
import { tw, twColor } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';
import { withOpacity } from '@/theme/with-opacity';

const ACCENT_TEAL = '#489A9F';
const REFERRAL_CODE = 'BISOLA200';

const REWARDS = [
  { title: 'You get ₦1,000', description: 'Credited once your friend completes their first task' },
  { title: 'They get ₦500 off', description: 'Applied automatically to their first posted task' },
];

export default function ReferAFriend() {
  const { colors } = useColorScheme();
  const fg = twColor(colors.foreground);
  const muted = twColor(colors.mutedForeground);
  const [copied, setCopied] = React.useState(false);

  const handleCopy = () => {
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleShare = () => {
    Share.share({
      message: `Join me on Sidekick! Use my code ${REFERRAL_CODE} to get ₦500 off your first task.`,
    });
  };

  return (
    <SafeAreaView
      style={[tw`flex-1`, { backgroundColor: colors.background }]}
      edges={['top', 'bottom']}>
      <View style={tw`flex-row items-center gap-3 px-4 pb-3 pt-2`}>
        <Pressable onPress={() => router.back()} hitSlop={8}>
          <ChevronLeft size={24} color={colors.foreground} />
        </Pressable>
        <Text fontWeight="bold" fontSize={16} classN={`text-[${fg}]`}>
          Refer a Friend
        </Text>
      </View>

      <ScrollView
        style={tw`flex-1 px-4`}
        contentContainerStyle={{ paddingBottom: 40 }}
        showsVerticalScrollIndicator={false}>
        <View style={tw`mt-4 items-center`}>
          <View
            style={[
              tw`h-16 w-16 items-center justify-center rounded-full`,
              { backgroundColor: withOpacity(ACCENT_TEAL, 0.12) },
            ]}>
            <Gift size={28} color={ACCENT_TEAL} />
          </View>
          <Text fontWeight="black" fontSize={20} classN={`mt-4 text-center text-[${fg}]`}>
            Invite friends, earn rewards
          </Text>
          <Text fontSize={13} classN={`mt-1.5 text-center text-[${muted}]`}>
            Share your code — you both get rewarded when they complete their first task.
          </Text>
        </View>

        <View
          style={[
            tw`mt-6 flex-row items-center justify-between rounded-2xl p-4`,
            { backgroundColor: colors.card, borderWidth: 1, borderColor: colors.grey5 },
          ]}>
          <View>
            <Text fontSize={11} classN={`text-[${muted}]`}>
              Your referral code
            </Text>
            <Text fontWeight="black" fontSize={20} classN={`mt-1 text-[${twColor(ACCENT_TEAL)}]`}>
              {REFERRAL_CODE}
            </Text>
          </View>
          <Pressable
            onPress={handleCopy}
            style={[
              tw`flex-row items-center gap-1.5 rounded-full px-3.5 py-2`,
              { backgroundColor: withOpacity(ACCENT_TEAL, 0.12) },
            ]}>
            <Copy size={14} color={ACCENT_TEAL} />
            <Text fontWeight="medium" fontSize={12} classN={`text-[${twColor(ACCENT_TEAL)}]`}>
              {copied ? 'Copied!' : 'Copy'}
            </Text>
          </Pressable>
        </View>

        <Pressable
          onPress={handleShare}
          style={[
            tw`mt-3 items-center justify-center rounded-full py-4`,
            { backgroundColor: ACCENT_TEAL },
          ]}>
          <Text fontWeight="bold" fontSize={15} classN="text-white">
            Share invite link
          </Text>
        </Pressable>

        <View
          style={[
            tw`mt-6 flex-row items-center gap-3 rounded-2xl p-3.5`,
            { backgroundColor: colors.card },
            CARD_SHADOW,
          ]}>
          <View
            style={[
              tw`h-10 w-10 items-center justify-center rounded-full`,
              { backgroundColor: withOpacity(ACCENT_TEAL, 0.12) },
            ]}>
            <Users size={17} color={ACCENT_TEAL} />
          </View>
          <View style={tw`flex-1`}>
            <Text fontWeight="bold" fontSize={13} classN={`text-[${fg}]`}>
              3 friends joined
            </Text>
            <Text fontSize={11} classN={`mt-0.5 text-[${muted}]`}>
              ₦3,000 earned so far
            </Text>
          </View>
        </View>

        <Text fontWeight="bold" fontSize={13} classN={`mt-6 text-[${fg}]`}>
          How it works
        </Text>
        <View style={tw`mt-2.5 gap-2.5`}>
          {REWARDS.map((reward) => (
            <View
              key={reward.title}
              style={[
                tw`rounded-2xl p-3.5`,
                { backgroundColor: colors.card, borderWidth: 1, borderColor: colors.grey5 },
              ]}>
              <Text fontWeight="bold" fontSize={13} classN={`text-[${fg}]`}>
                {reward.title}
              </Text>
              <Text fontSize={12} classN={`mt-0.5 text-[${muted}]`}>
                {reward.description}
              </Text>
            </View>
          ))}
        </View>
      </ScrollView>
    </SafeAreaView>
  );
}
