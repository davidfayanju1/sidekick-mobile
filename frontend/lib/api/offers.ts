import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { qk } from '@/lib/api/keys';
import type { Offer, OfferWithWorker } from '@/lib/api/schemas';
import { api } from '@/lib/axios';

/** Offers on a task — poster's view, with each worker's reputation attached. */
export function useTaskOffers(taskId: string | undefined) {
  return useQuery({
    queryKey: qk.offers.forTask(taskId ?? ''),
    enabled: !!taskId,
    queryFn: async () => {
      const { data } = await api.get<{ offers: OfferWithWorker[] }>(`/tasks/${taskId}/offers`);
      return data.offers ?? [];
    },
  });
}

/** Offers the signed-in worker has made. */
export function useMyOffers() {
  return useQuery({
    queryKey: qk.offers.mine,
    queryFn: async () => {
      const { data } = await api.get<{ offers: Offer[] }>('/me/offers');
      return data.offers ?? [];
    },
  });
}

/** Omitting `amount` offers at the poster's budget; sending one is a counter. */
export function useMakeOffer(taskId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (body: { amount?: number; message?: string }) => {
      const { data } = await api.post<{ offer: Offer }>(`/tasks/${taskId}/offers`, body);
      return data.offer;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: qk.offers.mine });
      queryClient.invalidateQueries({ queryKey: qk.offers.forTask(taskId) });
      queryClient.invalidateQueries({ queryKey: qk.tasks.detail(taskId) });
    },
  });
}

/** Invalidates everything an offer transition can touch, including the chat it may have created. */
function useOfferMutation<TArgs>(mutationFn: (args: TArgs) => Promise<unknown>, taskId?: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: qk.offers.mine });
      queryClient.invalidateQueries({ queryKey: qk.tasks.all });
      queryClient.invalidateQueries({ queryKey: qk.messaging.conversations });
      if (taskId) queryClient.invalidateQueries({ queryKey: qk.offers.forTask(taskId) });
    },
  });
}

export function useAcceptOffer(taskId?: string) {
  return useOfferMutation(async (offerId: string) => {
    const { data } = await api.post(`/offers/${offerId}/accept`, {});
    return data.accept;
  }, taskId);
}

export function useDeclineOffer(taskId?: string) {
  return useOfferMutation(
    async ({ offerId, reason }: { offerId: string; reason?: string }) =>
      (await api.post(`/offers/${offerId}/decline`, { reason })).data,
    taskId
  );
}

/** The poster's one-round counter. The worker then accepts or declines it. */
export function useCounterOffer(taskId?: string) {
  return useOfferMutation(
    async ({ offerId, amount }: { offerId: string; amount: number }) =>
      (await api.post(`/offers/${offerId}/counter`, { amount })).data,
    taskId
  );
}

export function useRespondToCounter(taskId?: string) {
  return useOfferMutation(
    async ({ offerId, action }: { offerId: string; action: 'accept' | 'decline' }) =>
      (await api.post(`/offers/${offerId}/respond`, { action })).data,
    taskId
  );
}

export function useWithdrawOffer(taskId?: string) {
  return useOfferMutation(
    async (offerId: string) => (await api.post(`/offers/${offerId}/withdraw`, {})).data,
    taskId
  );
}

/** The conversation a task's accepted offer created, if there is one. */
export function useTaskConversation(taskId: string | undefined) {
  return useQuery({
    queryKey: qk.messaging.forTask(taskId ?? ''),
    enabled: !!taskId,
    queryFn: async () => {
      const { data } = await api.get<{ conversation_id: string | null }>(
        `/tasks/${taskId}/conversation`
      );
      return data.conversation_id;
    },
  });
}
