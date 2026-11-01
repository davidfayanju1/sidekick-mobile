import { useActionSheet } from '@expo/react-native-action-sheet';
import DateTimePicker, {
  DateTimePickerAndroid,
  type DateTimePickerEvent,
} from '@react-native-community/datetimepicker';
import * as ImagePicker from 'expo-image-picker';
import { router, useLocalSearchParams } from 'expo-router';
import {
  Calendar,
  Check,
  ChevronDown,
  ChevronLeft,
  Heart,
  Image as ImageIcon,
  ImageOff,
  Upload,
  X,
} from 'lucide-react-native';
import * as React from 'react';
import { Image, Platform, ScrollView, TextInput, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import BottomSheet from '@/components/UI/BottomSheet';
import Text from '@/components/UI/Text';
import {
  EscrowPaymentSheet,
  PaymentSuccessSheet,
  TaskSummarySheet,
} from '@/components/UI/TaskSheets';

import { TaskSwipeCard, type TaskSwipeCardHandle } from '@/components/UI/TaskSwipeCard';
import { EmptyState, ErrorState } from '@/components/UI/EmptyState';
import { NothingNearby, OpenSlot } from '@/components/UI/illustrations';
import { SkeletonTaskCard } from '@/components/UI/Skeleton';
import { Spinner } from '@/components/UI/Spinner';
import { toBrowsableTask } from '@/lib/api/adapters';
import { usePaymentMethods } from '@/lib/api/finance';
import { useTrackPostedTask } from '@/lib/api/myTasks';
import { useCreateTask, useFeeQuote, useFundTask } from '@/lib/api/tasks';
import { money } from '@/lib/format';
import { locale } from '@/lib/locale';
import { flattenFeed, useFeed } from '@/lib/api/discovery';
import { useMakeOffer } from '@/lib/api/offers';
import { errorMessage } from '@/lib/axios';
import { geocode, useLocation } from '@/lib/useLocation';
import { tw, twColor } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';
import { useRoleStore } from '@/store/roleStore';
import { useTaskStore } from '@/store/taskStore';
import { toast } from '@/store/toastStore';
import { colors } from '@/theme/palette';
import { PressableScale } from '@/components/UI/PressableScale';

const MAX_PHOTOS = 3;

type WizardStep = 1 | 2;
type ActiveSheet = 'summary' | 'escrow' | 'success' | null;

function isSameDay(a: Date, b: Date) {
  return a.toDateString() === b.toDateString();
}

function formatDateLabel(date: Date) {
  const today = new Date();
  if (isSameDay(date, today)) return 'Today';

  const tomorrow = new Date(today);
  tomorrow.setDate(today.getDate() + 1);
  if (isSameDay(date, tomorrow)) return 'Tomorrow';

  return date.toLocaleDateString('en-US', { weekday: 'short', month: 'short', day: 'numeric' });
}

function formatTimeLabel(date: Date) {
  return date
    .toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit' })
    .replace(' ', '')
    .toLowerCase();
}

const CATEGORIES = [
  'Cleaning',
  'Delivery',
  'Tech Help',
  'Moving & Assembly',
  'Handyman',
  'Personal Errands',
  'Other',
];

function FieldLabel({ children }: { children: string }) {
  return (
    <Text variant="caption" fontWeight="medium" classN="text-black">
      {children}
    </Text>
  );
}

function BrowseTasks() {
  const { colors: systemColors } = useColorScheme();
  const currentTask = useTaskStore((state) => state.sidekickCurrentTask);
  const activeCardRef = React.useRef<TaskSwipeCardHandle>(null);

  // Browsing is the screen that visibly needs a location, so it's the one that
  // asks for it. Until a fix arrives the feed is unfiltered — the server only
  // sorts and filters by distance when lat/lng are present — so this widens
  // from "every open task" to "tasks near you" rather than gating the screen.
  const { coords } = useLocation({ request: true });

  const feed = useFeed(coords ? { limit: 20, lat: coords.lat, lng: coords.lng } : { limit: 20 });
  const cards = React.useMemo(() => flattenFeed(feed.data?.pages), [feed.data]);

  // Passed cards are remembered for the session only — the server has no
  // "hide this from me" concept, so a refetch would otherwise bring them back.
  const [passed, setPassed] = React.useState<string[]>([]);
  const [offered, setOffered] = React.useState<string[]>([]);

  const queue = React.useMemo(
    () =>
      cards
        .filter((card) => !passed.includes(card.id) && !offered.includes(card.id))
        .map(toBrowsableTask),
    [cards, passed, offered]
  );

  const makeOffer = useMakeOffer(queue[0]?.id ?? '');

  const handleSwiped = async (direction: 'left' | 'right') => {
    const current = queue[0];
    if (!current) return;

    if (direction === 'left') {
      setPassed((previous) => [...previous, current.id]);
      return;
    }

    // Swiping right sends an offer at the poster's budget — it does not assign
    // the task. Only the poster can accept, via POST /offers/{id}/accept.
    setOffered((previous) => [...previous, current.id]);
    try {
      await makeOffer.mutateAsync({});
      toast.success('Offer sent', {
        description: `${current.heroName} will be notified. You’ll hear back if they accept.`,
      });
    } catch (error) {
      setOffered((previous) => previous.filter((id) => id !== current.id));
      toast.error('Couldn’t send that offer', { description: errorMessage(error) });
    }
  };

  // Pull the next page in before the deck runs dry, so it never stalls mid-swipe.
  React.useEffect(() => {
    if (queue.length <= 3 && feed.hasNextPage && !feed.isFetchingNextPage) {
      feed.fetchNextPage();
    }
  }, [queue.length, feed]);

  if (currentTask) {
    return (
      <SafeAreaView style={[tw`flex-1`, { backgroundColor: systemColors.background }]}>
        <EmptyState
          illustration={<OpenSlot />}
          title="You already have a task in progress"
          body={`Finish “${currentTask.title}” before you take on another one.`}
          action={{
            label: 'View current task',
            onPress: () => router.push(`/task/${currentTask.id}`),
          }}
        />
      </SafeAreaView>
    );
  }

  return (
    <SafeAreaView style={[tw`flex-1`, { backgroundColor: systemColors.background }]}>
      <View style={tw`px-4 pt-2`}>
        <Text
          variant="title"
          fontWeight="black"
          classN={`text-[${twColor(systemColors.foreground)}]`}>
          Browse tasks nearby
        </Text>
        <Text variant="caption" classN={`mt-1 text-[${twColor(systemColors.mutedForeground)}]`}>
          Swipe right to offer, left to pass
        </Text>
      </View>

      <View style={tw`flex-1 px-4 pb-2 pt-5`}>
        {feed.isPending ? (
          <View style={tw`gap-3`}>
            {[0, 1, 2].map((index) => (
              <SkeletonTaskCard key={index} index={index} />
            ))}
          </View>
        ) : feed.isError ? (
          <ErrorState body={errorMessage(feed.error)} onRetry={() => feed.refetch()} />
        ) : queue.length === 0 ? (
          <EmptyState
            illustration={<NothingNearby />}
            title={passed.length > 0 ? 'That’s everything nearby' : 'No tasks nearby yet'}
            body={
              passed.length > 0
                ? 'You’ve seen every open task in your area. New ones appear here as they’re posted.'
                : 'Nothing has been posted around you yet. Check back shortly.'
            }
            action={{ label: 'Refresh', onPress: () => feed.refetch() }}
          />
        ) : (
          queue
            .slice(0, 3)
            .map((task, index) => (
              <TaskSwipeCard
                key={task.id}
                ref={index === 0 ? activeCardRef : undefined}
                task={task}
                active={index === 0}
                stackDepth={index}
                onSwiped={handleSwiped}
              />
            ))
        )}
      </View>

      {queue.length > 0 && (
        <View style={[tw`flex-row items-center justify-center gap-6 pt-4`, { paddingBottom: 110 }]}>
          <PressableScale
            onPress={() => activeCardRef.current?.swipeLeft()}
            style={[
              tw`h-14 w-14 items-center justify-center rounded-full`,
              {
                backgroundColor: systemColors.card,
                borderWidth: 1,
                borderColor: systemColors.grey5,
              },
            ]}>
            <X size={26} color={colors.danger} />
          </PressableScale>
          <PressableScale
            onPress={() => activeCardRef.current?.swipeRight()}
            disabled={makeOffer.isPending}
            style={[
              tw`h-16 w-16 items-center justify-center rounded-full`,
              { backgroundColor: colors.primary },
            ]}>
            {makeOffer.isPending ? (
              <Spinner size={24} color={colors.white} />
            ) : (
              <Heart size={26} color="white" fill="white" />
            )}
          </PressableScale>
        </View>
      )}
    </SafeAreaView>
  );
}

export default function Post() {
  const role = useRoleStore((state) => state.role);
  if (role === 'sidekick') return <BrowseTasks />;
  return <PostTaskWizard />;
}

function PostTaskWizard() {
  const { showActionSheetWithOptions } = useActionSheet();
  const { category: categoryParam } = useLocalSearchParams<{ category?: string }>();
  const initialCategory = CATEGORIES.includes(categoryParam ?? '') ? (categoryParam ?? null) : null;
  const [step, setStep] = React.useState<WizardStep>(1);
  const [title, setTitle] = React.useState('');
  const [category, setCategory] = React.useState<string | null>(initialCategory);
  const [description, setDescription] = React.useState('');
  const [budget, setBudget] = React.useState('');
  const [photos, setPhotos] = React.useState<string[]>([]);
  const [categorySheetVisible, setCategorySheetVisible] = React.useState(false);

  const [from, setFrom] = React.useState('');
  const [to, setTo] = React.useState('');
  const [dateTime, setDateTime] = React.useState<Date | null>(null);
  const [pendingDate, setPendingDate] = React.useState(new Date());
  const [dateSheetVisible, setDateSheetVisible] = React.useState(false);

  const [activeSheet, setActiveSheet] = React.useState<ActiveSheet>(null);
  // Set once the draft exists server-side, so a failed payment can be retried
  // without creating a second task.
  const [draftTaskId, setDraftTaskId] = React.useState<string | null>(null);

  const createTask = useCreateTask();
  const fundTask = useFundTask(draftTaskId ?? '');
  const trackPostedTask = useTrackPostedTask();
  const paymentMethods = usePaymentMethods();

  const budgetValue = Number(budget);
  const budgetMinor = Math.round(budgetValue * locale.minorUnits);

  // The platform fee is quoted by the server — the rule can change without an
  // app release, so never compute it on the client.
  const quote = useFeeQuote(budgetMinor);
  const showBudgetWarning = budget.length > 0 && budgetValue > 0 && budgetValue < 500;
  const canContinueStep1 = Boolean(title && category && description && budget);
  const canContinueStep2 = Boolean(from && to && dateTime);

  const handlePay = async () => {
    if (!dateTime || !category) return;

    const defaultMethod =
      paymentMethods.data?.find((method) => method.is_default) ?? paymentMethods.data?.[0];

    if (!defaultMethod) {
      toast.warning('Add a payment method first', {
        description: 'Your budget is held in escrow until the task is confirmed.',
        action: { label: 'Add card', onPress: () => router.push('/payment-method') },
      });
      return;
    }

    try {
      let taskId = draftTaskId;

      if (!taskId) {
        // Without coordinates the task never appears in a located Sidekick's
        // feed — the geo query drops rows with a null `location_lat`. Both
        // lookups are best-effort and run together rather than in sequence.
        const [approxAt, exactAt] = await Promise.all([geocode(from), geocode(to)]);

        const task = await createTask.mutateAsync({
          title,
          description,
          category,
          // `from` is the public area label shown in the feed; `to` is the full
          // address, released only to the assigned Sidekick.
          location_approx: from,
          location_exact: to,
          location_lat: approxAt?.lat,
          location_lng: approxAt?.lng,
          location_exact_lat: exactAt?.lat,
          location_exact_lng: exactAt?.lng,
          timing_type: 'specific_date',
          scheduled_for: dateTime.toISOString(),
          budget: budgetMinor,
        });
        taskId = task.id;
        setDraftTaskId(taskId);
        trackPostedTask(taskId);
      }

      // Funding escrow is what actually publishes the task.
      await fundTask.mutateAsync(defaultMethod.id);
      setActiveSheet('success');
    } catch (error) {
      toast.error('Couldn’t post your task', { description: errorMessage(error) });
    }
  };

  const handleAndroidDatePick = () => {
    DateTimePickerAndroid.open({
      value: dateTime ?? new Date(),
      mode: 'date',
      minimumDate: new Date(),
      onChange: (event: DateTimePickerEvent, selectedDate?: Date) => {
        if (event.type !== 'set' || !selectedDate) return;
        DateTimePickerAndroid.open({
          value: selectedDate,
          mode: 'time',
          onChange: (timeEvent: DateTimePickerEvent, selectedTime?: Date) => {
            if (timeEvent.type !== 'set' || !selectedTime) return;
            setDateTime(selectedTime);
          },
        });
      },
    });
  };

  const openDateTimePicker = () => {
    if (Platform.OS === 'android') {
      handleAndroidDatePick();
      return;
    }
    setPendingDate(dateTime ?? new Date());
    setDateSheetVisible(true);
  };

  const resetForm = () => {
    setStep(1);
    setTitle('');
    setCategory(null);
    setDescription('');
    setBudget('');
    setPhotos([]);
    setFrom('');
    setTo('');
    setDateTime(null);
    setActiveSheet(null);
    setDraftTaskId(null);
  };

  const handleBackHome = () => {
    resetForm();
    router.replace('/(tabs)');
  };

  const pickPhoto = async (fromCamera: boolean) => {
    const permission = fromCamera
      ? await ImagePicker.requestCameraPermissionsAsync()
      : await ImagePicker.requestMediaLibraryPermissionsAsync();
    if (!permission.granted) return;

    const result = fromCamera
      ? await ImagePicker.launchCameraAsync({ quality: 0.7 })
      : await ImagePicker.launchImageLibraryAsync({
          quality: 0.7,
          allowsMultipleSelection: true,
          selectionLimit: MAX_PHOTOS,
        });

    if (result.canceled) return;
    setPhotos((prev) => [...prev, ...result.assets.map((asset) => asset.uri)].slice(0, MAX_PHOTOS));
  };

  const handleAddPhoto = () => {
    showActionSheetWithOptions(
      { options: ['Take Photo', 'Choose from Library', 'Cancel'], cancelButtonIndex: 2 },
      (index) => {
        if (index === 0) pickPhoto(true);
        if (index === 1) pickPhoto(false);
      }
    );
  };

  const removePhoto = (uri: string) => setPhotos((prev) => prev.filter((photo) => photo !== uri));

  return (
    <SafeAreaView style={tw`flex-1 bg-white`}>
      <View style={tw`flex-row items-center justify-between px-4 pt-2`}>
        <View
          style={[
            tw`h-10 w-10 items-center justify-center rounded-full`,
            { backgroundColor: colors.orangeTint },
          ]}>
          <Text variant="body" fontWeight="medium" classN={`text-[${colors.orangeDark}]`}>
            B
          </Text>
        </View>
        <View
          style={{
            borderColor: colors.primary,
            ...tw`h-9 w-9 items-center justify-center rounded-full border`,
          }}>
          <Text variant="caption" fontWeight="medium" classN={`text-[${colors.primary}]`}>
            H
          </Text>
        </View>
      </View>

      <ScrollView
        style={tw`flex-1 px-4`}
        contentContainerStyle={{ paddingBottom: 130 }}
        showsVerticalScrollIndicator={false}
        keyboardShouldPersistTaps="handled">
        {step === 2 && (
          <PressableScale
            onPress={() => setStep(1)}
            hitSlop={8}
            style={tw`-ml-2 mt-2 h-9 w-9 items-center justify-center`}>
            <ChevronLeft size={24} color={colors.gray900} />
          </PressableScale>
        )}

        <Text
          variant="title"

          classN={step === 2 ? 'mt-1 text-black' : 'mt-3 text-black'}>
          What do you need?
        </Text>
        <Text variant="caption" classN={`mt-1 text-[${colors.gray500}]`}>
          Post your task for Sidekick to see
        </Text>

        {step === 1 && (
          <>
            <View style={tw`mt-6`}>
              <FieldLabel>Task Title</FieldLabel>
              <TextInput
                value={title}
                onChangeText={setTitle}
                placeholder="e.g. Deep clean 2-bedroom apartment"
                placeholderTextColor={colors.gray500}
                style={[
                  tw`mt-2 rounded-full px-4 text-black`,
                  { height: 52, borderWidth: 1, borderColor: colors.gray300 },
                ]}
              />
            </View>

            <View style={tw`mt-5`}>
              <FieldLabel>Category</FieldLabel>
              <PressableScale
                onPress={() => setCategorySheetVisible(true)}
                style={[
                  tw`mt-2 flex-row items-center justify-between rounded-full px-4`,
                  { height: 52, borderWidth: 1, borderColor: colors.gray300 },
                ]}>
                <Text variant="body" classN={category ? 'text-black' : `text-[${colors.gray500}]`}>
                  {category ?? 'Select category'}
                </Text>
                <ChevronDown size={18} color={colors.gray500} />
              </PressableScale>
            </View>

            <View style={tw`mt-5`}>
              <FieldLabel>Description</FieldLabel>
              <TextInput
                value={description}
                onChangeText={setDescription}
                placeholder={'Describe what needs to be done, any requirements\nand when...'}
                placeholderTextColor={colors.gray500}
                multiline
                style={[
                  tw`mt-2 rounded-2xl p-4 text-black`,
                  {
                    minHeight: 100,
                    borderWidth: 1,
                    borderColor: colors.gray300,
                    textAlignVertical: 'top',
                  },
                ]}
              />
            </View>

            <View style={tw`mt-5`}>
              <FieldLabel>Reference Photos (Optional)</FieldLabel>
              <View style={tw`mt-2 flex-row gap-3`}>
                {Array.from({ length: MAX_PHOTOS }).map((_, index) => {
                  const uri = photos[index];

                  if (uri) {
                    return (
                      <PressableScale
                        key={uri}
                        onLongPress={() => removePhoto(uri)}
                        style={tw`h-24 flex-1 overflow-hidden rounded-2xl`}>
                        <Image source={{ uri }} style={tw`h-full w-full`} />
                      </PressableScale>
                    );
                  }

                  const isNextSlot = index === photos.length;
                  return (
                    <PressableScale
                      key={index}
                      onPress={isNextSlot ? handleAddPhoto : undefined}
                      style={[
                        tw`h-24 flex-1 items-center justify-center rounded-2xl`,
                        { borderWidth: 1, borderColor: colors.gray300 },
                      ]}>
                      {isNextSlot ? (
                        <Upload size={20} color={colors.gray900} />
                      ) : (
                        <ImageIcon size={20} color={colors.gray300} />
                      )}
                    </PressableScale>
                  );
                })}
              </View>
            </View>

            <View style={tw`mt-5`}>
              <FieldLabel>{`Your Budget (${locale.currency})`}</FieldLabel>
              <TextInput
                value={budget}
                onChangeText={(text) => setBudget(text.replace(/[^0-9]/g, ''))}
                placeholder="0"
                placeholderTextColor={colors.gray500}
                keyboardType="number-pad"
                style={[
                  tw`mt-2 rounded-full px-4 text-black`,
                  { height: 52, borderWidth: 1, borderColor: colors.gray300 },
                ]}
              />
              {showBudgetWarning && (
                <Text variant="caption" classN={`mt-1.5 text-[${colors.orangeMuted}]`}>
                  {`Budgets under ${locale.currency}500 rarely attract Sidekick interest`}
                </Text>
              )}
            </View>

            <PressableScale
              disabled={!canContinueStep1}
              onPress={() => setStep(2)}
              style={[
                tw`mt-6 items-center justify-center rounded-full py-4`,
                { backgroundColor: colors.primary, opacity: canContinueStep1 ? 1 : 0.5 },
              ]}>
              <Text variant="body" fontWeight="medium" classN="text-white">
                Continue
              </Text>
            </PressableScale>
          </>
        )}

        {step === 2 && (
          <>
            <View style={tw`mt-6`}>
              <FieldLabel>Location</FieldLabel>
              <TextInput
                value={from}
                onChangeText={setFrom}
                placeholder="From"
                placeholderTextColor={colors.gray500}
                style={[
                  tw`mt-2 rounded-full px-4 text-black`,
                  { height: 52, borderWidth: 1, borderColor: colors.gray300 },
                ]}
              />
              <TextInput
                value={to}
                onChangeText={setTo}
                placeholder="To"
                placeholderTextColor={colors.gray500}
                style={[
                  tw`mt-2 rounded-full px-4 text-black`,
                  { height: 52, borderWidth: 1, borderColor: colors.gray300 },
                ]}
              />
              <View
                style={[
                  tw`mt-2 items-center justify-center rounded-2xl`,
                  { height: 160, borderWidth: 1, borderColor: colors.gray300 },
                ]}>
                <ImageOff size={26} color={colors.gray300} />
              </View>
            </View>

            <View style={tw`mt-5`}>
              <FieldLabel>Set Time & Date</FieldLabel>
              <PressableScale
                onPress={openDateTimePicker}
                style={[
                  tw`mt-2 flex-row items-center justify-between rounded-full px-4`,
                  { height: 52, borderWidth: 1, borderColor: colors.gray300 },
                ]}>
                <Text variant="body" classN={dateTime ? 'text-black' : `text-[${colors.gray500}]`}>
                  {dateTime
                    ? `${formatDateLabel(dateTime)} • ${formatTimeLabel(dateTime)}`
                    : 'Select date and time'}
                </Text>
                <Calendar size={18} color={colors.gray500} />
              </PressableScale>
            </View>

            <PressableScale
              disabled={!canContinueStep2}
              onPress={() => setActiveSheet('summary')}
              style={[
                tw`mt-6 items-center justify-center rounded-full py-4`,
                { backgroundColor: colors.primary, opacity: canContinueStep2 ? 1 : 0.5 },
              ]}>
              <Text variant="body" fontWeight="medium" classN="text-white">
                Continue
              </Text>
            </PressableScale>
          </>
        )}
      </ScrollView>

      <BottomSheet visible={categorySheetVisible} onClose={() => setCategorySheetVisible(false)}>
        <Text variant="subhead" fontWeight="black" classN="text-black">
          Select category
        </Text>
        <View style={tw`mt-2`}>
          {CATEGORIES.map((item) => {
            const isSelected = item === category;
            return (
              <PressableScale
                key={item}
                onPress={() => {
                  setCategory(item);
                  setCategorySheetVisible(false);
                }}
                style={[
                  tw`flex-row items-center justify-between rounded-2xl px-4 py-3.5`,
                  isSelected && { backgroundColor: colors.gray50 },
                ]}>
                <Text
                  variant="body"
                  fontWeight={isSelected ? 'bold' : 'normal'}
                  classN="text-black">
                  {item}
                </Text>
                {isSelected && <Check size={18} color={colors.primary} />}
              </PressableScale>
            );
          })}
        </View>
      </BottomSheet>

      {Platform.OS === 'ios' && (
        <BottomSheet visible={dateSheetVisible} onClose={() => setDateSheetVisible(false)}>
          <Text variant="subhead" fontWeight="black" classN="text-black">
            Set time & date
          </Text>
          <DateTimePicker
            value={pendingDate}
            mode="datetime"
            display="spinner"
            minimumDate={new Date()}
            onChange={(_, selectedDate) => selectedDate && setPendingDate(selectedDate)}
            style={tw`mt-2`}
          />
          <View style={tw`mt-2`}>
            <PressableScale
              onPress={() => {
                setDateTime(pendingDate);
                setDateSheetVisible(false);
              }}
              style={[
                tw`items-center justify-center rounded-full py-4`,
                { backgroundColor: colors.primary },
              ]}>
              <Text variant="body" fontWeight="medium" classN="text-white">
                Set date & time
              </Text>
            </PressableScale>
          </View>
        </BottomSheet>
      )}

      {dateTime && (
        <>
          <TaskSummarySheet
            visible={activeSheet === 'summary'}
            onClose={() => setActiveSheet(null)}
            onContinue={() => setActiveSheet('escrow')}
            category={category ?? ''}
            location={to}
            price={money(budgetMinor)}
            date={formatDateLabel(dateTime)}
            time={formatTimeLabel(dateTime)}
          />

          <EscrowPaymentSheet
            visible={activeSheet === 'escrow'}
            onClose={() => setActiveSheet('summary')}
            onPay={handlePay}
            budget={budgetMinor}
            platformFee={quote.data?.fee ?? 0}
            paying={createTask.isPending || fundTask.isPending}
          />
        </>
      )}

      <PaymentSuccessSheet visible={activeSheet === 'success'} onBackHome={handleBackHome} />
    </SafeAreaView>
  );
}
