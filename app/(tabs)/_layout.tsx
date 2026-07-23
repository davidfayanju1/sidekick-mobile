import { Tabs } from 'expo-router';
import * as React from 'react';

import ProfileSidebar from '@/components/UI/ProfileSidebar';
import { TabBar } from '@/components/UI/TabBar';

export default function TabLayout() {
  const [sidebarVisible, setSidebarVisible] = React.useState(false);

  return (
    <>
      <Tabs
        screenOptions={{ headerShown: false }}
        tabBar={(props) => <TabBar {...props} onProfilePress={() => setSidebarVisible(true)} />}>
        <Tabs.Screen name="index" options={{ title: 'My Tasks' }} />
        <Tabs.Screen name="post" options={{ title: 'Post' }} />
        <Tabs.Screen name="chats" options={{ title: 'Chats' }} />
        <Tabs.Screen name="profile" options={{ title: 'Profile' }} />
      </Tabs>

      <ProfileSidebar visible={sidebarVisible} onClose={() => setSidebarVisible(false)} />
    </>
  );
}
