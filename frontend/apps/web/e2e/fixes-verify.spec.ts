import { mkdirSync } from 'node:fs';
import { expect, test } from '@playwright/test';

/** Verifies the i18n fallback fix (corrections rendered raw keys) + native-control theming. */
const DIR = 'e2e/__screenshots__/fixes';
mkdirSync(DIR, { recursive: true });

test('corrections renders real copy (not raw i18n keys) + dropdowns are themed', async ({
  page,
}) => {
  const errors: string[] = [];
  page.on('pageerror', (e) => errors.push(e.message));
  await page.setViewportSize({ width: 1440, height: 1024 });

  await page.goto('/login');
  await page.getByLabel('Email').fill('sari.hadi@swp.example.com');
  await page.getByLabel('Kata Sandi').fill('password');
  await page.getByRole('button', { name: 'Masuk' }).click();
  await expect(page).toHaveURL('/');

  // Kehadiran → Koreksi (section sub-nav) — the screen that showed raw keys.
  await page.getByRole('link', { name: 'Kehadiran', exact: true }).first().click();
  await expect(page).toHaveURL('/attendance');
  await page.waitForTimeout(800);
  await page.getByRole('link', { name: 'Koreksi', exact: true }).first().click();
  await expect(page).toHaveURL('/corrections');
  await page.waitForTimeout(1200);

  // No raw i18n key should remain on screen.
  const body = await page.locator('body').innerText();
  const rawKeys = body.match(/\bcorrections\.[a-zA-Z.]+/g) ?? [];
  expect(rawKeys, `raw i18n keys still on screen: ${rawKeys.join(', ')}`).toEqual([]);

  await page.screenshot({ path: `${DIR}/01-corrections.png`, fullPage: true });

  // A dropdown control should be themed (solid surface, not transparent).
  await page.getByText('Koreksi Kehadiran').first().waitFor();
  await page.screenshot({ path: `${DIR}/02-corrections-filters.png` });

  expect(errors, `page errors:\n${errors.join('\n')}`).toEqual([]);
});
