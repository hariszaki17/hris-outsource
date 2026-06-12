/**
 * tests/e4/schedule-grid.spec.ts
 *
 * Exhaustive happy-path E2E for E4 · Jadwal Mingguan grid (F4.2/F4.4 — SA-1/2/4/7,
 * CH-1/2) driven against the REAL stack via the real ShiftPickerPopover. One test()
 * per Gherkin scenario from docs/epics/E4-shift-scheduling/prds/daily-schedule-assignment.md.
 *
 * Coverage:
 *   SA-1-assign-autopublish        empty in-window cell → popover → pick Pagi → published toast → chip appears
 *   SA-2-replace                   existing cell → pick a different shift → replace (status MODIFIED) → chip changes
 *   SA-4-picker-filtered-by-line   SVC-003 agent (Dewi) → popover lists Malam (SVC-003) + untagged Pagi (SM-3)
 *   SA-7-mark-day-off              popover → "Tandai Libur (OFF)" → cell shows Libur
 *   CH-1-clear-cell                existing cell → "Hapus jadwal" → cell empties (real DELETE 204)
 *   CH-2-edit-swap                 existing entry → change shift → status MODIFIED (real PATCH)
 *   grid-empty                     a company with no placements → empty state
 *
 * The grid renders one row per agent that already has a schedule entry in the week.
 * Seed (06-02) @ CMP-0021 (Plaza Senayan): Rudi (SWP-EMP-1108) on monday+1 (Tue, Pagi);
 * Dewi (SWP-EMP-3001) on monday+2 (Wed, Pagi). monday+3 (Thu) is Dewi's approved-leave day.
 * Login shiftLeader (Rudi @ CMP-0021 — in scope for both seeded agents).
 *
 * Route /schedule, validateSearch {company_id?, week?}. Selectors anchored on the REAL
 * cell aria-label ("{{agent}} — {{date}}") and the popover (e4-helpers).
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import {
  SEED,
  apiAs,
  assignShiftViaPopover,
  cellButton,
  clearCellViaPopover,
  expectClearedToast,
  expectPublishedToast,
  markDayOffViaPopover,
  openCell,
  popover,
  selectCompany,
  waitForToken,
} from '../../lib/e4-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

const RUDI = 'Rudi Wijaya';
const DEWI = 'Dewi Lestari';

/** Open the grid for Plaza Senayan and wait for the seeded agents to render. */
async function openGrid(page: import('@playwright/test').Page): Promise<void> {
  await page.goto('/schedule');
  await selectCompany(page, 'Plaza Senayan');
  // The grid only renders rows for agents with entries this week — Rudi + Dewi.
  await expect(page.getByText(RUDI).first()).toBeVisible({ timeout: 30_000 });
  await expect(page.getByText(DEWI).first()).toBeVisible({ timeout: 15_000 });
}

// ---------------------------------------------------------------------------
// SA-1 · assign to an empty in-window cell → auto-publish
// ---------------------------------------------------------------------------

test('SA-1-assign-autopublish · empty in-window cell → pick Pagi → published toast → chip appears', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await openGrid(page);

  // Rudi's Friday cell (monday+4) is in-window and empty.
  const freeDate = SEED.freeDate();
  await openCell(page, RUDI, freeDate);
  await assignShiftViaPopover(page, 'Pagi');
  await expectPublishedToast(page);

  // The cell now carries a Pagi chip (grid refetched via invalidate).
  const cell = cellButton(page, RUDI, freeDate);
  await expect(cell.getByText('Pagi')).toBeVisible({ timeout: 15_000 });
});

// ---------------------------------------------------------------------------
// SA-2 · replace an existing cell with a different shift
// ---------------------------------------------------------------------------

test('SA-2-replace · existing Pagi cell → pick a different shift → replace (cell updates)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await openGrid(page);

  // Dewi's Wednesday cell has the seeded Pagi entry (SVC-003 placement → Malam is allowed).
  const dewiDate = SEED.dewiEntryDate();
  const cell = cellButton(page, DEWI, dewiDate);
  await expect(cell.getByText('Pagi')).toBeVisible({ timeout: 20_000 });

  await openCell(page, DEWI, dewiDate);
  await assignShiftViaPopover(page, 'Malam');
  await expectPublishedToast(page);

  // The cell chip changed from Pagi → Malam (existing-entry update path).
  await expect(cellButton(page, DEWI, dewiDate).getByText('Malam')).toBeVisible({ timeout: 15_000 });
});

// ---------------------------------------------------------------------------
// SA-4 · picker filtered by the agent's service line (untagged always included)
// ---------------------------------------------------------------------------

