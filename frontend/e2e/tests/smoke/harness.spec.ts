/**
 * tests/smoke/harness.spec.ts
 *
 * Smoke test — proves the harness boots and the REAL FE dev server is reachable.
 * Does NOT log in or call the API; merely navigates to /login and verifies that
 * the real login screen (with its #identifier input) is rendered.
 *
 * Because VITE_ENABLE_MSW=false in playwright.config.ts, the browser will NOT
 * activate any service worker. The form being present is sufficient proof that
 * the Vite dev server is up and serving the real app bundle.
 *
 * This spec is referenced by plan 01-01 acceptance criteria.
 * Traceable to: HARN-01, e2e-harness-spec.md §Definition of done.
 */
import { test, expect } from '@playwright/test';

test('harness boots: real FE dev server reaches the login screen', async ({ page }) => {
  // Navigate to the login page served by the real Vite dev server (:4173).
  await page.goto('/login');

  // The login screen (login-screen.tsx) renders id="email" and id="password".
  // Their presence proves the real app bundle is loaded.
  await expect(page.locator('#identifier')).toBeVisible();
  await expect(page.locator('#password')).toBeVisible();

  // Verify no MSW service-worker banner is present (MSW logs to console when
  // active; we just check the input is there, which is sufficient for the smoke).
  // The key invariant: VITE_ENABLE_MSW=false means the real Go API on :8081
  // handles any request made by the form — MSW is not intercepting.
  const submitButton = page.locator('button[type="submit"]');
  await expect(submitButton).toBeVisible();
});
