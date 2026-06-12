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
