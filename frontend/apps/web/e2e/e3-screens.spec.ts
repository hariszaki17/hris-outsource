import { mkdirSync } from 'node:fs';
import { expect, test } from '@playwright/test';

/**
 * E3 visual verification — logs in as HR and SPA-navigates Penempatan, capturing the placement
 * list + create form at 1440x1024 with MSW data. Asserts zero page errors.
 */
const DIR = 'e2e/__screenshots__/e3';
mkdirSync(DIR, { recursive: true });

test('E3 Penempatan — list + create', async ({ page }) => {
  const errors: string[] = [];
  page.on('pageerror', (e) => errors.push(e.message));

  await page.setViewportSize({ width: 1440, height: 1024 });

  await page.goto('/login');
  await page.getByLabel('Email').fill('sari.hadi@swp.example.com');
  await page.getByLabel('Kata Sandi').fill('password');
  await page.getByRole('button', { name: 'Masuk' }).click();
  await expect(page).toHaveURL('/');

  await page.getByRole('link', { name: 'Penempatan' }).first().click();
  await expect(page).toHaveURL(/\/placements$/);
  await page.waitForTimeout(1500);
  await page.screenshot({ path: `${DIR}/01-placements.png` });

  // Create placement form
  await page.getByRole('link', { name: /Buat Penempatan/ }).first().click();
  await expect(page).toHaveURL(/\/placements\/new$/);
  await page.waitForTimeout(800);
  await page.screenshot({ path: `${DIR}/02-create-placement.png` });

  expect(errors, `page errors:\n${errors.join('\n')}`).toEqual([]);
});
