/**
 * lib/e10-helpers.ts
 *
 * Shared UI/API helpers for the E10 reporting / dashboard / notifications / export
 * E2E specs (Phase-11, the milestone capstone). Every selector is anchored on the
 * REAL rendered e10-reporting + dashboard component DOM (NOT assumptions):
 *
 *   - dashboard-screen.tsx (/, role-branched): HrDashboardView renders a TitleBand
 *     <h1>t('title')="Dashboard"</h1> + the `data.role_label` span ("HR Admin" /
 *     "Super Admin"), 4× StatCard (KPI labels), and an ApprovalInboxPanel (right,
 *     w-392) whose rows deep-link into E5/E6/E7. LeaderDashboardView renders the SAME
 *     <h1> + a subtitle "{company.name} · …", 4× today StatCard (total/clocked-in/
 *     late/absent), pending-count chips, schedule-alerts, and the ApprovalInboxPanel.
 *     Agent → AgentFallback (no-permission EmptyState).
 *   - billable-report-screen.tsx (/reports): TitleBand <h1>t('report.title')</h1> +
 *     "Ekspor" primary button (t('report.exportBtn')); period date inputs aria-label
 *     t('report.filterPeriodFrom')/filterPeriodTo; 4× StatCard (Jam Billable/Payable/
 *     Worked/Tingkat Verifikasi); DataTable rows = div.border-b keyed on group_key;
 *     pending Banner (warn) when pending_summary.pending_records > 0. The ExportModal
 *     (FJ6hX) opens off "Ekspor" via useExportFlow.
 *   - notifications-screen.tsx (/notifications): TitleBand <h1>t('title')</h1> +
 *     "Tandai semua dibaca" Button (hidden when unreadCount===0); read-state pill
 *     buttons (Semua/Belum dibaca/Sudah dibaca); kind FilterSelect; NotifCard rows
 *     grouped under date <section aria-label> (HARI INI/KEMARIN). Each card shows an
 *     unread dot when read_at===null; clicking marks read + navigates the deep_link.
 *
 * Reuses apiAs / PERSONAS / API_BASE / waitForToken / errorCode and the export_jobs
 * DB-poll (pollExportJob) from the Phase-10 e8-helpers so the export + capstone specs
 * import from one lib (.js extensions per the E2E convention). Adds:
 *   - pollExportJobUntil(): like pollExportJob but accepts a custom terminal-status set
 *     (the cancel test needs CANCELLED as terminal, not just DONE/FAILED).
 *   - pollNotification(): polls GET /notifications via apiAs until a row matching a
 *     predicate (kind + deep-link entity) appears for the logged-in recipient — proves
 *     the REAL auto-dispatched notification landed (worker-driven, not seeded).
 *   - LR / NTF seeded fixture ids used by the capstone.
 */

import { expect, type Locator, type Page } from '@playwright/test';
import * as path from 'node:path';
import * as fs from 'node:fs';
import pg from 'pg';
import { API_BASE, apiAs, errorCode, waitForToken } from './e5-helpers.js';
import { pollExportJob, type ExportJobRow } from './e8-helpers.js';
import { PERSONAS, type Persona } from './personas.js';

export { API_BASE, apiAs, errorCode, waitForToken, pollExportJob, PERSONAS };
export type { ExportJobRow, Persona };

const { Client } = pg;

// ---------------------------------------------------------------------------
// Seeded fixture ids the capstone drives (backend/cmd/seed/seed.go)
// ---------------------------------------------------------------------------

/** Leave-request fixtures relevant to the auto-dispatch capstone (seedLeave, 08-02). */
export const LR = {
  /**
   * Dewi Lestari (SWP-EMP-3001) @ CMP-0021 — PENDING_HR (leader-approved L1 row present).
   * HR `:approve-final` → LEAVE_APPROVED notification dispatched to the submitter (Dewi).
   * This is the capstone's REAL action (not a seeded notification).
   */
  dewiPendingHr: 'SWP-LR-8002',
} as const;

/** The submitter persona who RECEIVES the LEAVE_APPROVED notification (SWP-EMP-3001). */
export const DEWI: Persona = {
  email: 'dewi.lestari@swp.test',
  password: 'Dew1-Lestari-2026!',
  role: 'agent',
};

/** Seeded notification fixture ids (seedNotifications, 11-02). */
export const NTF = {
  /** HR Sari — LEAVE_REQUEST_SUBMITTED, unread, critical. */
  hrLeaveSubmitted: 'SWP-NTF-90001',
  /** HR Sari — ATTENDANCE_VERIFY_NEEDED, READ. */
  hrAttVerify: 'SWP-NTF-90002',
  /** Agent Budi — LEAVE_APPROVED, unread, critical. */
  agentLeaveApproved: 'SWP-NTF-90004',
} as const;

// ---------------------------------------------------------------------------
// DB url (mirror reset-db.ts / e8-helpers.ts)
// ---------------------------------------------------------------------------

const REPO_ROOT = path.resolve(import.meta.dirname, '../../..');
const ENV_FILE = path.join(REPO_ROOT, 'frontend', 'e2e', '.env.e2e');

