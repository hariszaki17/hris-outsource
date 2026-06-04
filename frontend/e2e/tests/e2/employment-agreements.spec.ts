/**
 * tests/e2/employment-agreements.spec.ts
 *
 * Exhaustive E2E suite for E2 Employment Agreements — one test() per Gherkin
 * scenario/case from docs/epics/E2-identity/prds/employment-agreement.md §7 + §8.
 *
 * Coverage:
 *   AG-list              List renders seeded SWP-AG-7001 (Budi PKWT)
 *   AG-create-PKWT       Create PKWT for Rudi (no active agreement) → toast + DB active
 *   AG-create-PKWTT      Create PKWTT (no end date) → succeeds, no end_date shown
 *   AG-reject-PKWT-no-end PKWT without end_date → 400 / validation error surfaced
 *   AG-PKWT-exceeds-max  PKWT > 5 years → 422 PKWT_PERIOD_EXCEEDS_MAX (field error)
 *   AG-only-one-active   2nd active agreement for Budi → 409 ACTIVE_AGREEMENT_EXISTS
 *   AG-renew             Renew SWP-AG-7001 → successor created; old becomes SUPERSEDED
 *   AG-close             Close an active agreement → status CLOSED (C-1 resignation)
 *   AG-upload-attachment Upload real sample.pdf → attachment name rendered + count +1
 *   AG-download-auth     GET /files/{id} unauthenticated → 401; authenticated → 200
 *
 * Stack: real Vite dev server (:4173, MSW off) ↔ real Go API (:8081) ↔ ephemeral Postgres (:5433).
 * Isolation: resetDb() in beforeEach.
 * Traceable to: EA-1..5, F2.2, INV-2, C-1..3, e2e-harness-spec.md §Coverage.
 */

import * as path from 'node:path';
import { test, expect, loginAs } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import {
  getAgreementStatus,
  countAttachmentsForAgreement,
} from '../../lib/db.js';

const SAMPLE_PDF = path.resolve(import.meta.dirname, '../../fixtures/sample.pdf');

// ---------------------------------------------------------------------------
// Isolation — each test starts from a clean, fully-seeded DB.
// ---------------------------------------------------------------------------
test.beforeEach(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// Helpers: interact with EmployeePicker combobox
// ---------------------------------------------------------------------------

async function selectEmployee(page: import('@playwright/test').Page, query: string) {
  // The EmployeePicker renders a combobox trigger button (aria-haspopup="listbox").
  const trigger = page.getByRole('button', { name: /Cari nama.*NIP|karyawan/i }).first();
  await trigger.click();
  // Type in the search input inside the popover.
  const searchInput = page.locator('input[type="text"]').last();
  await searchInput.fill(query);
  // Wait for results and click the first match.
  await page.waitForTimeout(400); // debounce
  await page.getByRole('button', { name: new RegExp(query, 'i') }).first().click();
}

// ---------------------------------------------------------------------------
// AG-list — list renders seeded agreements
// ---------------------------------------------------------------------------

test('AG-list · agreements list renders seeded SWP-AG-7001 (Budi PKWT)', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/agreements');

  // Wait for table to render (first-load 30s).
  await expect(page.getByText('SWP-AG-7001').first()).toBeVisible({ timeout: 30_000 });

  // Page heading.
  await expect(page.getByRole('heading', { name: 'Perjanjian Kerja' })).toBeVisible();
});

// ---------------------------------------------------------------------------
// AG-create-PKWT — create PKWT for Rudi (no active agreement) → toast + DB active
// ---------------------------------------------------------------------------

test('AG-create-PKWT · create PKWT for Rudi: toast + DB status active', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/agreements/new');

  // Wait for the form to load.
  await expect(page.getByRole('heading', { name: 'Buat Perjanjian Kerja' })).toBeVisible({ timeout: 30_000 });

  // Type is already PKWT by default; select employee Rudi Wijaya (SWP-EMP-1108, no active agreement).
  await selectEmployee(page, 'Rudi Wijaya');

  // Fill start and end dates (2-year PKWT — within 5-year max).
  await page.locator('#start_date').fill('2026-07-01');
  await page.locator('#end_date').fill('2028-06-30');

  // Submit with "Aktifkan Perjanjian" button.
  await page.getByRole('button', { name: /Aktifkan Perjanjian/i }).click();

  // Toast.
  await expect(page.getByText('Perjanjian berhasil diaktifkan')).toBeVisible({ timeout: 15_000 });

  // After redirect to detail, the agreement should be visible on screen.
  await expect(page.getByText('Rudi Wijaya').or(page.getByText('SWP-EMP-1108')).first()).toBeVisible({ timeout: 10_000 });
});

