import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { qk } from '@/lib/api/keys';
import type { AuthTokens, PublicProfile, RoleIntent, User } from '@/lib/api/schemas';
import { api, apiError } from '@/lib/axios';
import { endSession, useAuthStore } from '@/store/authStore';

// ── Fetchers ─────────────────────────────────────────────────────────────────

/**
 * Logs a session response with the tokens cut down to a fingerprint.
 *
 * A console carrying a live access token is one screenshot away from being a
 * working credential, and these logs are exactly what gets pasted into a bug
 * report. The prefix and length are enough to tell "a token came back" from
 * "the field is empty" or "it's the same one as last time", which is all you
 * ever need from a log.
 */
function logSession(label: string, data: AuthTokens) {
  if (!__DEV__) return;
  const fingerprint = (token: string | undefined) =>
    token ? `${token.slice(0, 12)}… (${token.length} chars)` : null;

  console.log(`[auth] ${label} ←`, {
    user: data.user,
    access_token: fingerprint(data.access_token),
    refresh_token: fingerprint(data.refresh_token),
  });
}

async function signIn(body: { email: string; password: string }) {
  if (__DEV__) console.log('[auth] POST /auth/signin →', { email: body.email });
  const { data } = await api.post<AuthTokens>('/auth/signin', body);
  logSession('signin', data);
  return data;
}

async function signUp(body: {
  email: string;
  password: string;
  display_name: string;
  /** ISO `YYYY-MM-DD`. Ignored by the API until `users` gains the column. */
  date_of_birth?: string;
}) {
  if (__DEV__) {
    console.log('[auth] POST /auth/signup →', {
      email: body.email,
      display_name: body.display_name,
    });
  }
  const { data } = await api.post<AuthTokens>('/auth/signup', body);
  logSession('signup', data);
  return data;
}

async function oauth(body: {
  provider: 'google' | 'apple';
  id_token: string;
  display_name?: string;
}) {
  const { data } = await api.post<AuthTokens>('/auth/oauth', body);
  return data;
}

// ── Hooks ────────────────────────────────────────────────────────────────────

export function useSignIn() {
  const setSession = useAuthStore((s) => s.setSession);
  return useMutation({ mutationFn: signIn, onSuccess: setSession });
}

export function useSignUp() {
  const setSession = useAuthStore((s) => s.setSession);
  return useMutation({ mutationFn: signUp, onSuccess: setSession });
}

export function useOAuthSignIn() {
  const setSession = useAuthStore((s) => s.setSession);
  return useMutation({ mutationFn: oauth, onSuccess: setSession });
}

export function useSignOut() {
  const signOut = useAuthStore((s) => s.signOut);
  // The cache is cleared and the user navigated by `useSessionGuard`, which
  // every other way of losing a session goes through too.
  return useMutation({ mutationFn: () => signOut('user') });
}

/**
 * The signed-in user. Seeded from the stored session, then refreshed.
 *
 * This is also where a session is confirmed to still be good. A token that
 * parses isn't the same as an account that exists — it can be revoked, or the
 * account deleted, from another device — so when `/me` won't answer for the
 * token, or answers without a user, the session is over and the app says so
 * rather than carrying on with a user it doesn't have.
 */
export function useMe() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const setUser = useAuthStore((s) => s.setUser);

  return useQuery({
    queryKey: qk.auth.me,
    enabled: isAuthenticated,
    queryFn: async () => {
      try {
        const { data } = await api.get<{ user: User }>('/me');
        if (!data?.user) throw new Error('No user on /me response');
        // Mirror into zustand so non-React code and screens that only need the
        // role don't have to subscribe to the query.
        setUser(data.user);
        return data.user;
      } catch (error) {
        // 401 arrives here only after the interceptor's refresh attempt, so by
        // now it means the whole session is spent, not that it needed a nudge.
        // 403 and 404 mean the account itself won't answer for this token.
        //
        // A null status is deliberately not in here: that's a request that
        // never reached the server, and signing someone out because their
        // train went into a tunnel would be worse than the problem.
        const { status } = apiError(error);
        if (status === 401 || status === 403 || status === 404) {
          await endSession('expired');
        }
        throw error;
      }
    },
  });
}

export function useUpdateProfile() {
  const queryClient = useQueryClient();
  const setUser = useAuthStore((s) => s.setUser);

  return useMutation({
    mutationFn: async (patch: {
      display_name?: string;
      bio?: string;
      location?: string;
      latitude?: number;
      longitude?: number;
      avatar_url?: string;
    }) => {
      const { data } = await api.patch<{ user: User }>('/me', patch);
      return data.user;
    },
    onSuccess: (user) => {
      setUser(user);
      queryClient.setQueryData(qk.auth.me, user);
    },
  });
}

export function useSetRole() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (role_intent: RoleIntent) => {
      const { data } = await api.post<{ user: User }>('/me/role', { role_intent });
      return data.user;
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: qk.auth.me }),
  });
}

export function useSendPhoneCode() {
  return useMutation({
    mutationFn: async (body: { user_id: string; phone: string }) => {
      if (__DEV__) console.log('[otp] POST /auth/phone/send →', body);
      try {
        const { data } = await api.post('/auth/phone/send', body);
        if (__DEV__) console.log('[otp] send response ←', data);
        return data;
      } catch (error) {
        // Log failures too — a 4xx here (rate limit, bad phone, unknown user)
        // is the usual reason no code ever arrives.
        if (__DEV__) console.log('[otp] send failed ←', apiError(error));
        throw error;
      }
    },
  });
}

export function useVerifyPhone() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (body: { phone: string; code: string }) => {
      if (__DEV__) console.log('[otp] POST /auth/phone/verify →', body);
      try {
        const { data } = await api.post('/auth/phone/verify', body);
        if (__DEV__) console.log('[otp] verify response ←', data);
        return data;
      } catch (error) {
        if (__DEV__) console.log('[otp] verify failed ←', apiError(error));
        throw error;
      }
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: qk.auth.me }),
  });
}

export function useRequestPasswordReset() {
  return useMutation({
    mutationFn: async (body: { email: string }) => {
      const { data } = await api.post('/auth/password-reset/request', body);
      return data;
    },
  });
}

export function useDeleteAccount() {
  const signOut = useAuthStore((s) => s.signOut);
  return useMutation({
    mutationFn: async () => {
      await api.delete('/me/account');
      // 'deleted', not 'user': there is no session left to revoke, and the
      // user doesn't need telling that the account they just deleted is gone.
      await signOut('deleted');
    },
  });
}

/** Another user's public profile — never contains phone, email or exact location. */
export function usePublicProfile(userId: string | undefined) {
  return useQuery({
    queryKey: qk.users.profile(userId ?? ''),
    enabled: !!userId,
    queryFn: async () => {
      const { data } = await api.get<{ user: PublicProfile }>(`/users/${userId}`);
      return data.user;
    },
  });
}
