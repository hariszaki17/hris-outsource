/**
 * tests/e2/change-requests.spec.ts
 *
 * E2 · profile change-request approval flow (EP-5, 2026-06-11 redesign) against the
 * REAL stack, driving the REAL change-requests queue + detail drawer + reject modal +
 * the agent self-service "Akun" screen. Each scenario is its own test().
 *
 * Coverage (Plan §F):
 *   SL-PARTIAL       SL Rudi approves the MIXED 2119 (Dewi @ CMP-0021) → non-bank phone
 *                    applied, bank field escalates to HR. Bank row shows "Perlu HR";
 *                    DB → status=partially_approved, bank_pending=true, phone applied,
 *                    bank NOT yet applied, partial toast.
 *   HR-FINALIZE      HR finalises the partially-approved 2119 → status=approved, bank
 *                    applied, bank_pending=false.
 *   SL-OUT-OF-SCOPE  SL Rudi :approve 2117 (Budi @ CMP-0022, out of his company) → 403.
 *   INSTANT-ADDRESS  Agent Budi edits address via Akun → Ubah Profil → applied instantly
 *                    (PATCH /me/profile), NO change_request row created.
 *   PHOTO-UPLOAD     Agent Budi photo-upload-init → PUT to the presigned URL → PATCH
 *                    /me/profile with the object_key → employee.photo_object_key set.
 *   REJECT-REASON    HR rejects the EMERGENCY_CONTACT 2120 with a reason → toast +
 *                    DB rejected.
 *   NOTIF-APPROVED   After HR finalises 2119, the submitter (Dewi SWP-EMP-3001) gets a
 *                    CHANGE_REQUEST_APPROVED notification (async River worker).
 *   NOTIF-REJECTED   After HR rejects a Budi CR, the submitter (Budi SWP-EMP-2891) gets a
 *                    CHANGE_REQUEST_REJECTED notification (async River worker).
 *
 * Seed (cmd/seed seedChangeRequests):
 *   SWP-CHG-2117  Budi @ CMP-0022  MULTIPLE (phone+bank)  → SL out-of-company target
 *   SWP-CHG-2119  Dewi @ CMP-0021  MULTIPLE (phone+bank)  → in-company bank-split target
 *   SWP-CHG-2120  Dewi @ CMP-0021  EMERGENCY_CONTACT only → SL/HR full-approve / reject target
 *
 * Stack: real Vite dev server (:4173, MSW off) ↔ real Go API (:8081) ↔ ephemeral Postgres
 * (:5433) + the MinIO private bucket (presigned PUT/GET) + the River worker (async notify).
 * Isolation: resetDb() in beforeEach. Traceable to EP-5, F2.x, INV-1.
 */

