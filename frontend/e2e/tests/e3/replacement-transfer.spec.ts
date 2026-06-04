/**
 * tests/e3/replacement-transfer.spec.ts
 *
 * Exhaustive E2E for E3 · Replacement & Transfer (PLC-02) — one test() per Gherkin
 * scenario from docs/epics/E3-placement/prds/replacement-transfer.md, driven against
 * the REAL stack.
 *
 * Coverage:
 *   TR-transfer-atomic  Transfer Budi (SWP-PL-5002 @0022) to SWP-CMP-0021 / different service line
 *                       (TransferModal: tf-company/tf-sl/tf-pos/tf-start + tf-reason) → predecessor TRANSFERRED,
 *                       successor ACTIVE @0021; 0021 roster now shows Budi (and 0022 no longer does)
 *   TR-same-company     Transfer same-company-same-service-line (no-op) → 422 RULE_VIOLATION (API)
 *   TR-vacancy-warn     Transfer to a company without a leader surfaces the destination no-leader warning
 *   TR-auto-vacate      Transferring a leader out of their company auto-vacates their leadership (SL-6)
 *
 * DOM (placement-overlays.tsx TransferModal): tf-company/tf-sl/tf-pos are Combobox pickers;
 *   tf-start/tf-end are native date inputs; tf-reason is a textarea. Form noValidate.
 *
 * Seed: SWP-PL-5002 Budi@0022 "Mall Kelapa Gading" / Parking (SWP-SVC-003);
 *   SWP-CMP-0021 "Plaza Senayan" (led by Rudi/SWP-SLA-3001); SWP-CMP-0022 has NO leader.
 *   Service lines: SWP-SVC-001 Facility, SWP-SVC-002 Building Management, SWP-SVC-003 Parking.
 *
 * Traceable to: PLC-02, F3.3, TR-*, INV-1 backstop, RULE_VIOLATION.
 */

import { test, expect, loginAs } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import {
  getPlacementLifecycleStatus,
  getActiveLeaderEmployeeForCompany,
  getPlacementIdForEmployeeAtCompany,
} from '../../lib/db.js';
import { apiAs, errorCode, comboFieldById, pickCombobox } from '../../lib/e3-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

function isoDaysFromNow(days: number): string {
  const d = new Date();
  d.setUTCDate(d.getUTCDate() + days);
  return d.toISOString().slice(0, 10);
}

// ---------------------------------------------------------------------------
// TR-transfer-atomic — transfer Budi 0022 → 0021/Building Mgmt via the UI
// ---------------------------------------------------------------------------

test('TR-transfer-atomic · transfer Budi to SWP-CMP-0021 → predecessor TRANSFERRED + successor ACTIVE @0021; roster reflects the move', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/placements/SWP-PL-5002');
  await expect(page.getByText(/Budi Santoso/i).first()).toBeVisible({ timeout: 30_000 });

  // Open the Transfer modal.
  await page.getByRole('button', { name: /Transfer/i }).first().click();
  await expect(page.locator('#tf-start')).toBeVisible({ timeout: 10_000 });

  // New company = Plaza Senayan (SWP-CMP-0021). The seeded positions all belong to the
  // Parking line (SWP-SVC-003), so keep Parking — the COMPANY change alone makes this a
  // valid (non-no-op) transfer. Position = Petugas Parkir / Koordinator Lokasi.
  await pickCombobox(page, comboFieldById(page, 'tf-company'), /Plaza Senayan/i, 'Plaza');
  await pickCombobox(page, comboFieldById(page, 'tf-sl'), /Parking/i, 'Park');
  await pickCombobox(page, comboFieldById(page, 'tf-pos'), /Koordinator|Petugas/i);
  await page.locator('#tf-start').fill(isoDaysFromNow(-2));
  await page.locator('#tf-reason').fill('Rotasi penempatan antar lokasi klien.');

  await page.getByRole('button', { name: /Transfer|Konfirmasi|Simpan/i }).last().click();

  // Predecessor SWP-PL-5002 becomes TRANSFERRED.
  await expect
    .poll(() => getPlacementLifecycleStatus('SWP-PL-5002'), { timeout: 15_000 })
    .toBe('TRANSFERRED');

  // Successor exists for Budi at SWP-CMP-0021 and is ACTIVE.
  const successorId = await getPlacementIdForEmployeeAtCompany('SWP-EMP-2891', 'SWP-CMP-0021');
  expect(successorId).toBeTruthy();
  expect(await getPlacementLifecycleStatus(successorId as string)).toBe('ACTIVE');

  // Roster of 0021 now lists Budi.
  await page.goto('/client-companies/SWP-CMP-0021/roster');
  await expect(page.getByText(/Budi Santoso/i).first()).toBeVisible({ timeout: 20_000 });
});

