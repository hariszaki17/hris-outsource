/**
 * tests/e2/employees.spec.ts
 *
 * Exhaustive E2E suite for E2 Employees — one test() per Gherkin scenario/case
 * from docs/epics/E2-identity/prds/employee-profile.md §7 + §8.
 *
 * Coverage:
 *   EP-list             List renders seeded employees (Budi Santoso, Sari Hadi)
 *   EP-create-data-only Create employee (name + NIK + join_at, no login) → toast + DB active
 *   EP-create-with-login Create with provision_login + login_email → row created (stub: UserID stays NULL)
 *   EP-reject-dup-NIK   Duplicate NIK → 409 conflict surfaced in UI
 *   EP-detail           Open Budi's detail → Profil tab shows phone + bank values
 *   EP-update           Edit employee phone → toast + getEmployeePhone reflects new value
 *   EP-deactivate       Deactivate → status badge Nonaktif + DB inactive (EP-2 / C-4)
 *   EP-reactivate       Reactivate after deactivate → status Aktif + DB active (C-3)
 *   RB                  Agent denied the employees screen (RBAC negative)
 *
 * Stack: real Vite dev server (:4173, MSW off) ↔ real Go API (:8081) ↔ ephemeral Postgres (:5433).
 * Isolation: resetDb() in beforeEach.
 * Traceable to: EP-1..5, F2.1, INV-1, C-1..4, e2e-harness-spec.md §Coverage.
 */

import { test, expect, loginAs } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import {
  getEmployeeStatus,
  getEmployeePhone,
  getEmployeeIdByNIK,
} from '../../lib/db.js';

// ---------------------------------------------------------------------------
// Use wider viewport so all DataTable columns (incl. AKSI/actions) are visible.
// ---------------------------------------------------------------------------
test.use({ viewport: { width: 1600, height: 900 } });

// ---------------------------------------------------------------------------
// Isolation — each test starts from a clean, fully-seeded DB.
// ---------------------------------------------------------------------------
test.beforeEach(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// EP-list — list renders seeded employees
// ---------------------------------------------------------------------------

test('EP-list · employees list renders seeded Budi Santoso and Sari Hadi', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/employees');

  // Wait for table to render (first-load 30s for Vite + session restore + API call).
  await expect(page.getByText('Budi Santoso').first()).toBeVisible({ timeout: 30_000 });
  await expect(page.getByText('Sari Hadi').first()).toBeVisible();

  // Page heading confirms correct screen.
  await expect(page.getByRole('heading', { name: 'Karyawan' })).toBeVisible();
});

// ---------------------------------------------------------------------------
// EP-create-data-only — create employee (no login provisioning) → toast + DB active
// ---------------------------------------------------------------------------

test('EP-create-data-only · create employee (data only, no login): row appears + DB active', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/employees');

  // Wait for existing list.
  await expect(page.getByText('Budi Santoso').first()).toBeVisible({ timeout: 30_000 });

  // Click "Tambah Karyawan" to navigate to /employees/new.
  await page.getByRole('button', { name: 'Tambah Karyawan' }).click();
  await page.waitForURL('**/employees/new', { timeout: 10_000 });

  // Fill required fields.
  const newNik = '3175001505909999';
  const newName = 'Citra Dewi E2E';
  await page.locator('#full_name').fill(newName);
  await page.locator('#nik').fill(newNik);
  await page.locator('#join_at').fill('2026-01-15');

  // Submit with "Simpan Karyawan".
  await page.getByRole('button', { name: 'Simpan Karyawan' }).click();

  // Success toast.
  await expect(page.getByText('Karyawan berhasil ditambahkan')).toBeVisible({ timeout: 15_000 });

  // DB-side: employee must exist and be active.
  const id = await getEmployeeIdByNIK(newNik);
  expect(id).not.toBeNull();
  const status = await getEmployeeStatus(id!);
  expect(status).toBe('active');
});

// ---------------------------------------------------------------------------
// EP-create-with-login — create with provision_login stub (UserID stays NULL)
// ---------------------------------------------------------------------------

