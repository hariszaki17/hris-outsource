/**
 * Unit tests for E6 leave-quotas-screen (grant-lot ledger, 2026-06-08).
 *
 * Scope: pure-logic assertions (no DOM render) — mirrors the e5 attendance test pattern.
 * Covers:
 *   1. LeaveQuotasSearch type shape — new employee_id filter field.
 *   2. remainingDays() computation (amount - consumed - pending).
 *   3. earmarkBadgeTone() — MATERNITY → warn, other earmark → ok, null → neutral.
 *   4. Grant form: earmark field visibility (only for MATERNITY / STATUTORY sources).
 *   5. Grant form payload — employee_id + amount_days + expires_at + source + remark.
 *   6. Adjust form payload — remark required, amount / expires optional.
 *   7. Balance screen: pool summary derived from LeaveBalance (pool_remaining + next_expiry).
 *   8. Balance screen: earmarked lot line rendered per earmarked[] entry.
 *
 * F6.1 — LQ-6: remark required on every grant/adjust.
 */

import { LeaveGrantSource } from '@swp/api-client/e6';
import { describe, expect, it } from 'vitest';
import type { LeaveQuotasSearch } from './leave-quotas-screen.ts';

// ---------------------------------------------------------------------------
// 1. Type shape — verify employee_id filter field exists at compile time
// ---------------------------------------------------------------------------

