/**
 * E7 Lembur — shared status/tier/source helpers (pre-created so the parallel screen agents
 * share one canonical mapping; G3/G4 — status color only via StatusBadge tones).
 *
 * Tones map to DESIGN-SYSTEM §2 (StatusTone: ok | warn | bad | info | onprogress | neutral).
 */
import { HolidayCategory, OvertimeSource, OvertimeStatus, OvertimeTier } from '@swp/api-client/e7';
import type { StatusTone } from '@swp/design-tokens';

/** OT lifecycle status → tone (DESIGN-SYSTEM §2). */
export function overtimeStatusTone(status: OvertimeStatus): StatusTone {
  switch (status) {
    case OvertimeStatus.APPROVED:
      return 'ok';
    case OvertimeStatus.PENDING_AGENT_CONFIRM:
    case OvertimeStatus.PENDING_L1:
    case OvertimeStatus.PENDING_HR:
      return 'onprogress';
    case OvertimeStatus.REJECTED:
      return 'bad';
    case OvertimeStatus.WITHDRAWN:
      return 'neutral';
    default:
      return 'neutral';
  }
}

/** i18n key for an OT status — relative to the `overtime` namespace (`useTranslation('overtime')`). */
export function overtimeStatusKey(status: OvertimeStatus): string {
  return `status.${status}`;
}

/** Day-type tier → tone. HOLIDAY is the strongest (info/brand-adjacent), WORKDAY neutral. */
export function overtimeTierTone(tier: OvertimeTier): StatusTone {
  switch (tier) {
    case OvertimeTier.HOLIDAY:
      return 'info';
    case OvertimeTier.RESTDAY:
      return 'warn';
    case OvertimeTier.WORKDAY:
      return 'neutral';
    default:
      return 'neutral';
  }
}

export function overtimeTierKey(tier: OvertimeTier): string {
  return `tier.${tier}`;
}

/** Capture path → tone. WORKED_WITHOUT_REQUEST is the flagged path (warn). */
export function overtimeSourceTone(source: OvertimeSource): StatusTone {
  switch (source) {
    case OvertimeSource.REQUESTED:
      return 'neutral';
    case OvertimeSource.AUTO_DETECTED:
      return 'info';
    case OvertimeSource.WORKED_WITHOUT_REQUEST:
      return 'warn';
    default:
      return 'neutral';
  }
}

export function overtimeSourceKey(source: OvertimeSource): string {
  // `source.<X>` is an object { label, short } in i18n (the detail screen also reads `.short`).
  return `source.${source}.label`;
}

export function holidayCategoryTone(category: HolidayCategory): StatusTone {
  switch (category) {
    case HolidayCategory.NATIONAL:
      return 'info';
    case HolidayCategory.REGIONAL:
      return 'warn';
    case HolidayCategory.CUSTOM:
      return 'neutral';
    default:
      return 'neutral';
  }
}

export function holidayCategoryKey(category: HolidayCategory): string {
  return `holidayCategory.${category}`;
}

/** Format counted OT minutes as a compact `Xj Ym` (jam/menit) string. */
export function formatOtMinutes(minutes: number): string {
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  if (h === 0) return `${m}m`;
  if (m === 0) return `${h}j`;
  return `${h}j ${m}m`;
}
