/**
 * tests/journey/onboarding-to-clock.spec.ts
 *
 * Cross-epic ONBOARDING JOURNEY (E2 → E3 → E4 → E5) against the REAL stack
 * (real Vite FE :4173, MSW off → real Go API :8081 → ephemeral Postgres :5433).
 *
 * Unlike the per-epic suites (which exhaustively cover one feature each), this
 * spec proves the whole chain links together on FRESH data, end to end:
 *
 *   1. HR admin creates a client company → an auto primary site is provisioned   (E2 · CC)
 *   2. HR admin onboards a shift-leader + an agent: employee (auto-login) →
 *      agreement → active placement at the new company → shift-leader assignment  (E2/E3)
 *   3. The shift leader logs in and publishes TODAY's shift for the agent
 *      via the weekly schedule grid                                               (E4 · SA)
 *   4. The agent logs in and clocks IN then OUT from the Kehadiran home           (E5 · clock)
 *
 * Step 1, 3 and 4 are driven through the REAL browser UI (the headline flows the
 * journey is meant to exercise). Step 2 drives the onboarding PLUMBING through the
 * real API (apiAs): employee create surfaces the one-time `temp_password` only in
 * its JSON response (never in the UI), and agreement/placement/SLA wiring is
 * already UI-covered by the e2/e3 suites — here we just need it persisted so the
 * downstream UI steps have real data to act on.
 *
 * Auth note: every employee auto-provisions a login on create (backend D1) with a
 * one-time temp password + must_change_password=true. The web login flow does NOT
 * force a change screen, so logging in with the temp password navigates straight
 * into the app. The shift_leader auth role is DERIVED per-request from the active
 * shift-leader-assignment (not stored on the user), so the freshly onboarded leader
 * can reach /schedule with no re-provisioning.
 *
 * Geolocation: clock-in calls navigator.geolocation; we grant the permission and
 * pin coordinates at the context level. The auto-created primary site has no
 * geofence, so any coordinates clear the geofence check.
 *
 * Isolation: resetDb() ONCE in beforeAll (serial journey — state must persist
 * across the four ordered steps; do NOT reset between them).
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import {
  countSitesForCompany,
  getActiveLeaderEmployeeForCompany,
  getCompanyByName,
  getEmployeeUserId,
  getLatestAttendanceForEmployee,
  getPlacementIdForEmployeeAtCompany,
  getPlacementLifecycleStatus,
  getPrimarySiteIdForCompany,
} from '../../lib/db.js';
import { apiAs, isoDaysFromNow, waitForToken } from '../../lib/e4-helpers.js';
import {
  assignShiftViaPopover,
  cellButton,
  expectPublishedToast,
  openCell,
} from '../../lib/e4-helpers.js';

// Wide viewport so the schedule grid + tables render every column.
test.use({ viewport: { width: 1600, height: 1000 } });

// Grant geolocation + pin coords (Plaza Senayan-ish; the auto-site has no geofence
// so the exact point only needs to exist for navigator.geolocation to resolve).
test.use({
  permissions: ['geolocation'],
  geolocation: { latitude: -6.2256, longitude: 106.7997 },
});

// One database lifecycle for the whole ordered journey.
test.beforeAll(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// Fixed (deterministic — resetDb wipes per run) journey constants.
// ---------------------------------------------------------------------------

const COMPANY_NAME = 'PT Journey Onboarding E2E';
const COMPANY_ADDRESS = 'Jl. Journey No. 1, Jakarta';

// Position is FREE-TEXT (no master / FK / service line, locked 2026-06-12) — the
// placement just carries the position string, reused as the e3 agent-placement suite does.
const POSITION = 'Petugas Parkir';

const SL = {
  fullName: 'Joko Pemimpin E2E',
  nik: '3201234567890001',
  phone: '+62-811-2000-0001',
  loginEmail: 'sl.journey@swp.test',
  password: 'Journey-SL-2026!', // temp password is captured from the create response
  agreementNo: 'PKWTT/SWP/2026/J-SL',
};

const AGENT = {
  fullName: 'Agus Agen E2E',
  nik: '3201234567890002',
  phone: '+62-811-2000-0002',
  loginEmail: 'agent.journey@swp.test',
  password: 'Journey-AG-2026!',
  agreementNo: 'PKWTT/SWP/2026/J-AG',
};

// Carried across the ordered steps (workers=1, single file → module state is safe).
const J: {
  companyId?: string;
  siteId?: string;
  slEmployeeId?: string;
  agentEmployeeId?: string;
  slTempPassword?: string;
  agentTempPassword?: string;
} = {};

// ---------------------------------------------------------------------------
// Small API helpers (login as hr_admin, hydrate token, then drive the real API).
// ---------------------------------------------------------------------------

/** Create an employee via the real API; returns { id, temp_password }. */
async function createEmployee(
  page: import('@playwright/test').Page,
  e: { fullName: string; nik: string; phone: string; loginEmail: string },
): Promise<{ id: string; tempPassword: string }> {
  const res = await apiAs(page, 'POST', '/employees', {
    full_name: e.fullName,
    nik: e.nik,
    join_at: isoDaysFromNow(-30),
    phone: e.phone,
    login_email: e.loginEmail,
  });
  expect(res.status, JSON.stringify(res.body)).toBe(201);
  const body = res.body as { id?: string; temp_password?: string };
  expect(body.id).toBeTruthy();
  expect(body.temp_password, 'login auto-provisioned → temp_password returned once').toBeTruthy();
  return { id: body.id as string, tempPassword: body.temp_password as string };
}

