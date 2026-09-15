import { create } from 'zustand';

import type { Task } from '@/components/UI/TaskParts';

interface TaskState {
  // A Sidekick can only ever have one active task — until it's marked
  // complete, they can't accept another one from the browse deck.
  sidekickCurrentTask: Task | null;
  // Real timestamps for the timeline on Task Details — lets it show
  // "Matched 2 mins ago" instead of a static label.
  sidekickTaskMatchedAt: number | null;
  sidekickTaskStartedAt: number | null;
  acceptTask: (task: Task) => void;
  startCurrentTask: () => void;
  completeCurrentTask: () => void;
}

export const useTaskStore = create<TaskState>((set) => ({
  sidekickCurrentTask: null,
  sidekickTaskMatchedAt: null,
  sidekickTaskStartedAt: null,
  acceptTask: (task) => set({ sidekickCurrentTask: task, sidekickTaskMatchedAt: Date.now() }),
  startCurrentTask: () =>
    set((state) => ({
      sidekickCurrentTask: state.sidekickCurrentTask && {
        ...state.sidekickCurrentTask,
        status: 'in_progress',
        statusLabel: 'In Progress',
        currentStep: 2,
      },
      sidekickTaskStartedAt: Date.now(),
    })),
  completeCurrentTask: () =>
    set({ sidekickCurrentTask: null, sidekickTaskMatchedAt: null, sidekickTaskStartedAt: null }),
}));