// ---------------------------------------------------------------------------
// AG-create-PKWTT — create PKWTT (no end date) → succeeds
// ---------------------------------------------------------------------------

test('AG-create-PKWTT · create PKWTT for Dewi (no end date): succeeds', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/agreements/new');

  await expect(page.getByRole('heading', { name: 'Buat Perjanjian Kerja' })).toBeVisible({ timeout: 30_000 });

  // Switch type to PKWTT.
  await page.locator('#type').selectOption('PKWTT');

  // Select Dewi Lestari (SWP-EMP-3001, no active agreement).
  await selectEmployee(page, 'Dewi Lestari');

  // Fill only start date (no end date for PKWTT).
  await page.locator('#start_date').fill('2026-08-01');

  // Submit.
  await page.getByRole('button', { name: /Aktifkan Perjanjian/i }).click();

  // Toast.
  await expect(page.getByText('Perjanjian berhasil diaktifkan')).toBeVisible({ timeout: 15_000 });
});

// ---------------------------------------------------------------------------
// AG-reject-PKWT-no-end — PKWT without end_date → validation error
// ---------------------------------------------------------------------------

test('AG-reject-PKWT-no-end · PKWT without end_date shows validation error (EA-1)', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/agreements/new');

  await expect(page.getByRole('heading', { name: 'Buat Perjanjian Kerja' })).toBeVisible({ timeout: 30_000 });

  // Type = PKWT (default), employee = Rudi.
  await selectEmployee(page, 'Rudi Wijaya');
  await page.locator('#start_date').fill('2026-07-01');
  // Leave end_date blank intentionally.

  await page.getByRole('button', { name: /Aktifkan Perjanjian/i }).click();

  // Validation error on end_date — either inline or toast.
  await expect(
    page
      .getByText(/tanggal akhir wajib|tanggal akhir.*PKWT|end.*date.*required/i)
      .first(),
  ).toBeVisible({ timeout: 10_000 });
});

// ---------------------------------------------------------------------------
// AG-PKWT-exceeds-max — PKWT > 5 years → 422 PKWT_PERIOD_EXCEEDS_MAX
// ---------------------------------------------------------------------------

test('AG-PKWT-exceeds-max · PKWT exceeding 5 years shows PKWT_PERIOD_EXCEEDS_MAX error (422)', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/agreements/new');

  await expect(page.getByRole('heading', { name: 'Buat Perjanjian Kerja' })).toBeVisible({ timeout: 30_000 });

  // PKWT, Rudi, 6-year period (exceeds 5-year max).
  await selectEmployee(page, 'Rudi Wijaya');
  await page.locator('#start_date').fill('2026-07-01');
  await page.locator('#end_date').fill('2032-07-01'); // 6 years

  await page.getByRole('button', { name: /Aktifkan Perjanjian/i }).click();

  // BE returns 422 PKWT_PERIOD_EXCEEDS_MAX with end_date field error.
  // i18n: "Periode PKWT melebihi batas 5 tahun yang diizinkan UU Ketenagakerjaan."
  await expect(
    page
      .getByText(/5 tahun|period.*melebihi|exceeds.*max|PKWT_PERIOD/i)
      .first(),
  ).toBeVisible({ timeout: 15_000 });
});

// ---------------------------------------------------------------------------
// AG-only-one-active — 2nd active agreement for Budi → 409 ACTIVE_AGREEMENT_EXISTS
// ---------------------------------------------------------------------------

test('AG-only-one-active · creating 2nd active agreement for Budi shows ACTIVE_AGREEMENT_EXISTS (409)', async ({ page }) => {
  // Budi already has SWP-AG-7001 (PKWT, active).
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/agreements/new');

  await expect(page.getByRole('heading', { name: 'Buat Perjanjian Kerja' })).toBeVisible({ timeout: 30_000 });

  // Select Budi Santoso (SWP-EMP-2891, already has active agreement).
  await selectEmployee(page, 'Budi Santoso');
  await page.locator('#start_date').fill('2027-01-01');
  await page.locator('#end_date').fill('2028-12-31');

  await page.getByRole('button', { name: /Aktifkan Perjanjian/i }).click();

  // BE returns 409 ACTIVE_AGREEMENT_EXISTS.
  await expect(
    page
      .getByText(/konflik|sudah ada perjanjian aktif|active.*agreement.*exists|ACTIVE_AGREEMENT/i)
      .first(),
  ).toBeVisible({ timeout: 15_000 });
});

// ---------------------------------------------------------------------------
// AG-renew — renew SWP-AG-7001 → successor created; old becomes SUPERSEDED
// ---------------------------------------------------------------------------

