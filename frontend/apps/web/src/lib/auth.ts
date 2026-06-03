/**
 * Thin auth abstraction (WEB-STACK §6). The token MODEL is deferred until the Go /auth
 * design — so the rest of the app only ever touches this module. Today: in-memory access
 * token (not persisted → XSS-safe by default; survives nothing on reload yet). When the
 * backend lands a refresh mechanism, only this file changes.
 */
import { configureApiClient } from '@swp/api-client';
import type { Role } from '@swp/shared';

/**
 * The signed-in staff member. The real shape comes from a `/me`-style endpoint once the Go
 * `/auth` design lands (WEB-STACK §6); until then it is set at login. The shell reads `role`
 * to gate nav (defense-in-depth — the API is the real gate, ENGINEERING.md C1).
 */
export interface SessionUser {
  name: string;
  role: Role;
  /** Avatar initials (display-only). */
  initials: string;
  /** Shift leaders are scoped to ONE client company; surfaced in the shell. */
  companyName?: string;
}

let accessToken: string | null = null;
let user: SessionUser | null = null;
const listeners = new Set<() => void>();

function emit() {
  for (const l of listeners) l();
}

export const auth = {
  getToken: () => accessToken,
  getUser: () => user,
  isAuthenticated: () => accessToken !== null,
  setToken(token: string | null) {
    accessToken = token;
    emit();
  },
  setUser(next: SessionUser | null) {
    user = next;
    emit();
  },
  /** Establish a session (token + user) in one step. */
  login(token: string, next: SessionUser) {
    accessToken = token;
    user = next;
    emit();
  },
  clear() {
    accessToken = null;
    user = null;
    emit();
  },
  subscribe(listener: () => void) {
    listeners.add(listener);
    return () => listeners.delete(listener);
  },
};

/** Wire the API client to this store. Called once at startup. */
export function installAuth() {
  configureApiClient({
    baseUrl: import.meta.env.VITE_API_BASE_URL ?? '/api/v1',
    getToken: () => auth.getToken(),
    onUnauthenticated: () => auth.clear(),
  });
}
