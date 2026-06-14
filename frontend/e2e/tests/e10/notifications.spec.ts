/**
 * tests/e10/notifications.spec.ts  ·  @e10-notif
 *
 * E10 · Notification center (GET /notifications + :mark-read + :mark-all-read) — NT-1..NT-6.
 *
 * Proves notifications-screen.tsx against the REAL Go BE (MSW off):
 *   - list renders the seeded SWP-NTF-9000x fixtures grouped by date (HARI INI / KEMARIN / …);
 *   - the read-state pills filter (UNREAD shows only unread cards; the seed gives HR Sari two
 *     unread + one read fixture);
 *   - single mark-read: clicking an unread NotifCard fires :mark-read → it leaves the UNREAD
 *     list (unread → read flip);
 *   - mark-all-read: "Tandai semua sudah dibaca" → :mark-all-read (reads marked_count) → the
 *     button hides (unreadCount 0) and the UNREAD filter is empty;
 *   - empty state when filtered to a kind with no rows.
 *
 * Every selector is anchored on the REAL notifications-screen.tsx / NotifCard molecule:
 *   - read-state pills are <button> "Semua" / "Belum dibaca" / "Sudah dibaca";
 *   - the mark-all button text = "Tandai semua sudah dibaca" (hidden when unreadCount===0);
 *   - kind filter is a FilterSelect aria-label "Semua jenis";
 *   - each NotifCard is a <button>; unread cards carry a trailing dot (we assert via the
 *     UNREAD-filter list membership rather than the dot pixel).
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import { gotoReady, listNotificationsVia } from '../../lib/e10-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

/** Seeded HR (Sari) notification titles. */
const HR_UNREAD_LEAVE = 'Pengajuan cuti baru'; // SWP-NTF-90001, unread, deep_link /leave-requests/SWP-LR-8002
const HR_READ_VERIFY = 'Verifikasi kehadiran'; // SWP-NTF-90002, READ

const pill = (page: import('@playwright/test').Page, name: string) =>
  page.getByRole('button', { name, exact: true });

/**
 * A NotifCard (a real <button> when clickable) whose accessible name contains `title`.
 * Anchored on the card BUTTON so it never collides with the kind-filter <option> of the
 * same text (e.g. "Verifikasi kehadiran" exists as both a card and a filter option).
 */
const card = (page: import('@playwright/test').Page, title: string) =>
  page.getByRole('button', { name: new RegExp(title) });

// ---------------------------------------------------------------------------
// List render + read-state filter
// ---------------------------------------------------------------------------

test('NOTIF-list-filter · seeded list renders; UNREAD pill shows only unread cards', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await gotoReady(page, '/notifications');

  // Title band renders.
  await expect(page.getByRole('heading', { name: 'Notifikasi' })).toBeVisible({ timeout: 30_000 });

  // ALL: both the unread leave fixture AND the read verify fixture render as cards.
  await expect(card(page, HR_UNREAD_LEAVE).first()).toBeVisible();
  await expect(card(page, HR_READ_VERIFY).first()).toBeVisible();

  // UNREAD pill: only unread cards remain — the READ "Verifikasi kehadiran" card disappears.
  await pill(page, 'Belum dibaca').click();
  await expect(card(page, HR_UNREAD_LEAVE).first()).toBeVisible();
  await expect(card(page, HR_READ_VERIFY)).toHaveCount(0);
});

// ---------------------------------------------------------------------------
// Single mark-read: clicking an unread card flips it to read
// ---------------------------------------------------------------------------

test('NOTIF-mark-read · clicking an unread NotifCard flips it unread→read (leaves UNREAD list)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await gotoReady(page, '/notifications');

  // Filter to UNREAD so the only cards shown are unread; the leave fixture is present.
  await pill(page, 'Belum dibaca').click();
  const leaveCard = card(page, HR_UNREAD_LEAVE).first();
  await expect(leaveCard).toBeVisible({ timeout: 30_000 });

  // Click it → fires :mark-read. (The card also navigates its deep_link; we return to the list.)
  await leaveCard.click();

  // CONTRACT proof: the notification is now READ in the BE (read_at non-null).
  await gotoReady(page, '/notifications');
  await expect
    .poll(
      async () => {
        const rows = await listNotificationsVia(page, '?read_state=UNREAD&limit=50');
        return rows.some((n) => n.title === HR_UNREAD_LEAVE);
      },
      { timeout: 15_000 },
    )
    .toBe(false);

  // UI: the UNREAD list no longer shows it.
  await pill(page, 'Belum dibaca').click();
  await expect(card(page, HR_UNREAD_LEAVE)).toHaveCount(0, { timeout: 15_000 });
});

// ---------------------------------------------------------------------------
// Mark-all-read: button hides + UNREAD empty + marked_count toast
// ---------------------------------------------------------------------------

test('NOTIF-mark-all · "Tandai semua sudah dibaca" clears unread (button hides, UNREAD empty)', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await gotoReady(page, '/notifications');

  const markAll = page.getByRole('button', { name: 'Tandai semua sudah dibaca' });
  await expect(markAll).toBeVisible({ timeout: 30_000 });

  await markAll.click();

  // marked_count success toast renders (FE reads res.data.marked_count — the 11-04 fix).
  await expect(page.getByText(/notifikasi ditandai sudah dibaca/i).first()).toBeVisible({
    timeout: 15_000,
  });

  // The button hides once unreadCount === 0.
  await expect(markAll).toBeHidden({ timeout: 15_000 });

  // CONTRACT: no unread notifications remain for HR Sari.
  await expect
    .poll(
      async () => {
        const rows = await listNotificationsVia(page, '?read_state=UNREAD&limit=50');
        return rows.length;
      },
      { timeout: 15_000 },
    )
    .toBe(0);
});

// ---------------------------------------------------------------------------
// Empty state — filter to a kind with no rows
// ---------------------------------------------------------------------------

test('NOTIF-empty-kind · filtering to a kind with no notifications shows the filtered-empty state', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await gotoReady(page, '/notifications');

  await expect(page.getByRole('heading', { name: 'Notifikasi' })).toBeVisible({ timeout: 30_000 });

  // HR Sari has no SCHEDULE_CHANGED notifications → filtered-empty state.
  await page.getByLabel('Semua jenis').selectOption('SCHEDULE_CHANGED');

  await expect(page.getByText(/Tidak ada hasil/i).first()).toBeVisible({ timeout: 15_000 });
});
