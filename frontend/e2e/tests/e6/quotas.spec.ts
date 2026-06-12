/**
 * tests/e6/quotas.spec.ts
 *
 * E6 · leave quota management (LVE-02) against the REAL stack, driving the REAL
 * leave-quotas-screen — redesigned 2026-06-08 into the F6.1 grant-lot LEDGER.
 * Each scenario is its own test().
 *
 * The screen is now a per-employee POOL aggregate (GET /leave-balances) with a
 * drill-in to the per-lot table (GET /leave-grants). Balances are Σ over the
 * employee's LeaveGrant lots (each amount/consumed/pending/expires); the legacy
 * flat `leave_quotas` table + bulk-grant/adjust-by-delta endpoints are deprecated.
 *
 * Coverage:
 *   LIST-remaining   HR /leave/quotas shows Dewi (pool_remaining 8) + Budi (pool_remaining 1).
 *                    pool_remaining = Σ(amount − consumed − pending) over un-earmarked lots.
 *   ADJUST-happy     Drill into Dewi → Sesuaikan the ANNUAL lot → #adj-amount=14 / #adj-remark →
 *                    "Simpan" → "Kuota diperbarui" toast → lot remaining 10, pool_remaining 10.
 *   ADJUST-refuse    Drill into Budi → Sesuaikan the ANNUAL lot → #adj-amount=10 (< consumed 11)
 *                    → 422 RULE_VIOLATION field error on #adj-amount; modal stays open; pool unchanged.
 *   BULK-grant       Tambah Kuota (grant a new lot) → fill the grant form → "Simpan" →
 *                    "Kuota ditambahkan" toast (the grant-lot replacement for the old bulk issuance).
 *   BALANCE-override over-balance HR final on 8003 surfaces the override modal → APPROVED
 *                    (the FE balance-recheck → override path, quota-driven).
 *
 * Seed (08-02, grant-lot ledger): SWP-LG-8001 Dewi ANNUAL amount 12 / consumed 4 / pending 0
 * → pool_remaining 8; SWP-LG-8002 Budi ANNUAL amount 12 / consumed 11 / pending 0 →
 * pool_remaining 1; SWP-LG-8003 Dewi MATERNITY earmark 90. SWP-LR-8003 Budi PENDING_HR over-balance.
 *
 * Screen DOM (leave-quotas-screen.tsx): list rows = div.border-b (employee_id mono); row click
 * drills in (?employee_id=…). Per-lot "Sesuaikan" Button aria "Sesuaikan lot {{id}}"
 * (t('actions.adjustAriaLabel')). Adjust modal: #adj-amount (number, absolute new amount),
 * #adj-remark (textarea), save "Simpan" (t('adjust.saveBtn')); success toast "Kuota diperbarui"
 * (t('adjust.successTitle')). Add quota: header "Tambah Kuota" (t('actions.grantLot')) → grant form
 * (#g-amount, #g-expires, #g-remark) → "Simpan" (t('grant.saveBtn')); toast "Kuota ditambahkan".
 */

import {
  BTN,
  EMP,
  LG,
  LR,
  balanceRemaining,
  expectLeaveStatus,
  grantRemaining,
  openLeaveDetail,
  waitForToken,
} from '../../lib/e6-helpers.js';
import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// LIST-remaining — the per-employee balance table renders the seeded pool math
// ---------------------------------------------------------------------------

