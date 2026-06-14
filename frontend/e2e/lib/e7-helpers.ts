/**
 * lib/e7-helpers.ts
 *
 * Shared UI/API helpers for the E7 overtime-approval / holiday-calendar E2E specs.
 * Every selector is anchored on the REAL rendered component DOM (NOT assumptions):
 *
 *   - overtime-approvals-screen.tsx (/overtime, role-branched HR/SL):
 *     DataTable rows are `div.border-b` (filter by the row's employee_name /
 *     company text, or the OT work_date). Default status: HR=PENDING_HR, SL=PENDING_L1.
 *     Per-row action buttons text = t('approvals.approve')="Setujui" /
 *     t('approvals.reject')="Tolak"; detail link "Lihat" (t('approvals.detail')).
 *     Row checkbox aria = t('approvals.selectRow',{id}) = "Pilih lembur {{id}}".
 *     Header "Setujui Massal" (t('approvals.bulkApprove')); bulk-reject button appears
 *     when selection>0. flagged_no_preapproval pill (StatusBadge warn).
 *   - overtime-detail-screen.tsx (/overtime/{id}): staff action buttons —
 *     "Setujui (L1)" (t('detail.actionApproveL1')), "Setujui (akhir)"
 *     (t('detail.actionApproveFinal')), "Tolak" (t('detail.actionReject')).
 *     The reject modal (overtime-detail-overlays.tsx) uses textarea #ot-reject-reason
 *     + confirm "Tolak lembur" (t('detail.rejectConfirmBtn')). Confirm/withdraw buttons
 *     render only for the AGENT role (mobile/self — out of web scope); the web confirm
 *     + withdraw flows are driven via apiAs (HR/leader pass the :confirm route guard).
 *     Tier-breakdown card (t('detail.sectionTierBreakdown')) + approvals timeline
 *     (t('detail.sectionTimeline')). The detail GET unwraps the {data} envelope
 *     (raw && 'id' in raw && 'status' in raw guard — already present).
 *   - overtime-rules-screen.tsx (/overtime/aturan, HR only): right pane
 *     "Kalender Hari Libur"; holidays render as <li> filtered by name. Add "+" button
 *     aria t('holidays.addTitle'); per-row edit Pencil aria t('holidays.editTitle');
 *     delete Trash2 aria t('holidays.deleteTitle').
 *   - holiday-overlays.tsx: HolidayFormModal fields #holiday-name / #holiday-date
 *     (type=date) / #holiday-category (FilterSelect) + recurring Toggle (role=switch);
 *     save "Simpan" (t('common.save')). DeleteHolidayConfirm: confirm disabled when
 *     in_use_by_overtime → Banner warn (t('holidays.inUseWarning')); conflict toast
 *     t('holidays.inUseError').
 *
 * Re-exports apiAs / errorCode / errorDetails / API_BASE / waitForToken from e5-helpers
 * (the same real-403/409 + token-hydration patterns) so e7 specs import from one lib.
 */

import { expect, type Locator, type Page } from '@playwright/test';
import { API_BASE, apiAs, errorCode, errorDetails, waitForToken } from './e5-helpers.js';

export { API_BASE, apiAs, errorCode, errorDetails, waitForToken };

// ---------------------------------------------------------------------------
// Seeded overtime fixture IDs (backend/cmd/seed/seed.go — seedOvertime, 09-02)
// ---------------------------------------------------------------------------

/** Overtime fixtures planted by the seed (09-02). All Asia/Jakarta-safe dates. */
export const OT = {
  /** Dewi @ CMP-0021, PENDING_AGENT_CONFIRM, AUTO_DETECTED. Confirm target. */
  confirmTarget: 'SWP-OT-30001',
  /** Dewi @ CMP-0021, PENDING_L1, WORKDAY. Rudi leader L1 target. */
  l1Target: 'SWP-OT-30002',
  /** Dewi @ CMP-0021, PENDING_HR. HR final target. */
  finalTarget: 'SWP-OT-30003',
  /** Rudi's OWN record (EMP-1108 @ CMP-0021), PENDING_L1. SELF_APPROVAL_FORBIDDEN. */
  self: 'SWP-OT-30004',
  /** Budi @ CMP-0022, PENDING_L1. OUT_OF_SCOPE for Rudi. */
  outOfScope: 'SWP-OT-30005',
  /** Dewi @ CMP-0021, PENDING_L1, counted 0 / skipped_too_short. OT_BELOW_MIN. */
  belowMin: 'SWP-OT-30006',
  /** Dewi @ CMP-0021, APPROVED terminal (list filter + bulk terminal-409). */
  approved: 'SWP-OT-30007',
  /** Dewi @ CMP-0021, REJECTED terminal (list filter). */
  rejected: 'SWP-OT-30008',
  /** Dewi @ CMP-0021, HOLIDAY APPROVED referencing SWP-HOL-9001 (HOLIDAY_IN_USE source). */
  holidayOt: 'SWP-OT-30009',
  /** Dewi @ CMP-0021, RESTDAY, PENDING_L1. */
  restday: 'SWP-OT-30010',
} as const;