function getDbUrl(): string {
  if (!fs.existsSync(ENV_FILE)) {
    throw new Error(`[e10-helpers] .env.e2e not found at ${ENV_FILE}`);
  }
  for (const line of fs.readFileSync(ENV_FILE, 'utf8').split('\n')) {
    const trimmed = line.trim();
    if (trimmed.startsWith('DATABASE_URL=')) {
      return trimmed.slice('DATABASE_URL='.length).trim();
    }
  }
  throw new Error('[e10-helpers] DATABASE_URL not found in .env.e2e');
}

// ---------------------------------------------------------------------------
// pollExportJobUntil — like pollExportJob but with a configurable terminal set so
// the :cancel test can wait for CANCELLED (the e8 pollExportJob only stops on
// DONE/FAILED). Returns the terminal export_jobs row.
// ---------------------------------------------------------------------------

export async function pollExportJobUntil(
  jobId: string,
  terminal: string[],
  { timeoutMs = 20_000, intervalMs = 250 }: { timeoutMs?: number; intervalMs?: number } = {},
): Promise<ExportJobRow & { status: string }> {
  const client = new Client({ connectionString: getDbUrl() });
  await client.connect();
  try {
    const deadline = Date.now() + timeoutMs;
    let last: (ExportJobRow & { status: string }) | undefined;
    while (Date.now() < deadline) {
      const res = await client.query(
        'SELECT status, row_count, artifact_ref FROM export_jobs WHERE id = $1',
        [jobId],
      );
      if (res.rows.length > 0) {
        last = res.rows[0] as ExportJobRow & { status: string };
        if (terminal.includes(last.status)) return last;
      }
      await new Promise((r) => setTimeout(r, intervalMs));
    }
    throw new Error(
      `[e10-helpers] pollExportJobUntil timed out after ${timeoutMs}ms for ${jobId} ` +
        `(wanted ${terminal.join('/')}, last: ${last?.status ?? 'NOT FOUND'})`,
    );
  } finally {
    await client.end();
  }
}

// ---------------------------------------------------------------------------
// Notification helpers (drive GET /notifications via apiAs as the recipient)
// ---------------------------------------------------------------------------

export interface NotificationDto {
  id: string;
  kind: string;
  title: string;
  body: string;
  read_at: string | null;
  deep_link?: { epic?: string; entity_id?: string; path?: string } | null;
  created_at: string;
}

/** The cursor-envelope page shape returned by GET /notifications under {data}. */
interface NotifPage {
  data?: NotificationDto[];
  has_more?: boolean;
  next_cursor?: string | null;
}

/** List the logged-in principal's notifications via apiAs (scope=self). */
export async function listNotificationsVia(
  page: Page,
  query = '?read_state=ALL&limit=50',
): Promise<NotificationDto[]> {
  const res = await apiAs(page, 'GET', `/notifications${query}`);
  expect(res.status, `GET /notifications → ${res.status}: ${JSON.stringify(res.body)}`).toBe(200);
  const page0 = (res.body as { data?: NotifPage })?.data;
  return page0?.data ?? [];
}

/**
 * pollNotification — poll GET /notifications (as the already-logged-in recipient) until
 * a notification matching `predicate` appears OR timeout. Proves the REAL auto-dispatched
 * row landed via the un-stubbed River worker (the dispatch is async — give it a bounded
 * poll, like pollExportJob). The page MUST already be logged in (call waitForToken first).
 */
export async function pollNotification(
  page: Page,
  predicate: (n: NotificationDto) => boolean,
  { timeoutMs = 20_000, intervalMs = 400 }: { timeoutMs?: number; intervalMs?: number } = {},
): Promise<NotificationDto> {
  const deadline = Date.now() + timeoutMs;
  let lastCount = -1;
  while (Date.now() < deadline) {
    const rows = await listNotificationsVia(page);
    lastCount = rows.length;
    const hit = rows.find(predicate);
    if (hit) return hit;
    await new Promise((r) => setTimeout(r, intervalMs));
  }
  throw new Error(
    `[e10-helpers] pollNotification timed out after ${timeoutMs}ms ` +
      `(no matching notification; last list size: ${lastCount})`,
  );
}

// ---------------------------------------------------------------------------
// Notification-center UI helpers (NotifCard rows; date <section aria-label> groups)
// ---------------------------------------------------------------------------

/** A NotifCard whose visible text contains `text` (the notification title/body). */
export function notifCard(page: Page, text: string): Locator {
  return page.getByText(text, { exact: false }).first();
}

/** Assert a NotifCard containing `text` is visible (list rendered). */
export async function expectNotifCard(page: Page, text: string): Promise<void> {
  await expect(notifCard(page, text)).toBeVisible({ timeout: 30_000 });
}

// ---------------------------------------------------------------------------
// Billable-report / dashboard table helpers (DataTable rows = div.border-b)
// ---------------------------------------------------------------------------

/** The billable-report DataTable row whose visible text contains `text` (group_key/label). */
export function reportRow(page: Page, text: string): Locator {
  return page.locator('div.border-b').filter({ hasText: text }).first();
}
