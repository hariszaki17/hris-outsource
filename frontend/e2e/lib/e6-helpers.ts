/**
 * lib/e6-helpers.ts
 *
 * Shared UI/API helpers for the E6 leave-approval / quota / calendar E2E specs.
 * Every selector is anchored on the REAL rendered component DOM (NOT assumptions):
 *
 *   - leave-approvals-screen.tsx: DataTable rows are `div.border-b` (filter by the
 *     row's employee_name / leave-request id text). Per-row action is a Link "Tinjau"
 *     (t('approvals.review')) → /leave/$leaveRequestId. Default status filter: HR=PENDING_HR,
 *     SL=PENDING_L1. Status badge renders t(`status.${status}`).
 *   - leave-detail-screen.tsx: action buttons — "Setujui & Teruskan" (SL L1 forward,
 *     t('detail.actionForward')), "Setujui" (HR final, t('detail.actionApprove')),
 *     "Tetap setujui (override)" (over-balance, t('detail.actionOverride')), "Tolak"
 *     (t('detail.actionReject')). RejectLeaveModal: textarea #reject-reason (min 5),
 *     confirm "Tolak permintaan" (t('reject.confirm')). BalanceChangedModal: textarea
 *     #override-reason (min 10), confirm "Tetap setujui (override)" (t('override.confirm')).
 *     schedule_impact section title "Dampak Jadwal" (t('detail.sectionScheduleImpact'));
 *     each affected entry renders its new_status RAW (the E6 contract value 'LEAVE').
 *     no-leader badge "Tanpa pemimpin shift" (t('detail.noLeaderBadge')).
 *   - leave-quotas-screen.tsx (/leave/quotas): table cols employee/leave_type/total/used/
 *     pending/remaining; per-row "Sesuaikan" button aria "Sesuaikan kuota {name}"
 *     (t('actions.adjustAriaLabel')). AdjustQuotaModal: #delta (number), #reason (textarea),
 *     save "Simpan Penyesuaian" (t('adjust.saveBtn')). Bulk-grant: header "Terbitkan Kuota
 *     Tahunan" (t('actions.bulkGrant')) → form → "Pratinjau" (t('bulkGrant.previewBtn')) →
 *     "Terbitkan" (t('bulkGrant.applyBtn')).
 *   - leave-calendar-screen.tsx (/leave/calendar): month nav buttons aria "Bulan
 *     sebelumnya"/"Bulan berikutnya"; show_pending Toggle role=switch aria "Tampilkan entri
 *     cuti pending" (t('showPendingAria')); DayCell renders Avatar + employee_name +
 *     StatusBadge(leave_type_code).
 *
 * Re-exports apiAs / errorCode / errorDetails / API_BASE / waitForToken from e5-helpers
 * (same real-409 + token-hydration patterns) so e6 specs import from one lib.
 */

import { expect, type Locator, type Page } from '@playwright/test';
import { API_BASE, apiAs, errorCode, errorDetails, waitForToken } from './e5-helpers.js';

export { API_BASE, apiAs, errorCode, errorDetails, waitForToken };

// ---------------------------------------------------------------------------
// Seeded leave-request / quota fixture IDs (backend/cmd/seed/seed.go — seedLeave)
// ---------------------------------------------------------------------------

/** Leave-request fixtures planted by the seed (08-02). */
export const LR = {
  /** Dewi @ CMP-0021, PENDING_L1, monday+4 (Fri). Rudi L1-forward target. */
  l1Target: 'SWP-LR-8001',
  /** Dewi @ CMP-0021, PENDING_HR (leader-approved). HR final-approve target. */
  finalTarget: 'SWP-LR-8002',
  /** Budi @ CMP-0022, PENDING_HR, 3 days vs remaining 1, no_leader.
   *  BALANCE_RECHECK / override target. */
  overrideTarget: 'SWP-LR-8003',
  /** Budi @ CMP-0022, PENDING_L1. Rudi cross-company OUT_OF_SCOPE target. */
  outOfScope: 'SWP-LR-8004',
  /** Dewi @ CMP-0021, APPROVED terminal (list filter + calendar). */
  approved: 'SWP-LR-8005',
  /** Dewi @ CMP-0021, REJECTED terminal (list filter). */
  rejected: 'SWP-LR-8006',
  /** Dewi @ CMP-0021, PENDING_HR, start=end=monday+2 (Wed) OVERLAPPING SWP-SCH-6002.
   *  The INV-3 loop-closer target. */
  inv3Overlap: 'SWP-LR-8007',
} as const;

