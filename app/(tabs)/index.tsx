import { router } from 'expo-router';
import { ChevronRight, TriangleAlert } from 'lucide-react-native';
import * as React from 'react';
import { Pressable, ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import RoleSelectSheet from '@/components/UI/RoleSelectSheet';
import Text from '@/components/UI/Text';
import { ConfirmTaskSheet, DisputeSheet, ReviewOffersSheet } from '@/components/UI/TaskSheets';
import { tw, twColor } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';
import { useOutstandingStepsCount, useRoleStore, type UserRole } from '@/store/roleStore';

const ACCENT_TEAL = '#489A9F';
const CATEGORY_COLOR = '#D1573B';

const CARD_SHADOW = {
  shadowColor: '#000',
  shadowOpacity: 0.06,
  shadowRadius: 8,
  shadowOffset: { width: 0, height: 2 },
  elevation: 2,
};

type TaskStatus = 'in_progress' | 'offers' | 'scheduled' | 'dispute';

type Task = {
  id: string;
  category: string;
  status: TaskStatus;
  statusLabel: string;
  title: string;
  meta: string;
  currentStep?: number;
  offersText?: string;
  disputeText?: string;
  payIn?: string;
  price?: string;
  sidekickName?: string;
  offerCount?: number;
};

const POSTED_TASKS: Task[] = [
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
  },
  {
    id: '4',
    category: 'Tech Help',
    status: 'dispute',
    statusLabel: 'Dispute',
    title: 'Laptop battery repair',
    meta: 'Saturday . 12:00pm',
    disputeText: 'Admin reviewing your dispute. Escrow frozen. Resolution in 24–48h.',
  },
];

const STATUS_STYLES: Record<TaskStatus, { bg: string; text: string }> = {
  in_progress: { bg: '#F5E6D3', text: '#B5762E' },
  offers: { bg: '#DCF5E3', text: '#2F9E56' },
  scheduled: { bg: '#DCE8FB', text: '#3B6FD1' },
  dispute: { bg: '#FBE2DC', text: '#D1573B' },
};

const STEPS = ['Posted', 'Matched', 'In Progress', 'Confirm', 'Payment'];

function StatusPill({ status, label }: { status: TaskStatus; label: string }) {
  const { bg, text } = STATUS_STYLES[status];
  return (
    <View style={[tw`rounded-full px-2.5 py-1`, { backgroundColor: bg }]}>
      <Text fontSize={11} classN={`text-[${text}]`}>
        {label}
      </Text>
    </View>
  );
}

function ProgressTracker({ currentStep }: { currentStep: number }) {
  const { colors } = useColorScheme();
  const muted = twColor(colors.mutedForeground);

  return (
    <View style={tw`mt-3`}>
      <View style={tw`flex-row items-center`}>
        {STEPS.map((_, index) => (
          <React.Fragment key={index}>
            <View
              style={[
                tw`h-5 w-5 items-center justify-center rounded-full`,
                index < currentStep
                  ? { backgroundColor: ACCENT_TEAL }
                  : index === currentStep
                    ? { borderWidth: 2, borderColor: ACCENT_TEAL, backgroundColor: colors.card }
                    : { borderWidth: 1.5, borderColor: colors.grey4, backgroundColor: colors.card },
              ]}>
              {index < currentStep ? (
                <View style={tw`h-1.5 w-1.5 rounded-full bg-white`} />
              ) : index === currentStep ? (
                <View style={[tw`h-1.5 w-1.5 rounded-full`, { backgroundColor: ACCENT_TEAL }]} />
              ) : null}
            </View>
            {index < STEPS.length - 1 && (
              <View
                style={[
                  tw`h-px flex-1`,
                  { backgroundColor: index < currentStep ? ACCENT_TEAL : colors.grey4 },
                ]}
              />
            )}
          </React.Fragment>
        ))}
      </View>
      <View style={tw`mt-1 flex-row`}>
        {STEPS.map((label) => (
          <Text key={label} fontSize={9} classN={`w-11 text-center text-[${muted}]`}>
            {label}
          </Text>
        ))}
      </View>
    </View>
  );
}

