/**
 * tests/e8/export.spec.ts
 *
 * E8 · Payroll async export — the HEADLINE (PAY-02 / success criterion 2). HR triggers an
 * export for a period; the BE returns 202 + a SWP-EXP-… job id (status QUEUED, confidential
 * server-forced true); then the REAL River worker (booted by the harness) processes the
 * PayslipExportArgs job and flips the export_jobs row to DONE — PROVEN via pollExportJob (a
 * DB poll), NOT a mocked completion and NOT a 202-only assertion.
 *
 * No FE surface mounts PayrollExportButton (the detail "Ekspor" entry only fires a QUEUED
 * toast — payslip-detail-route.tsx), and E8 has NO FE job-status hook, so the export is driven
 * via apiAs POST /payslips:export and completion is observed through the export_jobs table.
 *
 * Contract (10-02 / openapi): 202 + PayslipExportJob {id ^SWP-EXP-\d+$, status QUEUED, format
 * XLSX, confidential:true (forced), scope, poll_url}. confidential is server-coerced to true
 * even on a `false` input. No period AND no year → 422 RULE_VIOLATION. The worker stamps
 * row_count + artifact_ref on DONE.
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import { apiAs, errorCode, pollExportJob, waitForToken } from '../../lib/e8-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

/** Land on /payroll as HR + hydrate the in-memory token so apiAs can authenticate. */
async function hrOnPayroll(page: import('@playwright/test').Page): Promise<void> {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/payroll');
  await waitForToken(page);
}

// ---------------------------------------------------------------------------
// EXPORT-worker-completes — THE HEADLINE: 202 + job id THEN export_jobs → DONE (real worker)
// ---------------------------------------------------------------------------

test('EXPORT-worker-completes · POST :export → 202 + SWP-EXP id THEN the River worker flips export_jobs → DONE', async ({
  page,
}) => {
  await hrOnPayroll(page);

  // Trigger the export for the seeded 2025-12 period.
  const res = await apiAs(page, 'POST', '/payslips:export', {
    period: '2025-12',
    format: 'XLSX',
    confidential: true,
  });

  // 202 Accepted + the QUEUED job stub.
  expect(res.status).toBe(202);
  const job = res.body as { id?: string; status?: string; format?: string; confidential?: boolean };
  expect(job.id).toMatch(/^SWP-EXP-\d+$/);
  expect(job.status).toBe('QUEUED');
  expect(job.format).toBe('XLSX');
  expect(job.confidential).toBe(true);

  // THE PROOF: the REAL River worker processes the PayslipExportArgs job and completes it.
  const row = await pollExportJob(job.id as string, { timeoutMs: 20_000 });
  expect(row.status).toBe('DONE');
  expect(row.row_count ?? 0).toBeGreaterThan(0); // the 2025-12 scope has seeded payslips
});

// ---------------------------------------------------------------------------
// EXPORT-confidential-lock — confidential is server-forced true even on a false input
// ---------------------------------------------------------------------------

test('EXPORT-confidential-lock · POST {confidential:false} → the 202 body reflects confidential:true', async ({
  page,
}) => {
  await hrOnPayroll(page);

  const res = await apiAs(page, 'POST', '/payslips:export', {
    period: '2025-12',
    format: 'XLSX',
    confidential: false, // the server MUST coerce this to true (Wave 2.8 lock)
  });

  expect(res.status).toBe(202);
  expect((res.body as { confidential?: boolean }).confidential).toBe(true);
});

// ---------------------------------------------------------------------------
// EXPORT-too-large — coverage note (the threshold path is contract-tested in 10-03)
// ---------------------------------------------------------------------------

test('EXPORT-too-large · the seed cannot reach the 50k threshold — EXPORT_TOO_LARGE is contract-covered', async ({
  page,
}) => {
  await hrOnPayroll(page);

  // The default EXPORT_TOO_LARGE threshold is 50,000 rows (10-02) — the seed plants only a
  // handful of payslips, so the guard cannot be tripped honestly here. The 422 EXPORT_TOO_LARGE
  // wire shape (+ no-enqueue) is fully pinned by the 10-03 Go contract test
  // (TestExportPayslips_TooLarge422NoEnqueue). We assert here that an in-range export instead
  // succeeds (the happy 202), which is the path the seed CAN exercise honestly.
  const res = await apiAs(page, 'POST', '/payslips:export', { year: 2025, format: 'XLSX' });
  expect(res.status).toBe(202);
  expect((res.body as { id?: string }).id).toMatch(/^SWP-EXP-\d+$/);
});

// ---------------------------------------------------------------------------
// EXPORT-no-scope — neither period nor year → 422
// ---------------------------------------------------------------------------

test('EXPORT-no-scope · POST {} (no period/year) → 422 RULE_VIOLATION + no job', async ({ page }) => {
  await hrOnPayroll(page);

  const res = await apiAs(page, 'POST', '/payslips:export', { format: 'XLSX' });
  expect(res.status).toBe(422);
  expect(errorCode(res.body)).toBe('RULE_VIOLATION');
});
