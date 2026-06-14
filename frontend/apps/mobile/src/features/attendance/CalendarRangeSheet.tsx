/**
 * CalendarRangeSheet — custom calendar range picker bottom sheet (frame x2rDk).
 * Title "Pilih rentang tanggal" (15/700). Month nav: ‹ "Mei 2026" › (chevron-left / centered
 * month-year 14/700 / chevron-right). Weekday header Sen…Min (11/600 text-3). Day grid Mon-start,
 * equal-width cells, height 34, day number 13. Range selection: tap start, then end.
 * Endpoints = solid primary circle (white 700); in-between = primary-soft bg (primary number).
 * Bottom: full-width primary "Terapkan · {start} – {end}" button that applies + closes.
 */
import { color } from '@swp/design-tokens';
import {
  type DateRange,
  formatMonthYear,
  formatRangeShort,
  isWithinRange,
  monthGrid,
  shiftMonth,
  todayJakartaIso,
} from '@swp/shared/datetime';
import { ChevronLeft, ChevronRight } from 'lucide-react-native';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Pressable, View } from 'react-native';
import { BottomSheet } from '../../ui/BottomSheet';
import { type FontWeight, Text } from '../../ui/Text';

const WEEKDAY_KEYS = [
  'm:riwayat.wdMon',
  'm:riwayat.wdTue',
  'm:riwayat.wdWed',
  'm:riwayat.wdThu',
  'm:riwayat.wdFri',
  'm:riwayat.wdSat',
  'm:riwayat.wdSun',
];

export function CalendarRangeSheet({
  visible,
  initial,
  onClose,
  onApply,
}: {
  visible: boolean;
  initial: DateRange;
  onClose: () => void;
  onApply: (range: DateRange) => void;
}) {
  const { t } = useTranslation();

  const [monthAnchor, setMonthAnchor] = useState(initial.from);
  // Selection: start set; end null until the second tap completes the range.
  const [start, setStart] = useState<string>(initial.from);
  const [end, setEnd] = useState<string | null>(initial.to);

  function tapDay(iso: string) {
    if (end !== null) {
      // Begin a new range.
      setStart(iso);
      setEnd(null);
      return;
    }
    // Completing the range: order the two endpoints.
    if (iso < start) {
      setEnd(start);
      setStart(iso);
    } else {
      setEnd(iso);
    }
  }

  const selRange: DateRange = { from: start, to: end ?? start };
  const weeks = monthGrid(monthAnchor);
  const today = todayJakartaIso();

  return (
    <BottomSheet visible={visible} onClose={onClose} style={{ gap: 10 }}>
      <Text variant="subtitle" className="text-text">
        {t('m:riwayat.calendarTitle')}
      </Text>

      {/* Month nav */}
      <View className="flex-row items-center gap-3" style={{ paddingVertical: 2 }}>
        <Pressable onPress={() => setMonthAnchor((m) => shiftMonth(m, -1))} hitSlop={8}>
          <ChevronLeft size={18} color={color.text} />
        </Pressable>
        <Text variant="strong" weight="bold" className="flex-1 text-center text-text">
          {formatMonthYear(monthAnchor)}
        </Text>
        <Pressable onPress={() => setMonthAnchor((m) => shiftMonth(m, 1))} hitSlop={8}>
          <ChevronRight size={18} color={color.text} />
        </Pressable>
      </View>

      {/* Weekday header */}
      <View className="flex-row" style={{ gap: 4 }}>
        {WEEKDAY_KEYS.map((k) => (
          <View key={k} className="flex-1 items-center">
            <Text variant="badge" className="text-text-3">
              {t(k)}
            </Text>
          </View>
        ))}
      </View>

      {/* Day grid */}
      {weeks.map((week) => {
        const weekKey = week.find((d) => d !== null) ?? `${monthAnchor}-w`;
        return (
          <View key={weekKey} className="flex-row" style={{ gap: 4 }}>
            {week.map((iso, di) => {
              // Key by the weekday column (stable, unique within the row) — never the index.
              const cellKey = iso ?? `${weekKey}-${WEEKDAY_KEYS[di]}`;
              if (!iso) return <View key={cellKey} className="flex-1" style={{ height: 34 }} />;
              const day = Number(iso.slice(8, 10));
              const isStart = iso === start;
              const isEnd = end !== null && iso === end;
              const isEndpoint = isStart || isEnd;
              const inBetween = end !== null && isWithinRange(iso, selRange) && !isEndpoint;
              const bg = isEndpoint ? color.primary : inBetween ? color.primarySoft : 'transparent';
              const numColor = isEndpoint
                ? 'text-white'
                : inBetween || iso === today
                  ? 'text-primary'
                  : 'text-text';
              const numWeight: FontWeight = isEndpoint
                ? 'bold'
                : iso === today
                  ? 'semibold'
                  : 'medium';
              return (
                <Pressable
                  key={cellKey}
                  onPress={() => tapDay(iso)}
                  className="flex-1 items-center justify-center"
                  style={{ height: 34, borderRadius: 999, backgroundColor: bg }}
                >
                  <Text variant="secondary" weight={numWeight} className={numColor}>
                    {String(day)}
                  </Text>
                </Pressable>
              );
            })}
          </View>
        );
      })}

      {/* Apply */}
      <Pressable
        onPress={() => {
          onApply(selRange);
          onClose();
        }}
        className="flex-row items-center justify-center rounded-card"
        style={{
          backgroundColor: color.primary,
          paddingVertical: 12,
          paddingHorizontal: 20,
          marginTop: 2,
        }}
      >
        <Text variant="subtitle" className="text-white">
          {t('m:riwayat.calendarApply', { range: formatRangeShort(selRange) })}
        </Text>
      </Pressable>
    </BottomSheet>
  );
}
