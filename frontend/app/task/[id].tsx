import { router, useLocalSearchParams } from 'expo-router';
import { Check, ChevronLeft, Clock, MessageCircle, Star } from 'lucide-react-native';
import * as React from 'react';
import { Pressable, ScrollView, TextInput, View } from 'react-native';
import Animated, { FadeInDown, FadeOut } from 'react-native-reanimated';
import { SafeAreaView } from 'react-native-safe-area-context';

import { POSTED_TASKS } from '@/app/(tabs)/index';
import Text from '@/components/UI/Text';
import { ConfirmTaskSheet, DisputeSheet } from '@/components/UI/TaskSheets';
import {
  ACCENT_TEAL,
  CARD_SHADOW,
  StatusPill,
  formatTimeAgo,
  getCategoryMeta,
} from '@/components/UI/TaskParts';
import { TaskTimeline, type TimelineStep } from '@/components/UI/TaskTimeline';
import { tw, twColor } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';
import { useRoleStore } from '@/store/roleStore';
import { useTaskStore } from '@/store/taskStore';
import { withOpacity } from '@/theme/with-opacity';
import { colors } from '@/theme/palette';

const STEP_TITLES = ['Posted', 'Matched', 'In Progress', 'Confirm', 'Payment'];

function buildTimelineSteps(
  isSidekick: boolean,
  matchedAt: number | null,
  startedAt: number | null
): TimelineStep[] {
  const descriptions = isSidekick
    ? [
        'Task was posted by the Hero',
        'You accepted this task',
        "You're working on this task",
        'Waiting for the Hero to confirm completion',
        'Payment released to your wallet',
      ]
    : [
        'You posted this task',
        'A Sidekick accepted your task',
        'Your Sidekick is working on the task',
        'Confirm the task is complete to release payment',
        'Payment released to your Sidekick',
      ];

  const timestamps: (string | undefined)[] = [
    undefined,
    undefined,
    undefined,
    undefined,
    undefined,
  ];
  if (matchedAt) timestamps[1] = formatTimeAgo(matchedAt);
  if (startedAt) timestamps[2] = formatTimeAgo(startedAt);

  return STEP_TITLES.map((title, index) => ({
    title,
    description: descriptions[index],
    timestamp: timestamps[index],
  }));
}

