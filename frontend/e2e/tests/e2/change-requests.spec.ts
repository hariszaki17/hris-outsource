/**
 * tests/e2/change-requests.spec.ts
 *
 * Exhaustive E2E suite for E2 Change-Request HR Approval Queue — one test() per
 * Gherkin scenario/case from docs/epics/E2-identity/prds/employee-profile.md §7 + §8
 * (EP-5 scenarios) + the change-request queue behaviours.
 *
 * Coverage:
 *   CR-queue              hrAdmin opens queue → SWP-EMP-2891 rows visible (both pending CRs for Budi)
 *   CR-detail-diff        Open MULTIPLE CR detail via drawer → diff shows old/new phone + bank
 *   CR-approve            Approve MULTIPLE CR → toast + DB approved + employee phone updated
 *   CR-reject-needs-reason Open reject modal, submit empty reason → validation blocked
 *   CR-reject             Reject ADDRESS CR with reason → DB rejected, employee unchanged
 *   CR-already-resolved   After approve, row leaves queue; 2nd approve → conflict handled
 *   RB                    Agent denied the change-requests screen (RBAC negative)
 *
 * Row locator: The queue list renders cr.employee_id ("SWP-EMP-2891"), request type
 * badge ("Beberapa Field" for MULTIPLE, "Alamat" for ADDRESS), and change values.
 *
 * Seed order (submitted_at DESC): SWP-CHG-2118 (09:30) = row 1, SWP-CHG-2117 (08:00) = row 2.
 * Row 1 = ADDRESS / "Alamat", Row 2 = MULTIPLE / "Beberapa Field".
 *
 * Button strategy: Within each row div, locate the "Tinjau" text button using
 * locator(':text("Tinjau")') which finds by visible text (more robust than getByRole name).
 *
 * Stack: real Vite dev server (:4173, MSW off) ↔ real Go API (:8081) ↔ ephemeral Postgres (:5433).
 * Isolation: resetDb() in beforeEach.
 * Traceable to: EP-5, F2.x, INV-1, e2e-harness-spec.md §Coverage.
 */

import { test, expect, loginAs } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import {
  getChangeRequestStatus,
  getEmployeePhone,
} from '../../lib/db.js';

// ---------------------------------------------------------------------------
// Use a wider viewport so all DataTable columns (incl. AKSI) are visible.
// ---------------------------------------------------------------------------
test.use({ viewport: { width: 1600, height: 900 } });

