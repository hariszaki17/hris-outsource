/**
 * Branded SWP-<ENTITY>-<NUMERIC> ID types. ENGINEERING.md A3 · CONVENTIONS.md §4.
 *
 * IDs are OPAQUE strings — never parse the numeric portion. Branding stops an
 * `SWP-EMP` id being passed where an `SWP-PL` is expected, at compile time.
 */

declare const __brand: unique symbol;
type Brand<T extends string> = { readonly [__brand]: T };

/** A prefixed, branded id string. */
export type Id<P extends string> = string & Brand<P>;

// Entity prefixes — CONVENTIONS.md §4 entity-prefix table (keep in sync).
export const ID_PREFIX = {
  USER: 'SWP-USR',
  AUDIT_LOG: 'SWP-AL',
  EMPLOYEE: 'SWP-EMP',
  AGREEMENT: 'SWP-AG',
  CLIENT_COMPANY: 'SWP-CMP',
  LEAVE_TYPE: 'SWP-LT',
  ATTENDANCE_CODE: 'SWP-AC',
  OVERTIME_RULE: 'SWP-OTR',
  CHANGE_REQUEST: 'SWP-CHG',
  PLACEMENT: 'SWP-PL',
  SHIFT_LEADER_ASSIGNMENT: 'SWP-SLA',
  SHIFT_MASTER: 'SWP-SHF',
  SCHEDULE: 'SWP-SCH',
  ATTENDANCE: 'SWP-ATT',
  CORRECTION: 'SWP-COR',
  LEAVE_REQUEST: 'SWP-LR',
  LEAVE_QUOTA: 'SWP-LQ',
  OVERTIME: 'SWP-OT',
  HOLIDAY: 'SWP-HOL',
  PAYSLIP: 'SWP-PS',
  NOTIFICATION: 'SWP-NTF',
  EXPORT: 'SWP-EXP',
} as const;

export type EntityPrefix = (typeof ID_PREFIX)[keyof typeof ID_PREFIX];

// Convenience aliases used across the app.
export type UserId = Id<typeof ID_PREFIX.USER>;
export type EmployeeId = Id<typeof ID_PREFIX.EMPLOYEE>;
export type ClientCompanyId = Id<typeof ID_PREFIX.CLIENT_COMPANY>;
export type PlacementId = Id<typeof ID_PREFIX.PLACEMENT>;
export type ScheduleId = Id<typeof ID_PREFIX.SCHEDULE>;
export type AttendanceId = Id<typeof ID_PREFIX.ATTENDANCE>;
export type LeaveRequestId = Id<typeof ID_PREFIX.LEAVE_REQUEST>;
export type OvertimeId = Id<typeof ID_PREFIX.OVERTIME>;

const ID_RE = /^SWP-[A-Z]+-\d+$/;

/** True if `value` is a structurally valid SWP id with the given prefix. */
export function isId<P extends EntityPrefix>(prefix: P, value: unknown): value is Id<P> {
  return typeof value === 'string' && value.startsWith(`${prefix}-`) && ID_RE.test(value);
}

/** Assert + brand. Throws on malformed/wrong-prefix input (use at trust boundaries). */
export function asId<P extends EntityPrefix>(prefix: P, value: string): Id<P> {
  if (!isId(prefix, value)) {
    throw new Error(`Invalid id: expected ${prefix}-<n>, got "${value}"`);
  }
  return value;
}

/** Extract the entity prefix from any SWP id (for display/routing), or null if malformed. */
export function prefixOf(value: string): string | null {
  const m = value.match(/^(SWP-[A-Z]+)-\d+$/);
  return m?.[1] ?? null;
}