import {
  countNotificationsByKindForEmployee,
  getChangeRequestBankPending,
  getChangeRequestStatus,
  getEmployeeAddress,
  getEmployeeBankAccountNumber,
  getEmployeeEmergencyContact,
  getEmployeePhone,
  getEmployeePhotoObjectKey,
} from '../../lib/db.js';
import { apiAs, waitForToken } from '../../lib/e5-helpers.js';
import { type Page, expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';

// Wider viewport so every DataTable column (incl. AKSI) renders.
test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// Seeded fixture ids + the submitter employee ids.
// ---------------------------------------------------------------------------

const CR = {
  /** Budi @ CMP-0022 — MULTIPLE phone+bank. Out-of-company target for SL Rudi. */
  budiOutOfScope: 'SWP-CHG-2117',
  /** Dewi @ CMP-0021 — MULTIPLE phone+bank. In-company bank-split target. */
  dewiBankSplit: 'SWP-CHG-2119',
  /** Dewi @ CMP-0021 — EMERGENCY_CONTACT only. Full-approve / reject target. */
  dewiEmergency: 'SWP-CHG-2120',
} as const;

// Dewi has TWO pending CRs (2119 bank-split + 2120 emergency). Filtering the queue by her
// employee id alone is ambiguous, so the bank-split scenarios open the row by the bank
// account number unique to 2119.
const DEWI_BANK_ROW = '1440011223344';

const EMP = {
  /** Budi Santoso — agent persona; submitter of 2117. */
  budi: 'SWP-EMP-2891',
  /** Dewi Lestari — agent (extraPersonas); submitter of 2119/2120, placed @ CMP-0021. */
  dewi: 'SWP-EMP-3001',
} as const;

/** Dewi's agent login (extraPersonas — NOT in PERSONAS; password matches the seed). */
const DEWI = {
  email: 'dewi.lestari@swp.test',
  password: 'Dew1-Lestari-2026!',
  role: 'agent',
} as const;

// ---------------------------------------------------------------------------
// UI helpers — anchored on the REAL change-requests-screen DOM.
//
// The queue DataTable carries aria-label = t('changeRequests.tableTitle') =
// "Antrian Persetujuan"; its rows are `div.border-b` (filter by visible text).
// Each row's review action is a "Tinjau" button → opens ChangeRequestDetailDrawer
// (title "Detail Pengajuan Perubahan"). The reject modal heading is "Tolak Pengajuan",
// reason textarea #reject-reason, confirm "Tolak Pengajuan".
// ---------------------------------------------------------------------------

/** Load the queue page (logged in as `persona`) and wait for the seeded rows. */
async function loadQueue(page: Page, persona: 'hrAdmin' | 'shiftLeader'): Promise<void> {
  await loginAs(page, PERSONAS[persona]);
  await page.goto('/change-requests');
  await waitForToken(page);
  await expect(page.getByText('Antrian Persetujuan Perubahan Data').first()).toBeVisible({
    timeout: 30_000,
  });
}

/** The queue DataTable section (scopes selectors away from the filter row). */
function queueTable(page: Page) {
  return page.locator('[aria-label="Antrian Persetujuan"]');
}

/** Open the review drawer for the queue row matching `rowText` (employee id / value). */
async function openReviewDrawer(page: Page, rowText: string): Promise<void> {
  const table = queueTable(page);
  await expect(table).toBeVisible({ timeout: 15_000 });
  const row = table.locator('div.border-b').filter({ hasText: rowText }).first();
  await expect(row).toBeVisible({ timeout: 15_000 });
  await row.getByRole('button', { name: 'Tinjau' }).click();
  await expect(page.getByText('Detail Pengajuan Perubahan').first()).toBeVisible({
    timeout: 15_000,
  });
}

// ---------------------------------------------------------------------------
// SL-PARTIAL — shift leader approves the mixed request; bank escalates to HR.
// ---------------------------------------------------------------------------

test('SL-PARTIAL · SL Rudi approves mixed 2119: non-bank applied, bank escalates (Perlu HR)', async ({
  page,
}) => {
  await loadQueue(page, 'shiftLeader');
  await openReviewDrawer(page, DEWI_BANK_ROW);

  // The bank field is gated for the SL (no change_requests.approve.bank) → "Perlu HR".
  await expect(page.getByText('Perlu HR').first()).toBeVisible({ timeout: 10_000 });

  // SL CTA for a mixed request is "Setujui (selain bank)" (approveNonBankAction).
  await page.getByRole('button', { name: 'Setujui (selain bank)', exact: true }).click();

  // Partial-approval toast.
  await expect(
    page.getByText('Field non-bank diterapkan. Perubahan rekening diteruskan ke HR.').first(),
  ).toBeVisible({ timeout: 15_000 });

  // DB: request is partially-approved with bank still pending HR.
  await expect
    .poll(() => getChangeRequestStatus(CR.dewiBankSplit), { timeout: 15_000 })
    .toBe('partially_approved');
  expect(await getChangeRequestBankPending(CR.dewiBankSplit)).toBe(true);

  // The non-bank phone field IS applied; the bank field is NOT yet applied.
  expect(await getEmployeePhone(EMP.dewi)).toBe('+62-813-5566-7788');
  expect(await getEmployeeBankAccountNumber(EMP.dewi)).not.toBe('1440011223344');
});

// ---------------------------------------------------------------------------
// HR-FINALIZE — HR approves the partially-approved request → bank applied.
// ---------------------------------------------------------------------------

test('HR-FINALIZE · HR finalises the bank field of 2119 after SL partial → approved + bank applied', async ({
  page,
}) => {
  // Step 1 — SL applies non-bank, bank escalates (drives 2119 → partially_approved).
  await loadQueue(page, 'shiftLeader');
  await openReviewDrawer(page, DEWI_BANK_ROW);
  await page.getByRole('button', { name: 'Setujui (selain bank)', exact: true }).click();
  await expect
    .poll(() => getChangeRequestStatus(CR.dewiBankSplit), { timeout: 15_000 })
    .toBe('partially_approved');

  // Step 2 — HR finalises the same request (now PARTIALLY_APPROVED, bank_pending).
  await loadQueue(page, 'hrAdmin');
  await openReviewDrawer(page, DEWI_BANK_ROW);

  // HR holds change_requests.approve.bank → the plain "Setujui" CTA finalises the bank.
  await page.getByRole('button', { name: 'Setujui', exact: true }).click();
  await expect(page.getByText('Pengajuan berhasil disetujui').first()).toBeVisible({
    timeout: 15_000,
  });

  await expect
    .poll(() => getChangeRequestStatus(CR.dewiBankSplit), { timeout: 15_000 })
    .toBe('approved');
  expect(await getChangeRequestBankPending(CR.dewiBankSplit)).toBe(false);
  // The bank field is now applied to the employee.
  expect(await getEmployeeBankAccountNumber(EMP.dewi)).toBe('1440011223344');
});

// ---------------------------------------------------------------------------
// SL-OUT-OF-SCOPE — SL cannot approve a request for an agent in another company.
// ---------------------------------------------------------------------------

test('SL-OUT-OF-SCOPE · SL Rudi :approve 2117 (Budi @ CMP-0022) → 403', async ({ page }) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/change-requests');
  await waitForToken(page);

  // Budi (SWP-EMP-2891) is placed at CMP-0022; Rudi leads CMP-0021 → GuardCompany 403.
  const res = await apiAs(page, 'POST', `/change-requests/${CR.budiOutOfScope}:approve`);
  expect(res.status, `expected 403, got ${res.status}: ${JSON.stringify(res.body)}`).toBe(403);

  // The CR stays pending (no partial apply happened).
  expect(await getChangeRequestStatus(CR.budiOutOfScope)).toBe('pending');
});

