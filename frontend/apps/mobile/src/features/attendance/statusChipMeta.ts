/**
 * Status chip metadata for the Riwayat Kehadiran filter bar (frame GJI1a / l6UYy).
 *
 * Single-select status filter per AR-11: one AttendanceStatus or `null` = "Semua" (reset).
 * Colors mirror the .pen chips exactly — note INCOMPLETE/ABSENT use the `bad` (red) tone on
 * THIS screen's filter chips (the .pen uses $bad-* for "Tidak lengkap"), which differs from the
 * StatusBadge row tone (orange) only for INCOMPLETE. We keep the chip palette 1:1 with the frame.
 */
import type { AttendanceStatus } from '@swp/api-client/e5';

export type ChipTone = 'ok' | 'warn' | 'bad' | 'info';

export interface StatusChipDef {
  /** API enum value sent as the single `status` filter. */
  status: AttendanceStatus;
  /** i18n key under `m:riwayat.*` for the label (carries {{count}}). */
  labelKey: string;
  tone: ChipTone;
}

/** Display order matches the frame: Hadir · Terlambat · Tidak lengkap · (Absen) · (Cuti). */
export const STATUS_CHIPS: StatusChipDef[] = [
  { status: 'PRESENT', labelKey: 'm:riwayat.chipHadir', tone: 'ok' },
  { status: 'LATE', labelKey: 'm:riwayat.chipTerlambat', tone: 'warn' },
  { status: 'INCOMPLETE', labelKey: 'm:riwayat.chipTdkLengkap', tone: 'bad' },
  { status: 'ABSENT', labelKey: 'm:riwayat.chipAbsen', tone: 'bad' },
  { status: 'ON_LEAVE', labelKey: 'm:riwayat.chipCuti', tone: 'info' },
];

/** Tailwind classes per tone for the ACTIVE chip: bg = status-bg, border = status-border. */
export const activeChip: Record<ChipTone, { bg: string; border: string; text: string }> = {
  ok: { bg: 'bg-ok-bg', border: 'border-ok-border', text: 'text-ok-text' },
  warn: { bg: 'bg-warn-bg', border: 'border-warn-border', text: 'text-warn-text' },
  bad: { bg: 'bg-bad-bg', border: 'border-bad-border', text: 'text-bad-text' },
  info: { bg: 'bg-info-bg', border: 'border-info-border', text: 'text-info-text' },
};

/** Inactive status chip: surface bg, neutral border, status-hued text (the .pen `$*-tx` at 600). */
export const inactiveChipText: Record<ChipTone, string> = {
  ok: 'text-ok-text',
  warn: 'text-warn-text',
  bad: 'text-bad-text',
  info: 'text-info-text',
};
