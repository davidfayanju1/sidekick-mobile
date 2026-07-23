import { router } from 'expo-router';
import { Search } from 'lucide-react-native';
import * as React from 'react';
import { Pressable, ScrollView, TextInput, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import Text from '@/components/UI/Text';
import { tw } from '@/lib/tw';

const ACCENT_TEAL = '#489A9F';
const MUTED = '#9AA0A6';
const SEARCH_BG = '#EFF7F6';
const UNREAD_RED = '#E5484D';

type ChatTab = 'active' | 'completed';

type ChatPreview = {
  id: string;
  name: string;
  fullName: string;
  initial: string;
  avatarBg: string;
  avatarText: string;
  online: boolean;
  taskTitle: string;
  tab: ChatTab;
  statusLabel?: string;
  statusBg?: string;
  statusText?: string;
  lastMessage: string;
  time: string;
  unread?: number;
};

export const CHATS: ChatPreview[] = [
  {
    id: '1',
    name: 'Ade. B',
    fullName: 'Ade Balogun',
    initial: 'A',
    avatarBg: '#F0DCC8',
    avatarText: '#B5762E',
    online: true,
    taskTitle: 'Deep Clean Task',
    tab: 'active',
    statusLabel: 'In Progress',
    statusBg: '#DCF5E3',
    statusText: '#2F9E56',
    lastMessage: "I'm on my way - eta 15mins",
    time: '2m',
    unread: 2,
  },
  {
    id: '2',
    name: 'Kemi O.',
    fullName: 'Kemi Okafor',
    initial: 'K',
    avatarBg: '#DCE8FB',
    avatarText: '#3B6FD1',
    online: false,
    taskTitle: 'Grocery Pickup',
    tab: 'active',
    lastMessage: 'Can you do ₦4,200 instead?',
    time: '1h',
  },
  {
    id: '3',
    name: 'Tunde J.',
    fullName: 'Tunde Johnson',
    initial: 'T',
    avatarBg: '#DCF5E3',
    avatarText: '#2F9E56',
    online: false,
    taskTitle: 'Laptop Repair',
    tab: 'completed',
    statusLabel: 'Done',
    statusBg: '#DCE8FB',
    statusText: '#3B6FD1',
    lastMessage: 'Thank you! Great working with you',
    time: 'Yesterday',
  },
];

function ChatRow({ chat }: { chat: ChatPreview }) {
  return (
    <Pressable
      onPress={() => router.push(`/chat/${chat.id}`)}
      style={tw`flex-row items-center gap-3 border-b border-[#F0F1F2] py-3.5`}>
      <View
        style={[
          tw`h-12 w-12 items-center justify-center rounded-full`,
          { backgroundColor: chat.avatarBg },
        ]}>
        <Text fontWeight="bold" fontSize={16} classN={`text-[${chat.avatarText}]`}>
          {chat.initial}
        </Text>
      </View>

      <View style={tw`flex-1`}>
        <View style={tw`flex-row items-center gap-2`}>
          <Text fontWeight="bold" fontSize={15} classN="text-black">
            {chat.name}
          </Text>
          {chat.statusLabel && (
            <View
              style={[tw`rounded-full px-2 py-0.5`, { backgroundColor: chat.statusBg }]}>
              <Text fontSize={10} classN={`text-[${chat.statusText}]`}>
                {chat.statusLabel}
              </Text>
            </View>
          )}
        </View>
        <Text fontSize={13} classN="mt-0.5 text-[#9AA0A6]" numberOfLines={1}>
          {chat.lastMessage}
        </Text>
      </View>

      <View style={tw`items-end gap-1.5`}>
        <Text fontSize={11} classN="text-[#9AA0A6]">
          {chat.time}
        </Text>
        {!!chat.unread && (
          <View
            style={[
              tw`h-4.5 min-w-4.5 items-center justify-center rounded-full px-1`,
              { backgroundColor: UNREAD_RED },
            ]}>
            <Text fontWeight="bold" fontSize={10} classN="text-white">
              {chat.unread}
            </Text>
          </View>
        )}
      </View>
    </Pressable>
  );
}

export default function Chats() {
  const [activeTab, setActiveTab] = React.useState<ChatTab>('active');
  const [query, setQuery] = React.useState('');

  const chats = CHATS.filter(
    (chat) =>
      chat.tab === activeTab && chat.name.toLowerCase().includes(query.trim().toLowerCase())
  );

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

      <Text fontWeight="black" fontSize={24} classN="mt-3 px-4 text-black">
        Chats
      </Text>

      <View
        style={[
          tw`mx-4 mt-3 flex-row items-center gap-2 rounded-full px-4`,
          { height: 44, backgroundColor: SEARCH_BG },
        ]}>
        <Search size={17} color={MUTED} />
        <TextInput
          value={query}
          onChangeText={setQuery}
          placeholder="Search"
          placeholderTextColor={MUTED}
          style={tw`flex-1 text-black`}
        />
      </View>

      <View style={tw`mt-4 flex-row gap-5 px-4`}>
        <Pressable onPress={() => setActiveTab('active')}>
          <Text
            fontWeight={activeTab === 'active' ? 'bold' : 'normal'}
            fontSize={13}
            classN={activeTab === 'active' ? 'text-black' : 'text-[#9AA0A6]'}>
            Active
          </Text>
        </Pressable>
        <Pressable onPress={() => setActiveTab('completed')}>
          <Text
            fontWeight={activeTab === 'completed' ? 'bold' : 'normal'}
            fontSize={13}
            classN={activeTab === 'completed' ? 'text-black' : 'text-[#9AA0A6]'}>
            Completed
          </Text>
        </Pressable>
      </View>

      <ScrollView
        style={tw`mt-2 flex-1 px-4`}
        contentContainerStyle={{ paddingBottom: 110 }}
        showsVerticalScrollIndicator={false}>
        {chats.length === 0 ? (
          <View style={tw`mt-16 items-center`}>
            <Text fontSize={14} classN="text-[#9AA0A6]">
              No chats yet
            </Text>
          </View>
        ) : (
          chats.map((chat) => <ChatRow key={chat.id} chat={chat} />)
        )}
      </ScrollView>
    </SafeAreaView>
  );
}
