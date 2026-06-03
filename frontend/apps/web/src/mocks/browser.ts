import { handlers } from '@swp/api-client/mocks';
import { setupWorker } from 'msw/browser';

/**
 * MSW worker — serves the Orval-generated handlers (seeded from spec `*Example` payloads)
 * so the web app runs against the contract before the Go API exists (WEB-STACK §4).
 * Requires the service worker file: `pnpm dlx msw init public/ --save`.
 */
export const worker = setupWorker(...handlers);
