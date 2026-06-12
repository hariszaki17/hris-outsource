/**
 * tests/e2/service-lines-positions.spec.ts
 *
 * Exhaustive E2E suite for E2 Service Lines + Positions — one test() per Gherkin
 * scenario/case from docs/epics/E2-identity/prds/service-lines-positions.md §7 + §8.
 *
 * Coverage:
 *   SP-1a  List shows the 3 seeded service lines with position counts
 *   SP-1b  super_admin creates a service line → row appears
 *   SP-1c  hr_admin denied create service line → 403 / no add button (super_admin-only)
 *   SP-2   Rename service line → updated name visible
 *   SP-3a  Discontinue service line with active positions → SERVICE_LINE_IN_USE error (Parking)
 *   SP-3b  Discontinue service line with no positions → status INACTIVE
 *   SP-4a  Create position under service line → row appears
 *   SP-4b  Duplicate position name in same line → POSITION_IN_USE error
 *   SP-4c  Update position name → updated value visible
 *   SP-4d  Soft-delete position → position removed from active list (getPositionStatus === 'inactive')
 *
 * Seeded IDs from 03-03: SWP-SVC-001 (Facility Services), SWP-SVC-002 (Building Management),
 * SWP-SVC-003 (Parking), SWP-POS-014 (Petugas Parkir), SWP-POS-015 (Koordinator Lokasi).
 *
 * Stack: real Vite dev server (:4173, MSW off) ↔ real Go API (:8081) ↔ ephemeral Postgres (:5433).
 * Isolation: resetDb() in beforeEach.
 * Traceable to: SP-1..4, F2.x, INV-3, e2e-harness-spec.md §Coverage.
 */

import { test, expect, loginAs } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import {
  getServiceLineStatus,
  getPositionStatus,
} from '../../lib/db.js';

// Seeded IDs from 03-03 seed.
const PARKING_SVC_ID = 'SWP-SVC-003';

// ---------------------------------------------------------------------------
// Isolation
// ---------------------------------------------------------------------------
test.beforeEach(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// SP-1a — List shows 3 seeded service lines
// ---------------------------------------------------------------------------

test('SP-1a · service lines list shows Facility Services, Building Management, Parking', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/service-lines');

  await expect(page.getByText('Facility Services').first()).toBeVisible({ timeout: 30_000 });
  await expect(page.getByText('Building Management').first()).toBeVisible();
  await expect(page.getByText('Parking').first()).toBeVisible();

  // Page heading.
  await expect(page.getByRole('heading', { name: 'Lini Layanan' })).toBeVisible();
});

// ---------------------------------------------------------------------------
// SP-1b — super_admin creates a service line → row appears
// ---------------------------------------------------------------------------

test('SP-1b · super_admin creates service line and row appears in list', async ({ page }) => {
  await loginAs(page, PERSONAS.superAdmin);
  await page.goto('/service-lines');

  await expect(page.getByRole('heading', { name: 'Lini Layanan' })).toBeVisible({ timeout: 30_000 });

  // Click "Tambah Lini Layanan".
  await page.getByRole('button', { name: 'Tambah Lini Layanan' }).click();

  // Modal opens.
  await expect(page.locator('#sl-name')).toBeVisible({ timeout: 10_000 });
  await page.locator('#sl-name').fill('Security Services E2E');
  await page.getByRole('button', { name: 'Simpan' }).last().click();

  // Row should appear in the list (save closes modal and list refetches).
  await expect(page.getByText('Security Services E2E').first()).toBeVisible({ timeout: 15_000 });
});

// ---------------------------------------------------------------------------
// SP-1c — hr_admin: "Tambah Lini Layanan" action is restricted to super_admin
// ---------------------------------------------------------------------------

test('SP-1c · hr_admin cannot create service line (super_admin-only SP-1)', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/service-lines');

  await expect(page.getByText('Facility Services')).toBeVisible({ timeout: 30_000 });

  // The "Tambah Lini Layanan" button must NOT be visible for hr_admin, OR clicking it
  // must result in a 403 error. The RBAC check is server-side; client may hide the button.
  // We verify at least one of: button absent, or an error if triggered.
  const addBtn = page.getByRole('button', { name: 'Tambah Lini Layanan' });
  const btnVisible = await addBtn.isVisible().catch(() => false);

  if (btnVisible) {
    // If button is visible (client doesn't guard), clicking it should result in a 403 error.
    await addBtn.click();
    await page.locator('#sl-name').fill('Should Fail SL');
    await page.getByRole('button', { name: 'Simpan' }).click();
    // A forbidden/error message must appear.
    await expect(
      page
        .getByText(/tidak.*izin|forbidden|gagal|403/i)
        .first(),
    ).toBeVisible({ timeout: 15_000 });
  } else {
    // Button is hidden — RBAC enforced at client level. Pass.
    expect(btnVisible).toBe(false);
  }
});

