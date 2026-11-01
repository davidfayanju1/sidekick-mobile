import { useQueries } from '@tanstack/react-query';
import { router } from 'expo-router';
import { Search } from 'lucide-react-native';
import * as React from 'react';
import { ScrollView, TextInput, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { EmptyState, ErrorState } from '@/components/UI/EmptyState';
import { QuietConversation } from '@/components/UI/illustrations';
import { SkeletonChatRow } from '@/components/UI/Skeleton';
import Text from '@/components/UI/Text';
import { toChatPreview } from '@/lib/api/adapters';
import { qk } from '@/lib/api/keys';
import { useConversations } from '@/lib/api/messaging';
import type { Task } from '@/lib/api/schemas';
import { api, errorMessage } from '@/lib/axios';
import { tw } from '@/lib/tw';
import { colors } from '@/theme/palette';
import { PressableScale } from '@/components/UI/PressableScale';
import { StaggerItem } from '@/components/UI/Stagger';

type ChatTab = 'active' | 'completed';

/** The row shape produced by `toChatPreview` — see lib/api/adapters.ts. */
type ChatPreview = ReturnType<typeof toChatPreview>;

/** Avatar tints, picked from the task id so a conversation keeps the same colour. */
const AVATAR_TINTS = [
  { bg: colors.orangeTint, text: colors.orangeDark },
  { bg: colors.primaryTint, text: colors.primary },
  { bg: colors.greenTint, text: colors.green },
  { bg: colors.dangerTint, text: colors.danger },
] as const;

function avatarTint(id: string) {
  const sum = [...id].reduce((total, character) => total + character.charCodeAt(0), 0);
  return AVATAR_TINTS[sum % AVATAR_TINTS.length];
}

function ChatRow({ chat }: { chat: ChatPreview }) {
  return (
    <PressableScale
      onPress={() => router.push(`/chat/${chat.id}`)}
      style={tw`flex-row items-center gap-3 border-b border-[${colors.gray100}] py-3.5`}>
      <View
        style={[
          tw`h-12 w-12 items-center justify-center rounded-full`,
          { backgroundColor: avatarTint(chat.id).bg },
        ]}>
        <Text variant="subhead" fontWeight="bold" classN={`text-[${avatarTint(chat.id).text}]`}>
          {chat.initial}
        </Text>
      </View>

      <View style={tw`flex-1`}>
        <View style={tw`flex-row items-center gap-2`}>
          <Text variant="body" fontWeight="medium" classN="text-black">
            {chat.title}
          </Text>
          {chat.statusLabel && (
            <View style={[tw`rounded-full px-2 py-0.5`, { backgroundColor: colors.primaryTint }]}>
              <Text variant="micro" fontWeight="normal" classN={`text-[${colors.primary}]`}>
                {chat.statusLabel}
              </Text>
            </View>
          )}
        </View>
        <Text variant="caption" classN={`mt-0.5 text-[${colors.gray500}]`} numberOfLines={1}>
          {chat.lastMessage}
        </Text>
      </View>

      <View style={tw`items-end gap-1.5`}>
        <Text variant="micro" fontWeight="normal" classN={`text-[${colors.gray500}]`}>
          {chat.time}
        </Text>
        {!!chat.unread && (
          <View
            style={[
              tw`h-4.5 min-w-4.5 items-center justify-center rounded-full px-1`,
              { backgroundColor: colors.danger },
            ]}>
            <Text variant="micro" classN="text-white">
              {chat.unread}
            </Text>
          </View>
        )}
      </View>
    </PressableScale>
  );
}

export default function Chats() {
  const [activeTab, setActiveTab] = React.useState<ChatTab>('active');
  const [query, setQuery] = React.useState('');

  const conversations = useConversations();

  // The conversation payload carries no task status, so the Active/Completed
  // split needs the tasks themselves. These are already cached by the task
  // screens, so in practice most of these resolve without a request.
  const taskQueries = useQueries({
    queries: (conversations.data ?? []).map((conversation) => ({
      queryKey: qk.tasks.detail(conversation.task_id),
      queryFn: async () => {
        const { data } = await api.get<{ task: Task }>(`/tasks/${conversation.task_id}`);
        return data.task;
      },
      retry: false,
    })),
  });

  const chats = React.useMemo(() => {
    const rows = (conversations.data ?? []).map((conversation, index) =>
      toChatPreview(conversation, taskQueries[index]?.data)
    );
    const search = query.trim().toLowerCase();
    return rows.filter(
      (chat) =>
        (activeTab === 'completed' ? chat.isCompleted : !chat.isCompleted) &&
        chat.title.toLowerCase().includes(search)
    );
  }, [conversations.data, taskQueries, query, activeTab]);

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

      <Text variant="headline" fontWeight="black" classN="mt-2 px-4 text-black">
        Chats
      </Text>

      <View
        style={[
          tw`mx-4 mt-2 flex-row items-center gap-2 rounded-full px-4`,
          { height: 44, backgroundColor: colors.primarySurface },
        ]}>
        <Search size={17} color={colors.gray500} />
        <TextInput
          value={query}
          onChangeText={setQuery}
          placeholder="Search"
          placeholderTextColor={colors.gray500}
          style={tw`flex-1 text-black`}
        />
      </View>

      <View style={tw`mt-3 flex-row gap-5 px-4`}>
        <PressableScale onPress={() => setActiveTab('active')}>
          <Text
            variant="caption"
            fontWeight={activeTab === 'active' ? 'bold' : 'normal'}

            classN={activeTab === 'active' ? 'text-black' : `text-[${colors.gray500}]`}>
            Active
          </Text>
        </PressableScale>
        <PressableScale onPress={() => setActiveTab('completed')}>
          <Text
            variant="caption"
            fontWeight={activeTab === 'completed' ? 'bold' : 'normal'}

            classN={activeTab === 'completed' ? 'text-black' : `text-[${colors.gray500}]`}>
            Completed
          </Text>
        </PressableScale>
      </View>

      <ScrollView
        style={tw`mt-2 flex-1 px-4`}
        contentContainerStyle={{ paddingBottom: 110 }}
        showsVerticalScrollIndicator={false}>
        {conversations.isPending ? (
          <View style={tw`mt-2`}>
            {[0, 1, 2, 3].map((index) => (
              <SkeletonChatRow key={index} index={index} />
            ))}
          </View>
        ) : conversations.isError ? (
          <ErrorState
            body={errorMessage(conversations.error)}
            onRetry={() => conversations.refetch()}
          />
        ) : chats.length === 0 ? (
          <EmptyState
            illustration={<QuietConversation />}
            title={
              query
                ? 'Nothing matches that'
                : activeTab === 'completed'
                  ? 'No finished tasks yet'
                  : 'No conversations yet'
            }
            body={
              query
                ? 'Try a different task name.'
                : activeTab === 'completed'
                  ? 'Chats move here once a task is confirmed and paid.'
                  : 'A chat opens automatically the moment an offer is accepted.'
            }
          />
        ) : (
          chats.map((chat, i) => (
            <StaggerItem key={chat.id} index={i}>
              <ChatRow chat={chat} />
            </StaggerItem>
          ))
        )}
      </ScrollView>
    </SafeAreaView>
  );
}
