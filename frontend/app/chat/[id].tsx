import { router, useLocalSearchParams } from 'expo-router';
import { ChevronLeft, Send } from 'lucide-react-native';
import * as React from 'react';
import {
  KeyboardAvoidingView,
  Platform,
  Pressable,
  ScrollView,
  TextInput,
  View,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import Text from '@/components/UI/Text';
import { CHATS } from '@/app/(tabs)/chats';
import { tw } from '@/lib/tw';
import { colors } from '@/theme/palette';

type Message =
  | { id: string; type: 'text'; from: 'me' | 'them'; text: string; time: string }
  | { id: string; type: 'system'; text: string };

const CONVERSATIONS: Record<string, Message[]> = {
  '1': [
    {
      id: 'm1',
      type: 'text',
      from: 'them',
      text: 'Hi! I can do the cleaning. Is 10am okay?',
      time: '08:20 AM',
    },
    {
      id: 'm2',
      type: 'text',
      from: 'me',
      text: 'Yes 10am works. Do you have your own supplies?',
      time: '08:21 AM',
    },
    { id: 'm3', type: 'text', from: 'them', text: 'Yes I bring everything.', time: '08:22 AM' },
    {
      id: 'm4',
      type: 'text',
      from: 'me',
      text: 'Aright. I will be waiting for you.',
      time: '08:21 AM',
    },
    { id: 'm5', type: 'system', text: '₦12,000 locked in escrow • Task started' },
  ],
};

function formatTime(date: Date) {
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

export default function ChatDetail() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const chat = CHATS.find((c) => c.id === id);
  const scrollRef = React.useRef<ScrollView>(null);

  const [messages, setMessages] = React.useState<Message[]>(CONVERSATIONS[id ?? ''] ?? []);
  const [draft, setDraft] = React.useState('');

  const handleSend = () => {
    const text = draft.trim();
    if (!text) return;
    setMessages((prev) => [
      ...prev,
      { id: `local-${Date.now()}`, type: 'text', from: 'me', text, time: formatTime(new Date()) },
    ]);
    setDraft('');
    requestAnimationFrame(() => scrollRef.current?.scrollToEnd({ animated: true }));
  };

  return (
    <SafeAreaView style={tw`flex-1 bg-white`} edges={['top', 'bottom']}>
      <KeyboardAvoidingView
        style={tw`flex-1`}
        behavior={Platform.OS === 'ios' ? 'padding' : undefined}
        keyboardVerticalOffset={Platform.OS === 'ios' ? 8 : 0}>
        <View
          style={tw`flex-row items-center gap-3 border-b border-[${colors.surfaceSunken}] px-4 pb-3`}>
          <Pressable onPress={() => router.back()} hitSlop={8}>
            <ChevronLeft size={24} color={colors.textPrimary} />
          </Pressable>
          <View
            style={[
              tw`h-10 w-10 items-center justify-center rounded-full`,
              { backgroundColor: chat?.avatarBg ?? colors.accentSand },
            ]}>
            <Text
              fontWeight="bold"
              fontSize={15}
              classN={`text-[${chat?.avatarText ?? '${colors.pendingText}'}]`}>
              {chat?.initial ?? '?'}
            </Text>
          </View>
          <View>
            <Text fontWeight="bold" fontSize={15} classN="text-black">
              {chat?.fullName ?? 'Chat'}
            </Text>
            <Text fontSize={12} classN={`text-[${colors.brand}]`}>
              {chat?.online ? 'Online' : 'Offline'} . {chat?.taskTitle ?? ''}
            </Text>
          </View>
        </View>

        <ScrollView
          ref={scrollRef}
          style={tw`flex-1 px-4`}
          contentContainerStyle={{ paddingVertical: 16 }}
          showsVerticalScrollIndicator={false}
          onContentSizeChange={() => scrollRef.current?.scrollToEnd({ animated: false })}>
          {messages.length === 0 ? (
            <View style={tw`mt-16 items-center`}>
              <Text fontSize={14} classN={`text-[${colors.textMuted}]`}>
                No messages yet
              </Text>
            </View>
          ) : (
            messages.map((message) => {
              if (message.type === 'system') {
                return (
                  <View key={message.id} style={tw`my-3 items-center`}>
                    <View
                      style={[
                        tw`rounded-full px-4 py-2`,
                        { backgroundColor: colors.surfaceSunken },
                      ]}>
                      <Text fontSize={12} classN={`text-[${colors.textMuted}]`}>
                        {message.text}
                      </Text>
                    </View>
                  </View>
                );
              }

              const isMe = message.from === 'me';
              return (
                <View key={message.id} style={tw`mt-3 ${isMe ? 'items-end' : 'items-start'}`}>
                  <View
                    style={[
                      tw`max-w-[80%] rounded-2xl px-4 py-3`,
                      { backgroundColor: isMe ? colors.brandBubble : colors.surface },
                    ]}>
                    <Text fontSize={14} classN="text-black">
                      {message.text}
                    </Text>
                  </View>
                  <Text fontSize={10} classN={`mt-1 text-[${colors.textMuted}]`}>
                    {message.time}
                  </Text>
                </View>
              );
            })
          )}
        </ScrollView>

        <View
          style={tw`flex-row items-center gap-2.5 border-t border-[${colors.surfaceSunken}] px-4 py-3`}>
          <TextInput
            value={draft}
            onChangeText={setDraft}
            placeholder="Type a message"
            placeholderTextColor={colors.textMuted}
            style={[
              tw`flex-1 rounded-full px-4 text-black`,
              { height: 46, backgroundColor: colors.surface },
            ]}
          />
          <Pressable
            onPress={handleSend}
            style={[
              tw`h-11 w-11 items-center justify-center rounded-full`,
              { backgroundColor: colors.brand },
            ]}>
            <Send size={18} color="white" />
          </Pressable>
        </View>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}
