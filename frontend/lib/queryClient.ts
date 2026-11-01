import { QueryClient, focusManager } from '@tanstack/react-query';
import { isAxiosError } from 'axios';
import * as React from 'react';
import { AppState, type AppStateStatus } from 'react-native';

/** Client errors won't fix themselves on a retry — only retry transport/5xx failures. */
function retry(failureCount: number, error: unknown): boolean {
  const status = isAxiosError(error) ? error.response?.status : undefined;
  if (status && status >= 400 && status < 500) return false;
  return failureCount < 2;
}

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry,
      staleTime: 30_000,
      gcTime: 5 * 60_000,
      refetchOnWindowFocus: true,
      refetchOnReconnect: true,
    },
    mutations: { retry: false },
  },
});

/**
 * React Query's default focus detection is web-only; on native we map it to
 * AppState so queries refresh when the app comes back to the foreground.
 */
export function useAppStateFocusManager() {
  React.useEffect(() => {
    const subscription = AppState.addEventListener('change', (status: AppStateStatus) => {
      focusManager.setFocused(status === 'active');
    });
    return () => subscription.remove();
  }, []);
}