/** Give an employee an active (PKWTT) agreement; returns the agreement id. */
async function createAgreement(
  page: import('@playwright/test').Page,
  employeeId: string,
  agreementNo: string,
): Promise<string> {
  const res = await apiAs(page, 'POST', '/agreements', {
    employee_id: employeeId,
    type: 'PKWTT',
    agreement_no: agreementNo,
    start_date: '2020-03-01',
    end_date: null,
    compensation: { base_salary_idr: 4900000, effective_date: '2020-03-01' },
  });
  expect([200, 201], JSON.stringify(res.body)).toContain(res.status);
  const id = (res.body as { id?: string })?.id;
  expect(id).toBeTruthy();
  return id as string;
}

/** Place an employee at the journey company (backdated → ACTIVE). */
async function createPlacement(
  page: import('@playwright/test').Page,
  employeeId: string,
  agreementId: string,
): Promise<void> {
  const res = await apiAs(page, 'POST', '/placements', {
    employee_id: employeeId,
    agreement_id: agreementId,
    client_company_id: J.companyId,
    site_id: J.siteId,
    position: POSITION,
    start_date: isoDaysFromNow(-5),
    end_date: null,
    backdate_reason: 'Penempatan awal onboarding (uji E2E).',
  });
  expect(res.status, JSON.stringify(res.body)).toBe(201);
}

// Serial: the four steps are ONE ordered journey sharing module state (J). Serial
// mode keeps them in the same worker (state persists) and skips the rest on the
// first failure instead of running later steps against half-built state.
test.describe.serial('onboarding journey (E2 → E3 → E4 → E5)', () => {
// ===========================================================================
// STEP 1 — HR admin creates a client company → auto primary site (E2 · CC)
// ===========================================================================

test('journey-1 · HR admin creates the client company and an auto primary site is provisioned', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);

  // Create via the real UI (the headline flow).
  await page.goto('/client-companies/new');
  await page.locator('#cc-name').fill(COMPANY_NAME);
  await page.locator('#cc-address').fill(COMPANY_ADDRESS);
  await page.getByRole('button', { name: 'Simpan' }).click();

  await expect(page.getByText('Perusahaan berhasil ditambahkan')).toBeVisible({ timeout: 30_000 });
  await expect(page.getByText(COMPANY_NAME).first()).toBeVisible({ timeout: 10_000 });

  // DB: company exists + exactly one auto-created primary site.
  const companyId = await getCompanyByName(COMPANY_NAME);
  expect(companyId).not.toBeNull();
  expect(await countSitesForCompany(companyId!)).toBe(1);
  const siteId = await getPrimarySiteIdForCompany(companyId!);
  expect(siteId).not.toBeNull();

  J.companyId = companyId!;
  J.siteId = siteId!;
});

// ===========================================================================
// STEP 2 — Onboard the shift-leader + agent and wire them to the company
//          (employee+login → agreement → active placement → SLA) (E2/E3)
// ===========================================================================

