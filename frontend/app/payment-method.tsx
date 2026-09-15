import { router } from 'expo-router';
import { ChevronLeft } from 'lucide-react-native';
import * as React from 'react';
import { Pressable, ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import Form from '@/components/UI/Form';
import Text from '@/components/UI/Text';
import { tw } from '@/lib/tw';
import { useRoleStore } from '@/store/roleStore';

const ACCENT_TEAL = '#489A9F';

export default function PaymentMethod() {
  const setPaymentMethodAdded = useRoleStore((state) => state.setPaymentMethodAdded);
  const [cardNumber, setCardNumber] = React.useState('');
  const [cardholderName, setCardholderName] = React.useState('');
  const [expiry, setExpiry] = React.useState('');
  const [cvv, setCvv] = React.useState('');

  const canSave = Boolean(cardNumber && cardholderName && expiry && cvv);

  const handleSave = () => {
    setPaymentMethodAdded(true);
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
          Add a payment method
        </Text>
        <Text fontSize={13} classN="mt-1.5 text-[#6B7075]">
          Card details are handled securely and only used to fund tasks you post.
        </Text>

        <Form
          label="Card Number"
          labelFontSize={13}
          labelStyle="text-black"
          containerStyle="mt-6"
          placeholder="1234 5678 9012 3456"
          value={cardNumber}
          onChangeText={(text) => setCardNumber(text.replace(/[^0-9]/g, ''))}
          keyboardType="number-pad"
        />

        <Form
          label="Cardholder Name"
          labelFontSize={13}
          labelStyle="text-black"
          containerStyle="mt-5"
          placeholder=""
          value={cardholderName}
          onChangeText={setCardholderName}
          autoCapitalize="words"
        />

        <View style={tw`mt-5 flex-row gap-3`}>
          <View style={tw`flex-1`}>
            <Form
              label="Expiry"
              labelFontSize={13}
              labelStyle="text-black"
              placeholder="MM/YY"
              value={expiry}
              onChangeText={setExpiry}
              keyboardType="number-pad"
            />
          </View>
          <View style={tw`flex-1`}>
            <Form
              label="CVV"
              labelFontSize={13}
              labelStyle="text-black"
              placeholder="123"
              value={cvv}
              onChangeText={(text) => setCvv(text.replace(/[^0-9]/g, ''))}
              keyboardType="number-pad"
            />
          </View>
        </View>

        <Pressable
          disabled={!canSave}
          onPress={handleSave}
          style={[
            tw`mt-8 items-center justify-center rounded-full py-4`,
            { backgroundColor: ACCENT_TEAL, opacity: canSave ? 1 : 0.5 },
          ]}>
          <Text fontWeight="bold" fontSize={15} classN="text-white">
            Save card
          </Text>
        </Pressable>
      </ScrollView>
    </SafeAreaView>
  );
}
