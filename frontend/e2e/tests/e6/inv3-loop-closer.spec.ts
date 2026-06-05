/**
 * tests/e6/inv3-loop-closer.spec.ts
 *
 * E6 · the INV-3 loop-closer — the PROOF that approving a leave overlapping a
 * seeded E4 schedule entry (a) cancels that entry and (b) populates the REAL
 * approved_leave_days table, so the Phase-6 over-leave conflict now fires from the
 * production leave source (not the Phase-6 seeded fixture).
 *
 * The seed (08-02) plants:
 *   SWP-SCH-6002  Dewi (SWP-EMP-3001) SCHEDULED "Pagi" on monday+2 (Wed) @ CMP-0021.
 *   SWP-LR-8007   Dewi PENDING_HR leave on monday+2 (start=end) @ CMP-0021 — overlaps it.
 *
 * The conflict engine checks SHIFT_OVER_LEAVE (step 5) BEFORE DOUBLE_SHIFT (step 6):
 *   - BEFORE approval: no approved_leave_days row on monday+2 → a schedule create on the
 *     occupied date hits DOUBLE_SHIFT (the live SWP-SCH-6002 entry), NOT SHIFT_OVER_LEAVE.
 *   - AFTER approval: the approval inserts the approved_leave_days row AND cancels
 *     SWP-SCH-6002 → the same create now hits SHIFT_OVER_LEAVE from the real row.
 *
 * Status-value boundary (CRITICAL):
 *   E6 :approve-final response.schedule_impact[].new_status === 'LEAVE'  (DTO enum)
 *   E4 GET /schedule.ScheduleEntry.status               === 'CANCELLED_BY_LEAVE' (DB value)
 *   There is NO raw DB row with status='LEAVE'. NB the E6 GET /leave-requests/{id}
 *   does NOT re-derive schedule_impact (it is an action-response-only surface), so the
 *   'LEAVE' mapping is asserted on the approve-final action response (the contract surface
 *   that carries it) — driven against the REAL Go BE, not a mock.
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import {
  EMP,
  LR,
  SCH,
  apiAs,
  mondayPlus,
  openLeaveDetail,
  expectLeaveStatus,
  scheduleCheckOverLeave,
  scheduleEntryStatus,
  waitForToken,
} from '../../lib/e6-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// INV-3 loop-closer — approve 8007 → schedule cancelled + SHIFT_OVER_LEAVE
// fires from the REAL approved_leave_days row the approval inserted.
// ---------------------------------------------------------------------------

test('INV-3-loop-closer · approving 8007 cancels SWP-SCH-6002 (new_status LEAVE / DB CANCELLED_BY_LEAVE) and a fresh schedule check now hits SHIFT_OVER_LEAVE from the real approved_leave_days row', async ({
  page,
}) => {
  const wed = mondayPlus(2); // SWP-SCH-6002 + SWP-LR-8007 both land here.

  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/leave');
  await waitForToken(page);

  // -------------------------------------------------------------------------
  // Step 1 (pre-condition) — no approved_leave_days row for Dewi on monday+2 yet.
  // A schedule create on the occupied date hits DOUBLE_SHIFT (the live entry),
  // proving SHIFT_OVER_LEAVE is NOT yet sourced from leave for this day.
  // -------------------------------------------------------------------------
  const before = await scheduleCheckOverLeave(page, EMP.dewi, wed);
  expect(before.code).not.toBe('SHIFT_OVER_LEAVE');
  // The seeded SWP-SCH-6002 still occupies the cell → DOUBLE_SHIFT.
  expect(before.code).toBe('DOUBLE_SHIFT');

  // -------------------------------------------------------------------------
  // Step 2 (the approval) — HR finalises SWP-LR-8007 against the REAL BE.
  // The real detail screen loads the request from the live API (proves the screen
  // works), then the approve-final action is driven so we can capture the action
  // RESPONSE — the contract surface that carries schedule_impact[] (the GET does not
  // re-derive it). Assert the E6 DTO value new_status === 'LEAVE' (openapi
  // ScheduleImpactEntry.new_status enum [LEAVE, UNASSIGNED]) for the cancelled
  // SWP-SCH-6002 entry — sourced from the real CANCELLED_BY_LEAVE → LEAVE mapping.
  // -------------------------------------------------------------------------
  await openLeaveDetail(page, LR.inv3Overlap);
  // The detail header shows the PENDING_HR request loaded from the real BE.
  await expect(page.getByText('Menunggu HR').first()).toBeVisible({ timeout: 15_000 });

  const approve = await apiAs(
    page,
    'POST',
    `/leave-requests/${LR.inv3Overlap}:approve-final`,
    {},
  );
  expect(approve.status).toBe(200);
  const approved = (approve.body as { data?: { status?: string; schedule_impact?: Array<{ schedule_id?: string; new_status?: string }> } }).data;
  expect(approved?.status).toBe('APPROVED');
  // The schedule_impact[] lists SWP-SCH-6002 with the E6 contract value 'LEAVE'.
  const impact = approved?.schedule_impact ?? [];
  const cancelled = impact.find((s) => s.schedule_id === SCH.inv3Entry) ?? impact[0];
  expect(cancelled?.new_status).toBe('LEAVE');

  // The persisted leave-request status is APPROVED (the UI reflects this on refetch).
  await expectLeaveStatus(page, LR.inv3Overlap, 'APPROVED');

  // -------------------------------------------------------------------------
  // Step 3 (loop closed) — a fresh E4 schedule create for Dewi on monday+2 now hits
  // SHIFT_OVER_LEAVE from the REAL approved_leave_days row the approval inserted
  // (NOT the Phase-6 fixture). details.leave_request_id is the freshly-approved id.
  // -------------------------------------------------------------------------
  const after = await scheduleCheckOverLeave(page, EMP.dewi, wed);
  expect(after.status).toBe(409);
  expect(after.code).toBe('SHIFT_OVER_LEAVE');
  expect(after.details?.leave_request_id).toBe(LR.inv3Overlap);

  // -------------------------------------------------------------------------
  // Step 4 (schedule cancelled) — GET /schedule surfaces SWP-SCH-6002 with the
  // E4 DB status 'CANCELLED_BY_LEAVE' (NOT 'LEAVE', which lives only at the E6 DTO).
  // -------------------------------------------------------------------------
  const status = await scheduleEntryStatus(
    page,
    'SWP-CMP-0021',
    mondayPlus(0),
    mondayPlus(6),
    SCH.inv3Entry,
  );
  expect(status).toBe('CANCELLED_BY_LEAVE');
});
