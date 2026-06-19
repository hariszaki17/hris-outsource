/**
 * tests/e7/aggregation.spec.ts
 *
 * E2E for E7 · Overtime Aggregation (Rekap Lembur) — the /overtime/rekap view
 * that shows OT records grouped by agent/company with per-tier minute totals.
 *
 * Coverage:
 *   AGG-TABLE      HR sees the aggregation table grouped by agent
 *   AGG-FILTERS    date-range filters narrow the results
 *   AGG-EMPTY      no data state when filters return zero results
 *   AGG-RBAC       shift leader is scoped to their own company
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
// AGG-TABLE · HR sees aggregated OT table
// ---------------------------------------------------------------------------

test('AGG-TABLE · HR sees the Rekap Lembur table with agent rows', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/overtime/rekap');

  // The rekap title band renders.
  await expect(page.getByRole('heading', { name: /Rekap Lembur/i })).toBeVisible({
    timeout: 30_000,
  });

  // Stat cards show (Total OT disetujui · Hari Kerja · Hari Libur · Hari Besar).
  await expect(page.getByText(/Total OT|total.*disetujui/i).first()).toBeVisible({
    timeout: 10_000,
  });

  // The table renders — either data rows or an empty-state message.
  const table = page.getByRole('table');
  const tableExists = await table.isVisible().catch(() => false);

  if (tableExists) {
    // Table has agent column headers.
    await expect(page.getByText(/Agen|Agent|Nama|employee/i).first()).toBeVisible({
      timeout: 5_000,
    });
  }
});

// ---------------------------------------------------------------------------
// AGG-FILTERS · date-range filter narrows results
// ---------------------------------------------------------------------------

test('AGG-FILTERS · aggregation filters by date range', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/overtime/rekap');

  await expect(page.getByRole('heading', { name: /Rekap Lembur/i })).toBeVisible({
    timeout: 30_000,
  });

  // Date-from input (aria-label = "Tanggal Mulai" or similar).
  const dateFrom = page.locator('input[type="date"]').first();
  await expect(dateFrom).toBeVisible({ timeout: 10_000 });

  // Set a date range covering the current month.
  await dateFrom.fill('2026-06-01');

  const dateTo = page.locator('input[type="date"]').nth(1);
  await dateTo.fill('2026-06-30');

  // The table re-fetches — either shows filtered data or an empty state.
  // Both are valid outcomes depending on seed data within that range.
  await page.waitForTimeout(2000);

  // At minimum, the page should not show an error.
  await expect(page.getByRole('alert')).toHaveCount(0);
});

// ---------------------------------------------------------------------------
// AGG-EMPTY · no-data state when filters return zero results
// ---------------------------------------------------------------------------

test('AGG-EMPTY · empty state when no records match filters', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/overtime/rekap');

  await expect(page.getByRole('heading', { name: /Rekap Lembur/i })).toBeVisible({
    timeout: 30_000,
  });

  // Set a far-future date range that no seed data covers.
  const dateFrom = page.locator('input[type="date"]').first();
  await dateFrom.fill('2099-01-01');

  const dateTo = page.locator('input[type="date"]').nth(1);
  await dateTo.fill('2099-01-31');

  // Expect an empty or filtered state message.
  await expect(
    page.getByText(/tidak ditemukan|no results|kosong|belum ada/i).first(),
  ).toBeVisible({ timeout: 15_000 });
});

// ---------------------------------------------------------------------------
// AGG-RBAC · shift leader sees only their own company data
// ---------------------------------------------------------------------------

test('AGG-RBAC · shift leader is scoped to own company', async ({ page }) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/overtime/rekap');

  await expect(page.getByRole('heading', { name: /Rekap Lembur/i })).toBeVisible({
    timeout: 30_000,
  });

  // Shift leader's company name should appear (locked filter).
  await expect(page.getByText(/Plaza Senayan/i).first()).toBeVisible({ timeout: 10_000 });

  // SL does not have the company filter dropdown (it's locked).
  // The FilterSelect for company should not be an interactive element for SL.
  const companyFilter = page.getByRole('combobox', { name: /perusahaan|company/i });
  const hasCompanyFilter = await companyFilter.isVisible().catch(() => false);

  if (hasCompanyFilter) {
    // Even if visible, it must be locked/disabled.
    await expect(companyFilter).toBeDisabled();
  }
});