/** Holiday fixtures planted by the seed (09-02). Both NATIONAL, in-range. */
export const HOL = {
  /** Referenced by SWP-OT-30009 (APPROVED) → in_use, delete blocked (HOLIDAY_IN_USE). */
  inUse: 'SWP-HOL-9001',
  /** Free, deletable. Update / delete-free target. */
  free: 'SWP-HOL-9002',
} as const;

/** Employee + company IDs referenced by the OT fixtures. */
export const E7_EMP = {
  /** Dewi Lestari @ CMP-0021 — the OT subject for most records. */
  dewi: 'SWP-EMP-3001',
  /** Rudi Wijaya — the leader, his OWN OT (SWP-OT-30004) is SELF_APPROVAL_FORBIDDEN. */
  rudi: 'SWP-EMP-1108',
  /** Budi Santoso @ CMP-0022 — the cross-company OT subject. */
  budi: 'SWP-EMP-2891',
} as const;

export const E7_CMP = {
  /** Plaza Senayan — Rudi's scope. */
  own: 'SWP-CMP-0021',
  /** Mall Kelapa Gading — cross-company (out of Rudi's scope). */
  other: 'SWP-CMP-0022',
} as const;

/** Visible employee names rendered in the approvals queue (anchor rows on these). */
export const E7_NAME = {
  dewi: 'Dewi Lestari',
  rudi: 'Rudi Wijaya',
  budi: 'Budi Santoso',
} as const;

// ---------------------------------------------------------------------------
// Asia/Jakarta-anchored week dates (mirror the seed's mondayOfCurrentWeek which
// uses the UTC calendar date — see backend/cmd/seed/seed.go). The seed plants:
//   monday-14  SWP-HOL-9001 (in-use holiday) + SWP-OT-30009 work_date
//   monday+21  SWP-HOL-9002 (free holiday)
// ---------------------------------------------------------------------------

/** Monday (UTC calendar date) of the current week, "YYYY-MM-DD" — matches the seed. */
export function mondayIso(): string {
  const now = new Date();
  const d = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()));
  const offset = (d.getUTCDay() + 6) % 7; // Go: (Weekday()+6)%7 — ISO Monday-start
  d.setUTCDate(d.getUTCDate() - offset);
  return d.toISOString().slice(0, 10);
}

/** Monday + N days, "YYYY-MM-DD" (UTC-safe) — matches the seed's date math. */
export function mondayPlus(n: number): string {
  const [y, m, dd] = mondayIso().split('-').map(Number);
  const d = new Date(Date.UTC(y, m - 1, dd));
  d.setUTCDate(d.getUTCDate() + n);
  return d.toISOString().slice(0, 10);
}

/** SWP-HOL-9001's seeded date (monday-14) — the clash target for a duplicate create. */
export function inUseHolidayDate(): string {
  return mondayPlus(-14);
}

// ---------------------------------------------------------------------------
// Approvals-queue UI helpers (DataTable rows = div.border-b)
// ---------------------------------------------------------------------------

/** The approvals-queue DataTable row whose visible text contains `text`. */
export function otRow(page: Page, text: string): Locator {
  return page.locator('div.border-b').filter({ hasText: text }).first();
}

/** Assert an approvals-queue row containing `text` is visible (queue loaded). */
export async function expectOtRow(page: Page, text: string): Promise<void> {
  await expect(otRow(page, text)).toBeVisible({ timeout: 30_000 });
}

/** Assert NO approvals-queue row contains `text` (scoped-out / filtered). */
export async function expectNoOtRow(page: Page, text: string): Promise<void> {
  await expect(page.locator('div.border-b').filter({ hasText: text })).toHaveCount(0, {
    timeout: 20_000,
  });
}

/**
 * openOvertimeDetail — navigate straight to the overtime-detail route and wait for
 * the in-memory token to re-hydrate (post-goto 401 race) + the detail header to render
 * (the OT id chip appears once the detail GET resolves).
 */
export async function openOvertimeDetail(page: Page, id: string): Promise<void> {
  await page.goto(`/overtime/${id}`);
  await waitForToken(page);
  await expect(page.getByText(id).first()).toBeVisible({ timeout: 30_000 });
}

