/**
 * lib/api.ts
 *
 * Thin fetch wrappers for direct API assertions in the auth E2E suite.
 * Base: http://localhost:8081/api/v1 (Go API test port, per .env.e2e).
 *
 * apiLogin(email, password):
 *   POST /auth/login — returns the parsed LoginResponse + the raw Set-Cookie header
 *   so the refresh cookie can be captured for the /auth/refresh assertion.
 *
 * apiRefresh(cookie):
 *   POST /auth/refresh — sends the refresh cookie in the Cookie header; returns
 *   HTTP status + parsed body. Used by the AU-6/C-3 token-refresh scenario to
 *   deterministically assert that a new access_token is issued.
 *
 * Note: these helpers bypass the browser entirely (Node fetch). They are ONLY for
 * scenarios that need deterministic API-level assertions (e.g. the refresh test
 * where we need two access tokens to compare). For UI-driven flows, use loginAs().
 */

const API_BASE = 'http://localhost:8081/api/v1';

// ---------------------------------------------------------------------------
// Types (minimal subset of the OpenAPI shapes — no need to import generated types)
// ---------------------------------------------------------------------------

export interface LoginResponse {
  access_token: string;
  refresh_token: string;
  token_type: string;
  expires_in: number;
  user: {
    id: string;
    email: string;
    role: string;
    status: string;
    employee_id: string | null;
    full_name: string;
    last_login_at: string | null;
    scope: { type: string; company_id: string | null };
  };
}

export interface RefreshResponse {
  access_token: string;
  token_type: string;
  expires_in: number;
}

// ---------------------------------------------------------------------------
// apiLogin
// ---------------------------------------------------------------------------

export interface ApiLoginResult {
  /** Parsed 200 LoginResponse body. */
  body: LoginResponse;
  /** The raw value of the first Set-Cookie header (contains the refresh cookie). */
  setCookieHeader: string | null;
}

/**
 * apiLogin — POST /auth/login with email + password.
 * Throws if the response is not 200.
 *
 * @param email    Login email.
 * @param password Plaintext password.
 * @returns        Parsed body + raw Set-Cookie header.
 */
export async function apiLogin(email: string, password: string): Promise<ApiLoginResult> {
  const res = await fetch(`${API_BASE}/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password, stay_signed_in: false }),
  });

  if (!res.ok) {
    const text = await res.text();
    throw new Error(`[apiLogin] ${res.status} ${res.statusText}: ${text}`);
  }

  const body = (await res.json()) as LoginResponse;
  // fetch() merges duplicate Set-Cookie headers into a comma-joined string in Node 18+.
  const setCookieHeader = res.headers.get('set-cookie');

  return { body, setCookieHeader };
}

// ---------------------------------------------------------------------------
// apiRefresh
// ---------------------------------------------------------------------------

export interface ApiRefreshResult {
  status: number;
  body: RefreshResponse | null;
}

/**
 * apiRefresh — POST /auth/refresh, sending the refresh cookie.
 *
 * The refresh cookie is an httpOnly cookie set by the BE. In the browser it is
 * sent automatically; for this direct Node fetch we send it manually in the
 * Cookie header using the raw Set-Cookie value captured from apiLogin().
 *
 * @param setCookieHeader  The raw Set-Cookie header value from apiLogin.
 * @returns                HTTP status + parsed body (null if non-JSON response).
 */
export async function apiRefresh(setCookieHeader: string): Promise<ApiRefreshResult> {
  // Extract the cookie name=value pairs from the Set-Cookie header.
  // A Set-Cookie header looks like: name=value; Path=/; HttpOnly; ...
  // We strip the attributes and reconstruct the Cookie header.
  const cookieValue = setCookieHeader
    .split(',')
    .map((part) => part.trim().split(';')[0].trim())
    .join('; ');

  const res = await fetch(`${API_BASE}/auth/refresh`, {
    method: 'POST',
    headers: {
      Cookie: cookieValue,
    },
    credentials: 'include',
  });

  let body: RefreshResponse | null = null;
  const contentType = res.headers.get('content-type') ?? '';
  if (contentType.includes('application/json')) {
    body = (await res.json()) as RefreshResponse;
  }

  return { status: res.status, body };
}
