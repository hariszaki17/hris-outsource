/**
 * tests/e10/notifications-autodispatch.spec.ts  ·  @e10-capstone
 *
 * THE MILESTONE CAPSTONE — the notification LOOP-CLOSER, proven end-to-end against the
 * REAL stack (Go API + the un-stubbed River worker + ephemeral Postgres, MSW off):
 *
 *   A REAL prior-phase action (HR approves a seeded PENDING_HR leave via the real
 *   POST /leave-requests/{id}:approve-final) fires notify.Dispatch → the transactional
 *   outbox enqueues a NotificationArgs in the approval tx → the un-stubbed
 *   NotificationWorker INSERTs a `notifications` row for the SUBMITTER → that
 *   auto-dispatched LEAVE_APPROVED notification then APPEARS in the recipient's
 *   GET /notifications and in the /notifications UI, where mark-read flips it.
 *
 * This is NOT a seeded notification: the dispatched row references the JUST-approved
 * SWP-LR-8002 (the seeded LEAVE_APPROVED fixture references SWP-LR-8005), and it only
 * exists because the worker processed the real dispatch. The recipient is Dewi Lestari
 * (SWP-EMP-3001), the submitter of SWP-LR-8002.
 *
 * Selectors anchored on the REAL notifications-screen.tsx (NotifCard <button>, read-state
 * pills); the dispatch is driven via the real approve-final endpoint (apiAs) for
 * determinism, and the appearance is proven via both GET /notifications and the UI.
 */

import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';
import { DEWI, LR, apiAs, gotoReady, pollNotification, waitForToken } from '../../lib/e10-helpers.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

test('CAPSTONE-autodispatch · HR approve-final → auto-dispatched LEAVE_APPROVED appears in Dewi\'s notifications + mark-read flips it', async ({
  page,
}) => {
  // ----- 1) HR drives the REAL prior-phase action: approve-final on a PENDING_HR leave.
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/');
  await waitForToken(page);

  const approve = await apiAs(page, 'POST', `/leave-requests/${LR.dewiPendingHr}:approve-final`, {});
  expect(approve.status, `approve-final → ${approve.status}: ${JSON.stringify(approve.body)}`).toBe(
    200,
  );
  expect((approve.body as { data: { status: string } }).data.status).toBe('APPROVED');

  // ----- 2) The recipient (Dewi, the submitter) logs in and the AUTO-DISPATCHED
  //          notification appears — driven by the real worker, not a seed. Poll GET
  //          /notifications until the LEAVE_APPROVED row for the just-approved entity lands.
  await loginAs(page, DEWI);
  await page.goto('/notifications');
  await waitForToken(page);

  const dispatched = await pollNotification(
    page,
    (n) =>
      n.kind === 'LEAVE_APPROVED' &&
      n.read_at === null &&
      n.deep_link?.entity_id === LR.dewiPendingHr,
    { timeoutMs: 25_000 },
  );
  // Sanity: it references the just-approved leave (NOT the seeded SWP-LR-8005 fixture).
  expect(dispatched.deep_link?.entity_id).toBe(LR.dewiPendingHr);
  expect(dispatched.deep_link?.path).toContain(LR.dewiPendingHr);

  // ----- 3) In the UI: the new unread notification renders; clicking it flips unread→read.
  await gotoReady(page, '/notifications');

  // Filter to UNREAD so we deterministically target the unread auto-dispatched card.
  await page.getByRole('button', { name: 'Belum dibaca', exact: true }).click();
  const card = page.getByRole('button', { name: new RegExp(dispatched.title) }).first();
  await expect(card).toBeVisible({ timeout: 30_000 });

  // Click → fires :mark-read on the dispatched notification.
  await card.click();

  // CONTRACT proof: the dispatched notification is now READ in the BE (read_at non-null).
  await gotoReady(page, '/notifications');
  await expect
    .poll(
      async () => {
        const res = await apiAs(
          page,
          'GET',
          `/notifications?read_state=UNREAD&limit=50`,
        );
        const rows = (res.body as { data?: Array<{ deep_link?: { entity_id?: string } }> }).data ?? [];
        return rows.some((n) => n.deep_link?.entity_id === LR.dewiPendingHr);
      },
      { timeout: 15_000 },
    )
    .toBe(false);
});
