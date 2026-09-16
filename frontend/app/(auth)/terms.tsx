import { router, useLocalSearchParams } from 'expo-router';
import { ChevronDown } from 'lucide-react-native';
import React from 'react';
import { Pressable, ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import Text from '@/components/UI/Text';
import { tw } from '@/lib/tw';
import { colors } from '@/theme/palette';

type Segment = { text: string; bold?: boolean };

const INTRO =
  'These Terms and Conditions ("Terms") govern your use of the Sidekick mobile platform ("Platform", "Service") operated by Sidekick Technologies Limited ("Sidekick", "we", "us", "our"), a company incorporated under the laws of the Federal Republic of Nigeria. By creating an account or using the Platform in any capacity, you agree to be bound by these Terms in full. If you do not agree with any part of these Terms, you must not register or use the Platform.';

const SECTIONS: { title: string; intro: Segment[]; bullets: Segment[][] }[] = [
  {
    title: 'Escrow & Payment Policy',
    intro: [
      { text: 'By registering, you agree that ' },
      { text: "all task payments are processed through Sidekick's escrow system", bold: true },
      { text: '. This means:' },
    ],
    bullets: [
      [
        { text: 'Heroes must fund escrow ' },
        { text: 'before', bold: true },
        { text: ' a task goes live. Payment is not sent directly to the Sidekick.' },
      ],
      [
        {
          text: 'Funds are released to the Sidekick only after the Hero confirms task completion, or automatically after ',
        },
        { text: '24 hours', bold: true },
        { text: ' of no response.' },
      ],
      [
        { text: 'Sidekick earns the ' },
        { text: 'agreed price minus a 5% platform commission', bold: true },
        { text: '. This commission is non-negotiable and funds platform operations.' },
      ],
      [
        { text: 'In the event of a ' },
        { text: 'dispute', bold: true },
        {
          text: ", funds remain frozen in escrow until Sidekick's admin team resolves the case (within 48 hours).",
        },
      ],
      [
        { text: 'Refunds', bold: true },
        {
          text: " are issued if no Sidekick accepts a task within 24 hours, or if a dispute is resolved in the Hero's favour.",
        },
      ],
      [
        { text: 'Failed payments or withdrawals', bold: true },
        {
          text: ' must be retried by the user. Sidekick is not liable for delays caused by third-party payment providers (Paystack/Flutterwave).',
        },
      ],
    ],
  },
  {
    title: 'Trust, Safety & Identity',
    intro: [
      {
        text: 'You agree to the following trust and safety obligations as a condition of using Sidekick:',
      },
    ],
    bullets: [
      [
        { text: 'Real identity required', bold: true },
        {
          text: '. You must provide accurate personal information during registration. Fake, duplicate, or impersonated accounts will be permanently banned without refund.',
        },
      ],
      [
        { text: 'Identity verification.', bold: true },
        {
          text: ' Sidekick may request NIN, BVN, or a government-issued ID at any time to verify your identity. Refusal suspends your account.',
        },
      ],
      [
        { text: 'No harassment or misconduct.', bold: true },
        {
          text: ' Abusive, threatening, or discriminatory behaviour toward other users — on or off the platform — is grounds for immediate termination.',
        },
      ],
      [
        { text: 'You are responsible for your conduct.', bold: true },
        {
          text: ' Sidekick connects users but is not a party to any task contract. You accept all risk of physical interaction with other users.',
        },
      ],
      [
        { text: 'Suspicious accounts', bold: true },
        {
          text: '. Sidekick reserves the right to flag, suspend, or remove accounts showing unusual behaviour, at its sole discretion, with or without prior notice.',
        },
      ],
      [
        { text: 'Minors prohibited.', bold: true },
        {
          text: ' You confirm you are at least 18 years old. Accounts found to belong to minors will be immediately closed and funds returned to source.',
        },
      ],
    ],
  },
  {
    title: 'Platform Rules & Fair Use',
    intro: [
      {
        text: 'To keep Sidekick safe and functional for everyone, you agree to the following platform rules:',
      },
    ],
    bullets: [
      [
        { text: 'No off-platform transactions.', bold: true },
        {
          text: ' All payments for tasks discovered through Sidekick must go through the escrow system. Bypassing payments to avoid commission results in a permanent ban.',
        },
      ],
      [
        { text: 'Accurate task descriptions.', bold: true },
        {
          text: ' Heroes must post truthful task descriptions. Bait-and-switch tasks — where the actual work differs significantly from what was posted — are prohibited.',
        },
      ],
      [
        { text: 'Haggling limits.', bold: true },
        {
          text: ' Price negotiation is limited to 3 counter-offer rounds per task. Repeated harassment of Sidekicks with unreasonably low offers may result in account suspension.',
        },
      ],
      [
        { text: 'Prohibited tasks.', bold: true },
        {
          text: ' The following are strictly not allowed: illegal activities, tasks requiring professional licensing (medical, legal, electrical), adult content, tasks targeting vulnerable individuals, or any activity that violates Nigerian law.',
        },
      ],
      [
        { text: 'Review integrity.', bold: true },
        {
          text: ' Reviews and ratings must reflect genuine experiences. Fake reviews, review bombing, or coercing users into positive ratings is a bannable offence.',
        },
      ],
      [
        { text: 'Data & Privacy.', bold: true },
        {
          text: ' Sidekick collects and processes personal data as described in our Privacy Policy. By registering, you consent to this processing for the purpose of providing the platform service.',
        },
      ],
    ],
  },
];

function RichText({
  segments,
  prefix,
  classN,
}: {
  segments: Segment[];
  prefix?: string;
  classN?: string;
}) {
  return (
    <Text fontSize={13} classN={`${classN ?? ''} text-[${colors.textBody}]`.trim()}>
      {prefix}
      {segments.map((segment, i) =>
        segment.bold ? (
          <Text key={i} fontWeight="bold" fontSize={13} classN="text-black">
            {segment.text}
          </Text>
        ) : (
          <Text key={i} fontSize={13} classN={`text-[${colors.textBody}]`}>
            {segment.text}
          </Text>
        )
      )}
    </Text>
  );
}

export default function Terms() {
  const { phone } = useLocalSearchParams<{ phone?: string }>();
  const [openIndex, setOpenIndex] = React.useState<number | null>(0);
  const [acceptedIndexes, setAcceptedIndexes] = React.useState<Set<number>>(new Set());

  const allAccepted = acceptedIndexes.size === SECTIONS.length;

  function toggleSection(index: number) {
    setOpenIndex((current) => (current === index ? null : index));
  }

  function acceptSection(index: number) {
    const updated = new Set(acceptedIndexes).add(index);
    setAcceptedIndexes(updated);
    const nextUnaccepted = SECTIONS.findIndex((_, i) => !updated.has(i));
    setOpenIndex(nextUnaccepted !== -1 ? nextUnaccepted : null);
  }

  function handleCreateAccount() {
    if (!allAccepted) return;
    router.push({ pathname: '/verify-phone', params: { phone: phone ?? '' } });
  }

  return (
    <SafeAreaView style={tw`flex-1 bg-white`}>
      <View style={tw`flex-1 px-4`}>
        <Text fontWeight="bold" fontSize={24} classN="mt-3 text-black">
          Terms & Conditions
        </Text>
        <Text fontSize={14} classN={`mt-1 text-[${colors.textSubtle}]`}>
          Before you create an account, please read and accept our Terms and Conditions
        </Text>

        <ScrollView
          style={tw`mt-4`}
          contentContainerStyle={{ paddingBottom: 16 }}
          showsVerticalScrollIndicator={false}>
          <Text fontSize={13} classN="text-black">
            Effective Date: 1 January 2025
          </Text>
          <Text fontSize={13} classN="text-black">
            Governing Law: Federal Republic of Nigeria
          </Text>

          <View style={[tw`mt-4 h-px`, { backgroundColor: colors.borderDivider }]} />

          <Text fontSize={13} classN={`mt-4 text-[${colors.textBody}]`}>
            {INTRO}
          </Text>

          <View style={tw`mt-5 gap-3`}>
            {SECTIONS.map((section, index) => {
              const isOpen = openIndex === index;
              const isAccepted = acceptedIndexes.has(index);
              return (
                <View
                  key={section.title}
                  style={{
                    borderColor: colors.brandBorder,
                    ...tw`rounded-3xl border px-4 py-3`,
                  }}>
                  <Pressable onPress={() => toggleSection(index)} style={tw`flex-row items-center`}>
                    <View
                      style={{
                        ...tw`h-7 w-7 items-center bg-[${colors.borderLight}] justify-center rounded-full`,
                      }}>
                      <Text fontWeight="bold" fontSize={11.11} classN={`text-black`}>
                        {index + 1}
                      </Text>
                    </View>
                    <View style={tw`ml-3 flex-1`}>
                      <Text fontWeight="bold" fontSize={15} classN="text-black">
                        {section.title}
                      </Text>
                      {isAccepted ? (
                        <View
                          style={[
                            tw`mt-1 self-start rounded-full px-2 py-0.5`,
                            { backgroundColor: colors.brandTint },
                          ]}>
                          <Text fontSize={11} classN={`text-[${colors.brand}]`}>
                            Accepted
                          </Text>
                        </View>
                      ) : (
                        <Text fontSize={12} classN={`text-[${colors.textMutedAlt}]`}>
                          Tap to read • Required
                        </Text>
                      )}
                    </View>
                    <ChevronDown
                      size={20}
                      color={colors.textMutedAlt}
                      style={{ transform: [{ rotate: isOpen ? '180deg' : '0deg' }] }}
                    />
                  </Pressable>

                  {isOpen && (
                    <View style={tw`mt-3`}>
                      <RichText segments={section.intro} />
                      {section.bullets.map((segments, i) => (
                        <RichText key={i} segments={segments} prefix="—  " classN="mt-2" />
                      ))}

                      {!isAccepted && (
                        <View style={tw`mt-4 flex-row gap-3`}>
                          <Pressable
                            onPress={() => setOpenIndex(null)}
                            style={{
                              borderColor: colors.borderStrong,
                              ...tw`flex-1 items-center justify-center rounded-full border py-3`,
                            }}>
                            <Text fontWeight="medium" fontSize={14} classN="text-black">
                              Cancel
                            </Text>
                          </Pressable>
                          <Pressable
                            onPress={() => acceptSection(index)}
                            style={{
                              borderColor: colors.brand,
                              ...tw`flex-1 items-center justify-center rounded-full border py-3`,
                            }}>
                            <Text
                              fontWeight="medium"
                              fontSize={14}
                              classN={`text-[${colors.brand}]`}>
                              I Agree
                            </Text>
                          </Pressable>
                        </View>
                      )}
                    </View>
                  )}
                </View>
              );
            })}
          </View>
        </ScrollView>

        <Pressable
          disabled={!allAccepted}
          onPress={handleCreateAccount}
          style={{
            backgroundColor: allAccepted ? colors.brand : colors.brandDisabled,
            ...tw`mb-4 items-center justify-center rounded-full py-4`,
          }}>
          <Text
            fontWeight="bold"
            fontSize={16}
            classN={allAccepted ? 'text-white' : 'text-white/70'}>
            Create Account
          </Text>
        </Pressable>
      </View>
    </SafeAreaView>
  );
}
