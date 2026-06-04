/**
 * tests/e4/bulk-apply.spec.ts
 *
 * E4 · bulk apply-to-range (SA-3 / C-1) — partial-success proof against the REAL stack.
 * The BulkApplyModal: pick shift → start/end dates → weekday toggles → "Pratinjau" →
 * "Terapkan". Partial success = some cells applied, >=1 skipped with a conflict code.
 *
 * Coverage:
 *   BULK-partial       UI preview shows succeeded>0 AND failed>0 (range spans Dewi's approved-leave
 *                      Thu so >=1 cell fails SHIFT_OVER_LEAVE) → Terapkan → applied; apiAs cross-check
 *                      asserts :bulk-apply 200, succeeded non-empty, failed[0].error.code SHIFT_OVER_LEAVE
 *   BULK-all-failed    every cell outside the placement window → :bulk-apply 422, failed non-empty, succeeded empty
 *   BULK-weekdays-mask weekdays_mask Mon-Fri over a 7-day range → only weekday cells attempted
 *   CHECK-dry-run      :check (single conflicting) → 200 with failed[0].error.code AND no new entry persisted
 *
 * Seeded anchors (06-02): EMP-3001 approved-leave Thu = monday+3 (SWP-LR-44210);
 * EMP-3001 placement SWP-PL-5004 window = today-100 .. today+20. Login shiftLeader (Rudi @ CMP-0021).
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import { SEED, addDaysIso, apiAs, selectCompany, waitForToken } from '../../lib/e4-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

const BULK = { kind: 'bulk' as const };

// ---------------------------------------------------------------------------
// BULK-partial — preview succeeded>0 AND failed>0, then apply (UI + apiAs)
// ---------------------------------------------------------------------------

test('BULK-partial · range spanning the approved-leave Thu → preview shows succeeded>0 AND failed>0; apply persists the rest', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/schedule');
  await selectCompany(page, 'Plaza Senayan');
  // Grid renders Rudi + Dewi (seeded entries) — both feed the bulk modal's employee_ids.
  await expect(page.getByText('Dewi Lestari').first()).toBeVisible({ timeout: 30_000 });

  const start = SEED.dewiLeaveDate(); // monday+3 (Thu) — Dewi's leave day → 1 cell fails over-leave
  const end = SEED.freeDate(); // monday+4 (Fri)

  // Open the bulk modal.
  await page.getByRole('button', { name: 'Terapkan ke rentang' }).click();
  // Pick "Pagi" from the modal shift list (search then click the row).
  await page.getByPlaceholder('Cari shift…').first().fill('Pagi');
  await page.getByRole('button', { name: /^Pagi/ }).first().click();
  // Date range (type=date inputs).
  await page.locator('input[type="date"]').first().fill(start);
  await page.locator('input[type="date"]').nth(1).fill(end);

  // Preview → the info banner shows "<succeeded> jadwal akan dibuat · <failed> dilewati".
  await page.getByRole('button', { name: 'Pratinjau' }).click();
  // At least one skipped (Dewi over-leave) → "dilewati" with a non-zero count somewhere.
  await expect(page.getByText(/jadwal akan dibuat ·/i).first()).toBeVisible({ timeout: 15_000 });
  await expect(page.getByText(/0 dilewati/i)).toHaveCount(0, { timeout: 5_000 });

  // Apply → success toast.
  await page.getByRole('button', { name: /^Terapkan \(/ }).click();
  await expect(page.getByText(/berhasil diterapkan/i).first()).toBeVisible({ timeout: 15_000 });

  // apiAs cross-check: the same range over EMP-3001 yields 200 with succeeded AND a failed
  // SHIFT_OVER_LEAVE cell.
  await waitForToken(page);
  const res = await apiAs(page, 'POST', '/schedule:bulk-apply', {
    ...BULK,
    shift_master_id: 'SWP-SHF-001',
    start_date: start,
    end_date: addDaysIso(start, 2),
    employee_ids: ['SWP-EMP-3001'],
  });
  expect(res.status).toBe(200);
  const body = res.body as {
    succeeded?: unknown[];
    failed?: Array<{ error?: { code?: string } }>;
  };
  expect((body.succeeded ?? []).length).toBeGreaterThan(0);
  expect((body.failed ?? []).some((f) => f.error?.code === 'SHIFT_OVER_LEAVE')).toBe(true);
});

// ---------------------------------------------------------------------------
// BULK-all-failed — entire range outside the placement window → 422
// ---------------------------------------------------------------------------

test('BULK-all-failed · range entirely outside the placement window → :bulk-apply 422 (failed non-empty, succeeded empty)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/schedule');
  await waitForToken(page);

  const res = await apiAs(page, 'POST', '/schedule:bulk-apply', {
    ...BULK,
    shift_master_id: 'SWP-SHF-001',
    start_date: '2030-01-01',
    end_date: '2030-01-03',
    employee_ids: ['SWP-EMP-1108'],
  });
  expect(res.status).toBe(422);
  const body = res.body as { succeeded?: unknown[]; failed?: unknown[] };
  expect((body.failed ?? []).length).toBeGreaterThan(0);
  expect((body.succeeded ?? []).length).toBe(0);
});

// ---------------------------------------------------------------------------
// BULK-weekdays-mask — Mon-Fri over a 7-day range → only weekday cells attempted
// ---------------------------------------------------------------------------

test('BULK-weekdays-mask · weekdays_mask [1..5] over a Mon-Sun range → only 5 weekday cells attempted', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/schedule');
  await waitForToken(page);

  // A full Mon..Sun week INSIDE Rudi's placement (whole 2026). Use the current grid week.
  const monday = SEED.monday();
  const sunday = addDaysIso(monday, 6);
  const res = await apiAs(page, 'POST', '/schedule:check', {
    ...BULK,
    shift_master_id: 'SWP-SHF-001',
    start_date: monday,
    end_date: sunday,
    employee_ids: ['SWP-EMP-1108'],
    weekdays_mask: [1, 2, 3, 4, 5],
  });
  expect(res.status).toBe(200);
  const body = res.body as { succeeded?: unknown[]; failed?: unknown[] };
  // :check is side-effect-free; exactly 5 weekday cells are evaluated (succeeded + failed).
  const attempted = (body.succeeded ?? []).length + (body.failed ?? []).length;
  expect(attempted).toBe(5);
});

// ---------------------------------------------------------------------------
// CHECK-dry-run — :check has NO side effect
// ---------------------------------------------------------------------------

test('CHECK-dry-run · :check (single, conflicting) → 200 with failed[].error.code AND no entry persisted', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/schedule');
  await waitForToken(page);

  const leaveDate = SEED.dewiLeaveDate();

  // Dry-run check for Dewi on her leave date → 200 with a failed over-leave cell.
  const check = await apiAs(page, 'POST', '/schedule:check', {
    kind: 'single',
    employee_id: 'SWP-EMP-3001',
    shift_master_id: 'SWP-SHF-001',
    date: leaveDate,
    is_day_off: false,
  });
  expect(check.status).toBe(200);
  const cb = check.body as { failed?: Array<{ error?: { code?: string } }> };
  expect((cb.failed ?? [])[0]?.error?.code).toBe('SHIFT_OVER_LEAVE');

  // No entry was persisted for that cell (the grid list shows nothing new).
  const list = await apiAs(
    page,
    'GET',
    `/schedule?company_id=SWP-CMP-0021&start_date=${leaveDate}&end_date=${leaveDate}&employee_id=SWP-EMP-3001`,
  );
  expect(list.status).toBe(200);
  const lb = list.body as { data?: unknown[] };
  expect((lb.data ?? []).length).toBe(0);
});
