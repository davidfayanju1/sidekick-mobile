import { Tabs } from 'expo-router';

import { TabBar } from '@/components/UI/TabBar';

export default function TabLayout() {
  return (
    <Tabs screenOptions={{ headerShown: false }} tabBar={(props) => <TabBar {...props} />}>
      <Tabs.Screen name="index" options={{ title: 'My Tasks' }} />
      <Tabs.Screen name="post" options={{ title: 'Post' }} />
      <Tabs.Screen name="chats" options={{ title: 'Chats' }} />
      <Tabs.Screen name="profile" options={{ title: 'Profile' }} />
    </Tabs>
  );
}
