/**
 * lib/e8-helpers.ts
 *
 * Shared UI/API helpers for the E8 payroll-archive E2E specs. Every selector is
 * anchored on the REAL rendered e8-payroll component DOM (NOT assumptions):
 *
 *   - payslip-archive-screen.tsx (/payroll, HR/super only): DataTable rows are
 *     `div.border-b` (filter by employee_name / employee_id / period text). The
 *     SearchField placeholder t('archive.searchPlaceholder') drives `employee_id`
 *     (exact match). FilterSelect aria t('archive.filterYear')/filterMonth/filterStatus.
 *     Per-row action button t('archive.viewDetail')="Lihat detail" → onRowClick(id).
 *     Status cell = StatusBadge (FINAL neutral, DECRYPT_FAIL warn). Confidentiality
 *     banner (lock) t('archive.confBanner'). The DEFAULT year filter = CURRENT_YEAR
 *     (2026) — but the seed periods are 2025-11/2025-12, so a fresh load shows EMPTY;
 *     the archive specs MUST select year 2025 (or clear the year) to see seeded rows.
 *   - payslip-detail-screen.tsx (/payroll/$payslipId): useGetPayslip(id) unwraps
 *     query.data?.data. Decrypt-fail → Banner tone bad t('detail.decryptFailTitle');
 *     money via formatMoney(null)="—"; DecryptPlaceholder for empty breakdown; the
 *     "Ekspor" button t('detail.export') is HIDDEN on decrypt-fail rows. InfoRow shows
 *     source `{system} #{source_id}` ("lumen_swp #44218" for SWP-PS-90121).
 *   - audit-note-drawer.tsx (PayslipDetailRoute opens it via "Tambah Catatan"):
 *     list reads query.data?.data; append form textarea #audit-note-text (RHF+Zod
 *     min 8 / max 1000, noValidate present), submit t('auditNotes.submit')="Simpan".
 *     NOTE: the FE Zod MIN is 8 chars — E2E note text MUST be ≥ 8 chars.
 *
 * Export: NO surface mounts PayrollExportButton (the detail "Ekspor" entry only fires a
 * QUEUED toast — payslip-detail-route.tsx). So the export E2E drives the export via
 * apiAs POST /payslips:export and asserts the REAL River worker completes the job via
 * pollExportJob (export_jobs.status → DONE) — there is no FE job-status hook in E8.
 *
 * Re-exports apiAs / errorCode / errorDetails / API_BASE / waitForToken from e5-helpers
 * (the same real-403 + token-hydration patterns) so e8 specs import from one lib.
 */

import { expect, type Locator, type Page } from '@playwright/test';
import * as path from 'node:path';
import * as fs from 'node:fs';
import pg from 'pg';
import { API_BASE, apiAs, errorCode, errorDetails, waitForToken } from './e5-helpers.js';

export { API_BASE, apiAs, errorCode, errorDetails, waitForToken };

const { Client } = pg;

// ---------------------------------------------------------------------------
// Seeded payslip fixture IDs (backend/cmd/seed/seed.go — seedPayroll, 10-02)
// ---------------------------------------------------------------------------

/** Payslip fixtures planted by the seed (10-02). Periods 2025-11 / 2025-12. */
export const PS = {
  /** Budi Santoso (EMP-1042), 2025-12, FINAL, full breakdown + benefits + source #44218. */
  final: 'SWP-PS-90121',
  /** Rudi Hartono (EMP-1108), 2025-12, FINAL. */
  final2: 'SWP-PS-90122',
  /** Andi Pratama (EMP-2891), 2025-11, FINAL. */
  final3: 'SWP-PS-90123',
  /** Dewi Lestari (EMP-3001), 2025-11, FINAL (extra volume). */
  final4: 'SWP-PS-90124',
  /** Rudi Hartono (EMP-1108), 2025-12, DECRYPT_FAIL (garbage ciphertext) + 2 audit notes. */
  decryptFail: 'SWP-PS-90119',
} as const;

/** Seeded audit notes on the DECRYPT_FAIL payslip (author Sari Hadi / SWP-EMP-9001). */
export const PS_NOTE = {
  one: 'SWP-PS-90119-NOTE-1',
  two: 'SWP-PS-90119-NOTE-2',
} as const;

