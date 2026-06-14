/**
 * tests/e7/bulk.spec.ts
 *
 * E7 · bulk approve/reject partial-success (OVT-01) against the REAL stack. The bulk
 * envelope is BulkResult {succeeded:[id], failed:[{id, error:{code}}]} — 200 when ≥1
 * succeeded, 422 when all failed. Driven via apiAs for the mixed-row envelope shape
 * (terminal + cross-company rows are not all selectable through the queue UI), plus a
 * UI-driven happy bulk-approve via the "Setujui Massal" header button.
 *
 * Coverage:
 *   BULK-partial        HR bulk-approve [30003 PENDING_HR, 30007 APPROVED terminal] →
 *                       succeeded=[30003], failed=[30007 CONFLICT]; 200.
 *   BULK-reject-partial leader bulk-reject [30002 in-scope, 30005 cross-company] →
 *                       succeeded=[30002], failed=[30005 OUT_OF_SCOPE]; 200.
 *   BULK-all-fail       HR bulk-approve [30007, 30008] (both terminal) → 422, all failed.
 *   BULK-ui             HR selects the PENDING_HR row + "Setujui Massal" → success toast.
 *
 * Seed (09-02): 30002 PENDING_L1 (Rudi scope); 30003 PENDING_HR; 30005 PENDING_L1 @ CMP-0022;
 * 30007 APPROVED; 30008 REJECTED.
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import {
  E7_NAME,
  OT,
  OT_BTN,
  apiAs,
  bulk,
  bulkFailedCode,
  expectOtRow,
  otRow,
  waitForToken,
} from '../../lib/e7-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// BULK-partial — HR bulk-approve a PENDING_HR + a terminal record → partial success
// ---------------------------------------------------------------------------

test('BULK-partial · HR bulk-approve [30003 PENDING_HR, 30007 APPROVED] → succeeded 30003, failed 30007 CONFLICT', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/overtime');
  await waitForToken(page);

  const res = await apiAs(page, 'POST', '/overtime:bulk-approve', {
    ids: [OT.finalTarget, OT.approved],
    note: 'Persetujuan massal.',
  });
  expect(res.status).toBe(200);

  const body = bulk(res.body);
  expect(body.succeeded).toContain(OT.finalTarget);
  expect(bulkFailedCode(res.body, OT.approved)).toBe('CONFLICT');
});

// ---------------------------------------------------------------------------
// BULK-reject-partial — leader bulk-reject in-scope + cross-company → partial success
// ---------------------------------------------------------------------------

test('BULK-reject-partial · leader bulk-reject [30002 in-scope, 30005 CMP-0022] → succeeded 30002, failed 30005 OUT_OF_SCOPE', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/overtime');
  await waitForToken(page);

  const res = await apiAs(page, 'POST', '/overtime:bulk-reject', {
    ids: [OT.l1Target, OT.outOfScope],
    reason: 'Penolakan massal leader.',
  });
  expect(res.status).toBe(200);

  const body = bulk(res.body);
  expect(body.succeeded).toContain(OT.l1Target);
  expect(bulkFailedCode(res.body, OT.outOfScope)).toBe('OUT_OF_SCOPE');
});

// ---------------------------------------------------------------------------
// BULK-all-fail — every selected row terminal → 422, all failed
// ---------------------------------------------------------------------------

test('BULK-all-fail · HR bulk-approve [30007 APPROVED, 30008 REJECTED] → 422, all failed', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/overtime');
  await waitForToken(page);

  const res = await apiAs(page, 'POST', '/overtime:bulk-approve', {
    ids: [OT.approved, OT.rejected],
    note: 'Massal.',
  });
  expect(res.status).toBe(422);

  const body = bulk(res.body);
  expect(body.succeeded ?? []).toHaveLength(0);
  expect((body.failed ?? []).length).toBe(2);
});

// ---------------------------------------------------------------------------
// BULK-ui — the "Setujui Massal" header button drives a real bulk-approve
// ---------------------------------------------------------------------------

test('BULK-ui · HR selects the PENDING_HR row + "Setujui Massal" → success toast + APPROVED', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/overtime');
  await waitForToken(page);

  // Select the single PENDING_HR row (Dewi @ CMP-0021) via its row checkbox.
  await expectOtRow(page, E7_NAME.dewi);
  await otRow(page, E7_NAME.dewi).getByRole('checkbox').check();

  // The header button now reads "Setujui {count} lembur" (t('approvals.bulkApproveCount')).
  // Click it → opens BulkApproveModal → confirm "Setujui".
  await page.getByRole('button', { name: 'Setujui 1 lembur', exact: true }).click();
  await page.getByRole('button', { name: OT_BTN.bulkApproveConfirm, exact: true }).click();

  // Success toast (t('approvals.bulkApprovedToast')).
  await expect(page.getByText(/lembur disetujui/i).first()).toBeVisible({ timeout: 15_000 });
});
