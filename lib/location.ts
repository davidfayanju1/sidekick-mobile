import * as Location from 'expo-location';

import { useRoleStore } from '@/store/roleStore';

function buildLocationLabel(address: Location.LocationGeocodedAddress) {
  const area = address.district || address.subregion || address.street;
  const city = address.city || address.region;

  if (area && city && area !== city) return `${area}, ${city}`;
  return area || city || null;
}

/** Requests foreground location permission and, if granted, resolves a
 * human-readable label (e.g. "Lekki, Lagos") into the role store. Safe to
 * call without awaiting — it updates the store as it resolves. */
export async function captureUserLocation() {
  const { setLocationStatus, setLocationLabel } = useRoleStore.getState();
  setLocationStatus('loading');

  try {
    const { status } = await Location.requestForegroundPermissionsAsync();
    if (status !== 'granted') {
      setLocationStatus('denied');
      return;
    }

    const position = await Location.getCurrentPositionAsync({
      accuracy: Location.Accuracy.Balanced,
    });
    const [address] = await Location.reverseGeocodeAsync({
      latitude: position.coords.latitude,
      longitude: position.coords.longitude,
    });

    setLocationLabel(address ? buildLocationLabel(address) : null);
    setLocationStatus('granted');
  } catch {
    setLocationStatus('denied');
  }
}
