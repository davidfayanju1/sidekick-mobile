import { Tabs } from 'expo-router';

import { TabBar } from '@/components/UI/TabBar';
import { useMe } from '@/lib/api/auth';

export default function TabLayout() {
  // Every signed-in screen reads from the session user — the role it shows, the
  // setup steps it lists. Fetching it once here keeps those in step instead of
  // each tab racing its own copy.
  useMe();

  return (
    <Tabs screenOptions={{ headerShown: false }} tabBar={(props) => <TabBar {...props} />}>
      <Tabs.Screen name="index" options={{ title: 'My Tasks' }} />
      <Tabs.Screen name="post" options={{ title: 'Post' }} />
      <Tabs.Screen name="chats" options={{ title: 'Chats', href: null }} />
      <Tabs.Screen name="profile" options={{ title: 'Profile' }} />
    </Tabs>
  );
}
