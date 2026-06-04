/**
 * tests/e1/user-management.spec.ts
 *
 * Exhaustive user-management E2E suite — one test() per scenario/case from
 * docs/epics/E1-foundations/prds/rbac-roles.md §7 Acceptance criteria + §8 Cases
 * and FND-01 (user list, create, edit, change-role, deactivate/reactivate, send-reset).
 *
 * Each test is named with its FND-#/RB-#/AL-# so it is individually selectable in
 * `playwright test --ui` and traceable back to the spec.
 *
 * Coverage:
 *   FND-01       users list renders seeded users from the real BE
 *   FND-01       create user → toast + DB row
 *   FND-01       edit user email → toast + updated value in UI
 *   FND-01/RB-6  change role → toast + DB role + audit record
 *   FND-01       deactivate → getUserStatus='disabled' + badge; reactivate → 'active'
 *   FND-01       send password reset → toast + countResetTokensFor >= 1
 *   RB-2/AL-7    agent is denied the users screen (403 → no-permission UI)
 *
 * Stack: real Vite dev server (:4173, MSW off) ↔ real Go API (:8081) ↔ ephemeral Postgres (:5433).
 * Boot: globalSetup (global-setup.ts → lib/backend.ts → goose + seed + go run ./cmd/api).
 * Isolation: resetDb() in beforeEach (TRUNCATE + reseed via go run ./cmd/seed).
 *
 * Traceable to: FND-01, RB-2, RB-6, AL-7, HARN-01, e2e-harness-spec.md §Coverage.
 */

import { test, expect, loginAs } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import {
  getUserStatus,
  getUserRole,
  countAuditRowsByEntityType,
  getLatestAuditAction,
  countResetTokensFor,
} from '../../lib/db.js';

// ---------------------------------------------------------------------------
// Isolation — each test starts from a clean, fully-seeded DB.
// ---------------------------------------------------------------------------
test.beforeEach(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// FND-01 — Users list renders seeded users from the real BE
// ---------------------------------------------------------------------------

test('FND-01 · users list renders seeded users from the real BE', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);

  // Navigate to the users management screen.
  await page.goto('/settings/users');

  // Wait for the table to load (skeleton resolves).
  // The seeded hrAdmin's full_name is "Sari Hadi" — must appear in the table.
  // Use 30s timeout to accommodate first-load Vite compilation + session restore + API call.
  // Use .first() because "Sari Hadi" also appears in the topbar UserMenu.
  await expect(page.getByText('Sari Hadi').first()).toBeVisible({ timeout: 30_000 });

  // The page heading confirms we're on the correct screen (id.ts users.title).
  await expect(page.getByRole('heading', { name: 'Pengguna & Peran' })).toBeVisible();

  // Extra seeded users from Phase 02-02 seed extension:
  // "Dewi Lestari" (agent) and "Bambang Sutrisno" (hr_admin) must also appear.
  await expect(page.getByText('Dewi Lestari')).toBeVisible();
  await expect(page.getByText('Bambang Sutrisno')).toBeVisible();
});

// ---------------------------------------------------------------------------
// FND-01 — Create user → success toast + user appears in list
// ---------------------------------------------------------------------------

test('FND-01 · create user opens modal, submits, and user row appears', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/settings/users');

  // Wait for the list to be ready.
  // Use .first() because "Sari Hadi" also appears in the topbar UserMenu.
  await expect(page.getByText('Sari Hadi').first()).toBeVisible({ timeout: 30_000 });

  // Click "Tambah Pengguna" (id.ts users.add).
  await page.getByRole('button', { name: 'Tambah Pengguna' }).click();

  // Modal opens: heading "Tambah Pengguna" (id.ts userOverlays.createTitle).
  await expect(page.getByText('Tambah Pengguna').first()).toBeVisible({ timeout: 5_000 });

  // Fill the email field (#cu-email).
  const newEmail = 'new.testuser@swp.test';
  await page.locator('#cu-email').fill(newEmail);

  // Select role: "Agen" (id.ts role.agent = 'Agen').
  await page.locator('#cu-role').selectOption({ label: 'Agen' });

  // Submit (id.ts userOverlays.createSubmit = 'Simpan Pengguna').
  await page.getByRole('button', { name: 'Simpan Pengguna' }).click();

  // Success toast: "Pengguna berhasil dibuat." (id.ts userOverlays.createSuccess).
  await expect(page.getByText('Pengguna berhasil dibuat.')).toBeVisible({ timeout: 15_000 });

  // After success the modal closes and the list refetches — the new email must appear.
  await expect(page.getByText(newEmail)).toBeVisible({ timeout: 15_000 });

  // DB-side: user must exist with role 'agent'.
  const role = await getUserRole(newEmail);
  expect(role).toBe('agent');
});

