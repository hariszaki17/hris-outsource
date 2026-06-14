/**
 * Ajukan Koreksi (Correction Form) — E5 F5.4
 * Design reference: koreksi-form.png
 *
 * Layout (scrollable):
 *   ← Ajukan Koreksi                    ⓘ
 *   [info banner: 7 days window]
 *   Tanggal kehadiran *     [Sen, 1 Jun 2026]
 *   CATATAN ASLI            [● Auto clock-out]
 *     Masuk 07:02 / Pulang 15:00 (auto)
 *     Shift Pagi · 07:00–15:00 · Plaza Senayan
 *   Jenis koreksi *   [Clock-in][Clock-out][Status]  — tab toggle
 *   Waktu clock-out yang benar *  [15:10 ⏱]
 *   Alasan *          [multi-line text]
 *   Bukti pendukung (opsional)  [📎 filename ×]
 *   ─────────────────────────
 *   [Batal]  [→ Kirim koreksi]
 */
import { ApiError } from '@swp/api-client';
import {
  type Attendance,
  type AttendancePage,
  type CorrectionType,
  type CorrectionWriteRequest,
  useCreateCorrection,
  useListAttendance,
} from '@swp/api-client/e5';
import { color } from '@swp/design-tokens';
import { formatInstant } from '@swp/shared/datetime';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { ChevronLeft } from 'lucide-react-native';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Pressable, ScrollView, TextInput, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { Button } from '../src/ui/Button';
import { Card } from '../src/ui/Card';
import { StatusBadge } from '../src/ui/StatusBadge';
import { Text } from '../src/ui/Text';

// ── Helpers ──────────────────────────────────────────────────────────────────

function timeOf(iso?: string | null): string {
  return iso ? formatInstant(iso, { timeStyle: 'short' }) : '—:—';
}

function formatWindowDate(daysAgo: number): string {
  const d = new Date();
  d.setDate(d.getDate() - daysAgo);
  return d.toLocaleDateString('id-ID', {
    day: 'numeric',
    month: 'long',
    year: 'numeric',
    timeZone: 'Asia/Jakarta',
  });
}

// ── Type toggle ──────────────────────────────────────────────────────────────

type VisibleType = 'CHECK_IN' | 'CHECK_OUT' | 'STATUS';

const TYPE_TABS: { value: VisibleType; label: string }[] = [
  { value: 'CHECK_IN', label: 'Clock-in' },
  { value: 'CHECK_OUT', label: 'Clock-out' },
  { value: 'STATUS', label: 'Status' },
];

// ── Screen ────────────────────────────────────────────────────────────────────

