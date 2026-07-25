import { useActionSheet } from '@expo/react-native-action-sheet';
import DateTimePicker, {
  DateTimePickerAndroid,
  type DateTimePickerEvent,
} from '@react-native-community/datetimepicker';
import * as ImagePicker from 'expo-image-picker';
import { router } from 'expo-router';
import {
  Calendar,
  Check,
  ChevronDown,
  ChevronLeft,
  Image as ImageIcon,
  ImageOff,
  Upload,
} from 'lucide-react-native';
import * as React from 'react';
import { Image, Platform, Pressable, ScrollView, TextInput, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import BottomSheet from '@/components/UI/BottomSheet';
import Text from '@/components/UI/Text';
import {
  EscrowPaymentSheet,
  PaymentSuccessSheet,
  TaskSummarySheet,
} from '@/components/UI/TaskSheets';
import { tw } from '@/lib/tw';

const ACCENT_TEAL = '#489A9F';
const BORDER = '#DADADA';
const MUTED = '#9AA0A6';
const WARNING = '#D97A55';
const MAX_PHOTOS = 3;
const PLATFORM_FEE_PERCENT = 0.5;

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
    <Text fontWeight="medium" fontSize={13} classN="text-black">
      {children}
    </Text>
  );
}

export default function Post() {
  const { showActionSheetWithOptions } = useActionSheet();
  const [step, setStep] = React.useState<WizardStep>(1);
  const [title, setTitle] = React.useState('');
  const [category, setCategory] = React.useState<string | null>(null);
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

  const budgetValue = Number(budget);
  const showBudgetWarning = budget.length > 0 && budgetValue > 0 && budgetValue < 500;
  const canContinueStep1 = Boolean(title && category && description && budget);
  const canContinueStep2 = Boolean(from && to && dateTime);
  const platformFee = (budgetValue * PLATFORM_FEE_PERCENT) / 100;

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
            { backgroundColor: '#F0DCC8' },
          ]}>
          <Text fontWeight="bold" fontSize={15} classN="text-[#B5762E]">
            B
          </Text>
        </View>
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

      <ScrollView
        style={tw`flex-1 px-4`}
        contentContainerStyle={{ paddingBottom: 130 }}
        showsVerticalScrollIndicator={false}
        keyboardShouldPersistTaps="handled">
        {step === 2 && (
          <Pressable
            onPress={() => setStep(1)}
            hitSlop={8}
            style={tw`-ml-2 mt-3 h-9 w-9 items-center justify-center`}>
            <ChevronLeft size={24} color="#16181A" />
          </Pressable>
        )}

        <Text
          fontWeight="bold"
          fontSize={22}
          classN={step === 2 ? 'mt-1 text-black' : 'mt-4 text-black'}>
          What do you need?
        </Text>
        <Text fontSize={13} classN="mt-1 text-[#9AA0A6]">
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
                placeholderTextColor={MUTED}
                style={[
                  tw`mt-2 rounded-full px-4 text-black`,
                  { height: 52, borderWidth: 1, borderColor: BORDER },
                ]}
              />
            </View>

            <View style={tw`mt-5`}>
              <FieldLabel>Category</FieldLabel>
              <Pressable
                onPress={() => setCategorySheetVisible(true)}
                style={[
                  tw`mt-2 flex-row items-center justify-between rounded-full px-4`,
                  { height: 52, borderWidth: 1, borderColor: BORDER },
                ]}>
                <Text fontSize={14} classN={category ? 'text-black' : `text-[${MUTED}]`}>
                  {category ?? 'Select category'}
                </Text>
                <ChevronDown size={18} color={MUTED} />
              </Pressable>
            </View>

            <View style={tw`mt-5`}>
              <FieldLabel>Description</FieldLabel>
              <TextInput
                value={description}
                onChangeText={setDescription}
                placeholder={'Describe what needs to be done, any requirements\nand when...'}
                placeholderTextColor={MUTED}
                multiline
                style={[
                  tw`mt-2 rounded-2xl p-4 text-black`,
                  { minHeight: 100, borderWidth: 1, borderColor: BORDER, textAlignVertical: 'top' },
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
                      <Pressable
                        key={uri}
                        onLongPress={() => removePhoto(uri)}
                        style={tw`h-24 flex-1 overflow-hidden rounded-2xl`}>
                        <Image source={{ uri }} style={tw`h-full w-full`} />
                      </Pressable>
                    );
                  }

                  const isNextSlot = index === photos.length;
                  return (
                    <Pressable
                      key={index}
                      onPress={isNextSlot ? handleAddPhoto : undefined}
                      style={[
                        tw`h-24 flex-1 items-center justify-center rounded-2xl`,
                        { borderWidth: 1, borderColor: BORDER },
                      ]}>
                      {isNextSlot ? (
                        <Upload size={20} color="#16181A" />
                      ) : (
                        <ImageIcon size={20} color="#D9DCE0" />
                      )}
                    </Pressable>
                  );
                })}
              </View>
            </View>

            <View style={tw`mt-5`}>
              <FieldLabel>Your Budget (₦)</FieldLabel>
              <TextInput
                value={budget}
                onChangeText={(text) => setBudget(text.replace(/[^0-9]/g, ''))}
                placeholder="0"
                placeholderTextColor={MUTED}
                keyboardType="number-pad"
                style={[
                  tw`mt-2 rounded-full px-4 text-black`,
                  { height: 52, borderWidth: 1, borderColor: BORDER },
                ]}
              />
              {showBudgetWarning && (
                <Text fontSize={12} classN={`mt-1.5 text-[${WARNING}]`}>
                  Budget under ₦500 rarely attract sidekick interest
                </Text>
              )}
            </View>

            <Pressable
              disabled={!canContinueStep1}
              onPress={() => setStep(2)}
              style={[
                tw`mt-6 items-center justify-center rounded-full py-4`,
                { backgroundColor: ACCENT_TEAL, opacity: canContinueStep1 ? 1 : 0.5 },
              ]}>
              <Text fontWeight="bold" fontSize={15} classN="text-white">
                Continue
              </Text>
            </Pressable>
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
                placeholderTextColor={MUTED}
                style={[
                  tw`mt-2 rounded-full px-4 text-black`,
                  { height: 52, borderWidth: 1, borderColor: BORDER },
                ]}
              />
              <TextInput
                value={to}
                onChangeText={setTo}
                placeholder="To"
                placeholderTextColor={MUTED}
                style={[
                  tw`mt-3 rounded-full px-4 text-black`,
                  { height: 52, borderWidth: 1, borderColor: BORDER },
                ]}
              />
              <View
                style={[
                  tw`mt-3 items-center justify-center rounded-2xl`,
                  { height: 160, borderWidth: 1, borderColor: BORDER },
                ]}>
                <ImageOff size={26} color="#D9DCE0" />
              </View>
            </View>

            <View style={tw`mt-5`}>
              <FieldLabel>Set Time & Date</FieldLabel>
              <Pressable
                onPress={openDateTimePicker}
                style={[
                  tw`mt-2 flex-row items-center justify-between rounded-full px-4`,
                  { height: 52, borderWidth: 1, borderColor: BORDER },
                ]}>
                <Text fontSize={14} classN={dateTime ? 'text-black' : `text-[${MUTED}]`}>
                  {dateTime
                    ? `${formatDateLabel(dateTime)} • ${formatTimeLabel(dateTime)}`
                    : 'Select date and time'}
                </Text>
                <Calendar size={18} color={MUTED} />
              </Pressable>
            </View>

            <Pressable
              disabled={!canContinueStep2}
              onPress={() => setActiveSheet('summary')}
              style={[
                tw`mt-6 items-center justify-center rounded-full py-4`,
                { backgroundColor: ACCENT_TEAL, opacity: canContinueStep2 ? 1 : 0.5 },
              ]}>
              <Text fontWeight="bold" fontSize={15} classN="text-white">
                Continue
              </Text>
            </Pressable>
          </>
        )}
      </ScrollView>

      <BottomSheet visible={categorySheetVisible} onClose={() => setCategorySheetVisible(false)}>
        <Text fontWeight="black" fontSize={18} classN="text-black">
          Select category
        </Text>
        <View style={tw`mt-3`}>
          {CATEGORIES.map((item) => {
            const isSelected = item === category;
            return (
              <Pressable
                key={item}
                onPress={() => {
                  setCategory(item);
                  setCategorySheetVisible(false);
                }}
                style={[
                  tw`flex-row items-center justify-between rounded-2xl px-4 py-3.5`,
                  isSelected && { backgroundColor: '#F5F6F7' },
                ]}>
                <Text fontWeight={isSelected ? 'bold' : 'normal'} fontSize={14} classN="text-black">
                  {item}
                </Text>
                {isSelected && <Check size={18} color={ACCENT_TEAL} />}
              </Pressable>
            );
          })}
        </View>
      </BottomSheet>

      {Platform.OS === 'ios' && (
        <BottomSheet visible={dateSheetVisible} onClose={() => setDateSheetVisible(false)}>
          <Text fontWeight="black" fontSize={18} classN="text-black">
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
          <View style={tw`mt-3`}>
            <Pressable
              onPress={() => {
                setDateTime(pendingDate);
                setDateSheetVisible(false);
              }}
              style={[
                tw`items-center justify-center rounded-full py-4`,
                { backgroundColor: ACCENT_TEAL },
              ]}>
              <Text fontWeight="bold" fontSize={15} classN="text-white">
                Set date & time
              </Text>
            </Pressable>
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
            price={`₦${budgetValue.toLocaleString('en-US')}`}
            date={formatDateLabel(dateTime)}
            time={formatTimeLabel(dateTime)}
          />

          <EscrowPaymentSheet
            visible={activeSheet === 'escrow'}
            onClose={() => setActiveSheet('summary')}
            onPay={() => setActiveSheet('success')}
            budget={budgetValue}
            platformFee={platformFee}
          />
        </>
      )}

      <PaymentSuccessSheet visible={activeSheet === 'success'} onBackHome={handleBackHome} />
    </SafeAreaView>
  );
}
