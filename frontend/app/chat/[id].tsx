import { router, useLocalSearchParams } from 'expo-router';
import { ChevronLeft, RotateCw, Send } from 'lucide-react-native';
import * as React from 'react';
import {
  KeyboardAvoidingView,
  Platform,
  ScrollView,
  TextInput,
  View,
  type ScrollViewInstance,
} from 'react-native';
import Animated, { FadeIn } from 'react-native-reanimated';
import { SafeAreaView } from 'react-native-safe-area-context';

import { EmptyState, ErrorState } from '@/components/UI/EmptyState';
import { QuietConversation } from '@/components/UI/illustrations';
import { PressableScale } from '@/components/UI/PressableScale';
import { SkeletonText } from '@/components/UI/Skeleton';
import { Spinner } from '@/components/UI/Spinner';
import Text from '@/components/UI/Text';
import { useMessages, useSendMessage, type PendingMessage } from '@/lib/api/messaging';
import { errorMessage } from '@/lib/axios';
import { messageTime } from '@/lib/format';
import { tw } from '@/lib/tw';
import { useAuthStore } from '@/store/authStore';
import { toast } from '@/store/toastStore';
import { colors } from '@/theme/palette';

/** Stable id for an optimistic message so the server's copy can replace it. */
function localId() {
  return `local-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

export default function ChatDetail() {
  const { id } = useLocalSearchParams<{ id: string }>();
  // RN 0.88's strict TypeScript API: a ref holds the *instance* type, not the
  // component type — `ScrollView` is now the function component itself.
  const scrollRef = React.useRef<ScrollViewInstance>(null);
  const currentUserId = useAuthStore((state) => state.user?.id);

  const messagesQuery = useMessages(id);
  const sendMessage = useSendMessage(id ?? '', currentUserId);
  const [draft, setDraft] = React.useState('');

  const messages = messagesQuery.data ?? [];

  const send = async (body: string, retryId?: string) => {
    const text = body.trim();
    if (!text) return;
    if (!retryId) setDraft('');

    try {
      await sendMessage.mutateAsync({ body: text, localId: retryId ?? localId() });
      requestAnimationFrame(() => scrollRef.current?.scrollToEnd({ animated: true }));
    } catch (error) {
      // The message stays on screen marked failed — the row offers a retry, so
      // a toast here would just be a second copy of the same bad news.
      toast.error('Message didn’t send', { description: errorMessage(error) });
    }
  };

  const title = messages.find((message) => message.sender_name)?.sender_name ?? 'Chat';

  return (
    <SafeAreaView style={tw`flex-1 bg-white`} edges={['top', 'bottom']}>
      <KeyboardAvoidingView
        style={tw`flex-1`}
        behavior={Platform.OS === 'ios' ? 'padding' : undefined}
        keyboardVerticalOffset={Platform.OS === 'ios' ? 8 : 0}>
        <View style={tw`flex-row items-center gap-3 border-b border-[${colors.gray100}] px-4 pb-3`}>
          <PressableScale onPress={() => router.back()} hitSlop={8}>
            <ChevronLeft size={24} color={colors.gray900} />
          </PressableScale>
          <View
            style={[
              tw`h-10 w-10 items-center justify-center rounded-full`,
              { backgroundColor: colors.orangeTint },
            ]}>
            <Text variant="body" fontWeight="medium" classN={`text-[${colors.orangeDark}]`}>
              {title.charAt(0).toUpperCase()}
            </Text>
          </View>
          <View style={tw`flex-1`}>
            <Text variant="body" fontWeight="medium" classN="text-black" numberOfLines={1}>
              {title}
            </Text>
            <Text variant="caption" classN={`text-[${colors.primary}]`}>
              {messagesQuery.isFetching ? 'Updating…' : 'Task chat'}
            </Text>
          </View>
        </View>

        <ScrollView
          ref={scrollRef}
          style={tw`flex-1 px-4`}
          contentContainerStyle={{ paddingVertical: 16, flexGrow: 1 }}
          showsVerticalScrollIndicator={false}
          onContentSizeChange={() => scrollRef.current?.scrollToEnd({ animated: false })}>
          {messagesQuery.isPending ? (
            <View style={tw`gap-6`}>
              <SkeletonText lines={2} />
              <SkeletonText lines={1} index={2} />
              <SkeletonText lines={3} index={3} />
            </View>
          ) : messagesQuery.isError ? (
            <ErrorState
              body={errorMessage(messagesQuery.error)}
              onRetry={() => messagesQuery.refetch()}
            />
          ) : messages.length === 0 ? (
            <EmptyState
              illustration={<QuietConversation />}
              title="Say hello"
              body="Agree the details here. Keep payment on Sidekick — it’s the only way your escrow is protected."
            />
          ) : (
            messages.map((message) => (
              <MessageRow
                key={message.id}
                message={message}
                isMine={message.sender_id === currentUserId}
                onRetry={() => send(message.body ?? '', message.id)}
              />
            ))
          )}
        </ScrollView>

        <View
          style={tw`flex-row items-center gap-2.5 border-t border-[${colors.gray100}] px-4 py-3`}>
          <TextInput
            value={draft}
            onChangeText={setDraft}
            placeholder="Type a message"
            placeholderTextColor={colors.gray500}
            multiline
            maxLength={2000}
            onSubmitEditing={() => send(draft)}
            style={[
              tw`flex-1 rounded-3xl px-4 py-3 text-black`,
              { maxHeight: 110, minHeight: 46, backgroundColor: colors.gray50 },
            ]}
          />
          <PressableScale
            onPress={() => send(draft)}
            disabled={!draft.trim() || sendMessage.isPending}
            style={[
              tw`h-11 w-11 items-center justify-center rounded-full`,
              {
                backgroundColor: draft.trim() ? colors.primary : colors.primaryDisabled,
              },
            ]}>
            {sendMessage.isPending ? (
              <Spinner size={18} color={colors.white} />
            ) : (
              <Send size={18} color="white" />
            )}
          </PressableScale>
        </View>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}

/**
 * One message.
 *
 * A message still in flight sits at reduced opacity, and a failed one keeps its
 * text and gains a retry — the two states people actually need to tell apart.
 */
function MessageRow({
  message,
  isMine,
  onRetry,
}: {
  message: PendingMessage;
  isMine: boolean;
  onRetry: () => void;
}) {
  if (message.type === 'system') {
    return (
      <Animated.View entering={FadeIn.duration(220)} style={tw`my-3 items-center`}>
        <View style={[tw`rounded-full px-4 py-2`, { backgroundColor: colors.gray100 }]}>
          <Text variant="caption" classN={`text-center text-[${colors.gray500}]`}>
            {message.body}
          </Text>
        </View>
      </Animated.View>
    );
  }

  return (
    <Animated.View
      entering={FadeIn.duration(200)}
      style={[
        tw`mt-2 ${isMine ? 'items-end' : 'items-start'}`,
        message.pending && { opacity: 0.6 },
      ]}>
      <View
        style={[
          tw`max-w-[80%] rounded-2xl px-4 py-3`,
          { backgroundColor: isMine ? colors.primaryTint : colors.gray50 },
          message.failed ? { borderWidth: 1, borderColor: colors.danger } : undefined,
        ]}>
        <Text variant="body" classN="text-black">
          {message.body}
        </Text>
      </View>

      {message.failed ? (
        <PressableScale onPress={onRetry} hitSlop={8} style={tw`mt-1 flex-row items-center gap-1`}>
          <RotateCw size={11} color={colors.danger} />
          <Text variant="micro" fontWeight="medium" classN={`text-[${colors.danger}]`}>
            Not sent — tap to retry
          </Text>
        </PressableScale>
      ) : (
        <Text variant="micro" fontWeight="normal" classN={`mt-1 text-[${colors.gray500}]`}>
          {message.pending ? 'Sending…' : messageTime(message.created_at)}
        </Text>
      )}
    </Animated.View>
  );
}
