import { router } from 'expo-router';
import {
  ChevronRight,
  ClipboardList,
  Plus,
  ShieldCheck,
  TriangleAlert,
  type LucideIcon,
} from 'lucide-react-native';
import * as React from 'react';
import { Pressable, ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import RoleSelectSheet from '@/components/UI/RoleSelectSheet';
import Text from '@/components/UI/Text';
import { ReviewOffersSheet } from '@/components/UI/TaskSheets';
import {
  ACCENT_TEAL,
  CARD_SHADOW,
  ProgressTracker,
  QUICK_CATEGORIES,
  StatusPill,
  formatNaira,
  getCategoryMeta,
  parseNaira,
  type Task,
} from '@/components/UI/TaskParts';
import { LiveDot } from '@/components/UI/TaskTimeline';
import { tw, twColor } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';
import { useOutstandingStepsCount, useRoleStore, type UserRole } from '@/store/roleStore';
import { useTaskStore } from '@/store/taskStore';
import { withOpacity } from '@/theme/with-opacity';

export const POSTED_TASKS: Task[] = [
  {
    id: '1',
    category: 'Cleaning',
    status: 'in_progress',
    statusLabel: 'In Progress',
    title: 'Deep clean 2-bedroom apartment',
    meta: 'Ade B. started 2h ago • Awaiting completion',
    currentStep: 3,
    payIn: 'Pay in 3hours',
    price: '₦50,000',
    sidekickName: 'Ade B.',
    description: 'Deep clean of a 2-bedroom apartment, including kitchen and bathrooms.',
    etaWindow: '3 - 4 hrs',
  },
  {
    id: '2',
    category: 'Delivery',
    status: 'offers',
    statusLabel: '3 Offers',
    title: 'Pick up groceries from shoprite',
    meta: 'Posted 12mins ago. Lekki Phase 1',
    offersText: '3 Sidekicks made offers. Tap to view',
    offerCount: 4,
  },
  {
    id: '3',
    category: 'Tech Help',
    status: 'scheduled',
    statusLabel: 'Scheduled',
    title: 'Laptop battery repair',
    meta: 'Saturday . 12:00pm',
    currentStep: 1,
    payIn: 'Pay in 3days',
    price: '₦50,000',
    sidekickName: 'Chidi O.',
    description: 'Replace a worn-out laptop battery and check charging port.',
    etaWindow: 'Same day, within 3 hrs',
  },
  {
    id: '4',
    category: 'Tech Help',
    status: 'dispute',
    statusLabel: 'Dispute',
    title: 'Laptop battery repair',
    meta: 'Saturday . 12:00pm',
    disputeText: 'Admin reviewing your dispute. Escrow frozen. Resolution in 24–48h.',
    sidekickName: 'Chidi O.',
    description: 'Replace a worn-out laptop battery and check charging port.',
    etaWindow: 'Same day, within 3 hrs',
  },
];

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
  const { colors } = useColorScheme();
  const fg = twColor(colors.foreground);
  const muted = twColor(colors.mutedForeground);
  const { color: categoryColor, icon: CategoryIcon } = getCategoryMeta(task.category);

  return (
    <Pressable
      onPress={onPress}
      disabled={!onPress}
      style={[
        tw`mt-4 flex-row overflow-hidden rounded-2xl`,
        { backgroundColor: colors.card },
        CARD_SHADOW,
      ]}>
      <View style={{ width: 4, backgroundColor: categoryColor }} />
      <View style={tw`flex-1 p-4`}>
        <View style={tw`flex-row items-center justify-between`}>
          <View style={tw`flex-row items-center gap-1.5`}>
            <CategoryIcon size={13} color={categoryColor} />
            <Text fontWeight="bold" fontSize={11} classN={`text-[${categoryColor}]`}>
              {task.category.toUpperCase()}
            </Text>
          </View>
          <View style={tw`flex-row items-center gap-2`}>
            {showLiveBadge && task.status === 'in_progress' && <LiveDot />}
            <StatusPill status={task.status} label={task.statusLabel} />
          </View>
        </View>

        <Text fontWeight="bold" fontSize={16} classN={`mt-1 text-[${fg}]`}>
          {task.title}
        </Text>
        <Text fontSize={12} classN={`mt-0.5 text-[${muted}]`}>
          {task.meta}
        </Text>

        {task.offersText && (
          <View style={[tw`mt-3 rounded-xl px-3 py-2`, { backgroundColor: '#DCF5E3' }]}>
            <Text fontSize={12} classN="text-[#2F9E56]">
              {task.offersText}
            </Text>
          </View>
        )}

        {task.disputeText && (
          <View style={[tw`mt-3 rounded-xl px-3 py-2`, { backgroundColor: '#FBE2DC' }]}>
            <Text fontSize={12} classN="text-[#D1573B]">
              {task.disputeText}
            </Text>
          </View>
        )}

        {task.currentStep !== undefined && <ProgressTracker currentStep={task.currentStep} />}

        {task.price && (
          <View
            style={[
              tw`mt-3 flex-row items-center justify-between pt-3`,
              { borderTopWidth: 1, borderTopColor: colors.grey5 },
            ]}>
            <Text fontSize={12} classN="text-[#D97A55]">
              {task.payIn}
            </Text>
            <Text fontWeight="bold" fontSize={15} classN={`text-[${fg}]`}>
              {task.price}
            </Text>
          </View>
        )}
      </View>
    </Pressable>
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
  const { colors } = useColorScheme();
  const fg = twColor(colors.foreground);
  const muted = twColor(colors.mutedForeground);

  return (
    <View style={[tw`flex-1 rounded-2xl p-3.5`, { backgroundColor: colors.card }, CARD_SHADOW]}>
      <View
        style={[
          tw`h-8 w-8 items-center justify-center rounded-full`,
          { backgroundColor: withOpacity(ACCENT_TEAL, 0.12) },
        ]}>
        <Icon size={16} color={ACCENT_TEAL} />
      </View>
      <Text fontWeight="black" fontSize={17} classN={`mt-2 text-[${fg}]`}>
        {value}
      </Text>
      <Text fontSize={11} classN={`text-[${muted}]`}>
        {label}
      </Text>
    </View>
  );
}

function PostTaskBanner() {
  return (
    <Pressable
      onPress={() => router.push('/(tabs)/post')}
      style={[
        tw`mt-4 flex-row items-center gap-3 rounded-3xl p-4`,
        { backgroundColor: ACCENT_TEAL },
      ]}>
      <View
        style={[
          tw`h-11 w-11 items-center justify-center rounded-2xl`,
          { backgroundColor: withOpacity('#FFFFFF', 0.18) },
        ]}>
        <Plus size={22} color="white" />
      </View>
      <View style={tw`flex-1`}>
        <Text fontWeight="bold" fontSize={15} classN="text-white">
          Post a task
        </Text>
        <Text fontSize={12} classN="mt-0.5 text-white opacity-80">
          Tell us what you need done and get offers in minutes
        </Text>
      </View>
      <ChevronRight size={18} color="white" />
    </Pressable>
  );
}

function QuickCategories() {
  const { colors } = useColorScheme();
  const fg = twColor(colors.foreground);

  return (
    <View style={tw`mt-5`}>
      <Text fontWeight="bold" fontSize={13} classN={`text-[${fg}]`}>
        Need something else?
      </Text>
      <ScrollView
        horizontal
        showsHorizontalScrollIndicator={false}
        contentContainerStyle={tw`mt-3 gap-3 pr-4`}>
        {QUICK_CATEGORIES.map(({ name, label, color, icon: Icon }) => (
          <Pressable
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
          </Pressable>
        ))}
      </ScrollView>
    </View>
  );
}

export default function Dashboard() {
  const { role, hasChosenRole, setRole } = useRoleStore();
  const outstandingSteps = useOutstandingStepsCount();
  const { colors } = useColorScheme();
  const sidekickCurrentTask = useTaskStore((state) => state.sidekickCurrentTask);
  const [activeSheet, setActiveSheet] = React.useState<'offers' | null>(null);
  const isSidekick = role === 'sidekick';
  const fg = twColor(colors.foreground);
  const muted = twColor(colors.mutedForeground);
  const tasks = isSidekick ? (sidekickCurrentTask ? [sidekickCurrentTask] : []) : POSTED_TASKS;
  const firstName = 'Bisola';

  const totalInEscrow = POSTED_TASKS.reduce((sum, task) => sum + parseNaira(task.price), 0);

  const offersTask = POSTED_TASKS.find((task) => task.status === 'offers');

  const handleTaskPress = (task: Task) => {
    if (task.status === 'offers') {
      setActiveSheet('offers');
      return;
    }
    router.push(`/task/${task.id}`);
  };

  const handleRoleSelect = (selectedRole: UserRole) => {
    setRole(selectedRole);
    router.push('/get-started');
  };

  return (
    <SafeAreaView style={[tw`flex-1`, { backgroundColor: colors.background }]}>
      <View style={tw`flex-row items-center justify-between px-4 pt-2`}>
        <Pressable style={tw`flex-row items-center gap-2`}>
          <View
            style={[
              tw`h-10 w-10 items-center justify-center rounded-full`,
              { backgroundColor: '#F0DCC8' },
            ]}>
            <Text fontWeight="bold" fontSize={15} classN="text-[#B5762E]">
              B
            </Text>
          </View>
          <View>
            <View style={tw`flex-row items-center`}>
              <Text fontWeight="bold" fontSize={15} classN={`text-[${fg}]`}>
                Bisola Soks
              </Text>
              <ChevronRight size={16} color={colors.mutedForeground} />
            </View>
            <Text fontSize={12} classN={`text-[${muted}]`}>
              Lagos Island
            </Text>
          </View>
        </Pressable>

        <View
          style={{
            borderColor: ACCENT_TEAL,
            ...tw`h-9 w-9 items-center justify-center rounded-full border`,
          }}>
          <Text fontWeight="bold" fontSize={13} classN={`text-[${ACCENT_TEAL}]`}>
            {isSidekick ? 'S' : 'H'}
          </Text>
        </View>
      </View>

      <ScrollView
        style={tw`flex-1 px-4`}
        contentContainerStyle={{ paddingBottom: 110 }}
        showsVerticalScrollIndicator={false}>
        <Text fontWeight="black" fontSize={22} classN={`mt-5 text-[${fg}]`}>
          {getGreeting()}, {firstName} 👋
        </Text>
        <Text fontSize={13} classN={`mt-1 text-[${muted}]`}>
          {isSidekick ? 'Ready to find your next task?' : 'Ready to get something done today?'}
        </Text>

        {!isSidekick && (
          <>
            <PostTaskBanner />

            <View style={tw`mt-4 flex-row gap-3`}>
              <StatTile
                icon={ClipboardList}
                label="Active Tasks"
                value={String(POSTED_TASKS.length)}
              />
              <StatTile icon={ShieldCheck} label="In Escrow" value={formatNaira(totalInEscrow)} />
            </View>

            <QuickCategories />
          </>
        )}

        {hasChosenRole && outstandingSteps > 0 && (
          <Pressable
            onPress={() => router.push('/get-started')}
            style={[
              tw`mt-5 flex-row items-center gap-3 rounded-2xl p-3.5`,
              { backgroundColor: '#F5E6D3' },
            ]}>
            <TriangleAlert size={18} color="#B5762E" />
            <Text fontWeight="medium" fontSize={13} classN="flex-1 text-[#B5762E]">
              {outstandingSteps} step{outstandingSteps > 1 ? 's' : ''} left to finish setting up
              your {isSidekick ? 'Sidekick' : 'Hero'} account
            </Text>
            <ChevronRight size={16} color="#B5762E" />
          </Pressable>
        )}

        <View style={tw`mt-6 flex-row items-center justify-between`}>
          <Text fontWeight="bold" fontSize={16} classN={`text-[${fg}]`}>
            {isSidekick ? 'My Current Task' : 'My Posted Tasks'}
          </Text>
          {!isSidekick && tasks.length > 0 && (
            <Text fontSize={12} classN={`text-[${muted}]`}>
              {tasks.length} task{tasks.length > 1 ? 's' : ''}
            </Text>
          )}
        </View>

        {isSidekick && tasks.length > 0 && (
          <Text fontSize={12} classN={`mt-0.5 text-[${muted}]`}>
            Finish this task before you can accept another
          </Text>
        )}

        {tasks.length === 0 ? (
          <View style={tw`mt-10 items-center rounded-2xl p-8`}>
            <View
              style={[
                tw`h-14 w-14 items-center justify-center rounded-full`,
                { backgroundColor: withOpacity(ACCENT_TEAL, 0.12) },
              ]}>
              <ClipboardList size={24} color={ACCENT_TEAL} />
            </View>
            <Text fontWeight="medium" fontSize={14} classN={`mt-3 text-[${fg}]`}>
              {isSidekick ? 'No active tasks yet' : 'No tasks yet'}
            </Text>
            <Text fontSize={12} classN={`mt-1 text-center text-[${muted}]`}>
              {isSidekick
                ? 'Browse the Post tab to find tasks nearby'
                : 'Post your first task and get offers from nearby Sidekicks'}
            </Text>
            {!isSidekick && (
              <Pressable
                onPress={() => router.push('/(tabs)/post')}
                style={[
                  tw`mt-4 flex-row items-center gap-2 rounded-full px-5 py-3`,
                  { backgroundColor: ACCENT_TEAL },
                ]}>
                <Plus size={16} color="white" />
                <Text fontWeight="bold" fontSize={13} classN="text-white">
                  Post a task
                </Text>
              </Pressable>
            )}
          </View>
        ) : (
          tasks.map((task) => (
            <TaskCard
              key={task.id}
              task={task}
              onPress={() => handleTaskPress(task)}
              showLiveBadge={isSidekick}
            />
          ))
        )}
      </ScrollView>

      {offersTask && (
        <ReviewOffersSheet
          visible={activeSheet === 'offers'}
          onClose={() => setActiveSheet(null)}
          taskTitle={offersTask.title}
          offerCount={offersTask.offerCount}
        />
      )}

      <RoleSelectSheet visible={!hasChosenRole} onSelect={handleRoleSelect} />
    </SafeAreaView>
  );
}
