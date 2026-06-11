/**
 * tests/e1/authentication.spec.ts
 *
 * Exhaustive auth E2E suite — one test() per Gherkin scenario / edge case from
 * docs/epics/E1-foundations/prds/authentication.md §7 Acceptance criteria + §8 Cases.
 *
 * Each test is named with its AU-#/C-# so it is individually selectable in
 * `playwright test --ui` and traceable back to the spec.
 *
 * Coverage:
 *   AU-1/AU-3  successful login → dashboard, last_login_at recorded
 *   AU-1       wrong password → INVALID_CREDENTIALS banner
 *   AU-2       disabled account → ACCOUNT_DISABLED banner
 *   AU-6/C-3   token refresh → /auth/refresh returns new access_token
 *   AU-6       logout → session cleared → authed route redirects to /login
 *   UNAUTH     unauthenticated access to authed route → /login
 *   AU-4       forgot-password flow: request token + use token to set new password + re-login
 *   C-2        forgot-password unknown email → same generic 'sent' UI, zero DB rows
 *   AU-4 exp   reset with expired/invalid token → RESET_TOKEN_EXPIRED banner
 *   AU-5       [SKIPPED] rate-limiting — deferred (test env RATELIMIT_PER_MINUTE=6000)
 *
 * Stack: real Vite dev server (:4173, MSW off) ↔ real Go API (:8081) ↔ ephemeral Postgres (:5433).
 * Boot: globalSetup (global-setup.ts → lib/backend.ts → goose + seed + go run ./cmd/api).
 * Isolation: resetDb() in beforeEach (TRUNCATE + reseed via go run ./cmd/seed).
 *
 * Traceable to: AUTH-01 .. AUTH-04, HARN-01, e2e-harness-spec.md §Coverage.
 */

import { test, expect, loginAs } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import {
  seedResetToken,
  seedExpiredResetToken,
  disableUser,
  getLastLoginAt,
  countResetTokensFor,
} from '../../lib/db.js';
import { apiLogin, apiRefresh } from '../../lib/api.js';