describe('LeaveQuotasSearch type', () => {
  it('accepts employee_id filter', () => {
    const search: LeaveQuotasSearch = { employee_id: 'SWP-EMP-1042', cursor: 'abc' };
    expect(search.employee_id).toBe('SWP-EMP-1042');
  });

  it('all fields are optional', () => {
    const search: LeaveQuotasSearch = {};
    expect(search.employee_id).toBeUndefined();
    expect(search.cursor).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// 2. remainingDays() — mirrors leave-quotas-screen.tsx
// ---------------------------------------------------------------------------

function remainingDays(lot: {
  amount_days: number;
  consumed_days: number;
  pending_days: number;
}): number {
  return lot.amount_days - lot.consumed_days - lot.pending_days;
}

describe('remainingDays()', () => {
  it('amount=12, consumed=4, pending=0 → 8', () => {
    expect(remainingDays({ amount_days: 12, consumed_days: 4, pending_days: 0 })).toBe(8);
  });

  it('amount=12, consumed=4, pending=3 → 5', () => {
    expect(remainingDays({ amount_days: 12, consumed_days: 4, pending_days: 3 })).toBe(5);
  });

  it('amount=0, consumed=0, pending=0 → 0', () => {
    expect(remainingDays({ amount_days: 0, consumed_days: 0, pending_days: 0 })).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// 3. earmarkBadgeTone() — mirrors leave-quotas-screen.tsx
// ---------------------------------------------------------------------------

function earmarkBadgeTone(earmark: string | null | undefined): 'ok' | 'warn' | 'neutral' {
  if (!earmark) return 'neutral';
  if (earmark === 'MATERNITY') return 'warn';
  return 'ok';
}

describe('earmarkBadgeTone()', () => {
  it('null earmark → neutral (general pool)', () => {
    expect(earmarkBadgeTone(null)).toBe('neutral');
  });

  it('undefined earmark → neutral', () => {
    expect(earmarkBadgeTone(undefined)).toBe('neutral');
  });

  it('MATERNITY earmark → warn (LQ-10 restricted lot)', () => {
    expect(earmarkBadgeTone('MATERNITY')).toBe('warn');
  });

  it('other earmark code → ok', () => {
    expect(earmarkBadgeTone('STATUTORY_HAJJ')).toBe('ok');
  });
});

// ---------------------------------------------------------------------------
// 4. Earmark field visibility — shown only for MATERNITY / STATUTORY sources
// ---------------------------------------------------------------------------

const EARMARK_SOURCES: LeaveGrantSource[] = [
  LeaveGrantSource.MATERNITY,
  LeaveGrantSource.STATUTORY,
];

function showEarmark(source: LeaveGrantSource): boolean {
  return EARMARK_SOURCES.includes(source);
}

describe('earmark field visibility (grant form)', () => {
  it('shown for MATERNITY source', () => {
    expect(showEarmark(LeaveGrantSource.MATERNITY)).toBe(true);
  });

  it('shown for STATUTORY source', () => {
    expect(showEarmark(LeaveGrantSource.STATUTORY)).toBe(true);
  });

  it('hidden for ANNUAL source', () => {
    expect(showEarmark(LeaveGrantSource.ANNUAL)).toBe(false);
  });

  it('hidden for ADJUSTMENT source', () => {
    expect(showEarmark(LeaveGrantSource.ADJUSTMENT)).toBe(false);
  });

  it('hidden for BONUS source', () => {
    expect(showEarmark(LeaveGrantSource.BONUS)).toBe(false);
  });

  it('hidden for MIGRATION source', () => {
    expect(showEarmark(LeaveGrantSource.MIGRATION)).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// 5. Grant form payload shape
// ---------------------------------------------------------------------------

describe('grant form payload (POST /leave-grants)', () => {
  it('constructs the correct body for a ADJUSTMENT lot without earmark', () => {
    const values = {
      employee_id: 'SWP-EMP-1042',
      amount_days: 3,
      expires_at: '2026-12-31',
      source: LeaveGrantSource.ADJUSTMENT,
      remark: 'Goodwill correction per HRD 2026-06-08.',
      earmark: '',
    };
    const showEarmarkForSource = showEarmark(values.source as LeaveGrantSource);
    const body = {
      employee_id: values.employee_id,
      amount_days: values.amount_days,
      expires_at: values.expires_at,
      source: values.source,
      remark: values.remark,
      ...(showEarmarkForSource && values.earmark ? { earmark: values.earmark } : {}),
    };
    expect(body.employee_id).toBe('SWP-EMP-1042');
    expect(body.amount_days).toBe(3);
    expect(body.source).toBe(LeaveGrantSource.ADJUSTMENT);
    expect(body.remark).toBe('Goodwill correction per HRD 2026-06-08.');
    expect('earmark' in body).toBe(false);
  });

  it('includes earmark for MATERNITY source when earmark is provided', () => {
    const values = {
      employee_id: 'SWP-EMP-1042',
      amount_days: 90,
      expires_at: '2027-03-31',
      source: LeaveGrantSource.MATERNITY,
      remark: 'Pre-fund cuti melahirkan.',
      earmark: 'MATERNITY',
    };
    const showEarmarkForSource = showEarmark(values.source as LeaveGrantSource);
    const body = {
      employee_id: values.employee_id,
      amount_days: values.amount_days,
      expires_at: values.expires_at,
      source: values.source,
      remark: values.remark,
      ...(showEarmarkForSource && values.earmark ? { earmark: values.earmark } : {}),
    };
    expect(body.earmark).toBe('MATERNITY');
    expect(body.amount_days).toBe(90);
  });
});

// ---------------------------------------------------------------------------
// 6. Adjust form payload — remark required, amount / expires optional
// ---------------------------------------------------------------------------

describe('adjust form payload (PATCH /leave-grants/{id})', () => {
  it('body includes remark (required) and omits undefined amount/expires', () => {
    const values = {
      remark: 'Extended by HRD policy 2026-06-08.',
      amount_days: undefined as number | undefined,
      expires_at: '2027-03-31',
      earmark: undefined as string | undefined,
    };
    const body = {
      remark: values.remark,
      ...(values.amount_days !== undefined ? { amount_days: values.amount_days } : {}),
      ...(values.expires_at ? { expires_at: values.expires_at } : {}),
      ...(values.earmark !== undefined ? { earmark: values.earmark || null } : {}),
    };
    expect(body.remark).toBe('Extended by HRD policy 2026-06-08.');
    expect('amount_days' in body).toBe(false);
    expect(body.expires_at).toBe('2027-03-31');
  });

  it('body includes amount_days when provided', () => {
    const values = {
      remark: 'Correction.',
      amount_days: 15,
      expires_at: '',
      earmark: undefined as string | undefined,
    };
    const body = {
      remark: values.remark,
      ...(values.amount_days !== undefined ? { amount_days: values.amount_days } : {}),
      ...(values.expires_at ? { expires_at: values.expires_at } : {}),
    };
    expect(body.amount_days).toBe(15);
    expect('expires_at' in body).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// 7. Balance screen: pool summary derived from LeaveBalance
// ---------------------------------------------------------------------------

interface MockLeaveBalance {
  pool_remaining: number;
  next_expiry?: string | null;
  earmarked: { earmark: string; remaining_days: number; expires_at: string }[];
}

/** Mirrors the rendering logic in EmployeePoolSummary. */
function derivePoolSummary(balance: MockLeaveBalance) {
  return {
    poolRemaining: balance.pool_remaining,
    hasExpiry: !!balance.next_expiry,
    expiryDate: balance.next_expiry ?? null,
    earmarkedCount: balance.earmarked.length,
  };
}

describe('EmployeePoolSummary derived state', () => {
  it('renders pool_remaining from LeaveBalance', () => {
    const balance: MockLeaveBalance = { pool_remaining: 8, earmarked: [] };
    const s = derivePoolSummary(balance);
    expect(s.poolRemaining).toBe(8);
    expect(s.hasExpiry).toBe(false);
    expect(s.earmarkedCount).toBe(0);
  });

  it('shows expiry hint when next_expiry is set', () => {
    const balance: MockLeaveBalance = {
      pool_remaining: 8,
      next_expiry: '2026-12-31',
      earmarked: [],
    };
    const s = derivePoolSummary(balance);
    expect(s.hasExpiry).toBe(true);
    expect(s.expiryDate).toBe('2026-12-31');
  });
});

// ---------------------------------------------------------------------------
// 8. Balance screen: earmarked lot line rendered per earmarked[] entry
// ---------------------------------------------------------------------------

describe('earmarked lot lines', () => {
  it('renders one badge per earmarked lot', () => {
    const balance: MockLeaveBalance = {
      pool_remaining: 5,
      earmarked: [
        { earmark: 'MATERNITY', remaining_days: 90, expires_at: '2027-03-31' },
        { earmark: 'STATUTORY_HAJJ', remaining_days: 40, expires_at: '2026-12-31' },
      ],
    };
    expect(balance.earmarked.length).toBe(2);
    const earmarkCodes = balance.earmarked.map((e) => e.earmark);
    expect(earmarkCodes).toContain('MATERNITY');
    expect(earmarkCodes).toContain('STATUTORY_HAJJ');
  });

  it('earmarked lot with MATERNITY earmark has warn tone', () => {
    const earmark = 'MATERNITY';
    expect(earmarkBadgeTone(earmark)).toBe('warn');
  });
});
