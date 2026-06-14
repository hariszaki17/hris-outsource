/**
 * tests/e6/calendar.spec.ts
 *
 * E6 · leave calendar (LVE-03) against the REAL stack, driving the REAL
 * leave-calendar-screen. The seeded current-week PENDING entries (8007 Wed,
 * 8002 Thu — both Dewi @ CMP-0021) drive the show_pending toggle.
 *
 * The grid only renders when the month has at least one VISIBLE entry; with
 * show_pending OFF the current month (which holds only PENDING current-week
 * entries) is empty → the EmptyState shows. Toggling show_pending ON makes the
 * PENDING entries visible → the month grid + the Dewi day cell render.
 *
 * Coverage:
 *   EMPTY-default   HR /leave/calendar (show_pending OFF) → "Tidak ada cuti bulan ini".
 *   PENDING-toggle  toggling the role=switch ON → the grid + Dewi day cell appear.
 *
 * Seed (08-02): 8002 Dewi PENDING_HR (Thu, monday+3); 8007 Dewi PENDING_HR (Wed, monday+2);
 * both @ CMP-0021 in the current week.
 */

import { waitForToken } from '../../lib/e6-helpers.js';
import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

const DEWI = 'Dewi Lestari';

// ---------------------------------------------------------------------------
// EMPTY-default — show_pending OFF, no approved entries this month → empty state
// ---------------------------------------------------------------------------

test('EMPTY-default · HR /leave/calendar with show_pending OFF shows the empty state for a month with no leave', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/leave/calendar');
  await waitForToken(page);

  // The current month now carries an APPROVED back-history entry (SWP-LR-8005,
  // Dewi ANNUAL on the prior-approved date), so it is no longer empty. The
  // ledger backfill keeps all seeded leave inside the current month ± ~2 months;
  // navigate two months forward to a month with no seeded leave → with
  // show_pending OFF and no approved entry, the API returns zero entries →
  // EmptyState renders.
  const nextMonth = page.getByRole('button', { name: 'Bulan berikutnya' });
  await expect(nextMonth).toBeVisible({ timeout: 30_000 });
  await nextMonth.click();
  await nextMonth.click();

  await expect(page.getByText('Tidak ada cuti bulan ini').first()).toBeVisible({ timeout: 30_000 });
});

// ---------------------------------------------------------------------------
// PENDING-toggle — toggling show_pending ON reveals the grid + Dewi day cell
// ---------------------------------------------------------------------------

test('PENDING-toggle · toggling show_pending ON renders the month grid + the Dewi pending cell', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/leave/calendar');
  await waitForToken(page);

  const toggle = page.getByRole('switch', { name: 'Tampilkan entri cuti pending' });
  await expect(toggle).toBeVisible({ timeout: 30_000 });

  // OFF: the grid renders (the current month carries the APPROVED back-history
  // entry SWP-LR-8005, Dewi ANNUAL on the prior-approved date) so the weekday
  // header shows. The current-week PENDING entries (8001/8002/8007 — Dewi) are
  // hidden while show_pending is OFF. Capture the OFF Dewi-cell count.
  await expect(page.getByText('Sen', { exact: true }).first()).toBeVisible({ timeout: 15_000 });
  const dewiCells = page.getByText(DEWI);
  await expect(dewiCells.first()).toBeVisible({ timeout: 15_000 });
  const offCount = await dewiCells.count();

  // Toggle ON → the current-week PENDING Dewi entries (8007 Wed, 8002 Thu, 8001
  // Fri) surface in addition to any approved cells → strictly MORE Dewi cells.
  await toggle.click();
  await expect.poll(() => dewiCells.count(), { timeout: 15_000 }).toBeGreaterThan(offCount);
});
