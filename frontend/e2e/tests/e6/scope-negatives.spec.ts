/**
 * tests/e6/scope-negatives.spec.ts
 *
 * E6 · leader scope negatives (LVE-01 RBAC) against the REAL stack. A shift_leader
 * (Rudi @ CMP-0021) may only act on their OWN company's leave requests; HR/super is global.
 *
 * Coverage:
 *   SCOPE-403-l1   Rudi POST /leave-requests/SWP-LR-8004:approve-l1 (Budi @ CMP-0022) → 403 OUT_OF_SCOPE.
 *   SCOPE-403-list Rudi GET /leave-requests?company_id=SWP-CMP-0022 → 403 OUT_OF_SCOPE.
 *   QUEUE-hidden   the SL queue (/leave, default PENDING_L1) does NOT list the cross-company 8004.
 *   HR-global-ok   HR GET /leave-requests?company_id=SWP-CMP-0022 → 200 (global scope).
 *
 * Seed (08-02): SWP-LR-8004 Budi @ CMP-0022 PENDING_L1 (Rudi cross-company target);
 * SWP-LR-8001 Dewi @ CMP-0021 PENDING_L1 (in Rudi's scope — the queue's own-company row).
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import {
  LR,
  apiAs,
  errorCode,
  expectLeaveRow,
  expectNoLeaveRow,
  waitForToken,
} from '../../lib/e6-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// SCOPE-403-l1 — leader L1-approving a cross-company request → 403 OUT_OF_SCOPE
// ---------------------------------------------------------------------------

test('SCOPE-403-l1 · leader Rudi :approve-l1 on 8004 (Budi @ CMP-0022) → 403 OUT_OF_SCOPE', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/leave');
  await waitForToken(page);

  const res = await apiAs(page, 'POST', `/leave-requests/${LR.outOfScope}:approve-l1`, {});
  expect(res.status).toBe(403);
  expect(errorCode(res.body)).toBe('OUT_OF_SCOPE');
});

// ---------------------------------------------------------------------------
// SCOPE-403-list — leader listing another company → 403 OUT_OF_SCOPE
// ---------------------------------------------------------------------------

test('SCOPE-403-list · leader Rudi GET /leave-requests?company_id=SWP-CMP-0022 → 403 OUT_OF_SCOPE', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/leave');
  await waitForToken(page);

  const res = await apiAs(page, 'GET', '/leave-requests?company_id=SWP-CMP-0022');
  expect(res.status).toBe(403);
  expect(errorCode(res.body)).toBe('OUT_OF_SCOPE');
});

// ---------------------------------------------------------------------------
// QUEUE-hidden — the leader's own-company queue excludes the cross-company request
// ---------------------------------------------------------------------------

test('QUEUE-hidden · SL queue lists own-company 8001 but NOT cross-company 8004', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/leave');
  await waitForToken(page);

  // Default SL status filter = PENDING_L1 → Rudi's own-company 8001 (Dewi @ Plaza Senayan)
  // is listed. The queue renders employee_name, so anchor on the name.
  await expectLeaveRow(page, 'Dewi Lestari');
  // The cross-company CMP-0022 request 8004 (Budi Santoso) is scoped out of the leader queue.
  await expectNoLeaveRow(page, 'Budi Santoso');
});

// ---------------------------------------------------------------------------
// HR-global-ok — HR reads any company → 200
// ---------------------------------------------------------------------------

test('HR-global-ok · hr_admin GET /leave-requests?company_id=SWP-CMP-0022 → 200 (global scope)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/leave');
  await waitForToken(page);

  const res = await apiAs(page, 'GET', '/leave-requests?company_id=SWP-CMP-0022');
  expect(res.status).toBe(200);
});
