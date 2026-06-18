/**
 * Unit tests for the agent Kehadiran attendance-activity gate logic (E5 F5.8).
 *
 * Scope: pure-logic assertions (no DOM render) — mirrors the e5 attendance-dashboard /
 * me-akun test pattern. The gate helpers (`canClockOut`, `isValidNote`) live in
 * me-kehadiran-screen.tsx and are imported here so the rules can't silently drift.
 * The ACTIVITY_REQUIRED → message mapping re-encodes the same switch the screen uses
 * in handleClockError, keyed off the ApiError `code`.
 *
 * TC ids trace to docs/epics/E5-attendance/prds/attendance-activity.test-cases.md.
 */

import { ApiError } from '@swp/api-client';
import { describe, expect, it } from 'vitest';
import { ACTIVITY_NOTE_MAX, canClockOut, isValidNote } from './me-kehadiran-screen.tsx';

// ---------------------------------------------------------------------------
// Clock-out gate (AA-7) — TC-3, TC-4, TC-5
// ---------------------------------------------------------------------------

describe('canClockOut — clock-out gate (AA-7)', () => {
  it('TC-3: blocks clock-out when the open record has zero activities', () => {
    expect(canClockOut(0)).toBe(false);
  });

  it('TC-4: allows clock-out with exactly one activity', () => {
    expect(canClockOut(1)).toBe(true);
  });

  it('allows clock-out with many activities', () => {
    expect(canClockOut(3)).toBe(true);
  });

  it('TC-5: blocks again once the count drops back to zero (last activity deleted)', () => {
    // Simulate deleting the only activity: 1 → 0.
    let count = 1;
    expect(canClockOut(count)).toBe(true);
    count -= 1;
    expect(canClockOut(count)).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// Note validation (AA-6) — TC-10, TC-11
// ---------------------------------------------------------------------------

describe('isValidNote — note free-text validation (AA-6)', () => {
  it('TC-10: rejects an empty note', () => {
    expect(isValidNote('')).toBe(false);
  });

  it('TC-10: rejects a whitespace-only note (trimmed)', () => {
    expect(isValidNote('   ')).toBe(false);
  });

  it('accepts a normal note', () => {
    expect(isValidNote('Patroli lantai 1')).toBe(true);
  });

  it('TC-11: accepts a 500-char note (boundary)', () => {
    expect(isValidNote('a'.repeat(ACTIVITY_NOTE_MAX))).toBe(true);
  });

  it('TC-11: rejects a 501-char note (over-length)', () => {
    expect(isValidNote('a'.repeat(ACTIVITY_NOTE_MAX + 1))).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// ACTIVITY_REQUIRED maps to the gate message — TC-3
// ---------------------------------------------------------------------------

/**
 * Mirrors handleClockError's switch in me-kehadiran-screen.tsx: a 422 ApiError whose
 * code is ACTIVITY_REQUIRED resolves to the activity.required toast key (the agent stays
 * clocked in); any other code falls through to the generic clock error.
 */
function clockOutErrorKey(e: unknown): string {
  if (e instanceof ApiError) {
    if (e.code === 'ACTIVITY_REQUIRED') return 'activity.required';
    if (e.code === 'ON_LEAVE') return 'onLeaveToday';
  }
  return 'clockError';
}

describe('clockOutErrorKey — server gate mapping (AA-7)', () => {
  it('TC-3: maps 422 ACTIVITY_REQUIRED to the activity.required message', () => {
    const err = new ApiError(422, {
      error: {
        code: 'ACTIVITY_REQUIRED',
        message: 'no activities',
        fields: { activity_count: '0' },
      },
    });
    expect(clockOutErrorKey(err)).toBe('activity.required');
  });

  it('falls through to the generic clock error for an unrelated code', () => {
    const err = new ApiError(422, { error: { code: 'SOMETHING_ELSE', message: 'x' } });
    expect(clockOutErrorKey(err)).toBe('clockError');
  });

  it('falls through to the generic clock error for a non-ApiError', () => {
    expect(clockOutErrorKey(new Error('boom'))).toBe('clockError');
  });
});
