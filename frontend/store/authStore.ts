import { create } from 'zustand';

import type { AuthTokens, User } from '@/lib/api/schemas';
import { api } from '@/lib/axios';
import { clearTokens, loadTokens, setTokens, subscribeToTokens } from '@/lib/tokenStore';
import { useRoleStore } from '@/store/roleStore';

/**
 * Why a session ended.
 *
 * The app has to tell these apart: only an involuntary end owes the user an
 * explanation, and only a live token is worth spending a request to revoke.
 */
export type SessionEndReason = 'user' | 'expired' | 'deleted';

interface AuthState {
  user: User | null;
  isAuthenticated: boolean;
  /** False until the persisted session has been read off disk — gate routing on it. */
  isHydrated: boolean;
  /**
   * Set the moment a session ends, cleared once the app has acted on it.
   *
   * This is the signal `useSessionGuard` navigates on. Every way of losing a
   * session sets it — tapping Sign out, a refresh the interceptor gave up on,
   * a `/me` that no longer recognises the token — so there is one way out of
   * the app rather than one per call site.
   */
  sessionEnd: SessionEndReason | null;
  setSession: (session: AuthTokens) => void;
  setUser: (user: User) => void;
  signOut: (reason?: SessionEndReason) => Promise<void>;
  /** Called by the guard once it has sent the user back to the landing screen. */
  acknowledgeSessionEnd: () => void;
  hydrate: () => Promise<void>;
}

export const useAuthStore = create<AuthState>((set, get) => ({
  user: null,
  isAuthenticated: false,
  isHydrated: false,
  sessionEnd: null,

  setSession: (session) => {
    void setTokens({ accessToken: session.access_token, refreshToken: session.refresh_token });
    // Clearing `sessionEnd` matters: signing straight back in after being
    // signed out would otherwise look to the guard like the new session had
    // ended too, and bounce the user to the landing screen.
    set({ user: session.user, isAuthenticated: true, sessionEnd: null });
    // `role_intent` lives on the server, so the user picks a side once — not
    // on every launch. Mirroring it here, rather than in whichever screen
    // happens to call /me, means the home screen already knows.
    useRoleStore.getState().syncFromUser(session.user);
  },

  setUser: (user) => {
    set({ user });
    useRoleStore.getState().syncFromUser(user);
  },

  signOut: async (reason = 'user') => {
    // Re-entrant calls are easy to trigger — tapping Sign out while a dead
    // token is already tearing the session down — and the first reason is the
    // true one, so later ones are dropped rather than overwriting it.
    if (get().sessionEnd !== null) return;
    // And a session that's already gone can't end again. The interceptor tears
    // one down the moment a refresh fails, which is strictly before the `/me`
    // that provoked it sees its own error — without this, that one failure
    // would end the session twice and explain itself twice.
    if (!get().isAuthenticated) return;
    set({ sessionEnd: reason });

    // Revoking is only meaningful while the token still works. An expired
    // session has nothing left to revoke and a deleted account has nothing to
    // revoke it against; either way the call would 401 its way through a
    // doomed refresh before failing.
    if (reason === 'user') {
      try {
        await api.post('/auth/signout');
      } catch {
        // already invalid or offline — the local clear below is what matters
      }
    }
    await clearTokens();
  },

  acknowledgeSessionEnd: () => set({ sessionEnd: null }),

  hydrate: async () => {
    const tokens = await loadTokens();
    set({ isAuthenticated: tokens !== null, isHydrated: true });
  },
}));

// The axios interceptor clears tokens when a refresh fails, so the store has to
// follow the token store rather than the other way round.
subscribeToTokens((tokens) => {
  if (tokens !== null) return;

  const { isAuthenticated, sessionEnd } = useAuthStore.getState();
  // Boot clears keys that were never set, and signing out clears them twice.
  // Only a session that was actually live can end.
  if (!isAuthenticated) return;

  useAuthStore.setState({
    user: null,
    isAuthenticated: false,
    // A clear nobody asked for came from the interceptor giving up on a
    // refresh: the session expired underneath the user.
    sessionEnd: sessionEnd ?? 'expired',
  });
});

/** Non-reactive read, for use outside components. */
export const currentUserId = () => useAuthStore.getState().user?.id;

/** Ends the session from outside React — query layers, interceptors, handlers. */
export const endSession = (reason: SessionEndReason) => useAuthStore.getState().signOut(reason);
