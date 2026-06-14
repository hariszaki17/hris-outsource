/**
 * Runtime config for the API client. The app calls `configureApiClient` at startup,
 * which keeps this package decoupled from any bundler's env (testable, RN-reusable).
 */
export interface ApiClientConfig {
  /** Base URL incl. /api/v1. CONVENTIONS §2. */
  baseUrl: string;
  /** Returns the current bearer access token, or null. Auth model is pluggable (WEB-STACK §6). */
  getToken: () => string | null;
  /** Called when a request returns 401 (token rejected). */
  onUnauthenticated?: () => void;
}

const config: ApiClientConfig = {
  baseUrl: '/api/v1',
  getToken: () => null,
};

export function configureApiClient(partial: Partial<ApiClientConfig>): void {
  Object.assign(config, partial);
}

export function getConfig(): ApiClientConfig {
  return config;
}