function TaskCard({ task, onPress }: { task: Task; onPress?: () => void }) {
  const { colors } = useColorScheme();
  const fg = twColor(colors.foreground);
  const muted = twColor(colors.mutedForeground);

  return (
    <Pressable
      onPress={onPress}
      disabled={!onPress}
      style={[tw`mt-4 rounded-2xl p-4`, { backgroundColor: colors.card }, CARD_SHADOW]}>
      <View style={tw`flex-row items-center justify-between`}>
        <Text fontWeight="bold" fontSize={11} classN={`text-[${CATEGORY_COLOR}]`}>
          {task.category.toUpperCase()}
        </Text>
        <StatusPill status={task.status} label={task.statusLabel} />
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
    </Pressable>
  );
}

export default function Dashboard() {
  const { role, hasChosenRole, setRole } = useRoleStore();
  const outstandingSteps = useOutstandingStepsCount();
  const { colors } = useColorScheme();
  const [activeSheet, setActiveSheet] = React.useState<'confirm' | 'dispute' | 'offers' | null>(
    null
  );
  const isSidekick = role === 'sidekick';
  const fg = twColor(colors.foreground);
  const muted = twColor(colors.mutedForeground);
  const tasks = isSidekick ? [] : POSTED_TASKS;

  const confirmTask = POSTED_TASKS.find((task) => task.status === 'in_progress');
  const offersTask = POSTED_TASKS.find((task) => task.status === 'offers');

  const handleTaskPress = (task: Task) => {
    if (task.status === 'in_progress') setActiveSheet('confirm');
    if (task.status === 'offers') setActiveSheet('offers');
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

      <Text fontWeight="bold" fontSize={16} classN={`mx-4 mt-5 text-[${fg}]`}>
        {isSidekick ? 'My Tasks' : 'My Posted Tasks'}
      </Text>

      {hasChosenRole && outstandingSteps > 0 && (
        <Pressable
          onPress={() => router.push('/get-started')}
          style={[
            tw`mx-4 mt-3 flex-row items-center gap-3 rounded-2xl p-3.5`,
            { backgroundColor: '#F5E6D3' },
          ]}>
          <TriangleAlert size={18} color="#B5762E" />
          <Text fontWeight="medium" fontSize={13} classN="flex-1 text-[#B5762E]">
            {outstandingSteps} step{outstandingSteps > 1 ? 's' : ''} left to finish setting up your{' '}
            {isSidekick ? 'Sidekick' : 'Hero'} account
          </Text>
          <ChevronRight size={16} color="#B5762E" />
        </Pressable>
      )}

      <ScrollView
        style={tw`flex-1 px-4`}
        contentContainerStyle={{ paddingBottom: 110 }}
        showsVerticalScrollIndicator={false}>
        {tasks.length === 0 ? (
          <View style={tw`mt-16 items-center`}>
            <Text fontSize={14} classN={`text-[${muted}]`}>
              {isSidekick ? 'No active tasks yet' : 'No tasks yet'}
            </Text>
          </View>
        ) : (
          tasks.map((task) => (
            <TaskCard key={task.id} task={task} onPress={() => handleTaskPress(task)} />
          ))
        )}
      </ScrollView>

      {confirmTask && (
        <ConfirmTaskSheet
          visible={activeSheet === 'confirm'}
          onClose={() => setActiveSheet(null)}
          onReportDispute={() => setActiveSheet('dispute')}
          sidekickName={confirmTask.sidekickName ?? 'Your Sidekick'}
          price={confirmTask.price ?? ''}
        />
      )}

      <DisputeSheet
        visible={activeSheet === 'dispute'}
        onClose={() => setActiveSheet(null)}
        onSubmit={() => setActiveSheet(null)}
      />

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
