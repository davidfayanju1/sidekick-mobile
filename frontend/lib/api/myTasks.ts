import { useQueries } from '@tanstack/react-query';
import * as React from 'react';

import { qk } from '@/lib/api/keys';
import type { Task } from '@/lib/api/schemas';
import { api } from '@/lib/axios';
import { useAuthStore } from '@/store/authStore';
import { useMyTasksStore } from '@/store/myTasksStore';

/**
 * The tasks this user has posted, assembled from locally-tracked ids.
 *
 * Stands in for the `GET /me/tasks` endpoint the API doesn't have — see
 * `store/myTasksStore.ts` for what that costs us. Everything below is written
 * so that swapping in a real endpoint later only changes this file.
 */
export function useMyPostedTasks() {
  const userId = useAuthStore((state) => state.user?.id);
  const ids = useMyTasksStore((state) => (userId ? (state.byUser[userId] ?? []) : []));
  const remove = useMyTasksStore((state) => state.remove);

  const results = useQueries({
    queries: ids.map((id) => ({
      queryKey: qk.tasks.detail(id),
      queryFn: async () => {
        const { data } = await api.get<{ task: Task }>(`/tasks/${id}`);
        return data.task;
      },
      // A task deleted server-side will 404 forever — don't keep retrying it.
      retry: false,
    })),
  });

  // Drop ids the server no longer knows about, so a deleted task doesn't leave
  // a permanent error row on the home screen.
  React.useEffect(() => {
    if (!userId) return;
    results.forEach((result, index) => {
      const status = (result.error as { status?: number } | null)?.status;
      if (status === 404 || status === 403) remove(userId, ids[index]);
    });
  }, [results, ids, userId, remove]);

  const tasks = React.useMemo(
    () => results.map((result) => result.data).filter((task): task is Task => !!task),
    [results]
  );

  return {
    tasks,
    // Only "pending" while nothing has arrived yet; one slow task shouldn't
    // hold back the ones that already loaded.
    isPending: ids.length > 0 && tasks.length === 0 && results.some((r) => r.isPending),
    isError: ids.length > 0 && results.every((r) => r.isError),
    isEmpty: ids.length === 0,
    refetch: () => results.forEach((result) => result.refetch()),
  };
}

/** Call after a successful create so the task shows up on the home screen. */
export function useTrackPostedTask() {
  const userId = useAuthStore((state) => state.user?.id);
  const add = useMyTasksStore((state) => state.add);
  return React.useCallback(
    (taskId: string) => {
      if (userId) add(userId, taskId);
    },
    [userId, add]
  );
}
