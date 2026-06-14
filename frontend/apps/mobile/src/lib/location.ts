// GPS helper for clock-in/out (E5 F5.1). Foreground location only — the MVP clocks
// in/out while the app is open; background geofence is out of scope.
import * as Location from 'expo-location';

export type Coords = { lat: number; lng: number };

/** Request foreground permission + read the current position. Returns null if denied. */
export async function getCurrentCoords(): Promise<Coords | null> {
  const { status } = await Location.requestForegroundPermissionsAsync();
  if (status !== 'granted') return null;
  const pos = await Location.getCurrentPositionAsync({
    accuracy: Location.Accuracy.High,
  });
  return { lat: pos.coords.latitude, lng: pos.coords.longitude };
}
