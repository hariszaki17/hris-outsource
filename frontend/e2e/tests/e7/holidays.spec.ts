/**
 * tests/e7/holidays.spec.ts
 *
 * E7 · holiday-calendar CRUD (OVT-02) against the REAL stack, driving the REAL
 * overtime-rules-screen ("Kalender Hari Libur" pane) + holiday-overlays (HolidayFormModal
 * + DeleteHolidayConfirm). HR persona, /overtime/aturan. Each scenario is its own test().
 *
 * Coverage:
 *   HOL-create        "+" → HolidayFormModal → fill name/date/category → "Simpan" → toast +
 *                     the new holiday appears in the calendar list.
 *   HOL-clash         create on SWP-HOL-9001's date+category → 409 HOLIDAY_DATE_CLASH (apiAs).
 *   HOL-update        edit SWP-HOL-9002 name → 200 + the list reflects the new name.
 *   HOL-delete-free   delete SWP-HOL-9002 → 204 + removed from the list.
 *   HOL-delete-inuse  DeleteHolidayConfirm on SWP-HOL-9001 (in_use_by_overtime) → confirm
 *                     DISABLED + the in-use Banner; apiAs DELETE → 409 HOLIDAY_IN_USE.
 *
 * Seed (09-02): SWP-HOL-9001 NATIONAL @ monday-14, referenced by SWP-OT-30009 (APPROVED) →
 * in_use (delete blocked); SWP-HOL-9002 NATIONAL @ monday+21, free (deletable).
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import {
  HOL,
  OT_BTN,
  apiAs,
  errorCode,
  getHoliday,
  holidayRow,
  inUseHolidayDate,
  mondayPlus,
  waitForToken,
} from '../../lib/e7-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

async function openRules(page: import('@playwright/test').Page): Promise<void> {
  // Direct navigation to a deep authed route can land on /login when the refresh-token
  // rotation race leaves tryRestoreSession unauthenticated (the cookie was consumed by a
  // prior refresh). Retry the goto until the rules route renders (re-login if bounced).
  for (let attempt = 0; attempt < 3; attempt++) {
    await page.goto('/overtime/aturan');
    if (page.url().includes('/login')) {
      await loginAs(page, PERSONAS.hrAdmin);
      continue;
    }
    const header = page.getByText(/Kalender Hari Libur/).first();
    try {
      await expect(header).toBeVisible({ timeout: 20_000 });
      await waitForToken(page);
      return;
    } catch {
      // Fell through to /login mid-load, or a transient — retry.
      if (!page.url().includes('/login')) throw new Error('rules route did not render');
      await loginAs(page, PERSONAS.hrAdmin);
    }
  }
  throw new Error('openRules: could not reach /overtime/aturan');
}

// ---------------------------------------------------------------------------
// HOL-create — add a holiday through the modal → appears in the list
// ---------------------------------------------------------------------------

test('HOL-create · "+" → HolidayFormModal → Simpan → toast + new holiday in the calendar', async ({
  page,
}) => {
  await openRules(page);

  // A free, clearly-in-range date that collides with NO seeded holiday/OT.
  const freeDate = mondayPlus(35);
  const name = 'Hari Libur Uji E2E';

  await page.getByRole('button', { name: 'Tambah Hari Libur', exact: true }).click();
  await page.locator('#holiday-name').fill(name);
  await page.locator('#holiday-date').fill(freeDate);
  await page.locator('#holiday-category').selectOption('NATIONAL');
  await page.getByRole('button', { name: OT_BTN.save, exact: true }).click();

  // Success toast (t('holidays.toastCreated')).
  await expect(page.getByText('Hari libur ditambahkan').first()).toBeVisible({ timeout: 15_000 });
  // The new holiday appears in the "Kalender Hari Libur" list.
  await expect(holidayRow(page, name)).toBeVisible({ timeout: 15_000 });
});

// ---------------------------------------------------------------------------
// HOL-clash — create on an existing date+category → 409 HOLIDAY_DATE_CLASH
// ---------------------------------------------------------------------------

test('HOL-clash · create on SWP-HOL-9001 date+category → 409 HOLIDAY_DATE_CLASH', async ({
  page,
}) => {
  await openRules(page);

  const res = await apiAs(page, 'POST', '/holidays', {
    name: 'Duplikat Hari Libur',
    date: inUseHolidayDate(), // SWP-HOL-9001's seeded date
    category: 'NATIONAL',
    recurring: false,
  });
  expect(res.status).toBe(409);
  expect(errorCode(res.body)).toBe('HOLIDAY_DATE_CLASH');
});

// ---------------------------------------------------------------------------
// HOL-update — edit the free holiday's name → 200 + list reflects
// ---------------------------------------------------------------------------

test('HOL-update · edit SWP-HOL-9002 name → 200 + list reflects the new name', async ({ page }) => {
  await openRules(page);

  const newName = 'Hari Libur Bebas (diubah)';
  const res = await apiAs(page, 'PATCH', `/holidays/${HOL.free}`, {
    name: newName,
    date: mondayPlus(21),
    category: 'NATIONAL',
    recurring: false,
  });
  expect(res.status).toBe(200);

  const updated = await getHoliday(page, HOL.free);
  expect(updated?.name).toBe(newName);
});

// ---------------------------------------------------------------------------
// HOL-delete-free — delete the free holiday → 204 + removed
// ---------------------------------------------------------------------------

test('HOL-delete-free · delete SWP-HOL-9002 (free) → 204 + removed from list', async ({ page }) => {
  await openRules(page);

  const res = await apiAs(page, 'DELETE', `/holidays/${HOL.free}`);
  expect([200, 204]).toContain(res.status);

  // The free holiday is gone from the calendar.
  expect(await getHoliday(page, HOL.free)).toBeUndefined();
});

// ---------------------------------------------------------------------------
// HOL-delete-inuse — the in-use holiday's delete is blocked (UI + 409)
// ---------------------------------------------------------------------------

test('HOL-delete-inuse · DeleteHolidayConfirm on SWP-HOL-9001 → confirm disabled + Banner; DELETE → 409 HOLIDAY_IN_USE', async ({
  page,
}) => {
  await openRules(page);

  // SWP-HOL-9001 ("...terpakai") is referenced by SWP-OT-30009 (APPROVED) → in_use.
  const row = holidayRow(page, 'terpakai');
  await expect(row).toBeVisible({ timeout: 15_000 });
  await row.hover();
  await row.getByRole('button', { name: 'Hapus Hari Libur', exact: true }).click();

  // The in-use Banner shows and the confirm "Hapus" button is disabled.
  await expect(page.getByText(/dipakai oleh lembur/).first()).toBeVisible({ timeout: 10_000 });
  await expect(page.getByRole('button', { name: 'Hapus', exact: true })).toBeDisabled();

  // The real DELETE is rejected with 409 HOLIDAY_IN_USE.
  const res = await apiAs(page, 'DELETE', `/holidays/${HOL.inUse}`);
  expect(res.status).toBe(409);
  expect(errorCode(res.body)).toBe('HOLIDAY_IN_USE');
});
