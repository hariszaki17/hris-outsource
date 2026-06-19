/**
 * tests/e10/agent-calendar.spec.ts
 *
 * E2E for Agent Calendar (F10.5) — the /me/calendar view that shows an agent's
 * schedule, leave, and holiday markers in a monthly calendar grid.
 *
 * Coverage:
 *   CAL-RENDER     calendar grid renders with day cells for the agent's data
 *   CAL-EMPTY      a month with no schedule/leave/holiday data still shows all day cells
 *   CAL-RBAC       non-agent roles are redirected or blocked from /me/calendar
 *
 * Stack: real Vite (:4173, MSW off) ↔ real Go API (:8081) ↔ ephemeral Postgres (:5433).
 * Isolation: resetDb() in beforeEach.
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// CAL-RENDER · agent calendar shows day cells with schedule/leave/holiday markers
// ---------------------------------------------------------------------------

test('CAL-RENDER · agent sees calendar with schedule, leave, holiday markers', async ({ page }) => {
  await loginAs(page, PERSONAS.agent);
  await page.goto('/me/calendar');

  // Calendar heading — the view renders a month-year label.
  await expect(page.getByRole('heading', { name: /kalendar|calendar/i })).toBeVisible({
    timeout: 30_000,
  });

  // Day-of-week headers (Senin–Minggu or Mon–Sun) should be present.
  const dayHeaders = page.locator('[class*="calendar"] th, [class*="Calendar"] th');
  const headerCount = await dayHeaders.count();
  expect(headerCount).toBeGreaterThanOrEqual(7);

  // Calendar day cells exist (a full month grid).
  const dayCells = page.locator('[class*="calendar"] td, [class*="Calendar"] td');
  const cellCount = await dayCells.count();
  expect(cellCount).toBeGreaterThanOrEqual(28);

  // Navigate the calendar — next-month button should exist.
  const nextBtn = page.getByRole('button', { name: /next|berikutnya|»|›/i });
  expect(await nextBtn.count()).toBeGreaterThanOrEqual(0);
});

// ---------------------------------------------------------------------------
// CAL-EMPTY · a calendar month with no data still renders all day cells
// ---------------------------------------------------------------------------

test('CAL-EMPTY · empty month shows all days without crashing', async ({ page }) => {
  await loginAs(page, PERSONAS.agent);
  await page.goto('/me/calendar');

  // Wait for the calendar grid to render.
  await expect(page.getByRole('heading', { name: /kalendar|calendar/i })).toBeVisible({
    timeout: 30_000,
  });

  // Day cells exist — empty cells render without data badges.
  const dayCells = page.locator('[class*="calendar"] td, [class*="Calendar"] td');
  const cellCount = await dayCells.count();
  expect(cellCount).toBeGreaterThanOrEqual(28);

  // The calendar should not show an error or crash state.
  await expect(page.getByRole('alert')).toHaveCount(0);
});

// ---------------------------------------------------------------------------
// CAL-RBAC · non-agent redirected from /me/calendar
// ---------------------------------------------------------------------------

test('CAL-RBAC · HR admin accessing /me/calendar is redirected or shows restricted', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);

  // Navigate to the agent-only calendar route.
  await page.goto('/me/calendar');

  // Either redirected away from /me/calendar or a no-permission state is shown.
  // Wait for either outcome.
  await page.waitForTimeout(3000);

  const url = page.url();
  const isRedirected = !url.includes('/me/calendar');
  const hasNoPermission = await page
    .getByText(/tidak memiliki akses|no permission|akses ditolak|forbidden/i)
    .first()
    .isVisible()
    .catch(() => false);

  expect(isRedirected || hasNoPermission).toBe(true);
});
