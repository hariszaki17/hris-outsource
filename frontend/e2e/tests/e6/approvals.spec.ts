/**
 * tests/e6/approvals.spec.ts
 *
 * E6 · leave approval state machine (LVE-01) against the REAL stack, driving the
 * REAL leave-detail-screen + approvals queue. Each Gherkin scenario is its own test().
 *
 * Coverage:
 *   L1-FORWARD       SL Rudi forwards 8001 (PENDING_L1 → PENDING_HR) → toast + persisted PENDING_HR.
 *   HR-FINAL         HR approves the leader-approved 8002 (PENDING_HR → APPROVED) + timeline L1+HR.
 *   L1-THEN-FINAL    SL forwards 8001 then HR finalises the same request → APPROVED.
 *   REJECT-happy     HR rejects 8002 with a reason (>=5) → REJECTED + reason in the timeline.
 *   REJECT-min       reject reason <5 chars shows the min-length error and does not submit.
 *   OVERRIDE-happy   HR over-balance approve of 8003 → BalanceChangedModal → #override-reason (>=10)
 *                    → OVERRIDE_APPROVED (status APPROVED).
 *   OVERRIDE-min     override reason <10 chars is blocked (min-length error, no submit).
 *   LIST-default     HR /leave defaults to PENDING_HR rows (8002/8003/8007); status=APPROVED → 8005.
 *   NO-LEADER        8003 (CMP-0022, no leader) renders the no-leader badge.
 *
 * Seed (08-02): 8001 Dewi PENDING_L1 (Rudi L1 target); 8002 Dewi PENDING_HR (leader-approved);
 * 8003 Budi PENDING_HR over-balance (no_leader); 8005 APPROVED; 8007 Dewi PENDING_HR.
 */

import {
  BTN,
  LR,
  expectLeaveRow,
  expectLeaveStatus,
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
// L1-FORWARD — SL forwards a PENDING_L1 request → PENDING_HR
// ---------------------------------------------------------------------------

test('L1-FORWARD · SL Rudi forwards 8001 (PENDING_L1) → toast + persisted PENDING_HR', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await openLeaveDetail(page, LR.l1Target);

  await page.getByRole('button', { name: BTN.forward, exact: true }).click();

  // Success toast ("Diteruskan ke HR").
  await expect(page.getByText('Diteruskan ke HR').first()).toBeVisible({ timeout: 15_000 });

  // Persisted transition.
  await expectLeaveStatus(page, LR.l1Target, 'PENDING_HR');
});

// ---------------------------------------------------------------------------
// HR-FINAL — HR finalises a leader-approved PENDING_HR request → APPROVED
// ---------------------------------------------------------------------------

test('HR-FINAL · HR approves the leader-approved 8002 → APPROVED + L1/HR timeline', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await openLeaveDetail(page, LR.finalTarget);

  await page.getByRole('button', { name: BTN.approve, exact: true }).click();

  await expect(page.getByText('Cuti disetujui').first()).toBeVisible({ timeout: 15_000 });
  await expectLeaveStatus(page, LR.finalTarget, 'APPROVED');

  // Timeline now shows both the Shift-Leader (L1) and HR Admin stages.
  await expect(page.getByText('Pemimpin Shift').first()).toBeVisible({ timeout: 10_000 });
  await expect(page.getByText('HR Admin').first()).toBeVisible();
});

// ---------------------------------------------------------------------------
// L1-THEN-FINAL — the full two-level machine end-to-end
// ---------------------------------------------------------------------------

test('L1-THEN-FINAL · SL forwards 8001 then HR finalises the same request → APPROVED', async ({
  page,
}) => {
  // Step 1 — SL forward.
  await loginAs(page, PERSONAS.shiftLeader);
  await openLeaveDetail(page, LR.l1Target);
  await page.getByRole('button', { name: BTN.forward, exact: true }).click();
  await expect(page.getByText('Diteruskan ke HR').first()).toBeVisible({ timeout: 15_000 });
  await expectLeaveStatus(page, LR.l1Target, 'PENDING_HR');

  // Step 2 — HR final on the now-PENDING_HR request.
  await loginAs(page, PERSONAS.hrAdmin);
  await openLeaveDetail(page, LR.l1Target);
  await page.getByRole('button', { name: BTN.approve, exact: true }).click();
  await expect(page.getByText('Cuti disetujui').first()).toBeVisible({ timeout: 15_000 });
  await expectLeaveStatus(page, LR.l1Target, 'APPROVED');
});

// ---------------------------------------------------------------------------
// REJECT-happy — HR rejects with a reason → REJECTED
// ---------------------------------------------------------------------------

test('REJECT-happy · HR rejects 8002 with a reason → REJECTED + reason in the timeline', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await openLeaveDetail(page, LR.finalTarget);

  // Open the RejectLeaveModal.
  await page.getByRole('button', { name: BTN.reject, exact: true }).click();
  await page.locator('#reject-reason').fill('Coverage tidak mencukupi pada periode ini.');
  await page.getByRole('button', { name: BTN.rejectConfirm, exact: true }).click();

  await expect(page.getByText('Permintaan ditolak').first()).toBeVisible({ timeout: 15_000 });
  await expectLeaveStatus(page, LR.finalTarget, 'REJECTED');

  // The rejection reason surfaces in the approval timeline.
  await expect(page.getByText(/Coverage tidak mencukupi/).first()).toBeVisible({ timeout: 10_000 });
});

