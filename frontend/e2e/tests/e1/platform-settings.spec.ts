/**
 * tests/e1/platform-settings.spec.ts
 *
 * Platform settings E2E suite — one test() per scenario/case from
 * docs/epics/E1-foundations/prds/platform-conventions.md §7 Acceptance criteria.
 *
 * Each test is named with its FND-03/PC-# so it is individually selectable in
 * `playwright test --ui` and traceable back to the spec.
 *
 * Coverage:
 *   FND-03/PC-1/PC-2/PC-5  settings load from the real BE (locale / timezone / currency)
 *
 * Stack: real Vite dev server (:4173, MSW off) ↔ real Go API (:8081) ↔ ephemeral Postgres (:5433).
 * Boot: globalSetup (lib/backend.ts). Isolation: resetDb() in beforeEach.
 *
 * Traceable to: FND-03, PC-1, PC-2, PC-5, HARN-01, e2e-harness-spec.md §Coverage.
 */

import { test, expect, loginAs } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';

// ---------------------------------------------------------------------------
// Isolation — each test starts from a clean, fully-seeded DB.
// ---------------------------------------------------------------------------
test.beforeEach(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// FND-03 / PC-1 / PC-2 / PC-5 — Settings load from the real BE
// ---------------------------------------------------------------------------

test('FND-03/PC-1/PC-2/PC-5 · platform settings load locale, timezone, and currency from the real BE', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);

  // Navigate to the settings general route: /settings/general (settingsGeneralRoute).
  await page.goto('/settings/general');

  // Page heading: "Pengaturan" (id.ts settingsGeneral.title).
  // Allow extra time for session restore + API data load on fresh navigation.
  await expect(page.getByRole('heading', { name: 'Pengaturan' })).toBeVisible({ timeout: 30_000 });

  // The Lokalisasi card renders API-fed rows via useGetPlatformSettings.
  // Section header: "Lokalisasi" (id.ts settingsGeneral.section.localization → 'Lokalisasi').
  await expect(page.getByText('Lokalisasi')).toBeVisible({ timeout: 10_000 });

  // PC-1 — Locale row: the label from the BE for locale is "Bahasa Indonesia"
  // (the openapi spec seeds platform_settings with label='Bahasa Indonesia').
  // Use .first() in case text appears multiple times (label and value may both contain it).
  await expect(page.getByText('Bahasa Indonesia').first()).toBeVisible({ timeout: 10_000 });

  // PC-2 — Timezone row: value "Asia/Jakarta" (WIB) from the BE.
  // Use .first() because both label and value may contain "Asia/Jakarta".
  await expect(page.getByText('Asia/Jakarta').first()).toBeVisible({ timeout: 10_000 });

  // PC-5 — Currency row: value "IDR" or label containing "Rupiah" or "IDR"
  // (the seed sets currency label='Rupiah (IDR)' or the value='IDR').
  await expect(
    page.getByText(/Rupiah|IDR/).first(),
  ).toBeVisible({ timeout: 10_000 });

  // The section card also shows "Keamanan" (id.ts settingsGeneral.section.security).
  await expect(page.getByText('Keamanan')).toBeVisible();

  // The "v1 terkunci" / locked chips on the locale/timezone/currency rows confirm
  // the settings are read-only (PC-3: platform conventions locked in v1).
  // The SettingRow with locked=true renders a "Terkunci" chip.
  await expect(page.getByText('Terkunci').first()).toBeVisible();
});
