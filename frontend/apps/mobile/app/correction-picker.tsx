/**
 * Pilih Kehadiran (Correction attendance picker) — E5 F5.6.
 *
 * Step 1 of "Ajukan Koreksi": the agent finds the problematic attendance record. Lists the
 * agent's own attendance (useListAttendance — server resolves the principal, no employee_id sent)
 * with a free-text search + date range. Selecting a row hands its id + date to the
 * CHECK_IN/CHECK_OUT form (/correction). A secondary affordance opens the NEW_ENTRY form
 * (/correction?mode=new-entry) for a day with no record at all.
 *
 * Mirrors web correction-attendance-picker.tsx. No dead-flow states: loading / empty / no-results
 * / error all handled. All times via the Asia/Jakarta TZ layer (@swp/shared/datetime).
 */
import { type Attendance, type AttendancePage, useListAttendance } from '@swp/api-client/e5';
import { color } from '@swp/design-tokens';
import { formatInstant } from '@swp/shared/datetime';
import { useRouter } from 'expo-router';
import { CalendarPlus, ChevronLeft, ChevronRight, Search } from 'lucide-react-native';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, Pressable, ScrollView, TextInput, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { Card } from '../src/ui/Card';
import { StatusBadge } from '../src/ui/StatusBadge';
import { Text } from '../src/ui/Text';
import { usePullToRefresh } from '../src/ui/usePullToRefresh';

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

function timeOf(iso?: string | null): string {
  return iso ? formatInstant(iso, { timeStyle: 'short' }) : '—';
}
function dateOf(iso?: string | null): string {
  return iso
    ? new Date(iso).toLocaleDateString('id-ID', {
        weekday: 'short',
        day: 'numeric',
        month: 'short',
        timeZone: 'Asia/Jakarta',
      })
    : '—';
}

function attendanceStatusLabel(t: (k: string) => string, status: string): string {
  return t(`m:attendance.status.${status}`);
}

export default function CorrectionPickerScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const insets = useSafeAreaInsets();
  const refreshControl = usePullToRefresh();

  const [q, setQ] = useState('');
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');

  // Only forward valid YYYY-MM-DD date filters to the server (avoids 400 on partial input).
  const params = {
    limit: 30,
    q: q.trim() || undefined,
    date_from: DATE_RE.test(dateFrom.trim()) ? dateFrom.trim() : undefined,
    date_to: DATE_RE.test(dateTo.trim()) ? dateTo.trim() : undefined,
    sort: 'shift_start_at:desc',
  };

  const query = useListAttendance(params);
  const body = query.data?.data as AttendancePage | undefined;
  const rows: Attendance[] = body?.data ?? [];

  const hasFilters = Boolean(q.trim() || dateFrom.trim() || dateTo.trim());

  function resetFilters() {
    setQ('');
    setDateFrom('');
    setDateTo('');
  }

  function selectRow(r: Attendance) {
    router.push({
      pathname: '/correction',
      params: { attendanceId: r.id, date: r.check_in_at ?? r.shift_start_at ?? '' },
    });
  }

  function newEntry() {
    router.push({ pathname: '/correction', params: { mode: 'new-entry' } });
  }

  const inputClass = 'rounded-input border border-border bg-surface px-3 py-2.5 text-text text-sm';

  return (
    <View className="flex-1 bg-app-bg">
      {/* AppBar */}
      <View
        className="flex-row items-center gap-2 px-4 pb-2"
        style={{ paddingTop: insets.top + 8 }}
      >
        <Pressable onPress={() => router.back()} hitSlop={8}>
          <ChevronLeft size={24} color={color.text} />
        </Pressable>
        <Text variant="screenTitle" className="text-text">
          {t('m:koreksi.pickerTitle')}
        </Text>
      </View>

      <ScrollView
        className="flex-1"
        contentContainerStyle={{ padding: 16, gap: 12 }}
        refreshControl={refreshControl}
      >
        <Text variant="caption" className="text-text-3">
          {t('m:koreksi.pickerSubtitle')}
        </Text>

        {/* Search */}
        <View className="flex-row items-center rounded-input border border-border bg-surface px-3 py-2.5">
          <Search size={16} color={color.text3} />
          <TextInput
            value={q}
            onChangeText={setQ}
            placeholder={t('m:koreksi.pickerSearch')}
            placeholderTextColor={color.text3}
            className="ml-2 flex-1 text-text text-sm"
            style={{ outline: 'none' } as object}
          />
        </View>

        {/* Date range */}
        <View className="flex-row gap-2">
          <View className="flex-1 gap-1">
            <Text variant="caption" weight="semibold" className="text-text-3">
              {t('m:koreksi.pickerDateFrom')}
            </Text>
            <TextInput
              value={dateFrom}
              onChangeText={setDateFrom}
              placeholder="2026-06-01"
              placeholderTextColor={color.text3}
              keyboardType="numbers-and-punctuation"
              className={inputClass}
              style={{ outline: 'none' } as object}
            />
          </View>
          <View className="flex-1 gap-1">
            <Text variant="caption" weight="semibold" className="text-text-3">
              {t('m:koreksi.pickerDateTo')}
            </Text>
            <TextInput
              value={dateTo}
              onChangeText={setDateTo}
              placeholder="2026-06-15"
              placeholderTextColor={color.text3}
              keyboardType="numbers-and-punctuation"
              className={inputClass}
              style={{ outline: 'none' } as object}
            />
          </View>
        </View>

        {hasFilters ? (
          <Pressable onPress={resetFilters} className="self-start">
            <Text variant="strong" className="text-primary">
              {t('m:koreksi.pickerReset')}
            </Text>
          </Pressable>
        ) : null}

        {/* Rows */}
        {query.isLoading ? (
          <View className="items-center py-10">
            <ActivityIndicator />
          </View>
        ) : query.isError ? (
          <Card>
            <Text className="text-danger">{t('m:common.errorGeneric')}</Text>
          </Card>
        ) : rows.length === 0 ? (
          <Card>
            <Text variant="strong" className="text-text">
              {hasFilters ? t('m:koreksi.pickerNoResults') : t('m:koreksi.pickerEmpty')}
            </Text>
            {hasFilters ? (
              <Text variant="caption" className="mt-1 text-text-3">
                {t('m:koreksi.pickerNoResultsBody')}
              </Text>
            ) : null}
          </Card>
        ) : (
          <View className="gap-2">
            {rows.map((r) => (
              <Pressable key={r.id} onPress={() => selectRow(r)}>
                <Card>
                  <View className="flex-row items-center justify-between">
                    <Text variant="strong" className="text-text">
                      {dateOf(r.check_in_at ?? r.shift_start_at)}
                    </Text>
                    <StatusBadge status={r.status} label={attendanceStatusLabel(t, r.status)} />
                  </View>
                  <View className="mt-1 flex-row items-center justify-between">
                    <Text variant="caption" className="text-text-3 tabular-nums">
                      ↓ {timeOf(r.check_in_at)} ↑ {timeOf(r.check_out_at)}
                    </Text>
                    <ChevronRight size={16} color={color.text3} />
                  </View>
                </Card>
              </Pressable>
            ))}
          </View>
        )}

        {/* NEW_ENTRY affordance */}
        <Pressable
          onPress={newEntry}
          className="flex-row items-center justify-center gap-2 rounded-input border border-border bg-surface px-4 py-3"
        >
          <CalendarPlus size={18} color={color.primary} />
          <Text variant="strong" className="text-primary">
            {t('m:koreksi.pickerNewEntry')}
          </Text>
        </Pressable>
      </ScrollView>
    </View>
  );
}