export default function CorrectionForm() {
  const { t } = useTranslation();
  const router = useRouter();
  const insets = useSafeAreaInsets();
  const { attendanceId, date } = useLocalSearchParams<{
    attendanceId: string;
    date: string;
  }>();

  const create = useCreateCorrection();
  const [visibleType, setVisibleType] = useState<VisibleType>('CHECK_OUT');
  const [time, setTime] = useState('');
  const [reason, setReason] = useState('');
  const [err, setErr] = useState<string | null>(null);

  // Resolve the attendance record for "CATATAN ASLI" panel
  const list = useListAttendance({ limit: 50 });
  const listBody = list.data?.data as AttendancePage | undefined;
  const items: Attendance[] = listBody?.data ?? [];
  const record = items.find((a) => a.id === attendanceId);

  const windowDate = formatWindowDate(7);

  async function onSubmit() {
    setErr(null);
    if (!attendanceId) {
      setErr(t('m:correction.error'));
      return;
    }
    if (visibleType !== 'STATUS') {
      if (!/^([01]\d|2[0-3]):([0-5]\d)$/.test(time.trim())) {
        setErr(t('m:correction.badTime'));
        return;
      }
    }
    if (reason.trim().length < 5) {
      setErr(t('m:correction.badReason'));
      return;
    }

    const iso = `${(date ?? '').slice(0, 10)}T${time.trim() || '00:00'}:00+07:00`;
    const apiType: CorrectionType = visibleType === 'STATUS' ? 'CODE' : visibleType;

    const data: CorrectionWriteRequest = {
      attendance_id: attendanceId,
      type: apiType,
      reason: reason.trim(),
      proposed_check_in_at: visibleType === 'CHECK_IN' ? iso : undefined,
      proposed_check_out_at: visibleType === 'CHECK_OUT' ? iso : undefined,
    };

    try {
      await create.mutateAsync({ data });
      Alert.alert(t('m:correction.title'), t('m:correction.success'));
      router.back();
    } catch (e) {
      if (e instanceof ApiError) {
        if (e.code === 'OUTSIDE_CORRECTION_WINDOW') {
          setErr(t('m:correction.outsideWindow'));
          return;
        }
        if (e.code === 'CORRECTION_ALREADY_PENDING') {
          setErr(t('m:correction.alreadyPending'));
          return;
        }
      }
      setErr(t('m:correction.error'));
    }
  }

  const inputClass = 'rounded-input border border-border bg-surface px-4 py-3 text-text text-sm';

  return (
    <View className="flex-1 bg-app-bg">
      {/* AppBar: ← Ajukan Koreksi (safe-area top — clears the status bar / Dynamic Island) */}
      <View
        className="flex-row items-center gap-2 px-4 pb-2"
        style={{ paddingTop: insets.top + 8 }}
      >
        <Pressable onPress={() => router.back()} hitSlop={8}>
          <ChevronLeft size={24} color={color.text} />
        </Pressable>
        <Text variant="screenTitle" className="text-text">
          {t('m:correction.title')}
        </Text>
      </View>
      <ScrollView
        className="flex-1 bg-app-bg"
        contentContainerStyle={{ padding: 16, paddingBottom: 80, gap: 16 }}
      >
        {/* Info banner: 7-day window */}
        <View className="flex-row items-start gap-2 rounded-card border border-info-border bg-info-bg px-3 py-3">
          <Text variant="body" className="flex-1 text-info-text">
            {t('m:koreksi.formInfo', { date: windowDate })}
          </Text>
        </View>

        {/* Tanggal kehadiran */}
        <View className="gap-1.5">
          <Text variant="caption" weight="semibold" className="text-text-2">
            {t('m:koreksi.formDateLabel')}
          </Text>
          <View className="flex-row items-center justify-between rounded-input border border-border bg-surface px-4 py-3">
            <Text variant="body" className="text-text">
              {date
                ? new Date(date).toLocaleDateString('id-ID', {
                    weekday: 'short',
                    day: 'numeric',
                    month: 'long',
                    year: 'numeric',
                    timeZone: 'Asia/Jakarta',
                  })
                : '—'}
            </Text>
            <Text className="text-text-3">📅</Text>
          </View>
        </View>

        {/* CATATAN ASLI panel */}
        {record ? (
          <Card>
            <View className="flex-row items-center justify-between mb-2">
              <Text variant="caption" weight="semibold" className="text-text-3 tracking-wide">
                {t('m:koreksi.formCatatanAsli')}
              </Text>
              {record.auto_closed ? (
                <StatusBadge status="INCOMPLETE" label="Auto clock-out" />
              ) : null}
            </View>
            <View className="flex-row gap-4">
              <View>
                <Text variant="caption" className="text-text-3">
                  Masuk
                </Text>
                <Text variant="cardTitle" className="text-text">
                  {timeOf(record.check_in_at)}
                </Text>
              </View>
              <View>
                <Text variant="caption" className="text-text-3">
                  Pulang
                </Text>
                <Text
                  variant="cardTitle"
                  className={record.auto_closed ? 'text-bad-text' : 'text-text'}
                >
                  {timeOf(record.check_out_at)}
                  {record.auto_closed ? '  (auto)' : ''}
                </Text>
              </View>
            </View>
            {record.shift_start_at ? (
              <Text variant="caption" className="mt-2 text-text-3">
                Shift Pagi · {timeOf(record.shift_start_at)}–{timeOf(record.shift_end_at)} ·{' '}
                {record.site_name ?? record.company_name ?? ''}
              </Text>
            ) : null}
          </Card>
        ) : null}

        {/* Jenis koreksi toggle */}
        <View className="gap-1.5">
          <Text variant="caption" weight="semibold" className="text-text-2">
            {t('m:koreksi.formJenis')}
          </Text>
          <View className="flex-row gap-2">
            {TYPE_TABS.map((tab) => (
              <Pressable
                key={tab.value}
                onPress={() => setVisibleType(tab.value)}
                className={`flex-1 rounded-input border px-3 py-2.5 ${
                  visibleType === tab.value
                    ? 'border-primary bg-primary-soft'
                    : 'border-border bg-surface'
                }`}
              >
                <Text
                  variant="strong"
                  className={`text-center ${
                    visibleType === tab.value ? 'text-primary' : 'text-text-2'
                  }`}
                >
                  {tab.label}
                </Text>
              </Pressable>
            ))}
          </View>
        </View>

        {/* Time field (not shown for Status type) */}
        {visibleType !== 'STATUS' ? (
          <View className="gap-1.5">
            <Text variant="caption" weight="semibold" className="text-text-2">
              {visibleType === 'CHECK_IN' ? t('m:koreksi.formWaktuCi') : t('m:koreksi.formWaktuCo')}
            </Text>
            <View className="flex-row items-center rounded-input border border-primary bg-surface px-4 py-3">
              <TextInput
                value={time}
                onChangeText={setTime}
                placeholder="15:10"
                keyboardType="numbers-and-punctuation"
                className="flex-1 text-text text-sm"
                style={{ outline: 'none' } as object}
              />
              <Text className="text-text-3">⏱</Text>
            </View>
          </View>
        ) : null}

        {/* Alasan */}
        <View className="gap-1.5">
          <Text variant="caption" weight="semibold" className="text-text-2">
            {t('m:koreksi.formAlasan')}
          </Text>
          <TextInput
            value={reason}
            onChangeText={setReason}
            multiline
            numberOfLines={4}
            placeholder={t('m:koreksi.formAlasan')}
            className={`${inputClass} min-h-[80px]`}
            style={{ textAlignVertical: 'top' }}
          />
        </View>

        {/* Bukti pendukung (opsional) — placeholder; file upload deferred */}
        <View className="gap-1.5">
          <Text variant="caption" weight="semibold" className="text-text-2">
            {t('m:koreksi.formBukti')}
          </Text>
          <View className="flex-row items-center rounded-input border border-border bg-surface px-4 py-3">
            <Text variant="body" className="flex-1 text-text-3">
              📎 {t('m:koreksi.formBukti')}
            </Text>
          </View>
        </View>

        {/* Error */}
        {err ? (
          <Text variant="body" className="text-danger">
            {err}
          </Text>
        ) : null}

        {/* Footer buttons */}
        <View className="flex-row gap-3 pt-2">
          <Button
            label={t('m:koreksi.formBatal')}
            variant="secondary"
            onPress={() => router.back()}
            className="flex-1"
          />
          <Button
            label={t('m:koreksi.formKirim')}
            variant="primary"
            onPress={() => void onSubmit()}
            loading={create.isPending}
            className="flex-1"
          />
        </View>
      </ScrollView>
    </View>
  );
}