/** Leave-quota fixtures planted by the seed (08-02). */
export const LQ = {
  /** Dewi (SWP-EMP-3001): total 12, used 4 → remaining 8. Adjust happy target. */
  dewi: 'SWP-LQ-8001',
  /** Budi (SWP-EMP-2891): total 12, used 11 → remaining 1. Over-balance / refuse target. */
  budi: 'SWP-LQ-8002',
} as const;

/** Employee IDs referenced by the leave fixtures. */
export const EMP = {
  /** Dewi Lestari — the INV-3 / approval agent (SWP-SCH-6002 + SWP-LR-8007). */
  dewi: 'SWP-EMP-3001',
  /** Budi — the over-balance agent at CMP-0022. */
  budi: 'SWP-EMP-2891',
} as const;

/** The E4 schedule entry SWP-LR-8007 overlaps (Dewi on monday+2 / Wed). */
export const SCH = {
  inv3Entry: 'SWP-SCH-6002',
} as const;

// ---------------------------------------------------------------------------
// Asia/Jakarta-anchored week dates (mirror the seed's mondayOfCurrentWeek which
// uses the UTC calendar date — see backend/cmd/seed/seed.go). The seed plants:
//   monday+2 (Wed) SWP-SCH-6002 Dewi + SWP-LR-8007 (the INV-3 overlap)
//   monday+3 (Thu) SWP-LR-8002 Dewi (PENDING_HR) + approved_leave_days SWP-LR-44210
//   monday+4 (Fri) SWP-LR-8001 Dewi (PENDING_L1)
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

// ---------------------------------------------------------------------------
// Approvals-queue UI helpers (DataTable rows = div.border-b)
// ---------------------------------------------------------------------------

/** The approvals-queue DataTable row whose visible text contains `idOrName`. */
export function leaveRow(page: Page, idOrName: string): Locator {
  return page.locator('div.border-b').filter({ hasText: idOrName }).first();
}

/** Assert an approvals-queue row containing `idOrName` is visible (queue loaded). */
export async function expectLeaveRow(page: Page, idOrName: string): Promise<void> {
  await expect(leaveRow(page, idOrName)).toBeVisible({ timeout: 30_000 });
}

/** Assert NO approvals-queue row contains `idOrName` (scoped-out / filtered). */
export async function expectNoLeaveRow(page: Page, idOrName: string): Promise<void> {
  await expect(page.locator('div.border-b').filter({ hasText: idOrName })).toHaveCount(0, {
    timeout: 20_000,
  });
}

/**
 * openLeaveDetail — navigate straight to the leave-detail route and wait for the
 * in-memory token to re-hydrate (post-goto 401 race) + the detail header to render.
 */
export async function openLeaveDetail(page: Page, id: string): Promise<void> {
  await page.goto(`/leave/${id}`);
  await waitForToken(page);
  // The header card renders the leave-request id chip once the detail GET resolves.
  await expect(page.getByText(id).first()).toBeVisible({ timeout: 30_000 });
}

// ---------------------------------------------------------------------------
// Detail-screen action button labels (exact — both forward & approve start
// with "Setujui", so callers MUST use exact:true).
// ---------------------------------------------------------------------------

