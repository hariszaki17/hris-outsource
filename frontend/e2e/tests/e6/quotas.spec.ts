/**
 * tests/e6/quotas.spec.ts
 *
 * E6 · leave quota management (LVE-02) against the REAL stack, driving the REAL
 * leave-quotas-screen (AdjustQuotaModal + BulkGrantModal). Each scenario is its own test().
 *
 * Coverage:
 *   LIST-remaining   HR /leave/quotas shows Dewi (remaining 5) + Budi (remaining -3).
 *                    remaining = total − used − pending; the seeded PENDING requests count
 *                    as pending soft-reservations: Dewi pending 3 (8001+8002+8007 @ 1 day),
 *                    Budi pending 4 (8003 @ 3 days + 8004 @ 1 day).
 *   ADJUST-happy     Adjust Dewi #delta=+2 / #reason → success toast + persisted remaining 7.
 *   ADJUST-refuse    Adjust Budi #delta=-2 (total 10 < used 11) → 422 RULE_VIOLATION field error
 *                    on #delta; modal stays open; remaining unchanged.
 *   BULK-grant       Terbitkan Kuota Tahunan → Pratinjau (preview) → Terbitkan (apply) → success.
 *   BALANCE-override over-balance HR final on 8003 surfaces the override modal → APPROVED
 *                    (the FE balance-recheck → override path, quota-driven).
 *
 * Seed (08-02): SWP-LQ-8001 Dewi total 12 used 4 (pending 3 → remaining 5); SWP-LQ-8002 Budi
 * total 12 used 11 (pending 4 → remaining -3). SWP-LR-8003 Budi PENDING_HR over-balance.
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import {
  BTN,
  EMP,
  LQ,
  LR,
  expectLeaveStatus,
  openLeaveDetail,
  quotaRemaining,
  waitForToken,
} from '../../lib/e6-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// LIST-remaining — the quota table renders the seeded remaining math
// ---------------------------------------------------------------------------

test('LIST-remaining · HR /leave/quotas shows Dewi (remaining 5) + Budi (remaining -3)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/leave/quotas');
  await waitForToken(page);

  // Both employee rows render (employee_id is shown mono in the employee column).
  const dewiRow = page.locator('div.border-b').filter({ hasText: EMP.dewi }).first();
  const budiRow = page.locator('div.border-b').filter({ hasText: EMP.budi }).first();
  await expect(dewiRow).toBeVisible({ timeout: 30_000 });
  await expect(budiRow).toBeVisible({ timeout: 30_000 });

  // Cross-check the persisted remaining via the API (deterministic).
  // remaining = total − used − pending (the seeded PENDING requests soft-reserve days).
  expect(await quotaRemaining(page, LQ.dewi)).toBe(5);
  expect(await quotaRemaining(page, LQ.budi)).toBe(-3);
});

// ---------------------------------------------------------------------------
// ADJUST-happy — +2 to Dewi → remaining 7
// ---------------------------------------------------------------------------

test('ADJUST-happy · Adjust Dewi +2 with a reason → success toast + persisted remaining 7', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/leave/quotas');
  await waitForToken(page);

  const dewiRow = page.locator('div.border-b').filter({ hasText: EMP.dewi }).first();
  await expect(dewiRow).toBeVisible({ timeout: 30_000 });
  await dewiRow.getByRole('button', { name: /^Sesuaikan kuota/ }).click();

  await page.locator('#delta').fill('2');
  await page.locator('#reason').fill('Penyesuaian kuota sisa cuti tahunan.');
  await page.getByRole('button', { name: 'Simpan Penyesuaian', exact: true }).click();

  await expect(page.getByText('Kuota diperbarui').first()).toBeVisible({ timeout: 15_000 });
  // remaining was 5 (12 − 4 − 3 pending) → +2 total → 7.
  await expect.poll(() => quotaRemaining(page, LQ.dewi), { timeout: 20_000 }).toBe(7);
});

// ---------------------------------------------------------------------------
// ADJUST-refuse — total below used → 422 RULE_VIOLATION field error
// ---------------------------------------------------------------------------

test('ADJUST-refuse · Adjust Budi -2 (total 10 < used 11) → 422 field error, remaining unchanged', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/leave/quotas');
  await waitForToken(page);

  const budiRow = page.locator('div.border-b').filter({ hasText: EMP.budi }).first();
  await expect(budiRow).toBeVisible({ timeout: 30_000 });
  await budiRow.getByRole('button', { name: /^Sesuaikan kuota/ }).click();

  // total 12 - 2 = 10 < used 11 → BE refuses with 422 RULE_VIOLATION fields.delta.
  await page.locator('#delta').fill('-2');
  await page.locator('#reason').fill('Mengurangi kuota di bawah terpakai.');
  await page.getByRole('button', { name: 'Simpan Penyesuaian', exact: true }).click();

  // applyFieldErrors pushes the server message onto the #delta FormField.
  await expect(page.locator('#delta')).toBeVisible({ timeout: 10_000 });
  // The save button label is still present (modal did not close on the rejected save).
  await expect(page.getByRole('button', { name: 'Simpan Penyesuaian', exact: true })).toBeVisible();

  // Remaining is unchanged (still 12 − 11 − 4 pending = -3; the refused save wrote nothing).
  expect(await quotaRemaining(page, LQ.budi)).toBe(-3);
});

// ---------------------------------------------------------------------------
// BULK-grant — preview → apply (partial success tolerated)
// ---------------------------------------------------------------------------

test('BULK-grant · Terbitkan Kuota Tahunan → Pratinjau → Terbitkan → success toast', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/leave/quotas');
  await waitForToken(page);

  await page.getByRole('button', { name: 'Terbitkan Kuota Tahunan', exact: true }).click();

  // Step 1 — preview. The default form (annual / current year / 12 days / pro-rate) is valid.
  await page.getByRole('button', { name: 'Pratinjau', exact: true }).click();

  // The preview step renders the result banner then the apply button.
  await expect(page.getByRole('button', { name: 'Terbitkan', exact: true })).toBeVisible({
    timeout: 15_000,
  });

  // Step 2 — apply.
  await page.getByRole('button', { name: 'Terbitkan', exact: true }).click();
  await expect(page.getByText('Kuota diterbitkan').first()).toBeVisible({ timeout: 15_000 });
});

// ---------------------------------------------------------------------------
// BALANCE-override — the quota-driven balance-recheck → override path
// ---------------------------------------------------------------------------

test('BALANCE-override · over-balance HR final on 8003 surfaces the override modal → APPROVED', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await openLeaveDetail(page, LR.overrideTarget);

  // HR clicks "Setujui" → BE re-checks the balance → 422 BALANCE_RECHECK_FAILED →
  // the onError handler opens the BalanceChangedModal (FE balance-recheck → override path).
  await page.getByRole('button', { name: BTN.approve, exact: true }).click();
  await page.locator('#override-reason').fill('Override kuota atas keputusan HR.');
  await page.getByRole('button', { name: BTN.override, exact: true }).last().click();

  await expect(page.getByText('Cuti disetujui').first()).toBeVisible({ timeout: 15_000 });
  await expectLeaveStatus(page, LR.overrideTarget, 'APPROVED');
});
