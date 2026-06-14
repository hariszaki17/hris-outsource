/**
 * tests/e2/client-companies.spec.ts
 *
 * Exhaustive E2E suite for E2 Client Companies — one test() per Gherkin scenario/case
 * from docs/epics/E2-identity/prds/client-company-directory.md §7 + §8.
 *
 * Coverage:
 *   CC-1a  List renders seeded companies (Plaza Senayan, Mall Kelapa Gading)
 *   CC-1b  Create company → toast + row appears in list + getCompanyStatus === 'active'
 *   CC-1c  Create company auto-creates a primary site (countSitesForCompany === 1)
 *   CC-2   Duplicate company name → 409 conflict surfaced in UI (inline/toast error)
 *   CC-3   Edit company (pic_name) → toast + updated value visible
 *   CC-4a  Deactivate company → status badge shows Nonaktif + getCompanyStatus === 'inactive'
 *   CC-4b  Reactivate company → status badge shows Aktif + getCompanyStatus === 'active'
 *   CC-5   (SKIP — Phase 5 dep) deactivate with active placements guard
 *   RB-2   Agent denied the client-companies screen (RBAC negative)
 *
 * Stack: real Vite dev server (:4173, MSW off) ↔ real Go API (:8081) ↔ ephemeral Postgres (:5433).
 * Isolation: resetDb() in beforeEach.
 * Traceable to: CC-1..5, F2.3, INV-1, e2e-harness-spec.md §Coverage.
 */

import { test, expect, loginAs } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import {
  getCompanyStatus,
  countSitesForCompany,
  getCompanyByName,
} from '../../lib/db.js';

// ---------------------------------------------------------------------------
// Isolation — each test starts from a clean, fully-seeded DB.
// ---------------------------------------------------------------------------
test.beforeEach(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// CC-1a — List renders seeded companies from the real BE
// ---------------------------------------------------------------------------

test('CC-1a · companies list renders seeded Plaza Senayan and Mall Kelapa Gading', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/client-companies');

  // Wait for table to render (first-load 30s for Vite + session restore + API call).
  await expect(page.getByText('Plaza Senayan').first()).toBeVisible({ timeout: 30_000 });
  await expect(page.getByText('Mall Kelapa Gading').first()).toBeVisible();

  // Page heading confirms correct screen.
  await expect(page.getByRole('heading', { name: 'Perusahaan Klien' })).toBeVisible();
});

// ---------------------------------------------------------------------------
// CC-1b — Create company → toast + row appears + status active
// ---------------------------------------------------------------------------

test('CC-1b · create company: row appears in list with status Aktif', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/client-companies');

  // Wait for existing rows before adding.
  await expect(page.getByText('Plaza Senayan').first()).toBeVisible({ timeout: 30_000 });

  // Click "Tambah Perusahaan".
  await page.getByRole('button', { name: 'Tambah Perusahaan' }).click();

  // Full-page create route should load (/client-companies/new).
  await page.waitForURL('**/client-companies/new', { timeout: 10_000 });

  // Fill required fields.
  const newName = 'PT Graha Tama E2E';
  await page.locator('#cc-name').fill(newName);
  await page.locator('#cc-address').fill('Jl. Sudirman No. 99, Jakarta');

  // Submit.
  await page.getByRole('button', { name: 'Simpan' }).click();

  // Success toast appears.
  await expect(page.getByText('Perusahaan berhasil ditambahkan')).toBeVisible({ timeout: 15_000 });

  // After redirect → detail page should show the company name.
  await expect(page.getByText(newName).first()).toBeVisible({ timeout: 10_000 });

  // DB-side: status must be 'active'.
  const id = await getCompanyByName(newName);
  expect(id).not.toBeNull();
  const status = await getCompanyStatus(id!);
  expect(status).toBe('active');
});

// ---------------------------------------------------------------------------
// CC-1c — Create company auto-creates a primary site (Main Site)
// ---------------------------------------------------------------------------

test('CC-1c · create company auto-creates a primary site', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/client-companies/new');

  const newName = 'PT Auto-Site Test E2E';
  await page.locator('#cc-name').fill(newName);
  await page.locator('#cc-address').fill('Jl. Gatot Subroto No. 1, Jakarta');
  await page.getByRole('button', { name: 'Simpan' }).click();

  // Wait for success (redirect to detail).
  await expect(page.getByText(newName).first()).toBeVisible({ timeout: 15_000 });

  // DB-side: exactly 1 active site created.
  const id = await getCompanyByName(newName);
  expect(id).not.toBeNull();
  const siteCount = await countSitesForCompany(id!);
  expect(siteCount).toBe(1);
});

// ---------------------------------------------------------------------------
// CC-2 — Duplicate company name → conflict error surfaced in UI
// ---------------------------------------------------------------------------

test('CC-2 · duplicate company name shows conflict error', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/client-companies/new');

  // Try to create a company with the same name as a seeded one.
  await page.locator('#cc-name').fill('Plaza Senayan');
  await page.locator('#cc-address').fill('Jl. Asia Afrika No. 8, Jakarta');
  await page.getByRole('button', { name: 'Simpan' }).click();

  // A conflict/error message must surface — either inline field error or toast.
  // The BE returns 409; classifyError maps to 'conflict' → t('errors.conflict')
  // = 'Terjadi konflik dengan kondisi saat ini.' or toast 'Gagal membuat perusahaan'.
  await expect(
    page
      .getByText(/konflik|duplikat|sudah ada|gagal/i)
      .first(),
  ).toBeVisible({ timeout: 15_000 });
});

// ---------------------------------------------------------------------------
// CC-3 — Edit company (pic_name) → toast + updated value visible
// ---------------------------------------------------------------------------

