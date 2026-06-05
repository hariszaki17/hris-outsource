/**
 * tests/e10/exports.spec.ts  ·  @e10-export
 *
 * E10 · Generic export framework (POST /exports → GET /exports/{id} → :cancel) — EX-1..EX-6.
 * Proven against the REAL stack (Go API + the ReportExportWorker booted by the harness +
 * ephemeral Postgres, MSW off):
 *
 *   - EXPORT-worker-DONE: POST /exports (ATTENDANCE_BILLABLE, EXCEL) → 202 + a SWP-EXP-… id
 *     (status QUEUED) THEN the real ReportExportWorker flips export_jobs → DONE — PROVEN via
 *     pollExportJob (a DB poll), not a 202-only assertion. Also drives the UI: the billable
 *     report "Ekspor" button → ExportModal → confirm EXCEL → the modal reaches the progress/
 *     success step as the FE polls GET /exports/{id} (DONE→COMPLETED at the wire).
 *   - EXPORT-cancel: POST /exports → :cancel → the job reaches CANCELLED (the real cancel path;
 *     retried across the cancel-vs-worker race so the assertion is deterministic).
 *   - EXPORT-pdf-unsupported: POST /exports format=PDF → 422 EXPORT_FORMAT_UNSUPPORTED (the UI
 *     only offers Excel, so this is an apiAs-level assertion, mirroring the e8 export pattern).
 *
 * The worker-completes proof reuses pollExportJob (export_jobs.status === 'DONE'); the DB
 * status is DONE even though the wire maps it to COMPLETED (11-02b DTO).
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import {
  apiAs,
  errorCode,
  gotoReady,
  pollExportJob,
  pollExportJobUntil,
  waitForToken,
} from '../../lib/e10-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

const BILLABLE_REQUEST = {
  report_type: 'ATTENDANCE_BILLABLE',
  format: 'EXCEL',
  filters: { period_start: '2026-06-01', period_end: '2026-06-30' },
};

// ---------------------------------------------------------------------------
// THE HEADLINE — 202 + SWP-EXP id THEN the real worker flips export_jobs → DONE
// + the UI export modal reaches its terminal (success) step.
// ---------------------------------------------------------------------------

test('EXPORT-worker-DONE · POST /exports → 202 + SWP-EXP id THEN ReportExportWorker flips export_jobs → DONE; UI modal completes', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await gotoReady(page, '/reports');

  // API: create the export → 202 + QUEUED SWP-EXP job.
  const res = await apiAs(page, 'POST', '/exports', BILLABLE_REQUEST);
  expect(res.status, JSON.stringify(res.body)).toBe(202);
  const job = (res.body as { data: { id: string; status: string; format: string } }).data;
  expect(job.id).toMatch(/^SWP-EXP-\d+$/);
  expect(job.status).toBe('QUEUED');
  expect(job.format).toBe('EXCEL');

  // THE PROOF: the real ReportExportWorker processes the ReportExportArgs and completes it.
  const row = await pollExportJob(job.id, { timeoutMs: 20_000 });
  expect(row.status).toBe('DONE');

  // UI: drive the report screen's "Ekspor" button → ExportModal (Radix dialog) → confirm EXCEL
  // in the modal → the modal advances through progress to the success step (the FE polls
  // GET /exports/{id}, which maps the DB DONE → wire COMPLETED). Scope the confirm + download
  // to the dialog so the modal's "Ekspor" isn't confused with the TitleBand's "Ekspor".
  await page.getByRole('button', { name: 'Ekspor' }).first().click();
  const modal = page.getByRole('dialog');
  await expect(modal).toBeVisible({ timeout: 15_000 });
  await modal.getByRole('button', { name: 'Ekspor' }).click();
  // The modal reaches its terminal (success) step — the "Unduh" download affordance appears
  // once the FE sees COMPLETED.
  await expect(modal.getByRole('button', { name: 'Unduh' })).toBeVisible({ timeout: 25_000 });
});

// ---------------------------------------------------------------------------
// CANCEL — POST /exports → :cancel → CANCELLED (retried across the worker race)
// ---------------------------------------------------------------------------

test('EXPORT-cancel · POST /exports → :cancel reaches CANCELLED', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await gotoReady(page, '/reports');

  // The cancel races the worker (jobs complete in ~0s). Retry create+cancel until the cancel
  // wins (status CANCELLED) — the contract test (11-03) pins the QUEUED→CANCELLED transition;
  // here we prove the real :cancel path drives a job to CANCELLED end-to-end.
  let cancelled = false;
  for (let attempt = 0; attempt < 5 && !cancelled; attempt++) {
    const created = await apiAs(page, 'POST', '/exports', BILLABLE_REQUEST);
    expect(created.status).toBe(202);
    const id = (created.body as { data: { id: string } }).data.id;

    const cancel = await apiAs(page, 'POST', `/exports/${id}:cancel`, undefined);
    expect(cancel.status, JSON.stringify(cancel.body)).toBe(200);
    const status = (cancel.body as { data: { status: string } }).data.status;

    if (status === 'CANCELLED') {
      // DB confirms the terminal CANCELLED state via the real cancel path.
      const row = await pollExportJobUntil(id, ['CANCELLED', 'DONE', 'FAILED'], {
        timeoutMs: 10_000,
      });
      expect(row.status).toBe('CANCELLED');
      cancelled = true;
    }
    // else: the worker won this race (COMPLETED) — retry to win the cancel race.
  }
  expect(cancelled, 'cancel never won the race in 5 attempts').toBe(true);
});

// ---------------------------------------------------------------------------
// PDF UNSUPPORTED — apiAs-level (the UI only offers Excel)
// ---------------------------------------------------------------------------

test('EXPORT-pdf-unsupported · POST /exports format=PDF → 422 EXPORT_FORMAT_UNSUPPORTED', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/reports');
  await waitForToken(page);

  const res = await apiAs(page, 'POST', '/exports', {
    report_type: 'ATTENDANCE_BILLABLE',
    format: 'PDF',
    filters: { period_start: '2026-06-01', period_end: '2026-06-30' },
  });
  expect(res.status).toBe(422);
  expect(errorCode(res.body)).toBe('EXPORT_FORMAT_UNSUPPORTED');
});
