/**
 * Orval `httpClient: 'fetch'` mutator. Every generated hook routes through here, so
 * cross-cutting concerns live in ONE place (ENGINEERING.md B1/C2/C3):
 *   - bearer auth header from the pluggable token getter (C2)
 *   - auto Idempotency-Key (UUID v4) on mutations (C3 / CONVENTIONS §13)
 *   - 401 hook + typed ApiError on any non-2xx (B1)
 */
import { uuid } from '@swp/shared';
import { getConfig } from './config.ts';
import { ApiError, parseErrorEnvelope } from './errors.ts';

const MUTATION_METHODS = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);

export const customFetch = async <T>(url: string, options: RequestInit = {}): Promise<T> => {
  const { baseUrl, getToken, onUnauthenticated } = getConfig();
  const method = (options.method ?? 'GET').toUpperCase();
  const headers = new Headers(options.headers);

  const token = getToken();
  if (token) headers.set('Authorization', `Bearer ${token}`);
  // Set Content-Type only for non-FormData bodies (fetch auto-sets multipart/form-data
  // with the correct boundary for FormData — overriding it breaks the upload boundary).
  if (options.body && !(options.body instanceof FormData) && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }
  if (MUTATION_METHODS.has(method) && !headers.has('Idempotency-Key')) {
    headers.set('Idempotency-Key', uuid());
  }

  const fullUrl = url.startsWith('http') ? url : `${baseUrl}${url}`;
  let res: Response;
  try {
    // credentials:'include' is required for the httpOnly refresh cookie to be sent/received
    // cross-origin between FE (:4173 / :5173) and BE (:8081). The BE sets CORS
    // allow-origin for those origins with SameSite=Lax; Secure=false in the test env.
    res = await fetch(fullUrl, { ...options, headers, credentials: 'include' });
  } catch (cause) {
    throw new ApiError(0, undefined, 'Network error');
  }

  if (!res.ok) {
    const err = await parseErrorEnvelope(res);
    if (err.isUnauthenticated) onUnauthenticated?.();
    throw err;
  }

  // Orval's `httpClient: 'fetch'` types every operation's return as
  // `{ data; status; headers }` — the mutator MUST return that envelope (the response BODY
  // lives under `.data`). Non-2xx already threw above, so on success `.data` is the success body.
  const contentType = res.headers.get('Content-Type') ?? '';
  const body =
    res.status === 204
      ? undefined
      : contentType.includes('application/json')
        ? await res.json()
        : await res.text();
  return { data: body, status: res.status, headers: res.headers } as T;
};
