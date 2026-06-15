/**
 * Ajukan Koreksi (Correction Form) — E5 F5.6
 * Design reference: koreksi-form.png
 *
 * Two modes:
 *   • EXISTING row (CHECK_IN / CHECK_OUT): correct the clock-in/out time on an attendance record.
 *     Opened from the picker (/correction-picker) — receives `attendanceId` + `date`.
 *   • NEW_ENTRY: create a record for a day with none (forgot to clock in/out, or a day with no
 *     shift). Opened from the picker's "Buat untuk tanggal tanpa jadwal" — receives `mode=new-entry`;
 *     the agent fills work_date + jam masuk (+ jam keluar optional).
 *
 * Mirrors web me-correction-screen.tsx. Error codes → i18n:
 *   OUTSIDE_CORRECTION_WINDOW → outsideWindow · CORRECTION_ALREADY_PENDING → alreadyPending
 *   ATTENDANCE_ALREADY_EXISTS → alreadyExists · NO_ACTIVE_PLACEMENT → noPlacement · else error.
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

const TIME_RE = /^([01]\d|2[0-3]):([0-5]\d)$/;
const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

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

/** Asia/Jakarta is a fixed +07:00 offset (no DST) — safe to build the instant directly. */
function buildInstant(dateIso: string, time: string): string {
  return `${dateIso.slice(0, 10)}T${time.trim()}:00+07:00`;
}