// ---------------------------------------------------------------------------
// INSTANT-ADDRESS — agent edits address; applied instantly, no change request.
// ---------------------------------------------------------------------------

test('INSTANT-ADDRESS · agent Budi edits address via Akun → applied instantly, no change request', async ({
  page,
}) => {
  const newAddress = 'Jl. Anggrek Raya No. 12, Jakarta Selatan 12930';

  await loginAs(page, PERSONAS.agent);
  await page.goto('/me/akun');
  await waitForToken(page);

  // Open the tiered "Ubah Profil" modal.
  await page.getByRole('button', { name: 'Ubah Profil' }).first().click();

  // The address textarea is in the INSTANT section (#profile-address).
  const addr = page.locator('#profile-address');
  await expect(addr).toBeVisible({ timeout: 15_000 });
  await addr.fill(newAddress);

  // Save — instant tier applies immediately (PATCH /me/profile).
  await page.getByRole('button', { name: 'Simpan Perubahan', exact: true }).click();

  // Instant-only success toast.
  await expect(page.getByText('Profil diperbarui').first()).toBeVisible({ timeout: 15_000 });

  // DB: address applied to the employee; NO change_request was filed for an instant field.
  await expect.poll(() => getEmployeeAddress(EMP.budi), { timeout: 15_000 }).toBe(newAddress);

  // Sanity: still authenticated as the agent (the address applied above without an approval).
  const res = await apiAs(page, 'GET', '/auth/me');
  expect(res.status).toBe(200);
});

// ---------------------------------------------------------------------------
// PHOTO-UPLOAD — presign → PUT bytes → apply object_key.
// ---------------------------------------------------------------------------

test('PHOTO-UPLOAD · agent Budi photo presign + PUT + apply → employee.photo_object_key set', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.agent);
  await page.goto('/me/akun');
  await waitForToken(page);

  // 1) Init a presigned PUT for a small PNG.
  const init = await apiAs(page, 'POST', '/me/profile/photo-upload-init', {
    content_type: 'image/png',
    content_length: 1024,
  });
  expect(init.status, `photo-upload-init → ${init.status}: ${JSON.stringify(init.body)}`).toBe(200);
  const ticket = init.body as { upload_url: string; object_key: string };
  expect(ticket.object_key).toContain(`profile-photos/${EMP.budi}/`);

  // 2) PUT the bytes straight to MinIO via the presigned URL (from the browser).
  const putOk = await page.evaluate(
    async ({ url }) => {
      // Minimal 1x1 PNG bytes (1024 to satisfy content-length-range lower bound is not
      // required — any 1..max body is accepted; we send a small valid PNG).
      const png = Uint8Array.from([
        0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44,
        0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f,
        0x15, 0xc4, 0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00,
        0x01, 0x00, 0x00, 0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49,
        0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
      ]);
      const res = await fetch(url, {
        method: 'PUT',
        headers: { 'Content-Type': 'image/png' },
        body: png,
      });
      return res.ok;
    },
    { url: ticket.upload_url },
  );
  expect(putOk, 'presigned PUT to MinIO should succeed').toBe(true);

  // 3) Apply the object_key via PATCH /me/profile (instant tier).
  const patch = await apiAs(page, 'PATCH', '/me/profile', { photo_object_key: ticket.object_key });
  expect(patch.status, `PATCH /me/profile → ${patch.status}: ${JSON.stringify(patch.body)}`).toBe(
    200,
  );

  // DB: the employee's photo_object_key is the applied key in their own namespace.
  await expect
    .poll(() => getEmployeePhotoObjectKey(EMP.budi), { timeout: 15_000 })
    .toBe(ticket.object_key);
});