/** Visible employee names rendered in the archive (anchor rows on these). */
export const PS_NAME = {
  budi: 'Budi Santoso', // SWP-PS-90121
  rudi: 'Rudi Hartono', // SWP-PS-90122 + SWP-PS-90119 (decrypt-fail)
  andi: 'Andi Pratama', // SWP-PS-90123
  dewi: 'Dewi Lestari', // SWP-PS-90124
} as const;

/** Employee ids referenced by the payslip fixtures. */
export const PS_EMP = {
  budi: 'SWP-EMP-1042',
  rudi: 'SWP-EMP-1108',
  andi: 'SWP-EMP-2891',
  dewi: 'SWP-EMP-3001',
} as const;

/** The author name stamped on the seeded notes (note.author_name). */
export const PS_NOTE_AUTHOR = 'Sari Hadi';

// ---------------------------------------------------------------------------
// Archive-table UI helpers (DataTable rows = div.border-b)
// ---------------------------------------------------------------------------

/** The archive DataTable row whose visible text contains `text` (employee / id / period). */
export function payslipRow(page: Page, text: string): Locator {
  return page.locator('div.border-b').filter({ hasText: text }).first();
}

/** Assert an archive row containing `text` is visible (list loaded). */
export async function expectPayslipRow(page: Page, text: string): Promise<void> {
  await expect(payslipRow(page, text)).toBeVisible({ timeout: 30_000 });
}

/** Assert NO archive row contains `text` (filtered out). */
export async function expectNoPayslipRow(page: Page, text: string): Promise<void> {
  await expect(page.locator('div.border-b').filter({ hasText: text })).toHaveCount(0, {
    timeout: 20_000,
  });
}

// ---------------------------------------------------------------------------
// export_jobs DB poll — the KEY helper proving the River worker completed the job.
// E8 has NO FE job-status hook (the generalized status surface is E10/Phase-11), so
// the export E2E observes completion by polling export_jobs directly. Mirrors the pg
// Client connection pattern in reset-db.ts.
// ---------------------------------------------------------------------------

const REPO_ROOT = path.resolve(import.meta.dirname, '../../..');
const ENV_FILE = path.join(REPO_ROOT, 'frontend', 'e2e', '.env.e2e');

function getDbUrl(): string {
  if (!fs.existsSync(ENV_FILE)) {
    throw new Error(`[e8-helpers] .env.e2e not found at ${ENV_FILE}`);
  }
  const lines = fs.readFileSync(ENV_FILE, 'utf8').split('\n');
  for (const line of lines) {
    const trimmed = line.trim();
    if (trimmed.startsWith('DATABASE_URL=')) {
      return trimmed.slice('DATABASE_URL='.length).trim();
    }
  }
  throw new Error('[e8-helpers] DATABASE_URL not found in .env.e2e');
}

export interface ExportJobRow {
  status: string;
  row_count: number | null;
  artifact_ref: string | null;
}

/**
 * pollExportJob — poll the export_jobs row for `jobId` every 250ms until the worker
 * flips it to a terminal status (DONE or FAILED) or the timeout elapses. Returns the
 * terminal row {status, row_count, artifact_ref}. This is how the export E2E proves the
 * REAL River worker processed the PayslipExportArgs job (no FE job-status hook in E8).
 */
export async function pollExportJob(
  jobId: string,
  { timeoutMs = 15_000, intervalMs = 250 }: { timeoutMs?: number; intervalMs?: number } = {},
): Promise<ExportJobRow> {
  const client = new Client({ connectionString: getDbUrl() });
  await client.connect();
  try {
    const deadline = Date.now() + timeoutMs;
    let last: ExportJobRow | undefined;
    while (Date.now() < deadline) {
      const res = await client.query(
        'SELECT status, row_count, artifact_ref FROM export_jobs WHERE id = $1',
        [jobId],
      );
      if (res.rows.length > 0) {
        last = res.rows[0] as ExportJobRow;
        if (last.status === 'DONE' || last.status === 'FAILED') {
          return last;
        }
      }
      await new Promise((r) => setTimeout(r, intervalMs));
    }
    throw new Error(
      `[e8-helpers] pollExportJob timed out after ${timeoutMs}ms for ${jobId} ` +
        `(last status: ${last?.status ?? 'NOT FOUND'})`,
    );
  } finally {
    await client.end();
  }
}
