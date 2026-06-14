import { expect, test } from '@playwright/test';

/** F1.1 — Successful login reaches the dashboard. (authentication.md AC: "Successful login") */
test('unauthenticated user is sent to login, then reaches the dashboard', async ({ page }) => {
  await page.goto('/');
  await expect(page).toHaveURL(/\/login/);

  await page.getByLabel('Email').fill('sari.hadi@swp.example.com');
  await page.getByLabel('Kata Sandi').fill('password');
  await page.getByRole('button', { name: 'Masuk' }).click();

  await expect(page).toHaveURL('/');
  await expect(page.getByRole('heading', { name: 'Dasbor' })).toBeVisible();
});