// ---------------------------------------------------------------------------
// SP-2 — Rename service line → updated name visible
// ---------------------------------------------------------------------------

test('SP-2 · rename service line shows updated name', async ({ page }) => {
  await loginAs(page, PERSONAS.superAdmin);
  // Rename lives on the detail hub (the list "Edit" item just navigates here).
  await page.goto('/service-lines/SWP-SVC-001');

  // Detail header shows the current name.
  await expect(page.getByRole('heading', { name: 'Facility Services', exact: true })).toBeVisible({
    timeout: 30_000,
  });

  // Open the header "Edit" modal (EditServiceLineModal, field #sl-edit-name).
  await page.getByRole('button', { name: 'Edit' }).click();
  await expect(page.locator('#sl-edit-name')).toBeVisible({ timeout: 10_000 });
  await page.locator('#sl-edit-name').fill('Facility Services (Renamed)');
  await page.getByRole('button', { name: 'Simpan' }).last().click();

  // Updated name appears in the detail header after refetch.
  await expect(
    page.getByRole('heading', { name: 'Facility Services (Renamed)', exact: true }),
  ).toBeVisible({ timeout: 15_000 });
});

// ---------------------------------------------------------------------------
// SP-3a — Discontinue service line with active positions → SERVICE_LINE_IN_USE
// ---------------------------------------------------------------------------

test('SP-3a · discontinue Parking (has positions) shows SERVICE_LINE_IN_USE error', async ({ page }) => {
  await loginAs(page, PERSONAS.superAdmin);
  await page.goto('/service-lines');

  // Wait for list to load — check that Parking row is visible.
  await expect(page.getByRole('heading', { name: 'Lini Layanan' })).toBeVisible({ timeout: 30_000 });
  await expect(page.getByText('Parking').first()).toBeVisible({ timeout: 10_000 });

  // Open kebab for Parking (has 2 seeded positions). The trigger is the row's
  // popup-menu button (aria-haspopup="menu").
  const parkingRow = page.locator('div.border-b').filter({ hasText: 'Parking' });
  await parkingRow.locator('button[aria-haspopup="menu"]').click();

  await page.getByRole('menuitem', { name: 'Nonaktifkan' }).click();

  // Confirm dialog.
  await expect(page.getByText('Nonaktifkan Lini Layanan?')).toBeVisible({ timeout: 5_000 });
  await page.getByRole('button', { name: 'Ya, Nonaktifkan' }).click();

  // BE should reject with 409 SERVICE_LINE_IN_USE → UI shows error toast.
  // classifyError maps 409 → t('errors.conflict') = 'Terjadi konflik dengan kondisi saat ini.'
  await expect(
    page
      .getByText(/konflik|posisi aktif|gagal/i)
      .first(),
  ).toBeVisible({ timeout: 15_000 });

  // Status should still be ACTIVE (not changed).
  const status = await getServiceLineStatus(PARKING_SVC_ID);
  expect(status).toBe('active');
});

// ---------------------------------------------------------------------------
// SP-3b — Discontinue service line with no positions → status INACTIVE
// ---------------------------------------------------------------------------

test('SP-3b · discontinue service line with no positions succeeds (status inactive)', async ({ page }) => {
  await loginAs(page, PERSONAS.superAdmin);
  await page.goto('/service-lines');

  await expect(page.getByRole('heading', { name: 'Lini Layanan' })).toBeVisible({ timeout: 30_000 });

  // First create an empty service line (no positions) and then discontinue it.
  await page.getByRole('button', { name: 'Tambah Lini Layanan' }).click();
  await expect(page.locator('#sl-name')).toBeVisible({ timeout: 10_000 });
  await page.locator('#sl-name').fill('Empty Line E2E');
  await page.getByRole('button', { name: 'Simpan' }).last().click();
  await expect(page.getByText('Empty Line E2E').first()).toBeVisible({ timeout: 15_000 });

  // Now discontinue it.
  const emptyRow = page.locator('div.border-b').filter({ hasText: 'Empty Line E2E' });
  await emptyRow.getByRole('button', { name: 'Aksi baris' }).click();
  await page.getByRole('menuitem', { name: 'Nonaktifkan' }).click();

  await expect(page.getByText('Nonaktifkan Lini Layanan?')).toBeVisible({ timeout: 5_000 });
  await page.getByRole('button', { name: 'Ya, Nonaktifkan' }).click();

  // After confirm, wait for the row to update (the button triggers a refetch).
  // The toast shows 'Nonaktifkan' (same as menu label) — wait for the status badge instead.

  // Status badge in row should show Nonaktif.
  await expect(emptyRow.getByText('Nonaktif')).toBeVisible({ timeout: 10_000 });
});

