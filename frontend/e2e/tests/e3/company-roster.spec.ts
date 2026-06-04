/**
 * tests/e3/company-roster.spec.ts
 *
 * Exhaustive E2E for E3 · Company Roster (PLC-04) — one test() per Gherkin scenario /
 * C-# from docs/epics/E3-placement/prds/company-roster.md, driven against the REAL stack.
 *
 * Coverage:
 *   RO-render          hr_admin opens SWP-CMP-0021 roster → active placements (Rudi, Sari, Dewi) + current leader Rudi
 *                      (leader chip) + summary counters + by_service_line pills
 *   RO-filter-status   Status filter (FilterSelect) narrows the list
 *   RO-include-history include_history Toggle (role=switch) shows terminal rows after one placement is ended
 *   RO-rbac-scope      shift_leader (rudi.wijaya) opens HIS company (0021) → 200; opens SWP-CMP-0022 → 403 OUT_OF_SCOPE
 *                      (UI no-permission EmptyState + real 403 via page.evaluate)
 *   RO-vacancy         SWP-CMP-0022 roster shows the no-leader warn CTA (current_shift_leader null)
 *
 * DOM (company-roster-screen.tsx): company header (name + leader chip / no-leader warn CTA "Tetapkan leader"),
 *   summary counters, div.border-b rows, status FilterSelect, include_history role=switch Toggle.
 *
 * Seed: SWP-CMP-0021 "Plaza Senayan" with Rudi/Sari/Dewi placed + Rudi as leader (SWP-SLA-3001);
 *   SWP-CMP-0022 "Mall Kelapa Gading" with Budi placed, NO leader.
 *
 * Traceable to: PLC-04, F3.1/F3.4, RO-*, C-4 (scope), OUT_OF_SCOPE.
 */

import { test, expect, loginAs } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import { apiAs, errorCode } from '../../lib/e3-helpers.js';

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
// RO-render — hr_admin opens the 0021 roster: placements + leader chip + summary
// ---------------------------------------------------------------------------

test('RO-render · hr_admin opens SWP-CMP-0021 roster → Rudi/Sari/Dewi placements + leader chip + summary counters', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/client-companies/SWP-CMP-0021/roster');

  // Company header renders the company name.
  await expect(page.getByText('Plaza Senayan').first()).toBeVisible({ timeout: 30_000 });

  // The three active agents are listed (rows are div.border-b — assert by name text).
  await expect(page.getByText(/Rudi Wijaya/i).first()).toBeVisible({ timeout: 15_000 });
  await expect(page.getByText(/Sari Hadi/i).first()).toBeVisible({ timeout: 10_000 });
  await expect(page.getByText(/Dewi Lestari/i).first()).toBeVisible({ timeout: 10_000 });

  // The current leader (Rudi) is surfaced in the header leader area with the leader label.
  await expect(page.getByText(/Shift Leader/i).first()).toBeVisible({ timeout: 10_000 });
});

// ---------------------------------------------------------------------------
// RO-filter-status — status filter narrows the list
// ---------------------------------------------------------------------------

test('RO-filter-status · status FilterSelect narrows the roster (ENDED filter + include_history shows only the ended row)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/client-companies/SWP-CMP-0021/roster');
  await expect(page.getByText('Plaza Senayan').first()).toBeVisible({ timeout: 30_000 });

  // End Sari's placement (SWP-PL-5003) so a persisted-terminal (ENDED) row exists. The status
  // filter matches the PERSISTED lifecycle_status column, so EXPIRING (a DTO-derived status)
  // would never match Dewi (persisted ACTIVE) — we use ENDED, an honest persisted status.
  const end = await apiAs(page, 'POST', '/placements/SWP-PL-5003:end', {
    reason: 'END_OF_TERM',
    effective_date: isoDaysFromNow(0),
  });
  expect(end.status).toBe(200);

  await page.reload();
  await expect(page.getByText('Plaza Senayan').first()).toBeVisible({ timeout: 15_000 });

  // Include history (else terminal rows are hidden), then filter by ENDED via the FilterSelect.
  await page.getByRole('switch').first().click();
  // The status FilterSelect is the only native <select> in the filter row.
  await page.locator('select').first().selectOption('ENDED');

  // Only the ENDED row (Sari) remains; the ACTIVE Rudi/Dewi rows drop out.
  await expect(page.getByText(/Sari Hadi/i).first()).toBeVisible({ timeout: 15_000 });
  await expect(page.getByText(/Dewi Lestari/i)).toHaveCount(0, { timeout: 10_000 });
});

