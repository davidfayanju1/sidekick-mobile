import { useMutation, useQueryClient } from '@tanstack/react-query';

import { qk } from '@/lib/api/keys';
import { api } from '@/lib/axios';

/** Every completion step moves money or status, so they all invalidate the same branches. */
function useExecutionMutation<TArgs>(
  taskId: string,
  mutationFn: (args: TArgs) => Promise<unknown>
) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: qk.tasks.detail(taskId) });
      queryClient.invalidateQueries({ queryKey: qk.tasks.escrow(taskId) });
      queryClient.invalidateQueries({ queryKey: qk.tasks.all });
      queryClient.invalidateQueries({ queryKey: qk.finance.wallet });
      queryClient.invalidateQueries({ queryKey: qk.messaging.messages(taskId) });
    },
  });
}

/** Worker marks the job done. Starts the poster's 72-hour confirmation window. */
export function useCompleteTask(taskId: string) {
  return useExecutionMutation(taskId, async (body: { photo_urls?: string[] } = {}) => {
    const { data } = await api.post(`/tasks/${taskId}/complete`, body);
    return data;
  });
}

/** Poster confirms, which releases escrow to the worker immediately. */
export function useConfirmTask(taskId: string) {
  return useExecutionMutation(taskId, async () => {
    const { data } = await api.post(`/tasks/${taskId}/confirm`, {});
    return data;
  });
}

/** Freezes escrow and opens a dispute for an admin to resolve. */
export function useDisputeTask(taskId: string) {
  return useExecutionMutation(taskId, async (body: { reason: string; description: string }) => {
    const { data } = await api.post(`/tasks/${taskId}/dispute`, body);
    return data;
  });
}

export function useCancelTask(taskId: string) {
  return useExecutionMutation(taskId, async (body: { reason?: string } = {}) => {
    const { data } = await api.post(`/tasks/${taskId}/cancel`, body);
    return data;
  });
}

export function useSubmitDisputeEvidence(disputeId: string) {
  return useMutation({
    mutationFn: async (body: { evidence_urls: string[]; note?: string }) => {
      const { data } = await api.post(`/disputes/${disputeId}/evidence`, body);
      return data;
    },
  });
}
