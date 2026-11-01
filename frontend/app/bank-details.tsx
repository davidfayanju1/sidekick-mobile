import { router } from 'expo-router';
import { Check, ChevronDown, ChevronLeft } from 'lucide-react-native';
import * as React from 'react';
import { ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import BottomSheet from '@/components/UI/BottomSheet';
import Form from '@/components/UI/Form';
import Text from '@/components/UI/Text';
import { tw } from '@/lib/tw';
import { Button } from '@/components/UI/Button';
import { useAddPayoutMethod } from '@/lib/api/finance';
import { errorMessage } from '@/lib/axios';
import { bankDetailsSchema } from '@/lib/forms/schemas';
import { useZodForm } from '@/lib/forms/useZodForm';
import { locale } from '@/lib/locale';
import { useRoleStore } from '@/store/roleStore';
import { toast } from '@/store/toastStore';
import { colors } from '@/theme/palette';
import { PressableScale } from '@/components/UI/PressableScale';

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
    <Text variant="caption" fontWeight="medium" classN="text-black">
      {children}
    </Text>
  );
}

export default function BankDetails() {
  const setBankDetailsAdded = useRoleStore((state) => state.setBankDetailsAdded);
  const addPayoutMethod = useAddPayoutMethod();
  const [bankSheetVisible, setBankSheetVisible] = React.useState(false);

  const form = useZodForm(bankDetailsSchema, {
    bank_name: '',
    account_number: '',
    account_name: '',
  });

  const bank = (form.values.bank_name as string) || null;

  const handleSave = form.handleSubmit(async (details) => {
    try {
      await addPayoutMethod.mutateAsync({
        last4: details.account_number.slice(-4),
        bank_name: details.bank_name,
      });

      setBankDetailsAdded(true);
      toast.success('Bank details saved', {
        description: `${details.bank_name} •••• ${details.account_number.slice(-4)}`,
      });
      router.back();
    } catch (error) {
      toast.error('Couldn’t save your bank details', { description: errorMessage(error) });
    }
  });

  return (
    <SafeAreaView style={tw`flex-1 bg-white`}>
      <ScrollView
        style={tw`flex-1 px-4`}
        contentContainerStyle={{ paddingBottom: 24 }}
        keyboardShouldPersistTaps="handled"
        showsVerticalScrollIndicator={false}>
        <PressableScale
          onPress={() => router.back()}
          hitSlop={8}
          style={tw`-ml-2 mt-2 h-10 w-10 items-center justify-center`}>
          <ChevronLeft size={26} color={colors.black} />
        </PressableScale>

        <Text variant="title" classN="mt-2 text-black">
          Add bank details
        </Text>
        <Text variant="caption" classN={`mt-1.5 text-[${colors.gray700}]`}>
          This is where your earnings are sent when you withdraw.
        </Text>

        <View style={tw`mt-6`}>
          <FieldLabel>Bank</FieldLabel>
          <PressableScale
            onPress={() => setBankSheetVisible(true)}
            style={[
              tw`mt-2 flex-row items-center justify-between rounded-full px-4`,
              { height: 52, borderWidth: 1, borderColor: colors.gray300 },
            ]}>
            <Text variant="body" classN={bank ? 'text-black' : `text-[${colors.gray500}]`}>
              {bank ?? 'Select bank'}
            </Text>
            <ChevronDown size={18} color={colors.gray500} />
          </PressableScale>
        </View>

        <Form
          label="Account Number"
          labelFontSize={13}
          labelStyle="text-black"
          containerStyle="mt-5"
          placeholder="0123456789"
          keyboardType="number-pad"
          maxLength={locale.bank.accountLength}
          {...form.field('account_number')}
          onChangeText={(text) =>
            form.setValue(
              'account_number',
              text.replace(/[^0-9]/g, '').slice(0, locale.bank.accountLength)
            )
          }
        />

        <Form
          label="Account Name"
          labelFontSize={13}
          labelStyle="text-black"
          containerStyle="mt-5"
          placeholder=""
          autoCapitalize="words"
          {...form.field('account_name')}
        />

        {form.errors.bank_name ? (
          <Text variant="caption" classN={`ml-4 mt-1 text-[${colors.danger}]`}>
            {form.errors.bank_name}
          </Text>
        ) : null}

        <Button
          label="Save bank details"
          onPress={handleSave}
          loading={addPayoutMethod.isPending}
          succeeded={addPayoutMethod.isSuccess}
          size="lg"
          style={tw`mt-8`}
        />
      </ScrollView>

      <BottomSheet visible={bankSheetVisible} onClose={() => setBankSheetVisible(false)}>
        <Text variant="subhead" fontWeight="black" classN="text-black">
          Select bank
        </Text>
        <View style={tw`mt-2`}>
          {BANKS.map((item) => {
            const isSelected = item === bank;
            return (
              <PressableScale
                key={item}
                onPress={() => {
                  form.setValue('bank_name', item);
                  setBankSheetVisible(false);
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
    </SafeAreaView>
  );
}
