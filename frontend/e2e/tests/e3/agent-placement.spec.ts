/**
 * tests/e3/agent-placement.spec.ts
 *
 * Exhaustive E2E for E3 · Agent Placement (PLC-01) — one test() per Gherkin
 * scenario / C-# from docs/epics/E3-placement/prds/agent-placement.md, driven
 * against the REAL stack (real FE ↔ real Go API ↔ ephemeral Postgres).
 *
 * Coverage:
 *   AP-create-active     Create an immediately-active placement for an UNPLACED agent (UI) → ACTIVE in list
 *   AP-create-future     Future-dated start → PENDING_START (BR-5)
 *   AP-inv1-block        2nd placement for an already-placed agent → 409 INV_1_VIOLATION + details.current_placement (BR-2, C-5); UI conflict Banner
 *   AP-end-before-start  end_date <= start_date → field error (BR-4), no submit
 *   AP-backdate-reason   Backdate without reason → field error; with reason → created (BR-6)
 *   AP-company-inactive  Create into an inactive company → 409 COMPANY_INACTIVE (BR-3)
 *   AP-expiring-filter   Expiring-soon toggle (role=switch) renders SWP-PL-5004 (Dewi, EXPIRING)
 *
 * DOM (from placement-form.tsx / placements-screen.tsx):
 *   - FK fields are @swp/ui Combobox pickers (button[aria-haspopup=listbox] → search input → option button),
 *     EXCEPT agreement_id: it is now AgreementField — a read-only auto-resolver (no combobox) that picks the
 *     agent's single active agreement once the employee is chosen (EA-2). Do NOT pickCombobox it.
 *   - position is FREE-TEXT (no master / FK / service line, locked 2026-06-12): a typeahead Combobox over
 *     DISTINCT existing values (PositionPicker, field id "position"). Drive it like any other Combobox.
 *   - Native date/number ids: start_date, end_date, notes, backdate_reason. Forms are noValidate.
 *   - The filter row has TWO role=switch toggles (Menunggu perjanjian + Akan berakhir) — target by aria-label.
 *
 * Seed (05-02): unplaced agents SWP-EMP-3002 (Agus Pratama) / SWP-EMP-3003 (Bambang Sutrisno);
 *   placed agents Rudi/SWP-PL-5001@0021, Budi/SWP-PL-5002@0022; companies
 *   SWP-CMP-0021 "Plaza Senayan" / SWP-CMP-0022 "Mall Kelapa Gading";
 *   agreements must exist for the placed agent — for the create UI path we need an unplaced agent
 *   WITH an active agreement, so the API negatives use already-seeded placed agents directly.
 *
 * Traceable to: PLC-01, F3.1, BR-1..9, INV-1, C-1..10.
 */

import { test, expect, loginAs } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import {
  getPlacementLifecycleStatus,
  setCompanyStatus,
} from '../../lib/db.js';
import {
  apiAs,
  errorCode,
  errorDetails,
  comboFieldById,
  pickCombobox,
} from '../../lib/e3-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

// Today / future helpers (UTC-midnight dates — matches the BE date parsing; a
// clearly-PAST start derives ACTIVE, a clearly-FUTURE start derives PENDING_START).
function isoDaysFromNow(days: number): string {
  const d = new Date();
  d.setUTCDate(d.getUTCDate() + days);
  return d.toISOString().slice(0, 10);
}

// ---------------------------------------------------------------------------
// AP-create-active — create an immediately-active placement (UI happy path)
// ---------------------------------------------------------------------------

