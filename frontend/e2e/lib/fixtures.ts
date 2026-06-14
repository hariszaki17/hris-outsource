/**
 * lib/fixtures.ts
 *
 * Extended Playwright test fixtures for the SWP E2E harness.
 *
 * Exports:
 *   - `test`   — Playwright test extended with loginAs / storageStateFor helpers
 *   - `expect` — re-exported from @playwright/test
 *
 * loginAs(page, persona):
 *   Drives the REAL login screen (fills #identifier / #password, submits, waits for
 *   navigation away from /login). This is true E2E — it exercises the full auth
 *   path including the cookie-transport refresh mechanism.
 *
 * storageStateFor(persona):
 *   Builds a path under frontend/e2e/.auth/<key>.json where Playwright can
 *   save/load browser storage state (cookies incl. httpOnly refresh cookie).
 *   Use in specs that focus on post-login screens to skip re-login on every test.
 *   NOTE: the access token is in-memory (not in storageState), so specs that need
 *   an authenticated API call MUST either re-login or use loginAs in beforeEach.
 *   The storageState only speeds up restoring the cookie session on page load.
 *   Documented in README.md §StorageState caching.
 */

import * as fs from 'node:fs';
import * as path from 'node:path';
import { type Page, test as base, expect } from '@playwright/test';
import { PERSONAS, type Persona, type PersonaKey } from './personas.js';

// Directory for per-persona storage state JSON files.
const AUTH_DIR = path.resolve(import.meta.dirname, '../.auth');

// Ensure the .auth directory exists (gitignored).
if (!fs.existsSync(AUTH_DIR)) {
  fs.mkdirSync(AUTH_DIR, { recursive: true });
}

// ---------------------------------------------------------------------------
// loginAs — drives the real login screen
// ---------------------------------------------------------------------------

/**
 * Navigate to /login, fill the email + password inputs from the real login
 * screen (id="email", id="password"), submit, and wait for the browser to
 * navigate away from /login (indicating a successful session).
 */
async function loginAs(page: Page, persona: Persona | PersonaKey): Promise<void> {
  const p = typeof persona === 'string' ? PERSONAS[persona] : persona;

  await page.goto('/login');

  // The login screen renders id="identifier" (phone-or-email) and id="password" inputs
  // (features/auth/login-screen.tsx). The persona `email` doubles as the identifier value.
  await page.locator('#identifier').fill(p.email);
  await page.locator('#password').fill(p.password);
  await page.locator('button[type="submit"]').click();

  // Wait until the browser leaves /login — confirms successful authentication.
  await page.waitForURL((url) => !url.pathname.startsWith('/login'), { timeout: 15_000 });
}

// ---------------------------------------------------------------------------
// storageStateFor — per-persona .auth/<key>.json path
// ---------------------------------------------------------------------------

/**
 * Returns the path where Playwright should save/load the storage state for
 * the given persona key. Use with `page.context().storageState(...)` or
 * `browser.newContext({ storageState: storageStateFor('hrAdmin') })`.
 *
 * Remember: httpOnly refresh cookies ARE captured; the in-memory access token
 * is NOT. Re-use storageState to restore the cookie, then call /auth/refresh
 * (or re-navigate) to obtain a new access token if your spec needs one.
 */
function storageStateFor(persona: PersonaKey): string {
  return path.join(AUTH_DIR, `${persona}.json`);
}

// ---------------------------------------------------------------------------
// Extended test fixture
// ---------------------------------------------------------------------------

type E2EFixtures = {
  loginAs: (persona: Persona | PersonaKey) => Promise<void>;
  storageStateFor: (persona: PersonaKey) => string;
};

const test = base.extend<E2EFixtures>({
  loginAs: async ({ page }, use) => {
    await use((persona) => loginAs(page, persona));
  },
  storageStateFor: async ({}, use) => {
    // eslint-disable-line @typescript-eslint/no-unused-vars
    await use(storageStateFor);
  },
});

export { test, expect };
export { loginAs, storageStateFor };
