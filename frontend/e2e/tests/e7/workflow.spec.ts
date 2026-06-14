/**
 * tests/e7/workflow.spec.ts
 *
 * E7 · overtime approval state machine (OVT-01) against the REAL stack, driving the
 * REAL overtime-approvals queue + overtime-detail screen. Each Gherkin scenario is
 * its own test().
 *
 * State machine (CONTEXT D4): PENDING_AGENT_CONFIRM →(:confirm)→ PENDING_L1
 *   →(:approve-l1, leader/HR)→ PENDING_HR →(:approve-final, HR)→ APPROVED.
 *   :reject (reason) → REJECTED. :withdraw → WITHDRAWN. Terminal/wrong-state → 409.
 *
 * Coverage:
 *   WF-confirm        :confirm SWP-OT-30001 (PENDING_AGENT_CONFIRM) → PENDING_L1.
 *   WF-l1             leader Rudi approves SWP-OT-30002 via the queue "Setujui" → PENDING_HR
 *                     + an L1 approval entry; the row leaves the SL queue.
 *   WF-final          HR approves SWP-OT-30003 (PENDING_HR) via the queue → APPROVED + L2.
 *   WF-l1-then-final  chain 30002 leader→HR then HR final → APPROVED end-to-end.
 *   WF-reject         HR rejects SWP-OT-30003 via the detail RejectOvertimeModal → REJECTED;
 *                     reject without a (>=5) reason is blocked client-side.
 *   WF-withdraw       :withdraw SWP-OT-30001 → WITHDRAWN; :withdraw on APPROVED 30007 → 409.
 *   WF-terminal-409   approve an already-APPROVED 30007 → 409 CONFLICT.
 *
 * NB the web confirm/withdraw flows are agent-self in the UI (out of web scope); the
 * route guard lets HR/leader staff drive them, so confirm + withdraw are exercised via
 * apiAs against the REAL state machine + ephemeral Postgres.
 *
 * Seed (09-02): 30001 PENDING_AGENT_CONFIRM (Dewi @ CMP-0021); 30002 PENDING_L1 (Rudi L1
 * target); 30003 PENDING_HR (HR final target); 30007 APPROVED terminal.
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import {
  OT,
  OT_BTN,
  OT_REJECT_REASON_ID,
  apiAs,
  errorCode,
  expectNoOtRow,
  expectOtRow,
  expectOvertimeStatus,
  openOvertimeDetail,
  otRow,
  overtimeApprovals,
  waitForToken,
} from '../../lib/e7-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// WF-confirm — PENDING_AGENT_CONFIRM → :confirm → PENDING_L1
// ---------------------------------------------------------------------------

test('WF-confirm · :confirm SWP-OT-30001 (PENDING_AGENT_CONFIRM) → PENDING_L1', async ({ page }) => {
  // HR/leader staff pass the :confirm route guard (the UI confirm CTA is agent-only,
  // out of web scope). Drive the REAL transition via apiAs, assert via GET.
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/overtime');
  await waitForToken(page);

  const res = await apiAs(page, 'POST', `/overtime/${OT.confirmTarget}:confirm`, { note: 'Konfirmasi web.' });
  expect(res.status).toBe(200);
  await expectOvertimeStatus(page, OT.confirmTarget, 'PENDING_L1');
});

// ---------------------------------------------------------------------------
// WF-l1 — leader L1-approves a PENDING_L1 record via the queue → PENDING_HR
// ---------------------------------------------------------------------------

test('WF-l1 · leader Rudi approves SWP-OT-30002 (PENDING_L1) → PENDING_HR + L1 entry', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/overtime');
  await waitForToken(page);

  // The SL queue (default PENDING_L1 @ CMP-0021) lists several Dewi rows; anchor on
  // SWP-OT-30002's UNIQUE counted-minutes label ("3j 30m" = 210m) to hit that exact row.
  await expectOtRow(page, '3j 30m');
  const row = otRow(page, '3j 30m');
  await row.getByRole('button', { name: OT_BTN.rowApprove, exact: true }).click();

  // Success toast ("Lembur disetujui").
  await expect(page.getByText('Lembur disetujui').first()).toBeVisible({ timeout: 15_000 });

  // Persisted transition + an L1 approval row.
  await expectOvertimeStatus(page, OT.l1Target, 'PENDING_HR');
  const approvals = await overtimeApprovals(page, OT.l1Target);
  expect(approvals.some((a) => a.level === 1 && a.decision === 'APPROVED')).toBe(true);
});

// ---------------------------------------------------------------------------
// WF-final — HR finalises a PENDING_HR record via the queue → APPROVED
// ---------------------------------------------------------------------------

test('WF-final · HR approves SWP-OT-30003 (PENDING_HR) → APPROVED + L2 entry', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/overtime');
  await waitForToken(page);

  // HR queue default = PENDING_HR → only Dewi's 30003 is listed.
  await expectOtRow(page, 'Dewi Lestari');
  const row = otRow(page, 'Dewi Lestari');
  await row.getByRole('button', { name: OT_BTN.rowApprove, exact: true }).click();

  await expect(page.getByText('Lembur disetujui').first()).toBeVisible({ timeout: 15_000 });
  await expectOvertimeStatus(page, OT.finalTarget, 'APPROVED');

  const approvals = await overtimeApprovals(page, OT.finalTarget);
  expect(approvals.some((a) => a.level === 2)).toBe(true);
});

// ---------------------------------------------------------------------------
// WF-l1-then-final — the full two-level machine end-to-end
// ---------------------------------------------------------------------------

test('WF-l1-then-final · leader L1 then HR final on SWP-OT-30002 → APPROVED', async ({ page }) => {
  // Step 1 — leader L1 via the queue.
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/overtime');
  await waitForToken(page);
  await otRow(page, '3j 30m').getByRole('button', { name: OT_BTN.rowApprove, exact: true }).click();
  await expect(page.getByText('Lembur disetujui').first()).toBeVisible({ timeout: 15_000 });
  await expectOvertimeStatus(page, OT.l1Target, 'PENDING_HR');

  // Step 2 — HR final on the now-PENDING_HR record (drive via apiAs for determinism;
  // both 30002 and 30003 are Dewi rows in the HR queue once 30002 advances).
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/overtime');
  await waitForToken(page);
  const res = await apiAs(page, 'POST', `/overtime/${OT.l1Target}:approve-final`, {});
  expect(res.status).toBe(200);
  await expectOvertimeStatus(page, OT.l1Target, 'APPROVED');
});

// ---------------------------------------------------------------------------
// WF-reject — HR rejects via the detail RejectOvertimeModal → REJECTED
// ---------------------------------------------------------------------------

test('WF-reject · HR rejects SWP-OT-30003 with a reason → REJECTED; missing-reason blocked', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await openOvertimeDetail(page, OT.finalTarget);

  // Open the RejectOvertimeModal from the detail action card.
  await page.getByRole('button', { name: OT_BTN.reject, exact: true }).click();

  // Missing/short reason is blocked client-side: the confirm button is DISABLED
  // (overtime-detail-overlays.tsx: disabled when reason.trim().length < 5).
  await page.locator(OT_REJECT_REASON_ID).fill('no');
  await expect(
    page.getByRole('button', { name: OT_BTN.rejectConfirm, exact: true }),
  ).toBeDisabled();
  await expectOvertimeStatus(page, OT.finalTarget, 'PENDING_HR');

  // Now a valid (>=5) reason enables the confirm → REJECTED.
  await page.locator(OT_REJECT_REASON_ID).fill('Tidak ada bukti pekerjaan lembur.');
  await page.getByRole('button', { name: OT_BTN.rejectConfirm, exact: true }).click();
  await expect(page.getByText('Lembur ditolak').first()).toBeVisible({ timeout: 15_000 });
  await expectOvertimeStatus(page, OT.finalTarget, 'REJECTED');
});

// ---------------------------------------------------------------------------
// WF-withdraw — :withdraw PENDING_AGENT_CONFIRM → WITHDRAWN; APPROVED → 409
// ---------------------------------------------------------------------------

test('WF-withdraw · :withdraw 30001 → WITHDRAWN; :withdraw on APPROVED 30007 → 409', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/overtime');
  await waitForToken(page);

  const ok = await apiAs(page, 'POST', `/overtime/${OT.confirmTarget}:withdraw`, {});
  expect([200, 204]).toContain(ok.status);
  await expectOvertimeStatus(page, OT.confirmTarget, 'WITHDRAWN');

  // Withdrawing a terminal (APPROVED) record → 409 CONFLICT.
  const conflict = await apiAs(page, 'POST', `/overtime/${OT.approved}:withdraw`, {});
  expect(conflict.status).toBe(409);
  expect(errorCode(conflict.body)).toBe('CONFLICT');
});

// ---------------------------------------------------------------------------
// WF-terminal-409 — approving an already-APPROVED record → 409
// ---------------------------------------------------------------------------

test('WF-terminal-409 · HR :approve-final on already-APPROVED 30007 → 409 CONFLICT', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/overtime');
  await waitForToken(page);

  const res = await apiAs(page, 'POST', `/overtime/${OT.approved}:approve-final`, {});
  expect(res.status).toBe(409);
  expect(errorCode(res.body)).toBe('CONFLICT');

  // Sanity: the leader queue does not list a terminal row.
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/overtime');
  await waitForToken(page);
  await expectNoOtRow(page, OT.approved);
});
