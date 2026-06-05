/**
 * tests/e8/audit-notes.spec.ts
 *
 * E8 · Payslip audit notes — list + append-only create (PAY-02) against the REAL stack.
 * HR opens a payslip detail, opens the append-only audit-note Drawer ("Tambah Catatan"),
 * sees the seeded notes, creates a new note (≥ 8 chars per the FE Zod min), and the new
 * note appears in the list (refetch). The < 8-char validation is blocked client-side.
 *
 * Selectors anchored on payslip-detail-screen.tsx (onAddNote → drawer) + audit-note-drawer.tsx:
 * the drawer textarea is #audit-note-text (RHF+Zod min 8 / max 1000, noValidate present),
 * submit t('auditNotes.submit')="Tambahkan catatan"; success toast t('auditNotes.saveSuccess')
 * ="Catatan ditambahkan & tercatat audit."; min-length error t('auditNotes.validation.minLength')
 * ="Minimal 8 karakter."
 *
 * Seed (10-02): SWP-PS-90119 has two seeded notes (SWP-PS-90119-NOTE-1/2, author "Sari Hadi").
 * A note created on SWP-PS-90121 gets the composite id SWP-PS-90121-NOTE-1.
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import { PS, PS_NOTE, PS_NOTE_AUTHOR, apiAs, waitForToken } from '../../lib/e8-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

const NOTE_TEXTAREA = '#audit-note-text';
const SUBMIT = 'Tambahkan catatan';

/**
 * Open a payslip detail + the audit-note drawer ("Tambah Catatan"). Land on /payroll
 * FIRST + waitForToken so the token is hydrated before the deep detail route fires its
 * GET (avoids the post-goto auth-restore "context canceled" 500 → error-state race).
 */
async function openDrawer(page: import('@playwright/test').Page, id: string): Promise<void> {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/payroll');
  await waitForToken(page);
  await page.goto(`/payroll/${id}`);
  await waitForToken(page);
  await expect(page.getByText(id).first()).toBeVisible({ timeout: 30_000 });
  await page.getByRole('button', { name: 'Tambah Catatan' }).click();
  await expect(page.locator(NOTE_TEXTAREA)).toBeVisible({ timeout: 20_000 });
}

// ---------------------------------------------------------------------------
// NOTES-list — the two seeded notes on SWP-PS-90119 render with the author name
// ---------------------------------------------------------------------------

test('NOTES-list · drawer for SWP-PS-90119 lists the two seeded notes (author Sari Hadi)', async ({
  page,
}) => {
  await openDrawer(page, PS.decryptFail);

  // Both seeded notes' text + the author render in the drawer list. The detail screen ALSO
  // shows an inline notes section, so the note text appears twice (detail + drawer) — assert
  // within the drawer dialog to disambiguate.
  const drawer = page.getByRole('dialog');
  await expect(drawer.getByText(/Decrypt failed pada migrasi/)).toBeVisible();
  await expect(drawer.getByText(/Konfirmasi key payroll lama/)).toBeVisible();
  await expect(drawer.getByText(PS_NOTE_AUTHOR).first()).toBeVisible();

  // Confirm the seeded composite ids via the API (oldest-first).
  const res = await apiAs(page, 'GET', `/payslips/${PS.decryptFail}/audit-notes`);
  expect(res.status).toBe(200);
  const notes = (res.body as { data?: Array<{ id: string }> })?.data ?? [];
  const ids = notes.map((n) => n.id);
  expect(ids).toContain(PS_NOTE.one);
  expect(ids).toContain(PS_NOTE.two);
});

// ---------------------------------------------------------------------------
// NOTES-create — appending a note (≥8 chars) succeeds + appears in the list
// ---------------------------------------------------------------------------

test('NOTES-create · appending a ≥8-char note on SWP-PS-90121 succeeds + appears (composite NOTE-1)', async ({
  page,
}) => {
  await openDrawer(page, PS.final);

  const text = 'Verifikasi take-home pay dengan tim finance E2E.';
  await page.locator(NOTE_TEXTAREA).fill(text);
  await page.getByRole('button', { name: SUBMIT }).click();

  // Success toast.
  await expect(page.getByText('Catatan ditambahkan & tercatat audit.')).toBeVisible({
    timeout: 20_000,
  });

  // The new note appears in the drawer list (refetch). Scope to the dialog (the detail
  // screen's inline notes section may also render it after its own refetch).
  await expect(page.getByRole('dialog').getByText(text)).toBeVisible({ timeout: 20_000 });

  // And the API confirms the composite id + the text.
  const res = await apiAs(page, 'GET', `/payslips/${PS.final}/audit-notes`);
  expect(res.status).toBe(200);
  const notes = (res.body as { data?: Array<{ id: string; text: string }> })?.data ?? [];
  const created = notes.find((n) => n.id === `${PS.final}-NOTE-1`);
  expect(created).toBeTruthy();
  expect(created?.text).toBe(text);
});

// ---------------------------------------------------------------------------
// NOTES-validation — a < 8-char note is blocked client-side (Zod min 8)
// ---------------------------------------------------------------------------

test('NOTES-validation · a < 8-char note is blocked client-side with the min-length message', async ({
  page,
}) => {
  await openDrawer(page, PS.final);

  await page.locator(NOTE_TEXTAREA).fill('short'); // 5 chars < 8
  await page.getByRole('button', { name: SUBMIT }).click();

  // The Zod min-length error renders inline; no success toast.
  await expect(page.getByText('Minimal 8 karakter.')).toBeVisible({ timeout: 10_000 });
  await expect(page.getByText('Catatan ditambahkan & tercatat audit.')).toHaveCount(0);

  // The note was NOT persisted (no composite NOTE-1 created on this payslip).
  const res = await apiAs(page, 'GET', `/payslips/${PS.final}/audit-notes`);
  expect(res.status).toBe(200);
  const notes = (res.body as { data?: Array<{ id: string }> })?.data ?? [];
  expect(notes).toHaveLength(0);
});