test('EP-create-with-login · create employee with provision_login: row created (login stub — UserID NULL)', async ({ page }) => {
  // NOTE: EP-3 stub — provision_login and login_email are accepted by the BE but UserID
  // stays NULL (no E1 User created in Phase 4). We only assert that the employee row is
  // created successfully without a server error. Full login provisioning deferred.
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/employees/new');

  const stubNik = '3175001505908888';
  await page.locator('#full_name').fill('Doni Prasetyo E2E Login');
  await page.locator('#nik').fill(stubNik);
  await page.locator('#join_at').fill('2026-02-01');

  // Toggle provision login on.
  const toggle = page.getByRole('switch', { name: /provision.*login|login.*self/i }).first();
  await toggle.click();

  // Fill login email (now required by the form).
  const loginEmailField = page.locator('#login_email').or(page.getByPlaceholder(/login.*email/i));
  await loginEmailField.fill('doni.prasetyo.e2e@swp.test');

  // Submit.
  await page.getByRole('button', { name: 'Simpan Karyawan' }).click();

  // Success toast — employee row created even though UserID is NULL (stub).
  await expect(page.getByText('Karyawan berhasil ditambahkan')).toBeVisible({ timeout: 15_000 });

  // DB-side: employee must exist.
  const id = await getEmployeeIdByNIK(stubNik);
  expect(id).not.toBeNull();
});

// ---------------------------------------------------------------------------
// EP-reject-dup-NIK — duplicate NIK → 409 conflict surfaced in UI
// ---------------------------------------------------------------------------

test('EP-reject-dup-NIK · duplicate NIK shows DUPLICATE_NIK conflict error', async ({ page }) => {
  // Budi's seeded NIK = 3175001505902891
  const budiNik = '3175001505902891';

  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/employees/new');

  await page.locator('#full_name').fill('Duplikat NIK Test');
  await page.locator('#nik').fill(budiNik);
  await page.locator('#join_at').fill('2026-01-01');

  await page.getByRole('button', { name: 'Simpan Karyawan' }).click();

  // The BE returns 409 DUPLICATE_NIK; the FE surfaces it as a toast error or inline.
  // i18n: "NIK sudah terdaftar untuk karyawan lain." or generic conflict toast.
  await expect(
    page
      .getByText(/NIK sudah|duplikat|konflik|sudah ada|gagal/i)
      .first(),
  ).toBeVisible({ timeout: 15_000 });

  // No new row created — querying the NIK still returns Budi's ID only.
  const id = await getEmployeeIdByNIK(budiNik);
  expect(id).toBe('SWP-EMP-2891');
});

// ---------------------------------------------------------------------------
// EP-detail — open Budi's detail → Profil tab shows phone and bank account
// ---------------------------------------------------------------------------

test('EP-detail · employee detail: Profil tab renders phone and bank account for Budi Santoso', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/employees/SWP-EMP-2891');

  // Wait for detail to load (first-load 30s).
  await expect(page.getByText('Budi Santoso').first()).toBeVisible({ timeout: 30_000 });

  // Profil tab should be active by default; phone and bank data should appear.
  await expect(page.getByText('+62-812-3344-5566')).toBeVisible({ timeout: 10_000 });
  // BCA bank account seeded with account_number 1234567890
  await expect(page.getByText('1234567890')).toBeVisible({ timeout: 10_000 });
});

// ---------------------------------------------------------------------------
// EP-update — edit employee phone → toast + DB updated
// ---------------------------------------------------------------------------