export const BTN = {
  forward: 'Setujui & Teruskan',
  approve: 'Setujui',
  override: 'Tetap setujui (override)',
  reject: 'Tolak',
  rejectConfirm: 'Tolak permintaan',
} as const;

// ---------------------------------------------------------------------------
// Persisted-state assertion helpers (read the real API after a UI action)
// ---------------------------------------------------------------------------

/** GET a leave request and return its persisted `status` (via the in-memory token). */
export async function leaveStatus(page: Page, id: string): Promise<string | undefined> {
  const res = await apiAs(page, 'GET', `/leave-requests/${id}`);
  const rec = (res.body as { data?: { status?: string } } | null)?.data;
  return rec?.status;
}

/** Poll until a leave request reaches `expected` status (after a refetch-driven UI action). */
export async function expectLeaveStatus(
  page: Page,
  id: string,
  expected: string,
): Promise<void> {
  await expect.poll(() => leaveStatus(page, id), { timeout: 20_000 }).toBe(expected);
}

/**
 * quotaRemaining — GET /leave-quotas and return the persisted `remaining` for `id`.
 * apiAs() returns the RAW BE body (not the Orval `{data}` wrap), so the page envelope
 * is the top-level `{ data: [quotas], next_cursor, has_more }` (httpx.PageResponse).
 */
export async function quotaRemaining(page: Page, id: string): Promise<number | undefined> {
  const res = await apiAs(page, 'GET', '/leave-quotas?limit=50');
  const body = res.body as { data?: Array<{ id: string; remaining: number }> } | null;
  const rows = body?.data ?? [];
  return rows.find((q) => q.id === id)?.remaining;
}

// ---------------------------------------------------------------------------
// INV-3 over-leave probe — POST /schedule (create) for an agent on a date that
// now carries an approved_leave_days row → 409 SHIFT_OVER_LEAVE. Mirrors the
// proven E4 conflict-negatives pattern (top-level error.code + error.details).
// SWP-SHF-001 ("Pagi") is valid for Dewi's SVC-003 (Parking) placement.
// ---------------------------------------------------------------------------

export interface OverLeaveProbe {
  status: number;
  code: string | undefined;
  details: Record<string, unknown> | undefined;
}

/**
 * scheduleCheckOverLeave — attempt a schedule create for `employeeId` on `dateIso`
 * and surface the conflict code + details. After SWP-LR-8007 is approved, this
 * returns 409 SHIFT_OVER_LEAVE with details.leave_request_id === 'SWP-LR-8007'
 * (the REAL approved_leave_days row the approval inserted, not the Phase-6 fixture).
 */
export async function scheduleCheckOverLeave(
  page: Page,
  employeeId: string,
  dateIso: string,
  shiftMasterId = 'SWP-SHF-001',
): Promise<OverLeaveProbe> {
  const res = await apiAs(page, 'POST', '/schedule', {
    kind: 'single',
    employee_id: employeeId,
    shift_master_id: shiftMasterId,
    date: dateIso,
    is_day_off: false,
    force_replace: false,
  });
  return {
    status: res.status,
    code: errorCode(res.body),
    details: errorDetails(res.body),
  };
}

/**
 * scheduleEntryStatus — GET /schedule for a company over [startIso..endIso] and
 * return the ScheduleEntry matching `scheduleId`. The list response is the
 * top-level `{ data: [entries], warnings: [] }` envelope; required query params
 * are `company_id` + `start_date` + `end_date` (schedule_handler.go).
 */
export async function scheduleEntryStatus(
  page: Page,
  companyId: string,
  startIso: string,
  endIso: string,
  scheduleId: string,
): Promise<string | undefined> {
  const res = await apiAs(
    page,
    'GET',
    `/schedule?company_id=${companyId}&start_date=${startIso}&end_date=${endIso}`,
  );
  const body = res.body as { data?: Array<{ id: string; status: string }> } | null;
  const entries = body?.data ?? [];
  return entries.find((e) => e.id === scheduleId)?.status;
}
