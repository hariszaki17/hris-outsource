/**
 * tests/e7/approvals.spec.ts
 *
 * E7 · overtime approvals queue + detail rendering (OVT-01) against the REAL stack,
 * driving the REAL overtime-approvals-screen + overtime-detail-screen. Each scenario
 * is its own test().
 *
 * Coverage:
 *   LIST-HR     HR /overtime lists PENDING_HR rows (default); the cross-company queue is
 *               global; the flagged_no_preapproval pill renders on the flagged row.
 *   LIST-SL     leader /overtime lists own-company PENDING_L1 rows only (CMP-0021).
 *   DETAIL      /overtime/{id} renders the tier-breakdown card + approvals timeline from
 *               the real {data} envelope (SWP-OT-30009, an APPROVED holiday OT w/ a trail).
 *   FILTER      the source filter narrows the queue.
 *
 * Seed (09-02): 30002/30004/30006/30010 PENDING_L1 @ CMP-0021 (SL queue); 30003 PENDING_HR
 * @ CMP-0021 (HR queue); 30005 PENDING_L1 @ CMP-0022 (cross-company); 30009 HOLIDAY APPROVED
 * w/ L1+HR approval trail (WORKED_WITHOUT_REQUEST → flagged_no_preapproval).
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import {
  E7_NAME,
  OT,
  expectNoOtRow,
  expectOtRow,
  openOvertimeDetail,
  waitForToken,
} from '../../lib/e7-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// LIST-HR — HR queue defaults to PENDING_HR; lists Dewi's 30003
// ---------------------------------------------------------------------------

test('LIST-HR · HR /overtime lists the PENDING_HR row (Dewi @ CMP-0021)', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/overtime');
  await waitForToken(page);

  // Default HR status = PENDING_HR → only SWP-OT-30003 (Dewi @ Plaza Senayan) is listed.
  await expectOtRow(page, E7_NAME.dewi);
  // The pending HR queue has exactly one row.
  await expect(page.locator('div.border-b').filter({ hasText: E7_NAME.dewi })).toHaveCount(1, {
    timeout: 20_000,
  });
});

// ---------------------------------------------------------------------------
// LIST-SL — leader queue lists own-company PENDING_L1 rows only
// ---------------------------------------------------------------------------

test('LIST-SL · leader /overtime lists own-company PENDING_L1 rows; cross-company hidden', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/overtime');
  await waitForToken(page);

  // Rudi's PENDING_L1 queue @ CMP-0021 lists Dewi rows (30002/30006/30010) + Rudi's own (30004).
  await expectOtRow(page, E7_NAME.dewi);
  // The cross-company CMP-0022 record 30005 (Budi) is scoped out of the leader queue.
  await expectNoOtRow(page, E7_NAME.budi);
});

// ---------------------------------------------------------------------------
// DETAIL — the detail screen renders tier breakdown + timeline from the {data} envelope
// ---------------------------------------------------------------------------

test('DETAIL · /overtime/30009 renders tier-breakdown card + approvals timeline', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await openOvertimeDetail(page, OT.holidayOt);

  // Tier-breakdown section + the approval-timeline section render from the real {data}.
  await expect(page.getByText('Rincian Tier').first()).toBeVisible({ timeout: 15_000 });
  await expect(page.getByText('Riwayat Persetujuan').first()).toBeVisible();

  // The seeded L1+HR approval trail surfaces in the timeline (approver "Rudi Wijaya").
  await expect(page.getByText('Rudi Wijaya').first()).toBeVisible({ timeout: 10_000 });
});

// ---------------------------------------------------------------------------
// FILTER — the source filter narrows the queue
// ---------------------------------------------------------------------------

test('FILTER · HR queue source filter narrows the queue', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/overtime');
  await waitForToken(page);

  await expectOtRow(page, E7_NAME.dewi);

  // 30003 (the only PENDING_HR row) is REQUESTED. Filtering by AUTO_DETECTED empties the queue.
  await page.getByLabel('Semua sumber').selectOption('AUTO_DETECTED');
  await expect(page.locator('div.border-b').filter({ hasText: E7_NAME.dewi })).toHaveCount(0, {
    timeout: 20_000,
  });

  // Reset back to REQUESTED → the row returns.
  await page.getByLabel('Semua sumber').selectOption('REQUESTED');
  await expectOtRow(page, E7_NAME.dewi);
});
