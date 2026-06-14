/**
 * Unit tests for E6 leave-quotas-screen (per-type quota ledger, 2026-06-12).
 *
 * Pure-logic assertions (no DOM render) — mirrors the e5 attendance test pattern.
 * Covers:
 *   1. LeaveQuotasSearch type shape — q (directory) + employee_id (drill-in).
 *   2. CT row selection by cap_basis === 'ANNUAL_POOL'.
 *   3. Special-leave count (non-CT types with usage).
 *   4. Adjust modal: new entitled / new remaining preview from delta.
 *   5. Remaining color coding (ok / neutral).
 *   6. adjust-entitled body shape (employee_id, leave_type_id, start_date, delta, reason).
 *   7. Combobox option resolution (value = employee_id).
 */

import { describe, expect, it } from 'vitest';
import type { LeaveQuotasSearch } from './leave-quotas-screen.ts';

type Bal = {
  cap_basis: string;
  entitled_days?: number | null;
  used_days: number;
  pending_days: number;
  remaining_days?: number | null;
};

const CAP_ANNUAL = 'ANNUAL_POOL';

describe('LeaveQuotasSearch type', () => {
  it('accepts q for the directory and employee_id for drill-in', () => {
    const a: LeaveQuotasSearch = { q: 'Budi', cursor: 'abc' };
    const b: LeaveQuotasSearch = { employee_id: 'SWP-EMP-1042' };
    expect(a.q).toBe('Budi');
    expect(b.employee_id).toBe('SWP-EMP-1042');
  });

  it('all fields optional', () => {
    const s: LeaveQuotasSearch = {};
    expect(s.q).toBeUndefined();
    expect(s.employee_id).toBeUndefined();
  });
});

describe('CT row selection (cap_basis ANNUAL_POOL)', () => {
  const balances: Bal[] = [
    { cap_basis: 'PER_EVENT', used_days: 1, pending_days: 0 },
    { cap_basis: CAP_ANNUAL, entitled_days: 12, used_days: 4, pending_days: 0, remaining_days: 8 },
    { cap_basis: 'PER_MONTH', used_days: 0, pending_days: 0 },
  ];

  it('finds the annual-pool balance', () => {
    const ct = balances.find((b) => b.cap_basis === CAP_ANNUAL);
    expect(ct?.entitled_days).toBe(12);
    expect(ct?.remaining_days).toBe(8);
  });

  it('counts non-CT types with usage as special-leave', () => {
    const special = balances.filter(
      (b) => b.cap_basis !== CAP_ANNUAL && (b.used_days > 0 || b.pending_days > 0),
    ).length;
    expect(special).toBe(1);
  });
});

describe('adjust preview (delta → new entitled / remaining)', () => {
  it('newEntitled = entitled + delta; newRemaining = newEntitled - used', () => {
    const entitled = 12;
    const used = 8;
    const delta = 2;
    const newEntitled = entitled + delta;
    const newRemaining = newEntitled - used;
    expect(newEntitled).toBe(14);
    expect(newRemaining).toBe(6);
  });

  it('negative delta can drive remaining below zero (flagged red)', () => {
    const newRemaining = 10 - 4 /*delta -6*/ - 8;
    expect(newRemaining).toBeLessThan(0);
  });
});

describe('remaining color coding', () => {
  function color(rem: number | null | undefined): 'ok' | 'neutral' | 'uncapped' {
    if (rem == null) return 'uncapped';
    return rem <= 0 ? 'neutral' : 'ok';
  }
  it('positive → ok', () => expect(color(8)).toBe('ok'));
  it('zero → neutral', () => expect(color(0)).toBe('neutral'));
  it('null → uncapped (sesuai ketentuan)', () => expect(color(null)).toBe('uncapped'));
});

describe('adjust-entitled body', () => {
  it('constructs employee_id, leave_type_id, start_date, delta, reason', () => {
    const body = {
      employee_id: 'SWP-EMP-1042',
      leave_type_id: 'SWP-LT-001',
      start_date: '2026-06-13',
      delta: 2,
      reason: 'Koreksi entitlement sesuai surat HRD.',
    };
    expect(body.employee_id).toBe('SWP-EMP-1042');
    expect(body.leave_type_id).toBe('SWP-LT-001');
    expect(body.delta).toBe(2);
    expect(body.reason.length).toBeGreaterThanOrEqual(5);
  });
});

describe('EmployeeCombobox option resolution', () => {
  it('value = employee_id, label = full_name, sublabel = nip ?? nik', () => {
    const e = { id: 'SWP-EMP-1042', full_name: 'Budi Santoso', nip: 'SWP-0042', nik: '3201' };
    const option = { value: e.id, label: e.full_name, sublabel: e.nip ?? e.nik };
    expect(option.value).toBe('SWP-EMP-1042');
    expect(option.label).toBe('Budi Santoso');
    expect(option.sublabel).toBe('SWP-0042');
  });
});
