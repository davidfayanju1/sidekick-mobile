import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { idempotencyKey, qk } from '@/lib/api/keys';
import type { Escrow, Quote, Task } from '@/lib/api/schemas';
import { api } from '@/lib/axios';

export interface CreateTaskBody {
  title: string;
  description: string;
  category: string;
  location_approx: string;
  location_exact: string;
  location_lat?: number;
  location_lng?: number;
  location_exact_lat?: number;
  location_exact_lng?: number;
  timing_type: 'asap' | 'specific_date' | 'flexible_range';
  scheduled_for?: string | null;
  flexible_from?: string | null;
  flexible_to?: string | null;
  /** Minor units (pence). */
  budget: number;
}

export function useTask(taskId: string | undefined) {
  return useQuery({
    queryKey: qk.tasks.detail(taskId ?? ''),
    enabled: !!taskId,
    queryFn: async () => {
      const { data } = await api.get<{ task: Task }>(`/tasks/${taskId}`);
      return data.task;
    },
  });
}

export function useDrafts() {
  return useQuery({
    queryKey: qk.tasks.drafts,
    queryFn: async () => {
      const { data } = await api.get<{ tasks: Task[] }>('/me/drafts');
      return data.tasks ?? [];
    },
  });
}

export function useCreateTask() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (body: CreateTaskBody) => {
      const { data } = await api.post<{ task: Task }>('/tasks', body);
      return data.task;
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: qk.tasks.all }),
  });
}

export function useUpdateTask(taskId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (patch: Partial<CreateTaskBody>) => {
      const { data } = await api.patch<{ task: Task }>(`/tasks/${taskId}`, patch);
      return data.task;
    },
    onSuccess: (task) => {
      queryClient.setQueryData(qk.tasks.detail(taskId), task);
      queryClient.invalidateQueries({ queryKey: qk.tasks.all });
    },
  });
}

export function useDeleteTask() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (taskId: string) => {
      await api.delete(`/tasks/${taskId}`);
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: qk.tasks.all }),
  });
}

/** Autosaves the wizard so a half-filled task survives the app being closed. */
export function useSaveDraft(taskId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (payload: Record<string, unknown>) => {
      const { data } = await api.put<{ task: Task }>(`/tasks/${taskId}/draft`, payload);
      return data.task;
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: qk.tasks.drafts }),
  });
}

/**
 * Platform fee for a budget. Quoted server-side so the fee rule can change
 * without shipping an app update — never compute the fee on the client.
 */
export function useFeeQuote(budgetPence: number | undefined) {
  return useQuery({
    queryKey: qk.tasks.quote(budgetPence ?? 0),
    enabled: !!budgetPence && budgetPence > 0,
    staleTime: 5 * 60_000,
    queryFn: async () => {
      const { data } = await api.get<Quote>('/fees/quote', { params: { budget: budgetPence } });
      return data;
    },
  });
}

/** Funds escrow, which is what actually publishes a draft task. */
export function useFundTask(taskId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (paymentMethodId: string) => {
      const { data } = await api.post(`/tasks/${taskId}/fund`, {
        payment_method_id: paymentMethodId,
        idempotency_key: idempotencyKey(),
      });
      return data.fund;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: qk.tasks.all });
      queryClient.invalidateQueries({ queryKey: qk.finance.wallet });
    },
  });
}

export function useEscrow(taskId: string | undefined) {
  return useQuery({
    queryKey: qk.tasks.escrow(taskId ?? ''),
    enabled: !!taskId,
    queryFn: async () => {
      const { data } = await api.get<Escrow>(`/tasks/${taskId}/escrow`);
      return data;
    },
  });
}

/**
 * Two-step photo upload: ask for a signed URL, PUT the bytes straight to
 * storage, then tell the API it landed. Image bytes never pass through the API.
 */
export function useUploadTaskPhoto(taskId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ uri, type }: { uri: string; type: 'listing' | 'completion' }) => {
      const { data: grant } = await api.post<{ upload_url: string; key: string }>(
        `/tasks/${taskId}/photos/grant`,
        { type }
      );

      const blob = await (await fetch(uri)).blob();
      await fetch(grant.upload_url, {
        method: 'PUT',
        body: blob,
        headers: { 'Content-Type': blob.type || 'image/jpeg' },
      });

      const { data } = await api.post(`/tasks/${taskId}/photos/complete`, {
        key: grant.key,
        type,
      });
      return data;
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: qk.tasks.detail(taskId) }),
  });
}
