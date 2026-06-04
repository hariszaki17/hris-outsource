import { defineConfig, devices } from '@playwright/test';

/**
 * playwright.config.ts
 *
 * Full-stack Playwright config: runs the REAL Vite dev server (MSW OFF, pointed
 * at the Go API on :8081) against an ephemeral Postgres on :5433.
 *
 * Run modes (all run the SAME specs):
 *   pnpm e2e             – headless CI default
 *   pnpm e2e:headed      – headed (local debug with visible browser)
 *   pnpm e2e:ui          – Playwright UI (interactive, per-test run)
 *
 * Note on webServer command: We use `dev` (Vite dev server) rather than
 * `preview` (which requires a prior `build` step) so globalSetup can boot the
 * full stack without an extra build phase. The dev server honours the env vars
 * below at startup time, giving us MSW=false and the real API base URL.
 * Documented in README.md §Run modes.
 */
export default defineConfig({
  testDir: './tests',

  // Specs share a single seeded database — run serially so they don't stomp
  // each other. Each spec file calls resetDb() in beforeEach/beforeAll.
  fullyParallel: false,
  workers: 1,

  // Allow 90 s per test: the first few tests hit a cold Vite dev server
  // (first compilation + tryRestoreSession + real API round-trip) which can
  // take 20-50 s on a warm machine. After the first load, all subsequent
  // tests complete in 1-3 s.
  timeout: 90_000,

  retries: process.env.CI ? 1 : 0,

  reporter: [['list'], ['html', { open: 'never' }]],

  use: {
    baseURL: 'http://localhost:4173',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'on-first-retry',
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],

  globalSetup: './global-setup.ts',
  globalTeardown: './global-teardown.ts',

  webServer: {
    // Vite dev server: reads VITE_* env vars directly — no build step needed.
    command: 'pnpm --filter @swp/web dev --port 4173 --strictPort',
    url: 'http://localhost:4173',
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
    env: {
      // MSW must be OFF so the real Go API handles every request.
      VITE_ENABLE_MSW: 'false',
      // Point the FE at the test Go API (port 8081, separate from dev :8080).
      VITE_API_BASE_URL: 'http://localhost:8081/api/v1',
    },
  },
});