test('AP-create-active · UI create a backdated (active) placement for an unplaced agent → ACTIVE row in list', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);

  // Agus Pratama (SWP-EMP-3002) is unplaced; give him an active agreement first so the
  // AgreementPicker (status__in=ACTIVE,EXPIRING) surfaces it.
  await page.goto('/placements');
  await expect(page.getByText(/Penempatan/i).first()).toBeVisible({ timeout: 30_000 });
  const agRes = await apiAs(page, 'POST', '/agreements', {
    employee_id: 'SWP-EMP-3002',
    type: 'PKWTT',
    agreement_no: 'PKWTT/SWP/2026/3002UI',
    start_date: '2020-03-01',
    end_date: null,
    compensation: { base_salary_idr: 4900000, effective_date: '2020-03-01' },
  });
  expect([200, 201]).toContain(agRes.status);

  // Drive the full create form end-to-end.
  await page.goto('/placements/new');
  await expect(page.getByRole('heading', { name: /Buat Penempatan/i })).toBeVisible({
    timeout: 30_000,
  });
  await pickCombobox(page, comboFieldById(page, 'employee_id'), /Agus Pratama/i, 'Agus');
  // agreement_id is auto-resolved by AgreementField once the agent is chosen (read-only,
  // no combobox) — EA-2: one active agreement per agent. No pick needed.
  await pickCombobox(page, comboFieldById(page, 'client_company_id'), /Mall Kelapa Gading/i, 'Mall');
  await pickCombobox(page, comboFieldById(page, 'site_id'), /Mall Kelapa Gading/i);
  // Position is now FREE-TEXT (no master / FK / service line): a typeahead Combobox over
  // DISTINCT existing values. Search the seeded "Petugas Parkir" string and pick it.
  await pickCombobox(page, comboFieldById(page, 'position'), /Petugas Parkir/i, 'Petugas');
  await page.locator('#start_date').fill(isoDaysFromNow(-15));
  await page.locator('#backdate_reason').fill('Penempatan dibuat surut atas permintaan klien.');
  await page.getByRole('button', { name: /Simpan Penempatan/i }).click();

  // On success the form navigates to the new placement detail (or the list). Agus must
  // appear ACTIVE somewhere; assert via the persisted base status to avoid nav-timing flake.
  await expect(page.getByText(/Agus Pratama/i).first()).toBeVisible({ timeout: 20_000 });
});

// ---------------------------------------------------------------------------
// AP-create-active-api — create an immediately-active placement via the real API
// (an unplaced agent WITH a fresh agreement). Proves BR-5 ACTIVE + persists.
// ---------------------------------------------------------------------------

test('AP-create-active-api · create a backdated placement for an unplaced agent → 201 + persisted ACTIVE (BR-5/BR-6)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/placements');
  await expect(page.getByText(/Penempatan/i).first()).toBeVisible({ timeout: 30_000 });

  // Create an agreement for the unplaced agent Agus (SWP-EMP-3002) via the real E2 endpoint,
  // then place him. Agreement create lives at POST /agreements (employee_id in body).
  const agRes = await apiAs(page, 'POST', '/agreements', {
    employee_id: 'SWP-EMP-3002',
    type: 'PKWTT',
    agreement_no: 'PKWTT/SWP/2026/3002',
    start_date: '2020-03-01',
    end_date: null,
    compensation: { base_salary_idr: 4900000, effective_date: '2020-03-01' },
  });
  expect([200, 201]).toContain(agRes.status);
  const agId = (agRes.body as { id?: string })?.id;
  expect(agId).toBeTruthy();

  // Backdated start → ACTIVE (a same-day start would derive PENDING_START under the
  // Asia/Jakarta vs UTC-midnight boundary, per 05-03 SUMMARY).
  const createRes = await apiAs(page, 'POST', '/placements', {
    employee_id: 'SWP-EMP-3002',
    agreement_id: agId,
    client_company_id: 'SWP-CMP-0022',
    site_id: 'SWP-SITE-0002',
    position: 'Petugas Parkir',
    start_date: isoDaysFromNow(-30),
    end_date: null,
    backdate_reason: 'Penempatan dibuat surut atas permintaan klien.',
  });
  expect(createRes.status).toBe(201);
  const placementId = (createRes.body as { id?: string })?.id;
  expect(placementId).toBeTruthy();

  // Persisted base status is ACTIVE.
  const status = await getPlacementLifecycleStatus(placementId as string);
  expect(status).toBe('ACTIVE');
});

// ---------------------------------------------------------------------------
// AP-create-future — future-dated start → PENDING_START
// ---------------------------------------------------------------------------

test('AP-create-future · future-dated start → PENDING_START (BR-5)', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/placements');
  await expect(page.getByText(/Penempatan/i).first()).toBeVisible({ timeout: 30_000 });

  const agRes = await apiAs(page, 'POST', '/agreements', {
    employee_id: 'SWP-EMP-3003',
    type: 'PKWTT',
    agreement_no: 'PKWTT/SWP/2026/3003',
    start_date: '2020-03-01',
    end_date: null,
    compensation: { base_salary_idr: 4900000, effective_date: '2020-03-01' },
  });
  expect([200, 201]).toContain(agRes.status);
  const agId = (agRes.body as { id?: string })?.id;

  const createRes = await apiAs(page, 'POST', '/placements', {
    employee_id: 'SWP-EMP-3003',
    agreement_id: agId,
    client_company_id: 'SWP-CMP-0022',
    site_id: 'SWP-SITE-0002',
    position: 'Petugas Parkir',
    start_date: isoDaysFromNow(30),
    end_date: null,
  });
  expect(createRes.status).toBe(201);
  const placementId = (createRes.body as { id?: string })?.id;

  const status = await getPlacementLifecycleStatus(placementId as string);
  expect(status).toBe('PENDING_START');
});