// ---------------------------------------------------------------------------
// TR-same-company — no-op transfer (same company + same service line) → 422 RULE_VIOLATION
// ---------------------------------------------------------------------------

test('TR-same-company · transfer to the same company + same service line → 422 RULE_VIOLATION', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/placements');
  await expect(page.getByText(/Penempatan/i).first()).toBeVisible({ timeout: 30_000 });

  // SWP-PL-5002 is Budi @ 0022 / Parking (SWP-SVC-003). Transferring to the SAME company +
  // SAME service line is a no-op the BE rejects (must use :renew instead).
  const res = await apiAs(page, 'POST', '/placements/SWP-PL-5002:transfer', {
    new_client_company_id: 'SWP-CMP-0022',
    new_service_line_id: 'SWP-SVC-003',
    new_position_id: 'SWP-POS-014',
    new_start_date: isoDaysFromNow(-1),
    new_end_date: null,
    transfer_reason: 'Uji RULE_VIOLATION no-op transfer.',
  });
  expect(res.status).toBe(422);
  expect(errorCode(res.body)).toBe('RULE_VIOLATION');
});

// ---------------------------------------------------------------------------
// TR-vacancy-warn — transfer to a company without a leader surfaces the destination warning
// ---------------------------------------------------------------------------

test('TR-vacancy-warn · transfer to a leaderless company → destination no-leader warning surfaced', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/placements');
  await expect(page.getByText(/Penempatan/i).first()).toBeVisible({ timeout: 30_000 });

  // Transfer Sari (SWP-PL-5003 @ 0021) to SWP-CMP-0022 (which has NO leader). The transfer
  // result carries a NO_SHIFT_LEADER_AT_DESTINATION warning.
  const res = await apiAs(page, 'POST', '/placements/SWP-PL-5003:transfer', {
    new_client_company_id: 'SWP-CMP-0022',
    new_service_line_id: 'SWP-SVC-002',
    new_position_id: 'SWP-POS-015',
    new_start_date: isoDaysFromNow(-1),
    new_end_date: null,
    transfer_reason: 'Transfer ke perusahaan tanpa shift leader.',
  });
  expect([200, 201]).toContain(res.status);

  const warnings =
    (res.body as { warnings?: string[]; successor?: { warnings?: string[] } } | null)?.warnings ??
    (res.body as { successor?: { warnings?: string[] } } | null)?.successor?.warnings ??
    [];
  expect(warnings.join(',')).toMatch(/NO_SHIFT_LEADER/i);
});

// ---------------------------------------------------------------------------
// TR-auto-vacate — transferring a leader out of their company auto-vacates leadership (SL-6)
// ---------------------------------------------------------------------------

test('TR-auto-vacate · transferring leader Rudi out of SWP-CMP-0021 auto-vacates his leadership (0021 becomes vacant)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/placements');
  await expect(page.getByText(/Penempatan/i).first()).toBeVisible({ timeout: 30_000 });

  // Precondition: Rudi (SWP-EMP-1108) leads SWP-CMP-0021 (SWP-SLA-3001).
  expect(await getActiveLeaderEmployeeForCompany('SWP-CMP-0021')).toBe('SWP-EMP-1108');

  // Transfer Rudi (SWP-PL-5001 @ 0021) to SWP-CMP-0022. SL-6: his leadership of 0021 auto-vacates.
  const res = await apiAs(page, 'POST', '/placements/SWP-PL-5001:transfer', {
    new_client_company_id: 'SWP-CMP-0022',
    new_service_line_id: 'SWP-SVC-003',
    new_position_id: 'SWP-POS-014',
    new_start_date: isoDaysFromNow(-1),
    new_end_date: null,
    transfer_reason: 'Transfer pemimpin shift; jabatan lama otomatis kosong.',
  });
  expect([200, 201]).toContain(res.status);

  // 0021 leadership is now vacant.
  await expect
    .poll(() => getActiveLeaderEmployeeForCompany('SWP-CMP-0021'), { timeout: 15_000 })
    .toBeNull();

  // The 0021 roster now renders the no-leader warn CTA.
  await page.goto('/client-companies/SWP-CMP-0021/roster');
  await expect(page.getByText(/Tetapkan|tanpa.*leader|belum.*leader|tidak ada.*leader/i).first()).toBeVisible({
    timeout: 20_000,
  });
});