// ---------------------------------------------------------------------------
// REJECT-min — reason < 5 chars is blocked client-side
// ---------------------------------------------------------------------------

test('REJECT-min · reject reason under 5 chars shows the min-length error and does not submit', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await openLeaveDetail(page, LR.finalTarget);

  await page.getByRole('button', { name: BTN.reject, exact: true }).click();
  await page.locator('#reject-reason').fill('no');
  await page.getByRole('button', { name: BTN.rejectConfirm, exact: true }).click();

  // Min-length validation message appears; no toast, status unchanged.
  await expect(page.getByText('Alasan minimal 5 karakter.').first()).toBeVisible({
    timeout: 10_000,
  });
  await expectLeaveStatus(page, LR.finalTarget, 'PENDING_HR');
});

// ---------------------------------------------------------------------------
// OVERRIDE-happy — HR over-balance approve via the BalanceChangedModal
// ---------------------------------------------------------------------------

test('OVERRIDE-happy · HR over-balance approve of 8003 → override modal → APPROVED', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await openLeaveDetail(page, LR.overrideTarget);

  // 8003 is over-balance (Budi: remaining < 3 requested), but the detail GET does not
  // pre-flag balance_check.requires_override — the BE only re-checks at :approve-final.
  // So the HR CTA is the plain "Setujui": clicking it hits 422 BALANCE_RECHECK_FAILED,
  // whose onError handler opens the BalanceChangedModal (the Task-1 error.code fix).
  await page.getByRole('button', { name: BTN.approve, exact: true }).click();

  // The override modal opens off the 422; fill #override-reason (>=10) + confirm.
  await page.locator('#override-reason').fill('Disetujui HR atas pertimbangan operasional.');
  await page.getByRole('button', { name: BTN.override, exact: true }).last().click();

  await expect(page.getByText('Cuti disetujui').first()).toBeVisible({ timeout: 15_000 });
  await expectLeaveStatus(page, LR.overrideTarget, 'APPROVED');
});

// ---------------------------------------------------------------------------
// OVERRIDE-min — override reason < 10 chars is blocked client-side
// ---------------------------------------------------------------------------

test('OVERRIDE-min · override reason under 10 chars is blocked (min-length error, no submit)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await openLeaveDetail(page, LR.overrideTarget);

  // Open the override modal via the 422 error path, then submit a too-short reason.
  await page.getByRole('button', { name: BTN.approve, exact: true }).click();
  await page.locator('#override-reason').fill('singkat'); // 7 chars < 10
  await page.getByRole('button', { name: BTN.override, exact: true }).last().click();

  await expect(page.getByText('Alasan minimal 10 karakter.').first()).toBeVisible({
    timeout: 10_000,
  });
  await expectLeaveStatus(page, LR.overrideTarget, 'PENDING_HR');
});

// ---------------------------------------------------------------------------
// LIST-default — HR queue defaults to PENDING_HR; status filter to APPROVED
// ---------------------------------------------------------------------------

test('LIST-default · HR /leave shows PENDING_HR rows; APPROVED filter surfaces 8005', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/leave');
  await waitForToken(page);

  // Default status = PENDING_HR → the three seeded PENDING_HR requests are listed.
  // The queue renders employee_name (not the LR id), so anchor rows on the names:
  //   8002 + 8007 = Dewi Lestari @ Plaza Senayan; 8003 = Budi Santoso @ Mall Kelapa Gading.
  await expectLeaveRow(page, 'Dewi Lestari');
  await expectLeaveRow(page, 'Budi Santoso');
  // Exactly three PENDING_HR rows are listed (8002 + 8003 + 8007).
  await expect(page.locator('div.border-b').filter({ hasText: 'Cuti Tahunan' })).toHaveCount(3, {
    timeout: 20_000,
  });

  // Switch the status filter to APPROVED → the seeded APPROVED requests surface.
  // The ledger backfill now plants three APPROVED "Cuti Tahunan" requests:
  // 8005 (Dewi, 1d) + 8008 (Dewi, 3d, history) @ CMP-0021 and 8009 (Budi, 11d,
  // history) @ CMP-0022.
  await page.getByLabel('Semua status').selectOption('APPROVED');
  await expect(page.locator('div.border-b').filter({ hasText: 'Cuti Tahunan' })).toHaveCount(3, {
    timeout: 20_000,
  });
  await expectLeaveRow(page, 'Dewi Lestari');
  await expectLeaveRow(page, 'Budi Santoso');
});

// ---------------------------------------------------------------------------
// NO-LEADER — 8003 (CMP-0022, no leader) renders the no-leader badge
// ---------------------------------------------------------------------------

test('NO-LEADER · 8003 (CMP-0022, no shift leader) renders the no-leader badge', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await openLeaveDetail(page, LR.overrideTarget);

  await expect(page.getByText('Tanpa pemimpin shift').first()).toBeVisible({ timeout: 15_000 });
});