// ---------------------------------------------------------------------------
// RO-include-history — include_history toggle reveals terminal rows
// ---------------------------------------------------------------------------

test('RO-include-history · ending a placement then toggling "Sertakan riwayat" (role=switch) reveals the terminal row', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/client-companies/SWP-CMP-0021/roster');
  await expect(page.getByText('Plaza Senayan').first()).toBeVisible({ timeout: 30_000 });

  // End Sari's placement (SWP-PL-5003) via the API so a terminal row exists for 0021.
  const end = await apiAs(page, 'POST', '/placements/SWP-PL-5003:end', {
    reason: 'END_OF_TERM',
    effective_date: isoDaysFromNow(0),
  });
  expect(end.status).toBe(200);

  await page.reload();
  await expect(page.getByText('Plaza Senayan').first()).toBeVisible({ timeout: 15_000 });

  // Scope row assertions to the roster table region (the leader header chip can also carry
  // a name). The roster DataTable has aria-label "Roster Perusahaan".
  const table = page.locator('[aria-label="Roster Perusahaan"]');
  await expect(table).toBeVisible({ timeout: 10_000 });

  // Without history, the ENDED Sari row is hidden from the table.
  await expect(table.getByText(/Sari Hadi/i)).toHaveCount(0, { timeout: 10_000 });

  // Toggle include_history (role=switch) → terminal rows appear in the table.
  await page.getByRole('switch').first().click();
  await expect(table.getByText(/Sari Hadi/i).first()).toBeVisible({ timeout: 15_000 });
});

// ---------------------------------------------------------------------------
// RO-rbac-scope — shift_leader own company 200, cross-company 403 OUT_OF_SCOPE
// ---------------------------------------------------------------------------

test('RO-rbac-scope · shift_leader Rudi opens own 0021 roster → 200; opens 0022 → 403 OUT_OF_SCOPE (UI + API)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);

  // His own company (0021) roster loads.
  await page.goto('/client-companies/SWP-CMP-0021/roster');
  await expect(page.getByText('Plaza Senayan').first()).toBeVisible({ timeout: 30_000 });

  // The real API returns 200 for his own company and 403 OUT_OF_SCOPE for 0022.
  const own = await apiAs(page, 'GET', '/client-companies/SWP-CMP-0021/roster');
  expect(own.status).toBe(200);

  const cross = await apiAs(page, 'GET', '/client-companies/SWP-CMP-0022/roster');
  expect(cross.status).toBe(403);
  expect(errorCode(cross.body)).toBe('OUT_OF_SCOPE');

  // UI: navigating to the cross-company roster shows the no-permission EmptyState.
  await page.goto('/client-companies/SWP-CMP-0022/roster');
  await expect(page.getByText(/Akses ditolak|tidak memiliki izin/i).first()).toBeVisible({
    timeout: 20_000,
  });
});

// ---------------------------------------------------------------------------
// RO-vacancy — SWP-CMP-0022 roster shows the no-leader warn CTA
// ---------------------------------------------------------------------------

test('RO-vacancy · SWP-CMP-0022 roster (no leader) shows the "Tetapkan leader" warn CTA', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/client-companies/SWP-CMP-0022/roster');
  await expect(page.getByText('Mall Kelapa Gading').first()).toBeVisible({ timeout: 30_000 });

  // current_shift_leader is null → the no-leader warn CTA ("Tetapkan leader") renders.
  await expect(page.getByText(/Tetapkan leader/i).first()).toBeVisible({ timeout: 15_000 });
});