// ---------------------------------------------------------------------------
// Isolation — each test starts from a clean, fully-seeded DB.
// ---------------------------------------------------------------------------
test.beforeEach(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// AU-1 / AU-3 — Successful login → dashboard + last_login recorded
// ---------------------------------------------------------------------------

test('AU-1/AU-3 · successful login lands on the dashboard and records last_login', async ({ page }) => {
  // Before login, last_login_at must be null (fresh seed).
  const beforeLogin = await getLastLoginAt(PERSONAS.hrAdmin.email);
  expect(beforeLogin).toBeNull();

  // Drive the real login screen via the fixture.
  await loginAs(page, PERSONAS.hrAdmin);

  // The router lands on '/' (index = DashboardScreen) — not /login.
  await expect(page).toHaveURL(/^http:\/\/localhost:4173\/?$/);

  // The shell renders the user's full name in the TopbarUser component.
  // "Sari Hadi" is the hrAdmin's full_name from seed.go.
  await expect(page.getByText('Sari Hadi')).toBeVisible();

  // After login, last_login_at must be non-null.
  const afterLogin = await getLastLoginAt(PERSONAS.hrAdmin.email);
  expect(afterLogin).not.toBeNull();
});

// ---------------------------------------------------------------------------
// AU-1 — Wrong password → INVALID_CREDENTIALS banner
// ---------------------------------------------------------------------------

test('AU-1 · wrong password shows INVALID_CREDENTIALS banner', async ({ page }) => {
  await page.goto('/login');

  // Fill hrAdmin's email but a wrong password.
  await page.locator('#identifier').fill(PERSONAS.hrAdmin.email);
  await page.locator('#password').fill('WrongPassword99!');
  await page.locator('button[type="submit"]').click();

  // The login screen navigates to /login?error=invalid and renders the Banner.
  // Wait for URL to contain ?error=invalid.
  await page.waitForURL(/\/login\?.*error=invalid/, { timeout: 15_000 });

  // The Banner component renders with role="alert" (see packages/ui/src/molecules/banner.tsx).
  // Assert the error banner is visible.
  await expect(page.locator('[role="alert"]').first()).toBeVisible();

  // URL confirms the INVALID_CREDENTIALS code was mapped.
  expect(page.url()).toContain('error=invalid');
});

// ---------------------------------------------------------------------------
// AU-2 — Disabled account → ACCOUNT_DISABLED banner
// ---------------------------------------------------------------------------

test('AU-2 · disabled account is rejected with ACCOUNT_DISABLED', async ({ page }) => {
  // Disable the agent persona before attempting login.
  await disableUser(PERSONAS.agent.email);

  await page.goto('/login');
  await page.locator('#identifier').fill(PERSONAS.agent.email);
  await page.locator('#password').fill(PERSONAS.agent.password);
  await page.locator('button[type="submit"]').click();

  // Expect redirect to /login?error=disabled.
  await page.waitForURL(/\/login\?.*error=disabled/, { timeout: 15_000 });
  expect(page.url()).toContain('error=disabled');

  // The disabled Banner (icon ShieldX, role="alert") should be visible.
  await expect(page.locator('[role="alert"]').first()).toBeVisible();
});

// ---------------------------------------------------------------------------
// AU-6 / C-3 — Token refresh issues a new access token
// ---------------------------------------------------------------------------

test('AU-6/C-3 · refresh issues a new access token', async () => {
  // Use direct API helpers (no browser) — this is a pure API assertion.
  const { body: loginBody, setCookieHeader } = await apiLogin(
    PERSONAS.hrAdmin.email,
    PERSONAS.hrAdmin.password,
  );

  expect(loginBody.access_token).toBeTruthy();
  expect(setCookieHeader).not.toBeNull();

  // Give the server a small moment so the issued-at clock can advance (JWT iat precision = 1s).
  await new Promise((r) => setTimeout(r, 1100));

  // Call /auth/refresh using the refresh cookie from login.
  const { status, body: refreshBody } = await apiRefresh(setCookieHeader!);
  expect(status).toBe(200);
  expect(refreshBody).not.toBeNull();
  expect(refreshBody!.access_token).toBeTruthy();
  // The new access token must differ from the original (new iat/jti).
  expect(refreshBody!.access_token).not.toBe(loginBody.access_token);
  expect(refreshBody!.token_type).toBe('Bearer');
  expect(refreshBody!.expires_in).toBeGreaterThan(0);
});

// ---------------------------------------------------------------------------
// AU-6 — Logout clears the session and protects authed routes
// ---------------------------------------------------------------------------

test('AU-6 · logout clears the session and protects authed routes', async ({ page }) => {
  // Log in via the real login screen.
  await loginAs(page, PERSONAS.hrAdmin);
  await expect(page).toHaveURL(/^http:\/\/localhost:4173\/?$/);

  // Trigger logout: open the UserMenu dropdown and click the logout item.
  // TopbarUser renders as a <button> with the user name in a <span>.
  // Clicking the TopbarUser button opens the dropdown.
  const topbarUserBtn = page.getByRole('button', { name: /Sari Hadi/i }).first();
  await topbarUserBtn.click();

  // The logout button in the dropdown (common.logout i18n → "Keluar" (ID) / "Sign out" (EN)).
  // app default language is Bahasa Indonesia, so the text is "Keluar".
  const logoutBtn = page.getByRole('button', { name: /keluar|sign out/i });
  await logoutBtn.click();

  // The shell navigates to /login after logout.
  await page.waitForURL(/\/login/, { timeout: 15_000 });

  // Now navigate to an authed route — it should redirect back to /login.
  await page.goto('/');
  await page.waitForURL(/\/login/, { timeout: 10_000 });
  expect(page.url()).toContain('/login');
});

// ---------------------------------------------------------------------------
// UNAUTHENTICATED — authed route while logged out redirects to /login
// ---------------------------------------------------------------------------

test('UNAUTHENTICATED · authed route while logged out redirects to /login', async ({ page }) => {
  // Fresh page, no session.
  await page.goto('/');

  // TanStack Router's beforeLoad throws redirect({ to: '/login', search: { redirect: ... } }).
  await page.waitForURL(/\/login/, { timeout: 10_000 });
  expect(page.url()).toContain('/login');
});

// ---------------------------------------------------------------------------
// AU-4 — Full password reset flow: request → token → new password → re-login
// ---------------------------------------------------------------------------

test('AU-4 · password reset: request + use token sets a new password', async ({ page }) => {
  const hrAdminEmail = PERSONAS.hrAdmin.email;
  const KNOWN_PLAINTEXT = 'e2e-reset-token-hrAdmin-001';
  const NEW_PASSWORD = 'NewP@ssw0rd99!';

  // Step 1: Navigate to forgot-password and submit the hrAdmin email.
  await page.goto('/forgot-password');
  await page.locator('#identifier').fill(hrAdminEmail);
  await page.locator('button[type="submit"]').click();

  // Step 2: The BE processes the request and the FE advances to the 'sent' state.
  // The sent state shows forgot.sentTitle = "Periksa email Anda".
  await expect(page.getByText(/Periksa email Anda|Check your email/i)).toBeVisible({
    timeout: 10_000,
  });

  // Step 3: Obtain a reset token by seeding a known plaintext into the DB.
  // (The BE created its own token row from the POST request; we replace it with our known one
  // so we control the plaintext we present to the browser.)
  const plaintoken = await seedResetToken(hrAdminEmail, KNOWN_PLAINTEXT);

  // Step 4: Navigate to the reset-password screen with the known token.
  await page.goto(`/reset-password?token=${encodeURIComponent(plaintoken)}`);

  // Step 5: Fill the new password (meets policy: ≥10 chars, upper+lower+digit+symbol).
  await page.locator('#new-password').fill(NEW_PASSWORD);
  await page.locator('#confirm-password').fill(NEW_PASSWORD);

  // Wait for the submit button to become enabled (all requirements met).
  const submitBtn = page.locator('button[type="submit"]');
  await expect(submitBtn).toBeEnabled({ timeout: 5_000 });
  await submitBtn.click();

  // Step 6: The screen transitions to the 'success' state.
  // The success state renders reset.successTitle = "Kata sandi diperbarui" and a button
  // with reset.goLogin = "Masuk Sekarang".
  await expect(page.getByText(/Kata sandi diperbarui|Password.*updated/i)).toBeVisible({
    timeout: 10_000,
  });

  // Step 7: Log in with the NEW password — should land on the dashboard.
  await page.goto('/login');
  await page.locator('#identifier').fill(hrAdminEmail);
  await page.locator('#password').fill(NEW_PASSWORD);
  await page.locator('button[type="submit"]').click();
  await page.waitForURL(/^http:\/\/localhost:4173\/?$/, { timeout: 15_000 });
  await expect(page.getByText('Sari Hadi')).toBeVisible();
});

// ---------------------------------------------------------------------------
// C-2 — Forgot-password for an unknown email → same generic 'sent' response
// ---------------------------------------------------------------------------

test('C-2 · forgot-password for an unknown email returns the same generic response', async ({ page }) => {
  const unknownEmail = 'unknown.user@no-such-domain.test';

  await page.goto('/forgot-password');
  await page.locator('#identifier').fill(unknownEmail);
  await page.locator('button[type="submit"]').click();

  // The FE always advances to 'sent' (anti-enumeration, C-2 per authentication.md).
  // The sent state shows forgot.sentTitle = "Periksa email Anda".
  await expect(page.getByText(/Periksa email Anda|Check your email/i)).toBeVisible({
    timeout: 10_000,
  });

  // The DB must have zero password_reset_tokens rows for the unknown email.
  const count = await countResetTokensFor(unknownEmail);
  expect(count).toBe(0);
});

// ---------------------------------------------------------------------------
// AU-4 (expired) — Reset with an expired/invalid token → RESET_TOKEN_EXPIRED
// ---------------------------------------------------------------------------

test('AU-4 · reset with an expired/invalid token shows RESET_TOKEN_EXPIRED', async ({ page }) => {
  const EXPIRED_PLAINTEXT = 'e2e-expired-token-001';

  // Seed an already-expired token for the hrAdmin persona.
  const expiredToken = await seedExpiredResetToken(PERSONAS.hrAdmin.email, EXPIRED_PLAINTEXT);

  // Navigate to the reset screen with the expired token.
  await page.goto(`/reset-password?token=${encodeURIComponent(expiredToken)}`);

  // Fill a valid new password.
  await page.locator('#new-password').fill('ExpiredP@ss99!');
  await page.locator('#confirm-password').fill('ExpiredP@ss99!');

  const submitBtn = page.locator('button[type="submit"]');
  await expect(submitBtn).toBeEnabled({ timeout: 5_000 });
  await submitBtn.click();

  // The BE returns 401 RESET_TOKEN_EXPIRED; the FE sets tokenExpired=true and renders
  // the Banner with the expired title (reset.expiredTitle i18n key).
  // The Banner has role="alert" (packages/ui/src/molecules/banner.tsx).
  await expect(page.locator('[role="alert"]').first()).toBeVisible({ timeout: 10_000 });

  // Verify we're still on the reset-password page (no navigation to success).
  expect(page.url()).toContain('/reset-password');
});

// ---------------------------------------------------------------------------
// AU-5 — [SKIPPED] Rate-limiting / lockout
// ---------------------------------------------------------------------------

test.skip('AU-5 · repeated failures are rate-limited (deferred to a later phase)', async () => {
  // The test environment uses RATELIMIT_PER_MINUTE=6000 so the real limiter
  // cannot be triggered with a reasonable number of requests in a test run.
  // This scenario is deferred to a dedicated rate-limit phase that will:
  //   1. Start a backend with RATELIMIT_PER_MINUTE=5 (or a test env override)
  //   2. Submit 6 login attempts with wrong credentials
  //   3. Assert the 7th attempt returns 429 with an ACCOUNT_LOCKED-or-rate-limited error
  //   4. Assert the UI navigates to /login?error=locked and shows the locked Banner
  //
  // Ref: authentication.md AU-5, engineering.md RATELIMIT_PER_MINUTE env var.
});
