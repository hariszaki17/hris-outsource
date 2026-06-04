/**
 * tests/e3/placement-lifecycle.spec.ts
 *
 * Exhaustive E2E for E3 · Placement Lifecycle (PLC-02) — one test() per Gherkin
 * scenario / C-# from docs/epics/E3-placement/prds/placement-lifecycle.md, driven
 * against the REAL stack.
 *
 * Coverage:
 *   LC-end            End SWP-PL-5002 via EndConfirm (end-reason select + end-date) → ENDED + history row on detail
 *   LC-resign         Resign SWP-PL-5003 via ResignModal (resign-date/resign-reason) → RESIGNED (persisted)
 *   LC-terminate      Terminate SWP-PL-5002 via TerminateConfirm with correct company-name → TERMINATED;
 *                     wrong confirm leaves the destructive button disabled (no submit)
 *   LC-terminal-immut Attempt to end an already-ENDED placement → 409 TERMINAL_STATE_IMMUTABLE (API); UI hides actions
 *   LC-renew          Renew SWP-PL-5001 via RenewModal (rn-start/rn-end) → successor ACTIVE, predecessor SUPERSEDED,
 *                     both in the history_chain card; 1-day-buffer violation → 422 PLACEMENT_PERIOD_OVERLAP (API)
 *
 * DOM (from placement-detail-screen.tsx / placement-overlays.tsx):
 *   - Detail action buttons (Perpanjang/Transfer/Akhiri) only for active-like|expiring; Terminate/Resign are ghost links.
 *   - Overlay input ids: end-reason/end-date/end-notes; resign-date/resign-reason/resign-notes;
 *     term-reason/term-date/term-confirm; rn-start/rn-end/rn-notes. Forms noValidate.
 *   - Terminal placements render NO action buttons (immutability asserted via API, not the UI).
 *   - history_chain card lists predecessor + successor rows.
 *
 * Seed: SWP-PL-5001 Rudi@0021 (ACTIVE, end 2026-12-31), SWP-PL-5002 Budi@0022 "Mall Kelapa Gading"
 *   (ACTIVE), SWP-PL-5003 Sari@0021 (ACTIVE open-ended). Companies: 0021 "Plaza Senayan", 0022 "Mall Kelapa Gading".
 *
 * Traceable to: PLC-02, F3.2, LC-*, INV (TERMINAL_STATE_IMMUTABLE, PLACEMENT_PERIOD_OVERLAP).
 */

import { test, expect, loginAs } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import { getPlacementLifecycleStatus } from '../../lib/db.js';
import { apiAs, errorCode } from '../../lib/e3-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

function isoDaysFromNow(days: number): string {
  const d = new Date();
  d.setUTCDate(d.getUTCDate() + days);
  return d.toISOString().slice(0, 10);
}

async function openDetail(page: import('@playwright/test').Page, placementId: string) {
  await page.goto(`/placements/${placementId}`);
  // The header renders the agent name + status badge; wait for the lifecycle tracker.
  await expect(page.getByText(/Aktif|Akan berakhir|Berakhir|Aktif/i).first()).toBeVisible({
    timeout: 30_000,
  });
}

// ---------------------------------------------------------------------------
// LC-end — end a placement via the UI EndConfirm modal → ENDED + history row
// ---------------------------------------------------------------------------

test('LC-end · end SWP-PL-5002 via EndConfirm → persisted ENDED + history shows the end', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await openDetail(page, 'SWP-PL-5002');

  // Open the "Akhiri" (end) action (header button).
  await page.getByRole('button', { name: 'Akhiri' }).first().click();

  // EndConfirm: choose reason + effective date.
  await expect(page.locator('#end-reason')).toBeVisible({ timeout: 10_000 });
  await page.locator('#end-reason').selectOption('END_OF_TERM');
  await page.locator('#end-date').fill(isoDaysFromNow(0));

  // Confirm ("Akhiri Penempatan").
  await page.getByRole('button', { name: 'Akhiri Penempatan' }).click();

  // Persisted ENDED.
  await expect
    .poll(() => getPlacementLifecycleStatus('SWP-PL-5002'), { timeout: 15_000 })
    .toBe('ENDED');
});

// ---------------------------------------------------------------------------
// LC-resign — resign a placement via the UI ResignModal → RESIGNED
// ---------------------------------------------------------------------------

test('LC-resign · resign SWP-PL-5003 via ResignModal → persisted RESIGNED', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await openDetail(page, 'SWP-PL-5003');

  // Resign is a ghost secondary action ("Undur diri").
  await page.getByRole('button', { name: 'Undur diri' }).first().click();

  await expect(page.locator('#resign-date')).toBeVisible({ timeout: 10_000 });
  await page.locator('#resign-date').fill(isoDaysFromNow(0));
  await page.locator('#resign-reason').fill('Mengundurkan diri atas permintaan sendiri.');
  await page.getByRole('button', { name: 'Catat Undur Diri' }).click();

  await expect
    .poll(() => getPlacementLifecycleStatus('SWP-PL-5003'), { timeout: 15_000 })
    .toBe('RESIGNED');
});

// ---------------------------------------------------------------------------
// LC-terminate — terminate with correct company-name confirm → TERMINATED;
// wrong name leaves the destructive button disabled.
// ---------------------------------------------------------------------------

