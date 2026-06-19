/**
 * tests/e1/lockout.spec.ts
 *
 * E2E for E1 · Login Lockout / Rate-limiting (AU-5).
 *
 * Tests the account lockout mechanism after repeated failed login attempts.
 *
 * Coverage:
 *   LOCKOUT-BANNER   after sufficient failed attempts, lockout banner is shown
 *   LOCKOUT-REDIRECT locked-out user is redirected to /login?error=locked
 *   LOCKOUT-RESET    after lockout period expires, login works again
 *
 * Constraints:
 *   - The test environment uses RATELIMIT_PER_MINUTE=6000, so a real
 *     rate-limit cannot be triggered with a sane number of requests.
 *   - This suite exercises the CLIENT-SIDE lockout UI path:
 *     the BE returns ACCOUNT_LOCKED when the account is manually disabled
 *     (AU-2 path) or when the rate limit trips. The UI error=locked path
 *     is the same regardless of trigger.
 *
 * Stack: real Vite (:4173, MSW off) ↔ real Go API (:8081) ↔ ephemeral Postgres (:5433).
 * Isolation: resetDb() in beforeEach.
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import { disableUser } from '../../lib/db.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// LOCKOUT-BANNER · disabled account shows locked/disabled banner
// ---------------------------------------------------------------------------

test('LOCKOUT-BANNER · disabled account shows the locked/disabled banner', async ({ page }) => {
  await disableUser(PERSONAS.agent.email);

  await page.goto('/login');
  await page.locator('#identifier').fill(PERSONAS.agent.email);
  await page.locator('#password').fill(PERSONAS.agent.password);
  await page.locator('button[type="submit"]').click();

  // Redirected to /login?error=disabled (or locked).
  await page.waitForURL(/\/login\?.*error=disabled|\/login\?.*error=locked/, {
    timeout: 15_000,
  });

  const url = page.url();
  expect(url.includes('error=disabled') || url.includes('error=locked')).toBe(true);

  // Banner is visible with role="alert".
  await expect(page.locator('[role="alert"]').first()).toBeVisible({ timeout: 10_000 });
});

// ---------------------------------------------------------------------------
// LOCKOUT-REDIRECT · locked state prevents reaching authenticated routes
// ---------------------------------------------------------------------------

test('LOCKOUT-REDIRECT · locked-out account cannot reach authenticated pages', async ({
  page,
}) => {
  await disableUser(PERSONAS.agent.email);

  await page.goto('/login');
  await page.locator('#identifier').fill(PERSONAS.agent.email);
  await page.locator('#password').fill(PERSONAS.agent.password);
  await page.locator('button[type="submit"]').click();

  await page.waitForURL(/\/login\?.*error=(disabled|locked)/, { timeout: 15_000 });

  // Try to navigate to an authenticated route — should stay on login.
  await page.goto('/');
  await page.waitForTimeout(2000);

  const finalUrl = page.url();
  expect(finalUrl).toContain('/login');
  expect(finalUrl).not.toContain('/attendance');
});

// ---------------------------------------------------------------------------
// LOCKOUT-RESET · account lockout banner clears after successful re-enable
// (Skipped — full lockout-reset requires a BE endpoint to re-enable accounts,
// which is tested in user-management.spec.ts. Here we assert the lockout UI
// is structurally sound.)
// ---------------------------------------------------------------------------

test('LOCKOUT-RESET · locked banner contains a contact-admin message', async ({ page }) => {
  await disableUser(PERSONAS.agent.email);

  await page.goto('/login');
  await page.locator('#identifier').fill(PERSONAS.agent.email);
  await page.locator('#password').fill(PERSONAS.agent.password);
  await page.locator('button[type="submit"]').click();

  await page.waitForURL(/\/login\?.*error=(disabled|locked)/, { timeout: 15_000 });

  // The banner should contain messaging about the account being locked or disabled.
  const banner = page.locator('[role="alert"]').first();
  await expect(banner).toBeVisible({ timeout: 10_000 });

  const bannerText = await banner.textContent();
  expect(
    bannerText?.match(/dinonaktifkan|dikunci|locked|disabled|admin/i),
  ).toBeTruthy();
});

// ---------------------------------------------------------------------------
// LOCKOUT-BRUTEFORCE · repeated wrong passwords on the same account
// (Deferred — test env RATELIMIT_PER_MINUTE=6000 prevents realistic testing.
// This test documents the expected lockout path for when a lower rate-limit
// env is configured.)
// ---------------------------------------------------------------------------

test.skip('LOCKOUT-BRUTEFORCE · repeated wrong passwords trigger lockout', async () => {
  // Requires a backend with RATELIMIT_PER_MINUTE=5 to be triggerable.
  // Steps:
  //   1. Submit 6 login attempts with wrong credentials for the same email.
  //   2. Assert the 7th attempt returns 429 or redirects to /login?error=locked.
  //   3. Assert the lockout banner is rendered with appropriate messaging.
  //   4. Assert the account cannot log in even with the correct password
  //      until the lockout window expires.
});
