import type { AttendanceStatus } from '@swp/api-client/e5';
/**
 * StatusFilterChips — single-select status filter row (frames GJI1a / l6UYy · AR-11).
 * "Semua N" acts as the reset; selecting a status chip filters to that one status.
 * Chip: radius 6 · fontSize 12 · padding [6,12] · text-only (no icons) · gap 8 (.pen uses 6).
 *
 * States (1:1 with .pen):
 *  - Semua active: bg primary-soft, border primary, text primary (700).
 *  - status chip active: bg status-bg, border status-border, text status-text (700).
 *  - inactive status chip: bg surface, border, text status-hued (600).
 *  - inactive Semua: bg surface, border, text text-2.
 *
 * `counts` are computed for the CURRENT date range by the caller; a status whose count is 0 and
 * that is not ABSENT/PRESENT/LATE/INCOMPLETE may be hidden (Absen/Cuti only shown when present).
 */
import { useTranslation } from 'react-i18next';
import { ScrollView, View } from 'react-native';
import { Text } from '../../ui/Text';
import { ChipPressable } from './ChipPressable';
import { STATUS_CHIPS, type StatusChipDef, activeChip, inactiveChipText } from './statusChipMeta';

export interface StatusCounts {
  total: number;
  PRESENT: number;
  LATE: number;
  INCOMPLETE: number;
  ABSENT: number;
  ON_LEAVE: number;
}

const baseChip = 'flex-row items-center justify-center rounded-control border';
const chipPad = { paddingVertical: 6, paddingHorizontal: 12 } as const;

export function StatusFilterChips({
  selected,
  counts,
  onSelect,
}: {
  selected: AttendanceStatus | null;
  counts: StatusCounts;
  onSelect: (status: AttendanceStatus | null) => void;
}) {
  const { t } = useTranslation();

  // Always show core statuses; Absen/Cuti only when present in range (per spec).
  const visible: StatusChipDef[] = STATUS_CHIPS.filter((c) => {
    if (c.status === 'ABSENT' || c.status === 'ON_LEAVE') return counts[c.status] > 0;
    return true;
  });

  const semuaActive = selected === null;

  return (
    <ScrollView
      horizontal
      showsHorizontalScrollIndicator={false}
      contentContainerStyle={{ gap: 8 }}
    >
      <ChipPressable
        onPress={() => onSelect(null)}
        className={`${baseChip} ${semuaActive ? 'border-primary bg-primary-soft' : 'border-border bg-surface'}`}
        style={chipPad}
      >
        <Text
          variant="caption"
          weight={semuaActive ? 'bold' : 'semibold'}
          className={semuaActive ? 'text-primary' : 'text-text-2'}
        >
          {t('m:riwayat.chipSemua', { count: counts.total })}
        </Text>
      </ChipPressable>

      {visible.map((c) => {
        const active = selected === c.status;
        const tone = activeChip[c.tone];
        return (
          <ChipPressable
            key={c.status}
            onPress={() => onSelect(c.status)}
            className={`${baseChip} ${active ? `${tone.bg} ${tone.border}` : 'border-border bg-surface'}`}
            style={chipPad}
          >
            <Text
              variant="caption"
              weight={active ? 'bold' : 'semibold'}
              className={active ? tone.text : inactiveChipText[c.tone]}
            >
              {t(c.labelKey, { count: counts[c.status] })}
            </Text>
          </ChipPressable>
        );
      })}
      <View style={{ width: 4 }} />
    </ScrollView>
  );
}
