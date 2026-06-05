/**
 * tests/e10/dashboard.spec.ts  ·  @e10-dashboard
 *
 * E10 · Role-aware dashboard (GET /dashboards/me) — DB-1..DB-6.
 *
 * Proves the dashboard-screen.tsx role branching against the REAL Go BE (MSW off):
 *   - HR / super_admin → HrDashboardView: TitleBand <h1>"Dashboard"</h1> + the server's
 *     `role_label` ("HR Admin" / "Super Admin"), 4× KPI StatCards, and the ApprovalInboxPanel
 *     ("Perlu Tindakan") whose rows deep-link into E5/E6/E7/E3. The deep-link PATHS are
 *     asserted on the real /dashboards/me JSON (panel[].deep_link.path) — the loop-closer
 *     from a pending action to its queue.
 *   - shift_leader → LeaderDashboardView: same <h1> + a "{company.name} · …" subtitle, the
 *     leader `role_label` chip, today's clock-in/late/absent StatCards, and counts SCOPED to
 *     the leader's own company (the payload.role is shift_leader and carries a `company`, NOT
 *     the HR global `kpis` shape).
 *
 * Selectors anchored on the REAL components (dashboard-screen.tsx + approval-inbox-panel.tsx);
 * the role-scope assertion is driven through the real /dashboards/me payload via apiAs.
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import { apiAs, gotoReady } from '../../lib/e10-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

interface DashboardPayload {
  role: string;
  role_label: string;
  company?: { id?: string; name?: string };
  kpis?: { active_placements?: number; active_companies?: number };
  pending_approvals_panel?: Array<{
    kind: string;
    label: string;
    count: number;
    deep_link: { epic?: string; path?: string };
  }>;
}

async function getDashboard(page: import('@playwright/test').Page): Promise<DashboardPayload> {
  const res = await apiAs(page, 'GET', '/dashboards/me');
  expect(res.status, `GET /dashboards/me → ${res.status}: ${JSON.stringify(res.body)}`).toBe(200);
  return (res.body as { data: DashboardPayload }).data;
}

// ---------------------------------------------------------------------------
// HR — global KPIs + role_label + ApprovalInboxPanel with real deep-link paths
// ---------------------------------------------------------------------------

test('DASH-hr-global · HR sees the global HrDashboard (role_label, KPI cards, inbox deep-links)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await gotoReady(page, '/');

  // UI: the dashboard heading + the server-provided role label render.
  await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible({ timeout: 30_000 });
  await expect(page.getByText('HR Admin', { exact: false }).first()).toBeVisible();

  // UI: KPI StatCards (HR-only) render — "Penempatan Aktif" is always present in HrDashboardView.
  await expect(page.getByText('Penempatan Aktif', { exact: false }).first()).toBeVisible();

  // UI: the ApprovalInboxPanel ("Perlu Tindakan") renders on the HR dashboard.
  await expect(page.getByText('Perlu Tindakan', { exact: false }).first()).toBeVisible();

  // CONTRACT: the real payload is the HR global shape with KPIs + deep-linked panel rows.
  const d = await getDashboard(page);
  expect(d.role).toBe('hr_admin');
  expect(d.role_label).toBe('HR Admin');
  expect(d.kpis).toBeTruthy();
  expect(d.company).toBeUndefined(); // HR is NOT company-scoped (no company on the HR shape)

  // The seed plants PENDING leave (SWP-LR-8001..8004/8007), so the leave-approve panel row
  // must be present with its deep link into the E6 queue — the loop-closer from action → queue.
  const panel = d.pending_approvals_panel ?? [];
  const leaveRow = panel.find((r) => r.kind === 'LEAVE_APPROVE');
  expect(leaveRow, `panel kinds: ${panel.map((r) => r.kind).join(',')}`).toBeTruthy();
  expect(leaveRow?.count).toBeGreaterThan(0);
  expect(leaveRow?.deep_link.path).toContain('/leave-requests');
});

// ---------------------------------------------------------------------------
// super_admin — same shape as HR + the "Super Admin" label (D1)
// ---------------------------------------------------------------------------

test('DASH-super-label · super_admin gets the HR shape with role_label "Super Admin"', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.superAdmin);
  await gotoReady(page, '/');

  await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible({ timeout: 30_000 });

  const d = await getDashboard(page);
  expect(d.role).toBe('super_admin');
  expect(d.role_label).toBe('Super Admin');
  expect(d.kpis).toBeTruthy();
});

// ---------------------------------------------------------------------------
// shift_leader — LeaderDashboard scoped to OWN company (not global HR KPIs)
// ---------------------------------------------------------------------------

test('DASH-leader-scoped · shift_leader sees own-company LeaderDashboard (company subtitle, today cards), NOT global KPIs', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await gotoReady(page, '/');

  // UI: same heading; LeaderDashboardView renders the company name in the subtitle + the
  // leader role_label chip + today's "Sudah Absen" / "Terlambat" StatCards.
  await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible({ timeout: 30_000 });
  await expect(page.getByText('Plaza Senayan', { exact: false }).first()).toBeVisible();
  await expect(page.getByText('Sudah Absen', { exact: false }).first()).toBeVisible();
  await expect(page.getByText('Terlambat', { exact: false }).first()).toBeVisible();

  // CONTRACT: the payload is the leader shape, company-scoped (NOT the HR global kpis shape).
  const d = await getDashboard(page);
  expect(d.role).toBe('shift_leader');
  expect(d.role_label).toBeTruthy();
  expect(d.company?.id).toBe('SWP-CMP-0021'); // scoped to the leader's own company
  expect(d.kpis).toBeUndefined(); // leader does NOT receive the global HR KPI block

  // Any leader panel deep links are company-scoped (carry the company_id query) when present.
  for (const row of d.pending_approvals_panel ?? []) {
    expect(row.deep_link.path).toContain('SWP-CMP-0021');
  }
});