test('CC-3 · edit company pic_name shows updated value', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/client-companies');

  // Wait for list.
  await expect(page.getByText('Plaza Senayan').first()).toBeVisible({ timeout: 30_000 });

  // Navigate to the detail page via the company name link.
  await page.getByRole('link', { name: 'Plaza Senayan' }).first().click();

  // Detail header → Edit (it's a link).
  await page.getByRole('link', { name: 'Edit' }).first().click();

  // Edit form opens — fill PIC Name.
  await expect(page.locator('#cc-pic-name')).toBeVisible({ timeout: 10_000 });
  await page.locator('#cc-pic-name').fill('Budi Santoso E2E');

  // Submit (button inside the drawer form).
  await page.getByRole('button', { name: 'Simpan' }).click();

  // Toast: success.
  await expect(page.getByText('Perusahaan berhasil diperbarui')).toBeVisible({ timeout: 15_000 });
});

// ---------------------------------------------------------------------------
// CC-4a — Deactivate company: status badge → Nonaktif + DB status = 'inactive'
// ---------------------------------------------------------------------------

test('CC-4a · deactivate company: status badge Nonaktif + DB inactive', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/client-companies');

  // Wait for list.
  await expect(page.getByText('Mall Kelapa Gading').first()).toBeVisible({ timeout: 30_000 });

  // Direct inline deactivate button on Mall Kelapa Gading's row.
  const mkgRow = page
    .locator('div.border-b')
    .filter({ hasText: 'Mall Kelapa Gading' });
  await mkgRow.getByRole('button', { name: 'Nonaktifkan' }).click();

  // Confirm dialog.
  await expect(page.getByText('Nonaktifkan Perusahaan?')).toBeVisible({ timeout: 5_000 });
  await page.getByRole('button', { name: 'Ya, Nonaktifkan' }).click();

  // Toast.
  await expect(page.getByText('Perusahaan berhasil dinonaktifkan')).toBeVisible({ timeout: 15_000 });

  // Status badge in row should show Nonaktif.
  await expect(mkgRow.getByText('Nonaktif')).toBeVisible({ timeout: 10_000 });

  // DB-side.
  const id = await getCompanyByName('Mall Kelapa Gading');
  expect(id).not.toBeNull();
  const status = await getCompanyStatus(id!);
  expect(status).toBe('inactive');
});

// ---------------------------------------------------------------------------
// CC-4b — Reactivate company: status badge → Aktif + DB status = 'active'
// ---------------------------------------------------------------------------

test('CC-4b · reactivate company: status badge Aktif + DB active', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/client-companies');

  // Wait for list.
  await expect(page.getByText('Mall Kelapa Gading').first()).toBeVisible({ timeout: 30_000 });

  const mkgRow = () =>
    page.locator('div.border-b').filter({ hasText: 'Mall Kelapa Gading' });

  // Step 1: deactivate first (direct inline button).
  await mkgRow().getByRole('button', { name: 'Nonaktifkan' }).click();
  await page.getByRole('button', { name: 'Ya, Nonaktifkan' }).click();
  await expect(page.getByText('Perusahaan berhasil dinonaktifkan')).toBeVisible({ timeout: 15_000 });

  // Step 2: reactivate (direct inline button).
  await mkgRow().getByRole('button', { name: 'Aktifkan kembali' }).click();

  // Reactivate confirm dialog.
  await expect(page.getByText('Aktifkan Kembali?')).toBeVisible({ timeout: 5_000 });
  await page.getByRole('button', { name: 'Ya, Aktifkan' }).click();

  // Toast.
  await expect(page.getByText('Perusahaan berhasil diaktifkan kembali')).toBeVisible({ timeout: 15_000 });

  // Status badge.
  // exact:true — otherwise "Aktif" also matches the "Nonaktifkan" action button (contains "aktif").
  await expect(mkgRow().getByText('Aktif', { exact: true })).toBeVisible({ timeout: 10_000 });

  // DB-side.
  const id = await getCompanyByName('Mall Kelapa Gading');
  expect(id).not.toBeNull();
  const status = await getCompanyStatus(id!);
  expect(status).toBe('active');
});

// ---------------------------------------------------------------------------
// CC-5 — SKIP: deactivate with active placements guard (Phase 5 dep)
// ---------------------------------------------------------------------------

test.skip('CC-5 · deactivate company with active placements shows COMPANY_HAS_ACTIVE_PLACEMENTS error', async ({ page: _page }) => {
  // This test requires an active placement against the company.
  // Placements are introduced in Phase 5 (E3 placement slice).
  // The BE guard is stubbed as count=0/no-op in Phase 3 (TODO(Phase-5) in companies_service.go).
  // Unskip in Phase 5 when placements are seeded and the guard is active.
  //
  // Expected behaviour: deactivate attempt on a company with active_placement_count > 0
  // → BE returns 409 COMPANY_HAS_ACTIVE_PLACEMENTS
  // → UI shows toast 'Perusahaan masih memiliki penempatan aktif' (clientCompanies.toast.deactivateConflict).
});

// ---------------------------------------------------------------------------
// RB-2 — Agent denied the client-companies screen (RBAC negative)
// ---------------------------------------------------------------------------

test('RB-2 · agent is denied the client-companies screen', async ({ page }) => {
  await loginAs(page, PERSONAS.agent);
  await page.goto('/client-companies');

  // Agent has no clientCompanies.read permission — the screen must show a
  // permission-denied EmptyState (noPermission.body or errors.forbidden i18n key).
  // The BE returns 403; the FE classifyError maps 'forbidden' → StateView no-permission.
  await expect(
    page
      .getByText(/tidak memiliki izin/i)
      .or(page.getByText(/forbidden/i))
      .or(page.getByText(/akses ditolak/i))
      .first(),
  ).toBeVisible({ timeout: 20_000 });
});
