import { defineConfig, devices } from '@playwright/test';

/** E2E config. Tests drive the critical Gherkin flows from the PRDs (ENGINEERING.md F1). */
export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  reporter: 'list',
  use: { baseURL: 'http://localhost:5173', trace: 'on-first-retry' },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: {
    // Force the MSW mock layer ON so the stateful E11 handlers
    // (packages/api-client/src/e11-stateful-mocks.ts) serve the real approval lifecycle.
    // This overrides .env.development.local's VITE_ENABLE_MSW=false — process env takes
    // precedence over .env files in Vite.
    command: 'pnpm dev',
    url: 'http://localhost:5173',
    reuseExistingServer: !process.env.CI,
    env: { VITE_ENABLE_MSW: 'true', VITE_API_BASE_URL: '/api/v1' },
  },
});