test('EP-update · edit employee phone: toast + getEmployeePhone reflects new value', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/employees/SWP-EMP-2891');

  // Wait for detail screen.
  await expect(page.getByText('Budi Santoso').first()).toBeVisible({ timeout: 30_000 });

  // Click "Edit Profil" button.
  await page.getByRole('button', { name: 'Edit Profil' }).click();

  // Edit drawer / screen opens; fill phone field.
  await expect(page.locator('#phone').or(page.locator('input[type="tel"]').first())).toBeVisible({ timeout: 10_000 });
  await page.locator('#phone').fill('+62-899-9999-0001');

  // Submit changes.
  await page.getByRole('button', { name: 'Simpan Perubahan' }).click();

  // Toast: success.
  await expect(page.getByText('Data karyawan berhasil diperbarui')).toBeVisible({ timeout: 15_000 });

  // DB-side: phone must be updated.
  const phone = await getEmployeePhone('SWP-EMP-2891');
  expect(phone).toBe('+62-899-9999-0001');
});

// ---------------------------------------------------------------------------
// EP-deactivate — deactivate employee → status badge Nonaktif + DB inactive
// ---------------------------------------------------------------------------

test('EP-deactivate · deactivate employee: status badge Nonaktif + DB inactive', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/employees/SWP-EMP-2891');

  // Wait for detail screen.
  await expect(page.getByText('Budi Santoso').first()).toBeVisible({ timeout: 30_000 });

  // Click kebab / "Aksi lainnya" — opens deactivate confirm.
  await page.getByRole('button', { name: 'Aksi lainnya' }).click();

  // Confirm dialog.
  await expect(page.getByText('Nonaktifkan karyawan?')).toBeVisible({ timeout: 5_000 });
  await page.getByRole('button', { name: 'Nonaktifkan' }).first().click();

  // Toast.
  await expect(page.getByText('Karyawan dinonaktifkan')).toBeVisible({ timeout: 15_000 });

  // Status badge should now show Nonaktif.
  await expect(page.getByText('Nonaktif').first()).toBeVisible({ timeout: 10_000 });

  // DB-side.
  const status = await getEmployeeStatus('SWP-EMP-2891');
  expect(status).toBe('inactive');
});

// ---------------------------------------------------------------------------
// EP-reactivate — reactivate employee → status Aktif + DB active (C-3)
// ---------------------------------------------------------------------------

test('EP-reactivate · reactivate employee (C-3): status badge Aktif + DB active', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/employees/SWP-EMP-2891');

  await expect(page.getByText('Budi Santoso').first()).toBeVisible({ timeout: 30_000 });

  // Step 1: deactivate first.
  await page.getByRole('button', { name: 'Aksi lainnya' }).click();
  await expect(page.getByText('Nonaktifkan karyawan?')).toBeVisible({ timeout: 5_000 });
  await page.getByRole('button', { name: 'Nonaktifkan' }).first().click();
  await expect(page.getByText('Karyawan dinonaktifkan')).toBeVisible({ timeout: 15_000 });

  // Step 2: the MoreVertical button now opens reactivate confirm (since status = inactive).
  await page.getByRole('button', { name: 'Aksi lainnya' }).click();
  await expect(page.getByText('Aktifkan kembali karyawan?')).toBeVisible({ timeout: 5_000 });
  await page.getByRole('button', { name: 'Aktifkan Kembali' }).first().click();

  // Toast.
  await expect(page.getByText('Karyawan diaktifkan kembali')).toBeVisible({ timeout: 15_000 });

  // Status badge.
  await expect(page.getByText('Aktif').first()).toBeVisible({ timeout: 10_000 });

  // DB-side.
  const status = await getEmployeeStatus('SWP-EMP-2891');
  expect(status).toBe('active');
});

// ---------------------------------------------------------------------------
// RB — agent denied the employees screen (RBAC negative)
// ---------------------------------------------------------------------------

test('RB · agent is denied the employees screen', async ({ page }) => {
  await loginAs(page, PERSONAS.agent);
  await page.goto('/employees');

  // Agent has no employees.read permission — the screen must show permission denied.
  // The BE returns 403; the FE classifyError maps 'forbidden' → EmptyState no-permission.
  await expect(
    page
      .getByText(/tidak memiliki izin/i)
      .or(page.getByText(/akses ditolak/i))
      .or(page.getByText(/forbidden/i))
      .first(),
  ).toBeVisible({ timeout: 20_000 });
});