test('journey-2 · onboard a shift-leader and an agent, place both at the company, assign the leader', async ({
  page,
}) => {
  expect(J.companyId, 'step 1 must have created the company').toBeTruthy();

  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/');
  await waitForToken(page); // apiAs needs the in-memory bearer token

  // --- employees (auto-provision a login; capture the one-time temp password) ---
  const sl = await createEmployee(page, SL);
  const agent = await createEmployee(page, AGENT);
  J.slEmployeeId = sl.id;
  J.agentEmployeeId = agent.id;
  J.slTempPassword = sl.tempPassword;
  J.agentTempPassword = agent.tempPassword;

  // Both employees got a linked login user (D1 auto-provision).
  expect(await getEmployeeUserId(sl.id)).not.toBeNull();
  expect(await getEmployeeUserId(agent.id)).not.toBeNull();

  // --- agreements ---
  const slAgreement = await createAgreement(page, sl.id, SL.agreementNo);
  const agentAgreement = await createAgreement(page, agent.id, AGENT.agreementNo);

  // --- placements at the journey company (ACTIVE) ---
  await createPlacement(page, sl.id, slAgreement);
  await createPlacement(page, agent.id, agentAgreement);

  const slPlacementId = await getPlacementIdForEmployeeAtCompany(sl.id, J.companyId!);
  const agentPlacementId = await getPlacementIdForEmployeeAtCompany(agent.id, J.companyId!);
  expect(slPlacementId).not.toBeNull();
  expect(agentPlacementId).not.toBeNull();
  expect(await getPlacementLifecycleStatus(slPlacementId!)).toBe('ACTIVE');
  expect(await getPlacementLifecycleStatus(agentPlacementId!)).toBe('ACTIVE');

  // --- shift-leader assignment (company is vacant → first leader, no replace) ---
  const sla = await apiAs(page, 'POST', '/shift-leader-assignments', {
    client_company_id: J.companyId,
    employee_id: sl.id,
    start_date: isoDaysFromNow(0),
    replace: false,
  });
  expect(sla.status, JSON.stringify(sla.body)).toBe(201);
  expect(await getActiveLeaderEmployeeForCompany(J.companyId!)).toBe(sl.id);
});

// ===========================================================================
// STEP 3 — The shift leader logs in and publishes TODAY's shift for the agent
//          (E4 · schedule grid, auto-publish)
// ===========================================================================

test('journey-3 · the shift leader publishes today\'s shift for the agent', async ({ page }) => {
  expect(J.slTempPassword, 'step 2 must have onboarded the leader').toBeTruthy();

  // Log in as the freshly onboarded leader (temp password). shift_leader auth is
  // derived per-request from the active SLA created in step 2.
  await loginAs(page, { email: SL.loginEmail, password: J.slTempPassword!, role: 'shift_leader' });

  await page.goto('/schedule');

  // A shift leader is scoped to exactly one company, so the grid auto-selects it
  // (no company picker is rendered). The grid is roster-driven: the agent (active
  // placement, no entries yet) appears as a row.
  await expect(page.getByText(AGENT.fullName).first()).toBeVisible({ timeout: 30_000 });

  // Click the agent's TODAY cell and assign the all-lines "Pagi" shift → auto-publish.
  // cellButton() now formats the ISO date to the grid's "13 Jun" aria-label form itself,
  // so pass the Asia/Jakarta ISO date directly.
  const today = isoDaysFromNow(0);
  await openCell(page, AGENT.fullName, today);
  await assignShiftViaPopover(page, 'Pagi');
  await expectPublishedToast(page);

  // The cell now carries the Pagi chip.
  await expect(cellButton(page, AGENT.fullName, today).getByText('Pagi')).toBeVisible({
    timeout: 15_000,
  });
});

// ===========================================================================
// STEP 4 — The agent logs in and clocks IN then OUT (E5 · attendance)
// ===========================================================================

test('journey-4 · the agent clocks in and out from the Kehadiran home', async ({ page }) => {
  expect(J.agentTempPassword, 'step 2 must have onboarded the agent').toBeTruthy();
  expect(J.agentEmployeeId).toBeTruthy();

  await loginAs(page, { email: AGENT.loginEmail, password: J.agentTempPassword!, role: 'agent' });

  await page.goto('/me');

  // With a published shift for today, the clock CTA is enabled.
  const absen = page.getByRole('button', { name: 'Absen Sekarang' });
  await expect(absen).toBeEnabled({ timeout: 30_000 });

  // --- clock IN ---
  await absen.click();
  let dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible({ timeout: 10_000 });
  await dialog.getByRole('button', { name: 'Absen Masuk' }).click();
  await expect(page.getByText('Berhasil absen masuk')).toBeVisible({ timeout: 15_000 });

  // DB: an open attendance record now exists (check_in_at set, check_out_at still null).
  await expect
    .poll(async () => (await getLatestAttendanceForEmployee(J.agentEmployeeId!))?.check_in_at ?? null, {
      timeout: 15_000,
    })
    .not.toBeNull();
  expect((await getLatestAttendanceForEmployee(J.agentEmployeeId!))?.check_out_at ?? null).toBeNull();

  // --- clock OUT ---
  await absen.click();
  dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible({ timeout: 10_000 });
  await dialog.getByRole('button', { name: 'Absen Keluar' }).click();
  await expect(page.getByText('Berhasil absen keluar')).toBeVisible({ timeout: 15_000 });

  // DB: the record is now closed (check_out_at + worked_minutes set).
  await expect
    .poll(
      async () => (await getLatestAttendanceForEmployee(J.agentEmployeeId!))?.check_out_at ?? null,
      { timeout: 15_000 },
    )
    .not.toBeNull();
  const final = await getLatestAttendanceForEmployee(J.agentEmployeeId!);
  expect(final?.worked_minutes ?? null).not.toBeNull();
});
});
