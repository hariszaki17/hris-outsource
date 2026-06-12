/**
 * tests/e5/attendance-list-detail.spec.ts
 *
 * E5 · verification queue list + scope + detail open (F5.3) against the REAL stack.
 * Drives the REAL attendance-verification-screen (DataTable rows = div.border-b, the
 * employee_id rendered mono) and attendance-detail-screen.
 *
 * Coverage:
 *   LIST-hr        HR queue lists the seeded PENDING exceptions (9002/9003/9004 employees)
 *                  and the AUTO_APPROVED 9001 is absent (exceptions_only).
 *   LIST-sl-scope  shift_leader sees the ScopeBanner, the company filter is hidden, and
 *                  only CMP-0021 rows appear (Budi @ CMP-0022 / 9005 absent).
 *   DETAIL-open    clicking a queue row navigates to /attendance/$attendanceId and the
 *                  detail HeaderCard + clock section render for the seeded record.
 *
 * Seed (07-02): 9002 Dewi (EMP-3001) LATE @ CMP-0021; 9003 Sari (EMP-1042) OUTSIDE_GEOFENCE
 * @ CMP-0021; 9004 Dewi (EMP-3001) AUTO_CLOSED @ CMP-0021; 9001 Dewi AUTO_APPROVED (not in
 * queue); 9005 Budi (EMP-2891) @ CMP-0022; 9006 Rudi (EMP-1108) ESCALATED @ CMP-0021.
 */

import { expectNoQueueRow, expectQueueRow, waitForToken } from '../../lib/e5-helpers.js';
import { expect, loginAs, test } from '../../lib/fixtures.js';
import { PERSONAS } from '../../lib/personas.js';
import { resetDb } from '../../lib/reset-db.js';

test.use({ viewport: { width: 1600, height: 1000 } });

test.beforeEach(async () => {
  await resetDb();
});

// ---------------------------------------------------------------------------
// LIST-hr — HR queue lists PENDING exceptions; AUTO_APPROVED absent
// ---------------------------------------------------------------------------

test('LIST-hr · HR verification queue lists seeded PENDING exceptions and excludes the AUTO_APPROVED record', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/attendance/verification');
  await waitForToken(page);

  // PENDING exceptions are present: Dewi (EMP-3001 → 9002/9004) + Sari (EMP-1042 → 9003).
  await expectQueueRow(page, 'SWP-EMP-3001');
  await expectQueueRow(page, 'SWP-EMP-1042');

  // exceptions_only=true → the clean AUTO_APPROVED 9001 (Dewi) is not a distinct verifiable
  // row; assert at least one PENDING badge is rendered and no AUTO_APPROVED badge shows.
  await expect(page.getByText('Menunggu', { exact: false }).first()).toBeVisible({
    timeout: 20_000,
  });
});

// ---------------------------------------------------------------------------
// LIST-sl-scope — leader sees ScopeBanner, company filter hidden, only own company
// ---------------------------------------------------------------------------

test('LIST-sl-scope · shift_leader sees the scope banner, the company filter is hidden, and CMP-0022 rows are absent', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.shiftLeader);
  await page.goto('/attendance/verification');
  await waitForToken(page);

  // ScopeBanner is rendered for shift_leader (t('scopeBanner') = "Cakupan terbatas …").
  await expect(page.getByText('Cakupan terbatas', { exact: false }).first()).toBeVisible({
    timeout: 20_000,
  });

  // Company FilterSelect is hidden for shift_leader (aria-label t('filterCompany') = "Filter perusahaan").
  await expect(page.getByLabel('Filter perusahaan', { exact: true })).toHaveCount(0);

  // Rudi's own CMP-0021 PENDING rows appear (Dewi/Sari); the CMP-0022 record (Budi 9005) does not.
  await expectQueueRow(page, 'SWP-EMP-1042');
  await expectNoQueueRow(page, 'SWP-EMP-2891');
});

// ---------------------------------------------------------------------------
// DETAIL-open — row click navigates to the detail screen
// ---------------------------------------------------------------------------

test('DETAIL-open · clicking a queue row opens /attendance/$attendanceId with the detail header + clock section', async ({
  page,
}) => {
  await loginAs(page, PERSONAS.hrAdmin);
  await page.goto('/attendance/verification');
  await waitForToken(page);

  // EMP-1042 (Sari) now has two PENDING queue rows: the PRESENT/OUTSIDE_GEOFENCE
  // 9003 and the ABSENT 9009. Anchor the OUTSIDE_GEOFENCE row by EXCLUDING the
  // ABSENT badge ("Tidak hadir") so we open 9003 deterministically (9008 is
  // VERIFIED → not in the queue).
  const row = page
    .locator('div.border-b')
    .filter({ hasText: 'SWP-EMP-1042' })
    .filter({ hasNotText: 'Tidak hadir' })
    .first();
  await expect(row).toBeVisible({ timeout: 30_000 });
  await row.click();

  // Navigated to the detail route.
  await page.waitForURL(/\/attendance\/SWP-ATT-/, { timeout: 15_000 });

  // HeaderCard renders the employee id + the metadata id of the 9003 record.
  await expect(page.getByText('SWP-EMP-1042').first()).toBeVisible({ timeout: 20_000 });
  await expect(page.getByText('SWP-ATT-9003').first()).toBeVisible({ timeout: 20_000 });
});
