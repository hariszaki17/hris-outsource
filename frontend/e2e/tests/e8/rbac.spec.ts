/**
 * tests/e8/rbac.spec.ts
 *
 * E8 · Payroll archive RBAC (PAY-01 / INV-4) against the REAL stack. The web payroll archive
 * is hr_admin/super_admin ONLY — agent + shift_leader get the no-permission UI AND a real BE
 * 403 on every payroll endpoint (the BE route-level RequireRole is the real gate; the FE client
 * gate is defense-in-depth). An HR 200 positive control proves the endpoint is reachable.
 *
 * Selectors anchored on payslip-archive-screen.tsx: agent/shift_leader → EmptyState
 * variant="no-permission" (t('common.noPermission')). The real 403 is asserted via apiAs.
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import { PS, apiAs, errorCode, waitForToken } from '../../lib/e8-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// RBAC-agent — agent gets the no-permission UI + a real 403 on /payslips
// ---------------------------------------------------------------------------

test('RBAC-agent · agent /payroll → no-permission UI AND a real BE 403 on GET /payslips', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.agent);
  await page.goto('/payroll');
  await waitForToken(page);

  // Client gate: the no-permission EmptyState renders (no archive table).
  await expect(page.getByText('Akses ditolak').first()).toBeVisible({ timeout: 20_000 });
  await expect(page.locator('div.border-b')).toHaveCount(0);

  // The REAL gate: the BE 403s the list for an agent token.
  const res = await apiAs(page, 'GET', '/payslips');
  expect(res.status).toBe(403);
  expect(errorCode(res.body)).toBe('FORBIDDEN');
});

// ---------------------------------------------------------------------------
// RBAC-leader — shift_leader gets the no-permission UI + real 403s across the surface
// ---------------------------------------------------------------------------

test('RBAC-leader · shift_leader /payroll → no-permission UI AND real 403 on list/export/audit-notes', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/payroll');
  await waitForToken(page);

  await expect(page.getByText('Akses ditolak').first()).toBeVisible({ timeout: 20_000 });

  // Real BE 403 on the list…
  const list = await apiAs(page, 'GET', '/payslips');
  expect(list.status).toBe(403);
  expect(errorCode(list.body)).toBe('FORBIDDEN');

  // …on the async export…
  const exp = await apiAs(page, 'POST', '/payslips:export', { period: '2025-12', format: 'XLSX' });
  expect(exp.status).toBe(403);
  expect(errorCode(exp.body)).toBe('FORBIDDEN');

  // …and on the audit-notes read.
  const notes = await apiAs(page, 'GET', `/payslips/${PS.final}/audit-notes`);
  expect(notes.status).toBe(403);
  expect(errorCode(notes.body)).toBe('FORBIDDEN');
});

// ---------------------------------------------------------------------------
// RBAC-hr-allowed — positive control: HR can list payslips (200)
// ---------------------------------------------------------------------------

test('RBAC-hr-allowed · HR GET /payslips → 200 (positive control)', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/payroll');
  await waitForToken(page);

  const res = await apiAs(page, 'GET', '/payslips?year=2025');
  expect(res.status).toBe(200);
  const body = res.body as { data?: unknown[] };
  expect(Array.isArray(body.data)).toBe(true);
  expect((body.data ?? []).length).toBeGreaterThan(0);
});
