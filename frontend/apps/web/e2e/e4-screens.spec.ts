import { mkdirSync } from 'node:fs';
import { expect, test } from '@playwright/test';

/** E4 visual verification — schedule grid + shift master catalog. Asserts zero page errors. */
const DIR = 'e2e/__screenshots__/e4';
mkdirSync(DIR, { recursive: true });

test('E4 Jadwal — schedule grid + shift masters', async ({ page }) => {
  const errors: string[] = [];
  page.on('pageerror', (e) => errors.push(e.message));

  await page.setViewportSize({ width: 1440, height: 1024 });

  await page.goto('/login');
  await page.getByLabel('Email').fill('sari.hadi@swp.example.com');
  await page.getByLabel('Kata Sandi').fill('password');
  await page.getByRole('button', { name: 'Masuk' }).click();
  await expect(page).toHaveURL('/');

  await page.getByRole('link', { name: 'Jadwal' }).first().click();
  await expect(page).toHaveURL(/\/schedule$/);
  await page.waitForTimeout(1200);
  await page.screenshot({ path: `${DIR}/01-schedule-grid.png` });

  await page.getByRole('link', { name: 'Master Shift' }).first().click();
  await expect(page).toHaveURL(/\/shifts$/);
  await page.waitForTimeout(1500);
  await page.screenshot({ path: `${DIR}/02-shift-masters.png` });

  expect(errors, `page errors:\n${errors.join('\n')}`).toEqual([]);
});
