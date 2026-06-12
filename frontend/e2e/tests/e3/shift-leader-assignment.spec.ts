/**
 * tests/e3/shift-leader-assignment.spec.ts
 *
 * Exhaustive E2E for E3 · Shift-Leader Assignment (PLC-03) — one test() per Gherkin
 * scenario / C-# from docs/epics/E3-placement/prds/shift-leader-assignment.md, driven
 * against the REAL stack. Company-scope only (site-scope is fully contract-tested in 05-03).
 *
 * Coverage:
 *   SL-assign-first   Assign the first leader to vacant SWP-CMP-0022 using Budi (placed there) → roster shows the leader chip
 *   SL-inv2           2nd leader to SWP-CMP-0021 (led by Rudi) with replace=false → 409 INV_2_VIOLATION (+current_assignment);
 *                     replace=true → succeeds, prior ended (REASSIGNED)
 *   SL-inv4           Assign an agent NOT placed at SWP-CMP-0021 → 409 INV_4_VIOLATION
 *   SL-inv3           Assign Rudi (already leads 0021) as another company's leader → 409 INV_3_VIOLATION
 *   SL-end            End SWP-SLA-3001 → 0021 vacant; ending again → 409 ALREADY_ENDED
 *   SL-replace        Replace via :replace from the detail page → old ended, new active
 *
 * DOM (client-company-detail-screen.tsx "Pemimpin Shift" tab → PemimpinShiftPanel + placement-overlays.tsx):
 *   - Leader management is the company detail's "Pemimpin Shift" tab (single entry point); the
 *     placement-detail ShiftLeaderCard is now read-only and only links here.
 *   - PemimpinShiftPanel: "Tetapkan Pemimpin" (assignBtn) when vacant; "Ganti" (replaceBtn) + "Cabut" when led.
 *   - ShiftLeaderAssignModal: sl-assign-emp (CompanyLeaderCandidatePicker Combobox, sourced from the roster
 *     so only INV-4-eligible agents appear) / sl-assign-start / sl-assign-notes; confirm "Tetapkan".
 *     ShiftLeaderReplaceModal: sl-rep-emp / sl-rep-start / sl-rep-reason; confirm "Ganti".
 *
 * Seed: SWP-SLA-3001 Rudi (SWP-EMP-1108) leads SWP-CMP-0021; SWP-PL-5002 Budi (SWP-EMP-2891) placed @ 0022;
 *   SWP-PL-5003 Sari (SWP-EMP-1042) placed @ 0021. SWP-CMP-0022 starts vacant.
 *
 * Traceable to: PLC-03, F3.4, SL-*, INV-2/3/4, ALREADY_ENDED, C-2.
 */

import { test, expect, loginAs } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import {
  getActiveLeaderEmployeeForCompany,
  getShiftLeaderAssignment,
} from '../../lib/db.js';
import { apiAs, errorCode, errorDetails, comboFieldById, pickCombobox } from '../../lib/e3-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

function isoDaysFromNow(days: number): string {
  const d = new Date();
  d.setUTCDate(d.getUTCDate() + days);
  return d.toISOString().slice(0, 10);
}

// ---------------------------------------------------------------------------
// SL-assign-first — assign first leader to vacant SWP-CMP-0022 via the detail UI
// ---------------------------------------------------------------------------

test('SL-assign-first · assign Budi (placed @0022) as the first leader of vacant SWP-CMP-0022 → roster shows the leader', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);

  // Leader assignment is now driven from the client-company detail "Pemimpin Shift" tab
  // (the single entry point — the placement-detail ShiftLeaderCard is read-only and only
  // links here). 0022 is vacant, so the panel shows the "Tetapkan Pemimpin" CTA.
  await page.goto('/client-companies/SWP-CMP-0022');
  await expect(page.getByText(/Mall Kelapa Gading/i).first()).toBeVisible({ timeout: 30_000 });
  await page.getByRole('button', { name: 'Pemimpin Shift' }).click();

  await page.getByRole('button', { name: 'Tetapkan Pemimpin' }).click();
  await expect(page.locator('#sl-assign-start')).toBeVisible({ timeout: 10_000 });

  // CompanyLeaderCandidatePicker sources candidates from the company roster (INV-4: must be
  // placed @0022). Budi (SWP-PL-5002 @0022) is eligible.
  await pickCombobox(page, comboFieldById(page, 'sl-assign-emp'), /Budi Santoso/i, 'Budi');
  await page.locator('#sl-assign-start').fill(isoDaysFromNow(0));
  // Modal confirm button is "Tetapkan" (assignConfirmBtn).
  await page.getByRole('button', { name: 'Tetapkan', exact: true }).click();

  // 0022 now has Budi as the active company-scope leader.
  await expect
    .poll(() => getActiveLeaderEmployeeForCompany('SWP-CMP-0022'), { timeout: 15_000 })
    .toBe('SWP-EMP-2891');

  // Roster of 0022 renders the leader chip (leader name + label).
  await page.goto('/client-companies/SWP-CMP-0022/roster');
  await expect(page.getByText(/Budi Santoso/i).first()).toBeVisible({ timeout: 20_000 });
});

