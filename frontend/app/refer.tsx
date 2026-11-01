import { router } from 'expo-router';
import { ChevronLeft, Copy, Gift, Users } from 'lucide-react-native';
import * as React from 'react';
import { ScrollView, Share, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import Text from '@/components/UI/Text';
import { CARD_SHADOW } from '@/components/UI/TaskParts';
import { tw, twColor } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';
import { withOpacity } from '@/theme/with-opacity';
import { colors } from '@/theme/palette';
import { PressableScale } from '@/components/UI/PressableScale';

const REFERRAL_CODE = 'BISOLA200';

const REWARDS = [
  { title: 'You get ₦1,000', description: 'Credited once your friend completes their first task' },
  { title: 'They get ₦500 off', description: 'Applied automatically to their first posted task' },
];

export default function ReferAFriend() {
  const { colors: systemColors } = useColorScheme();
  const fg = twColor(systemColors.foreground);
  const muted = twColor(systemColors.mutedForeground);
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
      style={[tw`flex-1`, { backgroundColor: systemColors.background }]}
      edges={['top', 'bottom']}>
      <View style={tw`flex-row items-center gap-3 px-4 pb-3 pt-2`}>
        <PressableScale onPress={() => router.back()} hitSlop={8}>
          <ChevronLeft size={24} color={systemColors.foreground} />
        </PressableScale>
        <Text variant="subhead" fontWeight="bold" classN={`text-[${fg}]`}>
          Refer a Friend
        </Text>
      </View>

      <ScrollView
        style={tw`flex-1 px-4`}
        contentContainerStyle={{ paddingBottom: 40 }}
        showsVerticalScrollIndicator={false}>
        <View style={tw`mt-3 items-center`}>
          <View
            style={[
              tw`h-16 w-16 items-center justify-center rounded-full`,
              { backgroundColor: withOpacity(colors.primary, 0.12) },
            ]}>
            <Gift size={28} color={colors.primary} />
          </View>
          <Text variant="title" fontWeight="black" classN={`mt-3 text-center text-[${fg}]`}>
            Invite friends, earn rewards
          </Text>
          <Text variant="caption" classN={`mt-1.5 text-center text-[${muted}]`}>
            Share your code — you both get rewarded when they complete their first task.
          </Text>
        </View>

        <View
          style={[
            tw`mt-6 flex-row items-center justify-between rounded-2xl p-4`,
            { backgroundColor: systemColors.card, borderWidth: 1, borderColor: systemColors.grey5 },
          ]}>
          <View>
            <Text variant="micro" fontWeight="normal" classN={`text-[${muted}]`}>
              Your referral code
            </Text>
            <Text
              variant="title"
              fontWeight="black"

              classN={`mt-1 text-[${twColor(colors.primary)}]`}>
              {REFERRAL_CODE}
            </Text>
          </View>
          <PressableScale
            onPress={handleCopy}
            style={[
              tw`flex-row items-center gap-1.5 rounded-full px-3.5 py-2`,
              { backgroundColor: withOpacity(colors.primary, 0.12) },
            ]}>
            <Copy size={14} color={colors.primary} />
            <Text
              variant="caption"
              fontWeight="medium"
              classN={`text-[${twColor(colors.primary)}]`}>
              {copied ? 'Copied!' : 'Copy'}
            </Text>
          </PressableScale>
        </View>

        <PressableScale
          onPress={handleShare}
          style={[
            tw`mt-2 items-center justify-center rounded-full py-4`,
            { backgroundColor: colors.primary },
          ]}>
          <Text variant="body" fontWeight="medium" classN="text-white">
            Share invite link
          </Text>
        </PressableScale>

        <View
          style={[
            tw`mt-6 flex-row items-center gap-3 rounded-2xl p-3.5`,
            { backgroundColor: systemColors.card },
            CARD_SHADOW,
          ]}>
          <View
            style={[
              tw`h-10 w-10 items-center justify-center rounded-full`,
              { backgroundColor: withOpacity(colors.primary, 0.12) },
            ]}>
            <Users size={17} color={colors.primary} />
          </View>
          <View style={tw`flex-1`}>
            <Text variant="caption" fontWeight="medium" classN={`text-[${fg}]`}>
              3 friends joined
            </Text>
            <Text variant="micro" fontWeight="normal" classN={`mt-0.5 text-[${muted}]`}>
              ₦3,000 earned so far
            </Text>
          </View>
        </View>

        <Text variant="caption" fontWeight="medium" classN={`mt-6 text-[${fg}]`}>
          How it works
        </Text>
        <View style={tw`mt-2.5 gap-2.5`}>
          {REWARDS.map((reward) => (
            <View
              key={reward.title}
              style={[
                tw`rounded-2xl p-3.5`,
                {
                  backgroundColor: systemColors.card,
                  borderWidth: 1,
                  borderColor: systemColors.grey5,
                },
              ]}>
              <Text variant="caption" fontWeight="medium" classN={`text-[${fg}]`}>
                {reward.title}
              </Text>
              <Text variant="caption" classN={`mt-0.5 text-[${muted}]`}>
                {reward.description}
              </Text>
            </View>
          ))}
        </View>
      </ScrollView>
    </SafeAreaView>
  );
}
