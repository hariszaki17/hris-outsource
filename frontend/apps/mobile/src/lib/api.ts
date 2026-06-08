// Wire the shared api-client once at startup (side-effect import from app-providers).
// Base URL comes from EXPO_PUBLIC_API_BASE_URL; default is the Go dev server.
// NOTE: on a physical device, localhost is the device itself — set EXPO_PUBLIC_API_BASE_URL
// to the host machine's LAN IP (e.g. http://192.168.1.20:8081/api/v1).
import { configureApiClient } from '@swp/api-client';
import { tokenStore } from './auth';

export const API_BASE_URL = process.env.EXPO_PUBLIC_API_BASE_URL ?? 'http://localhost:8081/api/v1';

configureApiClient({
  baseUrl: API_BASE_URL,
  getToken: () => tokenStore.get(),
  onUnauthenticated: () => tokenStore.triggerForceLogout(),
});
