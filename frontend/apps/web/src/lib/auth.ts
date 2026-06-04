/**
 * Thin auth abstraction (WEB-STACK §6). The token MODEL is deferred until the Go /auth
 * design — so the rest of the app only ever touches this module. Today: in-memory access
 * token (not persisted → XSS-safe by default; survives nothing on reload yet). When the
 * backend lands a refresh mechanism, only this file changes.
 */
import { configureApiClient } from '@swp/api-client';
import type { MeResponse } from '@swp/api-client/e1';
import { permissionsForRole } from '@swp/shared';
import type { Permission, Role } from '@swp/shared';

/**
 * The signed-in staff member. The real shape comes from a `/me`-style endpoint once the Go
 * `/auth` design lands (WEB-STACK §6); until then it is set at login. The shell gates nav on
 * `permissions` (defense-in-depth — the API is the real gate, ENGINEERING.md C1).
 */
export interface SessionUser {
  name: string;
  role: Role;
  /**
   * Effective permissions (capability axis — docs/eng/NAVIGATION-AND-RBAC.md). Today derived
   * from `role` via `permissionsForRole()` at login; later returned verbatim by `/me`. The UI
   * checks these, never `role`, so new/custom roles need no client changes.
   */
  permissions: readonly Permission[];
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

/**
 * Map a BE `MeResponse` → the app's `SessionUser`. Called at login and can be reused
 * when `/auth/me` is fetched on page reload (Phase 2+).
 *
 * companyName: Phase 1 has no company-name lookup endpoint; for `shift_leader` scope we
 * surface the company_id literal (e.g. "SWP-CMP-0021") until Phase 3 resolves it.
 * TODO(Phase-3): resolve scope.company_id → company display name via the companies endpoint.
 */
export function buildSessionUser(u: MeResponse): SessionUser {
  const words = u.full_name.trim().split(/\s+/);
  const initials = words
    .slice(0, 2)
    .map((w) => w[0] ?? '')
    .join('')
    .toUpperCase();

  const companyName =
    u.scope?.type === 'company' && u.scope.company_id ? u.scope.company_id : undefined;

  return {
    name: u.full_name,
    role: u.role as Role,
    permissions: permissionsForRole(u.role as Role),
    initials,
    companyName,
  };
}

/** Wire the API client to this store. Called once at startup. */
export function installAuth() {
  configureApiClient({
    baseUrl: import.meta.env.VITE_API_BASE_URL ?? '/api/v1',
    getToken: () => auth.getToken(),
    onUnauthenticated: () => auth.clear(),
  });
}

/**
 * tryRestoreSession — attempt to hydrate the auth state from the httpOnly refresh cookie.
 *
 * Called once at app bootstrap (before React mounts). If the user previously logged in
 * and the refresh cookie is still valid, this restores the in-memory access token so that
 * the TanStack Router `beforeLoad` guard does not redirect authenticated users to /login
 * on page reload.
 *
 * Flow:
 *   1. POST /auth/refresh with credentials:'include' — cookie is read by the BE.
 *   2. If 200, extract access_token and call GET /auth/me with the token.
 *   3. Set auth.login(token, sessionUser) so `isAuthenticated()` returns true.
 *   4. On any failure (401, network error) — silently leave auth as unauthenticated.
 *      The router guards handle the redirect to /login.
 */
export async function tryRestoreSession(): Promise<void> {
  const baseUrl = (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? '/api/v1';
  try {
    // Step 1: Refresh. The BE reads the token from the httpOnly cookie.
    // We send an empty body (cookie is the real source per readRefreshToken in handler.go).
    const refreshRes = await fetch(`${baseUrl}/auth/refresh`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: '' }),
    });
    if (!refreshRes.ok) return; // No valid cookie → stay unauthenticated.

    const refreshData = (await refreshRes.json()) as { access_token: string };
    const token = refreshData.access_token;
    if (!token) return;

    // Step 2: Fetch the current user.
    const meRes = await fetch(`${baseUrl}/auth/me`, {
      method: 'GET',
      credentials: 'include',
      headers: { Authorization: `Bearer ${token}` },
    });
    if (!meRes.ok) return;

    const meData = (await meRes.json()) as Parameters<typeof buildSessionUser>[0];
    const sessionUser = buildSessionUser(meData);

    // Step 3: Hydrate the in-memory auth state.
    auth.login(token, sessionUser);
  } catch {
    // Network error or JSON parse failure — remain unauthenticated.
  }
}