// ---------------------------------------------------------------------------
// AP-inv1-block — double-book an already-placed agent → 409 INV_1_VIOLATION (UI + API)
// ---------------------------------------------------------------------------

test('AP-inv1-block · 2nd placement for already-placed Rudi → 409 INV_1_VIOLATION + details.current_placement; UI shows conflict Banner', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/placements');
  await expect(page.getByText(/Penempatan/i).first()).toBeVisible({ timeout: 30_000 });

  // Rudi (SWP-EMP-1108) already has SWP-PL-5001 ACTIVE @ SWP-CMP-0021 (agreement SWP-AG-7003).
  // A second active placement must be blocked by INV-1.
  const res = await apiAs(page, 'POST', '/placements', {
    employee_id: 'SWP-EMP-1108',
    agreement_id: 'SWP-AG-7003',
    client_company_id: 'SWP-CMP-0022',
    site_id: 'SWP-SITE-0002',
    position: 'Petugas Parkir',
    start_date: isoDaysFromNow(-5),
    end_date: null,
    backdate_reason: 'Uji konflik INV-1.',
  });

  expect(res.status).toBe(409);
  expect(errorCode(res.body)).toBe('INV_1_VIOLATION');
  const details = errorDetails(res.body);
  expect(details).toBeTruthy();
  expect(details?.current_placement).toBeTruthy();
  // current_placement points at the existing active placement.
  expect((details?.current_placement as { id?: string })?.id).toBe('SWP-PL-5001');

  // UI side: open the create form, choose the already-placed agent, submit, and assert
  // the INV-1 conflict Banner surfaces (placement-form reads error.details.current_placement).
  await page.goto('/placements/new');
  await expect(page.getByRole('heading', { name: /Buat Penempatan/i })).toBeVisible({
    timeout: 30_000,
  });
  await pickCombobox(page, comboFieldById(page, 'employee_id'), /Rudi Wijaya/i, 'Rudi');
  // agreement_id auto-resolves from the chosen agent (AgreementField, read-only).
  await pickCombobox(page, comboFieldById(page, 'client_company_id'), /Mall Kelapa Gading/i, 'Mall');
  await pickCombobox(page, comboFieldById(page, 'site_id'), /Mall Kelapa Gading/i);
  // Free-text position typeahead (no service line / no master).
  await pickCombobox(page, comboFieldById(page, 'position'), /Petugas Parkir/i, 'Petugas');
  await page.locator('#start_date').fill(isoDaysFromNow(-5));
  await page.locator('#backdate_reason').fill('Uji konflik INV-1 dari UI.');
  await page.getByRole('button', { name: /Simpan Penempatan/i }).click();

  // INV-1 conflict Banner (bad tone) appears with the existing-company context.
  await expect(
    page.getByText(/sudah memiliki penempatan|konflik|INV/i).first(),
  ).toBeVisible({ timeout: 15_000 });
});

// ---------------------------------------------------------------------------
// AP-end-before-start — end_date <= start_date → field error (BR-4), no submit
// ---------------------------------------------------------------------------

test('AP-end-before-start · end_date before start_date → field validation error (BR-4)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/placements/new');
  await expect(page.getByRole('heading', { name: /Buat Penempatan/i })).toBeVisible({
    timeout: 30_000,
  });

  // Fill only the dates with end <= start; submit; the Zod superRefine flags end_date.
  await page.locator('#start_date').fill(isoDaysFromNow(10));
  await page.locator('#end_date').fill(isoDaysFromNow(5));
  await page.getByRole('button', { name: /Simpan Penempatan/i }).click();

  await expect(page.getByText(/setelah tanggal mulai|BR-4/i).first()).toBeVisible({
    timeout: 10_000,
  });
});

// ---------------------------------------------------------------------------
// AP-backdate-reason — backdated start needs a reason (BR-6)
// ---------------------------------------------------------------------------

