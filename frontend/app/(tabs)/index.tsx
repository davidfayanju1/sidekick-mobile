import { router } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import {
  ChevronRight,
  ClipboardList,
  Plus,
  ShieldCheck,
  TriangleAlert,
  type LucideIcon,
} from 'lucide-react-native';
import * as React from 'react';
import { ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import RoleSelectSheet from '@/components/UI/RoleSelectSheet';
import Text from '@/components/UI/Text';
import { ReviewOffersSheet } from '@/components/UI/TaskSheets';
import {
  CARD_SHADOW,
  ProgressTracker,
  QUICK_CATEGORIES,
  StatusPill,
  getCategoryMeta,
  type Task,
} from '@/components/UI/TaskParts';
import { LiveDot } from '@/components/UI/TaskTimeline';
import { EmptyState } from '@/components/UI/EmptyState';
import { AwaitingOffers, OpenSlot } from '@/components/UI/illustrations';
import { SkeletonTaskCard } from '@/components/UI/Skeleton';
import { toDisplayTask } from '@/lib/api/adapters';
import { useSetRole } from '@/lib/api/auth';
import { money } from '@/lib/format';
import { useMyPostedTasks } from '@/lib/api/myTasks';
import { errorMessage } from '@/lib/axios';
import { tw, twColor } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';
import {
  intentFromRole,
  useOutstandingStepsCount,
  useRoleStore,
  type UserRole,
} from '@/store/roleStore';
import { useAuthStore } from '@/store/authStore';
import { toast } from '@/store/toastStore';
import { useTaskStore } from '@/store/taskStore';
import { withOpacity } from '@/theme/with-opacity';
import { colors } from '@/theme/palette';
import { PressableScale } from '@/components/UI/PressableScale';
import { StaggerItem } from '@/components/UI/Stagger';

function getGreeting() {
  const hour = new Date().getHours();
  if (hour < 12) return 'Good morning';
  if (hour < 17) return 'Good afternoon';
  return 'Good evening';
}

function TaskCard({
  task,
  onPress,
  showLiveBadge,
}: {
  task: Task;
  onPress?: () => void;
  showLiveBadge?: boolean;
}) {
  const { colors: systemColors } = useColorScheme();
  const fg = twColor(systemColors.foreground);
  const muted = twColor(systemColors.mutedForeground);
  const { color: categoryColor, icon: CategoryIcon } = getCategoryMeta(task.category);

  return (
    <PressableScale
      onPress={onPress}
      disabled={!onPress}
      style={[
        tw`mt-3 flex-row overflow-hidden rounded-2xl`,
        { backgroundColor: systemColors.card },
        CARD_SHADOW,
      ]}>
      <View style={{ width: 4, backgroundColor: categoryColor }} />
      <View style={tw`flex-1 p-4`}>
        <View style={tw`flex-row items-center justify-between`}>
          <View style={tw`flex-row items-center gap-1.5`}>
            <CategoryIcon size={13} color={categoryColor} />
            <Text variant="micro" classN={`text-[${categoryColor}]`}>
              {task.category.toUpperCase()}
            </Text>
          </View>
          <View style={tw`flex-row items-center gap-2`}>
            {showLiveBadge && task.status === 'in_progress' && <LiveDot />}
            <StatusPill status={task.status} label={task.statusLabel} />
          </View>
        </View>

        <Text variant="subhead" fontWeight="bold" classN={`mt-1 text-[${fg}]`}>
          {task.title}
        </Text>
        <Text variant="caption" classN={`mt-0.5 text-[${muted}]`}>
          {task.meta}
        </Text>

        {task.offersText && (
          <View style={[tw`mt-2 rounded-xl px-3 py-2`, { backgroundColor: colors.greenTint }]}>
            <Text variant="caption" classN={`text-[${colors.green}]`}>
              {task.offersText}
            </Text>
          </View>
        )}

        {task.disputeText && (
          <View style={[tw`mt-2 rounded-xl px-3 py-2`, { backgroundColor: colors.dangerTint }]}>
            <Text variant="caption" classN={`text-[${colors.danger}]`}>
              {task.disputeText}
            </Text>
          </View>
        )}

        {task.currentStep !== undefined && <ProgressTracker currentStep={task.currentStep} />}

        {task.price && (
          <View
            style={[
              tw`mt-2 flex-row items-center justify-between pt-3`,
              { borderTopWidth: 1, borderTopColor: systemColors.grey5 },
            ]}>
            <Text variant="caption" classN={`text-[${colors.orangeMuted}]`}>
              {task.payIn}
            </Text>
            <Text variant="body" fontWeight="medium" classN={`text-[${fg}]`}>
              {task.price}
            </Text>
          </View>
        )}
      </View>
    </PressableScale>
  );
}

function StatTile({
  icon: Icon,
  label,
  value,
}: {
  icon: LucideIcon;
  label: string;
  value: string;
}) {
  const { colors: systemColors } = useColorScheme();
  const fg = twColor(systemColors.foreground);
  const muted = twColor(systemColors.mutedForeground);

  return (
    <View
      style={[tw`flex-1 rounded-2xl p-3.5`, { backgroundColor: systemColors.card }, CARD_SHADOW]}>
      <View
        style={[
          tw`h-8 w-8 items-center justify-center rounded-full`,
          { backgroundColor: withOpacity(colors.primary, 0.12) },
        ]}>
        <Icon size={16} color={colors.primary} />
      </View>
      <Text variant="subhead" fontWeight="black" classN={`mt-2 text-[${fg}]`}>
        {value}
      </Text>
      <Text variant="micro" fontWeight="normal" classN={`text-[${muted}]`}>
        {label}
      </Text>
    </View>
  );
}

function PostTaskBanner() {
  return (
    <PressableScale
      onPress={() => router.push('/(tabs)/post')}
      style={[
        tw`mt-3 flex-row items-center gap-3 rounded-3xl p-4`,
        { backgroundColor: colors.primary },
      ]}>
      <View
        style={[
          tw`h-11 w-11 items-center justify-center rounded-2xl`,
          { backgroundColor: withOpacity(colors.white, 0.18) },
        ]}>
        <Plus size={22} color="white" />
      </View>
      <View style={tw`flex-1`}>
        <Text variant="body" fontWeight="medium" classN="text-white">
          Post a task
        </Text>
        <Text variant="caption" classN="mt-0.5 text-white opacity-80">
          Tell us what you need done and get offers in minutes
        </Text>
      </View>
      <ChevronRight size={18} color="white" />
    </PressableScale>
  );
}

function QuickCategories() {
  const { colors: systemColors } = useColorScheme();
  const fg = twColor(systemColors.foreground);

  return (
    <View style={tw`mt-5`}>
      <Text variant="caption" fontWeight="medium" classN={`text-[${fg}]`}>
        Need something else?
      </Text>
      <ScrollView
        horizontal
        showsHorizontalScrollIndicator={false}
        contentContainerStyle={tw`mt-2 gap-3 pr-4`}>
        {QUICK_CATEGORIES.map(({ name, label, color, icon: Icon }) => (
          <PressableScale
            key={name}
            onPress={() => router.push({ pathname: '/(tabs)/post', params: { category: name } })}
            style={tw`w-16 items-center gap-1.5`}>
            <View
              style={[
                tw`h-12 w-12 items-center justify-center rounded-2xl`,
                { backgroundColor: withOpacity(color, 0.12) },
              ]}>
              <Icon size={20} color={color} />
            </View>
            <Text fontSize={10.5} classN={`text-center text-[${fg}]`} numberOfLines={1}>
              {label}
            </Text>
          </PressableScale>
        ))}
      </ScrollView>
    </View>
  );
}

export default function Dashboard() {
  const { role, hasChosenRole, setRole } = useRoleStore();
  const outstandingSteps = useOutstandingStepsCount();
  const { colors: systemColors } = useColorScheme();
  const sidekickCurrentTask = useTaskStore((state) => state.sidekickCurrentTask);
  const [activeSheet, setActiveSheet] = React.useState<'offers' | null>(null);
  const isSidekick = role === 'sidekick';
  const fg = twColor(systemColors.foreground);
  const muted = twColor(systemColors.mutedForeground);
  // Text and hairlines sitting on the navy hero.
  const onNavyMuted = twColor(withOpacity(colors.white, 0.72));
  const onNavyBorder = withOpacity(colors.white, 0.45);
  const posted = useMyPostedTasks();
  const user = useAuthStore((state) => state.user);

  console.log(user, 'current user');

  // The Hero list is live; the Sidekick's single active task still comes from
  // the local task store until an "accepted offer" endpoint exists to read it
  // back from the server.
  const postedTasks = React.useMemo(() => posted.tasks.map(toDisplayTask), [posted.tasks]);
  const tasks = isSidekick ? (sidekickCurrentTask ? [sidekickCurrentTask] : []) : postedTasks;

  // Only money actually held in escrow counts — a draft or an open task hasn't
  // been charged yet.
  const totalInEscrow = React.useMemo(
    () =>
      posted.tasks
        .filter((task) => task.escrow_status === 'secured')
        .reduce((sum, task) => sum + (task.total_charge ?? task.budget ?? 0), 0),
    [posted.tasks]
  );

  const offersTask = postedTasks.find((task) => task.status === 'offers');

  const handleTaskPress = (task: Task) => {
    if (task.status === 'offers') {
      setActiveSheet('offers');
      return;
    }
    router.push(`/task/${task.id}`);
  };

  const setRoleIntent = useSetRole();

  const handleRoleSelect = async (selectedRole: UserRole) => {
    // The server owns `role_intent`, so it decides first. Flipping locally and
    // letting the request fail quietly would leave the app showing a role the
    // account doesn't have — and the sheet wouldn't come back to say so.
    try {
      await setRoleIntent.mutateAsync(intentFromRole(selectedRole));
      setRole(selectedRole);
      router.push('/get-started');
    } catch (error) {
      toast.error('Couldn’t save your role', { description: errorMessage(error) });
    }
  };

  return (
    <SafeAreaView style={[tw`flex-1`, { backgroundColor: colors.primary }]}>
      <StatusBar style="light" />
      {/* Navy hero — puts the brand above the fold and gives the screen an anchor. */}
      <View style={tw`px-4 pb-7 pt-2`}>
        <View style={tw`flex-row items-center justify-between`}>
          <PressableScale style={tw`flex-row items-center gap-2`} haptic="selection">
            <View
              style={[
                tw`h-10 w-10 items-center justify-center rounded-full`,
                { backgroundColor: colors.orangeTint },
              ]}>
              <Text variant="body" fontWeight="bold" classN={`text-[${colors.orangeDark}]`}>
                B
              </Text>
            </View>
            <View>
              <View style={tw`flex-row items-center`}>
                <Text variant="body" fontWeight="medium" classN="text-white">
                  {user?.display_name || 'Current User'}
                </Text>
                <ChevronRight size={16} color={onNavyMuted} />
              </View>
              <Text variant="caption" classN={`text-[${onNavyMuted}]`}>
                Lagos Island
              </Text>
            </View>
          </PressableScale>

          <View
            style={{
              borderColor: onNavyBorder,
              ...tw`h-9 w-9 items-center justify-center rounded-full border`,
            }}>
            <Text variant="caption" fontWeight="medium" classN="text-white">
              {isSidekick ? 'S' : 'H'}
            </Text>
          </View>
        </View>

        <Text variant="headline" classN="mt-6 text-white">
          {getGreeting()} 👋
        </Text>
        <Text variant="body" classN={`mt-1.5 text-[${onNavyMuted}]`}>
          {isSidekick ? 'Ready to find your next task?' : 'Ready to get something done today?'}
        </Text>
      </View>

      {/* Light sheet lifts off the hero. */}
      <View
        style={[
          tw`flex-1 overflow-hidden rounded-t-3xl`,
          { backgroundColor: systemColors.background },
        ]}>
        <ScrollView
          style={tw`flex-1 px-4`}
          contentContainerStyle={{ paddingBottom: 110, paddingTop: 22 }}
          showsVerticalScrollIndicator={false}>
          {!isSidekick && (
            <>
              <PostTaskBanner />

              <View style={tw`mt-3 flex-row gap-3`}>
                <StatTile
                  icon={ClipboardList}
                  label="Active Tasks"
                  value={String(postedTasks.length)}
                />
                <StatTile icon={ShieldCheck} label="In Escrow" value={money(totalInEscrow)} />
              </View>

              <QuickCategories />
            </>
          )}

          {hasChosenRole && outstandingSteps > 0 && (
            <PressableScale
              onPress={() => router.push('/get-started')}
              style={[
                tw`mt-5 flex-row items-center gap-3 rounded-2xl p-3.5`,
                { backgroundColor: colors.orangeTint },
              ]}>
              <TriangleAlert size={18} color={colors.orangeDark} />
              <Text
                variant="caption"
                fontWeight="medium"
                classN={`flex-1 text-[${colors.orangeDark}]`}>
                {outstandingSteps} step{outstandingSteps > 1 ? 's' : ''} left to finish setting up
                your {isSidekick ? 'Sidekick' : 'Hero'} account
              </Text>
              <ChevronRight size={16} color={colors.orangeDark} />
            </PressableScale>
          )}

          <View style={tw`mt-6 flex-row items-center justify-between`}>
            <Text variant="subhead" fontWeight="bold" classN={`text-[${fg}]`}>
              {isSidekick ? 'My Current Task' : 'My Posted Tasks'}
            </Text>
            {!isSidekick && tasks.length > 0 && (
              <Text variant="caption" classN={`text-[${muted}]`}>
                {tasks.length} task{tasks.length > 1 ? 's' : ''}
              </Text>
            )}
          </View>

          {isSidekick && tasks.length > 0 && (
            <Text variant="caption" classN={`mt-0.5 text-[${muted}]`}>
              Finish this task before you can accept another
            </Text>
          )}

          {!isSidekick && posted.isPending ? (
            <View style={tw`mt-4`}>
              {[0, 1, 2].map((index) => (
                <SkeletonTaskCard key={index} index={index} />
              ))}
            </View>
          ) : tasks.length === 0 ? (
            isSidekick ? (
              <EmptyState
                compact
                illustration={<OpenSlot />}
                title="Nothing on your plate"
                body="Browse tasks nearby and send an offer — you can hold one job at a time."
                action={{ label: 'Find work', onPress: () => router.push('/(tabs)/post') }}
              />
            ) : (
              <EmptyState
                compact
                illustration={<AwaitingOffers />}
                title="No tasks posted yet"
                body="Post what you need done and nearby Sidekicks will send you offers."
                action={{ label: 'Post a task', onPress: () => router.push('/(tabs)/post') }}
              />
            )
          ) : (
            tasks.map((task, i) => (
              <StaggerItem key={task.id} index={i}>
                <TaskCard
                  task={task}
                  onPress={() => handleTaskPress(task)}
                  showLiveBadge={isSidekick}
                />
              </StaggerItem>
            ))
          )}
        </ScrollView>
      </View>

      {offersTask && (
        <ReviewOffersSheet
          visible={activeSheet === 'offers'}
          onClose={() => setActiveSheet(null)}
          taskTitle={offersTask.title}
          taskId={offersTask.id}
        />
      )}

      {/* Only ask once we know the server doesn't already have an answer.
          While `user` is null the session is still loading, and showing the
          sheet then would make a returning user pick a side they already chose. */}
      <RoleSelectSheet
        visible={!!user && !hasChosenRole}
        onSelect={handleRoleSelect}
        loading={setRoleIntent.isPending}
      />
    </SafeAreaView>
  );
}
