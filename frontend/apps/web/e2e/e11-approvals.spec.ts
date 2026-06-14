import { mkdirSync } from 'node:fs';
import { type Page, expect, test } from '@playwright/test';

/**
 * E11 · Approvals — end-to-end coverage driving the REAL stateful MSW layer
 * (packages/api-client/src/e11-stateful-mocks.ts). No handler overrides from the test:
 * every assertion observes the mock's true transitions (approve→advance→APPROVED,
 * reject→REJECTED, super-admin bypass→APPROVED+BYPASS trail, template upsert validation
 * + version bump + pending reset).
 *
 * Determinism model (from the mock contract):
 *   - Each `page.goto(...)` reloads the bundle → the store RE-SEEDS (per-test isolation).
 *   - Within ONE test, SPA navigation keeps store state (multi-step flows persist).
 *   - A re-login resets the store. So each test runs as a SINGLE user to avoid reseeds.
 *
 * Login → role by email (mock contract):
 *   superadmin@swp.test→super_admin · hradmin@swp.test→hr_admin
 *   leader@swp.test→shift_leader · agent@swp.test→agent
 */

const DIR = 'e2e/__screenshots__/e11';
mkdirSync(DIR, { recursive: true });

const EMAIL = {
  super: 'superadmin@swp.test',
  hr: 'hradmin@swp.test',
  leader: 'leader@swp.test',
  agent: 'agent@swp.test',
} as const;

/**
 * Log in (selecting the mock role by email) and land on the post-login home.
 * Staff (super/hr/leader) → '/'; agent → '/me'. We then SPA/goto to the target.
 */
async function login(page: Page, email: string): Promise<void> {
  await page.goto('/login');
  await page.getByLabel('Email').fill(email);
  await page.getByLabel('Kata Sandi').fill('password');
  await page.getByRole('button', { name: 'Masuk' }).click();
  // Staff land on '/', agent on '/me'. Wait until we leave the login page.
  await expect(page).not.toHaveURL(/\/login/);
}

/**
 * SPA navigation (no reload). A full `page.goto` re-seeds the stateful MSW store AND drops
 * the in-memory session (so the mock's currentUser → null, losing the logged-in role). After
 * login we must navigate via the router so the session + store state persist for the test.
 * Uses the router exposed on window (main.tsx, MSW-flag-guarded).
 */
