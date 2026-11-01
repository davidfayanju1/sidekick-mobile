import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { qk } from '@/lib/api/keys';
import type { Notification } from '@/lib/api/schemas';
import { api } from '@/lib/axios';

export function useNotifications() {
  return useQuery({
    queryKey: qk.notifications.list,
    queryFn: async () => {
      const { data } = await api.get<{ notifications: Notification[] }>('/me/notifications');
      return data.notifications ?? [];
    },
  });
}

/** Drives the tab-bar badge, so it polls independently of the list screen. */
export function useUnreadCount() {
  return useQuery({
    queryKey: qk.notifications.unreadCount,
    queryFn: async () => {
      const { data } = await api.get<{ count: number }>('/me/notifications/unread-count');
      return data.count ?? 0;
    },
    refetchInterval: 30_000,
  });
}

export function useMarkNotificationRead() {
  const queryClient = useQueryClient();
  const key = qk.notifications.list;

  return useMutation({
    mutationFn: async (id: string) => {
      await api.post(`/me/notifications/${id}/read`, {});
    },
    // The row should grey out the instant it's tapped — waiting for a round
    // trip to un-bold a notification reads as a missed tap.
    onMutate: async (id) => {
      await queryClient.cancelQueries({ queryKey: key });
      const previous = queryClient.getQueryData<Notification[]>(key);
      queryClient.setQueryData<Notification[]>(key, (current) =>
        (current ?? []).map((n) => (n.id === id ? { ...n, is_read: true } : n))
      );
      return { previous };
    },
    onError: (_error, _id, context) => {
      if (context?.previous) queryClient.setQueryData(key, context.previous);
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: key });
      queryClient.invalidateQueries({ queryKey: qk.notifications.unreadCount });
    },
  });
}

export function useMarkAllRead() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      await api.post('/me/notifications/read-all', {});
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: qk.notifications.list });
      queryClient.invalidateQueries({ queryKey: qk.notifications.unreadCount });
    },
  });
}

export interface NotificationPrefs {
  offers?: boolean;
  messages?: boolean;
  payments?: boolean;
  reminders?: boolean;
}

export function useNotificationPrefs() {
  return useQuery({
    queryKey: qk.notifications.prefs,
    queryFn: async () => {
      const { data } = await api.get<{ prefs: NotificationPrefs }>('/me/notification-prefs');
      return data.prefs ?? {};
    },
  });
}

export function useUpdateNotificationPrefs() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (prefs: NotificationPrefs) => {
      const { data } = await api.patch<{ prefs: NotificationPrefs }>(
        '/me/notification-prefs',
        prefs
      );
      return data.prefs;
    },
    onSuccess: (prefs) => queryClient.setQueryData(qk.notifications.prefs, prefs),
  });
}

/** Registers this device's Expo push token so the backend can fan out to it. */
export function useRegisterPushToken() {
  return useMutation({
    mutationFn: async (body: { token: string; platform: 'ios' | 'android' }) => {
      const { data } = await api.post('/me/push-tokens', body);
      return data;
    },
  });
}
