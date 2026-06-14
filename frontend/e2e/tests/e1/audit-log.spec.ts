/**
 * tests/e1/audit-log.spec.ts
 *
 * Exhaustive audit-log E2E suite — one test() per Gherkin scenario/case from
 * docs/epics/E1-foundations/prds/audit-log.md §7 Acceptance criteria + §8 Cases.
 *
 * Each test is named with its FND-02/AL-# so it is individually selectable in
 * `playwright test --ui` and traceable back to the spec.
 *
 * Coverage:
 *   FND-02        audit log lists seeded entries from the real BE
 *   FND-02/AL-5   filter by entity_type=user shows only user-entity rows
 *   FND-02        cursor pagination advances to a second page (with extra seeded rows)
 *   FND-02        click a row opens the detail drawer with before/after content
 *   AL-7          agent is denied the audit-log screen (403 → no-permission UI)
 *
 * Note (C-2 bulk-granularity — AL-2 bulk delete sub-test): bulk operations are out of
 * scope for Phase 2 (no bulk-delete endpoint in E1). Skipped with a note.
 *
 * Stack: real Vite dev server (:4173, MSW off) ↔ real Go API (:8081) ↔ ephemeral Postgres (:5433).
 * Boot: globalSetup (lib/backend.ts). Isolation: resetDb() in beforeEach.
 *
 * Traceable to: FND-02, AL-5, AL-7, HARN-01, e2e-harness-spec.md §Coverage.
 */

import { test, expect, loginAs } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import { countAuditRowsByEntityType, insertAuditRows } from '../../lib/db.js';

// ---------------------------------------------------------------------------
// Isolation — each test starts from a clean, fully-seeded DB.
// ---------------------------------------------------------------------------
test.beforeEach(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// FND-02 — Audit log lists seeded entries from the real BE
// ---------------------------------------------------------------------------

test('FND-02 · audit log lists seeded entries from the real BE', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);

  // Navigate to the audit-log route: /settings/audit-log (settingsAuditRoute).
  await page.goto('/settings/audit-log');

  // Page heading: "Audit Log" (id.ts auditLog.title).
  // Allow extra time for session restore + API data load on fresh navigation.
  await expect(page.getByRole('heading', { name: 'Audit Log' })).toBeVisible({ timeout: 50_000 });

  // The seed (Phase 02-02) inserts 5 audit_log rows. At least one row must be visible.
  // Check the footer row count label — "Menampilkan 5 entri · append-only" confirms data loaded.
  // This is more reliable than searching for specific action text inside StatusBadge spans.
  await expect(page.getByText(/Menampilkan \d+ entri/).first()).toBeVisible({ timeout: 50_000 });

  // The subtitle "Catatan append-only..." must be present.
  await expect(page.getByText(/append-only/i).first()).toBeVisible();
});

// ---------------------------------------------------------------------------
// FND-02 / AL-5 — Filter by entity_type=user shows only user rows
// ---------------------------------------------------------------------------

test('FND-02/AL-5 · filter by entity type user shows only user-entity rows', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/settings/audit-log');

  // Wait for the initial list to load — look for the row count label in the footer.
  await expect(page.getByRole('heading', { name: 'Audit Log' })).toBeVisible({ timeout: 50_000 });
  await expect(page.getByText(/Menampilkan \d+ entri/).first()).toBeVisible({ timeout: 50_000 });

  // The filter for entity type has aria-label = "Filter entitas" (id.ts auditLog.filterEntityLabel).
  // Select "Pengguna" (user) from the entity-type filter select.
  await page.getByRole('combobox', { name: 'Filter entitas' }).selectOption({ label: 'Pengguna' });

  // Wait for the table to update (URL search params change triggers refetch).
  // After filtering to entity_type=user, only user-entity rows should be visible.
  // The footer label updates: previously "5 entri", now "4 entri" (4 user rows).
  await expect(page.getByText(/Menampilkan \d+ entri/).first()).toBeVisible({ timeout: 10_000 });

  // Count the rows via DB and assert the count matches what we seeded for 'user'.
  const dbCount = await countAuditRowsByEntityType('user');
  // After resetDb + seed: 4 user-entity rows in the seed (2x CREATE + user.change_role + user.deactivate).
  // The UI should show a count label containing the count.
  // "Menampilkan N entri · append-only" (id.ts auditLog.rowCount).
  // The footer renders this text twice (left label + pagination component). Use .first().
  await expect(page.getByText(new RegExp(`Menampilkan ${dbCount} entri`)).first()).toBeVisible({
    timeout: 5_000,
  });
});

// ---------------------------------------------------------------------------
// FND-02 — Cursor pagination advances (with extra seeded rows)
// ---------------------------------------------------------------------------