export default function TaskDetails() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { colors: systemColors } = useColorScheme();
  const fg = twColor(systemColors.foreground);
  const muted = twColor(systemColors.mutedForeground);
  const role = useRoleStore((state) => state.role);
  const isSidekick = role === 'sidekick';
  const sidekickCurrentTask = useTaskStore((state) => state.sidekickCurrentTask);
  const sidekickTaskMatchedAt = useTaskStore((state) => state.sidekickTaskMatchedAt);
  const sidekickTaskStartedAt = useTaskStore((state) => state.sidekickTaskStartedAt);
  const startCurrentTask = useTaskStore((state) => state.startCurrentTask);
  const completeCurrentTask = useTaskStore((state) => state.completeCurrentTask);

  const isOwnCurrentTask = isSidekick && sidekickCurrentTask?.id === id;
  const task = isOwnCurrentTask ? sidekickCurrentTask : POSTED_TASKS.find((item) => item.id === id);

  const [activeSheet, setActiveSheet] = React.useState<'confirm' | 'dispute' | null>(null);
  const [rating, setRating] = React.useState(0);
  const [reviewNote, setReviewNote] = React.useState('');
  const [reviewSubmitted, setReviewSubmitted] = React.useState(false);
  const [justStarted, setJustStarted] = React.useState(false);

  React.useEffect(() => {
    if (!justStarted) return;
    const timeout = setTimeout(() => setJustStarted(false), 2600);
    return () => clearTimeout(timeout);
  }, [justStarted]);

  if (!task) {
    return (
      <SafeAreaView
        style={[
          tw`flex-1 items-center justify-center`,
          { backgroundColor: systemColors.background },
        ]}>
        <Text fontSize={14} classN={`text-[${muted}]`}>
          Task not found
        </Text>
      </SafeAreaView>
    );
  }

  const { color: categoryColor, icon: CategoryIcon } = getCategoryMeta(task.category);
  const counterpartName = isSidekick ? task.heroName : task.sidekickName;

  const handleStart = () => {
    startCurrentTask();
    setJustStarted(true);
  };

  const handleMarkComplete = () => {
    completeCurrentTask();
    router.replace('/(tabs)');
  };

  const handleSubmitReview = () => {
    if (rating === 0) return;
    setReviewSubmitted(true);
  };

  const timelineSteps = buildTimelineSteps(
    isSidekick,
    isOwnCurrentTask ? sidekickTaskMatchedAt : null,
    isOwnCurrentTask ? sidekickTaskStartedAt : null
  );

  return (
    <SafeAreaView
      style={[tw`flex-1`, { backgroundColor: systemColors.background }]}
      edges={['top', 'bottom']}>
      <View style={tw`flex-row items-center gap-3 px-4 pb-3 pt-2`}>
        <Pressable onPress={() => router.back()} hitSlop={8}>
          <ChevronLeft size={24} color={systemColors.foreground} />
        </Pressable>
        <Text fontWeight="bold" fontSize={16} classN={`text-[${fg}]`}>
          Task Details
        </Text>
      </View>

      <ScrollView
        style={tw`flex-1 px-4`}
        contentContainerStyle={{ paddingBottom: 40 }}
        showsVerticalScrollIndicator={false}>
        <View style={tw`flex-row items-center justify-between`}>
          <View style={tw`flex-row items-center gap-1.5`}>
            <CategoryIcon size={14} color={categoryColor} />
            <Text fontWeight="bold" fontSize={12} classN={`text-[${categoryColor}]`}>
              {task.category.toUpperCase()}
            </Text>
          </View>
          <StatusPill status={task.status} label={task.statusLabel} />
        </View>

        <Text fontWeight="black" fontSize={20} classN={`mt-2 text-[${fg}]`}>
          {task.title}
        </Text>
        {task.description && (
          <Text fontSize={13} classN={`mt-1.5 text-[${muted}]`}>
            {task.description}
          </Text>
        )}

        {isSidekick && isOwnCurrentTask && task.status === 'matched' && (
          <View
            style={[
              tw`mt-4 flex-row items-center gap-3 rounded-2xl p-3.5`,
              { backgroundColor: withOpacity(ACCENT_TEAL, 0.12) },
            ]}>
            <View
              style={[
                tw`h-10 w-10 items-center justify-center rounded-full`,
                { backgroundColor: ACCENT_TEAL },
              ]}>
              <Check size={18} color="white" strokeWidth={3} />
            </View>
            <View style={tw`flex-1`}>
              <Text fontWeight="bold" fontSize={14} classN={`text-[${twColor(ACCENT_TEAL)}]`}>
                Task accepted!
              </Text>
              <Text fontSize={12} classN={`mt-0.5 text-[${twColor(ACCENT_TEAL)}]`}>
                You&rsquo;ll earn {task.price} once it&rsquo;s done. Tap Start when you&rsquo;re
                heading out.
              </Text>
            </View>
          </View>
        )}

        {counterpartName && (
          <View
            style={[
              tw`mt-4 flex-row items-center gap-3 rounded-2xl p-3.5`,
              { backgroundColor: systemColors.card },
              CARD_SHADOW,
            ]}>
            <View
              style={[
                tw`h-11 w-11 items-center justify-center rounded-full`,
                { backgroundColor: colors.accentSand },
              ]}>
              <Text fontWeight="bold" fontSize={15} classN={`text-[${colors.pendingText}]`}>
                {counterpartName.charAt(0)}
              </Text>
            </View>
            <View style={tw`flex-1`}>
              <Text fontWeight="bold" fontSize={14} classN={`text-[${fg}]`}>
                {counterpartName}
              </Text>
              <Text fontSize={11} classN={`mt-0.5 text-[${muted}]`}>
                {isSidekick ? 'Task Hero' : 'Your Sidekick'}
              </Text>
            </View>
            <Pressable
              onPress={() => router.push(`/chat/${task.id}`)}
              hitSlop={8}
              style={[
                tw`h-10 w-10 items-center justify-center rounded-full`,
                { backgroundColor: withOpacity(ACCENT_TEAL, 0.12) },
              ]}>
              <MessageCircle size={18} color={ACCENT_TEAL} />
            </Pressable>
          </View>
        )}

        {task.currentStep !== undefined && (
          <View
            style={[tw`mt-4 rounded-2xl p-4`, { backgroundColor: systemColors.card }, CARD_SHADOW]}>
            <Text fontWeight="bold" fontSize={14} classN={`text-[${fg}]`}>
              Progress
            </Text>
            <TaskTimeline steps={timelineSteps} currentStep={task.currentStep} />
          </View>
        )}

        {task.disputeText && (
          <View style={[tw`mt-4 rounded-xl px-3 py-2.5`, { backgroundColor: colors.disputeBg }]}>
            <Text fontSize={12} classN={`text-[${colors.dangerText}]`}>
              {task.disputeText}
            </Text>
          </View>
        )}

        <View
          style={[tw`mt-4 rounded-2xl p-4`, { backgroundColor: systemColors.card }, CARD_SHADOW]}>
          <View style={tw`flex-row items-center gap-2`}>
            <Clock size={14} color={systemColors.mutedForeground} />
            <Text fontSize={12} classN={`text-[${muted}]`}>
              Min. delivery time
            </Text>
          </View>
          <Text fontWeight="bold" fontSize={14} classN={`mt-1 text-[${fg}]`}>
            {task.etaWindow ?? 'Not specified'}
          </Text>

          {task.price && (
            <View
              style={[
                tw`mt-3 flex-row items-center justify-between pt-3`,
                { borderTopWidth: 1, borderTopColor: systemColors.grey5 },
              ]}>
              <Text fontSize={12} classN={`text-[${colors.warning}]`}>
                {task.payIn}
              </Text>
              <Text fontWeight="black" fontSize={17} classN={`text-[${fg}]`}>
                {task.price}
              </Text>
            </View>
          )}
        </View>

        {isSidekick && task.status === 'matched' && (
          <Pressable
            onPress={handleStart}
            style={[
              tw`mt-5 items-center justify-center rounded-full py-4`,
              { backgroundColor: ACCENT_TEAL },
            ]}>
            <Text fontWeight="bold" fontSize={15} classN="text-white">
              Start task
            </Text>
          </Pressable>
        )}

        {justStarted && (
          <Animated.View
            entering={FadeInDown}
            exiting={FadeOut}
            style={[
              tw`mt-3 flex-row items-center gap-2.5 rounded-2xl p-3`,
              { backgroundColor: withOpacity(colors.success, 0.12) },
            ]}>
            <View
              style={[
                tw`h-7 w-7 items-center justify-center rounded-full`,
                { backgroundColor: colors.success },
              ]}>
              <Check size={14} color="white" strokeWidth={3} />
            </View>
            <Text fontWeight="medium" fontSize={13} classN={`text-[${twColor(colors.success)}]`}>
              Task started! Your timeline is updated below.
            </Text>
          </Animated.View>
        )}

        {isSidekick && task.status === 'in_progress' && (
          <Pressable
            onPress={handleMarkComplete}
            style={[
              tw`mt-5 items-center justify-center rounded-full py-4`,
              { backgroundColor: ACCENT_TEAL },
            ]}>
            <Text fontWeight="bold" fontSize={15} classN="text-white">
              Mark task as complete
            </Text>
          </Pressable>
        )}

        {!isSidekick && task.status === 'in_progress' && (
          <Pressable
            onPress={() => setActiveSheet('confirm')}
            style={[
              tw`mt-5 items-center justify-center rounded-full py-4`,
              { backgroundColor: ACCENT_TEAL },
            ]}>
            <Text fontWeight="bold" fontSize={15} classN="text-white">
              Confirm task done
            </Text>
          </Pressable>
        )}

        {!isSidekick && task.sidekickName && (
          <View style={tw`mt-6`}>
            <Text fontWeight="bold" fontSize={15} classN={`text-[${fg}]`}>
              {reviewSubmitted ? 'Thanks for the feedback!' : `Rate ${task.sidekickName}`}
            </Text>
            {!reviewSubmitted && (
              <>
                <View style={tw`mt-3 flex-row gap-2`}>
                  {Array.from({ length: 5 }).map((_, index) => {
                    const filled = index < rating;
                    return (
                      <Pressable key={index} onPress={() => setRating(index + 1)} hitSlop={6}>
                        <Star
                          size={28}
                          color={filled ? colors.star : colors.borderLight}
                          fill={filled ? colors.star : colors.borderLight}
                        />
                      </Pressable>
                    );
                  })}
                </View>
                <TextInput
                  value={reviewNote}
                  onChangeText={setReviewNote}
                  placeholder="Leave a comment (optional)"
                  placeholderTextColor={systemColors.mutedForeground}
                  multiline
                  style={[
                    tw`mt-3 rounded-2xl p-4`,
                    {
                      borderWidth: 1,
                      borderColor: systemColors.grey5,
                      minHeight: 88,
                      textAlignVertical: 'top',
                      color: systemColors.foreground,
                    },
                  ]}
                />
                <Pressable
                  disabled={rating === 0}
                  onPress={handleSubmitReview}
                  style={[
                    tw`mt-3 items-center justify-center rounded-full py-3.5`,
                    { backgroundColor: ACCENT_TEAL, opacity: rating === 0 ? 0.5 : 1 },
                  ]}>
                  <Text fontWeight="bold" fontSize={14} classN="text-white">
                    Submit review
                  </Text>
                </Pressable>
              </>
            )}
          </View>
        )}
      </ScrollView>

      {!isSidekick && (
        <>
          <ConfirmTaskSheet
            visible={activeSheet === 'confirm'}
            onClose={() => setActiveSheet(null)}
            onReportDispute={() => setActiveSheet('dispute')}
            sidekickName={task.sidekickName ?? 'Your Sidekick'}
            price={task.price ?? ''}
          />
          <DisputeSheet
            visible={activeSheet === 'dispute'}
            onClose={() => setActiveSheet(null)}
            onSubmit={() => setActiveSheet(null)}
          />
        </>
      )}
    </SafeAreaView>
  );
}
