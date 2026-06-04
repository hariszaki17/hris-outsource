/**
 * tests/e5/scope-negatives.spec.ts
 *
 * E5 · leader scope + own-record negatives (RBAC §17 / INV) against the REAL stack.
 * A shift_leader (Rudi @ CMP-0021) may only act on their OWN company's records and may
 * NOT verify their own escalated record; HR/super-admin scope is global.
 *
 * Coverage:
 *   SCOPE-403-verify  Rudi POST /attendance/9005:verify (CMP-0022) → 403 OUT_OF_SCOPE.
 *   SCOPE-403-list    Rudi GET /attendance?company_id=CMP-0022 → 403 OUT_OF_SCOPE.
 *   VERIFY-OWN-403    Rudi POST /attendance/9006:verify (his own ESCALATED) → 403 VERIFY_OWN_RECORD.
 *   HR-global-ok      HR GET /attendance?company_id=CMP-0022 → 200 (global scope).
 *
 * Seed (07-02): 9005 Budi @ CMP-0022 (cross-company); 9006 Rudi (EMP-1108) ESCALATED @ CMP-0021.
 * OUTSIDE_CORRECTION_WINDOW (422) is contract-tested in 07-03 against the exported
 * CheckCorrectionWindow seam — the correction-CREATE path is mobile/agent-only (out of web
 * scope), and 07-02 seeds only in-window corrections, so it is not reachable from the web E2E.
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import { ATT, apiAs, errorCode, waitForToken } from '../../lib/e5-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// SCOPE-403-verify — leader verifying a cross-company record → 403 OUT_OF_SCOPE
// ---------------------------------------------------------------------------

test('SCOPE-403-verify · leader Rudi verifying 9005 (Budi @ CMP-0022) → 403 OUT_OF_SCOPE', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/attendance/verification');
  await waitForToken(page);

  const res = await apiAs(page, 'POST', `/attendance/${ATT.cmp22OutOfScope}:verify`, {});
  expect(res.status).toBe(403);
  expect(errorCode(res.body)).toBe('OUT_OF_SCOPE');
});

// ---------------------------------------------------------------------------
// SCOPE-403-list — leader listing another company → 403 OUT_OF_SCOPE
// ---------------------------------------------------------------------------

test('SCOPE-403-list · leader Rudi GET /attendance?company_id=SWP-CMP-0022 → 403 OUT_OF_SCOPE', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/attendance/verification');
  await waitForToken(page);

  const res = await apiAs(
    page,
    'GET',
    '/attendance?company_id=SWP-CMP-0022&exceptions_only=true',
  );
  expect(res.status).toBe(403);
  expect(errorCode(res.body)).toBe('OUT_OF_SCOPE');
});

// ---------------------------------------------------------------------------
// VERIFY-OWN-403 — leader verifying their own escalated record → 403 VERIFY_OWN_RECORD
// ---------------------------------------------------------------------------

test('VERIFY-OWN-403 · leader Rudi verifying his OWN escalated 9006 → 403 VERIFY_OWN_RECORD', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/attendance/verification');
  await waitForToken(page);

  const res = await apiAs(page, 'POST', `/attendance/${ATT.rudiOwnEscalated}:verify`, {});
  expect(res.status).toBe(403);
  expect(errorCode(res.body)).toBe('VERIFY_OWN_RECORD');
});

// ---------------------------------------------------------------------------
// HR-global-ok — HR reads any company → 200
// ---------------------------------------------------------------------------

test('HR-global-ok · hr_admin GET /attendance?company_id=SWP-CMP-0022 → 200 (global scope)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/attendance/verification');
  await waitForToken(page);

  const res = await apiAs(page, 'GET', '/attendance?company_id=SWP-CMP-0022');
  expect(res.status).toBe(200);
});
