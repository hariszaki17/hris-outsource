/**
 * tests/e4/leader-scope.spec.ts
 *
 * E4 · leader scope (INV-3 / SA-3) — a shift_leader may only schedule/list agents at
 * their OWN company; HR/super-admin scope is global. Asserted against the REAL API.
 *
 * Coverage:
 *   SCOPE-403-create  Rudi (leader @ CMP-0021) POST /schedule for EMP-2891 (Budi @ CMP-0022) → 403 OUT_OF_SCOPE
 *   SCOPE-403-list    Rudi GET /schedule?company_id=SWP-CMP-0022 → 403 OUT_OF_SCOPE (cannot read another company)
 *   SCOPE-hr-global   hr_admin GET /schedule?company_id=SWP-CMP-0022 → 200 (global scope)
 *
 * Seed (06-02): Budi (SWP-EMP-2891) placed at CMP-0022 (SWP-PL-5002, start 2026-02-01) — the
 * leader-scope-403 target. Rudi leads CMP-0021 only.
 */

import { SEED, addDaysIso, apiAs, errorCode, waitForToken } from '../../lib/e4-helpers.js';
import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// SCOPE-403-create
// ---------------------------------------------------------------------------

test('SCOPE-403-create · leader Rudi scheduling EMP-2891 (Budi @ CMP-0022, out of scope) → 403 OUT_OF_SCOPE', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/schedule');
  await waitForToken(page);

  const res = await apiAs(page, 'POST', '/schedule', {
    kind: 'single',
    employee_id: 'SWP-EMP-2891', // Budi @ CMP-0022 — outside Rudi's CMP-0021 scope
    shift_master_id: 'SWP-SHF-001',
    date: SEED.freeDate(), // in-window for SWP-PL-5002 (start 2026-02-01)
    is_day_off: false,
  });
  expect(res.status).toBe(403);
  expect(errorCode(res.body)).toBe('OUT_OF_SCOPE');
});

// ---------------------------------------------------------------------------
// SCOPE-403-list
// ---------------------------------------------------------------------------

test('SCOPE-list-auto-scope · leader Rudi GET /schedule?company_id=SWP-CMP-0022 → 200, silently pinned to his own company', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/schedule');
  await waitForToken(page);

  const monday = SEED.monday();
  // The schedule LIST endpoint does NOT 403 a cross-company read. The backend
  // (ScheduleService.ListSchedule) auto-scopes a shift_leader: it ignores the supplied
  // company_id and pins the filter to the leader's OWN company (CMP-0021), then runs the
  // GuardCompany check against that — so a leader can never SEE another company's grid,
  // but the request resolves 200 with their own data rather than an error. (The WRITE path
  // — POST /schedule for an out-of-scope employee — is the real 403 OUT_OF_SCOPE; see
  // SCOPE-403-create.)
  const res = await apiAs(
    page,
    'GET',
    `/schedule?company_id=SWP-CMP-0022&start_date=${monday}&end_date=${addDaysIso(monday, 6)}`,
  );
  expect(res.status, JSON.stringify(res.body)).toBe(200);
  // The cross-company company_id was dropped — none of the returned entries belong to
  // CMP-0022 (the leader only ever sees CMP-0021 rows, or an empty week).
  const entries = ((res.body as { data?: Array<{ company_id?: string }> })?.data ?? []) as Array<{
    company_id?: string;
  }>;
  for (const e of entries) {
    expect(e.company_id).not.toBe('SWP-CMP-0022');
  }
});

// ---------------------------------------------------------------------------
// SCOPE-hr-global
// ---------------------------------------------------------------------------

test('SCOPE-hr-global · hr_admin GET /schedule?company_id=SWP-CMP-0022 → 200 (global scope)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/schedule');
  await waitForToken(page);

  const monday = SEED.monday();
  const res = await apiAs(
    page,
    'GET',
    `/schedule?company_id=SWP-CMP-0022&start_date=${monday}&end_date=${addDaysIso(monday, 6)}`,
  );
  expect(res.status).toBe(200);
});
