/**
 * DateRangeSheet — date-range presets bottom sheet (frame txgoB).
 * Dim scrim (#18181B66) · sheet surface · top corners radius 16 · padding [12,16,24,16].
 * Title "Rentang tanggal" (15/700). Options as full-width rows with a top border-soft divider:
 *   Bulan ini (selected = primary text + lucide check primary right) · 30 hari terakhir ·
 *   Bulan lalu · Custom… (right lucide chevron-right). Selecting a preset sets+closes;
 *   Custom… opens the calendar sheet.
 */
import { color } from '@swp/design-tokens';
import { type DateRange, lastNDaysRange, monthRange, prevMonthRange } from '@swp/shared/datetime';
import { Check, ChevronRight } from 'lucide-react-native';
import { useTranslation } from 'react-i18next';
import { Pressable } from 'react-native';
import { BottomSheet } from '../../ui/BottomSheet';
import { Text } from '../../ui/Text';

export type PresetId = 'thisMonth' | 'last30' | 'prevMonth' | 'custom';

/** Which preset (if any) the current range corresponds to — drives the check mark. */
export function activePreset(range: DateRange): PresetId | null {
  const eq = (r: DateRange) => r.from === range.from && r.to === range.to;
  if (eq(monthRange())) return 'thisMonth';
  if (eq(lastNDaysRange(30))) return 'last30';
  if (eq(prevMonthRange())) return 'prevMonth';
  return 'custom';
}

export function DateRangeSheet({
  visible,
  current,
  onClose,
  onApply,
  onCustom,
}: {
  visible: boolean;
  current: DateRange;
  onClose: () => void;
  onApply: (range: DateRange) => void;
  onCustom: () => void;
}) {
  const { t } = useTranslation();
  const active = activePreset(current);

  const presets: { id: PresetId; labelKey: string; range?: DateRange }[] = [
    { id: 'thisMonth', labelKey: 'm:riwayat.presetThisMonth', range: monthRange() },
    { id: 'last30', labelKey: 'm:riwayat.presetLast30', range: lastNDaysRange(30) },
    { id: 'prevMonth', labelKey: 'm:riwayat.presetPrevMonth', range: prevMonthRange() },
    { id: 'custom', labelKey: 'm:riwayat.presetCustom' },
  ];

  return (
    <BottomSheet visible={visible} onClose={onClose}>
      <Text variant="subtitle" className="text-text" style={{ paddingBottom: 4 }}>
        {t('m:riwayat.rangeTitle')}
      </Text>

      {presets.map((p) => {
        const selected = active === p.id;
        return (
          <Pressable
            key={p.id}
            onPress={() => {
              if (p.id === 'custom') {
                onCustom();
              } else if (p.range) {
                onApply(p.range);
                onClose();
              }
            }}
            className="flex-row items-center gap-3 border-t border-border-soft"
            style={{ paddingVertical: 14, paddingHorizontal: 16 }}
          >
            <Text
              variant={selected ? 'strong' : 'body'}
              className={`flex-1 ${selected ? 'text-primary' : 'text-text'}`}
            >
              {t(p.labelKey)}
            </Text>
            {selected ? (
              <Check size={18} color={color.primary} />
            ) : p.id === 'custom' ? (
              <ChevronRight size={18} color={color.text3} />
            ) : null}
          </Pressable>
        );
      })}
    </BottomSheet>
  );
}
