import { useActionSheet } from '@expo/react-native-action-sheet';
import * as ImagePicker from 'expo-image-picker';
import { router } from 'expo-router';
import { Check, ChevronDown, ChevronLeft, Upload } from 'lucide-react-native';
import * as React from 'react';
import { Image, ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import BottomSheet from '@/components/UI/BottomSheet';
import Text from '@/components/UI/Text';
import { tw } from '@/lib/tw';
import { useRoleStore } from '@/store/roleStore';
import { colors } from '@/theme/palette';
import { PressableScale } from '@/components/UI/PressableScale';
import { Button } from '@/components/UI/Button';
import { useSubmitVerification } from '@/lib/api/trust';
import { errorMessage } from '@/lib/axios';
import { toast } from '@/store/toastStore';

/**
 * The API accepts three document types (`passport`, `driving_licence`,
 * `national_id`). A voter's card is filed under national_id — it's a
 * government identity document, which is the distinction the reviewer cares
 * about.
 */
const ID_TYPES = [
  { label: 'National ID (NIN)', value: 'national_id' },
  { label: "Driver's License", value: 'driving_licence' },
  { label: 'International Passport', value: 'passport' },
  { label: "Voter's Card", value: 'national_id' },
] as const;

type DocumentType = (typeof ID_TYPES)[number]['value'];

function FieldLabel({ children }: { children: string }) {
  return (
    <Text variant="caption" fontWeight="medium" classN="text-black">
      {children}
    </Text>
  );
}

export default function VerifyId() {
  const { showActionSheetWithOptions } = useActionSheet();
  const setIdVerificationStatus = useRoleStore((state) => state.setIdVerificationStatus);
  const submitVerification = useSubmitVerification();
  const [idType, setIdType] = React.useState<string | null>(null);
  const [documentType, setDocumentType] = React.useState<DocumentType | null>(null);
  const [idTypeSheetVisible, setIdTypeSheetVisible] = React.useState(false);
  const [photoUri, setPhotoUri] = React.useState<string | null>(null);

  const canSubmit = Boolean(idType && photoUri);

  const pickPhoto = async (fromCamera: boolean) => {
    const permission = fromCamera
      ? await ImagePicker.requestCameraPermissionsAsync()
      : await ImagePicker.requestMediaLibraryPermissionsAsync();
    if (!permission.granted) return;

    const result = fromCamera
      ? await ImagePicker.launchCameraAsync({ quality: 0.7 })
      : await ImagePicker.launchImageLibraryAsync({ quality: 0.7 });

    if (result.canceled) return;
    setPhotoUri(result.assets[0].uri);
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

  const handleSubmit = async () => {
    if (!photoUri || !documentType) return;
    try {
      await submitVerification.mutateAsync({ uri: photoUri, documentType });
      setIdVerificationStatus('pending');
      toast.success('Sent for review', {
        description: 'We’ll let you know once your ID has been checked — usually within a day.',
      });
      router.back();
    } catch (error) {
      toast.error('Couldn’t submit your ID', { description: errorMessage(error) });
    }
  };

  return (
    <SafeAreaView style={tw`flex-1 bg-white`}>
      <ScrollView
        style={tw`flex-1 px-4`}
        contentContainerStyle={{ paddingBottom: 24 }}
        showsVerticalScrollIndicator={false}>
        <PressableScale
          onPress={() => router.back()}
          hitSlop={8}
          style={tw`-ml-2 mt-2 h-10 w-10 items-center justify-center`}>
          <ChevronLeft size={26} color={colors.black} />
        </PressableScale>

        <Text variant="title" classN="mt-2 text-black">
          Verify your ID
        </Text>
        <Text variant="caption" classN={`mt-1.5 text-[${colors.gray700}]`}>
          This helps Heroes trust who&rsquo;s showing up. Review usually takes under 24 hours.
        </Text>

        <View style={tw`mt-6`}>
          <FieldLabel>ID Type</FieldLabel>
          <PressableScale
            onPress={() => setIdTypeSheetVisible(true)}
            style={[
              tw`mt-2 flex-row items-center justify-between rounded-full px-4`,
              { height: 52, borderWidth: 1, borderColor: colors.gray300 },
            ]}>
            <Text variant="body" classN={idType ? 'text-black' : `text-[${colors.gray500}]`}>
              {idType ?? 'Select ID type'}
            </Text>
            <ChevronDown size={18} color={colors.gray500} />
          </PressableScale>
        </View>

        <View style={tw`mt-5`}>
          <FieldLabel>Photo of your ID</FieldLabel>
          {photoUri ? (
            <PressableScale
              onPress={handleAddPhoto}
              style={tw`mt-2 h-40 overflow-hidden rounded-2xl`}>
              <Image source={{ uri: photoUri }} style={tw`h-full w-full`} />
            </PressableScale>
          ) : (
            <PressableScale
              onPress={handleAddPhoto}
              style={[
                tw`mt-2 h-40 items-center justify-center rounded-2xl`,
                { borderWidth: 1, borderColor: colors.gray300 },
              ]}>
              <Upload size={22} color={colors.gray900} />
              <Text variant="caption" classN={`mt-2 text-[${colors.gray500}]`}>
                Tap to upload a clear photo
              </Text>
            </PressableScale>
          )}
        </View>

        <Button
          label="Submit for review"
          onPress={handleSubmit}
          disabled={!canSubmit}
          loading={submitVerification.isPending}
          succeeded={submitVerification.isSuccess}
          size="lg"
          style={tw`mt-8`}
        />
      </ScrollView>

      <BottomSheet visible={idTypeSheetVisible} onClose={() => setIdTypeSheetVisible(false)}>
        <Text variant="subhead" fontWeight="black" classN="text-black">
          Select ID type
        </Text>
        <View style={tw`mt-2`}>
          {ID_TYPES.map((item) => {
            const isSelected = item.label === idType;
            return (
              <PressableScale
                key={item.label}
                onPress={() => {
                  setIdType(item.label);
                  setDocumentType(item.value);
                  setIdTypeSheetVisible(false);
                }}
                style={[
                  tw`flex-row items-center justify-between rounded-2xl px-4 py-3.5`,
                  isSelected && { backgroundColor: colors.gray50 },
                ]}>
                <Text
                  variant="body"
                  fontWeight={isSelected ? 'bold' : 'normal'}
                  classN="text-black">
                  {item.label}
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