test('SA-4-picker-filtered-by-service-line · SVC-003 agent → popover lists Malam (SVC-003) + untagged Pagi', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await openGrid(page);

  // Dewi's placement is SVC-003 (Parking). Open her free Friday cell.
  await openCell(page, DEWI, SEED.freeDate());
  const pop = popover(page);
  await expect(pop).toBeVisible({ timeout: 10_000 });

  // Malam (tagged SVC-003) appears AND untagged Pagi appears (SM-3: untagged available to all).
  await expect(pop.locator('button', { hasText: 'Malam' }).first()).toBeVisible({ timeout: 10_000 });
  await expect(pop.locator('button', { hasText: 'Pagi' }).first()).toBeVisible({ timeout: 10_000 });
});

// ---------------------------------------------------------------------------
// SA-7 · mark a cell as day off
// ---------------------------------------------------------------------------

test('SA-7-mark-day-off · popover → "Tandai Libur (OFF)" → cell shows Libur', async ({ page }) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await openGrid(page);

  // Rudi's free Friday cell → mark day off.
  const freeDate = SEED.freeDate();
  await openCell(page, RUDI, freeDate);
  await markDayOffViaPopover(page);
  await expectPublishedToast(page);

  await expect(cellButton(page, RUDI, freeDate).getByText('Libur')).toBeVisible({ timeout: 15_000 });
});

// ---------------------------------------------------------------------------
// CH-1 · clear an existing cell (real DELETE 204)
// ---------------------------------------------------------------------------

test('CH-1-clear-cell · existing (future) cell → "Hapus jadwal" → cell empties', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await openGrid(page);

  // C-5: a leader cannot clear a PAST-dated entry (attendance references it). The seeded
  // Tue/Wed cells may be in the past depending on the weekday "today" falls on, so we first
  // create a fresh FUTURE cell (in-window) as the leader, then clear that — keeping CH-1 a
  // genuine leader-scoped delete that is immune to the current weekday.
  const futureDate = SEED.freeDate();
  await openCell(page, RUDI, futureDate);
  await assignShiftViaPopover(page, 'Pagi');
  await expectPublishedToast(page);
  await expect(cellButton(page, RUDI, futureDate).getByText('Pagi')).toBeVisible({
    timeout: 15_000,
  });

  // Now clear it via the popover (real DELETE 204).
  await openCell(page, RUDI, futureDate);
  await clearCellViaPopover(page);
  await expectClearedToast(page);

  await expect(cellButton(page, RUDI, futureDate).getByText('Pagi')).toHaveCount(0, {
    timeout: 15_000,
  });
});

// ---------------------------------------------------------------------------
// CH-2 · edit/swap an existing entry → status MODIFIED (assert via apiAs)
// ---------------------------------------------------------------------------

test('CH-2-edit-swap · changing an existing entry shift → status MODIFIED', async ({ page }) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await openGrid(page);

  // Edit Rudi's seeded Tuesday Pagi entry to Malam via the popover (existing-entry → PATCH).
  const rudiDate = SEED.rudiEntryDate();
  await expect(cellButton(page, RUDI, rudiDate).getByText('Pagi')).toBeVisible({ timeout: 20_000 });
  await openCell(page, RUDI, rudiDate);
  await assignShiftViaPopover(page, 'Malam');
  await expectPublishedToast(page);

  // The persisted entry SWP-SCH-6001 now has status MODIFIED (PATCH re-runs the engine
  // with ForceReplace=true, sets MODIFIED). Confirm via the real list API.
  const list = await apiAs(
    page,
    'GET',
    `/schedule?company_id=SWP-CMP-0021&start_date=${SEED.monday()}&end_date=${SEED.dewiLeaveDate()}`,
  );
  expect(list.status).toBe(200);
  const env = list.body as { data?: Array<{ id: string; status: string }> };
  const entry = (env.data ?? []).find((e) => e.id === 'SWP-SCH-6001');
  expect(entry?.status).toBe('MODIFIED');
});

// ---------------------------------------------------------------------------
// grid-empty · a company with no placements → empty state
// ---------------------------------------------------------------------------

test('grid-empty · company with no placements → empty state', async ({ page }) => {
  // The grid is ROSTER-driven: it renders one row per ACTIVE placement (regardless of
  // entries). Both seeded companies have placements, so to exercise the empty state we
  // create a fresh company (auto primary site, zero placements) and select it.
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/');
  await waitForToken(page);

  const emptyCompany = 'PT Kosong Jadwal E2E';
  const res = await apiAs(page, 'POST', '/client-companies', {
    name: emptyCompany,
    address: 'Jl. Kosong No. 1, Jakarta',
  });
  expect([200, 201]).toContain(res.status);

  await page.goto('/schedule');
  await selectCompany(page, emptyCompany);

  await expect(page.getByText(/Belum ada|tidak ada agen|Tidak ada agen/i).first()).toBeVisible({
    timeout: 30_000,
  });
});
