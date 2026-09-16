import { useActionSheet } from '@expo/react-native-action-sheet';
import * as ImagePicker from 'expo-image-picker';
import { router } from 'expo-router';
import { Check, ChevronDown, ChevronLeft, Upload } from 'lucide-react-native';
import * as React from 'react';
import { Image, Pressable, ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import BottomSheet from '@/components/UI/BottomSheet';
import Text from '@/components/UI/Text';
import { tw } from '@/lib/tw';
import { useRoleStore } from '@/store/roleStore';
import { colors } from '@/theme/palette';

const ID_TYPES = [
  'National ID (NIN)',
  "Driver's License",
  'International Passport',
  "Voter's Card",
];

function FieldLabel({ children }: { children: string }) {
  return (
    <Text fontWeight="medium" fontSize={13} classN="text-black">
      {children}
    </Text>
  );
}

export default function VerifyId() {
  const { showActionSheetWithOptions } = useActionSheet();
  const setIdVerificationStatus = useRoleStore((state) => state.setIdVerificationStatus);
  const [idType, setIdType] = React.useState<string | null>(null);
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

  const handleSubmit = () => {
    setIdVerificationStatus('pending');
    router.back();
  };

  return (
    <SafeAreaView style={tw`flex-1 bg-white`}>
      <ScrollView
        style={tw`flex-1 px-4`}
        contentContainerStyle={{ paddingBottom: 24 }}
        showsVerticalScrollIndicator={false}>
        <Pressable
          onPress={() => router.back()}
          hitSlop={8}
          style={tw`-ml-2 mt-3 h-10 w-10 items-center justify-center`}>
          <ChevronLeft size={26} color={colors.iconPrimary} />
        </Pressable>

        <Text fontWeight="bold" fontSize={22} classN="mt-2 text-black">
          Verify your ID
        </Text>
        <Text fontSize={13} classN={`mt-1.5 text-[${colors.textSecondary}]`}>
          This helps Heroes trust who&rsquo;s showing up. Review usually takes under 24 hours.
        </Text>

        <View style={tw`mt-6`}>
          <FieldLabel>ID Type</FieldLabel>
          <Pressable
            onPress={() => setIdTypeSheetVisible(true)}
            style={[
              tw`mt-2 flex-row items-center justify-between rounded-full px-4`,
              { height: 52, borderWidth: 1, borderColor: colors.border },
            ]}>
            <Text fontSize={14} classN={idType ? 'text-black' : `text-[${colors.textMuted}]`}>
              {idType ?? 'Select ID type'}
            </Text>
            <ChevronDown size={18} color={colors.textMuted} />
          </Pressable>
        </View>

        <View style={tw`mt-5`}>
          <FieldLabel>Photo of your ID</FieldLabel>
          {photoUri ? (
            <Pressable onPress={handleAddPhoto} style={tw`mt-2 h-40 overflow-hidden rounded-2xl`}>
              <Image source={{ uri: photoUri }} style={tw`h-full w-full`} />
            </Pressable>
          ) : (
            <Pressable
              onPress={handleAddPhoto}
              style={[
                tw`mt-2 h-40 items-center justify-center rounded-2xl`,
                { borderWidth: 1, borderColor: colors.border },
              ]}>
              <Upload size={22} color={colors.textPrimary} />
              <Text fontSize={12} classN={`mt-2 text-[${colors.textMuted}]`}>
                Tap to upload a clear photo
              </Text>
            </Pressable>
          )}
        </View>

        <Pressable
          disabled={!canSubmit}
          onPress={handleSubmit}
          style={[
            tw`mt-8 items-center justify-center rounded-full py-4`,
            { backgroundColor: colors.brand, opacity: canSubmit ? 1 : 0.5 },
          ]}>
          <Text fontWeight="bold" fontSize={15} classN="text-white">
            Submit for review
          </Text>
        </Pressable>
      </ScrollView>

      <BottomSheet visible={idTypeSheetVisible} onClose={() => setIdTypeSheetVisible(false)}>
        <Text fontWeight="black" fontSize={18} classN="text-black">
          Select ID type
        </Text>
        <View style={tw`mt-3`}>
          {ID_TYPES.map((item) => {
            const isSelected = item === idType;
            return (
              <Pressable
                key={item}
                onPress={() => {
                  setIdType(item);
                  setIdTypeSheetVisible(false);
                }}
                style={[
                  tw`flex-row items-center justify-between rounded-2xl px-4 py-3.5`,
                  isSelected && { backgroundColor: colors.surface },
                ]}>
                <Text fontWeight={isSelected ? 'bold' : 'normal'} fontSize={14} classN="text-black">
                  {item}
                </Text>
                {isSelected && <Check size={18} color={colors.brand} />}
              </Pressable>
            );
          })}
        </View>
      </BottomSheet>
    </SafeAreaView>
  );
}
