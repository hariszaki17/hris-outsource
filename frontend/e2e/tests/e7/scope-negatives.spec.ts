/**
 * tests/e7/scope-negatives.spec.ts
 *
 * E7 · leader scope + self-approval negatives + OT_BELOW_MIN (OVT-01 RBAC/INV-5) against
 * the REAL stack. A shift_leader (Rudi @ CMP-0021) may only L1-approve their OWN company's
 * OT, never their own record. Mirrors tests/e6/scope-negatives.spec.ts (apiAs real-403).
 *
 * Coverage:
 *   OT-SCOPE-403   Rudi :approve-l1 SWP-OT-30005 (Budi @ CMP-0022) → 403 OUT_OF_SCOPE.
 *   OT-SELF-403    Rudi :approve-l1 SWP-OT-30004 (Rudi's OWN employee_id) → 403 SELF_APPROVAL_FORBIDDEN.
 *   OT-QUEUE-hidden the SL queue (/overtime default PENDING_L1) does NOT list cross-company 30005.
 *   OT-LIST-403    Rudi GET /overtime?company_id=SWP-CMP-0022 → 403 OUT_OF_SCOPE.
 *   OT-BELOW-MIN   the seeded below-min SWP-OT-30006 surfaces the OT_BELOW_MIN effect:
 *                  calculation.skipped_too_short=true, counted_minutes=0, min_minutes_threshold>0.
 *
 * NB OT_BELOW_MIN's only production trigger is the OT create/auto-detect path (mobile/system,
 * OUT of the web HTTP surface). The 422 wire shape (OT_BELOW_MIN + fields.{counted_minutes,
 * min_minutes}) is pinned by the 09-03 Go contract test via the exported EnforceMinMinutes
 * seam; here we assert the seeded below-min record's calculation honestly through the real GET,
 * not a fabricated web 422.
 *
 * Seed (09-02): 30005 Budi @ CMP-0022 PENDING_L1; 30004 Rudi's own (EMP-1108) PENDING_L1;
 * 30006 Dewi counted 0 / skipped_too_short PENDING_L1.
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import {
  E7_CMP,
  E7_NAME,
  OT,
  apiAs,
  errorCode,
  expectNoOtRow,
  expectOtRow,
  waitForToken,
} from '../../lib/e7-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// OT-SCOPE-403 — leader L1-approving a cross-company OT → 403 OUT_OF_SCOPE
// ---------------------------------------------------------------------------

test('OT-SCOPE-403 · leader Rudi :approve-l1 on 30005 (Budi @ CMP-0022) → 403 OUT_OF_SCOPE', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/overtime');
  await waitForToken(page);

  const res = await apiAs(page, 'POST', `/overtime/${OT.outOfScope}:approve-l1`, {});
  expect(res.status).toBe(403);
  expect(errorCode(res.body)).toBe('OUT_OF_SCOPE');
});

// ---------------------------------------------------------------------------
// OT-SELF-403 — leader approving their OWN OT → 403 SELF_APPROVAL_FORBIDDEN
// ---------------------------------------------------------------------------

test('OT-SELF-403 · leader Rudi :approve-l1 on 30004 (Rudi own) → 403 SELF_APPROVAL_FORBIDDEN', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/overtime');
  await waitForToken(page);

  const res = await apiAs(page, 'POST', `/overtime/${OT.self}:approve-l1`, {});
  expect(res.status).toBe(403);
  expect(errorCode(res.body)).toBe('SELF_APPROVAL_FORBIDDEN');
});

// ---------------------------------------------------------------------------
// OT-QUEUE-hidden — the leader's own-company queue excludes the cross-company OT
// ---------------------------------------------------------------------------

test('OT-QUEUE-hidden · SL queue lists own-company rows but NOT cross-company 30005 (Budi)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/overtime');
  await waitForToken(page);

  // Rudi's PENDING_L1 queue @ CMP-0021 renders Dewi rows; the cross-company Budi row is scoped out.
  await expectOtRow(page, E7_NAME.dewi);
  await expectNoOtRow(page, E7_NAME.budi);
});

// ---------------------------------------------------------------------------
// OT-LIST-403 — leader listing another company → 403 OUT_OF_SCOPE
// ---------------------------------------------------------------------------

test('OT-LIST-403 · leader Rudi GET /overtime?company_id=SWP-CMP-0022 → 403 OUT_OF_SCOPE', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/overtime');
  await waitForToken(page);

  const res = await apiAs(page, 'GET', `/overtime?company_id=${E7_CMP.other}`);
  expect(res.status).toBe(403);
  expect(errorCode(res.body)).toBe('OUT_OF_SCOPE');
});

// ---------------------------------------------------------------------------
// OT-BELOW-MIN — the seeded below-min record surfaces the OT_BELOW_MIN effect
// ---------------------------------------------------------------------------

test('OT-BELOW-MIN · seeded SWP-OT-30006 calculation reflects below-min skip (counted 0, threshold>0)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/overtime');
  await waitForToken(page);

  // GET the seeded below-min record. Its calculation honestly carries the OT_BELOW_MIN
  // effect: the worked duration is below the rule's min_minutes so 0 minutes are counted.
  const res = await apiAs(page, 'GET', `/overtime/${OT.belowMin}`);
  expect(res.status).toBe(200);
  const calc = (res.body as { data?: { calculation?: { skipped_too_short?: boolean; counted_minutes?: number; min_minutes_threshold?: number } } })
    ?.data?.calculation;
  expect(calc?.skipped_too_short).toBe(true);
  expect(calc?.counted_minutes).toBe(0);
  expect(calc?.min_minutes_threshold).toBeGreaterThan(0);
  // (The OT_BELOW_MIN 422 wire shape — code + fields.{counted_minutes,min_minutes} — is
  // pinned by the 09-03 contract test; its create/auto-detect trigger is out of web scope.)
});
