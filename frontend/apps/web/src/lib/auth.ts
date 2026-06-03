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
