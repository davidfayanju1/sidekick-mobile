import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { qk } from '@/lib/api/keys';
import type { Conversation, Message } from '@/lib/api/schemas';
import { api } from '@/lib/axios';

/** A message that exists only on this device until the server confirms it. */
export interface PendingMessage extends Message {
  pending?: true;
  failed?: true;
}

export function useConversations() {
  return useQuery({
    queryKey: qk.messaging.conversations,
    queryFn: async () => {
      const { data } = await api.get<{ conversations: Conversation[] }>('/conversations');
      return data.conversations ?? [];
    },
    // The chat list is the one screen people leave open waiting for a reply.
    refetchInterval: 20_000,
  });
}

export function useMessages(conversationId: string | undefined) {
  return useQuery({
    queryKey: qk.messaging.messages(conversationId ?? ''),
    enabled: !!conversationId,
    queryFn: async () => {
      const { data } = await api.get<{ messages: Message[] }>(
        `/conversations/${conversationId}/messages`
      );
      return (data.messages ?? []) as PendingMessage[];
    },
    refetchInterval: 8_000,
  });
}

/**
 * Sends a message, showing it immediately.
 *
 * A chat that waits for a round trip before showing your own words feels
 * broken even on a fast connection. The message goes into the cache straight
 * away marked `pending`, gets swapped for the server's copy on success, and is
 * marked `failed` (not removed) if it doesn't land — losing someone's typing is
 * worse than showing it with a retry affordance.
 */
export function useSendMessage(conversationId: string, currentUserId: string | undefined) {
  const queryClient = useQueryClient();
  const key = qk.messaging.messages(conversationId);

  return useMutation({
    mutationFn: async ({ body }: { body: string; localId: string }) => {
      const { data } = await api.post<{ message: Message }>(
        `/conversations/${conversationId}/messages`,
        { body }
      );
      return data.message;
    },

    onMutate: async ({ body, localId }) => {
      await queryClient.cancelQueries({ queryKey: key });
      const previous = queryClient.getQueryData<PendingMessage[]>(key);

      const optimistic: PendingMessage = {
        id: localId,
        conversation_id: conversationId,
        sender_id: currentUserId ?? null,
        sender_name: null,
        type: 'text',
        body,
        system_event: null,
        attachment_url: null,
        created_at: new Date().toISOString(),
        pending: true,
      };

      queryClient.setQueryData<PendingMessage[]>(key, [...(previous ?? []), optimistic]);
      return { previous, localId };
    },

    onSuccess: (message, _variables, context) => {
      queryClient.setQueryData<PendingMessage[]>(key, (current) =>
        (current ?? []).map((m) => (m.id === context?.localId ? message : m))
      );
      queryClient.invalidateQueries({ queryKey: qk.messaging.conversations });
    },

    onError: (_error, _variables, context) => {
      queryClient.setQueryData<PendingMessage[]>(key, (current) =>
        (current ?? []).map((m) =>
          m.id === context?.localId ? { ...m, pending: undefined, failed: true } : m
        )
      );
    },
  });
}

export function useMarkRead(conversationId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (messageId: string) => {
      await api.post(`/messages/${messageId}/read`, {});
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: qk.messaging.conversations });
      queryClient.invalidateQueries({ queryKey: qk.messaging.messages(conversationId) });
    },
  });
}

export function useSendChatImage(conversationId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (uri: string) => {
      const { data: grant } = await api.post<{ upload_url: string; key: string }>(
        `/conversations/${conversationId}/images/grant`,
        {}
      );

      const blob = await (await fetch(uri)).blob();
      await fetch(grant.upload_url, {
        method: 'PUT',
        body: blob,
        headers: { 'Content-Type': blob.type || 'image/jpeg' },
      });

      const { data } = await api.post<{ message: Message }>(
        `/conversations/${conversationId}/images`,
        { key: grant.key }
      );
      return data.message;
    },
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: qk.messaging.messages(conversationId) }),
  });
}

export function useReportConversation(conversationId: string) {
  return useMutation({
    mutationFn: async (body: { reason: string; detail?: string }) => {
      const { data } = await api.post(`/conversations/${conversationId}/report`, body);
      return data;
    },
  });
}

export function useBlocks() {
  return useQuery({
    queryKey: qk.messaging.blocks,
    queryFn: async () => {
      const { data } = await api.get<{ blocks: { user_id: string }[] }>('/blocks');
      return data.blocks ?? [];
    },
  });
}

export function useBlockUser() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ userId, blocked }: { userId: string; blocked: boolean }) => {
      if (blocked) await api.post(`/users/${userId}/block`, {});
      else await api.delete(`/users/${userId}/block`);
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: qk.messaging.blocks }),
  });
}
