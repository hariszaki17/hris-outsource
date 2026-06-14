/**
 * lib/e3-helpers.ts
 *
 * Shared UI/API helpers for the E3 placement E2E specs. Derived from the REAL
 * rendered component DOM (placements-screen / placement-form / placement-overlays /
 * company-roster-screen) and the @swp/ui Combobox primitive:
 *
 *   - Combobox (used by every FK picker): trigger is a `button[aria-haspopup="listbox"]`;
 *     opening renders a popover with a search `input` and option `<button>` rows whose
 *     text is the option label. To select: click trigger → type → click the option button.
 *   - DataTable rows are `div.border-b` (filter by visible text), scoped to the table.
 *   - Toggles are `role=switch`. Forms use noValidate (JS validation only).
 *
 * apiAs(page, method, path, body):
 *   Drive a direct authenticated API request from the browser context using the
 *   in-memory access token exposed by auth.ts via the `window.__swp_get_token__`
 *   getter (only defined when VITE_ENABLE_MSW='false'). Returns { status, body }.
 *   This is how the negative-invariant scenarios assert the REAL 409 envelope.
 */

import { expect, type Locator, type Page } from '@playwright/test';

export const API_BASE = 'http://localhost:8081/api/v1';

// ---------------------------------------------------------------------------
// Direct authenticated API call from the browser (uses in-memory access token)
// ---------------------------------------------------------------------------

export interface ApiResult {
  status: number;
  body: unknown;
}

/**
 * apiAs — issue an authenticated fetch() from inside the page using the in-memory
 * Bearer token (window.__swp_get_token__). The page MUST already be logged in.
 */
export async function apiAs(
  page: Page,
  method: string,
  path: string,
  body?: unknown,
): Promise<ApiResult> {
  return page.evaluate(
    async ({ base, m, p, b }) => {
      const token = (window as unknown as { __swp_get_token__?: string }).__swp_get_token__ ?? null;
      const headers: Record<string, string> = { 'Content-Type': 'application/json' };
      if (token) headers.Authorization = `Bearer ${token}`;
      // Action endpoints (:end, :transfer, …) and creates require an Idempotency-Key.
      headers['Idempotency-Key'] = `e2e-${Date.now()}-${Math.random().toString(36).slice(2)}`;
      const res = await fetch(`${base}${p}`, {
        method: m,
        headers,
        credentials: 'include',
        body: b !== undefined ? JSON.stringify(b) : undefined,
      });
      let parsed: unknown = null;
      const ct = res.headers.get('content-type') ?? '';
      if (ct.includes('application/json')) parsed = await res.json();
      return { status: res.status, body: parsed };
    },
    { base: API_BASE, m: method, p: path, b: body },
  );
}

/** Extract `error.code` from a standard API error envelope ({ error: { code, ... } }). */
export function errorCode(body: unknown): string | undefined {
  const env = body as { error?: { code?: string } } | null;
  return env?.error?.code;
}

/** Extract `error.details` (INVViolationDetails) from a standard API error envelope. */
export function errorDetails(body: unknown): Record<string, unknown> | undefined {
  const env = body as { error?: { details?: Record<string, unknown> } } | null;
  return env?.error?.details;
}

// ---------------------------------------------------------------------------
// Combobox / FK-picker interaction (matches @swp/ui Combobox DOM)
// ---------------------------------------------------------------------------

/**
 * pickCombobox — open the Combobox rooted at `triggerScope`, type `search`, and
 * click the first option button whose text matches `optionText`.
 *
 * `triggerScope` is a Locator that contains exactly one Combobox trigger
 * (e.g. a FormField wrapper, or a fixed-width filter div).
 */
export async function pickCombobox(
  page: Page,
  triggerScope: Locator,
  optionText: string | RegExp,
  search?: string,
): Promise<void> {
  const trigger = triggerScope.locator('button[aria-haspopup="listbox"]').first();
  await expect(trigger).toBeVisible({ timeout: 10_000 });
  await trigger.click();

  // The popover is rendered as a sibling within the same container; its search input
  // is the first text input that appears after opening.
  const popoverInput = triggerScope.locator('input[type="text"]').first();
  if (search !== undefined) {
    await popoverInput.fill(search);
  }

  // Option rows are <button> elements inside the popover <ul>. Match by visible text.
  const option = triggerScope.locator('ul button', { hasText: optionText }).first();
  await expect(option).toBeVisible({ timeout: 10_000 });
  await option.click();
}

/**
 * fieldScope — locate the FormField wrapper that contains a control with the given
 * stable id (htmlFor on the FormField). For Combobox-backed fields the id is on the
 * FormField label, and the trigger lives in the same wrapper. We climb to the
 * nearest FormField container via the label[for=...] then its parent.
 */
export function comboFieldById(page: Page, htmlFor: string): Locator {
  // FormField renders a <label htmlFor={id}> sibling to the control. The control
  // (Combobox) shares the same wrapper. Use xpath to grab the label's parent block.
  return page.locator(`xpath=//label[@for="${htmlFor}"]/..`);
}
