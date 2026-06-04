/**
 * tests/e5/verify-reject.spec.ts
 *
 * E5 · single verify / reject (F5.3) against the REAL stack, driving the REAL
 * attendance-detail-screen + verification-queue inline action.
 *
 * Coverage:
 *   VERIFY-single      HR opens detail 9003, clicks Verifikasi → ConfirmDialog confirm →
 *                      success toast; the verification badge flips to VERIFIED on refetch.
 *   REJECT-single      HR opens detail 9002, clicks Tolak, fills #detail-reject-reason
 *                      (>=5 chars), confirms → success toast + REJECTED + reason rendered.
 *   REJECT-validation  reject reason <5 chars → ConfirmDialog confirm button disabled.
 *   VERIFY-queue-inline queue inline Verifikasi → confirm → row leaves the PENDING queue.
 *
 * Detail selectors (attendance-detail-screen.tsx): back-row Verifikasi/Tolak buttons
 * (t('verifyBtn')/t('rejectBtn')); reject reason Input id="detail-reject-reason"; the
 * ConfirmDialog confirm labels = t('verifyConfirm')="Verifikasi" / t('rejectConfirm')="Tolak".
 * Seed (07-02): 9002 Dewi LATE, 9003 Sari OUTSIDE_GEOFENCE — both PENDING @ CMP-0021.
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import { ATT, apiAs, queueRow, waitForToken } from '../../lib/e5-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// VERIFY-single — detail Verifikasi → confirm → VERIFIED
// ---------------------------------------------------------------------------

test('VERIFY-single · HR verifies attendance 9003 from the detail screen → success toast + VERIFIED badge', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto(`/attendance/${ATT.geoCmp21}`);
  await waitForToken(page);

  // Detail header renders.
  await expect(page.getByText(ATT.geoCmp21).first()).toBeVisible({ timeout: 20_000 });

  // Click the back-row Verifikasi button → ConfirmDialog → confirm.
  await page.getByRole('button', { name: 'Verifikasi', exact: true }).first().click();
  await page.getByRole('button', { name: 'Verifikasi', exact: true }).last().click();

  // Success toast.
  await expect(page.getByText('Record diverifikasi').first()).toBeVisible({ timeout: 15_000 });

  // Cross-check the persisted state via the API.
  const res = await apiAs(page, 'GET', `/attendance/${ATT.geoCmp21}`);
  expect(res.status).toBe(200);
  const rec = (res.body as { data?: { verification_status?: string } }).data;
  expect(rec?.verification_status).toBe('VERIFIED');
});

// ---------------------------------------------------------------------------
// REJECT-single — detail Tolak + reason → REJECTED
// ---------------------------------------------------------------------------

test('REJECT-single · HR rejects attendance 9002 with a reason → success toast + REJECTED + reason persisted', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto(`/attendance/${ATT.lateCmp21}`);
  await waitForToken(page);

  await expect(page.getByText(ATT.lateCmp21).first()).toBeVisible({ timeout: 20_000 });

  // Open the reject ConfirmDialog.
  await page.getByRole('button', { name: 'Tolak', exact: true }).first().click();

  // Fill the reason (>=5 chars) then confirm.
  await page.locator('#detail-reject-reason').fill('Bukti foto tidak sesuai jam masuk');
  await page.getByRole('button', { name: 'Tolak', exact: true }).last().click();

  await expect(page.getByText('Record ditolak').first()).toBeVisible({ timeout: 15_000 });

  // Cross-check persisted REJECTED + reason.
  const res = await apiAs(page, 'GET', `/attendance/${ATT.lateCmp21}`);
  expect(res.status).toBe(200);
  const rec = (res.body as { data?: { verification_status?: string; reject_reason?: string } })
    .data;
  expect(rec?.verification_status).toBe('REJECTED');
  expect(rec?.reject_reason).toContain('Bukti foto');
});

// ---------------------------------------------------------------------------
// REJECT-validation — <5 char reason → confirm disabled
// ---------------------------------------------------------------------------

test('REJECT-validation · reject reason under 5 chars keeps the confirm button disabled', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto(`/attendance/${ATT.geoCmp21}`);
  await waitForToken(page);

  await expect(page.getByText(ATT.geoCmp21).first()).toBeVisible({ timeout: 20_000 });
  await page.getByRole('button', { name: 'Tolak', exact: true }).first().click();

  await page.locator('#detail-reject-reason').fill('abc'); // 3 chars < 5

  // The ConfirmDialog confirm (last "Tolak" button) is disabled (confirmDisabled).
  await expect(page.getByRole('button', { name: 'Tolak', exact: true }).last()).toBeDisabled();
});

// ---------------------------------------------------------------------------
// VERIFY-queue-inline — inline Verifikasi from the queue row → leaves PENDING
// ---------------------------------------------------------------------------

test('VERIFY-queue-inline · inline Verifikasi on a queue row → confirm → the record leaves the PENDING queue', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/attendance/verification');
  await waitForToken(page);

  // Sari's row (EMP-1042 → 9003) is unambiguous in the queue.
  const row = queueRow(page, 'SWP-EMP-1042');
  await expect(row).toBeVisible({ timeout: 30_000 });

  // Click the row's inline Verifikasi button (scoped to the row).
  await row.getByRole('button', { name: 'Verifikasi', exact: true }).click();

  // ConfirmDialog confirm.
  await page.getByRole('button', { name: 'Verifikasi', exact: true }).last().click();
  await expect(page.getByText('Record diverifikasi').first()).toBeVisible({ timeout: 15_000 });

  // After the refetch, 9003 is no longer PENDING → its row leaves the queue.
  await expect(page.locator('div.border-b').filter({ hasText: 'SWP-EMP-1042' })).toHaveCount(0, {
    timeout: 20_000,
  });
});
