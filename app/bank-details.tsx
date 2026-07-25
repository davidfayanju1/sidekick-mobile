import { router } from 'expo-router';
import { Check, ChevronDown, ChevronLeft } from 'lucide-react-native';
import * as React from 'react';
import { Pressable, ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import BottomSheet from '@/components/UI/BottomSheet';
import Form from '@/components/UI/Form';
import Text from '@/components/UI/Text';
import { tw } from '@/lib/tw';
import { useRoleStore } from '@/store/roleStore';

const ACCENT_TEAL = '#489A9F';
const BORDER = '#DADADA';
const MUTED = '#9AA0A6';

const BANKS = [
  'Access Bank',
  'GTBank',
  'Zenith Bank',
  'UBA',
  'First Bank',
  'Kuda',
  'Opay',
  'Moniepoint',
];

function FieldLabel({ children }: { children: string }) {
  return (
    <Text fontWeight="medium" fontSize={13} classN="text-black">
      {children}
    </Text>
  );
}

export default function BankDetails() {
  const setBankDetailsAdded = useRoleStore((state) => state.setBankDetailsAdded);
  const [bank, setBank] = React.useState<string | null>(null);
  const [bankSheetVisible, setBankSheetVisible] = React.useState(false);
  const [accountNumber, setAccountNumber] = React.useState('');
  const [accountName, setAccountName] = React.useState('');

  const canSave = Boolean(bank && accountNumber.length === 10 && accountName);

  const handleSave = () => {
    setBankDetailsAdded(true);
    router.back();
  };

  return (
    <SafeAreaView style={tw`flex-1 bg-white`}>
      <ScrollView
        style={tw`flex-1 px-4`}
        contentContainerStyle={{ paddingBottom: 24 }}
        keyboardShouldPersistTaps="handled"
        showsVerticalScrollIndicator={false}>
        <Pressable
          onPress={() => router.back()}
          hitSlop={8}
          style={tw`-ml-2 mt-3 h-10 w-10 items-center justify-center`}>
          <ChevronLeft size={26} color="#000" />
        </Pressable>

        <Text fontWeight="bold" fontSize={22} classN="mt-2 text-black">
          Add bank details
        </Text>
        <Text fontSize={13} classN="mt-1.5 text-[#6B7075]">
          This is where your earnings are sent when you withdraw.
        </Text>

        <View style={tw`mt-6`}>
          <FieldLabel>Bank</FieldLabel>
          <Pressable
            onPress={() => setBankSheetVisible(true)}
            style={[
              tw`mt-2 flex-row items-center justify-between rounded-full px-4`,
              { height: 52, borderWidth: 1, borderColor: BORDER },
            ]}>
            <Text fontSize={14} classN={bank ? 'text-black' : `text-[${MUTED}]`}>
              {bank ?? 'Select bank'}
            </Text>
            <ChevronDown size={18} color={MUTED} />
          </Pressable>
        </View>

        <Form
          label="Account Number"
          labelFontSize={13}
          labelStyle="text-black"
          containerStyle="mt-5"
          placeholder="0123456789"
          value={accountNumber}
          onChangeText={(text) => setAccountNumber(text.replace(/[^0-9]/g, '').slice(0, 10))}
          keyboardType="number-pad"
        />

        <Form
          label="Account Name"
          labelFontSize={13}
          labelStyle="text-black"
          containerStyle="mt-5"
          placeholder=""
          value={accountName}
          onChangeText={setAccountName}
          autoCapitalize="words"
        />

        <Pressable
          disabled={!canSave}
          onPress={handleSave}
          style={[
            tw`mt-8 items-center justify-center rounded-full py-4`,
            { backgroundColor: ACCENT_TEAL, opacity: canSave ? 1 : 0.5 },
          ]}>
          <Text fontWeight="bold" fontSize={15} classN="text-white">
            Save bank details
          </Text>
        </Pressable>
      </ScrollView>

      <BottomSheet visible={bankSheetVisible} onClose={() => setBankSheetVisible(false)}>
        <Text fontWeight="black" fontSize={18} classN="text-black">
          Select bank
        </Text>
        <View style={tw`mt-3`}>
          {BANKS.map((item) => {
            const isSelected = item === bank;
            return (
              <Pressable
                key={item}
                onPress={() => {
                  setBank(item);
                  setBankSheetVisible(false);
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
    </SafeAreaView>
  );
}