// ── Type toggle (existing mode) ────────────────────────────────────────────────

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
  const { attendanceId, date, mode } = useLocalSearchParams<{
    attendanceId?: string;
    date?: string;
    mode?: string;
  }>();

  const isNewEntry = mode === 'new-entry';

  const create = useCreateCorrection();
  const [visibleType, setVisibleType] = useState<VisibleType>('CHECK_OUT');
  const [time, setTime] = useState(''); // existing mode: the single corrected time
  const [workDate, setWorkDate] = useState(''); // new-entry: tanggal kerja
  const [checkIn, setCheckIn] = useState(''); // new-entry: jam masuk
  const [checkOut, setCheckOut] = useState(''); // new-entry: jam keluar (optional)
  const [reason, setReason] = useState('');
  const [err, setErr] = useState<string | null>(null);

  // Resolve the attendance record for "CATATAN ASLI" panel (existing mode only).
  const list = useListAttendance({ limit: 50 }, { query: { enabled: !isNewEntry } });
  const listBody = list.data?.data as AttendancePage | undefined;
  const items: Attendance[] = listBody?.data ?? [];
  const record = items.find((a) => a.id === attendanceId);

  const windowDate = formatWindowDate(7);

  async function onSubmit() {
    setErr(null);

    let data: CorrectionWriteRequest;

    if (isNewEntry) {
      if (!DATE_RE.test(workDate.trim())) {
        setErr(t('m:koreksi.badDate'));
        return;
      }
      if (!TIME_RE.test(checkIn.trim())) {
        setErr(t('m:correction.badTime'));
        return;
      }
      if (checkOut.trim() && !TIME_RE.test(checkOut.trim())) {
        setErr(t('m:correction.badTime'));
        return;
      }
      if (reason.trim().length < 5) {
        setErr(t('m:correction.badReason'));
        return;
      }
      data = {
        type: 'NEW_ENTRY',
        work_date: workDate.trim(),
        reason: reason.trim(),
        proposed_check_in_at: buildInstant(workDate.trim(), checkIn),
        proposed_check_out_at: checkOut.trim()
          ? buildInstant(workDate.trim(), checkOut)
          : undefined,
      };
    } else {
      if (!attendanceId) {
        setErr(t('m:correction.error'));
        return;
      }
      if (visibleType !== 'STATUS' && !TIME_RE.test(time.trim())) {
        setErr(t('m:correction.badTime'));
        return;
      }
      if (reason.trim().length < 5) {
        setErr(t('m:correction.badReason'));
        return;
      }
      const iso = buildInstant(date ?? '', time.trim() || '00:00');
      const apiType: CorrectionType = visibleType === 'STATUS' ? 'CODE' : visibleType;
      data = {
        attendance_id: attendanceId,
        type: apiType,
        reason: reason.trim(),
        proposed_check_in_at: visibleType === 'CHECK_IN' ? iso : undefined,
        proposed_check_out_at: visibleType === 'CHECK_OUT' ? iso : undefined,
      };
    }

    try {
      await create.mutateAsync({ data });
      Alert.alert(t('m:correction.title'), t('m:correction.success'));
      router.back();
    } catch (e) {
      if (e instanceof ApiError) {
        switch (e.code) {
          case 'OUTSIDE_CORRECTION_WINDOW':
          case 'OUTSIDE_WINDOW':
            setErr(t('m:correction.outsideWindow'));
            return;
          case 'CORRECTION_ALREADY_PENDING':
          case 'ALREADY_PENDING':
            setErr(t('m:correction.alreadyPending'));
            return;
          case 'ATTENDANCE_ALREADY_EXISTS':
            setErr(t('m:koreksi.alreadyExists'));
            return;
          case 'NO_ACTIVE_PLACEMENT':
            setErr(t('m:koreksi.noPlacement'));
            return;
        }
      }
      setErr(t('m:correction.error'));
    }
  }

  const inputClass = 'rounded-input border border-border bg-surface px-4 py-3 text-text text-sm';
  const title = isNewEntry ? t('m:koreksi.newEntryTitle') : t('m:correction.title');

  return (
    <View className="flex-1 bg-app-bg">
      {/* AppBar (safe-area top — clears the status bar / Dynamic Island) */}
      <View
        className="flex-row items-center gap-2 px-4 pb-2"
        style={{ paddingTop: insets.top + 8 }}
      >
        <Pressable onPress={() => router.back()} hitSlop={8}>
          <ChevronLeft size={24} color={color.text} />
        </Pressable>
        <Text variant="screenTitle" className="text-text">
          {title}
        </Text>
      </View>
      <ScrollView
        className="flex-1 bg-app-bg"
        contentContainerStyle={{ padding: 16, paddingBottom: 80, gap: 16 }}
      >
        {/* Info banner */}
        <View className="flex-row items-start gap-2 rounded-card border border-info-border bg-info-bg px-3 py-3">
          <Text variant="body" className="flex-1 text-info-text">
            {isNewEntry
              ? t('m:koreksi.newEntryHint')
              : t('m:koreksi.formInfo', { date: windowDate })}
          </Text>
        </View>

        {isNewEntry ? (
          <>
            {/* Tanggal kerja */}
            <View className="gap-1.5">
              <Text variant="caption" weight="semibold" className="text-text-2">
                {t('m:koreksi.newEntryWorkDate')}
              </Text>
              <TextInput
                value={workDate}
                onChangeText={setWorkDate}
                placeholder="2026-06-15"
                placeholderTextColor={color.text3}
                keyboardType="numbers-and-punctuation"
                className={inputClass}
                style={{ outline: 'none' } as object}
              />
            </View>

            {/* Jam masuk + keluar */}
            <View className="flex-row gap-3">
              <View className="flex-1 gap-1.5">
                <Text variant="caption" weight="semibold" className="text-text-2">
                  {t('m:koreksi.newEntryWaktuMasuk')}
                </Text>
                <TextInput
                  value={checkIn}
                  onChangeText={setCheckIn}
                  placeholder="08:00"
                  placeholderTextColor={color.text3}
                  keyboardType="numbers-and-punctuation"
                  className={inputClass}
                  style={{ outline: 'none' } as object}
                />
              </View>
              <View className="flex-1 gap-1.5">
                <Text variant="caption" weight="semibold" className="text-text-2">
                  {t('m:koreksi.newEntryWaktuKeluar')}
                </Text>
                <TextInput
                  value={checkOut}
                  onChangeText={setCheckOut}
                  placeholder="16:00"
                  placeholderTextColor={color.text3}
                  keyboardType="numbers-and-punctuation"
                  className={inputClass}
                  style={{ outline: 'none' } as object}
                />
              </View>
            </View>
          </>
        ) : (
          <>
            {/* Tanggal kehadiran (read-only) */}
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
                    Shift · {timeOf(record.shift_start_at)}–{timeOf(record.shift_end_at)} ·{' '}
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
                  {visibleType === 'CHECK_IN'
                    ? t('m:koreksi.formWaktuCi')
                    : t('m:koreksi.formWaktuCo')}
                </Text>
                <View className="flex-row items-center rounded-input border border-primary bg-surface px-4 py-3">
                  <TextInput
                    value={time}
                    onChangeText={setTime}
                    placeholder="15:10"
                    placeholderTextColor={color.text3}
                    keyboardType="numbers-and-punctuation"
                    className="flex-1 text-text text-sm"
                    style={{ outline: 'none' } as object}
                  />
                  <Text className="text-text-3">⏱</Text>
                </View>
              </View>
            ) : null}
          </>
        )}

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
            placeholderTextColor={color.text3}
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
