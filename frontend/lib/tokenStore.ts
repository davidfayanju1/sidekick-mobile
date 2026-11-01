import * as SecureStore from 'expo-secure-store';

const ACCESS_KEY = 'sidekick.access_token';
const REFRESH_KEY = 'sidekick.refresh_token';

export interface AuthTokens {
  accessToken: string;
  refreshToken: string;
}

// Kept in memory so interceptors can read tokens synchronously — SecureStore is
// only the durable copy, read once on boot and written on every change.
let tokens: AuthTokens | null = null;
let loaded = false;

type Listener = (tokens: AuthTokens | null) => void;
const listeners = new Set<Listener>();

function emit() {
  listeners.forEach((listener) => listener(tokens));
}

/** Notified whenever tokens are set or cleared — used to drive the auth store. */
export function subscribeToTokens(listener: Listener): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function getTokens(): AuthTokens | null {
  return tokens;
}

/**
 * SecureStore is unavailable on web and can throw on a locked keychain, so a
 * failure here is survivable — the in-memory copy still backs the session.
 *
 * It is *not* silent, though. A swallowed write looks exactly like a working
 * app until the next launch, when the user is mysteriously signed out, so the
 * failure is logged in development where someone can act on it.
 */
async function write(key: string, value: string | null) {
  try {
    if (value === null) await SecureStore.deleteItemAsync(key);
    else await SecureStore.setItemAsync(key, value);
  } catch (error) {
    if (__DEV__) {
      console.warn(
        `[tokens] could not persist "${key}" — the session will not survive a restart.`,
        error
      );
    }
  }
}

async function read(key: string): Promise<string | null> {
  try {
    return await SecureStore.getItemAsync(key);
  } catch (error) {
    if (__DEV__) console.warn(`[tokens] could not read "${key}"`, error);
    return null;
  }
}

export async function setTokens(next: AuthTokens) {
  tokens = next;
  loaded = true;
  emit();
  await Promise.all([write(ACCESS_KEY, next.accessToken), write(REFRESH_KEY, next.refreshToken)]);
  if (__DEV__) console.log('[tokens] session persisted');
}

export async function clearTokens() {
  tokens = null;
  loaded = true;
  emit();
  await Promise.all([write(ACCESS_KEY, null), write(REFRESH_KEY, null)]);
}

/** Restores the persisted session. Call once during app boot, before rendering routes. */
export async function loadTokens(): Promise<AuthTokens | null> {
  if (loaded) return tokens;
  const [accessToken, refreshToken] = await Promise.all([read(ACCESS_KEY), read(REFRESH_KEY)]);
  loaded = true;
  tokens = accessToken && refreshToken ? { accessToken, refreshToken } : null;
  if (__DEV__) {
    console.log(
      `[tokens] restored session: ${tokens ? 'yes' : 'no'}`,
      tokens ? { access: `${tokens.accessToken.slice(0, 12)}…` } : { accessToken, refreshToken }
    );
  }
  emit();
  return tokens;
}
