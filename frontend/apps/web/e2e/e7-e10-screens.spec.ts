import { mkdirSync } from 'node:fs';
import { expect, test } from '@playwright/test';

/**
 * Consolidated visual verification for E6 Cuti, E7 Lembur, E8 Payroll, E10 Reporting/Notifikasi.
 * Boots the app (MSW on), logs in as HR, and SPA-navigates via the sidebar (8 design modules) +
 * the section sub-nav (DESIGN-SYSTEM IA — payroll under Laporan, notifications = topbar bell).
 * Auth is in-memory — never goto() an authed URL. Captures at 1440x1024; asserts zero page errors.
 */
const DIR = 'e2e/__screenshots__/e7-e10';
mkdirSync(DIR, { recursive: true });

test('E6–E10 — flagship screens via the 8-item sidebar + section sub-nav', async ({ page }) => {
  const errors: string[] = [];
  page.on('pageerror', (e) => errors.push(e.message));

  await page.setViewportSize({ width: 1440, height: 1024 });

  // Login (stubbed → hr_admin) → lands on the E10 dashboard at /
  await page.goto('/login');
  await page.getByLabel('Email').fill('sari.hadi@swp.example.com');
  await page.getByLabel('Kata Sandi').fill('password');
  await page.getByRole('button', { name: 'Masuk' }).click();
  await expect(page).toHaveURL('/');
  await page.waitForTimeout(1200);
  await page.screenshot({ path: `${DIR}/00-dashboard.png`, fullPage: true });

  // Sidebar must hold exactly 8 primary modules (the design contract).
  const menu = page.locator('nav').first();
  await expect(menu.getByText('Payroll')).toHaveCount(0); // payroll is NOT a sidebar item
  await expect(menu.getByText('Notifikasi')).toHaveCount(0); // notifications = topbar bell

  // E6 Cuti (sidebar)
  await page.getByRole('link', { name: 'Cuti', exact: true }).first().click();
  await expect(page).toHaveURL('/leave');
  await page.waitForTimeout(1400);
  await page.screenshot({ path: `${DIR}/01-leave-approvals.png`, fullPage: true });

  // E7 Lembur (sidebar) — section sub-nav shows Persetujuan/Rekap/Aturan & Libur
  await page.getByRole('link', { name: 'Lembur', exact: true }).first().click();
  await expect(page).toHaveURL('/overtime');
  await page.waitForTimeout(1400);
  await page.screenshot({ path: `${DIR}/02-overtime-approvals.png`, fullPage: true });

  // E10 Laporan — billable report (sidebar)
  await page.getByRole('link', { name: 'Laporan', exact: true }).first().click();
  await expect(page).toHaveURL('/reports');
  await page.waitForTimeout(1400);
  await page.screenshot({ path: `${DIR}/04-billable-report.png`, fullPage: true });

  // E8 Payroll — reached via the Laporan section sub-nav (not the sidebar)
  await page.getByRole('link', { name: 'Payroll', exact: true }).first().click();
  await expect(page).toHaveURL('/payroll');
  await page.waitForTimeout(1400);
  await page.screenshot({ path: `${DIR}/03-payroll-archive.png`, fullPage: true });

  // E10 Notifikasi — topbar bell
  await page.getByRole('button', { name: 'Notifikasi' }).click();
  await expect(page).toHaveURL('/notifications');
  await page.waitForTimeout(1400);
  await page.screenshot({ path: `${DIR}/05-notifications.png`, fullPage: true });

  expect(errors, `page errors:\n${errors.join('\n')}`).toEqual([]);
});
