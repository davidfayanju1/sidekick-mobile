import * as Location from 'expo-location';
import * as React from 'react';

import { api } from '@/lib/axios';
import { useAuthStore } from '@/store/authStore';
import { distanceMetres, useLocationStore, type Coords } from '@/store/locationStore';

/** A fix younger than this is good enough — don't wake the GPS for it. */
const FIX_MAX_AGE_MS = 5 * 60_000;
/**
 * How far the user must move before their stored profile coordinates are
 * rewritten. Tasks are browsed in kilometres, so metre-by-metre accuracy buys
 * nothing and would mean a write on every fix.
 */
const SYNC_THRESHOLD_M = 1_000;

function toCoords(position: Location.LocationObject): Coords {
  return {
    lat: position.coords.latitude,
    lng: position.coords.longitude,
    takenAt: position.timestamp,
  };
}

/**
 * Mirrors the fix onto the user's profile.
 *
 * Note this is *not* what makes the feed geo-aware — `/feed` reads lat/lng
 * from the query string, and nothing server-side reads `users.latitude` yet.
 * It's stored so the account keeps a home area across reinstalls, and it
 * fails quietly: nothing on screen depends on it.
 */
async function syncToProfile(fix: Coords) {
  if (!useAuthStore.getState().isAuthenticated) return;

  const { syncedCoords, setSyncedCoords } = useLocationStore.getState();
  if (syncedCoords && distanceMetres(syncedCoords, fix) < SYNC_THRESHOLD_M) return;

  try {
    // A patch of two fields: the API reads the user first and overwrites only
    // what's present, so this can't clear their display name or bio.
    await api.patch('/me', { latitude: fix.lat, longitude: fix.lng });
    setSyncedCoords(fix);
  } catch {
    // offline, or the session just ended — the next fix will try again
  }
}

export interface UseLocationOptions {
  /**
   * Ask for permission on mount. Pass this only from a screen whose purpose
   * the user can see is local — being asked for your location by a splash
   * screen is how people learn to tap Don't Allow.
   */
  request?: boolean;
}

/**
 * Takes a fix and stores it. Lives outside React because every one of its
 * steps is an external system: the permission sheet, the GPS, the store and
 * the API. Nothing here is render state.
 */
export async function locate({ prompt }: { prompt: boolean }): Promise<Coords | null> {
  const { setPermission, setCoords, setLocating } = useLocationStore.getState();

  let granted = (await Location.getForegroundPermissionsAsync()).granted;
  if (!granted && prompt) {
    granted = (await Location.requestForegroundPermissionsAsync()).granted;
  }
  setPermission(granted ? 'granted' : 'denied');
  if (!granted) return null;

  setLocating(true);
  try {
    // The cached fix lands immediately and is usually within a street or two;
    // the live one replaces it a second later. Showing the near-right order
    // now beats showing the wrong order until the GPS answers.
    const cached = await Location.getLastKnownPositionAsync({ maxAge: FIX_MAX_AGE_MS });
    if (cached) setCoords(toCoords(cached));

    // Balanced is ~100m. Tasks are filtered in kilometres, so High accuracy
    // would spend battery and seconds on precision the feed throws away.
    const current = await Location.getCurrentPositionAsync({
      accuracy: Location.Accuracy.Balanced,
    });
    const fix = toCoords(current);
    setCoords(fix);
    void syncToProfile(fix);
    return fix;
  } catch {
    // Services disabled or no signal. Any persisted fix stays usable.
    return null;
  } finally {
    setLocating(false);
  }
}

/**
 * The device's current position, for screens that show tasks by distance.
 *
 * Everything here degrades rather than blocks. If permission is refused, or
 * location services are off, or the fix times out, `coords` stays null and the
 * feed falls back to the server's ungeolocated ordering — a worse feed, not a
 * broken one, so nothing gates on this resolving.
 */
export function useLocation({ request: requestOnMount = false }: UseLocationOptions = {}) {
  const coords = useLocationStore((state) => state.coords);
  const permission = useLocationStore((state) => state.permission);
  const isLocating = useLocationStore((state) => state.isLocating);

  React.useEffect(() => {
    if (!requestOnMount) return;

    const { permission: current, coords: existing } = useLocationStore.getState();
    // Asking again after a refusal does nothing on iOS but is pestering on
    // Android, and either way the answer is already known.
    if (current === 'denied') return;
    // A fresh fix is a fresh fix; don't wake the GPS on every tab switch.
    if (existing && Date.now() - existing.takenAt < FIX_MAX_AGE_MS) return;

    void locate({ prompt: true });
  }, [requestOnMount]);

  return {
    coords,
    permission,
    isLocating,
    /** Re-ask explicitly, e.g. from a "use my location" button. */
    refresh: () => locate({ prompt: true }),
  };
}

/**
 * Turns a typed address into coordinates, or null if it can't be placed.
 *
 * Tasks need this or they are invisible to the browse feed: the geo query is
 * `AND t.location_lat IS NOT NULL AND … earth_distance(…) <= radius`, so a
 * task posted without coordinates is filtered out entirely the moment a
 * Sidekick has a location. Failing to geocode is survivable — the task is
 * still created and still reachable by search — so this never blocks posting.
 */
export async function geocode(address: string): Promise<{ lat: number; lng: number } | null> {
  const query = address.trim();
  if (!query) return null;
  try {
    const [match] = await Location.geocodeAsync(query);
    return match ? { lat: match.latitude, lng: match.longitude } : null;
  } catch {
    // No geocoder (Android without Play services), offline, or rate-limited.
    return null;
  }
}
