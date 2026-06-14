/**
 * lib/e4-helpers.ts
 *
 * Shared UI/API helpers for the E4 schedule & shifts E2E specs. Every selector is
 * anchored on the REAL rendered component DOM (NOT assumptions):
 *
 *   - schedule-grid-screen.tsx: ClientCompanyPicker (Combobox), grid cells are
 *     <button aria-label={t('cell.ariaLabel',{agent,date})}> where the rendered
 *     Bahasa value is "{{agent}} — {{date}}" (id.ts schedule.cell.ariaLabel). We
 *     match a cell by [aria-label*="<agent>"][aria-label*="<date>"].
 *   - schedule-overlays.tsx ShiftPickerPopover: rooted in a div whose aria-label is
 *     t('picker.title',{name}) = "Pilih shift untuk {{name}}". Inside, each shift is a
 *     ShiftRow <button> whose text contains shift.name. Quick actions: "Tandai Libur (OFF)"
 *     / "Hapus jadwal". Success toast "Jadwal dipublish otomatis"; conflict toast title
 *     "Jadwal tidak dapat disimpan".
 *   - BulkApplyModal: shift search list (<button> rows), #start/#end omitted — date
 *     inputs are type=date with sibling labels "Mulai"/"Selesai"; weekday toggle buttons
 *     labelled Sen..Min; "Pratinjau"/"Terapkan" footer buttons; preview banner shows
 *     succeeded/failed counts.
 *
 * Re-exports apiAs / errorCode / errorDetails / API_BASE / pickCombobox / comboFieldById
 * from e3-helpers (the same real-409 + Combobox patterns) so e4 specs import from one lib.
 */

import { expect, type Locator, type Page } from '@playwright/test';
import {
  API_BASE,
  apiAs,
  comboFieldById,
  errorCode,
  errorDetails,
  pickCombobox,
} from './e3-helpers.js';

export { API_BASE, apiAs, comboFieldById, errorCode, errorDetails, pickCombobox };

/**
 * waitForToken — block until the in-memory access token (window.__swp_get_token__) is
 * hydrated. After a full page.goto(), JS module memory is reset and the token is
 * repopulated ASYNCHRONOUSLY by tryRestoreSession (refresh-cookie → /auth/refresh).
 * apiAs() needs the Bearer token, so call this before the first apiAs on a freshly
 * navigated page to avoid a 401 race.
 */
export async function waitForToken(page: Page): Promise<void> {
  await expect
    .poll(
      () =>
        page.evaluate(
          () => (window as unknown as { __swp_get_token__?: string }).__swp_get_token__ ?? null,
        ),
      { timeout: 20_000 },
    )
    .toBeTruthy();
}

// ---------------------------------------------------------------------------
// Asia/Jakarta-anchored week dates (mirror the seed's mondayOfCurrentWeek and the
// web grid, which both anchor "today"/the week on the Asia/Jakarta calendar date —
// see backend/cmd/seed/seed.go + schedule-grid-screen.tsx). The seed plants:
//   monday+1 (Tue) SWP-SCH-6001 Rudi (SWP-EMP-1108)
//   monday+2 (Wed) SWP-SCH-6002 Dewi (SWP-EMP-3001)
//   monday+3 (Thu) approved-leave SWP-LR-44210 for Dewi (SWP-EMP-3001)
// We compute the same Monday so the negative/positive dates line up exactly, even
// across the UTC↔WIB midnight boundary (where the UTC date is a day behind WIB).
// ---------------------------------------------------------------------------

/** Today's calendar date in Asia/Jakarta as "YYYY-MM-DD" (matches the grid + seed). */
export function todayJakartaIso(): string {
  return new Date().toLocaleDateString('en-CA', { timeZone: 'Asia/Jakarta' });
}

/** Monday (Asia/Jakarta calendar date) of the current week, "YYYY-MM-DD" — matches the seed. */
export function mondayOfCurrentWeekIso(): string {
  const [y, m, dd] = todayJakartaIso().split('-').map(Number);
  const d = new Date(Date.UTC(y, m - 1, dd));
  const offset = (d.getUTCDay() + 6) % 7; // Go: (Weekday()+6)%7 — ISO Monday-start
  d.setUTCDate(d.getUTCDate() - offset);
  return d.toISOString().slice(0, 10);
}

/** Add N days to a "YYYY-MM-DD" string (UTC-safe). */
export function addDaysIso(iso: string, n: number): string {
  const [y, m, dd] = iso.split('-').map(Number);
  const d = new Date(Date.UTC(y, m - 1, dd));
  d.setUTCDate(d.getUTCDate() + n);
  return d.toISOString().slice(0, 10);
}

/** N days from today (Asia/Jakarta), "YYYY-MM-DD" — matches the grid's Jakarta "today". */
export function isoDaysFromNow(days: number): string {
  return addDaysIso(todayJakartaIso(), days);
}

// Convenience anchors keyed off the seed's Monday.
export const SEED = {
  monday: () => mondayOfCurrentWeekIso(),
  /** Tuesday — Rudi (EMP-1108) seeded entry SWP-SCH-6001. */
  rudiEntryDate: () => addDaysIso(mondayOfCurrentWeekIso(), 1),
  /** Wednesday — Dewi (EMP-3001) seeded entry SWP-SCH-6002. */
  dewiEntryDate: () => addDaysIso(mondayOfCurrentWeekIso(), 2),
  /** Thursday — Dewi (EMP-3001) approved-leave SWP-LR-44210 (over-leave target). */
  dewiLeaveDate: () => addDaysIso(mondayOfCurrentWeekIso(), 3),
  /** Friday — an in-window, free cell for happy-path assigns. */
  freeDate: () => addDaysIso(mondayOfCurrentWeekIso(), 4),
} as const;

