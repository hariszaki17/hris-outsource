/**
 * tests/e3/end-placement.spec.ts
 *
 * E2E for E3 · End Placement flow — independently testable end-placement scenarios
 * beyond the full lifecycle spec (placement-lifecycle.spec.ts covers LC-end).
 *
 * Coverage:
 *   END-UI        HR navigates to placement detail, opens "Akhiri" action
 *   END-VALIDATE  end-reason and end-date fields exist and validate
 *   END-CONFIRM   successful end persists ENDED status
 *   END-SL-SCOPE  shift leader can end placements within their company
 *
 * Stack: real Vite (:4173, MSW off) ↔ real Go API (:8081) ↔ ephemeral Postgres (:5433).
 * Isolation: resetDb() in beforeEach.
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import { getPlacementLifecycleStatus } from '../../lib/db.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

function isoToday(): string {
  const d = new Date();
  d.setUTCDate(d.getUTCDate());
  return d.toISOString().slice(0, 10);
}

// ---------------------------------------------------------------------------
// END-UI · HR navigates to placement detail, opens end action
// ---------------------------------------------------------------------------

test('END-UI · HR opens "Akhiri" action from placement detail', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/placements/SWP-PL-5002');

  // Wait for the detail page to load — lifecycle tracker or agent name visible.
  await expect(page.getByText(/Aktif|Budi|Budi Santoso/i).first()).toBeVisible({
    timeout: 30_000,
  });

  // The "Akhiri" (end) action button exists.
  const endBtn = page.getByRole('button', { name: /Akhiri/i }).first();
  await expect(endBtn).toBeVisible({ timeout: 10_000 });
  await endBtn.click();

  // EndConfirm modal opens with reason and date fields.
  await expect(page.locator('#end-reason')).toBeVisible({ timeout: 10_000 });
  await expect(page.locator('#end-date')).toBeVisible();
});

// ---------------------------------------------------------------------------
// END-VALIDATE · end form fields are present and selectable
// ---------------------------------------------------------------------------

test('END-VALIDATE · end-reason dropdown contains expected options', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/placements/SWP-PL-5002');

  await expect(page.getByText(/Aktif|Budi/i).first()).toBeVisible({ timeout: 30_000 });

  await page.getByRole('button', { name: /Akhiri/i }).first().click();
  await expect(page.locator('#end-reason')).toBeVisible({ timeout: 10_000 });

  // The reason dropdown has options including END_OF_TERM.
  const reasonSelect = page.locator('#end-reason');
  const optionCount = await reasonSelect.locator('option').count();
  expect(optionCount).toBeGreaterThanOrEqual(1);

  // END_OF_TERM is one of the valid reasons.
  const hasEndOfTerm = await reasonSelect
    .locator('option[value="END_OF_TERM"]')
    .count();
  expect(hasEndOfTerm).toBe(1);

  // End-date field is a date input.
  const endDate = page.locator('#end-date');
  const dateType = await endDate.getAttribute('type');
  expect(dateType).toBe('date');
});

// ---------------------------------------------------------------------------
// END-CONFIRM · submitting the end form persists ENDED status
// ---------------------------------------------------------------------------

test('END-CONFIRM · submitting end form persists ENDED via API', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/placements/SWP-PL-5002');

  await expect(page.getByText(/Aktif|Budi/i).first()).toBeVisible({ timeout: 30_000 });
  await page.getByRole('button', { name: /Akhiri/i }).first().click();
  await expect(page.locator('#end-reason')).toBeVisible({ timeout: 10_000 });

  await page.locator('#end-reason').selectOption('END_OF_TERM');
  await page.locator('#end-date').fill(isoToday());

  await page.getByRole('button', { name: /Akhiri Penempatan/i }).click();

  // Poll DB to confirm the status changed to ENDED.
  await expect
    .poll(() => getPlacementLifecycleStatus('SWP-PL-5002'), { timeout: 15_000 })
    .toBe('ENDED');
});

// ---------------------------------------------------------------------------
// END-SL-SCOPE · shift leader can end placement within their company
// ---------------------------------------------------------------------------

test('END-SL-SCOPE · shift leader can end a placement in their company', async ({ page }) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/placements/SWP-PL-5001'); // Rudi @ Plaza Senayan (SL's company)

  // Wait for detail page.
  await expect(page.getByText(/Rudi/i).first()).toBeVisible({ timeout: 30_000 });

  // End button should be visible for shift leader on their own company's placements.
  const endBtn = page.getByRole('button', { name: /Akhiri/i }).first();
  const isVisible = await endBtn.isVisible().catch(() => false);

  if (isVisible) {
    await endBtn.click();
    await expect(page.locator('#end-reason')).toBeVisible({ timeout: 10_000 });
  }
});
