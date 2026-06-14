/**
 * lib/e5-helpers.ts
 *
 * Shared UI/API helpers for the E5 attendance-verification & corrections E2E specs.
 * Every selector is anchored on the REAL rendered component DOM (NOT assumptions):
 *
 *   - attendance-verification-screen.tsx: DataTable rows are `div.border-b` (filter by
 *     the row's employee_id / employee_name text, scoped to the table). The select-all
 *     Checkbox aria-label = t('selectAll') = "Pilih semua"; per-row Checkbox aria-label =
 *     t('selectRow',{name}) = "Pilih {{name}}". Per-row action buttons text = t('verifyBtn')
 *     /t('rejectBtn') = "Verifikasi"/"Tolak". BulkBar appears when ≥1 selected with
 *     t('bulkVerifyBtn')/t('bulkRejectBtn'). ConfirmDialog confirm buttons = t('bulkVerifyConfirm')
 *     etc. Reject reason inputs: #bulk-reject-reason / #single-reject-reason (min 5 chars).
 *     SL sees the scope banner (t('scopeBanner')) + the company filter is hidden.
 *   - attendance-detail-screen.tsx: verify button text = t('verifyBtn'), reject = t('rejectBtn');
 *     reject reason Input id="detail-reject-reason"; own-escalated record hides the action
 *     buttons and shows t('escalatedToHR') = "Dieskalasi ke HR".
 *   - corrections-screen.tsx: DataTable rows clickable → CorrectionDetailDrawer; footer
 *     Approve button text = t('corrections.approve') = "Setujui"; Reject opens
 *     RejectCorrectionModal (textarea register('reason'), id="reject-reason").
 *
 * Re-exports apiAs / errorCode / errorDetails / API_BASE from e3-helpers and waitForToken
 * from e4-helpers (the same real-409 + token-hydration patterns) so e5 specs import from
 * one lib. Adds apiAsWithKey() for the idempotency-replay test (fixed Idempotency-Key).
 */

import { expect, type Locator, type Page } from '@playwright/test';
import { API_BASE, apiAs, errorCode, errorDetails } from './e3-helpers.js';
import { waitForToken } from './e4-helpers.js';

export { API_BASE, apiAs, errorCode, errorDetails, waitForToken };

// ---------------------------------------------------------------------------
// Seeded fixture IDs (backend/cmd/seed/seed.go — seedAttendance + seedCorrections)
// ---------------------------------------------------------------------------

/** Attendance fixtures planted by the seed (07-02). */
export const ATT = {
  /** Dewi @ CMP-0021 — clean AUTO_APPROVED (NOT in the verification queue). */
  autoApproved: 'SWP-ATT-9001',
  /** Dewi @ CMP-0021 — PENDING, flags={LATE}, 18m late. Correction CHECK_IN target. */
  lateCmp21: 'SWP-ATT-9002',
  /** Sari @ CMP-0021 — PENDING, flags={OUTSIDE_GEOFENCE}. */
  geoCmp21: 'SWP-ATT-9003',
  /** Dewi @ CMP-0021 — PENDING, flags={AUTO_CLOSED}, no clock-out. Correction CHECK_OUT target. */
  autoClosedCmp21: 'SWP-ATT-9004',
  /** Budi @ CMP-0022 — PENDING LATE. Cross-company OUT_OF_SCOPE target for Rudi. */
  cmp22OutOfScope: 'SWP-ATT-9005',
  /** Rudi's OWN ESCALATED record @ CMP-0021. VERIFY_OWN_RECORD target. */
  rudiOwnEscalated: 'SWP-ATT-9006',
} as const;

/** Correction fixtures planted by the seed (07-02). */
export const COR = {
  /** PENDING/CHECK_OUT on SWP-ATT-9004 — approve target (applies → 9004 gains CORRECTED). */
  approveTarget: 'SWP-COR-8001',
  /** PENDING/CHECK_IN on SWP-ATT-9002 — reject target. */
  rejectTarget: 'SWP-COR-8002',
} as const;

// ---------------------------------------------------------------------------
// apiAsWithKey — like apiAs but with a CALLER-SUPPLIED fixed Idempotency-Key so
// two calls replay identically (apiAs always sends a fresh random key per call).
// ---------------------------------------------------------------------------

export interface ApiResult {
  status: number;
  body: unknown;
}

/**
 * apiAsWithKey — issue an authenticated fetch() from inside the page using the in-memory
 * Bearer token and a FIXED Idempotency-Key. Calling twice with the same key + same body
 * replays the stored 2xx response; same key + DIFFERENT body → 409 IDEMPOTENCY_KEY_REUSED.
 * The page MUST already be logged in (call waitForToken first after a goto).
 */
export async function apiAsWithKey(
  page: Page,
  method: string,
  path: string,
  body: unknown,
  idempotencyKey: string,
): Promise<ApiResult> {
  return page.evaluate(
    async ({ base, m, p, b, key }) => {
      const token = (window as unknown as { __swp_get_token__?: string }).__swp_get_token__ ?? null;
      const headers: Record<string, string> = {
        'Content-Type': 'application/json',
        'Idempotency-Key': key,
      };
      if (token) headers.Authorization = `Bearer ${token}`;
      const res = await fetch(`${base}${p}`, {
        method: m,
        headers,
        credentials: 'include',
        body: b !== undefined ? JSON.stringify(b) : undefined,
      });
      let parsed: unknown = null;
      const ct = res.headers.get('content-type') ?? '';
      if (ct.includes('application/json')) parsed = await res.json();
      return { status: res.status, body: parsed };
    },
    { base: API_BASE, m: method, p: path, b: body, key: idempotencyKey },
  );
}

// ---------------------------------------------------------------------------
// Bulk-envelope helpers (BulkActionResponse {succeeded, failed})
// ---------------------------------------------------------------------------

export interface BulkBody {
  succeeded?: string[];
  failed?: Array<{ id?: string; error?: { code?: string; message?: string } }>;
}

/** Coerce a response body to the BulkActionResponse shape. */
export function bulk(body: unknown): BulkBody {
  return (body as BulkBody) ?? {};
}

// ---------------------------------------------------------------------------
// Verification-queue UI helpers (DataTable rows = div.border-b)
// ---------------------------------------------------------------------------

/**
 * queueRow — the verification-queue DataTable row whose visible text contains `text`
 * (e.g. the employee_id `SWP-EMP-3001`, rendered mono in the employee column).
 */
export function queueRow(page: Page, text: string): Locator {
  return page.locator('div.border-b').filter({ hasText: text }).first();
}

/** Assert a verification-queue row containing `text` is visible (queue loaded). */
export async function expectQueueRow(page: Page, text: string): Promise<void> {
  await expect(queueRow(page, text)).toBeVisible({ timeout: 30_000 });
}

/** Assert NO verification-queue row contains `text` (scoped-out / not an exception). */
export async function expectNoQueueRow(page: Page, text: string): Promise<void> {
  await expect(page.locator('div.border-b').filter({ hasText: text })).toHaveCount(0, {
    timeout: 20_000,
  });
}