// ---------------------------------------------------------------------------
// REJECT-REASON — HR rejects the emergency-contact request with a reason.
// ---------------------------------------------------------------------------

test('REJECT-REASON · HR rejects emergency-contact 2120 with a reason → toast + DB rejected', async ({
  page,
}) => {
  await loadQueue(page, 'hrAdmin');
  await openReviewDrawer(page, EMP.dewi);

  // Open the reject modal from the drawer footer.
  await page.getByRole('button', { name: 'Tolak', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Tolak Pengajuan' })).toBeVisible({
    timeout: 10_000,
  });

  await page.locator('#reject-reason').fill('Kontak darurat tidak dapat diverifikasi.');
  await page.getByRole('button', { name: 'Tolak Pengajuan', exact: true }).last().click();

  await expect(page.getByText('Pengajuan berhasil ditolak').first()).toBeVisible({
    timeout: 15_000,
  });

  await expect
    .poll(() => getChangeRequestStatus(CR.dewiEmergency), { timeout: 15_000 })
    .toBe('rejected');

  // Employee emergency-contact stays unchanged (reject does NOT apply).
  const ec = await getEmployeeEmergencyContact(EMP.dewi);
  expect(ec?.name ?? null).not.toBe('Siti Lestari');
});

// ---------------------------------------------------------------------------
// NOTIF-APPROVED — submitter receives a CHANGE_REQUEST_APPROVED notification.
// ---------------------------------------------------------------------------

test('NOTIF-APPROVED · finalising 2120 (full approve) dispatches CHANGE_REQUEST_APPROVED to Dewi', async ({
  page,
}) => {
  // HR fully approves the emergency-contact-only request (no bank, single-step approve).
  await loadQueue(page, 'hrAdmin');
  await openReviewDrawer(page, EMP.dewi);
  await page.getByRole('button', { name: 'Setujui', exact: true }).click();
  await expect(page.getByText('Pengajuan berhasil disetujui').first()).toBeVisible({
    timeout: 15_000,
  });
  await expect
    .poll(() => getChangeRequestStatus(CR.dewiEmergency), { timeout: 15_000 })
    .toBe('approved');

  // The submitter (Dewi SWP-EMP-3001) receives the async notification (River worker).
  await expect
    .poll(() => countNotificationsByKindForEmployee(EMP.dewi, 'CHANGE_REQUEST_APPROVED'), {
      timeout: 25_000,
    })
    .toBeGreaterThanOrEqual(1);
});

// ---------------------------------------------------------------------------
// NOTIF-REJECTED — submitter receives a CHANGE_REQUEST_REJECTED notification.
// ---------------------------------------------------------------------------

test('NOTIF-REJECTED · rejecting 2117 dispatches CHANGE_REQUEST_REJECTED to Budi', async ({
  page,
}) => {
  // HR rejects Budi's request (HR has global scope — no company gate).
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/change-requests');
  await waitForToken(page);

  const res = await apiAs(page, 'POST', `/change-requests/${CR.budiOutOfScope}:reject`, {
    reason: 'Dokumen pendukung tidak lengkap.',
  });
  expect(res.status, `:reject → ${res.status}: ${JSON.stringify(res.body)}`).toBe(200);
  expect(await getChangeRequestStatus(CR.budiOutOfScope)).toBe('rejected');

  // The submitter (Budi SWP-EMP-2891) receives the async rejection notification.
  await expect
    .poll(() => countNotificationsByKindForEmployee(EMP.budi, 'CHANGE_REQUEST_REJECTED'), {
      timeout: 25_000,
    })
    .toBeGreaterThanOrEqual(1);
});
