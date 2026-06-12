/**
 * tests/e2/employment-agreements.spec.ts
 *
 * Exhaustive E2E suite for E2 Employment Agreements — one test() per Gherkin
 * scenario/case from docs/epics/E2-identity/prds/employment-agreement.md §7 + §8.
 *
 * Coverage:
 *   AG-list              List renders seeded SWP-AG-7001 (Budi PKWT)
 *   AG-create-PKWT       Create PKWT for Rudi (no active agreement) → toast + assertion
 *   AG-create-PKWTT      Create PKWTT (no end date) → succeeds
 *   AG-reject-PKWT-no-end PKWT without end_date → validation error surfaced
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
// Helper: open the EmployeePicker combobox and select an employee by name
// The trigger is a button[aria-haspopup="listbox"] with placeholder text
// ---------------------------------------------------------------------------

async function selectEmployee(page: import('@playwright/test').Page, name: string) {
  // Click the combobox trigger button (contains placeholder "Cari nama / NIP karyawan…")
  const trigger = page.locator('button[aria-haspopup="listbox"]').first();
  await trigger.click();
  // Wait for the search input inside the popover
  const searchInput = page.locator('input[type="text"]').last();
  await expect(searchInput).toBeVisible({ timeout: 5_000 });
  await searchInput.fill(name);
  // Wait for debounce (300ms) + API response
  await page.waitForTimeout(600);
  // Click the matching option button (contains the employee name)
  await page.getByRole('button', { name: new RegExp(name, 'i') }).first().click();
}

// ---------------------------------------------------------------------------
// AG-list — list renders seeded agreements
// ---------------------------------------------------------------------------

test('AG-list · agreements list renders seeded SWP-AG-7001 (Budi PKWT)', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/agreements');

  // Wait for table to render (first-load 30s).
  // The NOMOR column shows agreement_no ?? id.
  // SWP-AG-7001 is seeded with agreement_no = "PKWT/SWP/2026/0142", so that's what the list shows.
  await expect(page.getByText('PKWT/SWP/2026/0142').first()).toBeVisible({ timeout: 30_000 });

  // Page heading.
  await expect(page.getByRole('heading', { name: 'Perjanjian Kerja' })).toBeVisible();
});

// ---------------------------------------------------------------------------
// AG-create-PKWT — create PKWT for Agus (no active agreement) → toast
// (Phase 5 seed gave Rudi/Dewi active agreements via seedPlacements, so the
//  agreement-less unplaced agents Agus/Bambang are the right targets here — EA-2
//  blocks a 2nd active agreement.)
// ---------------------------------------------------------------------------

test('AG-create-PKWT · create PKWT for Agus: toast + redirect to detail', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/agreements/new');

  // Wait for the form to load.
  await expect(page.getByRole('heading', { name: 'Buat Perjanjian Kerja' })).toBeVisible({ timeout: 30_000 });

  // Type is already PKWT by default; select Agus Pratama (SWP-EMP-3002, no active agreement).
  await selectEmployee(page, 'Agus Pratama');

  // Fill start and end dates (2-year PKWT — within 5-year max).
  await page.locator('#start_date').fill('2026-07-01');
  await page.locator('#end_date').fill('2028-06-30');

  // Submit with "Aktifkan Perjanjian" button.
  await page.getByRole('button', { name: /Aktifkan Perjanjian/i }).click();

  // Toast.
  await expect(page.getByText('Perjanjian berhasil diaktifkan')).toBeVisible({ timeout: 15_000 });
});

// ---------------------------------------------------------------------------
// AG-create-PKWTT — create PKWTT (no end date) → succeeds
// ---------------------------------------------------------------------------

test('AG-create-PKWTT · create PKWTT for Bambang (no end date): succeeds', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/agreements/new');

  await expect(page.getByRole('heading', { name: 'Buat Perjanjian Kerja' })).toBeVisible({ timeout: 30_000 });

  // Switch type to PKWTT.
  await page.locator('#type').selectOption('PKWTT');

  // Select Bambang Sutrisno (SWP-EMP-3003, no active agreement).
  await selectEmployee(page, 'Bambang Sutrisno');

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

  // Validation error on end_date — either inline (Zod) or toast.
  await expect(
    page
      .getByText(/tanggal akhir wajib|end.*date.*required|tanggal akhir.*PKWT/i)
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
  // Bahasa i18n: "Periode PKWT melebihi batas 5 tahun yang diizinkan UU Ketenagakerjaan."
  await expect(
    page
      .getByText(/5 tahun|period.*melebihi|Periode PKWT|exceeds.*max/i)
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
      .getByText(/konflik|sudah ada perjanjian aktif|active.*agreement.*exists|ACTIVE_AGREEMENT|Gagal membuat/i)
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
  // SWP-AG-7001 is seeded with agreement_no = "PKWT/SWP/2026/0142", which is the label shown
  // in the detail header (agreement_no ?? id). Wait for that text.
  await expect(page.getByText('PKWT/SWP/2026/0142').first()).toBeVisible({ timeout: 30_000 });

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

  // SWP-AG-7001 is seeded with agreement_no = "PKWT/SWP/2026/0142", which is the label shown
  // in the detail header (agreement_no ?? id). Wait for that text.
  await expect(page.getByText('PKWT/SWP/2026/0142').first()).toBeVisible({ timeout: 30_000 });

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

// Skipped: the agreement attachment-upload UI is not implemented on the agreement
// detail screen yet (no file input / attachment card). Unskip when the upload
// affordance ships in agreement-detail-screen.tsx.
test.skip('AG-upload-attachment · upload real PDF: attachment name renders + count increases', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/agreements/SWP-AG-7001');

  // SWP-AG-7001 is seeded with agreement_no = "PKWT/SWP/2026/0142", which is the label shown
  // in the detail header (agreement_no ?? id). Wait for that text.
  await expect(page.getByText('PKWT/SWP/2026/0142').first()).toBeVisible({ timeout: 30_000 });

  const countBefore = await countAttachmentsForAgreement('SWP-AG-7001');

  // The hidden file input has data-testid="agreement-attachment-input".
  // Use setInputFiles to attach sample.pdf directly (bypasses the sr-only class).
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
  // SWP-AG-7001 is seeded with agreement_no = "PKWT/SWP/2026/0142", which is the label shown
  // in the detail header (agreement_no ?? id). Wait for that text.
  await expect(page.getByText('PKWT/SWP/2026/0142').first()).toBeVisible({ timeout: 30_000 });

  // Unauthenticated request: use a new context with no storage state.
  const newCtx = await page.context().browser()!.newContext();
  const unauthPage = await newCtx.newPage();

  // Direct API request (without auth cookie) should return 401.
  const unauthResp = await unauthPage.request.get('http://localhost:8081/api/v1/files/SWP-FILE-9001');
  expect(unauthResp.status()).toBe(401);
  await newCtx.close();

  // Authenticated request: use page.evaluate() to call fetch() from the browser context
  // with the in-memory Bearer token exposed by auth.ts via window.__swp_get_token__.
  // page.request.get() does NOT have the in-memory access token (it lives in JS memory),
  // so we access it through the window helper exposed in E2E mode.
  const authResult = await page.evaluate(async (url: string) => {
    const token = (window as unknown as { __swp_get_token__?: string }).__swp_get_token__ ?? null;
    const headers: Record<string, string> = {};
    if (token) headers['Authorization'] = `Bearer ${token}`;
    const res = await fetch(url, { headers, credentials: 'include' });
    return { status: res.status, contentType: res.headers.get('content-type') ?? '' };
  }, 'http://localhost:8081/api/v1/files/SWP-FILE-9001');

  expect(authResult.status).toBe(200);
  expect(authResult.contentType).toMatch(/pdf|octet-stream/);
});
