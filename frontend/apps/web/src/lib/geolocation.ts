/**
 * Browser geolocation wrapper for web clock-in/out (docs/eng/AGENT-WEB-ACCESS.md §5).
 *
 * Mirrors the mobile `getCurrentCoords()` helper: returns `{ lat, lng }` on success or `null`
 * when location is unavailable (permission denied, position unavailable, timeout, or the API is
 * missing / called in a non-secure context). Callers treat `null` as "GPS denied/unavailable"
 * and surface a friendly message — they never throw.
 *
 * Requires a secure context (https or localhost); the dev server (localhost:5173) qualifies.
 */
export interface Coords {
  lat: number;
  lng: number;
}

export function getCurrentCoords(): Promise<Coords | null> {
  if (typeof navigator === 'undefined' || !navigator.geolocation) {
    return Promise.resolve(null);
  }
  return new Promise((resolve) => {
    navigator.geolocation.getCurrentPosition(
      (pos) => resolve({ lat: pos.coords.latitude, lng: pos.coords.longitude }),
      () => resolve(null),
      { enableHighAccuracy: true, timeout: 10_000, maximumAge: 0 },
    );
  });
}
