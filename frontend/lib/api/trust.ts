import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { qk } from '@/lib/api/keys';
import type { Report, Review } from '@/lib/api/schemas';
import { api } from '@/lib/axios';

// ── Reviews ──────────────────────────────────────────────────────────────────

/**
 * Reviews for a user. Double-blind: nothing is published until both sides have
 * reviewed or 14 days pass, so an empty list can mean "not yet visible".
 */
export function useUserReviews(userId: string | undefined) {
  return useQuery({
    queryKey: qk.users.reviews(userId ?? ''),
    enabled: !!userId,
    queryFn: async () => {
      const { data } = await api.get<{ reviews: Review[] }>(`/users/${userId}/reviews`);
      return data.reviews ?? [];
    },
  });
}

export function useMyReviews() {
  return useQuery({
    queryKey: qk.reviews.mine,
    queryFn: async () => {
      const { data } = await api.get<{ reviews: Review[] }>('/me/reviews');
      return data.reviews ?? [];
    },
  });
}

export function useLeaveReview(taskId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (body: { rating: number; comment?: string }) => {
      const { data } = await api.post<{ review: Review }>(`/tasks/${taskId}/reviews`, body);
      return data.review;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: qk.reviews.mine });
      queryClient.invalidateQueries({ queryKey: qk.tasks.detail(taskId) });
    },
  });
}

// ── ID verification ──────────────────────────────────────────────────────────

/**
 * Uploads an ID document and opens a verification case.
 *
 * Same signed-URL dance as task photos — an identity document especially
 * should go straight to storage rather than through the API.
 */
export function useSubmitVerification() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      uri,
      documentType,
    }: {
      uri: string;
      documentType: 'passport' | 'driving_licence' | 'national_id';
    }) => {
      const { data: grant } = await api.post<{ upload_url: string; key: string }>(
        '/me/avatar/upload-url',
        { purpose: 'id_document' }
      );

      const blob = await (await fetch(uri)).blob();
      await fetch(grant.upload_url, {
        method: 'PUT',
        body: blob,
        headers: { 'Content-Type': blob.type || 'image/jpeg' },
      });

      const { data } = await api.post<{ verification_id: string; status: string }>(
        '/me/verifications',
        { document_type: documentType, document_url: grant.key }
      );
      return data;
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: qk.auth.me }),
  });
}

// ── Reports ──────────────────────────────────────────────────────────────────

export function useMyReports() {
  return useQuery({
    queryKey: qk.trust.reports,
    queryFn: async () => {
      const { data } = await api.get<{ reports: Report[] }>('/me/reports');
      return data.reports ?? [];
    },
  });
}

export function useReportUser() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (body: {
      reported_user_id: string;
      reason: string;
      detail?: string;
      task_id?: string;
      conversation_id?: string;
    }) => {
      const { data } = await api.post<{ report: Report }>('/reports', body);
      return data.report;
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: qk.trust.reports }),
  });
}
