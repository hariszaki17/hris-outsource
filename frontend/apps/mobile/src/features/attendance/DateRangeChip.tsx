/**
 * DateRangeChip — full-width date-range trigger for Riwayat Kehadiran (frame GJI1a).
 * Surface bg · 1px border · radius 12 · padding [11,14] · gap 12.
 * Left: lucide calendar-range (text-2). Center: range label (14/600, fills). Right: chevron-down (text-3).
 * Tap → opens the presets sheet.
 */
import { color } from '@swp/design-tokens';
import { type DateRange, formatRange } from '@swp/shared/datetime';
import { CalendarRange, ChevronDown } from 'lucide-react-native';
import { Pressable } from 'react-native';
import { Text } from '../../ui/Text';

export function DateRangeChip({ range, onPress }: { range: DateRange; onPress: () => void }) {
  return (
    <Pressable
      onPress={onPress}
      className="flex-row items-center gap-3 rounded-card border border-border bg-surface"
      style={{ paddingVertical: 11, paddingHorizontal: 14 }}
    >
      <CalendarRange size={18} color={color.text2} />
      <Text variant="strong" className="flex-1 text-text">
        {formatRange(range)}
      </Text>
      <ChevronDown size={18} color={color.text3} />
    </Pressable>
  );
}
