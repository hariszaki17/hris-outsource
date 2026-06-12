/**
 * E4 · Roster-compliance + week-date helpers (EPICS §8 D1/D3) — shared, pure.
 *
 * Single source of truth for "is this agent over-scheduled / working a holiday"
 * so the schedule grid and the leader-dashboard compliance panel agree.
 *
 * A "worked day" = a scheduled shift that is not a day-off and not cancelled by
 * approved leave. Within a visible Mon–Sun window:
 *   • noRest   → all 7 days worked (zero weekly rest)         → violation
 *   • longRun  → ≥6 consecutive worked days (but has a rest)  → warning
 * Cross-week consecutive runs need neighbour-week data (follow-up); v1 is
 * week-scoped, which is where the leader acts.
 */
import { type ScheduleEntry, ScheduleEntryStatus } from '@swp/api-client/e4';
import type { Holiday } from '@swp/api-client/e7';
import { todayJakartaIso } from '@swp/shared';

const MS_PER_DAY = 86_400_000;

// ---------------------------------------------------------------------------
// Plain-date helpers (Asia/Jakarta), UTC-midnight to avoid TZ drift
// ---------------------------------------------------------------------------

/** Parse "YYYY-MM-DD" into a UTC midnight Date. */
export function parsePlainDate(iso: string): Date {
  const parts = iso.split('-');
  return new Date(Date.UTC(Number(parts[0]), Number(parts[1]) - 1, Number(parts[2])));
}

/** Format a UTC midnight Date to "YYYY-MM-DD". */
export function toIsoDate(d: Date): string {
  return d.toISOString().slice(0, 10);
}

/** Current date in Asia/Jakarta as "YYYY-MM-DD" (canonical helper in @swp/shared). */
export function currentJakartaIso(): string {
  return todayJakartaIso();
}

/** Add N days to a "YYYY-MM-DD" string. */
export function addDays(iso: string, n: number): string {
  return toIsoDate(new Date(parsePlainDate(iso).getTime() + n * MS_PER_DAY));
}

/** Monday of the week containing the given ISO date. */
export function getMondayOfWeek(iso: string): string {
  const d = parsePlainDate(iso);
  const dow = d.getUTCDay(); // 0=Sun..6=Sat
  const delta = dow === 0 ? -6 : 1 - dow;
  return toIsoDate(new Date(d.getTime() + delta * MS_PER_DAY));
}

/** 7 ISO dates Mon→Sun starting from `monday`. */
export function weekDays(monday: string): string[] {
  return Array.from({ length: 7 }, (_, i) => addDays(monday, i));
}

// ---------------------------------------------------------------------------
// Agent rows from raw schedule entries
// ---------------------------------------------------------------------------

export type AgentRow = {
  employeeId: string;
  employeeName: string;
  placementId: string;
  serviceLineId?: string;
  serviceLineName?: string;
  cells: Record<string, ScheduleEntry | undefined>; // date string → entry
};

export function buildAgentRows(entries: ScheduleEntry[]): AgentRow[] {
  const map = new Map<string, AgentRow>();
  for (const e of entries) {
    const key = `${e.employee_id}::${e.placement_id}`;
    if (!map.has(key)) {
      map.set(key, {
        employeeId: e.employee_id,
        employeeName: String(e.employee_name ?? e.employee_id),
        placementId: e.placement_id,
        serviceLineId: e.service_line_id ?? undefined,
        serviceLineName: String(e.company_name ?? ''),
        cells: {},
      });
    }
    const row = map.get(key)!;
    row.cells[e.work_date] = e;
    if (e.service_line_id) row.serviceLineId = e.service_line_id;
  }
  return Array.from(map.values());
}

type RosterPlacement = {
  id: string;
  employee_id: string;
  employee_name?: string;
  service_line_id?: string;
  service_line_name?: string;
};

/**
 * Roster-driven rows: every active placement becomes a row (empty cells if
 * unscheduled), then schedule entries are overlaid. Entries whose placement is
 * NOT in the active roster (e.g. an ended placement with leftover entries) still
 * get a row so nothing scheduled silently disappears.
 */
