import AsyncStorage from '@react-native-async-storage/async-storage';
import { create } from 'zustand';
import { createJSONStorage, persist } from 'zustand/middleware';

/**
 * Where the user is.
 *
 * `/feed` only filters and sorts by distance when it's given `lat`/`lng`
 * (`earth_distance(ll_to_earth($1, $2), …)` in internal/discovery), so without
 * a fix the browse feed is just "every open task, newest first" — which is the
 * wrong feed for a marketplace where someone has to physically turn up.
 *
 * The last fix is persisted so the next launch can sort by distance from the
 * first frame instead of showing a wrong order and reshuffling once the GPS
 * answers. A stale fix is still a far better guess than none.
 */
export type LocationPermission = 'unknown' | 'granted' | 'denied';

export interface Coords {
  lat: number;
  lng: number;
  /** When this fix was taken, as epoch ms — a day-old fix is worth saying so. */
  takenAt: number;
}

interface LocationState {
  coords: Coords | null;
  permission: LocationPermission;
  /**
   * True while a fix is being taken. This lives here rather than in React
   * state because the GPS is an external system driving it, not a render.
   */
  isLocating: boolean;
  /** The fix last written to the user's profile, so we only PATCH on real movement. */
  syncedCoords: Coords | null;
  setCoords: (coords: Coords) => void;
  setPermission: (permission: LocationPermission) => void;
  setLocating: (isLocating: boolean) => void;
  setSyncedCoords: (coords: Coords) => void;
  clear: () => void;
}

export const useLocationStore = create<LocationState>()(
  persist(
    (set) => ({
      coords: null,
      permission: 'unknown',
      isLocating: false,
      syncedCoords: null,
      setCoords: (coords) => set({ coords }),
      setPermission: (permission) => set({ permission }),
      setLocating: (isLocating) => set({ isLocating }),
      setSyncedCoords: (syncedCoords) => set({ syncedCoords }),
      clear: () =>
        set({ coords: null, permission: 'unknown', isLocating: false, syncedCoords: null }),
    }),
    {
      name: 'sidekick.location',
      storage: createJSONStorage(() => AsyncStorage),
      // Whether the GPS is busy is true of this moment, not of this device.
      partialize: ({ coords, permission, syncedCoords }) => ({ coords, permission, syncedCoords }),
    }
  )
);

/** Metres between two points, by the haversine formula. */
export function distanceMetres(a: Coords, b: Coords): number {
  const R = 6_371_000;
  const toRad = (deg: number) => (deg * Math.PI) / 180;
  const dLat = toRad(b.lat - a.lat);
  const dLng = toRad(b.lng - a.lng);
  const lat1 = toRad(a.lat);
  const lat2 = toRad(b.lat);
  const h = Math.sin(dLat / 2) ** 2 + Math.sin(dLng / 2) ** 2 * Math.cos(lat1) * Math.cos(lat2);
  return 2 * R * Math.asin(Math.sqrt(h));
}