test('FND-02 · cursor pagination advances to the next page', async ({ page }) => {
  // Insert enough extra audit rows to exceed the PAGE_SIZE (50) so pagination is testable.
  // The screen uses PAGE_SIZE = 50. The seed has 5 rows. Add 50 more → 55 total.
  await insertAuditRows(50);

  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/settings/audit-log');

  // Wait for the table to load. With 55 rows, the first page (50) should show has_more=true.
  await expect(page.getByRole('heading', { name: 'Audit Log' })).toBeVisible({ timeout: 50_000 });
  // Footer label confirms data loaded ("Menampilkan 50 entri · append-only").
  await expect(page.getByText(/Menampilkan \d+ entri/).first()).toBeVisible({ timeout: 50_000 });

  // The CursorPagination "Berikutnya" (id.ts common.next) button should be enabled.
  // It is rendered inside the table footer only when rows.length > 0 and has_more=true.
  const nextBtn = page.getByRole('button', { name: 'Berikutnya' });
  await expect(nextBtn).toBeVisible({ timeout: 10_000 });
  await expect(nextBtn).toBeEnabled();

  // Capture the first page's "Menampilkan N entri" count text before navigating.
  // DataTable uses <div> rows (not <tr>/<td>), so capture the footer count label.
  const countBefore = await page.getByText(/Menampilkan \d+ entri/).first().textContent();

  // Click "Berikutnya".
  await nextBtn.click();

  // Wait for navigation/refetch — the URL should now contain a cursor param.
  await page.waitForURL(/cursor=/, { timeout: 10_000 });

  // The second page shows fewer rows (55 - 50 = 5 remaining).
  // The count label should change: "Menampilkan 50 entri" → "Menampilkan 5 entri".
  const countAfter = await page.getByText(/Menampilkan \d+ entri/).first().textContent();
  expect(countAfter).not.toEqual(countBefore);

  // "Sebelumnya" (id.ts common.prev) button must now be visible.
  const prevBtn = page.getByRole('button', { name: 'Sebelumnya' });
  await expect(prevBtn).toBeVisible();
  await expect(prevBtn).toBeEnabled();
});

// ---------------------------------------------------------------------------
// FND-02 — Click row opens detail drawer with before/after content
// ---------------------------------------------------------------------------

test('FND-02 · clicking an audit row opens the detail drawer with entry content', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/settings/audit-log');

  // Wait for list — footer count label confirms data is loaded.
  await expect(page.getByText(/Menampilkan \d+ entri/).first()).toBeVisible({ timeout: 50_000 });

  // Click the first data row (the DataTable has onRowClick set; rows are <div> not <tr>).
  // The DataTable renders rows as div.border-b with role="button" when onRowClick is set.
  // The audit-log screen passes onRowClick so rows have role="button".
  // Each data row is the first role="button" in the scroll body (excluding filter buttons).
  // Use the locator: div.border-b with role="button" to find the clickable data rows.
  // More specifically, target the data row that contains the audit time "4 Jun" pattern.
  const firstDataRow = page.locator('div.border-b[role="button"]').first();
  await firstDataRow.click();

  // The AuditDetailDrawer opens with the entry's data.
  // The drawer shows an "append-only" lock note at the footer (id.ts auditLog.appendOnlyNote).
  // This text is unique to the drawer and not shown elsewhere on the audit-log screen.
  await expect(page.getByText(/Entri append-only/i)).toBeVisible({ timeout: 10_000 });

  // The drawer body shows the diff section title "PERUBAHAN (before → after)"
  // with the full text including the arrow — this is distinct from the DataTable column
  // header which is just "PERUBAHAN" (without the "(before → after)" part).
  // Use exact text match on the full diffTitle string to avoid strict mode violation.
  await expect(page.getByText('PERUBAHAN (before → after)')).toBeVisible({ timeout: 5_000 });

  // The drawer has a close button labeled "Tutup" (id.ts common.close).
  await page.getByRole('button', { name: 'Tutup' }).last().click();

  // After closing, wait for the animation.
  await page.waitForTimeout(300);
});

// ---------------------------------------------------------------------------
// AL-7 — Agent is denied the audit log (RBAC negative)
// ---------------------------------------------------------------------------

test('AL-7 · agent is denied the audit log screen', async ({ page }) => {
  // Log in as an agent (no audit_log.read permission).
  await loginAs(page, PERSONAS.agent);

  // Navigate to the audit-log route.
  await page.goto('/settings/audit-log');

  // The screen should render the no-permission EmptyState:
  // Either "Anda tidak memiliki izin untuk tindakan ini." (id.ts errors.forbidden)
  // or "Audit log hanya untuk HR/Super Admin." (id.ts auditLog.noPermissionBody).
  await expect(
    page
      .getByText(/tidak memiliki izin/i)
      .or(page.getByText(/hanya untuk HR/i))
      .first(),
  ).toBeVisible({ timeout: 15_000 });
});

// ---------------------------------------------------------------------------
// C-2 / bulk granularity — Skipped (N/A for Phase 2)
// ---------------------------------------------------------------------------

test.skip('AL-2/C-2 · bulk delete — N/A for Phase 2 (no bulk-delete endpoint in E1)', async () => {
  // Bulk delete is not part of the E1 MVP endpoints. The audit log is append-only.
  // This scenario will be revisited if a bulk-admin action is added in a later epic.
  // Reference: audit-log.md §8 C-2, EPICS.md §8 E1.
});