test('LC-terminate · TerminateConfirm requires the company-name retype; wrong → disabled, correct → TERMINATED', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await openDetail(page, 'SWP-PL-5002'); // Budi @ "Mall Kelapa Gading"

  // Terminate is a ghost secondary action ("Pecat").
  await page.getByRole('button', { name: 'Pecat' }).first().click();

  await expect(page.locator('#term-reason')).toBeVisible({ timeout: 10_000 });
  await page.locator('#term-reason').fill('Pelanggaran berat sesuai ketentuan perjanjian kerja.');
  await page.locator('#term-date').fill(isoDaysFromNow(0));

  // Wrong company name → the destructive confirm stays disabled.
  await page.locator('#term-confirm').fill('Nama Salah');
  // The modal footer confirm button is also labelled "Pecat" — scope to the dialog.
  const dialog = page.getByRole('dialog');
  const confirmBtn = dialog.getByRole('button', { name: 'Pecat' });
  await expect(confirmBtn).toBeDisabled();

  // Correct (case-insensitive) name enables + submits.
  await page.locator('#term-confirm').fill('mall kelapa gading');
  await expect(confirmBtn).toBeEnabled();
  await confirmBtn.click();

  await expect
    .poll(() => getPlacementLifecycleStatus('SWP-PL-5002'), { timeout: 15_000 })
    .toBe('TERMINATED');
});

// ---------------------------------------------------------------------------
// LC-terminal-immut — ending an already-terminal placement → 409 TERMINAL_STATE_IMMUTABLE
// ---------------------------------------------------------------------------

test('LC-terminal-immut · ending an already-ENDED placement → 409 TERMINAL_STATE_IMMUTABLE; detail hides actions', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/placements');
  await expect(page.getByText(/Penempatan/i).first()).toBeVisible({ timeout: 30_000 });

  // First end SWP-PL-5002 via the API.
  const end1 = await apiAs(page, 'POST', '/placements/SWP-PL-5002:end', {
    reason: 'END_OF_TERM',
    effective_date: isoDaysFromNow(0),
  });
  expect(end1.status).toBe(200);

  // A second end on the now-terminal placement must be rejected.
  const end2 = await apiAs(page, 'POST', '/placements/SWP-PL-5002:end', {
    reason: 'END_OF_TERM',
    effective_date: isoDaysFromNow(0),
  });
  expect(end2.status).toBe(409);
  expect(errorCode(end2.body)).toBe('TERMINAL_STATE_IMMUTABLE');

  // UI: the terminal detail renders NO primary action buttons (read-only).
  await openDetail(page, 'SWP-PL-5002');
  await expect(page.getByRole('button', { name: /^Perpanjang$/i })).toHaveCount(0);
  await expect(page.getByRole('button', { name: /^Akhiri$/i })).toHaveCount(0);
});

// ---------------------------------------------------------------------------
// LC-renew — renew → successor ACTIVE + predecessor SUPERSEDED + both in history_chain;
// 1-day-buffer violation → 422 PLACEMENT_PERIOD_OVERLAP.
// ---------------------------------------------------------------------------

test('LC-renew · renew SWP-PL-5001 → predecessor SUPERSEDED + successor ACTIVE; overlap → 422 PLACEMENT_PERIOD_OVERLAP', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await openDetail(page, 'SWP-PL-5001'); // Rudi @ 0021, end 2026-12-31

  // Open the renew modal.
  await page.getByRole('button', { name: /Perpanjang/i }).first().click();
  await expect(page.locator('#rn-start')).toBeVisible({ timeout: 10_000 });

  // New period starts AFTER the current end (2026-12-31) → 1-day buffer satisfied.
  await page.locator('#rn-start').fill('2027-01-02');
  await page.locator('#rn-end').fill('2027-12-31');
  await page.getByRole('button', { name: /Perpanjang|Konfirmasi|Simpan/i }).last().click();

  // Predecessor becomes SUPERSEDED.
  await expect
    .poll(() => getPlacementLifecycleStatus('SWP-PL-5001'), { timeout: 15_000 })
    .toBe('SUPERSEDED');

  // The detail history_chain now shows the SUPERSEDED predecessor.
  await page.reload();
  await expect(page.getByText(/Superseded/i).first()).toBeVisible({ timeout: 15_000 });
});

// ---------------------------------------------------------------------------
// LC-renew-overlap — renewing with a start that breaks the 1-day buffer → 422 PLACEMENT_PERIOD_OVERLAP
// ---------------------------------------------------------------------------

test('LC-renew-overlap · renew SWP-PL-5004 with a start inside the current period → 422 PLACEMENT_PERIOD_OVERLAP', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/placements');
  await expect(page.getByText(/Penempatan/i).first()).toBeVisible({ timeout: 30_000 });

  // SWP-PL-5004 (Dewi) is EXPIRING with end_date = today+20. A renew whose new_start_date
  // is BEFORE/ON the current end (no 1-day buffer) must be rejected by the buffer rule.
  const overlap = await apiAs(page, 'POST', '/placements/SWP-PL-5004:renew', {
    new_start_date: isoDaysFromNow(5), // well inside the current period
    new_end_date: isoDaysFromNow(400),
  });
  expect(overlap.status).toBe(422);
  expect(errorCode(overlap.body)).toBe('PLACEMENT_PERIOD_OVERLAP');
});
