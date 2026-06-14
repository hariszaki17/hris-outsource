/**
 * tests/e4/conflict-negatives.spec.ts
 *
 * Exhaustive conflict-engine negatives for E4 · schedule writes, asserted against the
 * REAL Go API (no mocks). Each conflict code is its own test(). Direct writes use
 * apiAs (POST /schedule → top-level {error:{code,details}} envelope); the SHIFT_OVER_LEAVE
 * and DOUBLE_SHIFT cases additionally assert the real popover UX.
 *
 * Codes (06-02 engine order):
 *   CONF-double-shift              EMP-1108 on its seeded Tue date, force_replace=false → 409 DOUBLE_SHIFT (+existing_entry_id)
 *   CONF-outside-placement-period EMP-1108 on 2030-01-01 (outside placement) → 422 OUTSIDE_PLACEMENT_PERIOD
 *   CONF-shift-over-leave         EMP-3001 on the seeded approved-leave Thu (SWP-LR-44210) → 409 SHIFT_OVER_LEAVE
 *                                 (details.leave_request_id === 'SWP-LR-44210') — honest seed, not mocked
 *   CONF-shift-deactivated        deactivate SWP-SHF-002, then assign it → 422 SHIFT_DEACTIVATED
 *
 * (The former SHIFT_NOT_FOR_SERVICE_LINE conflict is dropped: shift master is now
 *  service-line-independent — locked 2026-06-12. Any active shift is assignable to any agent.)
 *
 * Login shiftLeader (Rudi @ CMP-0021) — all targets are in his scope so the BE reaches the
 * 422/409 conflict checks (not OUT_OF_SCOPE). Seeded anchors per 06-02 SUMMARY.
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import {
  SEED,
  apiAs,
  cellButton,
  errorCode,
  errorDetails,
  expectConflictToast,
  openCell,
  selectCompany,
  waitForToken,
} from '../../lib/e4-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

const SINGLE = { kind: 'single' as const };
const DEWI = 'Dewi Lestari';

// ---------------------------------------------------------------------------
// CONF-double-shift (409) — EMP-1108 already has a seeded entry on its Tue date
// ---------------------------------------------------------------------------

test('CONF-double-shift · re-assigning EMP-1108 on its occupied date (force_replace=false) → 409 DOUBLE_SHIFT', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/schedule');
  await waitForToken(page);

  const res = await apiAs(page, 'POST', '/schedule', {
    ...SINGLE,
    employee_id: 'SWP-EMP-1108',
    shift_master_id: 'SWP-SHF-001',
    date: SEED.rudiEntryDate(), // SWP-SCH-6001 occupies this cell
    is_day_off: false,
    force_replace: false,
  });
  expect(res.status).toBe(409);
  expect(errorCode(res.body)).toBe('DOUBLE_SHIFT');
  expect(errorDetails(res.body)?.existing_entry_id).toBeTruthy();
});

// ---------------------------------------------------------------------------
// CONF-outside-placement-period (422) — date outside the placement window
// ---------------------------------------------------------------------------

test('CONF-outside-placement-period · EMP-1108 on 2030-01-01 (outside placement) → 422 OUTSIDE_PLACEMENT_PERIOD', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/schedule');
  await waitForToken(page);

  const res = await apiAs(page, 'POST', '/schedule', {
    ...SINGLE,
    employee_id: 'SWP-EMP-1108',
    shift_master_id: 'SWP-SHF-001',
    date: '2030-01-01', // far outside SWP-PL-5001 (2026-01-01 .. 2026-12-31)
    is_day_off: false,
  });
  expect(res.status).toBe(422);
  expect(errorCode(res.body)).toBe('OUTSIDE_PLACEMENT_PERIOD');
});

// ---------------------------------------------------------------------------
// CONF-shift-over-leave (409) — honest, driven by the seeded approved_leave_days row
// ---------------------------------------------------------------------------

test('CONF-shift-over-leave · EMP-3001 on the seeded approved-leave Thu (SWP-LR-44210) → 409 SHIFT_OVER_LEAVE (+leave_request_id)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);

  // 1) Real API: POST for Dewi on her approved-leave date → 409 SHIFT_OVER_LEAVE with the
  //    seeded leave_request_id. This exercises the REAL approved_leave_days read (not a mock).
  await page.goto('/schedule');
  await waitForToken(page);
  const res = await apiAs(page, 'POST', '/schedule', {
    ...SINGLE,
    employee_id: 'SWP-EMP-3001',
    shift_master_id: 'SWP-SHF-001',
    date: SEED.dewiLeaveDate(), // monday+3 (Thu) — approved_leave_days SWP-LR-44210
    is_day_off: false,
  });
  expect(res.status).toBe(409);
  expect(errorCode(res.body)).toBe('SHIFT_OVER_LEAVE');
  expect(errorDetails(res.body)?.leave_request_id).toBe('SWP-LR-44210');

  // 2) Real UI: the popover :check pre-flight blocks Dewi's leave cell with the over-leave toast.
  await selectCompany(page, 'Plaza Senayan');
  await expect(page.getByText(DEWI).first()).toBeVisible({ timeout: 30_000 });
  await openCell(page, DEWI, SEED.dewiLeaveDate());
  // Pick "Pagi" in the popover → :check returns failed[].error.code SHIFT_OVER_LEAVE → block toast.
  await page.locator('[aria-label^="Pilih shift untuk"] button', { hasText: 'Pagi' }).first().click();
  await expectConflictToast(page, /cuti yang disetujui|SWP-LR-44210/i);

  // The cell stays empty (no entry was written).
  await expect(cellButton(page, DEWI, SEED.dewiLeaveDate()).getByText('Pagi')).toHaveCount(0, {
    timeout: 10_000,
  });
});

// ---------------------------------------------------------------------------
// CONF-shift-deactivated (422) — picked master is inactive
// ---------------------------------------------------------------------------

test('CONF-shift-deactivated · assigning a deactivated master → 422 SHIFT_DEACTIVATED', async ({
  page,
}) => {
  // hr_admin to deactivate the master (shift-master writes are super/hr only).
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/shifts');
  await waitForToken(page);
  const deact = await apiAs(page, 'POST', '/shift-masters/SWP-SHF-002:deactivate', {});
  expect(deact.status).toBe(200);

  // Assign the now-inactive Malam to Dewi on a free in-window date.
  const res = await apiAs(page, 'POST', '/schedule', {
    ...SINGLE,
    employee_id: 'SWP-EMP-3001',
    shift_master_id: 'SWP-SHF-002',
    date: SEED.freeDate(),
    is_day_off: false,
  });
  expect(res.status).toBe(422);
  expect(errorCode(res.body)).toBe('SHIFT_DEACTIVATED');
});
