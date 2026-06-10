// Mobile auth token store. RN diverges from web: web keeps the refresh token in an httpOnly
// cookie; React Native has no cookie jar, so we keep the access token in memory and persist
// the refresh token in the OS keychain via expo-secure-store. The access token is what the
// api-client mutator reads through getToken() on every request.
import * as SecureStore from 'expo-secure-store';

const REFRESH_KEY = 'swp.refresh_token';

let accessToken: string | null = null;
let forceLogout: () => void = () => {};

export const tokenStore = {
  /** Current bearer; consumed by configureApiClient.getToken (mutator.ts). */
  get: () => accessToken,
  setAccess: (token: string | null) => {
    accessToken = token;
  },
  /** Persist a full session after login. */
  async setSession(access: string, refresh: string | undefined) {
    accessToken = access;
    if (refresh) {
      await SecureStore.setItemAsync(REFRESH_KEY, refresh);
    }
  },
  /** Rotate just the stored refresh token (refresh endpoint may return a new one). */
  async persistRefresh(refresh: string | undefined) {
    if (refresh) {
      await SecureStore.setItemAsync(REFRESH_KEY, refresh);
    }
  },
  getRefresh: () => SecureStore.getItemAsync(REFRESH_KEY),
  async clear() {
    accessToken = null;
    await SecureStore.deleteItemAsync(REFRESH_KEY);
  },
  /** SessionProvider registers its signOut so a 401 (onUnauthenticated) can drop the session. */
  registerForceLogout(fn: () => void) {
    forceLogout = fn;
  },
  triggerForceLogout() {
    forceLogout();
  },
};