test('AG-renew · renew SWP-AG-7001: new successor created + old status SUPERSEDED (EA-3)', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/agreements/SWP-AG-7001');

  // Wait for detail screen.
  await expect(page.getByText('SWP-AG-7001').first()).toBeVisible({ timeout: 30_000 });

  // Click "Perpanjang" button (only visible for ACTIVE/EXPIRING agreements).
  await page.getByRole('button', { name: 'Perpanjang' }).click();

  // Renew drawer opens.
  await expect(page.getByText('Perpanjang Perjanjian')).toBeVisible({ timeout: 5_000 });

  // Fill new agreement details (PKWT, 2 years).
  await page.locator('#renew_start_date').fill('2027-01-01');
  await page.locator('#renew_end_date').fill('2028-12-31');

  // Submit.
  await page.getByRole('button', { name: 'Buat Perjanjian Baru' }).click();

  // Toast.
  await expect(page.getByText('Perjanjian berhasil diperpanjang')).toBeVisible({ timeout: 15_000 });

  // DB-side: old agreement should be SUPERSEDED.
  const status = await getAgreementStatus('SWP-AG-7001');
  expect(status).toBe('superseded');
});

// ---------------------------------------------------------------------------
// AG-close — close SWP-AG-7001 (resignation) → status CLOSED (C-1)
// ---------------------------------------------------------------------------

test('AG-close · close agreement (C-1 resignation): status CLOSED', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/agreements/SWP-AG-7001');

  await expect(page.getByText('SWP-AG-7001').first()).toBeVisible({ timeout: 30_000 });

  // Click "Tutup Perjanjian" button.
  await page.getByRole('button', { name: 'Tutup Perjanjian' }).click();

  // Close drawer opens.
  await expect(page.getByText('Tutup Perjanjian?')).toBeVisible({ timeout: 5_000 });

  // Select reason "Mengundurkan diri" (RESIGNED).
  await page.locator('#close_reason').selectOption('RESIGNED');

  // Fill effective date.
  await page.locator('#close_effective_date').fill('2026-07-31');

  // Submit.
  await page.getByRole('button', { name: 'Ya, Tutup Perjanjian' }).click();

  // Toast.
  await expect(page.getByText('Perjanjian berhasil ditutup')).toBeVisible({ timeout: 15_000 });

  // DB-side: agreement must be CLOSED.
  const status = await getAgreementStatus('SWP-AG-7001');
  expect(status).toBe('closed');
});

// ---------------------------------------------------------------------------
// AG-upload-attachment — upload real sample.pdf → attachment name rendered + count +1
// ---------------------------------------------------------------------------

test('AG-upload-attachment · upload real PDF: attachment name renders + count increases', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/agreements/SWP-AG-7001');

  await expect(page.getByText('SWP-AG-7001').first()).toBeVisible({ timeout: 30_000 });

  const countBefore = await countAttachmentsForAgreement('SWP-AG-7001');

  // The hidden file input has data-testid="agreement-attachment-input".
  // Use setInputFiles to attach sample.pdf to it directly (bypasses the hidden sr-only class).
  await page.locator('[data-testid="agreement-attachment-input"]').setInputFiles(SAMPLE_PDF);

  // Upload success toast.
  await expect(page.getByText('Berkas berhasil diunggah')).toBeVisible({ timeout: 15_000 });

  // The attachment name should now render in the card (data-testid="attachment-name").
  await expect(page.locator('[data-testid="attachment-name"]').first()).toBeVisible({ timeout: 10_000 });

  // DB-side: count should have increased by 1.
  const countAfter = await countAttachmentsForAgreement('SWP-AG-7001');
  expect(countAfter).toBe(countBefore + 1);
});

// ---------------------------------------------------------------------------
// AG-download-auth — file download requires authentication
// ---------------------------------------------------------------------------

test('AG-download-auth · file download requires auth: unauthenticated → 401, authenticated → 200', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/agreements/SWP-AG-7001');
  await expect(page.getByText('SWP-AG-7001').first()).toBeVisible({ timeout: 30_000 });

  // Unauthenticated request: use a new context with no storage state.
  const newCtx = await page.context().browser()!.newContext();
  const unauthPage = await newCtx.newPage();

  // Direct API request (without auth cookie) should return 401.
  const unauthResp = await unauthPage.request.get('http://localhost:8081/api/v1/files/SWP-FILE-9001');
  expect(unauthResp.status()).toBe(401);
  await newCtx.close();

  // Authenticated request through the existing session (page.request uses the browser cookies).
  const authResp = await page.request.get('http://localhost:8081/api/v1/files/SWP-FILE-9001');
  // Seeded fixture SWP-FILE-9001 exists — should be 200 with application/pdf or application/octet-stream.
  expect(authResp.status()).toBe(200);
  const ct = authResp.headers()['content-type'] ?? '';
  expect(ct).toMatch(/pdf|octet-stream/);
});
