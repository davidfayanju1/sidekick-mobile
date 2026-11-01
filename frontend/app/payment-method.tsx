import { router } from 'expo-router';
import { ChevronLeft } from 'lucide-react-native';
import * as React from 'react';
import { ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { Button } from '@/components/UI/Button';
import Form from '@/components/UI/Form';
import Text from '@/components/UI/Text';
import { useAddPaymentMethod } from '@/lib/api/finance';
import { errorMessage } from '@/lib/axios';
import { cardSchema } from '@/lib/forms/schemas';
import { useZodForm } from '@/lib/forms/useZodForm';
import { tw } from '@/lib/tw';
import { useRoleStore } from '@/store/roleStore';
import { toast } from '@/store/toastStore';
import { colors } from '@/theme/palette';
import { PressableScale } from '@/components/UI/PressableScale';

/** Visa 4, Mastercard 5, Amex 3, Verve 5061. Enough to label the saved card. */
function brandOf(number: string): string {
  if (number.startsWith('4')) return 'Visa';
  if (number.startsWith('5061') || number.startsWith('5078')) return 'Verve';
  if (/^5[1-5]/.test(number)) return 'Mastercard';
  if (/^3[47]/.test(number)) return 'Amex';
  return 'Card';
}

export default function PaymentMethod() {
  const setPaymentMethodAdded = useRoleStore((state) => state.setPaymentMethodAdded);
  const addPaymentMethod = useAddPaymentMethod();

  const form = useZodForm(cardSchema, { number: '', name: '', expiry: '', cvv: '' });

  const handleSave = form.handleSubmit(async (card) => {
    const [month, year] = card.expiry.split('/').map(Number);

    try {
      // The card number and CVV never leave this device. The API stores a
      // provider token plus the last four digits — see the "Card data never
      // touches Sidekick servers" rule in BACKEND.md §2.
      //
      // TODO: pass the payment provider SDK's token as `provider_ref` once a
      // provider is chosen; until then the mutation mints a sandbox one.
      await addPaymentMethod.mutateAsync({
        last4: card.number.slice(-4),
        brand: brandOf(card.number),
        exp_month: month,
        exp_year: 2000 + year,
      });

      setPaymentMethodAdded(true);
      toast.success('Card saved', {
        description: `${brandOf(card.number)} ending ${card.number.slice(-4)}`,
      });
      router.back();
    } catch (error) {
      toast.error('Couldn’t save that card', { description: errorMessage(error) });
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
          Add a payment method
        </Text>
        <Text variant="caption" classN={`mt-1.5 text-[${colors.gray700}]`}>
          Card details are handled securely and only used to fund tasks you post.
        </Text>

        <Form
          label="Card Number"
          labelFontSize={13}
          labelStyle="text-black"
          containerStyle="mt-6"
          placeholder="1234 5678 9012 3456"
          keyboardType="number-pad"
          maxLength={19}
          {...form.field('number')}
          onChangeText={(text) => form.setValue('number', text.replace(/[^0-9]/g, ''))}
        />

        <Form
          label="Cardholder Name"
          labelFontSize={13}
          labelStyle="text-black"
          containerStyle="mt-5"
          placeholder=""
          autoCapitalize="words"
          {...form.field('name')}
        />

        <View style={tw`mt-5 flex-row gap-3`}>
          <View style={tw`flex-1`}>
            <Form
              label="Expiry"
              labelFontSize={13}
              labelStyle="text-black"
              placeholder="MM/YY"
              keyboardType="number-pad"
              maxLength={5}
              {...form.field('expiry')}
              onChangeText={(text) => {
                // Insert the slash as they type so MM/YY never has to be typed.
                const digits = text.replace(/[^0-9]/g, '').slice(0, 4);
                form.setValue(
                  'expiry',
                  digits.length > 2 ? `${digits.slice(0, 2)}/${digits.slice(2)}` : digits
                );
              }}
            />
          </View>
          <View style={tw`flex-1`}>
            <Form
              label="CVV"
              labelFontSize={13}
              labelStyle="text-black"
              placeholder="123"
              keyboardType="number-pad"
              maxLength={4}
              {...form.field('cvv')}
              onChangeText={(text) => form.setValue('cvv', text.replace(/[^0-9]/g, ''))}
            />
          </View>
        </View>

        <Button
          label="Save card"
          onPress={handleSave}
          loading={addPaymentMethod.isPending}
          succeeded={addPaymentMethod.isSuccess}
          size="lg"
          style={tw`mt-8`}
        />
      </ScrollView>
    </SafeAreaView>
  );
}