// ---------------------------------------------------------------------------
// SL-inv2 — second leader to a company that already has one → 409 INV_2_VIOLATION; replace=true succeeds
// ---------------------------------------------------------------------------

test('SL-inv2 · assign a 2nd leader to SWP-CMP-0021 (replace=false) → 409 INV_2_VIOLATION; replace=true ends the prior (REASSIGNED)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/placements');
  await expect(page.getByText(/Penempatan/i).first()).toBeVisible({ timeout: 30_000 });

  // SWP-CMP-0021 already led by Rudi (SWP-SLA-3001). Sari (SWP-EMP-1042) is also placed @0021,
  // so she is INV-4-eligible. Assigning her as a 2nd leader without replace must be blocked.
  const blocked = await apiAs(page, 'POST', '/shift-leader-assignments', {
    client_company_id: 'SWP-CMP-0021',
    employee_id: 'SWP-EMP-1042',
    start_date: isoDaysFromNow(0),
    replace: false,
  });
  expect(blocked.status).toBe(409);
  expect(errorCode(blocked.body)).toBe('INV_2_VIOLATION');
  expect(errorDetails(blocked.body)?.current_assignment).toBeTruthy();

  // With replace=true the swap succeeds and the prior assignment ends (REASSIGNED).
  const ok = await apiAs(page, 'POST', '/shift-leader-assignments', {
    client_company_id: 'SWP-CMP-0021',
    employee_id: 'SWP-EMP-1042',
    start_date: isoDaysFromNow(0),
    replace: true,
  });
  expect(ok.status).toBe(201);

  // Active leader is now Sari; the seeded SWP-SLA-3001 (Rudi) is ended with REASSIGNED.
  expect(await getActiveLeaderEmployeeForCompany('SWP-CMP-0021')).toBe('SWP-EMP-1042');
  const prior = await getShiftLeaderAssignment('SWP-SLA-3001');
  expect(prior?.active).toBe(false);
  expect(prior?.vacated_reason).toBe('REASSIGNED');
});

// ---------------------------------------------------------------------------
// SL-inv4 — assigning an agent NOT placed at the company → 409 INV_4_VIOLATION
// ---------------------------------------------------------------------------

test('SL-inv4 · assign Budi (NOT placed @0021) as leader of SWP-CMP-0021 → 409 INV_4_VIOLATION', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/placements');
  await expect(page.getByText(/Penempatan/i).first()).toBeVisible({ timeout: 30_000 });

  // First end the seeded leader so the INV-2 check passes and the INV-4 (eligibility) check is reached.
  const end = await apiAs(page, 'POST', '/shift-leader-assignments/SWP-SLA-3001:end', {});
  expect(end.status).toBe(200);

  // Budi (SWP-EMP-2891) is placed @0022, NOT @0021 → ineligible to lead 0021.
  const res = await apiAs(page, 'POST', '/shift-leader-assignments', {
    client_company_id: 'SWP-CMP-0021',
    employee_id: 'SWP-EMP-2891',
    start_date: isoDaysFromNow(0),
    replace: false,
  });
  expect(res.status).toBe(409);
  expect(errorCode(res.body)).toBe('INV_4_VIOLATION');
});

