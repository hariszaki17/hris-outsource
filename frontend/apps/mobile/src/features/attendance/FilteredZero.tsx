/**
 * FilteredZero — empty state when the active filter (status and/or range) yields no records
 * (design-system comp/EmptyFilteredZero intent · no dead-flow). Offers two recovery CTAs:
 * reset status to "Semua", and widen/change the date range.
 */
import { color } from '@swp/design-tokens';
import { CalendarRange, SearchX } from 'lucide-react-native';
import { useTranslation } from 'react-i18next';
import { Pressable, View } from 'react-native';
import { Text } from '../../ui/Text';

export function FilteredZero({
  showResetStatus,
  onResetStatus,
  onWidenRange,
}: {
  showResetStatus: boolean;
  onResetStatus: () => void;
  onWidenRange: () => void;
}) {
  const { t } = useTranslation();
  return (
    <View
      className="items-center rounded-card border border-border bg-surface px-5 py-8"
      style={{ gap: 6 }}
    >
      <View
        className="items-center justify-center rounded-pill bg-surface-2"
        style={{ width: 48, height: 48, marginBottom: 4 }}
      >
        <SearchX size={22} color={color.text3} />
      </View>
      <Text variant="subtitle" className="text-text">
        {t('m:riwayat.filteredEmptyTitle')}
      </Text>
      <Text variant="caption" className="text-center text-text-2" style={{ marginBottom: 6 }}>
        {t('m:riwayat.filteredEmptyBody')}
      </Text>
      <View className="flex-row items-center" style={{ gap: 8 }}>
        {showResetStatus ? (
          <Pressable
            onPress={onResetStatus}
            className="rounded-control border border-primary bg-primary-soft"
            style={{ paddingVertical: 8, paddingHorizontal: 14 }}
          >
            <Text variant="caption" weight="semibold" className="text-primary">
              {t('m:riwayat.filteredEmptyResetStatus')}
            </Text>
          </Pressable>
        ) : null}
        <Pressable
          onPress={onWidenRange}
          className="flex-row items-center rounded-control border border-border bg-surface"
          style={{ paddingVertical: 8, paddingHorizontal: 14, gap: 6 }}
        >
          <CalendarRange size={14} color={color.text2} />
          <Text variant="caption" weight="semibold" className="text-text-2">
            {t('m:riwayat.filteredEmptyWiden')}
          </Text>
        </Pressable>
      </View>
    </View>
  );
}
