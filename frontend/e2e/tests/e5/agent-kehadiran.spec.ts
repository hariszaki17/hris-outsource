/**
 * tests/e5/agent-kehadiran.spec.ts
 *
 * Agent "Kehadiran" home (/me) — the merged dashboard + attendance + schedule surface
 * (brainstorm.pen frame nwlSV). Drives the REAL stack as the agent persona.
 *
 * Coverage:
 *   KEHADIRAN-LAYOUT  "Jadwal Minggu Ini" (left) + "Riwayat Kehadiran" (right) render
 *                     side by side (same row), not stacked top/bottom.
 *   ABSEN-MODAL       The "Absen Sekarang" CTA opens the Absen modal (frame GHxuN) with
 *                     the live clock, shift card (Jam Masuk / Jam Keluar) and the
 *                     Absen Masuk/Keluar action — i.e. a real popup, not an inline button.
 *   ABSEN-DETAIL      Clicking a history row opens the attendance detail modal.
 *
 * Stack: real Vite (:4173, MSW off) ↔ real Go API (:8081) ↔ ephemeral Postgres (:5433).
 * Isolation: resetDb() in beforeEach.
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { resetDb } from '../../lib/reset-db.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

test('KEHADIRAN-LAYOUT · schedule + history render side by side on /me', async ({ page }) => {
  await loginAs(page, 'agent');
  await page.goto('/me');

  const schedule = page.getByRole('heading', { name: 'Jadwal Minggu Ini' });
  const history = page.getByRole('heading', { name: 'Riwayat Kehadiran' });
  await expect(schedule).toBeVisible({ timeout: 15_000 });
  await expect(history).toBeVisible();

  // Side-by-side (two columns) ⇒ the two panel headers sit on roughly the same row.
  const a = await schedule.boundingBox();
  const b = await history.boundingBox();
  expect(a).not.toBeNull();
  expect(b).not.toBeNull();
  if (a && b) {
    expect(Math.abs(a.y - b.y)).toBeLessThan(8);
    expect(b.x).toBeGreaterThan(a.x); // history column is to the right of schedule
  }
});

test('ABSEN-MODAL · "Absen Sekarang" opens the clock modal with shift + jam fields', async ({
  page,
}) => {
  await loginAs(page, 'agent');
  await page.goto('/me');

  await page.getByRole('button', { name: 'Absen Sekarang' }).click();

  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible({ timeout: 10_000 });
  await expect(dialog.getByText('Jam Masuk')).toBeVisible();
  await expect(dialog.getByText('Jam Keluar')).toBeVisible();
  await expect(dialog.getByRole('button', { name: /Absen (Masuk|Keluar)/ })).toBeVisible();
});

test('ABSEN-DETAIL · clicking a history row opens the attendance detail modal', async ({
  page,
}) => {
  await loginAs(page, 'agent');
  await page.goto('/me');

  const historyPanel = page.locator('section', {
    has: page.getByRole('heading', { name: 'Riwayat Kehadiran' }),
  });
  await expect(historyPanel).toBeVisible({ timeout: 15_000 });

  const rows = historyPanel.getByRole('button');
  if ((await rows.count()) === 0) {
    test.skip(true, 'agent has no seeded attendance history');
    return;
  }

  await rows.first().click();
  await expect(page.getByRole('dialog').getByText('Detail Kehadiran')).toBeVisible({
    timeout: 10_000,
  });
});
