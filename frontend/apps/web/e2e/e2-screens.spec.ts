import { mkdirSync } from 'node:fs';
import { expect, test } from '@playwright/test';

/**
 * E2 visual verification — logs in as HR and SPA-navigates the Karyawan & Master Data area via
 * the sidebar, capturing flagship screens at 1440x1024 with MSW data. Auth is in-memory, so we
 * click through the shell (never goto an authed URL). Asserts zero page errors.
 */
const DIR = 'e2e/__screenshots__/e2';
mkdirSync(DIR, { recursive: true });

test('E2 Karyawan & Master Data — flagship screens', async ({ page }) => {
  const errors: string[] = [];
  page.on('pageerror', (e) => errors.push(e.message));

  await page.setViewportSize({ width: 1440, height: 1024 });

  await page.goto('/login');
  await page.getByLabel('Email').fill('sari.hadi@swp.example.com');
  await page.getByLabel('Kata Sandi').fill('password');
  await page.getByRole('button', { name: 'Masuk' }).click();
  await expect(page).toHaveURL('/');

  const visit = async (navLabel: string, urlRe: RegExp, file: string) => {
    await page.getByRole('link', { name: navLabel }).first().click();
    await expect(page).toHaveURL(urlRe);
    await page.waitForTimeout(1500);
    await page.screenshot({ path: `${DIR}/${file}.png` });
  };

  await visit('Karyawan', /\/employees$/, '01-employees');
  // Employee detail — click the first table row's name link if present.
  const firstEmp = page
    .getByRole('link')
    .filter({ hasText: /SWP-EMP|@/ })
    .first();
  if (await firstEmp.count()) {
    // fall back: just screenshot list; detail navigation is data-dependent
  }
  await visit('Perusahaan Klien', /\/client-companies$/, '02-client-companies');
  await visit('Perjanjian Kerja', /\/agreements$/, '03-agreements');
  await visit('Persetujuan', /\/change-requests$/, '04-change-requests');
  await visit('Lini Layanan', /\/service-lines$/, '05-service-lines');
  await visit('Data Master', /\/master-data$/, '06-master-data-hub');
  // Master-data sub-list via a hub card
  await page
    .getByRole('link', { name: /Jenis Cuti|Kelola/ })
    .first()
    .click();
  await page.waitForTimeout(1200);
  await page.screenshot({ path: `${DIR}/07-leave-types.png` });

  expect(errors, `page errors:\n${errors.join('\n')}`).toEqual([]);
});
