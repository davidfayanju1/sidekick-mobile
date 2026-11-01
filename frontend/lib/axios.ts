import axios, { isAxiosError } from 'axios';

import { clearTokens, getTokens, setTokens } from '@/lib/tokenStore';

/** Set EXPO_PUBLIC_API_URL in .env to point at staging/production. */
export const API_BASE_URL = 'https://sidekick-mobile.onrender.com/';

export const api = axios.create({
  baseURL: API_BASE_URL,
  timeout: 20_000,
  headers: { 'Content-Type': 'application/json' },
});

api.interceptors.request.use((config) => {
  const tokens = getTokens();
  if (tokens) config.headers.Authorization = `Bearer ${tokens.accessToken}`;
  return config;
});

// Access tokens last 15 minutes, so a 401 usually just means "expired" — swap in
// a fresh pair and replay the request once before giving up on the session.
api.interceptors.response.use(undefined, async (error) => {
  const config = error.config;
  if (error.response?.status !== 401 || !config || config._retried || !getTokens()) {
    throw error;
  }
  config._retried = true;
  try {
    await refreshSession();
    return api.request(config);
  } catch {
    await clearTokens();
    throw error;
  }
});

/** Bare client: refreshing must not recurse through the 401 handler above. */
async function refreshSession() {
  const { data } = await axios.post(`${API_BASE_URL}/auth/refresh`, {
    refresh_token: getTokens()?.refreshToken,
  });
  await setTokens({ accessToken: data.access_token, refreshToken: data.refresh_token });
}

/** The API's error body: `{"code": "bad_request", "message": "..."}`. */
interface ApiErrorBody {
  code?: string;
  message?: string;
}

export interface ApiErrorInfo {
  /** HTTP status, or null if the request never reached the server. */
  status: number | null;
  /**
   * The API's own code, e.g. `unauthorized`. Not axios's `error.code`, which
   * carries transport codes like `ERR_BAD_REQUEST` and is useless for branching
   * on business logic.
   */
  code: string | null;
  message: string;
}

/**
 * Unpacks an error into the three things calling code ever needs.
 *
 * Use this instead of reaching into `error.response.data` at the call site:
 * `catch` binds `unknown`, so every call site would otherwise need its own cast,
 * and a cast that's wrong fails silently.
 */
export function apiError(error: unknown): ApiErrorInfo {
  if (isAxiosError<ApiErrorBody>(error)) {
    const body = error.response?.data;
    return {
      status: error.response?.status ?? null,
      code: body?.code ?? null,
      message:
        body?.message ??
        (error.response
          ? error.message
          : error.code === 'ECONNABORTED'
            ? 'The request timed out. Please try again.'
            : 'No internet connection.'),
    };
  }
  return {
    status: null,
    code: null,
    message: error instanceof Error ? error.message : 'Something went wrong.',
  };
}

/** Pulls the API's `{code, message}` body out of an axios error for display. */
export function errorMessage(error: unknown): string {
  return apiError(error).message;
}

export default api;
