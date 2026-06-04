/**
 * tests/e5/corrections.spec.ts
 *
 * E5 · corrections approve / reject (F5.4) against the REAL stack, driving the REAL
 * corrections-screen + CorrectionDetailDrawer + RejectCorrectionModal.
 *
 * Coverage:
 *   COR-list     HR /corrections lists the PENDING corrections 8001 + 8002.
 *   COR-approve  click 8001 row → drawer (diff table renders) → Setujui → success toast;
 *                the correction → APPLIED AND the target attendance 9004 gains CORRECTED.
 *   COR-reject   click 8002 row → drawer → Tolak → RejectCorrectionModal reason (>=5) →
 *                Konfirmasi tolak → success toast; status → REJECTED.
 *
 * Selectors (correction-overlays.tsx): DataTable rows = div.border-b (filtered by the
 * attendance_id rendered mono); drawer footer Approve = t('corrections.approve')="Setujui",
 * Reject = t('corrections.reject')="Tolak"; RejectCorrectionModal textarea id="reject-reason",
 * submit = t('corrections.rejectConfirm')="Konfirmasi tolak".
 * Seed (07-02): 8001 PENDING/CHECK_OUT on 9004 (approve → 9004 CORRECTED); 8002 PENDING/CHECK_IN
 * on 9002 (reject). Both requester EMP-3001 → match rows by attendance_id.
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import { ATT, COR, apiAs, waitForToken } from '../../lib/e5-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

/** A corrections DataTable row (div.border-b) matched by the attendance_id it renders. */
function corRow(page: import('@playwright/test').Page, attendanceId: string) {
  return page.locator('div.border-b').filter({ hasText: attendanceId }).first();
}

// ---------------------------------------------------------------------------
// COR-list — PENDING corrections listed
// ---------------------------------------------------------------------------

test('COR-list · HR corrections queue lists the seeded PENDING corrections (8001 + 8002)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/corrections');
  await waitForToken(page);

  // 8001 targets 9004, 8002 targets 9002 — both rendered as the attendance_id mono.
  await expect(corRow(page, ATT.autoClosedCmp21)).toBeVisible({ timeout: 30_000 });
  await expect(corRow(page, ATT.lateCmp21)).toBeVisible({ timeout: 30_000 });
});

// ---------------------------------------------------------------------------
// COR-approve — drawer Setujui → APPLIED + target attendance CORRECTED
// ---------------------------------------------------------------------------

test('COR-approve · approve 8001 → drawer diff renders → Setujui → APPLIED and attendance 9004 gains CORRECTED', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/corrections');
  await waitForToken(page);

  // Open the 8001 (→ 9004) row drawer.
  await corRow(page, ATT.autoClosedCmp21).click();

  // The drawer + diff table render — assert the diff row that is UNIQUE to the drawer
  // (the CHECK_OUT field key); the type label "Koreksi jam keluar" also appears in the
  // dimmed list row behind the overlay, so anchor on the diff-table content instead.
  await expect(page.getByText('SWP-COR-8001', { exact: false }).first()).toBeVisible({
    timeout: 15_000,
  });
  await expect(page.getByText('check_out_at', { exact: false }).first()).toBeVisible({
    timeout: 15_000,
  });

  // Approve (drawer footer Setujui).
  await page.getByRole('button', { name: 'Setujui', exact: true }).click();
  await expect(page.getByText('Koreksi berhasil disetujui.').first()).toBeVisible({
    timeout: 15_000,
  });

  // The correction → APPLIED.
  const cor = await apiAs(page, 'GET', `/corrections/${COR.approveTarget}`);
  expect(cor.status).toBe(200);
  expect((cor.body as { data?: { status?: string } }).data?.status).toBe('APPLIED');

  // The target attendance 9004 now carries the CORRECTED flag.
  const att = await apiAs(page, 'GET', `/attendance/${ATT.autoClosedCmp21}`);
  expect(att.status).toBe(200);
  const flags = (att.body as { data?: { flags?: string[] } }).data?.flags ?? [];
  expect(flags).toContain('CORRECTED');
});

// ---------------------------------------------------------------------------
// COR-reject — drawer Tolak → modal reason → REJECTED
// ---------------------------------------------------------------------------

test('COR-reject · reject 8002 via the reject modal → success toast → status REJECTED', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/corrections');
  await waitForToken(page);

  await corRow(page, ATT.lateCmp21).click();

  // Open the RejectCorrectionModal (drawer footer Tolak).
  await page.getByRole('button', { name: 'Tolak', exact: true }).click();

  // Fill the reason textarea (>=5 chars) and confirm.
  await page.locator('#reject-reason').fill('Bukti tidak mendukung klaim jam masuk');
  await page.getByRole('button', { name: 'Konfirmasi tolak', exact: true }).click();

  await expect(page.getByText('Koreksi berhasil ditolak.').first()).toBeVisible({
    timeout: 15_000,
  });

  // The correction → REJECTED.
  const cor = await apiAs(page, 'GET', `/corrections/${COR.rejectTarget}`);
  expect(cor.status).toBe(200);
  expect((cor.body as { data?: { status?: string } }).data?.status).toBe('REJECTED');
});
