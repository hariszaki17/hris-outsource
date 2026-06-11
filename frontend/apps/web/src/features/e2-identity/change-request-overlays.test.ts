/**
 * Unit tests for the change-request review bank-split gating (Plan §E/§F).
 *
 * Scope: pure-logic assertions (no DOM render) — mirrors the e6 leave-quotas / nav test pattern.
 * The gating lives inline in `change-request-overlays.tsx` (ChangeRequestDetailDrawer / DiffRow);
 * these tests re-encode the SAME rules so the bank boundary can't silently drift.
 *
 * Bank-split (E2 employee-profile.md): a shift leader (company-scoped) holds
 * `change_requests.approve` but NOT `change_requests.approve.bank`. They may apply the non-bank
 * fields; the `bank_account` field is shown read-only with a "Perlu HR" badge and escalates to
 * HR. HR/super-admin hold `.approve.bank` and can finalize the bank field. Client gating is
 * defense-in-depth only — the Go API is the real gate (ENGINEERING.md C1).
 */

import { permissionsForRole } from '@swp/shared';
import { describe, expect, it } from 'vitest';
import { hasPermission } from '../../app/nav.ts';

// Mirrors change-request-overlays.tsx.
const BANK_FIELD_KEY = 'bank_account';
const BANK_PERMISSION = 'change_requests.approve.bank' as const;

/** True if the reviewer can finalize the bank field (mirrors `canApproveBank`). */
function canApproveBank(permissions: readonly string[]): boolean {
  return permissions.includes(BANK_PERMISSION);
}

/** Whether a given diff row is bank-gated (read-only "Perlu HR") for this reviewer. */
function isBankGated(fieldKey: string, permissions: readonly string[]): boolean {
  return fieldKey === BANK_FIELD_KEY && !canApproveBank(permissions);
}

// ---------------------------------------------------------------------------
// 1. canApproveBank — role bundle membership
// ---------------------------------------------------------------------------

describe('canApproveBank (change_requests.approve.bank)', () => {
  it('shift_leader lacks the bank sub-permission (non-bank approver only)', () => {
    const perms = permissionsForRole('shift_leader');
    // SL has the base approve key…
    expect(perms).toContain('change_requests.approve');
    // …but NOT the bank sub-permission.
    expect(canApproveBank(perms)).toBe(false);
  });

  it('hr_admin and super_admin hold the bank sub-permission', () => {
    for (const role of ['hr_admin', 'super_admin'] as const) {
      expect(canApproveBank(permissionsForRole(role))).toBe(true);
    }
  });

  it('hasPermission resolves the bank gate identically to the inline check', () => {
    // The screen uses user.permissions.includes(...); the nav helper must agree for the
    // route-level/inbox visibility path. (Defense-in-depth, single source of truth.)
    const sl = permissionsForRole('shift_leader');
    const hr = permissionsForRole('hr_admin');
    expect(hasPermission(sl, BANK_PERMISSION)).toBe(false);
    expect(hasPermission(hr, BANK_PERMISSION)).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// 2. DiffRow bank gating — the bank row is disabled ("Perlu HR") without the perm
// ---------------------------------------------------------------------------

describe('DiffRow bank gating (bankGated)', () => {
  it('the bank_account row is gated for a shift_leader (shown read-only, "Perlu HR")', () => {
    expect(isBankGated('bank_account', permissionsForRole('shift_leader'))).toBe(true);
  });

  it('non-bank rows are NEVER gated, even for a shift_leader', () => {
    for (const field of ['phone', 'emergency_contact', 'address']) {
      expect(isBankGated(field, permissionsForRole('shift_leader'))).toBe(false);
    }
  });

  it('the bank_account row is editable for HR (has the bank perm → not gated)', () => {
    expect(isBankGated('bank_account', permissionsForRole('hr_admin'))).toBe(false);
    expect(isBankGated('bank_account', permissionsForRole('super_admin'))).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// 3. Approve CTA copy — mirrors the drawer's label resolution
// ---------------------------------------------------------------------------

/** Resolves the approve-button i18n key from the request shape + reviewer perm. */
function approveCtaKey(opts: {
  permissions: readonly string[];
  hasBankField: boolean;
  hasNonBankField: boolean;
}): string {
  const can = canApproveBank(opts.permissions);
  // Bank-only request a leader can't finalize → approving escalates it to HR.
  if (!can && opts.hasBankField && !opts.hasNonBankField) {
    return 'changeRequests.escalateToHrAction';
  }
  // Mixed request a leader approves → only the non-bank fields apply.
  if (!can && opts.hasBankField && opts.hasNonBankField) {
    return 'changeRequests.approveNonBankAction';
  }
  return 'changeRequests.approveAction';
}

describe('approve CTA copy (bank-split aware)', () => {
  const sl = permissionsForRole('shift_leader');
  const hr = permissionsForRole('hr_admin');

  it('SL · bank-only request → "escalate to HR"', () => {
    expect(approveCtaKey({ permissions: sl, hasBankField: true, hasNonBankField: false })).toBe(
      'changeRequests.escalateToHrAction',
    );
  });

  it('SL · mixed request → "approve non-bank" (bank escalates)', () => {
    expect(approveCtaKey({ permissions: sl, hasBankField: true, hasNonBankField: true })).toBe(
      'changeRequests.approveNonBankAction',
    );
  });

  it('SL · non-bank-only request → plain "approve"', () => {
    expect(approveCtaKey({ permissions: sl, hasBankField: false, hasNonBankField: true })).toBe(
      'changeRequests.approveAction',
    );
  });

  it('HR · any request → plain "approve" (can finalize the bank field)', () => {
    expect(approveCtaKey({ permissions: hr, hasBankField: true, hasNonBankField: true })).toBe(
      'changeRequests.approveAction',
    );
    expect(approveCtaKey({ permissions: hr, hasBankField: true, hasNonBankField: false })).toBe(
      'changeRequests.approveAction',
    );
  });
});