// ---------------------------------------------------------------------------
// Company picker (ClientCompanyPicker = Combobox in schedule-grid-screen)
// ---------------------------------------------------------------------------

/**
 * Drive the ClientCompanyPicker (the Combobox in the grid header) to `name`.
 *
 * A shift_leader is scoped to exactly ONE company, so the grid auto-selects it and
 * renders NO picker (the heading shows "Jadwal Mingguan — <company>"). hr_admin /
 * super_admin are global and DO get the picker. Wait for whichever appears, and only
 * drive the combobox when it exists — otherwise the company is already selected.
 */
export async function selectCompany(page: Page, name: string): Promise<void> {
  // The ClientCompanyPicker lives in a fixed-width wrapper `div.w-72` right after the header.
  const trigger = page.locator('div.w-72 button[aria-haspopup="listbox"]').first();
  const heading = page.getByRole('heading', { name: /Jadwal Mingguan/i });

  // Wait until the header settles: either the picker (global roles) or the
  // auto-selected grid heading (single-company leader) is on screen.
  await expect(trigger.or(heading).first()).toBeVisible({ timeout: 30_000 });

  if ((await trigger.count()) > 0) {
    const scope = page.locator('div.w-72').first();
    await pickCombobox(page, scope, name, name);
  }
}

// ---------------------------------------------------------------------------
// Grid cell button (aria-label = "{{agent}} — {{date}}")
// ---------------------------------------------------------------------------

/**
 * gridCellDateLabel — format an ISO date the SAME way the grid does in the cell
 * aria-label (schedule-grid-screen.tsx formatDayMonthId: id-ID, day-numeric +
 * short month, parsed as UTC midnight). The aria-label embeds e.g. "13 Jun", NOT
 * the ISO date, so matching on the raw ISO never hits.
 */
export function gridCellDateLabel(dateIso: string): string {
  const [y, m, d] = dateIso.split('-').map(Number);
  return new Intl.DateTimeFormat('id-ID', {
    day: 'numeric',
    month: 'short',
    timeZone: 'UTC',
  }).format(new Date(Date.UTC(y, m - 1, d)));
}

/** Locate the grid cell button for an agent on a given ISO date. */
export function cellButton(page: Page, agentName: string, dateIso: string): Locator {
  const dateLabel = gridCellDateLabel(dateIso);
  return page
    .locator(`button[aria-label*="${agentName}"][aria-label*="${dateLabel}"]`)
    .first();
}

/** Click the grid cell for an agent+date to open the ShiftPickerPopover. */
export async function openCell(page: Page, agentName: string, dateIso: string): Promise<void> {
  const cell = cellButton(page, agentName, dateIso);
  await expect(cell).toBeVisible({ timeout: 20_000 });
  await cell.click();
}

// ---------------------------------------------------------------------------
// ShiftPickerPopover interactions
// ---------------------------------------------------------------------------

/** The open ShiftPickerPopover root (aria-label "Pilih shift untuk {name}"). */
export function popover(page: Page): Locator {
  return page.locator('[aria-label^="Pilih shift untuk"]').first();
}

/** Inside the open popover, click the ShiftRow button whose text contains `shiftName`. */
export async function assignShiftViaPopover(page: Page, shiftName: string): Promise<void> {
  const pop = popover(page);
  await expect(pop).toBeVisible({ timeout: 10_000 });
  const row = pop.locator('button', { hasText: shiftName }).first();
  await expect(row).toBeVisible({ timeout: 10_000 });
  await row.click();
}

/** Inside the open popover, click "Tandai Libur (OFF)". */
export async function markDayOffViaPopover(page: Page): Promise<void> {
  const pop = popover(page);
  await pop.getByText('Tandai Libur (OFF)').first().click();
}

/** Inside the open popover, click "Hapus jadwal" (clear cell). */
export async function clearCellViaPopover(page: Page): Promise<void> {
  const pop = popover(page);
  await pop.getByText('Hapus jadwal').first().click();
}

// ---------------------------------------------------------------------------
// Toast assertions
// ---------------------------------------------------------------------------

/** Assert the auto-publish success toast ("Jadwal dipublish otomatis") appears. */
export async function expectPublishedToast(page: Page): Promise<void> {
  await expect(page.getByText('Jadwal dipublish otomatis').first()).toBeVisible({
    timeout: 15_000,
  });
}

/** Assert the cleared toast ("Jadwal dihapus") appears. */
export async function expectClearedToast(page: Page): Promise<void> {
  await expect(page.getByText('Jadwal dihapus').first()).toBeVisible({ timeout: 15_000 });
}

/** Assert the conflict block toast title + (optional) a message regex. */
export async function expectConflictToast(page: Page, messageRegex?: RegExp): Promise<void> {
  await expect(page.getByText('Jadwal tidak dapat disimpan').first()).toBeVisible({
    timeout: 15_000,
  });
  if (messageRegex) {
    await expect(page.getByText(messageRegex).first()).toBeVisible({ timeout: 10_000 });
  }
}
