/**
 * Public surface of the API client. Generated hooks are imported per-epic
 * (`@swp/api-client/e1`, `/e6`); this barrel exposes the stable hand-authored core.
 */
export { configureApiClient, getConfig, type ApiClientConfig } from './config.ts';
export { ApiError, parseErrorEnvelope, type ErrorEnvelope } from './errors.ts';
export { createQueryClient } from './query-client.ts';
export { customFetch } from './mutator.ts';