test('LIST-remaining · HR /leave/quotas shows Dewi (pool_remaining 8) + Budi (pool_remaining 1)', async ({
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

  // Cross-check the persisted POOL remaining via the API (deterministic).
  // pool_remaining = Σ(amount − consumed − pending) over un-earmarked lots.
  expect(await balanceRemaining(page, EMP.dewi)).toBe(8);
  expect(await balanceRemaining(page, EMP.budi)).toBe(1);
});

// ---------------------------------------------------------------------------
// ADJUST-happy — set Dewi's ANNUAL lot to 14 → lot remaining 10, pool 10
// ---------------------------------------------------------------------------

test('ADJUST-happy · Adjust Dewi ANNUAL lot to 14 with a remark → success toast + persisted remaining 10', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/leave/quotas');
  await waitForToken(page);

  // Drill into Dewi's lot ledger.
  const dewiRow = page.locator('div.border-b').filter({ hasText: EMP.dewi }).first();
  await expect(dewiRow).toBeVisible({ timeout: 30_000 });
  await dewiRow.click();

  // Sesuaikan the ANNUAL lot (aria "Sesuaikan lot SWP-LG-8001").
  await page.getByRole('button', { name: `Sesuaikan lot ${LG.dewiAnnual}` }).click();

  // The adjust modal sets the ABSOLUTE new amount_days (not a delta). 14 − 4 consumed → 10.
  await page.locator('#adj-amount').fill('14');
  await page.locator('#adj-remark').fill('Penyesuaian kuota sisa cuti tahunan.');
  await page.getByRole('button', { name: 'Simpan', exact: true }).click();

  await expect(page.getByText('Kuota diperbarui').first()).toBeVisible({ timeout: 15_000 });
  // Lot remaining was 8 (12 − 4 − 0) → amount 14 → remaining 10; pool_remaining → 10.
  await expect
    .poll(() => grantRemaining(page, EMP.dewi, LG.dewiAnnual), { timeout: 20_000 })
    .toBe(10);
  await expect.poll(() => balanceRemaining(page, EMP.dewi), { timeout: 20_000 }).toBe(10);
});

// ---------------------------------------------------------------------------
// ADJUST-refuse — amount below consumed → 422 RULE_VIOLATION field error
// ---------------------------------------------------------------------------

test('ADJUST-refuse · Adjust Budi ANNUAL lot to 10 (< consumed 11) → 422 field error, remaining unchanged', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/leave/quotas');
  await waitForToken(page);

  // Drill into Budi's lot ledger.
  const budiRow = page.locator('div.border-b').filter({ hasText: EMP.budi }).first();
  await expect(budiRow).toBeVisible({ timeout: 30_000 });
  await budiRow.click();

  // Sesuaikan the ANNUAL lot (aria "Sesuaikan lot SWP-LG-8002").
  await page.getByRole('button', { name: `Sesuaikan lot ${LG.budiAnnual}` }).click();

  // amount_days 10 < consumed 11 → BE refuses with 422 RULE_VIOLATION fields.amount_days.
  await page.locator('#adj-amount').fill('10');
  await page.locator('#adj-remark').fill('Mengurangi kuota di bawah terpakai.');
  await page.getByRole('button', { name: 'Simpan', exact: true }).click();

  // applyFieldErrors pushes the server message onto the #adj-amount FormField; the modal
  // stays open (the save button label is still present) on the rejected save.
  await expect(page.locator('#adj-amount')).toBeVisible({ timeout: 10_000 });
  await expect(page.getByRole('button', { name: 'Simpan', exact: true })).toBeVisible();

  // Remaining is unchanged (still 12 − 11 − 0 = 1; the refused save wrote nothing).
  expect(await balanceRemaining(page, EMP.budi)).toBe(1);
});

// ---------------------------------------------------------------------------
// BULK-grant — grant a NEW lot via "Tambah Kuota" (the grant-lot replacement)
// ---------------------------------------------------------------------------

test('BULK-grant · Tambah Kuota → grant a new lot for Dewi → success toast', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/leave/quotas');
  await waitForToken(page);

  // Open the grant-lot modal from the list header.
  await page.getByRole('button', { name: 'Tambah Kuota', exact: true }).click();

  // The Combobox trigger shows the placeholder until opened; click it to reveal the
  // search input, then type to resolve the employee via the typeahead.
  await page.getByRole('button', { name: 'Ketik nama, NIK, atau NIP…' }).click();
  const search = page.getByPlaceholder('Ketik nama, NIK, atau NIP…');
  await expect(search).toBeVisible({ timeout: 15_000 });
  await search.fill('Dewi');
  // The option list is debounced/async; click Dewi's option once it renders.
  await page.getByRole('button', { name: /Dewi Lestari/ }).click();

  // Fill the grant lot: amount, expiry, required remark.
  await page.locator('#g-amount').fill('5');
  await page.locator('#g-expires').fill('2026-12-31');
  await page.locator('#g-remark').fill('Hibah kuota tambahan untuk periode berjalan.');
  await page.getByRole('button', { name: 'Simpan', exact: true }).click();

  await expect(page.getByText('Kuota ditambahkan').first()).toBeVisible({ timeout: 15_000 });
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
