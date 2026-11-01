import { router } from 'expo-router';
import * as React from 'react';

import { queryClient } from '@/lib/queryClient';
import { useAuthStore } from '@/store/authStore';
import { useLocationStore } from '@/store/locationStore';
import { toast } from '@/store/toastStore';

/**
 * Sends the user back to the landing screen whenever their session ends.
 *
 * Tapping Sign out is only one of the ways that happens. The axios interceptor
 * clears the tokens when a refresh fails, and `/me` ends the session when the
 * account behind it has gone. Those paths used to leave the app sitting on a
 * signed-in screen with no credentials, where every request 401s and nothing
 * on screen says why — so the exit lives here, once, instead of at each place
 * a session can die.
 *
 * It keys off the `sessionEnd` reason rather than `isAuthenticated` being
 * false, which is what makes it different from the route guards this app had
 * before: a session that *ended* is a specific event, whereas not being signed
 * in is the normal state of someone half-way through sign-up.
 */
export function useSessionGuard() {
  const sessionEnd = useAuthStore((state) => state.sessionEnd);
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated);
  const acknowledge = useAuthStore((state) => state.acknowledgeSessionEnd);

  React.useEffect(() => {
    // The reason is set before the tokens are actually gone, so wait for the
    // teardown to land — navigating first would unmount screens while their
    // queries could still fire one more authenticated request.
    if (sessionEnd === null || isAuthenticated) return;

    acknowledge();
    // Nothing cached belongs to whoever signs in next on this device — and
    // that includes where the last account was, which would otherwise make
    // the next one look already synced and skip its first profile write.
    queryClient.clear();
    useLocationStore.getState().clear();
    router.replace('/');

    // A deliberate sign-out speaks for itself, and a deleted account has
    // already been confirmed. Only an end the user didn't ask for needs saying.
    if (sessionEnd === 'expired') {
      toast.info('You’ve been signed out', {
        description: 'Your session expired. Please sign in again.',
      });
    }
  }, [sessionEnd, isAuthenticated, acknowledge]);
}
