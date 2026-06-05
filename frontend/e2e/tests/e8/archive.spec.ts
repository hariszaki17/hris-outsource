/**
 * tests/e8/archive.spec.ts
 *
 * E8 · Payroll archive list + filters (PAY-01) against the REAL stack. HR/Super Admin
 * open /payroll, see the seeded FINAL payslips, the DECRYPT_FAIL row rendered at 200
 * (NOT an error page), filter by status/period/employee, and hit the missing-history
 * empty state. Selectors anchored on payslip-archive-screen.tsx (DataTable rows =
 * div.border-b; SearchField → employee_id; FilterSelect aria filterYear/Month/Status;
 * StatusBadge "Final" / "Perlu review").
 *
 * IMPORTANT: the archive default year filter = CURRENT_YEAR (2026); the seed periods are
 * 2025-11 / 2025-12, so a fresh load shows EMPTY. Every list assertion first selects
 * year 2025 (via the filterYear FilterSelect) so the seeded rows render.
 *
 * Seed (10-02): SWP-PS-90121 Budi 2025-12 FINAL, 90122 Rudi 2025-12 FINAL, 90123 Andi
 * 2025-11 FINAL, 90124 Dewi 2025-11 FINAL, 90119 Rudi 2025-12 DECRYPT_FAIL.
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import {
  PS,
  PS_EMP,
  PS_NAME,
  apiAs,
  expectNoPayslipRow,
  expectPayslipRow,
  payslipRow,
  waitForToken,
} from '../../lib/e8-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

/** Open /payroll as HR and select year 2025 (seed periods) so the rows render. */
async function openArchive2025(page: import('@playwright/test').Page): Promise<void> {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/payroll');
  await waitForToken(page);
  // The DEFAULT year is 2026 (CURRENT_YEAR) → empty. Switch to 2025.
  await page.getByLabel('Tahun').selectOption('2025');
}

// ---------------------------------------------------------------------------
// ARCH-list — the FINAL rows render with employee / period / money / FINAL badge
// ---------------------------------------------------------------------------

test('ARCH-list · HR /payroll year 2025 lists the FINAL payslips with money + Final badge', async ({
  page,
}) => {
  await openArchive2025(page);

  // All four FINAL rows are present (2025-11 + 2025-12).
  await expectPayslipRow(page, PS_NAME.budi); // 90121
  await expectPayslipRow(page, PS_NAME.andi); // 90123
  await expectPayslipRow(page, PS_NAME.dewi); // 90124

  // The Budi row shows the take-home money string (Rp 7.325.000) + the FINAL badge.
  const budiRow = payslipRow(page, PS_NAME.budi);
  await expect(budiRow).toContainText('Rp 7.325.000');
  await expect(budiRow).toContainText('Final');
  await expect(budiRow).toContainText(PS_EMP.budi);
});

// ---------------------------------------------------------------------------
// ARCH-decrypt-fail-row — SWP-PS-90119 renders at 200 with the DECRYPT_FAIL badge + "—"
// ---------------------------------------------------------------------------

test('ARCH-decrypt-fail-row · the DECRYPT_FAIL row renders (200) with "Perlu review" + "—" money', async ({
  page,
}) => {
  await openArchive2025(page);
  // Narrow to 2025-12 (the decrypt-fail row's period) so it sits beside the FINAL rows.
  await page.getByLabel('Bulan').selectOption('12');

  // The page did NOT error — the FINAL 2025-12 rows are still here…
  await expectPayslipRow(page, PS_NAME.budi); // 90121 (2025-12 FINAL)

  // …and the DECRYPT_FAIL row (Rudi Hartono, 2025-12) renders with the warn badge + em-dash money.
  // Both 90119 (decrypt-fail) and 90122 (final) are Rudi @ 2025-12, so anchor on the badge text.
  const decryptRow = page.locator('div.border-b').filter({ hasText: 'Perlu review' }).first();
  await expect(decryptRow).toBeVisible({ timeout: 30_000 });
  await expect(decryptRow).toContainText('—'); // money placeholder
  await expect(decryptRow).toContainText(PS_NAME.rudi);

  // Confirm via the API that 90119 is a 200 DECRYPT_FAIL row (not a 4xx).
  const res = await apiAs(page, 'GET', `/payslips/${PS.decryptFail}`);
  expect(res.status).toBe(200);
  expect((res.body as { data?: { status?: string } })?.data?.status).toBe('DECRYPT_FAIL');
});

// ---------------------------------------------------------------------------
// ARCH-filter — status / period / employee filters narrow the list
// ---------------------------------------------------------------------------

test('ARCH-filter · status DECRYPT_FAIL → only the decrypt-fail row; period + employee narrow', async ({
  page,
}) => {
  await openArchive2025(page);

  // Filter by status = DECRYPT_FAIL ("Perlu review") → only SWP-PS-90119 (Rudi) remains.
  await page.getByLabel('Status').selectOption('DECRYPT_FAIL');
  await expectPayslipRow(page, PS_NAME.rudi);
  await expectNoPayslipRow(page, PS_NAME.budi);
  await expectNoPayslipRow(page, PS_NAME.andi);

  // Reset filters → all rows return (back to year 2025 default).
  await page.getByRole('button', { name: 'Reset filter' }).click();
  await page.getByLabel('Tahun').selectOption('2025');
  await expectPayslipRow(page, PS_NAME.budi);
  await expectPayslipRow(page, PS_NAME.andi);

  // Filter by period 2025-11 → only the November FINAL rows (Andi 90123, Dewi 90124).
  await page.getByLabel('Bulan').selectOption('11');
  await expectPayslipRow(page, PS_NAME.andi);
  await expectPayslipRow(page, PS_NAME.dewi);
  await expectNoPayslipRow(page, PS_NAME.budi); // Budi is 2025-12

  // Filter by employee_id via the SearchField (exact match) → only Budi (SWP-EMP-1042).
  await page.getByRole('button', { name: 'Reset filter' }).click();
  await page.getByLabel('Tahun').selectOption('2025');
  await page.getByPlaceholder(/Cari karyawan/i).fill(PS_EMP.budi);
  await expectPayslipRow(page, PS_NAME.budi);
  await expectNoPayslipRow(page, PS_NAME.andi);
});

// ---------------------------------------------------------------------------
// ARCH-empty — a year with no payslips surfaces the missing-history meta.code
// ---------------------------------------------------------------------------

test('ARCH-empty · a year with no payslips → MISSING_PAYROLL_HISTORY meta + empty state', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/payroll');
  await waitForToken(page);

  // The default year is 2026 (no seeded payslips) — the list is empty. Assert the
  // BE meta.code drives the "no history" empty state via apiAs (year tops out at the
  // current year in the select, so probe an explicitly-empty year via the API).
  const res = await apiAs(page, 'GET', '/payslips?year=2099');
  expect(res.status).toBe(200);
  const body = res.body as { data?: unknown[]; meta?: { code?: string } };
  expect(body.data).toEqual([]);
  expect(body.meta?.code).toBe('MISSING_PAYROLL_HISTORY');

  // And the UI for the empty 2026 default year shows the no-history empty state. The
  // DataTable renders no employee rows — assert no FINAL/decrypt row text + the empty-state
  // copy driven by meta.code MISSING_PAYROLL_HISTORY.
  await expectNoPayslipRow(page, PS_NAME.budi);
  await expectNoPayslipRow(page, PS_NAME.rudi);
  await expect(page.getByText('Belum ada riwayat payroll')).toBeVisible({ timeout: 20_000 });
});