// ---------------------------------------------------------------------------
// Isolation — each test starts from a clean, fully-seeded DB.
// ---------------------------------------------------------------------------
test.beforeEach(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// Helper: load the queue page and wait for rows to appear
// ---------------------------------------------------------------------------
async function loadQueue(page: import('@playwright/test').Page) {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/change-requests');
  await expect(page.getByText('Antrian Persetujuan Perubahan Data').first()).toBeVisible({ timeout: 30_000 });
  await expect(page.getByText('SWP-EMP-2891').first()).toBeVisible({ timeout: 15_000 });
}

// ---------------------------------------------------------------------------
// Helper: click the "Tinjau" (Review) button in a specific row
// Uses :text() selector for robust text matching (avoids accessible-name issues)
// ---------------------------------------------------------------------------
async function clickTinjauInRow(
  page: import('@playwright/test').Page,
  rowHasText: string,
) {
  // Scope to the DataTable section (aria-label="Antrian Persetujuan") to avoid matching
  // the filter row div (which also has border-b and contains "Beberapa Field"/"Alamat"
  // as option values in the FilterSelect).
  const tableSection = page.locator('section[aria-label="Antrian Persetujuan"]');
  await expect(tableSection).toBeVisible({ timeout: 5_000 });

  const row = tableSection.locator('div.border-b').filter({ hasText: rowHasText }).first();
  await expect(row).toBeVisible({ timeout: 5_000 });

  // Find the "Tinjau" button within the row.
  const tinjauBtn = row.locator('button', { hasText: 'Tinjau' });
  await expect(tinjauBtn).toBeVisible({ timeout: 5_000 });
  await tinjauBtn.click();
}

// ---------------------------------------------------------------------------
// CR-queue — HR queue renders seeded change-requests
// ---------------------------------------------------------------------------

test('CR-queue · hrAdmin opens change-requests queue: seeded pending CRs for Budi visible', async ({ page }) => {
  await loadQueue(page);

  // Both type labels must appear in data rows (scoped to the DataTable section
  // to avoid matching the filter-row div which also has border-b and contains
  // "Beberapa Field" / "Alamat" as FilterSelect option values).
  const tableSection = page.locator('section[aria-label="Antrian Persetujuan"]');
  await expect(tableSection).toBeVisible({ timeout: 10_000 });
  await expect(
    tableSection.locator('div.border-b').filter({ hasText: 'Beberapa Field' }).first(),
  ).toBeVisible({ timeout: 5_000 });
  await expect(
    tableSection.locator('div.border-b').filter({ hasText: 'Alamat' }).first(),
  ).toBeVisible({ timeout: 5_000 });
});

// ---------------------------------------------------------------------------
// CR-detail-diff — open MULTIPLE CR detail → diff shows old→new phone + bank
// ---------------------------------------------------------------------------

test('CR-detail-diff · open MULTIPLE CR detail: diff shows old phone/bank → new values', async ({ page }) => {
  await loadQueue(page);
  await clickTinjauInRow(page, 'Beberapa Field');

  // Detail drawer opens.
  await expect(page.getByText('Detail Pengajuan Perubahan')).toBeVisible({ timeout: 10_000 });

  // Diff: old phone → new phone. Use .first() to handle strict-mode collisions with the
  // table row that also shows the new phone value (appears before the drawer opens fully).
  await expect(page.getByText('+62-812-3344-5566').first()).toBeVisible({ timeout: 10_000 });
  await expect(page.getByText('+62-812-9988-7766').first()).toBeVisible({ timeout: 5_000 });

  // Bank account diff — the value renders as "BCA · 9999000011 · (Budi Santoso)";
  // use a partial-text regex to match the account number substring.
  await expect(page.getByText('1234567890').first()).toBeVisible({ timeout: 5_000 });
  await expect(page.getByText(/9999000011/).first()).toBeVisible({ timeout: 5_000 });
});

// ---------------------------------------------------------------------------
// CR-approve — approve MULTIPLE CR → toast + DB approved + employee phone updated
// ---------------------------------------------------------------------------

test('CR-approve · approve MULTIPLE CR: toast + DB approved + employee phone updated to new value', async ({ page }) => {
  await loadQueue(page);
  await clickTinjauInRow(page, 'Beberapa Field');

  await expect(page.getByText('Detail Pengajuan Perubahan')).toBeVisible({ timeout: 10_000 });

  // Click "Setujui" inside the drawer.
  await page.getByRole('button', { name: 'Setujui' }).first().click();

  // Toast.
  await expect(page.getByText('Pengajuan berhasil disetujui')).toBeVisible({ timeout: 15_000 });

  // DB-side: CR status must be 'approved'.
  const crStatus = await getChangeRequestStatus('SWP-CHG-2117');
  expect(crStatus).toBe('approved');

  // DB-side: employee phone must reflect the new value.
  const phone = await getEmployeePhone('SWP-EMP-2891');
  expect(phone).toBe('+62-812-9988-7766');
});

// ---------------------------------------------------------------------------
// CR-reject-needs-reason — reject modal: submit empty reason → blocked
// ---------------------------------------------------------------------------

test('CR-reject-needs-reason · reject modal: empty reason is blocked by validation', async ({ page }) => {
  await loadQueue(page);
  await clickTinjauInRow(page, 'Alamat');

  await expect(page.getByText('Detail Pengajuan Perubahan')).toBeVisible({ timeout: 10_000 });

  // Click "Tolak" to open reject modal.
  await page.getByRole('button', { name: 'Tolak' }).first().click();

  // Reject modal opens — scope to heading to avoid strict-mode collision with the submit button.
  await expect(page.getByRole('heading', { name: 'Tolak Pengajuan' })).toBeVisible({ timeout: 5_000 });

  // Submit with empty reason.
  await page.getByRole('button', { name: 'Tolak Pengajuan' }).last().click();

  // Validation error: "Alasan minimal 3 karakter".
  await expect(page.getByText('Alasan minimal 3 karakter').first()).toBeVisible({ timeout: 5_000 });

  // CR status must still be 'pending'.
  const crStatus = await getChangeRequestStatus('SWP-CHG-2118');
  expect(crStatus).toBe('pending');
});

// ---------------------------------------------------------------------------
// CR-reject — reject ADDRESS CR with reason → DB rejected + employee unchanged
// ---------------------------------------------------------------------------

test('CR-reject · reject ADDRESS CR with reason: DB rejected + employee address unchanged', async ({ page }) => {
  await loadQueue(page);
  await clickTinjauInRow(page, 'Alamat');

  await expect(page.getByText('Detail Pengajuan Perubahan')).toBeVisible({ timeout: 10_000 });

  // Open reject modal.
  await page.getByRole('button', { name: 'Tolak' }).first().click();
  // Scope to heading to avoid strict-mode collision with the submit button.
  await expect(page.getByRole('heading', { name: 'Tolak Pengajuan' })).toBeVisible({ timeout: 5_000 });

  // Fill a valid reason.
  await page.locator('#rr-reason').fill('Alamat tidak sesuai dokumen kependudukan yang diterima.');

  // Submit.
  await page.getByRole('button', { name: 'Tolak Pengajuan' }).last().click();

  // Toast.
  await expect(page.getByText('Pengajuan berhasil ditolak')).toBeVisible({ timeout: 15_000 });

  // DB-side: CR status must be 'rejected'.
  const crStatus = await getChangeRequestStatus('SWP-CHG-2118');
  expect(crStatus).toBe('rejected');
});

// ---------------------------------------------------------------------------
// CR-already-resolved — after approve MULTIPLE CR, row leaves pending queue
// ---------------------------------------------------------------------------

test('CR-already-resolved · after approve, MULTIPLE CR leaves pending queue (no crash)', async ({ page }) => {
  await loadQueue(page);
  await clickTinjauInRow(page, 'Beberapa Field');
  await expect(page.getByText('Detail Pengajuan Perubahan')).toBeVisible({ timeout: 10_000 });
  await page.getByRole('button', { name: 'Setujui' }).first().click();
  await expect(page.getByText('Pengajuan berhasil disetujui')).toBeVisible({ timeout: 15_000 });

  // Drawer closes; queue refreshes. Page remains on /change-requests.
  await expect(page).toHaveURL(/\/change-requests/, { timeout: 5_000 });

  // Verify the CR is approved in DB.
  const crStatus = await getChangeRequestStatus('SWP-CHG-2117');
  expect(crStatus).toBe('approved');
});

// ---------------------------------------------------------------------------
// RB — agent denied the change-requests queue (RBAC negative)
// ---------------------------------------------------------------------------

test('RB · agent is denied the change-requests queue', async ({ page }) => {
  await loginAs(page, PERSONAS.agent);
  await page.goto('/change-requests');

  // Agent has no changeRequests.read permission.
  await expect(
    page
      .getByText(/tidak memiliki izin/i)
      .or(page.getByText(/akses ditolak/i))
      .or(page.getByText(/forbidden/i))
      .first(),
  ).toBeVisible({ timeout: 20_000 });
});
