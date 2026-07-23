import { ChevronRight } from 'lucide-react-native';
import * as React from 'react';
import { Pressable, ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import Text from '@/components/UI/Text';
import { ConfirmTaskSheet, DisputeSheet, ReviewOffersSheet } from '@/components/UI/TaskSheets';
import { tw } from '@/lib/tw';

const ACCENT_TEAL = '#489A9F';
const CATEGORY_COLOR = '#D1573B';
const MUTED = '#9AA0A6';

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
                    ? { borderWidth: 2, borderColor: ACCENT_TEAL, backgroundColor: 'white' }
                    : { borderWidth: 1.5, borderColor: '#D9DCE0', backgroundColor: 'white' },
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
                  { backgroundColor: index < currentStep ? ACCENT_TEAL : '#D9DCE0' },
                ]}
              />
            )}
          </React.Fragment>
        ))}
      </View>
      <View style={tw`mt-1 flex-row`}>
        {STEPS.map((label) => (
          <Text key={label} fontSize={9} classN="w-11 text-center text-[#9AA0A6]">
            {label}
          </Text>
        ))}
      </View>
    </View>
  );
}

function TaskCard({ task, onPress }: { task: Task; onPress?: () => void }) {
  return (
    <Pressable
      onPress={onPress}
      disabled={!onPress}
      style={[tw`mt-4 rounded-2xl bg-white p-4`, CARD_SHADOW]}>
      <View style={tw`flex-row items-center justify-between`}>
        <Text fontWeight="bold" fontSize={11} classN={`text-[${CATEGORY_COLOR}]`}>
          {task.category.toUpperCase()}
        </Text>
        <StatusPill status={task.status} label={task.statusLabel} />
      </View>

      <Text fontWeight="bold" fontSize={16} classN="mt-1 text-black">
        {task.title}
      </Text>
      <Text fontSize={12} classN="mt-0.5 text-[#9AA0A6]">
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
            { borderTopWidth: 1, borderTopColor: '#F0F1F2' },
          ]}>
          <Text fontSize={12} classN="text-[#D97A55]">
            {task.payIn}
          </Text>
          <Text fontWeight="bold" fontSize={15} classN="text-black">
            {task.price}
          </Text>
        </View>
      )}
    </Pressable>
  );
}

export default function Dashboard() {
  const [activeTab, setActiveTab] = React.useState<'posted' | 'mine'>('posted');
  const [activeSheet, setActiveSheet] = React.useState<'confirm' | 'dispute' | 'offers' | null>(
    null
  );
  const tasks = activeTab === 'posted' ? POSTED_TASKS : [];

  const confirmTask = POSTED_TASKS.find((task) => task.status === 'in_progress');
  const offersTask = POSTED_TASKS.find((task) => task.status === 'offers');

  const handleTaskPress = (task: Task) => {
    if (task.status === 'in_progress') setActiveSheet('confirm');
    if (task.status === 'offers') setActiveSheet('offers');
  };

  return (
    <SafeAreaView style={tw`flex-1 bg-[#FFFFFF]`}>
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
              <Text fontWeight="bold" fontSize={15} classN="text-black">
                Bisola Soks
              </Text>
              <ChevronRight size={16} color={MUTED} />
            </View>
            <Text fontSize={12} classN="text-[#9AA0A6]">
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
            H
          </Text>
        </View>
      </View>

      <View style={[tw`mx-4 mt-4 flex-row rounded-[15px] p-1`, { backgroundColor: '#E7E9EB' }]}>
        <Pressable
          onPress={() => setActiveTab('posted')}
          style={[
            tw`flex-1 items-center rounded-[10px] py-2`,
            activeTab === 'posted' && { backgroundColor: ACCENT_TEAL },
          ]}>
          <Text
            fontWeight="medium"
            fontSize={13}
            classN={activeTab === 'posted' ? 'text-white' : 'text-[#6B7075]'}>
            My Posted Tasks
          </Text>
        </Pressable>
        <Pressable
          onPress={() => setActiveTab('mine')}
          style={[
            tw`flex-1 items-center rounded-[10px] py-2`,
            activeTab === 'mine' && { backgroundColor: ACCENT_TEAL },
          ]}>
          <Text
            fontWeight="medium"
            fontSize={13}
            classN={activeTab === 'mine' ? 'text-white' : 'text-[#6B7075]'}>
            My Tasks
          </Text>
        </Pressable>
      </View>

      <ScrollView
        style={tw`flex-1 px-4`}
        contentContainerStyle={{ paddingBottom: 110 }}
        showsVerticalScrollIndicator={false}>
        {tasks.length === 0 ? (
          <View style={tw`mt-16 items-center`}>
            <Text fontSize={14} classN="text-[#9AA0A6]">
              No tasks yet
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
    </SafeAreaView>
  );
}
