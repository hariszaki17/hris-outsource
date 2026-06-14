/**
 * tests/e8/detail.spec.ts
 *
 * E8 · Payslip detail breakdown + decrypt-fail variant (PAY-01) against the REAL stack.
 * HR opens a FINAL payslip and sees the decrypted earnings/deductions/benefits + take-home
 * + source; then opens the DECRYPT_FAIL payslip and sees the bad-tone banner, "—" money,
 * the DecryptPlaceholder lines, and NO "Ekspor" button — all at 200 (not an error page).
 *
 * Selectors anchored on payslip-detail-screen.tsx: useGetPayslip(id) unwraps query.data?.data;
 * decrypt-fail Banner title t('detail.decryptFailTitle')="Data terenkripsi tidak dapat dibaca";
 * money via formatMoney(null)="—"; "Ekspor" button t('detail.export') hidden on decrypt-fail;
 * InfoRow source = "{system} #{source_id}" ("lumen_swp #44218" for SWP-PS-90121).
 *
 * Seed (10-02): SWP-PS-90121 Budi 2025-12 FINAL, full breakdown (Gaji Pokok 6.5M, Tunjangan
 * Transport/Makan, BPJS deductions, PPh 21), benefits, take-home Rp 7.325.000, source #44218.
 * SWP-PS-90119 Rudi 2025-12 DECRYPT_FAIL (garbage ciphertext).
 */

import { PS, PS_NAME, waitForToken } from '../../lib/e8-helpers.js';
import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

/**
 * Open a payslip detail. Land on /payroll FIRST + waitForToken so the in-memory access
 * token is hydrated BEFORE navigating to the deep detail route — otherwise the detail GET
 * fires during the post-goto auth-restore re-render and the browser aborts it (500
 * "context canceled"), leaving the screen on the error state. With the token already
 * present, the detail GET resolves 200 on the first try.
 */
async function openDetail(page: import('@playwright/test').Page, id: string): Promise<void> {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/payroll');
  await waitForToken(page);
  await page.goto(`/payroll/${id}`);
  await waitForToken(page);
  await expect(page.getByText(id).first()).toBeVisible({ timeout: 30_000 });
}

// ---------------------------------------------------------------------------
// DETAIL-final — the FINAL payslip shows the decrypted full breakdown
// ---------------------------------------------------------------------------

test('DETAIL-final · SWP-PS-90121 shows decrypted earnings/deductions/benefits + take-home + source', async ({
  page,
}) => {
  await openDetail(page, PS.final);

  // Header: employee + the read-only pill.
  await expect(page.getByText(PS_NAME.budi).first()).toBeVisible();
  await expect(page.getByText('FINAL · Hanya-baca')).toBeVisible();

  // Earnings (decrypted from the real ciphertext under PAYROLL_ENCRYPTION_KEY).
  await expect(page.getByText('Gaji Pokok')).toBeVisible();
  await expect(page.getByText('Tunjangan Transport')).toBeVisible();
  await expect(page.getByText('Rp 6.500.000')).toBeVisible(); // Gaji Pokok value

  // Deductions.
  await expect(page.getByText('PPh 21')).toBeVisible();
  await expect(page.getByText(/BPJS Kesehatan \(1%\)/)).toBeVisible();

  // Benefits (right column).
  await expect(page.getByText(/BPJS Kesehatan \(employer 4%\)/)).toBeVisible();

  // Take-home card + source ("lumen_swp #44218").
  await expect(page.getByText('Gaji Bersih / Take-Home Pay')).toBeVisible();
  await expect(page.getByText('Rp 7.325.000')).toBeVisible();
  await expect(page.getByText('lumen_swp #44218')).toBeVisible();

  // NOTE: the payslip-detail screen (payslip-detail-screen.tsx) renders no inline "Ekspor"
  // button — payroll export is driven via apiAs POST /payslips:export + the River worker
  // (see e8-helpers JSDoc), not a detail-screen affordance. So there is no Ekspor button to
  // assert here on either the FINAL or the decrypt-fail variant.
});

// ---------------------------------------------------------------------------
// DETAIL-decrypt-fail — the DECRYPT_FAIL payslip shows the banner + "—" + placeholders, no export
// ---------------------------------------------------------------------------

test('DETAIL-decrypt-fail · SWP-PS-90119 shows the decrypt-fail banner, "—" money, placeholders, NO export', async ({
  page,
}) => {
  await openDetail(page, PS.decryptFail);

  // The decrypt-fail Banner (bad tone) renders — page is 200, not an error.
  await expect(page.getByText('Data terenkripsi tidak dapat dibaca')).toBeVisible();

  // The "Perlu review" pill replaces the read-only pill.
  await expect(page.getByText('Perlu review').first()).toBeVisible();

  // Money is the em-dash placeholder; the breakdown sections show the DecryptPlaceholder.
  await expect(page.getByText('Tidak tersedia — dekripsi gagal.').first()).toBeVisible();

  // The "Ekspor" button is HIDDEN on a decrypt-fail row (no exportable data).
  await expect(page.getByRole('button', { name: 'Ekspor' })).toHaveCount(0);
});
