import { resetE11Store } from '@swp/api-client/e11-stateful-mocks';
import { handlers } from '@swp/api-client/mocks';
import { setupWorker } from 'msw/browser';

/**
 * MSW worker — serves the Orval-generated handlers (seeded from spec `*Example` payloads)
 * so the web app runs against the contract before the Go API exists (WEB-STACK §4).
 * Requires the service worker file: `pnpm dlx msw init public/ --save`.
 */
export const worker = setupWorker(...handlers);

// E2E test helper: let Playwright reset the stateful E11 approval store between specs so each
// scenario starts from the deterministic seed without a full page reload. No-op in prod.
if (typeof window !== 'undefined') {
  (window as unknown as { __resetE11?: () => void }).__resetE11 = resetE11Store;
}