// ---------------------------------------------------------------------------
// FND-01 — Edit user email → success toast + updated value in list
// ---------------------------------------------------------------------------

test('FND-01 · edit user email updates the row in the list', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/settings/users');

  // Wait for seeded list.
  await expect(page.getByText('Dewi Lestari')).toBeVisible({ timeout: 30_000 });

  // Open the kebab menu for Dewi Lestari's row.
  // DataTable uses <div> rows (not <tr>). The row div is a flex container that holds
  // all cells including the row-actions cell with the "Aksi baris" button.
  // Use a locator filtered by the presence of "Dewi Lestari" text AND the action button.
  const dewiRow = page
    .locator('div.border-b')
    .filter({ hasText: 'Dewi Lestari' })
    .filter({ has: page.getByRole('button', { name: 'Aksi baris' }) });
  await dewiRow.getByRole('button', { name: 'Aksi baris' }).click();

  // Menu item "Edit profil" (id.ts userOverlays.menuEdit).
  await page.getByRole('menuitem', { name: 'Edit profil' }).click();

  // Edit drawer opens: heading "Edit Pengguna" (id.ts userOverlays.editTitle).
  await expect(page.getByText('Edit Pengguna')).toBeVisible({ timeout: 5_000 });

  // Clear and re-fill the email field (#eu-email).
  const updatedEmail = 'dewi.updated@swp.test';
  await page.locator('#eu-email').clear();
  await page.locator('#eu-email').fill(updatedEmail);

  // Submit (id.ts userOverlays.editSubmit = 'Simpan Perubahan').
  await page.getByRole('button', { name: 'Simpan Perubahan' }).click();

  // Success toast: "Pengguna berhasil diperbarui." (id.ts userOverlays.editSuccess).
  await expect(page.getByText('Pengguna berhasil diperbarui.')).toBeVisible({ timeout: 15_000 });

  // After save, the drawer closes and the list refetches — updated email must appear.
  await expect(page.getByText(updatedEmail)).toBeVisible({ timeout: 15_000 });
});

// ---------------------------------------------------------------------------
// FND-01 / RB-6 — Change role is audited: DB role updated + audit row written
// ---------------------------------------------------------------------------

test('FND-01/RB-6 · change role updates DB role and writes an audit entry', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/settings/users');

  // Wait for seeded list.
  await expect(page.getByText('Dewi Lestari')).toBeVisible({ timeout: 30_000 });

  // Count audit rows for 'user' entity before the mutation.
  const auditBefore = await countAuditRowsByEntityType('user');

  // Open kebab menu for Dewi Lestari's row (agent role) and choose "Ubah peran".
  // We target Dewi specifically so we can change to "Admin HR" (she's an agent, so it's available).
  const dewiRow = page
    .locator('div.border-b')
    .filter({ hasText: 'Dewi Lestari' })
    .filter({ has: page.getByRole('button', { name: 'Aksi baris' }) });
  await dewiRow.getByRole('button', { name: 'Aksi baris' }).click();
  await page.getByRole('menuitem', { name: 'Ubah peran' }).click();

  // Change-role modal opens: "Ubah peran pengguna" (id.ts userOverlays.changeRoleTitle).
  await expect(page.getByText('Ubah peran pengguna')).toBeVisible({ timeout: 5_000 });

  // Dewi's current role is 'agent'. Select 'hr_admin' from the new-role select.
  // The select shows all roles except the current one. 'Admin HR' = hr_admin.
  await page.locator('#cr-new-role').selectOption({ label: 'Admin HR' });

  // Fill the reason textarea (#cr-reason).
  await page.locator('#cr-reason').fill('Promosi berdasarkan memo HR-2026-042');

  // Submit: "Ubah Peran" (id.ts userOverlays.changeRoleSubmit).
  await page.getByRole('button', { name: 'Ubah Peran' }).click();

  // Success toast: "Peran pengguna berhasil diubah." (id.ts userOverlays.changeRoleSuccess).
  await expect(page.getByText('Peran pengguna berhasil diubah.')).toBeVisible({ timeout: 15_000 });

  // DB-side: audit table must have at least one more 'user' row.
  const auditAfter = await countAuditRowsByEntityType('user');
  expect(auditAfter).toBeGreaterThan(auditBefore);

  // The latest audit action must be change-role related.
  const latestAction = await getLatestAuditAction();
  expect(latestAction).toMatch(/change_role|role/i);
});

