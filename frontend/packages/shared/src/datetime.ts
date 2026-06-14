/**
 * Asia/Jakarta-canonical date/time helpers. CONVENTIONS.md §10 · ENGINEERING.md E4.
 *
 * SWP operates only in Indonesia (WIB, UTC+7) in v1. ALL display/derivation goes through
 * here — never `new Date()` formatting in components. The API sends:
 *   - instants:   RFC 3339 UTC  "2026-06-03T14:32:08Z"
 *   - dates:      "2026-06-03"
 *   - local time: "HH:MM" (always Asia/Jakarta, e.g. shift start_time)
 */
import { Temporal } from '@js-temporal/polyfill';

export const TZ = 'Asia/Jakarta';
export const LOCALE_ID = 'id-ID';

/** Parse an RFC 3339 UTC instant and render it in WIB. */
export function formatInstant(
  iso: string,
  opts: Intl.DateTimeFormatOptions = { dateStyle: 'medium', timeStyle: 'short' },
): string {
  const zoned = Temporal.Instant.from(iso).toZonedDateTimeISO(TZ);
  return new Intl.DateTimeFormat(LOCALE_ID, { ...opts, timeZone: TZ }).format(
    new Date(zoned.epochMilliseconds),
  );
}

/**
 * Render a calendar date with no timezone math. Accepts a plain date ("2026-06-03") OR — for
 * resilience when a full RFC 3339 instant ("2026-06-03T14:32:08Z") is passed by mistake — derives
 * the *Asia/Jakarta calendar date* of that instant (so a UTC late-evening instant maps to the
 * correct WIB day). This avoids the `Temporal.PlainDate.from` "Z designator not supported" throw.
 */
export function formatDate(
  isoDate: string,
  opts: Intl.DateTimeFormatOptions = { dateStyle: 'medium' },
): string {
  const isInstant = isoDate.includes('T');
  const pd = isInstant
    ? Temporal.Instant.from(isoDate).toZonedDateTimeISO(TZ).toPlainDate()
    : Temporal.PlainDate.from(isoDate);
  return new Intl.DateTimeFormat(LOCALE_ID, { ...opts, timeZone: 'UTC' }).format(
    new Date(Date.UTC(pd.year, pd.month - 1, pd.day)),
  );
}

/** Normalize an "HH:MM" Asia/Jakarta local shift time (validates range). */
export function formatLocalTime(hhmm: string): string {
  const t = Temporal.PlainTime.from(hhmm);
  return `${String(t.hour).padStart(2, '0')}:${String(t.minute).padStart(2, '0')}`;
}

/** Current instant as an RFC 3339 UTC string (server is authoritative; use only for client-side UX). */
export function nowIso(): string {
  return Temporal.Now.instant().toString();
}

/**
 * Today's calendar date in Asia/Jakarta as "YYYY-MM-DD".
 *
 * This is the canonical "today" for all date-grid / scheduling derivations — it MUST
 * match how the backend resolves "today" (agent dashboard `today_shift`, attendance,
 * the E4 seed). Near the UTC↔WIB midnight boundary the UTC date and the WIB date
 * diverge by a day; always anchor on WIB so the web grid and the API agree.
 */
export function todayJakartaIso(): string {
  return Temporal.Now.plainDateISO(TZ).toString();
}

/** Whole days between two calendar dates (inclusive of neither end; for ranges). */
export function daysBetween(startIsoDate: string, endIsoDate: string): number {
  return Temporal.PlainDate.from(startIsoDate).until(Temporal.PlainDate.from(endIsoDate)).days;
}

/**
 * An inclusive calendar-date range, both ends "YYYY-MM-DD". `from <= to` always.
 * The canonical shape for E5 attendance `date_from` / `date_to` query params.
 */
export interface DateRange {
  from: string;
  to: string;
}

/** First→last day of the calendar month containing `anchorIsoDate` (default: today WIB). */
export function monthRange(anchorIsoDate: string = todayJakartaIso()): DateRange {
  const pd = Temporal.PlainDate.from(anchorIsoDate);
  const first = pd.with({ day: 1 });
  const last = pd.with({ day: pd.daysInMonth });
  return { from: first.toString(), to: last.toString() };
}

/** First→last day of the month *before* the one containing `anchorIsoDate` (default: today WIB). */
export function prevMonthRange(anchorIsoDate: string = todayJakartaIso()): DateRange {
  return monthRange(Temporal.PlainDate.from(anchorIsoDate).subtract({ months: 1 }).toString());
}

/** Trailing `count` days ending on `anchorIsoDate` inclusive (default: 30 days ending today WIB). */
export function lastNDaysRange(count = 30, anchorIsoDate: string = todayJakartaIso()): DateRange {
  const end = Temporal.PlainDate.from(anchorIsoDate);
  const start = end.subtract({ days: count - 1 });
  return { from: start.toString(), to: end.toString() };
}