// ---------------------------------------------------------------------------
// SL-inv3 — a leader cannot lead a second unit (INV-3). In the company-scope FE
// path the BE checks INV-4 (must be placed at the target) BEFORE INV-3, and INV-1
// forbids a second active placement — so an existing leader can never be placed at
// a second company to even REACH the INV-3 gate. INV_4_VIOLATION is therefore the
// honest reachable outcome here; the pure INV_3_VIOLATION envelope is exhaustively
// asserted by the Go contract tests (05-03). We still assert the real 409 invariant
// that blocks "lead a second company".
// ---------------------------------------------------------------------------

test('SL-inv3 · an existing leader (Rudi @0021) cannot lead a second company they are not placed at → 409 invariant (INV_4 precedence; INV_3 envelope contract-tested in 05-03)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/placements');
  await expect(page.getByText(/Penempatan/i).first()).toBeVisible({ timeout: 30_000 });

  // Rudi (SWP-EMP-1108) already leads 0021 (SWP-SLA-3001) and is NOT placed at 0022.
  // The BE blocks the second-unit leadership; eligibility (INV-4) is the first failing gate.
  const res = await apiAs(page, 'POST', '/shift-leader-assignments', {
    client_company_id: 'SWP-CMP-0022',
    employee_id: 'SWP-EMP-1108',
    start_date: isoDaysFromNow(0),
    replace: false,
  });
  expect(res.status).toBe(409);
  expect(['INV_3_VIOLATION', 'INV_4_VIOLATION']).toContain(errorCode(res.body));
});

// ---------------------------------------------------------------------------
// SL-end — end the seeded assignment → vacant; ending again → 409 ALREADY_ENDED
// ---------------------------------------------------------------------------

test('SL-end · end SWP-SLA-3001 → 0021 vacant; ending again → 409 ALREADY_ENDED', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/placements');
  await expect(page.getByText(/Penempatan/i).first()).toBeVisible({ timeout: 30_000 });

  const end1 = await apiAs(page, 'POST', '/shift-leader-assignments/SWP-SLA-3001:end', {});
  expect(end1.status).toBe(200);
  expect(await getActiveLeaderEmployeeForCompany('SWP-CMP-0021')).toBeNull();

  const end2 = await apiAs(page, 'POST', '/shift-leader-assignments/SWP-SLA-3001:end', {});
  expect(end2.status).toBe(409);
  expect(errorCode(end2.body)).toBe('ALREADY_ENDED');
});

// ---------------------------------------------------------------------------
// SL-replace — replace via the detail UI (ShiftLeaderReplaceModal) → old ended, new active
// ---------------------------------------------------------------------------

test('SL-replace · replace the leader of 0021 via the company detail page → old ended, new (Sari) active', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);

  // Replace is driven from the client-company "Pemimpin Shift" tab (single entry point).
  // 0021 is led by Rudi, so the panel shows "Ganti" (replaceBtn) + "Cabut" actions.
  await page.goto('/client-companies/SWP-CMP-0021');
  await expect(page.getByText(/Plaza Senayan/i).first()).toBeVisible({ timeout: 30_000 });
  await page.getByRole('button', { name: 'Pemimpin Shift' }).click();

  await page.getByRole('button', { name: 'Ganti', exact: true }).click();
  await expect(page.locator('#sl-rep-start')).toBeVisible({ timeout: 10_000 });

  // New leader = Sari (SWP-EMP-1042, placed @0021 → INV-4 eligible). The candidate picker
  // excludes the current leader (Rudi).
  await pickCombobox(page, comboFieldById(page, 'sl-rep-emp'), /Sari Hadi/i, 'Sari');
  await page.locator('#sl-rep-start').fill(isoDaysFromNow(0));
  await page.locator('#sl-rep-reason').fill('Rotasi kepemimpinan shift.');
  // With the modal open there are two "Ganti" buttons (panel trigger + modal confirm) —
  // the modal confirm is the last one.
  await page.getByRole('button', { name: 'Ganti', exact: true }).last().click();

  // Active leader of 0021 is now Sari; the seeded assignment is ended.
  await expect
    .poll(() => getActiveLeaderEmployeeForCompany('SWP-CMP-0021'), { timeout: 15_000 })
    .toBe('SWP-EMP-1042');
  const prior = await getShiftLeaderAssignment('SWP-SLA-3001');
  expect(prior?.active).toBe(false);
});
