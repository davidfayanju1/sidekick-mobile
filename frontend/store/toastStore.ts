import { create } from 'zustand';

export type ToastVariant = 'success' | 'error' | 'info' | 'warning';

export interface Toast {
  id: string;
  variant: ToastVariant;
  title: string;
  description?: string;
  /** Milliseconds on screen. 0 pins it until dismissed. */
  duration: number;
  action?: { label: string; onPress: () => void };
}

export interface ToastOptions {
  description?: string;
  duration?: number;
  action?: { label: string; onPress: () => void };
}

/**
 * One toast is on screen at a time; the rest wait their turn.
 *
 * Stacking them looked like a wall of banners covering the screen, and the
 * cards behind read as disabled rather than queued. Showing one and promoting
 * the next on dismiss is what Stripe, Linear and Revolut all do.
 *
 * The queue is capped so a burst can't trap the user behind a backlog.
 */
const MAX_QUEUED = 3;

const DEFAULT_DURATION: Record<ToastVariant, number> = {
  success: 2600,
  info: 3000,
  warning: 4000,
  // Failures need longer: the user has to read what went wrong, not just notice it.
  error: 5000,
};

interface ToastState {
  /** Front of the queue is what's on screen; the rest follow. */
  toasts: Toast[];
  push: (toast: Omit<Toast, 'id' | 'duration'> & { duration?: number }) => string;
  dismiss: (id: string) => void;
  clear: () => void;
}

let counter = 0;

export const useToastStore = create<ToastState>((set) => ({
  toasts: [],

  push: ({ variant, title, description, action, duration }) => {
    const id = `toast-${++counter}`;
    set((state) => ({
      toasts: [
        ...state.toasts,
        {
          id,
          variant,
          title,
          description,
          action,
          duration: duration ?? DEFAULT_DURATION[variant],
        },
        // Drop from the front: the oldest unseen message is the most stale.
      ].slice(-MAX_QUEUED),
    }));
    return id;
  },

  dismiss: (id) => set((state) => ({ toasts: state.toasts.filter((t) => t.id !== id) })),

  clear: () => set({ toasts: [] }),
}));

/**
 * Imperative API — importable anywhere, including outside React.
 *
 * Deliberately not a hook: most toasts fire from a mutation's `onError`, which
 * is a plain callback with no component around it.
 */
export const toast = {
  success: (title: string, options?: ToastOptions) =>
    useToastStore.getState().push({ variant: 'success', title, ...options }),
  error: (title: string, options?: ToastOptions) =>
    useToastStore.getState().push({ variant: 'error', title, ...options }),
  info: (title: string, options?: ToastOptions) =>
    useToastStore.getState().push({ variant: 'info', title, ...options }),
  warning: (title: string, options?: ToastOptions) =>
    useToastStore.getState().push({ variant: 'warning', title, ...options }),
  dismiss: (id: string) => useToastStore.getState().dismiss(id),
  clear: () => useToastStore.getState().clear(),
};