// ---------------------------------------------------------------------------
// Detail-screen action button labels (exact). Several start with "Setujui", so
// callers MUST use exact:true. Queue-row "Setujui"/"Tolak"/"Detail" are separate.
// ---------------------------------------------------------------------------

export const OT_BTN = {
  /** Detail screen: SL L1 approve (t('detail.actionApproveL1')). */
  approveL1: 'Setujui (L1)',
  /** Detail screen: HR final approve (t('detail.actionApproveFinal')). */
  approveFinal: 'Setujui (akhir)',
  /** Detail + queue: open reject (t('detail.actionReject') / t('approvals.reject')). */
  reject: 'Tolak',
  /** Detail reject-modal confirm (t('detail.rejectConfirmBtn')). */
  rejectConfirm: 'Tolak lembur',
  /** Queue row: per-row approve (t('approvals.approve')). */
  rowApprove: 'Setujui',
  /** Queue row: detail link (t('approvals.detail')). */
  rowDetail: 'Detail',
  /** Queue header: bulk-approve (t('approvals.bulkApprove')). */
  bulkApprove: 'Setujui massal',
  /** Bulk-approve modal confirm (t('bulkApprove.confirm')). */
  bulkApproveConfirm: 'Setujui',
  /** Bulk-reject modal confirm (t('bulkReject.confirm')). */
  bulkRejectConfirm: 'Tolak',
  /** Holiday form save (t('common.save')). */
  save: 'Simpan',
} as const;

/** The detail reject-modal reason textarea id (overtime-detail-overlays.tsx). */
export const OT_REJECT_REASON_ID = '#ot-reject-reason';
/** The bulk-approve note textarea id (overtime-queue-overlays.tsx). */
export const OT_BULK_NOTE_ID = '#bulk-approve-note';
/** The bulk-reject reason textarea id (overtime-queue-overlays.tsx). */
export const OT_BULK_REJECT_REASON_ID = '#bulk-reject-reason';

// ---------------------------------------------------------------------------
// Persisted-state assertion helpers (read the real API after a UI action)
// ---------------------------------------------------------------------------

/** GET an overtime record and return its persisted `status` (via the in-memory token). */
export async function overtimeStatus(page: Page, id: string): Promise<string | undefined> {
  const res = await apiAs(page, 'GET', `/overtime/${id}`);
  const rec = (res.body as { data?: { status?: string } } | null)?.data;
  return rec?.status;
}

/** Poll until an overtime record reaches `expected` status (after a refetch-driven UI action). */
export async function expectOvertimeStatus(
  page: Page,
  id: string,
  expected: string,
): Promise<void> {
  await expect.poll(() => overtimeStatus(page, id), { timeout: 20_000 }).toBe(expected);
}

/** GET an overtime record and return its recorded approval trail (level + decision). */
export async function overtimeApprovals(
  page: Page,
  id: string,
): Promise<Array<{ level: number; decision: string }>> {
  const res = await apiAs(page, 'GET', `/overtime/${id}`);
  const rec = (res.body as { data?: { approvals?: Array<{ level: number; decision: string }> } } | null)
    ?.data;
  return rec?.approvals ?? [];
}

// ---------------------------------------------------------------------------
// Bulk-envelope helpers (BulkResult {succeeded, failed})
// ---------------------------------------------------------------------------

export interface BulkBody {
  succeeded?: string[];
  failed?: Array<{ id?: string; error?: { code?: string; message?: string } }>;
}

/** Coerce a response body to the BulkResult shape. */
export function bulk(body: unknown): BulkBody {
  return (body as BulkBody) ?? {};
}

/** The error.code recorded against `id` in a BulkResult.failed[] entry. */
export function bulkFailedCode(body: unknown, id: string): string | undefined {
  return bulk(body).failed?.find((f) => f.id === id)?.error?.code;
}

// ---------------------------------------------------------------------------
// Holiday-calendar UI helpers (/overtime/aturan, right pane <li> rows)
// ---------------------------------------------------------------------------

/** The holiday list item whose visible text contains `name`. */
export function holidayRow(page: Page, name: string): Locator {
  return page.locator('li').filter({ hasText: name }).first();
}

/** GET /holidays and return the holiday matching `id` (or undefined). */
export async function getHoliday(
  page: Page,
  id: string,
): Promise<{ id: string; name: string; in_use_by_overtime?: boolean } | undefined> {
  const res = await apiAs(page, 'GET', '/holidays?limit=200');
  const body = res.body as { data?: Array<{ id: string; name: string; in_use_by_overtime?: boolean }> } | null;
  return (body?.data ?? []).find((h) => h.id === id);
}
