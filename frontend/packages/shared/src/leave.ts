/**
 * Leave-domain helpers shared by the web + mobile request forms.
 *
 * Kept dependency-free (primitives in, primitives out) so it does not pull `@swp/api-client`
 * into `@swp/shared`. Callers pass the cap fields off a `LeaveTypeBalance`.
 */

/** A leave type's metering shape, mirrored from `LeaveTypeBalance` (E6 per-type ledger). */
export interface LeaveCap {
  cap_basis: string;
  cap_unit: string;
  cap_value?: number | null;
}

/**
 * cap_basis values that hold a standing, adjustable quota window — mirrors the backend
 * `LeaveTypeCapBasis.QuotaBearing()` (domain/leave/leave.go). PER_EVENT / UNCAPPED meter at
 * request time without a row, so they have no quota HR can add or adjust.
 */
const QUOTA_BEARING_CAP_BASES = new Set([
  'ANNUAL_POOL',
  'PER_MONTH',
  'PER_YEAR_COUNT',
  'LIFETIME_ONCE',
  'SERVICE_UNPAID',
]);

/**
 * Whether a leave type meters against a standing quota window — i.e. HR can add/adjust its
 * entitlement (the "Tambah/Sesuaikan Kuota" flow). False for PER_EVENT / UNCAPPED types, whose
 * days are validated per request with no row to adjust.
 */
export function isQuotaBearing(capBasis: string): boolean {
  return QUOTA_BEARING_CAP_BASES.has(capBasis);
}

/**
 * Reset-cadence i18n key for a leave type's `cap_basis` (leave-entitlement-assignment §7.1/§7.2).
 * `cap_basis` is interpreted purely as a reset cadence now (PRD §4.3). Callers translate the
 * returned key; the actual copy lives in the i18n files (Bahasa default). Returns the key suffix
 * only — callers prefix with their namespace's reset block (e.g. `resetCadence.ANNUAL_POOL`).
 */
export function resetCadenceKey(capBasis: string): string {
  switch (capBasis) {
    case 'ANNUAL_POOL':
      return 'ANNUAL_POOL';
    case 'PER_MONTH':
      return 'PER_MONTH';
    case 'PER_YEAR_COUNT':
      return 'PER_YEAR_COUNT';
    case 'LIFETIME_ONCE':
    case 'SERVICE_UNPAID':
      return 'LIFETIME_ONCE';
    case 'PER_EVENT':
      return 'PER_EVENT';
    default:
      return 'UNCAPPED';
  }
}

/**
 * Statutory fixed-duration leave (UU 13/2003 / PP 35/2021 art. 93): a leave type that grants a
 * FIXED number of days per occurrence — e.g. menikah 3 hari, kematian keluarga inti 2 hari,
 * istri melahirkan 2 hari. Returns that statutory day count (to pre-fill the request range), or
 * `null` when the duration is not fixed:
 *   - `cap_unit === 'COUNT'`  → meters occurrences, not days (e.g. sakit tanpa surat, 5×/tahun)
 *   - `cap_basis === 'ANNUAL_POOL'` → annual pool, agent picks the range freely
 *   - `cap_value` null/≤0     → UNCAPPED / "sesuai ketentuan" (sakit dgn surat, tugas negara)
 */
export function statutoryFixedDays(cap: LeaveCap | null | undefined): number | null {
  if (!cap) return null;
  if (cap.cap_unit !== 'DAYS') return null;
  if (cap.cap_basis === 'ANNUAL_POOL') return null;
  if (cap.cap_value == null || cap.cap_value <= 0) return null;
  return cap.cap_value;
}
