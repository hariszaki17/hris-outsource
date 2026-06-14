import { mkdirSync } from 'node:fs';
import { expect, test } from '@playwright/test';

/**
 * E1 visual verification — boots the app (MSW on), logs in as HR, and SPA-navigates the admin
 * console capturing flagship screens + overlays at 1440x1024. Auth is in-memory (wiped on reload),
 * so we click through the shell — never goto() an authed URL. Asserts zero page errors.
 */
const DIR = 'e2e/__screenshots__/e1';
mkdirSync(DIR, { recursive: true });

test('E1 admin console — flagship screens + overlays', async ({ page }) => {
  const errors: string[] = [];
  page.on('pageerror', (e) => errors.push(e.message));

  await page.setViewportSize({ width: 1440, height: 1024 });

  // Login (stubbed → hr_admin)
  await page.goto('/login');
  await page.getByLabel('Email').fill('sari.hadi@swp.example.com');
  await page.getByLabel('Kata Sandi').fill('password');
  await page.getByRole('button', { name: 'Masuk' }).click();
  await expect(page).toHaveURL('/');
  await page.screenshot({ path: `${DIR}/00-dashboard.png` });

  // Settings overview (sidebar → Pengaturan)
  await page.getByRole('link', { name: 'Pengaturan' }).first().click();
  await expect(page).toHaveURL('/settings');
  await expect(page.getByRole('heading', { name: 'Pengaturan' })).toBeVisible();
  await page.waitForTimeout(900);
  await page.screenshot({ path: `${DIR}/01-settings-overview.png` });

  // Users list
  await page.getByRole('link', { name: 'Pengguna & Peran' }).first().click();
  await expect(page).toHaveURL('/settings/users');
  await page.waitForTimeout(1500);
  await page.screenshot({ path: `${DIR}/02-users-list.png` });

  // Create-user modal
  await page.getByRole('button', { name: 'Tambah Pengguna' }).click();
  await page.waitForTimeout(900);
  await page.screenshot({ path: `${DIR}/03-users-create-modal.png` });
  await page.keyboard.press('Escape');
  await page.waitForTimeout(200);

  // Audit log list
  await page.getByRole('link', { name: 'Audit Log' }).first().click();
  await expect(page).toHaveURL('/settings/audit-log');
  await page.waitForTimeout(1500);
  await page.screenshot({ path: `${DIR}/04-audit-log.png` });

  // Settings general
  await page.getByRole('link', { name: 'Umum' }).first().click();
  await expect(page).toHaveURL('/settings/general');
  await page.waitForTimeout(1500);
  await page.screenshot({ path: `${DIR}/05-settings-general.png` });

  expect(errors, `page errors:\n${errors.join('\n')}`).toEqual([]);
});
