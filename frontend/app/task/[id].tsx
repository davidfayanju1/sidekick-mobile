import { router, useLocalSearchParams } from 'expo-router';
import { Check, ChevronLeft, Clock, MessageCircle, Star } from 'lucide-react-native';
import * as React from 'react';
import { ScrollView, TextInput, View } from 'react-native';
import Animated, { FadeInDown, FadeOut } from 'react-native-reanimated';
import { SafeAreaView } from 'react-native-safe-area-context';

import { toDisplayTask } from '@/lib/api/adapters';
import { useCompleteTask, useConfirmTask, useDisputeTask } from '@/lib/api/execution';
import { useTaskConversation } from '@/lib/api/offers';
import { useTask } from '@/lib/api/tasks';
import { useLeaveReview } from '@/lib/api/trust';
import { errorMessage } from '@/lib/axios';
import { toast } from '@/store/toastStore';
import Text from '@/components/UI/Text';
import { EmptyState, ErrorState } from '@/components/UI/EmptyState';
import { NoMatch } from '@/components/UI/illustrations';
import { SkeletonTaskCard, SkeletonText } from '@/components/UI/Skeleton';
import { ConfirmTaskSheet, DisputeSheet } from '@/components/UI/TaskSheets';
import { CARD_SHADOW, StatusPill, formatTimeAgo, getCategoryMeta } from '@/components/UI/TaskParts';
import { TaskTimeline, type TimelineStep } from '@/components/UI/TaskTimeline';
import { tw, twColor } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';
import { useRoleStore } from '@/store/roleStore';
import { useTaskStore } from '@/store/taskStore';
import { withOpacity } from '@/theme/with-opacity';
import { colors } from '@/theme/palette';
import { PressableScale } from '@/components/UI/PressableScale';

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

  const taskQuery = useTask(id);
  const completeTask = useCompleteTask(id);
  const confirmTask = useConfirmTask(id);
  const disputeTask = useDisputeTask(id);
  const leaveReview = useLeaveReview(id);
  // The chat route is keyed by conversation, not task — an accepted offer is
  // what creates one, so this is null until then.
  const conversationId = useTaskConversation(id);

  // The Sidekick's in-flight task is still held locally (see myTasksStore for
  // why); everything else reads from the server.
  const task = isOwnCurrentTask
    ? sidekickCurrentTask
    : taskQuery.data
      ? toDisplayTask(taskQuery.data)
      : undefined;

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
      <SafeAreaView style={[tw`flex-1`, { backgroundColor: systemColors.background }]}>
        {taskQuery.isPending ? (
          <View style={tw`p-4`}>
            <SkeletonTaskCard />
            <View style={tw`mt-4`}>
              <SkeletonText lines={4} />
            </View>
          </View>
        ) : taskQuery.isError ? (
          <ErrorState body={errorMessage(taskQuery.error)} onRetry={() => taskQuery.refetch()} />
        ) : (
          <EmptyState
            illustration={<NoMatch />}
            title="This task isn’t here"
            body="It may have been cancelled, or the link is out of date."
            action={{ label: 'Back to home', onPress: () => router.replace('/(tabs)') }}
          />
        )}
      </SafeAreaView>
    );
  }

  const { color: categoryColor, icon: CategoryIcon } = getCategoryMeta(task.category);
  const counterpartName = isSidekick ? task.heroName : task.sidekickName;

  const handleStart = () => {
    // Starting is a local milestone — the API has no "in progress" transition,
    // it goes straight from assigned to completed.
    startCurrentTask();
    setJustStarted(true);
  };

  const handleMarkComplete = async () => {
    try {
      await completeTask.mutateAsync({});
      completeCurrentTask();
      toast.success('Marked as complete', {
        description: 'The Hero has 72 hours to confirm before payment auto-releases.',
      });
      router.replace('/(tabs)');
    } catch (error) {
      toast.error('Couldn’t mark it complete', { description: errorMessage(error) });
    }
  };

  const handleConfirm = async () => {
    try {
      await confirmTask.mutateAsync(undefined);
      setActiveSheet(null);
      toast.success('Payment released', {
        description: 'The escrow has been sent to your Sidekick.',
      });
    } catch (error) {
      toast.error('Couldn’t confirm the task', { description: errorMessage(error) });
    }
  };

  const handleDispute = async (reason: string, description: string) => {
    try {
      await disputeTask.mutateAsync({ reason, description });
      setActiveSheet(null);
      toast.warning('Dispute opened', {
        description: 'Escrow is frozen while our team reviews it.',
      });
    } catch (error) {
      toast.error('Couldn’t open the dispute', { description: errorMessage(error) });
    }
  };

  const handleSubmitReview = async () => {
    if (rating === 0) return;
    try {
      await leaveReview.mutateAsync({ rating, comment: reviewNote || undefined });
      setReviewSubmitted(true);
    } catch (error) {
      toast.error('Couldn’t submit your review', { description: errorMessage(error) });
    }
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
        <PressableScale onPress={() => router.back()} hitSlop={8}>
          <ChevronLeft size={24} color={systemColors.foreground} />
        </PressableScale>
        <Text variant="subhead" fontWeight="bold" classN={`text-[${fg}]`}>
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
            <Text variant="caption" fontWeight="medium" classN={`text-[${categoryColor}]`}>
              {task.category.toUpperCase()}
            </Text>
          </View>
          <StatusPill status={task.status} label={task.statusLabel} />
        </View>

        <Text variant="title" fontWeight="black" classN={`mt-2 text-[${fg}]`}>
          {task.title}
        </Text>
        {task.description && (
          <Text variant="caption" classN={`mt-1.5 text-[${muted}]`}>
            {task.description}
          </Text>
        )}

        {isSidekick && isOwnCurrentTask && task.status === 'matched' && (
          <View
            style={[
              tw`mt-3 flex-row items-center gap-3 rounded-2xl p-3.5`,
              { backgroundColor: withOpacity(colors.primary, 0.12) },
            ]}>
            <View
              style={[
                tw`h-10 w-10 items-center justify-center rounded-full`,
                { backgroundColor: colors.primary },
              ]}>
              <Check size={18} color="white" strokeWidth={3} />
            </View>
            <View style={tw`flex-1`}>
              <Text variant="body" fontWeight="medium" classN={`text-[${twColor(colors.primary)}]`}>
                Task accepted!
              </Text>
              <Text variant="caption" classN={`mt-0.5 text-[${twColor(colors.primary)}]`}>
                You&rsquo;ll earn {task.price} once it&rsquo;s done. Tap Start when you&rsquo;re
                heading out.
              </Text>
            </View>
          </View>
        )}

        {counterpartName && (
          <View
            style={[
              tw`mt-3 flex-row items-center gap-3 rounded-2xl p-3.5`,
              { backgroundColor: systemColors.card },
              CARD_SHADOW,
            ]}>
            <View
              style={[
                tw`h-11 w-11 items-center justify-center rounded-full`,
                { backgroundColor: colors.orangeTint },
              ]}>
              <Text variant="body" fontWeight="medium" classN={`text-[${colors.orangeDark}]`}>
                {counterpartName.charAt(0)}
              </Text>
            </View>
            <View style={tw`flex-1`}>
              <Text variant="body" fontWeight="medium" classN={`text-[${fg}]`}>
                {counterpartName}
              </Text>
              <Text variant="micro" fontWeight="normal" classN={`mt-0.5 text-[${muted}]`}>
                {isSidekick ? 'Task Hero' : 'Your Sidekick'}
              </Text>
            </View>
            <PressableScale
              onPress={() =>
                conversationId.data
                  ? router.push(`/chat/${conversationId.data}`)
                  : toast.info('No chat yet', {
                      description: 'A chat opens as soon as you accept an offer.',
                    })
              }
              hitSlop={8}
              style={[
                tw`h-10 w-10 items-center justify-center rounded-full`,
                { backgroundColor: withOpacity(colors.primary, 0.12) },
              ]}>
              <MessageCircle size={18} color={colors.primary} />
            </PressableScale>
          </View>
        )}

        {task.currentStep !== undefined && (
          <View
            style={[tw`mt-3 rounded-2xl p-4`, { backgroundColor: systemColors.card }, CARD_SHADOW]}>
            <Text variant="body" fontWeight="medium" classN={`text-[${fg}]`}>
              Progress
            </Text>
            <TaskTimeline steps={timelineSteps} currentStep={task.currentStep} />
          </View>
        )}

        {task.disputeText && (
          <View style={[tw`mt-3 rounded-xl px-3 py-2.5`, { backgroundColor: colors.dangerTint }]}>
            <Text variant="caption" classN={`text-[${colors.danger}]`}>
              {task.disputeText}
            </Text>
          </View>
        )}

        <View
          style={[tw`mt-3 rounded-2xl p-4`, { backgroundColor: systemColors.card }, CARD_SHADOW]}>
          <View style={tw`flex-row items-center gap-2`}>
            <Clock size={14} color={systemColors.mutedForeground} />
            <Text variant="caption" classN={`text-[${muted}]`}>
              Min. delivery time
            </Text>
          </View>
          <Text variant="body" fontWeight="medium" classN={`mt-1 text-[${fg}]`}>
            {task.etaWindow ?? 'Not specified'}
          </Text>

          {task.price && (
            <View
              style={[
                tw`mt-2 flex-row items-center justify-between pt-3`,
                { borderTopWidth: 1, borderTopColor: systemColors.grey5 },
              ]}>
              <Text variant="caption" classN={`text-[${colors.orangeMuted}]`}>
                {task.payIn}
              </Text>
              <Text variant="subhead" fontWeight="black" classN={`text-[${fg}]`}>
                {task.price}
              </Text>
            </View>
          )}
        </View>

        {isSidekick && task.status === 'matched' && (
          <PressableScale
            onPress={handleStart}
            style={[
              tw`mt-5 items-center justify-center rounded-full py-4`,
              { backgroundColor: colors.primary },
            ]}>
            <Text variant="body" fontWeight="medium" classN="text-white">
              Start task
            </Text>
          </PressableScale>
        )}

        {justStarted && (
          <Animated.View
            entering={FadeInDown}
            exiting={FadeOut}
            style={[
              tw`mt-2 flex-row items-center gap-2.5 rounded-2xl p-3`,
              { backgroundColor: withOpacity(colors.green, 0.12) },
            ]}>
            <View
              style={[
                tw`h-7 w-7 items-center justify-center rounded-full`,
                { backgroundColor: colors.green },
              ]}>
              <Check size={14} color="white" strokeWidth={3} />
            </View>
            <Text variant="caption" fontWeight="medium" classN={`text-[${twColor(colors.green)}]`}>
              Task started! Your timeline is updated below.
            </Text>
          </Animated.View>
        )}

        {isSidekick && task.status === 'in_progress' && (
          <PressableScale
            onPress={handleMarkComplete}
            style={[
              tw`mt-5 items-center justify-center rounded-full py-4`,
              { backgroundColor: colors.primary },
            ]}>
            <Text variant="body" fontWeight="medium" classN="text-white">
              Mark task as complete
            </Text>
          </PressableScale>
        )}

        {!isSidekick && task.status === 'in_progress' && (
          <PressableScale
            onPress={() => setActiveSheet('confirm')}
            style={[
              tw`mt-5 items-center justify-center rounded-full py-4`,
              { backgroundColor: colors.primary },
            ]}>
            <Text variant="body" fontWeight="medium" classN="text-white">
              Confirm task done
            </Text>
          </PressableScale>
        )}

        {!isSidekick && task.sidekickName && (
          <View style={tw`mt-6`}>
            <Text variant="body" fontWeight="medium" classN={`text-[${fg}]`}>
              {reviewSubmitted ? 'Thanks for the feedback!' : `Rate ${task.sidekickName}`}
            </Text>
            {!reviewSubmitted && (
              <>
                <View style={tw`mt-2 flex-row gap-2`}>
                  {Array.from({ length: 5 }).map((_, index) => {
                    const filled = index < rating;
                    return (
                      <PressableScale key={index} onPress={() => setRating(index + 1)} hitSlop={6}>
                        <Star
                          size={28}
                          color={filled ? colors.star : colors.gray200}
                          fill={filled ? colors.star : colors.gray200}
                        />
                      </PressableScale>
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
                    tw`mt-2 rounded-2xl p-4`,
                    {
                      borderWidth: 1,
                      borderColor: systemColors.grey5,
                      minHeight: 88,
                      textAlignVertical: 'top',
                      color: systemColors.foreground,
                    },
                  ]}
                />
                <PressableScale
                  disabled={rating === 0}
                  onPress={handleSubmitReview}
                  style={[
                    tw`mt-2 items-center justify-center rounded-full py-3.5`,
                    { backgroundColor: colors.primary, opacity: rating === 0 ? 0.5 : 1 },
                  ]}>
                  <Text variant="body" fontWeight="medium" classN="text-white">
                    Submit review
                  </Text>
                </PressableScale>
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
            onConfirm={handleConfirm}
            confirming={confirmTask.isPending}
            sidekickName={task.sidekickName ?? 'Your Sidekick'}
            price={task.price ?? ''}
          />
          <DisputeSheet
            visible={activeSheet === 'dispute'}
            onClose={() => setActiveSheet(null)}
            onSubmit={handleDispute}
            submitting={disputeTask.isPending}
          />
        </>
      )}
    </SafeAreaView>
  );
}