// ---------------------------------------------------------------------------
// SP-4a — Create position under Parking → row appears in detail screen
// ---------------------------------------------------------------------------

test('SP-4a · create position under Parking service line: row appears', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto(`/service-lines/${PARKING_SVC_ID}`);

  // Wait for detail page (heading = "Parking").
  await expect(page.getByRole('heading', { name: 'Parking', exact: true })).toBeVisible({ timeout: 30_000 });

  // Click "Tambah Posisi".
  await page.getByRole('button', { name: 'Tambah Posisi' }).click();

  // Position modal opens.
  await expect(page.locator('#pos-name')).toBeVisible({ timeout: 10_000 });
  await page.locator('#pos-name').fill('Valet Parkir E2E');

  // Submit button for position modal
  await page.getByRole('dialog').locator('button[type="submit"]').click();

  // Row appears in the positions table.
  await expect(page.getByText('Valet Parkir E2E')).toBeVisible({ timeout: 15_000 });
});

// ---------------------------------------------------------------------------
// SP-4b — Duplicate position name in same line → POSITION_IN_USE error
// ---------------------------------------------------------------------------

test('SP-4b · duplicate position name in same service line shows POSITION_IN_USE error', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto(`/service-lines/${PARKING_SVC_ID}`);

  await expect(page.getByRole('heading', { name: 'Parking', exact: true })).toBeVisible({ timeout: 30_000 });

  await page.getByRole('button', { name: 'Tambah Posisi' }).click();
  await expect(page.locator('#pos-name')).toBeVisible({ timeout: 10_000 });

  // Use same name as seeded "Petugas Parkir" (SWP-POS-014) → 409.
  await page.locator('#pos-name').fill('Petugas Parkir');
  await page.getByRole('dialog').locator('button[type="submit"]').click();

  // A conflict error must surface.
  // conflict toast = t('errors.conflict') = 'Terjadi konflik dengan kondisi saat ini.'
  await expect(
    page
      .getByText(/konflik|duplikat|sudah ada|gagal/i)
      .first(),
  ).toBeVisible({ timeout: 15_000 });
});

// ---------------------------------------------------------------------------
// SP-4c — Update position name → updated value visible
// ---------------------------------------------------------------------------

test('SP-4c · update position name shows updated value in table', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto(`/service-lines/${PARKING_SVC_ID}`);

  await expect(page.getByText('Petugas Parkir').first()).toBeVisible({ timeout: 30_000 });

  // Click the kebab / edit button for "Petugas Parkir".
  // The row action button uses t('users.rowActions') = 'Aksi baris'.
  // The edit menuitem uses t('common.save') = 'Simpan' (screen quirk — see service-line-detail-screen.tsx).
  const posRow = page.locator('div.border-b').filter({ hasText: 'Petugas Parkir' }).first();
  await posRow.getByRole('button', { name: 'Aksi baris' }).click();

  await page.getByRole('menuitem', { name: 'Edit Posisi' }).click();

  // Position modal opens — update name.
  await expect(page.locator('#pos-name')).toBeVisible({ timeout: 10_000 });
  await page.locator('#pos-name').fill('Petugas Parkir (Updated)');
  await page.getByRole('dialog').locator('button[type="submit"]').click();

  // Updated name appears.
  await expect(page.getByText('Petugas Parkir (Updated)').first()).toBeVisible({ timeout: 15_000 });
});

// ---------------------------------------------------------------------------
// SP-4d — Soft-delete position → removed from active list (DB status = inactive)
// ---------------------------------------------------------------------------

test('SP-4d · soft-delete position removes it from active list and DB status is inactive', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto(`/service-lines/${PARKING_SVC_ID}`);

  await expect(page.getByText('Koordinator Lokasi')).toBeVisible({ timeout: 30_000 });

  // Soft-delete "Koordinator Lokasi" (SWP-POS-015).
  const posRow = page.locator('div.border-b').filter({ hasText: 'Koordinator Lokasi' });
  await posRow.getByRole('button', { name: 'Aksi baris' }).click();

  await page.getByRole('menuitem', { name: 'Hapus' }).click();

  // Confirm delete dialog.
  await expect(page.getByText('Hapus Posisi?')).toBeVisible({ timeout: 5_000 });
  await page.getByRole('button', { name: 'Hapus' }).last().click();

  // Position disappears from the list (or shows in a filtered-away state).
  await expect(page.getByText('Koordinator Lokasi')).toBeHidden({ timeout: 15_000 });

  // DB-side: position is soft-deleted (deleted_at set → getPositionStatus = 'inactive').
  const status = await getPositionStatus('SWP-POS-015');
  expect(status).toBe('inactive');
});