async function nav(page: Page, to: string): Promise<void> {
  await page.waitForFunction(() => Boolean((window as { __router?: unknown }).__router));
  await page.evaluate(
    (target) =>
      (window as unknown as { __router: { navigate: (o: { to: string }) => Promise<void> } }).__router.navigate(
        { to: target },
      ),
    to,
  );
  await expect(page).toHaveURL(new RegExp(to.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
}

/**
 * Add a member to template line `lineNo` via the inline EmployeePicker (Combobox):
 *   "Tambah anggota" → reveals the combobox trigger → click trigger to open the popover →
 *   type the query in the search input → click the matching option.
 */
async function addMember(
  page: Page,
  lineNo: number,
  query: string,
  fullName: string,
): Promise<void> {
  const card = page.getByTestId(`line-card-${lineNo}`);
  await card.getByTestId(`line-add-member-${lineNo}`).click();
  const picker = card.getByTestId(`line-picker-${lineNo}`);
  // The combobox renders a trigger button first; click it to open the search popover.
  await picker.getByRole('button').first().click();
  await picker.locator('input').fill(query);
  await page.getByRole('button', { name: new RegExp(fullName) }).click();
}

// ---------------------------------------------------------------------------
// TEMPLATE EDITOR (hr_admin) — F11.1
// ---------------------------------------------------------------------------

test.describe('E11 · Template Persetujuan (hr_admin)', () => {
  test('1 · loads SWP-CMP-001 → 2 seeded line cards', async ({ page }) => {
    await login(page, EMAIL.hr);
    await nav(page, '/client-companies/SWP-CMP-001/approval-template');

    await expect(page.getByTestId('line-card-1')).toBeVisible();
    await expect(page.getByTestId('line-card-2')).toBeVisible();
    await expect(page.getByTestId('line-card-3')).toHaveCount(0);

    // line1 = leader OR hr; line2 = hr (seeded).
    await expect(
      page.getByTestId('line-card-1').getByTestId('member-chip-SWP-USR-LEADER'),
    ).toBeVisible();
    await expect(
      page.getByTestId('line-card-1').getByTestId('member-chip-SWP-USR-HR'),
    ).toBeVisible();
    await expect(
      page.getByTestId('line-card-2').getByTestId('member-chip-SWP-USR-HR'),
    ).toBeVisible();

    await page.screenshot({ path: `${DIR}/01-template-editor.png`, fullPage: true });
  });

  test('2 · search + add distinct members to line 1 (OR-set)', async ({ page }) => {
    await login(page, EMAIL.hr);
    await nav(page, '/client-companies/SWP-CMP-001/approval-template');

    const line1 = page.getByTestId('line-card-1');
    // Add a distinct member (Dewi Lestari → SWP-USR-LEAD).
    await addMember(page, 1, 'Dewi', 'Dewi Lestari');
    await expect(line1.getByTestId('member-chip-SWP-USR-LEAD')).toBeVisible();

    // Add a second distinct member (Citra Putri → SWP-USR-HR2).
    await addMember(page, 1, 'Citra', 'Citra Putri');
    await expect(line1.getByTestId('member-chip-SWP-USR-HR2')).toBeVisible();

    // Original seeded members still present → an OR-set of 4.
    await expect(line1.getByTestId('member-chip-SWP-USR-LEADER')).toBeVisible();
    await expect(line1.getByTestId('member-chip-SWP-USR-HR')).toBeVisible();
  });

  test('3 · remove a member chip', async ({ page }) => {
    await login(page, EMAIL.hr);
    await nav(page, '/client-companies/SWP-CMP-001/approval-template');

    const line1 = page.getByTestId('line-card-1');
    await expect(line1.getByTestId('member-chip-SWP-USR-LEADER')).toBeVisible();
    await line1.getByTestId('member-remove-SWP-USR-LEADER').click();
    await expect(line1.getByTestId('member-chip-SWP-USR-LEADER')).toHaveCount(0);
    // The other member remains.
    await expect(line1.getByTestId('member-chip-SWP-USR-HR')).toBeVisible();
  });

  test('4 · add line 3 + a member, then remove line 3', async ({ page }) => {
    await login(page, EMAIL.hr);
    await nav(page, '/client-companies/SWP-CMP-001/approval-template');

    await expect(page.getByTestId('line-card-3')).toHaveCount(0);
    await page.getByTestId('template-add-line').click();
    await expect(page.getByTestId('line-card-3')).toBeVisible();

    const line3 = page.getByTestId('line-card-3');
    await addMember(page, 3, 'Super', 'Super Admin');
    await expect(line3.getByTestId('member-chip-SWP-USR-SUPER')).toBeVisible();

    // Remove line 3 → back to 2 lines.
    await line3.getByTestId('line-remove-3').click();
    await expect(page.getByTestId('line-card-3')).toHaveCount(0);
    await expect(page.getByTestId('line-card-2')).toBeVisible();
  });

  test('5 · min-2 validation: emptying a line blocks Save', async ({ page }) => {
    await login(page, EMAIL.hr);
    await nav(page, '/client-companies/SWP-CMP-001/approval-template');

    // Empty line 2 (it has a single member: HR).
    const line2 = page.getByTestId('line-card-2');
    await line2.getByTestId('member-remove-SWP-USR-HR').click();
    await expect(line2.getByTestId('member-chip-SWP-USR-HR')).toHaveCount(0);

    // Save is disabled (client gate: every line needs ≥1 member) + block hint shown.
    await expect(page.getByTestId('template-save')).toBeDisabled();
    await expect(page.getByText('Setiap baris perlu minimal 1 anggota aktif.')).toBeVisible();
  });

  test('6 · valid save → reset-pending confirm → success toast', async ({ page }) => {
    await login(page, EMAIL.hr);
    await nav(page, '/client-companies/SWP-CMP-001/approval-template');

    // Make a real edit: add a member to line 2 so the payload differs but stays valid.
    const line2 = page.getByTestId('line-card-2');
    await addMember(page, 2, 'Dewi', 'Dewi Lestari');
    await expect(line2.getByTestId('member-chip-SWP-USR-LEAD')).toBeVisible();

    await expect(page.getByTestId('template-save')).toBeEnabled();
    await page.getByTestId('template-save').click();

    // Reset-pending confirm modal (uoTwN).
    const dialog = page.getByRole('dialog');
    await expect(dialog.getByText('Simpan & reset permintaan menunggu?')).toBeVisible();
    await dialog.getByRole('button', { name: 'Simpan & reset' }).click();

    // Success toast (store bumps version + resets pending → real mutation succeeded).
    await expect(page.getByText('Template tersimpan')).toBeVisible();
  });

  test('7 · SWP-CMP-999 (no template) → create/fallback state, not an error', async ({ page }) => {
    await login(page, EMAIL.hr);
    await nav(page, '/client-companies/SWP-CMP-999/approval-template');

    // 404 → fallback note + empty 2-line skeleton (not an error surface).
    await expect(page.getByTestId('template-fallback')).toBeVisible();
    await expect(page.getByTestId('line-card-1')).toBeVisible();
    await expect(page.getByTestId('line-card-2')).toBeVisible();
    // No template loaded → no delete affordance.
    await expect(page.getByTestId('template-delete')).toHaveCount(0);

    await page.screenshot({ path: `${DIR}/07-template-fallback.png`, fullPage: true });
  });

  test('8 · delete template → confirm → reverts to fallback', async ({ page }) => {
    await login(page, EMAIL.hr);
    await nav(page, '/client-companies/SWP-CMP-001/approval-template');

    await expect(page.getByTestId('template-delete')).toBeVisible();
    await page.getByTestId('template-delete').click();

    const dialog = page.getByRole('dialog');
    await expect(dialog.getByText('Hapus template?')).toBeVisible();
    await dialog.getByRole('button', { name: 'Hapus', exact: true }).click();

    await expect(page.getByText('Template dihapus')).toBeVisible();
    // After deletion + refetch the screen falls back (no template → fallback note).
    await expect(page.getByTestId('template-fallback')).toBeVisible();
  });
});

// ---------------------------------------------------------------------------
// INBOX (shift_leader) — F11.3
// ---------------------------------------------------------------------------

test.describe('E11 · Kotak Masuk (shift_leader)', () => {
  test('9 · lists PEND1 (Baris 1/2); SWP-APV-SELF hidden (requester==leader)', async ({ page }) => {
    await login(page, EMAIL.leader);
    await nav(page, '/inbox');

    await expect(page.getByTestId('instance-row-SWP-APV-PEND1')).toBeVisible();
    await expect(page.getByText('Baris 1/2').first()).toBeVisible();
    // SELF is requested by the leader → excluded from their own inbox (INV-3).
    await expect(page.getByTestId('instance-row-SWP-APV-SELF')).toHaveCount(0);
    // Leader is on C1 line1, so REJ + BYP also appear; MID2 (C2 line2) does not.
    await expect(page.getByTestId('instance-row-SWP-APV-REJ')).toBeVisible();
    await expect(page.getByTestId('instance-row-SWP-APV-MID2')).toHaveCount(0);

    await page.screenshot({ path: `${DIR}/09-inbox.png`, fullPage: true });
  });

  test('10 · approve PEND1 from a row → advances to line2 → drops from inbox', async ({ page }) => {
    await login(page, EMAIL.leader);
    await nav(page, '/inbox');

    await expect(page.getByTestId('instance-row-SWP-APV-PEND1')).toBeVisible();
    await page.getByTestId('inbox-approve-SWP-APV-PEND1').click();
    await expect(page.getByText('Permintaan disetujui')).toBeVisible();

    // Now on line2 ([HR]) — leader is not a member → row leaves the leader's inbox.
    await expect(page.getByTestId('instance-row-SWP-APV-PEND1')).toHaveCount(0);
  });

  test('11 · reject SWP-APV-REJ from a row → reason modal → leaves inbox', async ({ page }) => {
    await login(page, EMAIL.leader);
    await nav(page, '/inbox');

    await expect(page.getByTestId('instance-row-SWP-APV-REJ')).toBeVisible();
    await page.getByTestId('inbox-reject-SWP-APV-REJ').click();

    // Reason required (≥5 chars), then confirm.
    await page.getByTestId('reject-reason-input').fill('Jadwal tidak memungkinkan.');
    await page.getByTestId('reject-confirm').click();

    await expect(page.getByText('Permintaan ditolak')).toBeVisible();
    // REJECTED is terminal → drops from the PENDING inbox.
    await expect(page.getByTestId('instance-row-SWP-APV-REJ')).toHaveCount(0);
  });

  test('12 · type tabs filter Lembur/Cuti', async ({ page }) => {
    await login(page, EMAIL.leader);
    await nav(page, '/inbox');

    // Leader pending: PEND1(Cuti), REJ(Cuti), BYP(Lembur).
    await expect(page.getByTestId('instance-row-SWP-APV-BYP')).toBeVisible();

    // Filter to Lembur → only BYP remains.
    await page.getByRole('tab', { name: 'Lembur' }).click();
    await expect(page.getByTestId('instance-row-SWP-APV-BYP')).toBeVisible();
    await expect(page.getByTestId('instance-row-SWP-APV-PEND1')).toHaveCount(0);
    await expect(page.getByTestId('instance-row-SWP-APV-REJ')).toHaveCount(0);

    // Filter to Cuti → PEND1 + REJ, no BYP.
    await page.getByRole('tab', { name: 'Cuti' }).click();
    await expect(page.getByTestId('instance-row-SWP-APV-PEND1')).toBeVisible();
    await expect(page.getByTestId('instance-row-SWP-APV-REJ')).toBeVisible();
    await expect(page.getByTestId('instance-row-SWP-APV-BYP')).toHaveCount(0);
  });
});

// ---------------------------------------------------------------------------
// DETAIL — F11.2 / F11.3
// ---------------------------------------------------------------------------

test.describe('E11 · Detail Permintaan', () => {
  test('13 · full-approve (hr_admin, HR in both lines) → line2 → APPROVED', async ({ page }) => {
    await login(page, EMAIL.hr);
    await nav(page, '/approval-instances/SWP-APV-PEND1');

    // Starts PENDING on line1.
    await expect(page.getByTestId('detail-status')).toHaveAttribute('data-status', 'PENDING');
    await expect(page.getByTestId('chain-line-1')).toHaveAttribute('data-state', 'current');
    await expect(page.getByTestId('chain-line-2')).toHaveAttribute('data-state', 'upcoming');

    // First approve → line1 cleared (done), line2 current.
    await page.getByTestId('detail-approve').click();
    await expect(page.getByText('Permintaan disetujui')).toBeVisible();
    await expect(page.getByTestId('chain-line-1')).toHaveAttribute('data-state', 'done');
    await expect(page.getByTestId('chain-line-2')).toHaveAttribute('data-state', 'current');
    await expect(page.getByTestId('detail-status')).toHaveAttribute('data-status', 'PENDING');

    // Second approve (last line) → APPROVED terminal.
    await page.getByTestId('detail-approve').click();
    await expect(page.getByTestId('detail-status')).toHaveAttribute('data-status', 'APPROVED');
    await expect(page.getByTestId('terminal-banner')).toBeVisible();
    await expect(page.getByTestId('detail-approve')).toHaveCount(0);
    await expect(page.getByTestId('detail-reject')).toHaveCount(0);

    await page.screenshot({ path: `${DIR}/13-detail-approved.png`, fullPage: true });
  });

  test('14 · reject (leader) → reason → REJECTED', async ({ page }) => {
    await login(page, EMAIL.leader);
    await nav(page, '/approval-instances/SWP-APV-REJ');

    await expect(page.getByTestId('detail-status')).toHaveAttribute('data-status', 'PENDING');
    await page.getByTestId('detail-reject').click();
    await page.getByTestId('reject-reason-input').fill('Tidak disetujui oleh atasan.');
    await page.getByTestId('reject-confirm').click();

    await expect(page.getByTestId('detail-status')).toHaveAttribute('data-status', 'REJECTED');
    await expect(page.getByTestId('terminal-banner')).toBeVisible();
    await expect(page.getByTestId('detail-approve')).toHaveCount(0);

    await page.screenshot({ path: `${DIR}/14-detail-rejected.png`, fullPage: true });
  });

  test('15 · bypass (super_admin) → reason → APPROVED + BYPASS trail; non-super has no affordance', async ({
    page,
  }) => {
    // First confirm a non-super (leader) does NOT see the bypass affordance.
    await login(page, EMAIL.leader);
    await nav(page, '/approval-instances/SWP-APV-BYP');
    await expect(page.getByTestId('detail-status')).toHaveAttribute('data-status', 'PENDING');
    await expect(page.getByTestId('bypass-card')).toHaveCount(0);

    // Re-login as super admin (this resets the store → BYP is PENDING again).
    await login(page, EMAIL.super);
    await nav(page, '/approval-instances/SWP-APV-BYP');

    await expect(page.getByTestId('bypass-card')).toBeVisible();
    await page.getByTestId('detail-bypass').click();

    // Reason required (≥10 chars) → confirm.
    await page.getByTestId('bypass-reason-input').fill('Eskalasi mendesak dari klien utama.');
    await page.getByTestId('bypass-confirm').click();

    await expect(page.getByTestId('detail-status')).toHaveAttribute('data-status', 'APPROVED');
    await expect(page.getByTestId('terminal-banner')).toBeVisible();
    // BYPASS entry recorded in the action trail.
    await expect(page.getByText(/melewati persetujuan \(bypass\)/i)).toBeVisible();

    await page.screenshot({ path: `${DIR}/15-detail-bypassed.png`, fullPage: true });
  });

  test('16 · self-approval blocked (leader on SWP-APV-SELF)', async ({ page }) => {
    await login(page, EMAIL.leader);
    await nav(page, '/approval-instances/SWP-APV-SELF');

    // Requester == sole line1 member == current user → UI disables Approve (defense-in-depth,
    // INV-3) and shows the self-approval notice. Status stays PENDING.
    await expect(page.getByTestId('detail-status')).toHaveAttribute('data-status', 'PENDING');
    await expect(page.getByTestId('detail-approve')).toBeDisabled();
    await expect(
      page.getByText('Anda tidak dapat menyetujui permintaan Anda sendiri.'),
    ).toBeVisible();
    // Still PENDING (no transition happened).
    await expect(page.getByTestId('detail-status')).toHaveAttribute('data-status', 'PENDING');
  });

  test('17 · terminal render (APPROVED + REJECTED) → no action buttons', async ({ page }) => {
    await login(page, EMAIL.hr);

    await nav(page, '/approval-instances/SWP-APV-DONE');
    await expect(page.getByTestId('detail-status')).toHaveAttribute('data-status', 'APPROVED');
    await expect(page.getByTestId('terminal-banner')).toBeVisible();
    await expect(page.getByTestId('detail-approve')).toHaveCount(0);
    await expect(page.getByTestId('detail-reject')).toHaveCount(0);

    await nav(page, '/approval-instances/SWP-APV-REJD');
    await expect(page.getByTestId('detail-status')).toHaveAttribute('data-status', 'REJECTED');
    await expect(page.getByTestId('terminal-banner')).toBeVisible();
    await expect(page.getByTestId('detail-approve')).toHaveCount(0);
    await expect(page.getByTestId('detail-reject')).toHaveCount(0);
  });

  test('18 · chain timeline (SWP-APV-MID2): line1 done, line2 current, line3 upcoming', async ({
    page,
  }) => {
    await login(page, EMAIL.hr);
    await nav(page, '/approval-instances/SWP-APV-MID2');

    await expect(page.getByTestId('chain-line-1')).toHaveAttribute('data-state', 'done');
    await expect(page.getByTestId('chain-line-2')).toHaveAttribute('data-state', 'current');
    await expect(page.getByTestId('chain-line-3')).toHaveAttribute('data-state', 'upcoming');

    await page.screenshot({ path: `${DIR}/18-detail-chain.png`, fullPage: true });
  });
});

// ---------------------------------------------------------------------------
// E6 → E11 WIRING
// ---------------------------------------------------------------------------

test.describe('E11 · E6 leave wiring', () => {
  test('19 · leave detail for SWP-LR-PEND1 renders the approval chain + link', async ({ page }) => {
    await login(page, EMAIL.hr);
    await nav(page, '/leave/SWP-LR-PEND1');

    // The leave detail reuses the E11 ApprovalChainTimeline via approval_instance_id.
    await expect(page.getByTestId('chain-line-1')).toBeVisible();
    await expect(page.getByTestId('chain-line-2')).toBeVisible();
    // A link to the standalone approval instance is present.
    await expect(
      page.getByRole('link', { name: /lihat rantai|rantai persetujuan/i }).first(),
    ).toBeVisible();
  });
});
