/**
 * E8 Payroll — shared status/money helpers (pre-created so the parallel screen agents share one
 * canonical mapping; G3/G4 — status color only via StatusBadge tones).
 *
 * Money is an IDR decimal string (e.g. "7325000.00"), nullable when an encrypted column failed
 * to decrypt (status DECRYPT_FAIL). Never parse as float for display arithmetic — format only.
 */
import { type Money, PayslipStatus } from '@swp/api-client/e8';
import type { StatusTone } from '@swp/design-tokens';

/** Payslip status → tone. FINAL is a calm neutral (read-only archive), DECRYPT_FAIL warns. */
export function payslipStatusTone(status: PayslipStatus): StatusTone {
  switch (status) {
    case PayslipStatus.FINAL:
      return 'neutral';
    case PayslipStatus.DECRYPT_FAIL:
      return 'warn';
    default:
      return 'neutral';
  }
}

export function payslipStatusKey(status: PayslipStatus): string {
  return `status.${status}`;
}

/**
 * Format an IDR Money string for display: `Rp 7.325.000`. Returns the em-dash placeholder for
 * null/undefined (decrypt-fail or omitted). Drops sub-rupiah cents (IDR has no minor unit in use).
 */
export function formatMoney(value: Money | undefined): string {
  if (value == null) return '—';
  const n = Number(value);
  if (!Number.isFinite(n)) return '—';
  return `Rp ${Math.round(n).toLocaleString('id-ID')}`;
}

/** Format a `YYYY-MM` period as `Bulan YYYY` in Bahasa (e.g. "Januari 2026"). */
const MONTH_ID = [
  'Januari',
  'Februari',
  'Maret',
  'April',
  'Mei',
  'Juni',
  'Juli',
  'Agustus',
  'September',
  'Oktober',
  'November',
  'Desember',
];
export function formatPeriod(period: string): string {
  const [y, m] = period.split('-');
  const idx = Number(m) - 1;
  if (idx < 0 || idx > 11) return period;
  return `${MONTH_ID[idx]} ${y}`;
}