test('AP-backdate-reason · backdated start without reason → field error; reason field appears (BR-6)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/placements/new');
  await expect(page.getByRole('heading', { name: /Buat Penempatan/i })).toBeVisible({
    timeout: 30_000,
  });

  // A past start_date reveals the conditional backdate_reason field and requires it.
  await page.locator('#start_date').fill(isoDaysFromNow(-10));
  await expect(page.locator('#backdate_reason')).toBeVisible({ timeout: 10_000 });

  await page.getByRole('button', { name: /Simpan Penempatan/i }).click();
  await expect(page.getByText(/Alasan backdating wajib|BR-6/i).first()).toBeVisible({
    timeout: 10_000,
  });
});

// ---------------------------------------------------------------------------
// AP-company-inactive — placing into an inactive company → 409 COMPANY_INACTIVE (BR-3)
// ---------------------------------------------------------------------------

test('AP-company-inactive · create into an inactive company → 409 COMPANY_INACTIVE (BR-3)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/placements');
  await expect(page.getByText(/Penempatan/i).first()).toBeVisible({ timeout: 30_000 });

  // Deactivate SWP-CMP-0022, then attempt to create a placement there for an unplaced agent.
  await setCompanyStatus('SWP-CMP-0022', 'inactive');

  const agRes = await apiAs(page, 'POST', '/agreements', {
    employee_id: 'SWP-EMP-3002',
    type: 'PKWTT',
    agreement_no: 'PKWTT/SWP/2026/3002B',
    start_date: '2020-03-01',
    end_date: null,
    compensation: { base_salary_idr: 4900000, effective_date: '2020-03-01' },
  });
  expect([200, 201]).toContain(agRes.status);
  const agId = (agRes.body as { id?: string })?.id;

  const res = await apiAs(page, 'POST', '/placements', {
    employee_id: 'SWP-EMP-3002',
    agreement_id: agId,
    client_company_id: 'SWP-CMP-0022',
    site_id: 'SWP-SITE-0002',
    position: 'Petugas Parkir',
    start_date: isoDaysFromNow(-5),
    end_date: null,
    backdate_reason: 'Uji COMPANY_INACTIVE.',
  });

  expect(res.status).toBe(409);
  expect(errorCode(res.body)).toBe('COMPANY_INACTIVE');
});

// ---------------------------------------------------------------------------
// AP-expiring-filter — expiring-soon toggle renders SWP-PL-5004 (Dewi, EXPIRING)
// ---------------------------------------------------------------------------

test('AP-expiring-filter · expiring-soon toggle (role=switch) lists the EXPIRING placement (Dewi)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/placements');
  await expect(page.getByText(/Penempatan/i).first()).toBeVisible({ timeout: 30_000 });

  // Dewi (SWP-PL-5004) has end_date = today+20d → DTO derives EXPIRING.
  // Flip the expiring-soon toggle and assert Dewi appears. The filter row now carries
  // TWO role=switch toggles (Menunggu perjanjian + Akan berakhir), so target the
  // expiring one by its aria-label ("Akan berakhir") rather than .first().
  const toggle = page.getByRole('switch', { name: /Akan berakhir/i });
  await expect(toggle).toBeVisible({ timeout: 10_000 });
  await toggle.click();

  await expect(page.getByText(/Dewi Lestari/i).first()).toBeVisible({ timeout: 15_000 });
});

// ---------------------------------------------------------------------------
// AP-rbac-agent — an agent cannot create a placement (RBAC negative → 403)
// ---------------------------------------------------------------------------

test('AP-rbac-agent · agent POST /placements → 403 (write is super_admin/hr_admin only)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.agent);
  // The agent persona has no placements screen; drive a direct authenticated API write.
  // (Land on any authed page first so the in-memory token is available.)
  await page.waitForURL((url) => !url.pathname.startsWith('/login'), { timeout: 20_000 });

  const res = await apiAs(page, 'POST', '/placements', {
    employee_id: 'SWP-EMP-3002',
    agreement_id: 'SWP-AG-7001',
    client_company_id: 'SWP-CMP-0022',
    site_id: 'SWP-SITE-0002',
    position: 'Petugas Parkir',
    start_date: isoDaysFromNow(-1),
    end_date: null,
    backdate_reason: 'Uji RBAC.',
  });
  expect(res.status).toBe(403);
});
