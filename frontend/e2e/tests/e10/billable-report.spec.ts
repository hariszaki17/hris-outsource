/**
 * tests/e10/billable-report.spec.ts  ·  @e10-report
 *
 * E10 · Billable attendance report (GET /reports/attendance-billable) — BR-1..BR-7, INV-4.
 *
 * Proves billable-report-screen.tsx against the REAL Go BE (MSW off):
 *   - HR: /reports renders the TitleBand ("Kehadiran & Jam Billable" + "Ekspor" button), the
 *     period date inputs, the 4 summary StatCards (Jam Billable / Payable / Worked / Tingkat
 *     Verifikasi), and the DataTable. The seed plants VERIFIED billable rows (SWP-ATT-9007/9008
 *     @ CMP-0021, bound to the billable AC-001) so the report is NON-EMPTY for the current
 *     month — at least one div.border-b row with billable hours renders.
 *   - leader scope: a shift_leader running the report is SERVER-SCOPED to their own company
 *     (the company filter is hidden in the UI); the report renders without error and the
 *     payload is scoped to SWP-CMP-0021 (a cross-company company_id → 403 OUT_OF_SCOPE).
 *
 * Selectors anchored on the REAL billable-report-screen.tsx (DataTable rows = div.border-b,
 * keyed on group_key); the scope assertion is driven through the real endpoint via apiAs.
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import { apiAs, errorCode, gotoReady, reportRow } from '../../lib/e10-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

/** Current-month period bounds (matches the screen's DEFAULT_PERIOD_*). */
function currentMonthRange(): { start: string; end: string } {
  const now = new Date();
  const start = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-01`;
  const end = new Date(now.getFullYear(), now.getMonth() + 1, 0).toISOString().slice(0, 10);
  return { start, end };
}

// ---------------------------------------------------------------------------
// HR — report renders summary + rows (non-empty for the seeded VERIFIED billable rows)
// ---------------------------------------------------------------------------

test('REPORT-hr-renders · HR /reports renders TitleBand + summary StatCards + billable rows', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await gotoReady(page, '/reports');

  // TitleBand + Ekspor button render.
  await expect(page.getByRole('heading', { name: 'Kehadiran & Jam Billable' })).toBeVisible({
    timeout: 30_000,
  });
  await expect(page.getByRole('button', { name: 'Ekspor' })).toBeVisible();

  // Period date inputs render (aria-labelled).
  await expect(page.getByLabel('Periode dari')).toBeVisible();
  await expect(page.getByLabel('Periode sampai')).toBeVisible();

  // Summary StatCards render their labels.
  await expect(page.getByText('Jam Billable', { exact: false }).first()).toBeVisible();
  await expect(page.getByText('Tingkat Verifikasi', { exact: false }).first()).toBeVisible();

  // CONTRACT: the current-month report is NON-EMPTY (seed plants VERIFIED billable rows for
  // CMP-0021). Assert via the real endpoint that rows + a positive billable summary exist.
  const { start, end } = currentMonthRange();
  const res = await apiAs(
    page,
    'GET',
    `/reports/attendance-billable?period_start=${start}&period_end=${end}&group_by=employee`,
  );
  expect(res.status, JSON.stringify(res.body)).toBe(200);
  const report = (res.body as {
    data: {
      rows: Array<{ group_key: string; group_label: string; billable_hours: number }>;
      summary: { total_billable_hours: number };
    };
  }).data;
  expect(report.rows.length).toBeGreaterThan(0);
  expect(report.summary.total_billable_hours).toBeGreaterThan(0);

  // UI: the screen unwraps the BE {data} envelope (the 11-04 fix) and renders the report —
  // at least one DataTable row (div.border-b) for a seeded billable employee is visible, and
  // the row shows its group label (e.g. "Dewi Lestari").
  const firstRow = report.rows[0];
  await expect(reportRow(page, firstRow.group_key)).toBeVisible({ timeout: 30_000 });
  await expect(reportRow(page, firstRow.group_key)).toContainText(firstRow.group_label);
});

// ---------------------------------------------------------------------------
// Leader scope — server-scoped to own company; cross-company → OUT_OF_SCOPE
// ---------------------------------------------------------------------------

// Skipped: BR-4 (PRD) says a shift_leader sees their own company's billable report,
// but the shipped role model (packages/shared/src/rbac.ts + NAVIGATION-AND-RBAC.md)
// gives shift_leader NO reports.read. This conflict needs an EPICS.md §8 ruling before
// the test can assert the correct behavior (SL own-company 200 vs SL fully denied 403).
test.skip('REPORT-leader-scoped · shift_leader report is own-company scoped; cross-company company_id → 403 OUT_OF_SCOPE', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await gotoReady(page, '/reports');

  // The report screen renders for the leader (no company filter — it's server-scoped).
  await expect(page.getByRole('heading', { name: 'Kehadiran & Jam Billable' })).toBeVisible({
    timeout: 30_000,
  });

  const { start, end } = currentMonthRange();

  // Own company (implicit / matching) → 200.
  const ownRes = await apiAs(
    page,
    'GET',
    `/reports/attendance-billable?period_start=${start}&period_end=${end}&company_id=SWP-CMP-0021&group_by=employee`,
  );
  expect(ownRes.status, JSON.stringify(ownRes.body)).toBe(200);

  // A DIFFERENT company → 403 OUT_OF_SCOPE (leader cannot read another company's data).
  const crossRes = await apiAs(
    page,
    'GET',
    `/reports/attendance-billable?period_start=${start}&period_end=${end}&company_id=SWP-CMP-0022&group_by=employee`,
  );
  expect(crossRes.status).toBe(403);
  expect(errorCode(crossRes.body)).toBe('OUT_OF_SCOPE');
});

// ---------------------------------------------------------------------------
// Period guard — a >1yr range → 422 REPORT_PERIOD_TOO_WIDE (BR-5)
// ---------------------------------------------------------------------------

test('REPORT-period-too-wide · a >1 year range → 422 REPORT_PERIOD_TOO_WIDE', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await gotoReady(page, '/reports');

  const res = await apiAs(
    page,
    'GET',
    '/reports/attendance-billable?period_start=2024-01-01&period_end=2025-06-01&group_by=employee',
  );
  expect(res.status).toBe(422);
  expect(errorCode(res.body)).toBe('REPORT_PERIOD_TOO_WIDE');
});
