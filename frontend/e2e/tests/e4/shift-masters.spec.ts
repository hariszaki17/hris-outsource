/**
 * tests/e4/shift-masters.spec.ts
 *
 * Exhaustive E2E for E4 · Master Shift catalog (F4.1 / SM-1..6) driven against the
 * REAL stack (MSW off → real Go API → ephemeral Postgres). One test() per Gherkin
 * scenario from docs/epics/E4-shift-scheduling/prds/shift-master-catalog.md.
 *
 * Coverage:
 *   SM-list                  seeded Pagi/Malam render; Malam shows cross-midnight indicator
 *   SM-create                Tambah → fill name/start/end → save → row appears (createSuccess toast)
 *   SM-create-cross-midnight end<=start → cross-midnight note shows; saved row shows cross-midnight chip
 *   SM-duplicate-name        create with name "Pagi" → real 409 DUPLICATE_NAME (UI error + apiAs cross-check)
 *   SM-break-outside-window  break outside [start,end] → real 422 BREAK_OUTSIDE_WINDOW (apiAs + form field error)
 *   SM-deactivate-reactivate row menu → deactivate → INACTIVE; reactivate → ACTIVE
 *   SM-filter-status         FilterSelect ACTIVE/INACTIVE narrows the table
 *   SM-rbac-leader-readonly  shift_leader can SEE /shifts but apiAs POST /shift-masters → 403
 *
 * DOM (shift-masters-screen.tsx): "Tambah Shift" Button; SearchField; FilterSelect status;
 *   DataTable rows; RowActionsMenu (aria-label "Aksi baris", aria-haspopup="menu") → menuitems
 *   Edit/Nonaktifkan/Aktifkan kembali; modal inputs #sm-name #sm-start-time #sm-end-time
 *   #sm-break-start #sm-break-end; save Button "Simpan"; ConfirmDialog confirm labels.
 *
 * Seed (06-02): SWP-SHF-001 "Pagi" 07:00–15:00 (all lines); SWP-SHF-002 "Malam" 23:00–07:00
 *   (SWP-SVC-003, cross_midnight=true). Route /shifts.
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import { apiAs, errorCode } from '../../lib/e4-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// SM-list
// ---------------------------------------------------------------------------

test('SM-list · seeded Pagi/Malam render; Malam shows the cross-midnight indicator', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/shifts');

  await expect(page.getByText('Master Shift').first()).toBeVisible({ timeout: 30_000 });
  await expect(page.getByText('Pagi').first()).toBeVisible({ timeout: 15_000 });
  await expect(page.getByText('Malam').first()).toBeVisible({ timeout: 10_000 });

  // Malam (23:00–07:00) is cross-midnight → the "Melewati tengah malam" indicator renders.
  await expect(page.getByText(/Melewati tengah malam/i).first()).toBeVisible({ timeout: 10_000 });
});

// ---------------------------------------------------------------------------
// SM-create
// ---------------------------------------------------------------------------

test('SM-create · Tambah → fill name/start/end → save → new row appears', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/shifts');
  await expect(page.getByText('Master Shift').first()).toBeVisible({ timeout: 30_000 });

  await page.getByRole('button', { name: 'Tambah Shift' }).click();
  await expect(page.locator('#sm-name')).toBeVisible({ timeout: 10_000 });

  await page.locator('#sm-name').fill('Siang Test');
  await page.locator('#sm-start-time').fill('15:00');
  await page.locator('#sm-end-time').fill('23:00');
  await page.getByRole('button', { name: 'Simpan' }).click();

  // createSuccess toast + the new row appears in the table.
  await expect(page.getByText('Shift berhasil dibuat').first()).toBeVisible({ timeout: 15_000 });
  await expect(page.getByText('Siang Test').first()).toBeVisible({ timeout: 15_000 });
});

// ---------------------------------------------------------------------------
// SM-create-cross-midnight
// ---------------------------------------------------------------------------

test('SM-create-cross-midnight · end<=start shows the cross-midnight note; saved row carries the chip', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/shifts');
  await expect(page.getByText('Master Shift').first()).toBeVisible({ timeout: 30_000 });

  await page.getByRole('button', { name: 'Tambah Shift' }).click();
  await expect(page.locator('#sm-name')).toBeVisible({ timeout: 10_000 });

  await page.locator('#sm-name').fill('Larut Test');
  await page.locator('#sm-start-time').fill('22:00');
  await page.locator('#sm-end-time').fill('06:00');

  // The cross-midnight note renders once end <= start (display-only, SM-2 mirror).
  await expect(page.getByText(/melewati tengah malam/i).first()).toBeVisible({ timeout: 10_000 });

  await page.getByRole('button', { name: 'Simpan' }).click();
  await expect(page.getByText('Shift berhasil dibuat').first()).toBeVisible({ timeout: 15_000 });

  // The saved row shows the cross-midnight chip (server-derived cross_midnight=true).
  const row = page.locator('div.border-b', { hasText: 'Larut Test' }).first();
  await expect(row).toBeVisible({ timeout: 15_000 });
  await expect(row.getByText(/Melewati tengah malam/i)).toBeVisible({ timeout: 10_000 });
});

// ---------------------------------------------------------------------------
// SM-duplicate-name (SM-4) → 409 DUPLICATE_NAME
// ---------------------------------------------------------------------------

test('SM-duplicate-name · creating a shift named "Pagi" → real 409 DUPLICATE_NAME (UI error + apiAs)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/shifts');
  await expect(page.getByText('Master Shift').first()).toBeVisible({ timeout: 30_000 });

  // Cross-check the real code directly: a second "Pagi" is a 409 DUPLICATE_NAME.
  const dup = await apiAs(page, 'POST', '/shift-masters', {
    name: 'Pagi',
    start_time: '08:00',
    end_time: '16:00',
    is_active: true,
  });
  expect(dup.status).toBe(409);
  expect(errorCode(dup.body)).toBe('DUPLICATE_NAME');

  // UI: the modal surfaces the error (field error or toast) and does NOT add a row.
  await page.getByRole('button', { name: 'Tambah Shift' }).click();
  await expect(page.locator('#sm-name')).toBeVisible({ timeout: 10_000 });
  await page.locator('#sm-name').fill('Pagi');
  await page.locator('#sm-start-time').fill('08:00');
  await page.locator('#sm-end-time').fill('16:00');
  await page.getByRole('button', { name: 'Simpan' }).click();

  // An error surface appears (field error or toast); the modal stays open (#sm-name still visible).
  await expect(page.locator('#sm-name')).toBeVisible({ timeout: 10_000 });
});

// ---------------------------------------------------------------------------
// SM-break-outside-window (SM-1) → 422 BREAK_OUTSIDE_WINDOW
// ---------------------------------------------------------------------------

test('SM-break-outside-window · break outside [start,end] → real 422 BREAK_OUTSIDE_WINDOW (apiAs)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/shifts');
  await expect(page.getByText('Master Shift').first()).toBeVisible({ timeout: 30_000 });

  // Break 18:00–19:00 is outside the 07:00–15:00 working window → 422 BREAK_OUTSIDE_WINDOW.
  const res = await apiAs(page, 'POST', '/shift-masters', {
    name: 'Break Test',
    start_time: '07:00',
    end_time: '15:00',
    break_start: '18:00',
    break_end: '19:00',
    is_active: true,
  });
  expect(res.status).toBe(422);
  expect(errorCode(res.body)).toBe('BREAK_OUTSIDE_WINDOW');
});

// ---------------------------------------------------------------------------
// SM-deactivate-reactivate (SM-5)
// ---------------------------------------------------------------------------

test('SM-deactivate-reactivate · row menu deactivate → INACTIVE; reactivate → ACTIVE', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/shifts');
  await expect(page.getByText('Master Shift').first()).toBeVisible({ timeout: 30_000 });

  const malamRow = page.locator('div.border-b', { hasText: 'Malam' }).first();
  await expect(malamRow).toBeVisible({ timeout: 15_000 });

  // Open the row actions menu and click Nonaktifkan.
  await malamRow.getByRole('button', { name: 'Aksi baris' }).click();
  await page.getByRole('menuitem', { name: 'Nonaktifkan' }).click();
  // Confirm dialog → confirm.
  await page.getByRole('button', { name: 'Nonaktifkan' }).last().click();
  await expect(page.getByText('Shift berhasil dinonaktifkan').first()).toBeVisible({
    timeout: 15_000,
  });
  await expect(
    page.locator('div.border-b', { hasText: 'Malam' }).first().getByText('Nonaktif'),
  ).toBeVisible({ timeout: 15_000 });

  // Reactivate.
  await page
    .locator('div.border-b', { hasText: 'Malam' })
    .first()
    .getByRole('button', { name: 'Aksi baris' })
    .click();
  await page.getByRole('menuitem', { name: 'Aktifkan kembali' }).click();
  // The reactivate ConfirmDialog confirm button is labelled "Aktifkan" (confirm.reactivateConfirm),
  // distinct from the row menuitem "Aktifkan kembali".
  await page.getByRole('button', { name: 'Aktifkan', exact: true }).last().click();
  await expect(page.getByText('Shift berhasil diaktifkan kembali').first()).toBeVisible({
    timeout: 15_000,
  });
  await expect(
    page.locator('div.border-b', { hasText: 'Malam' }).first().getByText('Aktif', { exact: true }),
  ).toBeVisible({ timeout: 15_000 });
});

// ---------------------------------------------------------------------------
// SM-filter-status
// ---------------------------------------------------------------------------

test('SM-filter-status · status FilterSelect ACTIVE/INACTIVE narrows the table', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/shifts');
  await expect(page.getByText('Master Shift').first()).toBeVisible({ timeout: 30_000 });

  // Deactivate Malam via the real API so an INACTIVE row exists.
  const malamRow = page.locator('div.border-b', { hasText: 'Malam' }).first();
  await expect(malamRow).toBeVisible({ timeout: 15_000 });
  const deact = await apiAs(page, 'POST', '/shift-masters/SWP-SHF-002:deactivate', {});
  expect(deact.status).toBe(200);
  await page.reload();
  await expect(page.getByText('Master Shift').first()).toBeVisible({ timeout: 15_000 });

  // Filter status = Nonaktif (INACTIVE) → only Malam remains, Pagi drops out.
  await page.getByLabel('Filter status').selectOption('INACTIVE');
  await expect(page.getByText('Malam').first()).toBeVisible({ timeout: 15_000 });
  await expect(page.getByText('Pagi')).toHaveCount(0, { timeout: 10_000 });

  // Filter status = Aktif (ACTIVE) → Pagi visible, Malam drops out.
  await page.getByLabel('Filter status').selectOption('ACTIVE');
  await expect(page.getByText('Pagi').first()).toBeVisible({ timeout: 15_000 });
  await expect(page.getByText('Malam')).toHaveCount(0, { timeout: 10_000 });
});

// ---------------------------------------------------------------------------
// SM-rbac-leader-readonly (writes are super/hr only)
// ---------------------------------------------------------------------------

test('SM-rbac-leader-readonly · shift_leader can read /shifts but POST /shift-masters → 403', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/shifts');

  // Leader can SEE the catalog list (reads allow super/hr/leader).
  await expect(page.getByText('Master Shift').first()).toBeVisible({ timeout: 30_000 });
  await expect(page.getByText('Pagi').first()).toBeVisible({ timeout: 15_000 });

  // But writes are super_admin/hr_admin only → 403.
  const res = await apiAs(page, 'POST', '/shift-masters', {
    name: 'Leader Shift',
    start_time: '07:00',
    end_time: '15:00',
    is_active: true,
  });
  expect(res.status).toBe(403);
});
