// Wire the shared api-client once at startup (side-effect import from app-providers).
import { configureApiClient } from '@swp/api-client';
import { tokenStore } from './auth';

// iOS simulator's localhost is its OWN loopback, NOT the host Mac.
// Use the host's LAN IP instead. Physical device: same thing, use host LAN IP.
// EXPO_PUBLIC_API_BASE_URL overrides this default.
export const API_BASE_URL =
  typeof process !== 'undefined' && process.env?.EXPO_PUBLIC_API_BASE_URL
    ? process.env.EXPO_PUBLIC_API_BASE_URL
    : 'http://192.168.1.3:8080/api/v1';

configureApiClient({
  baseUrl: API_BASE_URL,
  getToken: () => tokenStore.get(),
  onUnauthenticated: () => tokenStore.triggerForceLogout(),
});