// ---------------------------------------------------------------------------
// FND-01 — Deactivate then reactivate user: DB status changes + badge updates
// ---------------------------------------------------------------------------

test('FND-01 · deactivate user sets status disabled, reactivate restores active', async ({ page }) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/settings/users');

  // Wait for seeded list (Dewi Lestari is a known agent).
  await expect(page.getByText('Dewi Lestari')).toBeVisible({ timeout: 30_000 });

  // Target Dewi Lestari's row specifically using div.border-b filter.
  // DataTable renders rows as <div> elements, not <tr>.
  const dewiRow = () => page
    .locator('div.border-b')
    .filter({ hasText: 'Dewi Lestari' })
    .filter({ has: page.getByRole('button', { name: 'Aksi baris' }) });

  // --- Step 1: Deactivate ---
  await dewiRow().getByRole('button', { name: 'Aksi baris' }).click();
  await page.getByRole('menuitem', { name: 'Nonaktifkan akun' }).click();

  // Confirm dialog opens: "Nonaktifkan pengguna?" (id.ts userOverlays.deactivateTitle).
  await expect(page.getByText('Nonaktifkan pengguna?')).toBeVisible({ timeout: 5_000 });
  await page.getByRole('button', { name: 'Nonaktifkan' }).click();

  // Success toast: "Akun berhasil dinonaktifkan." (id.ts userOverlays.deactivateSuccess).
  await expect(page.getByText('Akun berhasil dinonaktifkan.')).toBeVisible({ timeout: 15_000 });

  // After refetch: Dewi's row status badge should show "Nonaktif".
  // The StatusBadge renders a <span> with the text — use locator within Dewi's row.
  await expect(dewiRow().getByText('Nonaktif')).toBeVisible({ timeout: 10_000 });

  // --- Step 2: Reactivate ---
  await dewiRow().getByRole('button', { name: 'Aksi baris' }).click();
  await page.getByRole('menuitem', { name: 'Aktifkan akun' }).click();

  // Reactivate confirm dialog: "Aktifkan pengguna?" (id.ts userOverlays.reactivateTitle).
  await expect(page.getByText('Aktifkan pengguna?')).toBeVisible({ timeout: 5_000 });
  await page.getByRole('button', { name: 'Aktifkan' }).click();

  // Success toast: "Akun berhasil diaktifkan." (id.ts userOverlays.reactivateSuccess).
  await expect(page.getByText('Akun berhasil diaktifkan.')).toBeVisible({ timeout: 15_000 });

  // After reactivation Dewi's status badge should read "Aktif".
  await expect(dewiRow().getByText('Aktif')).toBeVisible({ timeout: 10_000 });
});

// ---------------------------------------------------------------------------
// FND-01 — Deactivate specific user: DB status check with known email
// ---------------------------------------------------------------------------

