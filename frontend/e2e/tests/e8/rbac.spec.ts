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

import { PS, PS_EMP, apiAs, errorCode, waitForToken } from '../../lib/e8-helpers.js';
import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// RBAC-agent — agent gets the no-permission UI + a real 403 on /payslips
// ---------------------------------------------------------------------------

test('RBAC-agent · agent /payroll → no-permission UI; BE archive is self-scoped (no global access)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.agent);
  await page.goto('/payroll');
  await waitForToken(page);

  // Client gate: an agent lacks `payroll.read`, so the router's capability guard
  // (authedRoute.beforeLoad → routeRequirement('/payroll')='payroll.read') redirects to
  // the global /forbidden NoPermissionScreen BEFORE the HR payroll archive screen mounts.
  // That screen shows the canonical no-permission copy and renders no archive table.
  await expect(page).toHaveURL(/\/forbidden$/, { timeout: 20_000 });
  await expect(page.getByText('Anda tidak memiliki izin untuk tindakan ini.').first()).toBeVisible({
    timeout: 20_000,
  });
  await expect(page.locator('div.border-b')).toHaveCount(0);

  // The REAL gate (server-side, PAY-01 scope:self): the agent has NO access to the global
  // HR archive. GET /payslips is force-scoped to the caller's own employee_id (200 with
  // ONLY their own rows), and the HR-only surfaces (export / another employee's slips)
  // are denied:
  //   - an explicit OTHER employee_id → 403 OUT_OF_SCOPE (no existence leak). The agent
  //     persona (agent.budi@swp.test) is SWP-EMP-2891; PS_EMP.budi (SWP-EMP-1042) is a
  //     different employee, so the service rejects the cross-employee read.
  const other = await apiAs(page, 'GET', `/payslips?employee_id=${PS_EMP.budi}`);
  expect(other.status).toBe(403);
  expect(errorCode(other.body)).toBe('OUT_OF_SCOPE');

  //   - the async export is HR/Super-Admin only → 403 FORBIDDEN
  const exp = await apiAs(page, 'POST', '/payslips:export', { period: '2025-12', format: 'XLSX' });
  expect(exp.status).toBe(403);
  expect(errorCode(exp.body)).toBe('FORBIDDEN');
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

  // A shift_leader also lacks `payroll.read` → the router redirects to /forbidden (the
  // global NoPermissionScreen) before the payroll screen mounts. Same canonical copy.
  await expect(page).toHaveURL(/\/forbidden$/, { timeout: 20_000 });
  await expect(page.getByText('Anda tidak memiliki izin untuk tindakan ini.').first()).toBeVisible({
    timeout: 20_000,
  });

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