/** True when `isoDate` ("YYYY-MM-DD") falls within `range` inclusive. */
export function isWithinRange(isoDate: string, range: DateRange): boolean {
  return isoDate >= range.from && isoDate <= range.to;
}

/**
 * The Asia/Jakarta calendar date ("YYYY-MM-DD") of an RFC 3339 instant OR a plain date.
 * Pass-through for plain dates; for instants, resolves the WIB day (handles the UTC↔WIB
 * midnight boundary). Useful to bucket attendance records by their WIB shift day.
 */
export function jakartaDateOf(iso: string): string {
  return iso.includes('T')
    ? Temporal.Instant.from(iso).toZonedDateTimeISO(TZ).toPlainDate().toString()
    : Temporal.PlainDate.from(iso).toString();
}

/**
 * Human range label, e.g. "1 – 30 Mei 2026" (same month/year collapsed) or
 * "28 Mei – 3 Jun 2026" (cross-month) or "30 Des 2025 – 2 Jan 2026" (cross-year).
 * Bahasa month names via `id-ID`. Matches the .pen DateRange chip + Terapkan button copy.
 */
export function formatRange(range: DateRange): string {
  const a = Temporal.PlainDate.from(range.from);
  const b = Temporal.PlainDate.from(range.to);
  const day = (p: Temporal.PlainDate) => String(p.day);
  const monShort = (p: Temporal.PlainDate) =>
    new Intl.DateTimeFormat(LOCALE_ID, { month: 'short', timeZone: 'UTC' }).format(
      new Date(Date.UTC(p.year, p.month - 1, p.day)),
    );
  if (a.year === b.year && a.month === b.month) {
    return `${day(a)} – ${day(b)} ${monShort(b)} ${b.year}`;
  }
  if (a.year === b.year) {
    return `${day(a)} ${monShort(a)} – ${day(b)} ${monShort(b)} ${b.year}`;
  }
  return `${day(a)} ${monShort(a)} ${a.year} – ${day(b)} ${monShort(b)} ${b.year}`;
}

/** Short range label without year, e.g. "5 – 18 Mei" — for the calendar Terapkan button. */
export function formatRangeShort(range: DateRange): string {
  const a = Temporal.PlainDate.from(range.from);
  const b = Temporal.PlainDate.from(range.to);
  const monShort = (p: Temporal.PlainDate) =>
    new Intl.DateTimeFormat(LOCALE_ID, { month: 'short', timeZone: 'UTC' }).format(
      new Date(Date.UTC(p.year, p.month - 1, p.day)),
    );
  if (a.month === b.month && a.year === b.year) {
    return `${a.day} – ${b.day} ${monShort(b)}`;
  }
  return `${a.day} ${monShort(a)} – ${b.day} ${monShort(b)}`;
}

/** "Mei 2026" month-year title for the calendar nav header. */
export function formatMonthYear(anchorIsoDate: string): string {
  const pd = Temporal.PlainDate.from(anchorIsoDate);
  return new Intl.DateTimeFormat(LOCALE_ID, { month: 'long', year: 'numeric', timeZone: 'UTC' })
    .format(new Date(Date.UTC(pd.year, pd.month - 1, pd.day)))
    .replace(/^./, (c) => c.toUpperCase());
}

/**
 * Monday-start calendar grid for the month containing `anchorIsoDate`, as weeks of 7 cells.
 * Each cell is the "YYYY-MM-DD" date, or `null` for leading/trailing blanks. Matches the
 * .pen calendar (Sen-start, equal-width cells, blanks before day 1).
 */
export function monthGrid(anchorIsoDate: string): (string | null)[][] {
  const pd = Temporal.PlainDate.from(anchorIsoDate);
  const first = pd.with({ day: 1 });
  // Temporal dayOfWeek: Mon=1 … Sun=7 → leading blanks before Monday.
  const lead = first.dayOfWeek - 1;
  const cells: (string | null)[] = [];
  for (let i = 0; i < lead; i++) cells.push(null);
  for (let d = 1; d <= pd.daysInMonth; d++) cells.push(first.with({ day: d }).toString());
  while (cells.length % 7 !== 0) cells.push(null);
  const weeks: (string | null)[][] = [];
  for (let i = 0; i < cells.length; i += 7) weeks.push(cells.slice(i, i + 7));
  return weeks;
}

/** Add `months` to the month anchor (clamped to day 1), as "YYYY-MM-DD". For calendar nav. */
export function shiftMonth(anchorIsoDate: string, months: number): string {
  return Temporal.PlainDate.from(anchorIsoDate).with({ day: 1 }).add({ months }).toString();
}