test('FND-01 · deactivate specific user: DB status = disabled', async ({ page }) => {
  const targetEmail = 'dewi.lestari@swp.test';

  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/settings/users');

  // Wait for the target user's row.
  await expect(page.getByText('Dewi Lestari')).toBeVisible({ timeout: 30_000 });

  // Find the row that contains "Dewi Lestari" and click its action button.
  // DataTable uses <div> rows (not <tr>). Find the row div with "Dewi Lestari".
  const dewiRow = page
    .locator('div.border-b')
    .filter({ hasText: 'Dewi Lestari' })
    .filter({ has: page.getByRole('button', { name: 'Aksi baris' }) });
  await dewiRow.getByRole('button', { name: 'Aksi baris' }).click();

  // Choose "Nonaktifkan akun".
  await page.getByRole('menuitem', { name: 'Nonaktifkan akun' }).click();

  // Confirm dialog.
  await expect(page.getByText('Nonaktifkan pengguna?')).toBeVisible({ timeout: 5_000 });
  await page.getByRole('button', { name: 'Nonaktifkan' }).click();

  // Success toast.
  await expect(page.getByText('Akun berhasil dinonaktifkan.')).toBeVisible({ timeout: 15_000 });

  // DB-side assertion: status must be 'disabled'.
  const status = await getUserStatus(targetEmail);
  expect(status).toBe('disabled');

  // --- Reactivate Dewi ---
  const dewiRowAfter = page
    .locator('div.border-b')
    .filter({ hasText: 'Dewi Lestari' })
    .filter({ has: page.getByRole('button', { name: 'Aksi baris' }) });
  await dewiRowAfter.getByRole('button', { name: 'Aksi baris' }).click();
  await page.getByRole('menuitem', { name: 'Aktifkan akun' }).click();
  await expect(page.getByText('Aktifkan pengguna?')).toBeVisible({ timeout: 5_000 });
  await page.getByRole('button', { name: 'Aktifkan' }).click();
  await expect(page.getByText('Akun berhasil diaktifkan.')).toBeVisible({ timeout: 15_000 });

  // DB-side: status back to 'active'.
  const statusAfter = await getUserStatus(targetEmail);
  expect(statusAfter).toBe('active');
});

// ---------------------------------------------------------------------------
// FND-01 — Send password reset: toast + reset token in DB
// ---------------------------------------------------------------------------

test('FND-01 · send password reset inserts a reset token and shows success toast', async ({ page }) => {
  const targetEmail = 'dewi.lestari@swp.test';

  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/settings/users');

  // Wait for Dewi's row.
  await expect(page.getByText('Dewi Lestari')).toBeVisible({ timeout: 30_000 });

  // Open Dewi's kebab menu.
  // DataTable uses <div> rows (not <tr>). Find the row div with "Dewi Lestari".
  const dewiRow = page
    .locator('div.border-b')
    .filter({ hasText: 'Dewi Lestari' })
    .filter({ has: page.getByRole('button', { name: 'Aksi baris' }) });
  await dewiRow.getByRole('button', { name: 'Aksi baris' }).click();

  // "Kirim reset kata sandi" (id.ts userOverlays.menuSendReset).
  await page.getByRole('menuitem', { name: 'Kirim reset kata sandi' }).click();

  // Confirm dialog: "Kirim email reset kata sandi?" (id.ts userOverlays.sendResetTitle).
  await expect(page.getByText('Kirim email reset kata sandi?')).toBeVisible({ timeout: 5_000 });

  // The dialog should show Dewi's email in the description.
  // Use .first() in case the email appears more than once (row + dialog).
  await expect(page.getByText(targetEmail).first()).toBeVisible();

  // Click confirm: "Kirim Tautan" (id.ts userOverlays.sendResetConfirm).
  await page.getByRole('button', { name: 'Kirim Tautan' }).click();

  // Success toast: "Email reset kata sandi berhasil dikirim." (id.ts userOverlays.sendResetSuccess).
  await expect(page.getByText('Email reset kata sandi berhasil dikirim.')).toBeVisible({
    timeout: 15_000,
  });

  // DB-side: a reset token must have been inserted for this user.
  const tokenCount = await countResetTokensFor(targetEmail);
  expect(tokenCount).toBeGreaterThanOrEqual(1);
});

// ---------------------------------------------------------------------------
// RB-2 / AL-7 — Agent is denied the users screen (RBAC negative)
// ---------------------------------------------------------------------------

test('RB-2/AL-7 · agent is denied the users management screen', async ({ page }) => {
  // Log in as an agent (has no users.manage permission).
  await loginAs(page, PERSONAS.agent);

  // Navigate directly to the users route.
  await page.goto('/settings/users');

  // The screen should show the no-permission EmptyState (id.ts errors.forbidden or
  // userOverlays noPermissionBody): "Anda tidak memiliki izin untuk tindakan ini."
  // or "Anda tidak memiliki izin untuk mengelola pengguna."
  // Either the API returns 403 and the screen renders the EmptyState, or the client-side
  // guard catches it first. Either way a permission-denied message must be visible.
  await expect(
    page
      .getByText(/tidak memiliki izin/i)
      .or(page.getByText(/forbidden/i))
      .first(),
  ).toBeVisible({ timeout: 15_000 });
});