export function buildAgentRowsFromRoster(
  placements: RosterPlacement[],
  entries: ScheduleEntry[],
): AgentRow[] {
  const map = new Map<string, AgentRow>();
  for (const p of placements) {
    const key = `${p.employee_id}::${p.id}`;
    map.set(key, {
      employeeId: p.employee_id,
      employeeName: String(p.employee_name ?? p.employee_id),
      placementId: p.id,
      serviceLineId: p.service_line_id ?? undefined,
      serviceLineName: p.service_line_name ?? undefined,
      cells: {},
    });
  }
  for (const e of entries) {
    const key = `${e.employee_id}::${e.placement_id}`;
    let row = map.get(key);
    if (!row) {
      row = {
        employeeId: e.employee_id,
        employeeName: String(e.employee_name ?? e.employee_id),
        placementId: e.placement_id,
        serviceLineId: e.service_line_id ?? undefined,
        serviceLineName: undefined,
        cells: {},
      };
      map.set(key, row);
    }
    row.cells[e.work_date] = e;
    if (e.service_line_id) row.serviceLineId = e.service_line_id;
  }
  return Array.from(map.values());
}

// ---------------------------------------------------------------------------
// Holiday scoping (EPICS §8 D1) — global holidays + those matching a company's
// service lines, projected onto the visible week.
// ---------------------------------------------------------------------------

export function buildHolidayMaps(
  holidays: Holiday[],
  days: string[],
  serviceLineIds: Set<string>,
): { holidaySet: Set<string>; holidayNameByDate: Map<string, string> } {
  const nameByDate = new Map<string, string>();
  for (const h of holidays) {
    if (!days.includes(h.date)) continue;
    const applies =
      h.applicable_service_lines.length === 0 ||
      h.applicable_service_lines.some((id) => serviceLineIds.has(id));
    if (!applies) continue;
    if (!nameByDate.has(h.date)) nameByDate.set(h.date, h.name);
  }
  return { holidaySet: new Set(nameByDate.keys()), holidayNameByDate: nameByDate };
}

// ---------------------------------------------------------------------------
// Compliance
// ---------------------------------------------------------------------------

export const REST_VIOLATION_DAYS = 7; // worked all 7 visible days → no weekly rest
export const LONG_RUN_WARN_DAYS = 6; // ≥6 consecutive worked days → approaching cap

type AgentCells = Record<string, ScheduleEntry | undefined>;

export function isWorkedEntry(entry: ScheduleEntry | undefined): boolean {
  if (!entry) return false;
  if (entry.is_day_off) return false;
  if (entry.status === ScheduleEntryStatus.CANCELLED_BY_LEAVE) return false;
  return !!entry.shift_master_name;
}

export function longestWorkedRun(cells: AgentCells, days: string[]): number {
  let best = 0;
  let cur = 0;
  for (const d of days) {
    if (isWorkedEntry(cells[d])) {
      cur += 1;
      if (cur > best) best = cur;
    } else {
      cur = 0;
    }
  }
  return best;
}

export type RowCompliance = {
  workedCount: number;
  longestRun: number;
  noRest: boolean;
  longRun: boolean;
  holidayShiftCount: number;
};

export function computeCompliance(
  cells: AgentCells,
  days: string[],
  holidaySet: Set<string>,
): RowCompliance {
  let workedCount = 0;
  let holidayShiftCount = 0;
  for (const d of days) {
    if (isWorkedEntry(cells[d])) {
      workedCount += 1;
      if (holidaySet.has(d)) holidayShiftCount += 1;
    }
  }
  const longestRun = longestWorkedRun(cells, days);
  const noRest = workedCount >= REST_VIOLATION_DAYS;
  return {
    workedCount,
    longestRun,
    noRest,
    longRun: !noRest && longestRun >= LONG_RUN_WARN_DAYS,
    holidayShiftCount,
  };
}

/** Has at least one compliance signal worth surfacing to the leader. */
export function hasComplianceIssue(c: RowCompliance): boolean {
  return c.noRest || c.longRun || c.holidayShiftCount > 0;
}
