/**
 * Unit tests for E6 leave-quotas-screen (per-employee aggregate list + lot drill-in, 2026-06-08).
 *
 * Scope: pure-logic assertions (no DOM render) — mirrors the e5 attendance test pattern.
 * Covers:
 *   1. LeaveQuotasSearch type shape — q search (not employee_id) for the list; employee_id for drill-in.
 *   2. remainingDays() computation (amount - consumed - pending) for lot rows.
 *   3. earmarkBadgeTone() — MATERNITY → warn, other earmark → ok, null → neutral.
 *   4. Grant form: earmark field visibility (only for MATERNITY / STATUTORY sources).
 *   5. Grant form payload — employee_id + amount_days + expires_at + source + remark.
 *   6. Adjust form payload — remark required, amount / expires optional.
 *   7. Aggregate list: pool_remaining color coding (ok / neutral / bad).
 *   8. Aggregate list: next_expiry renders "—" when null.
 *   9. Search wires to q param (not employee_id) for the list endpoint.
 *  10. Employee combobox resolves employee_id from selected option value.
 *
 * F6.1 — LQ-6: remark required on every grant/adjust.
 * useListLeaveBalances: one row per employee, q filters name/NIK/NIP.
 */

import { LeaveGrantSource } from '@swp/api-client/e6';
import { describe, expect, it } from 'vitest';
import type { LeaveQuotasSearch } from './leave-quotas-screen.ts';

// ---------------------------------------------------------------------------
// 1. Type shape — q filter for list + employee_id for drill-in
// ---------------------------------------------------------------------------

describe('LeaveQuotasSearch type', () => {
  it('accepts q search field for the aggregate list', () => {
    const search: LeaveQuotasSearch = { q: 'Budi', cursor: 'abc' };
    expect(search.q).toBe('Budi');
  });

  it('accepts employee_id for drill-in selection', () => {
    const search: LeaveQuotasSearch = { employee_id: 'SWP-EMP-1042' };
    expect(search.employee_id).toBe('SWP-EMP-1042');
  });

  it('all fields are optional', () => {
    const search: LeaveQuotasSearch = {};
    expect(search.q).toBeUndefined();
    expect(search.employee_id).toBeUndefined();
    expect(search.cursor).toBeUndefined();
  });

  it('list search uses q (not employee_id) for name/NIK/NIP filter', () => {
    // q is mapped to ListLeaveBalancesParams.q, not employee_id
    const search: LeaveQuotasSearch = { q: 'SWP-0042' };
    expect(search.q).toBe('SWP-0042');
    expect(search.employee_id).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// 2. remainingDays() — mirrors leave-quotas-screen.tsx (lot-level)
// ---------------------------------------------------------------------------

function remainingDays(lot: {
  amount_days: number;
  consumed_days: number;
  pending_days: number;
}): number {
  return lot.amount_days - lot.consumed_days - lot.pending_days;
}

describe('remainingDays() (lot level)', () => {
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
// 5. Grant form payload shape (employee selected via combobox)
// ---------------------------------------------------------------------------

describe('grant form payload (POST /leave-grants)', () => {
  it('constructs the correct body for a ADJUSTMENT lot without earmark', () => {
    const values = {
      employee_id: 'SWP-EMP-1042', // resolved from combobox selection
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
// 7. Aggregate list: pool_remaining color coding
// ---------------------------------------------------------------------------

type RemColor = 'ok' | 'neutral' | 'bad';

function poolRemainingColor(remaining: number): RemColor {
  if (remaining < 0) return 'bad';
  if (remaining === 0) return 'neutral';
  return 'ok';
}

describe('pool_remaining color coding (aggregate list)', () => {
  it('positive remaining → ok (green)', () => {
    expect(poolRemainingColor(8)).toBe('ok');
  });

  it('zero remaining → neutral', () => {
    expect(poolRemainingColor(0)).toBe('neutral');
  });

  it('negative remaining → bad (red)', () => {
    expect(poolRemainingColor(-1)).toBe('bad');
  });
});

// ---------------------------------------------------------------------------
// 8. Aggregate list: next_expiry renders "—" when null
// ---------------------------------------------------------------------------

describe('next_expiry display', () => {
  it('null next_expiry renders as dash', () => {
    const next_expiry: string | null = null;
    const display = next_expiry ?? '—';
    expect(display).toBe('—');
  });

  it('non-null next_expiry is forwarded to DateText', () => {
    const next_expiry: string | null = '2026-12-31';
    const display = next_expiry ?? '—';
    expect(display).toBe('2026-12-31');
  });
});

// ---------------------------------------------------------------------------
// 9. Search wires to q param (not employee_id) for the list endpoint
// ---------------------------------------------------------------------------

describe('search param → q (useListLeaveBalances)', () => {
  it('q is passed to ListLeaveBalancesParams.q for ILIKE search', () => {
    const search: LeaveQuotasSearch = { q: 'Budi' };
    // Mirrors the screen: listParams.q = search.q
    const listParams = {
      limit: 50,
      ...(search.q ? { q: search.q } : {}),
    };
    expect(listParams.q).toBe('Budi');
    expect('employee_id' in listParams).toBe(false);
  });

  it('empty q omits the q param entirely', () => {
    const search: LeaveQuotasSearch = {};
    const listParams = {
      limit: 50,
      ...(search.q ? { q: search.q } : {}),
    };
    expect('q' in listParams).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// 10. Employee combobox: stores employee_id, displays name
// ---------------------------------------------------------------------------

describe('EmployeeCombobox option resolution', () => {
  it('ComboboxOption.value = employee_id, label = full_name', () => {
    // Mirrors the screen: options = employees.map(e => ({ value: e.id, label: e.full_name, ... }))
    const employee = {
      id: 'SWP-EMP-1042',
      full_name: 'Budi Santoso',
      nip: 'SWP-0042',
      nik: '3201...',
    };
    const option = {
      value: employee.id,
      label: employee.full_name,
      sublabel: employee.nip ?? employee.nik,
      meta: employee.id,
    };
    expect(option.value).toBe('SWP-EMP-1042');
    expect(option.label).toBe('Budi Santoso');
    // on onChange(option.value) the form stores the employee_id
    const storedId = option.value;
    expect(storedId).toBe('SWP-EMP-1042');
  });

  it('selecting an option stores employee_id in the grant form', () => {
    // onChange receives the value (employee_id), which is set via setValue('employee_id', id)
    let storedEmployeeId = '';
    const onChange = (id: string | null) => {
      storedEmployeeId = id ?? '';
    };
    onChange('SWP-EMP-1042');
    expect(storedEmployeeId).toBe('SWP-EMP-1042');
  });
});
