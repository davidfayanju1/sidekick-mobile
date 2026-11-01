import AsyncStorage from '@react-native-async-storage/async-storage';
import { create } from 'zustand';
import { createJSONStorage, persist } from 'zustand/middleware';

/**
 * Tasks this device has posted.
 *
 * The API has no "list tasks I posted" endpoint, and the discovery feed
 * deliberately excludes your own tasks (`t.poster_id <> $N` in
 * internal/discovery/discovery.go), so the Hero home screen has nothing to read
 * from. This keeps the ids locally and the screen fetches each one by id.
 *
 * Known limits of doing it this way, both fixed by a `GET /me/tasks` endpoint:
 *   - a new device, or a reinstall, starts with an empty list
 *   - it costs one request per task instead of one for the page
 *
 * Ids are scoped by user so switching accounts on one device doesn't show the
 * previous account's tasks.
 */
interface MyTasksState {
  /** Task ids keyed by the user that created them. */
  byUser: Record<string, string[]>;
  add: (userId: string, taskId: string) => void;
  remove: (userId: string, taskId: string) => void;
  idsFor: (userId: string | undefined) => string[];
  clear: (userId: string) => void;
}

export const useMyTasksStore = create<MyTasksState>()(
  persist(
    (set, get) => ({
      byUser: {},

      add: (userId, taskId) =>
        set((state) => {
          const existing = state.byUser[userId] ?? [];
          if (existing.includes(taskId)) return state;
          // Newest first — the home screen shows them in this order.
          return { byUser: { ...state.byUser, [userId]: [taskId, ...existing] } };
        }),

      remove: (userId, taskId) =>
        set((state) => ({
          byUser: {
            ...state.byUser,
            [userId]: (state.byUser[userId] ?? []).filter((id) => id !== taskId),
          },
        })),

      idsFor: (userId) => (userId ? (get().byUser[userId] ?? []) : []),

      clear: (userId) => set((state) => ({ byUser: { ...state.byUser, [userId]: [] } })),
    }),
    {
      name: 'sidekick.my-tasks',
      storage: createJSONStorage(() => AsyncStorage),
    }
  )
);
