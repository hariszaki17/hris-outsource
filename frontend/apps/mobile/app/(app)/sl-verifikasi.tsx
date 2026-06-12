/**
 * SL Verifikasi Kehadiran — E5 F5.3 (Shift Leader)
 * Design reference:
 *   sl-verifikasi-queue.png    — main queue list with filter tabs + bulk-select
 *   sl-detail-verifikasi-sheet.png — bottom sheet: detail + Verifikasi / Tolak buttons
 *   sl-tolak-verifikasi-sheet.png  — bottom sheet: reject reason form
 *   sl-verifikasi-massal-sheet.png — bottom sheet: bulk-approve confirmation
 *
 * Layout:
 *   Verifikasi Kehadiran   🔍  ⚙
 *   [company] · hanya pengecualian
 *   [Semua][Terlambat][Geofence][Auto] — filter pills
 *   [✓ N dipilih · semua serupa   Setujui massal →] — if any selected (green banner)
 *   List of verification rows (exception records only, with checkbox + avatar)
 *
 * Each row has a checkbox; selecting any rows shows the bulk-approve banner.
 * Tapping a row opens the detail bottom-sheet (over scrim).
 *
 * Bottom sheets use Modal + scrim overlay.
 */
import {
  type Attendance,
  type AttendancePage,
  type BulkVerifyAttendanceBody,
  type RejectAttendanceBody,
  type VerifyAttendanceBody,
  useBulkVerifyAttendance,
  useListAttendance,
  useRejectAttendance,
  useVerifyAttendance,
} from '@swp/api-client/e5';
import { formatInstant } from '@swp/shared/datetime';
import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  ActivityIndicator,
  Alert,
  Modal,
  Pressable,
  ScrollView,
  TextInput,
  View,
} from 'react-native';
import { Button } from '../../src/ui/Button';
import { Card } from '../../src/ui/Card';
import { StatusBadge } from '../../src/ui/StatusBadge';
import { Text } from '../../src/ui/Text';

// ── Helpers ──────────────────────────────────────────────────────────────────

function timeOf(iso?: string | null): string {
  if (!iso) return '—:—';
  return new Date(iso).toLocaleTimeString('id-ID', {
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
    timeZone: 'Asia/Jakarta',
  });
}

function dateLabel(iso?: string | null): string {
  if (!iso) return '—';
  return new Date(iso).toLocaleDateString('id-ID', {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
    year: 'numeric',
    timeZone: 'Asia/Jakarta',
  });
}

function initials(name?: string): string {
  if (!name) return '??';
  return name
    .split(' ')
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase() ?? '')
    .join('');
}

// Exception flag label for a row
function exceptionLabel(item: Attendance): { label: string; tone: 'warn' | 'bad' | 'info' } {
  if (item.auto_closed)
    return { label: 'Auto-tutup (lupa CO)', tone: 'bad' };
  const geoFlag = item.flags.includes('OUTSIDE_GEOFENCE');
  if (geoFlag) {
    const dist = item.geofence_in?.distance_m ?? '?';
    return { label: `Di luar geofence (~${dist}m)`, tone: 'bad' };
  }
  if (item.late_minutes && item.late_minutes > 0)
    return { label: `Terlambat ${item.late_minutes}m`, tone: 'warn' };
  return { label: 'Pengecualian', tone: 'info' };
}

type FilterKey = 'ALL' | 'LATE' | 'GEOFENCE' | 'AUTO';

function matchesFilter(item: Attendance, key: FilterKey): boolean {
  if (key === 'ALL') return true;
  if (key === 'LATE') return (item.late_minutes ?? 0) > 0;
  if (key === 'GEOFENCE') return item.flags.includes('OUTSIDE_GEOFENCE');
  if (key === 'AUTO') return item.auto_closed;
  return true;
}

// ── Detail bottom sheet ───────────────────────────────────────────────────────

function DetailSheet({
  item,
  onClose,
  onVerify,
  onReject,
}: {
  item: Attendance;
  onClose: () => void;
  onVerify: () => void;
  onReject: () => void;
}) {
  const { t } = useTranslation();
  const { label: excLabel, tone } = exceptionLabel(item);
  const toneClass = {
    warn: { bg: 'bg-warn-bg', border: 'border-warn-border', text: 'text-warn-text' },
    bad: { bg: 'bg-bad-bg', border: 'border-bad-border', text: 'text-bad-text' },
    info: { bg: 'bg-info-bg', border: 'border-info-border', text: 'text-info-text' },
  }[tone];

  return (
    <View className="rounded-t-[20px] bg-surface pb-8 pt-3">
      {/* Handle */}
      <View className="w-10 h-1 rounded-pill bg-border self-center mb-4" />

      <ScrollView className="px-4" contentContainerStyle={{ gap: 12 }}>
        {/* Employee header */}
        <View className="flex-row items-center justify-between">
          <View className="flex-row items-center gap-3">
            <View className="h-10 w-10 rounded-pill bg-primary-soft items-center justify-center">
              <Text className="font-bold text-primary text-sm">
                {initials(item.employee_name)}
              </Text>
            </View>
            <View>
              <Text variant="body" className="font-bold text-text">
                {item.employee_name ?? item.employee_id}
              </Text>
              <Text variant="caption" className="text-text-3">
                {item.employee_id} · {item.service_line} · {item.site_name ?? item.company_name}
              </Text>
            </View>
          </View>
          <StatusBadge status="LATE" label={excLabel} />
        </View>

        {/* Date + times */}
        <View>
          <Text variant="caption" className="text-text-3">
            {dateLabel(item.check_in_at ?? item.shift_start_at)}
          </Text>
          <Text className="font-bold text-text mt-0.5" style={{ fontSize: 22 }}>
            {timeOf(item.check_in_at)} → {timeOf(item.check_out_at)}
          </Text>
          <View className="flex-row items-center gap-2 mt-1">
            <Text variant="caption" className="text-text-3">
              Shift Pagi · {timeOf(item.shift_start_at)}–{timeOf(item.shift_end_at)}
            </Text>
            <View className="rounded-pill border border-border bg-surface px-2 py-0.5">
              <Text variant="caption" className="text-text-3">
                Pagi
              </Text>
            </View>
          </View>
        </View>

        {/* Exception rule banner */}
        <View className={`flex-row items-start gap-2 rounded-card border px-3 py-2.5 ${toneClass.bg} ${toneClass.border}`}>
          <Text className={`flex-1 text-sm font-semibold ${toneClass.text}`}>
            {excLabel}
            {item.late_minutes && item.late_minutes > 0
              ? ` · perlu keputusan SL (VF-3)`
              : ''}
          </Text>
        </View>

        {/* Geofence + Photo mini tiles */}
        <View className="flex-row gap-3">
          <Card className="flex-1">
            <Text variant="caption" className="text-text-3">
              Geofence
            </Text>
            <Text className="mt-1 font-semibold text-ok-text text-sm">
              {item.geofence_in
                ? `OK · ${item.geofence_in.distance_m}m`
                : '—'}
            </Text>
          </Card>
          <Card className="flex-1">
            <Text variant="caption" className="text-text-3">
              Foto
            </Text>
            <Text className="mt-1 font-semibold text-text-2 text-sm">
              {item.photo_in_id ? 'Terverifikasi' : '—'}
            </Text>
          </Card>
        </View>

        {/* View photos link */}
        {item.photo_in_id ? (
          <Pressable className="flex-row items-center justify-between rounded-card border border-border bg-surface px-3 py-3">
            <Text className="text-text text-sm">Lihat foto clock-in &amp; clock-out</Text>
            <Text className="text-text-3">↗</Text>
          </Pressable>
        ) : null}

        {/* Map placeholder */}
        <View className="rounded-card border border-ok-border bg-ok-bg items-center justify-center py-6">
          <Text className="text-ok-text font-semibold text-sm">
            {item.site_name ?? item.company_name} ·{' '}
            {item.geofence_in ? `${item.geofence_in.distance_m}m dari titik geofence` : 'Lokasi tidak tersedia'}
          </Text>
        </View>

        {/* Action buttons */}
        <Button
          label={t('m:slVerif.verifikasi')}
          variant="primary"
          onPress={onVerify}
        />
        <Button
          label={t('m:slVerif.tolakKoreksi')}
          variant="secondary"
          onPress={onReject}
        />
      </ScrollView>
    </View>
  );
}

// ── Reject bottom sheet ───────────────────────────────────────────────────────

function RejectSheet({
  onClose,
  onSubmit,
  loading,
}: {
  onClose: () => void;
  onSubmit: (reason: string) => void;
  loading: boolean;
}) {
  const { t } = useTranslation();
  const [reason, setReason] = useState('');

  return (
    <View className="rounded-t-[20px] bg-surface pb-8 pt-3 px-4">
      <View className="w-10 h-1 rounded-pill bg-border self-center mb-4" />
      {/* Header */}
      <View className="flex-row items-center justify-between mb-4">
        <View className="flex-row items-center gap-3">
          <View className="h-10 w-10 rounded-pill bg-bad-bg items-center justify-center">
            <Text className="text-bad-text font-bold">↩</Text>
          </View>
          <Text variant="body" className="font-bold text-text">
            {t('m:slVerif.tolakTitle')}
          </Text>
        </View>
        <Pressable onPress={onClose}>
          <Text className="text-text-3 text-lg">×</Text>
        </Pressable>
      </View>

      <Text className="text-text-2 text-sm mb-4">{t('m:slVerif.tolakDesc')}</Text>

      {/* Reason field */}
      <View className="gap-1.5 mb-3">
        <Text variant="caption" className="font-semibold text-text-2">
          {t('m:slVerif.tolakAlasan')}
        </Text>
        <TextInput
          value={reason}
          onChangeText={setReason}
          multiline
          numberOfLines={4}
          placeholder={t('m:slVerif.tolakPlaceholder')}
          className="rounded-input border border-border bg-surface px-4 py-3 text-text text-sm min-h-[80px]"
          style={{ textAlignVertical: 'top' }}
        />
      </View>

      <Text variant="caption" className="text-text-3 mb-4">
        {t('m:slVerif.tolakFootnote')}
      </Text>

      <View className="flex-row gap-3">
        <Button
          label={t('m:slVerif.tolakBatal')}
          variant="secondary"
          onPress={onClose}
          className="flex-1"
        />
        <Button
          label={t('m:slVerif.tolakBtn')}
          variant="danger"
          disabled={reason.trim().length < 3}
          loading={loading}
          onPress={() => onSubmit(reason.trim())}
          className="flex-1"
        />
      </View>
    </View>
  );
}

// ── Bulk-approve bottom sheet ─────────────────────────────────────────────────

function MassalSheet({
  count,
  description,
  onClose,
  onSubmit,
  loading,
}: {
  count: number;
  description: string;
  onClose: () => void;
  onSubmit: (note: string) => void;
  loading: boolean;
}) {
  const { t } = useTranslation();
  const [note, setNote] = useState('');

  return (
    <View className="rounded-t-[20px] bg-surface pb-8 pt-3 px-4">
      <View className="w-10 h-1 rounded-pill bg-border self-center mb-4" />
      {/* Header */}
      <View className="flex-row items-center justify-between mb-4">
        <View className="flex-row items-center gap-3">
          <View className="h-10 w-10 rounded-pill bg-ok-bg items-center justify-center">
            <Text className="text-ok-text font-bold">✓✓</Text>
          </View>
          <Text variant="body" className="font-bold text-text">
            {t('m:slVerif.massalTitle', { count })}
          </Text>
        </View>
        <Pressable onPress={onClose}>
          <Text className="text-text-3 text-lg">×</Text>
        </Pressable>
      </View>

      <Text className="text-text text-sm mb-3">
        <Text className="font-bold text-primary">{count} catatan</Text> akan ditandai Terverifikasi.
      </Text>

      {/* Include summary */}
      <View className="flex-row items-center gap-2 rounded-card border border-border bg-surface-2 px-3 py-2 mb-4">
        <Text className="text-text-3">≡</Text>
        <Text className="flex-1 text-text-2 text-sm">
          {t('m:slVerif.massalInclude', { desc: description })}
        </Text>
      </View>

      {/* Optional note */}
      <View className="gap-1.5 mb-3">
        <Text variant="caption" className="font-semibold text-text-2">
          {t('m:slVerif.massalCatatan')}
        </Text>
        <TextInput
          value={note}
          onChangeText={setNote}
          multiline
          numberOfLines={3}
          placeholder={t('m:slVerif.massalPlaceholder')}
          className="rounded-input border border-border bg-surface px-4 py-3 text-text text-sm min-h-[60px]"
          style={{ textAlignVertical: 'top' }}
        />
      </View>

      <Text variant="caption" className="text-text-3 mb-4">
        {t('m:slVerif.massalFootnote')}
      </Text>

      <View className="flex-row gap-3">
        <Button
          label={t('m:slVerif.massalBatal')}
          variant="secondary"
          onPress={onClose}
          className="flex-1"
        />
        <Button
          label={t('m:slVerif.massalBtn', { count })}
          variant="primary"
          loading={loading}
          onPress={() => onSubmit(note.trim())}
          className="flex-1"
        />
      </View>
    </View>
  );
}

// ── Verification row ──────────────────────────────────────────────────────────

function VerifRow({
  item,
  selected,
  onToggle,
  onPress,
}: {
  item: Attendance;
  selected: boolean;
  onToggle: () => void;
  onPress: () => void;
}) {
  const { label: excLabel, tone } = exceptionLabel(item);
  const excBg = { warn: 'bg-warn-bg', bad: 'bg-bad-bg', info: 'bg-info-bg' }[tone];
  const excText = { warn: 'text-warn-text', bad: 'text-bad-text', info: 'text-info-text' }[tone];
  const excBorder = { warn: 'border-warn-border', bad: 'border-bad-border', info: 'border-info-border' }[tone];

  return (
    <Pressable
      onPress={onPress}
      className={`rounded-card border ${selected ? 'border-primary' : 'border-border'} bg-surface p-3`}
    >
      <View className="flex-row items-center gap-3">
        {/* Checkbox */}
        <Pressable
          onPress={onToggle}
          className={`h-6 w-6 rounded-control border items-center justify-center ${
            selected ? 'border-primary bg-primary' : 'border-border bg-surface'
          }`}
        >
          {selected ? (
            <Text className="text-surface text-xs font-bold">✓</Text>
          ) : null}
        </Pressable>

        {/* Avatar */}
        <View className="h-9 w-9 rounded-pill bg-primary-soft items-center justify-center">
          <Text className="font-bold text-primary text-xs">
            {initials(item.employee_name)}
          </Text>
        </View>

        {/* Content */}
        <View className="flex-1">
          <Text variant="body" className="font-bold text-text">
            {item.employee_name ?? item.employee_id}
          </Text>
          <Text variant="caption" className="text-text-3">
            {item.shift_start_at
              ? `Pagi ${timeOf(item.shift_start_at)}–${timeOf(item.shift_end_at)}`
              : '—'}{' '}
            · CI {timeOf(item.check_in_at)} · CO {timeOf(item.check_out_at)}
          </Text>
          <View className="mt-1.5 self-start">
            <View
              className={`flex-row items-center gap-1 rounded-pill border px-2 py-0.5 ${excBg} ${excBorder}`}
            >
              <Text className={`text-xs font-semibold ${excText}`}>{excLabel}</Text>
            </View>
          </View>
        </View>

        <Text className="text-text-3 text-lg ml-1">›</Text>
      </View>
    </Pressable>
  );
}

// ── Screen ────────────────────────────────────────────────────────────────────

type SheetMode = 'none' | 'detail' | 'reject' | 'massal';

export default function SlVerifikasiScreen() {
  const { t } = useTranslation();
  const qc = useQueryClient();

  const [filter, setFilter] = useState<FilterKey>('ALL');
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [activeItem, setActiveItem] = useState<Attendance | null>(null);
  const [sheetMode, setSheetMode] = useState<SheetMode>('none');

  const list = useListAttendance({
    limit: 50,
    // Only fetch exception (PENDING verification) records — server filters by verification_status
  });
  const body = list.data?.data as AttendancePage | undefined;
  const allItems: Attendance[] = (body?.data ?? []).filter(
    (a) => a.verification_status === 'PENDING' || a.verification_status === 'ESCALATED',
  );

  const filtered = allItems.filter((a) => matchesFilter(a, filter));

  const verifyMut = useVerifyAttendance();
  const rejectMut = useRejectAttendance();
  const bulkVerifyMut = useBulkVerifyAttendance();

  async function doVerify(id: string) {
    try {
      await verifyMut.mutateAsync({ id, data: {} as VerifyAttendanceBody });
      await qc.invalidateQueries({ queryKey: ['/attendance'] });
      setSheetMode('none');
      setActiveItem(null);
      Alert.alert('Berhasil', 'Kehadiran diverifikasi.');
    } catch {
      Alert.alert('Gagal', 'Tidak dapat memverifikasi. Coba lagi.');
    }
  }

  async function doReject(id: string, reason: string) {
    try {
      await rejectMut.mutateAsync({
        id,
        data: { reason } as RejectAttendanceBody,
      });
      await qc.invalidateQueries({ queryKey: ['/attendance'] });
      setSheetMode('none');
      setActiveItem(null);
      Alert.alert('Berhasil', 'Kehadiran ditolak.');
    } catch {
      Alert.alert('Gagal', 'Tidak dapat menolak. Coba lagi.');
    }
  }

  async function doBulkVerify(note: string) {
    const ids = Array.from(selected);
    try {
      await bulkVerifyMut.mutateAsync({
        data: {
          ids,
          note: note || undefined,
        } as BulkVerifyAttendanceBody,
      });
      await qc.invalidateQueries({ queryKey: ['/attendance'] });
      setSelected(new Set());
      setSheetMode('none');
      Alert.alert('Berhasil', `${ids.length} kehadiran diverifikasi.`);
    } catch {
      Alert.alert('Gagal', 'Tidak dapat memverifikasi massal. Coba lagi.');
    }
  }

  function toggleSelect(id: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  // Derive bulk-approve description
  const selectedItems = allItems.filter((a) => selected.has(a.id));
  const lateCount = selectedItems.filter((a) => (a.late_minutes ?? 0) > 0).length;
  const massalDesc = lateCount > 0
    ? `${lateCount} hadir terlambat <25 menit`
    : `${selected.size} catatan`;

  // Company context (from first item)
  const companyName = allItems[0]?.company_name ?? allItems[0]?.site_name ?? '';

  const FILTER_TABS: { key: FilterKey; label: string }[] = [
    { key: 'ALL', label: `Semua ${allItems.length}` },
    {
      key: 'LATE',
      label: `Terlambat ${allItems.filter((a) => (a.late_minutes ?? 0) > 0).length}`,
    },
    {
      key: 'GEOFENCE',
      label: `Geofence ${allItems.filter((a) => a.flags.includes('OUTSIDE_GEOFENCE')).length}`,
    },
    { key: 'AUTO', label: `Auto ${allItems.filter((a) => a.auto_closed).length}` },
  ];

  return (
    <>
      <ScrollView
        className="flex-1 bg-app-bg"
        contentContainerStyle={{ padding: 16, gap: 12 }}
      >
        {/* Subtitle */}
        {companyName ? (
          <Text variant="caption" className="text-text-3">
            {companyName} · hanya pengecualian
          </Text>
        ) : null}

        {/* Filter tabs */}
        <ScrollView
          horizontal
          showsHorizontalScrollIndicator={false}
          contentContainerStyle={{ gap: 8 }}
          className="-mx-4 px-4"
        >
          {FILTER_TABS.map((f) => {
            const active = filter === f.key;
            return (
              <Pressable
                key={f.key}
                onPress={() => setFilter(f.key)}
                className={`rounded-pill border px-3 py-1.5 ${
                  active ? 'border-primary bg-primary' : 'border-border bg-surface'
                }`}
              >
                <Text
                  className={`text-sm font-semibold ${
                    active ? 'text-surface' : 'text-text-2'
                  }`}
                >
                  {f.label}
                </Text>
              </Pressable>
            );
          })}
        </ScrollView>

        {/* Bulk-approve banner */}
        {selected.size > 0 ? (
          <Pressable
            onPress={() => setSheetMode('massal')}
            className="flex-row items-center justify-between rounded-card border border-ok-border bg-ok-bg px-3 py-2.5"
          >
            <View className="flex-row items-center gap-2">
              <Text className="text-ok-text">✓✓</Text>
              <Text className="text-ok-text text-sm font-semibold">
                {t('m:slVerif.selected', { count: selected.size })}
              </Text>
            </View>
            <View className="flex-row items-center gap-1">
              <Text className="text-ok-text text-sm font-semibold">
                {t('m:slVerif.setujuiMassal')}
              </Text>
              <Text className="text-ok-text">→</Text>
            </View>
          </Pressable>
        ) : null}

        {/* List */}
        {list.isLoading ? (
          <View className="items-center py-10">
            <ActivityIndicator />
          </View>
        ) : filtered.length === 0 ? (
          <Card>
            <Text variant="caption" className="text-text-3">
              {t('m:slVerif.empty')}
            </Text>
          </Card>
        ) : (
          <View className="gap-3">
            {filtered.map((item) => (
              <VerifRow
                key={item.id}
                item={item}
                selected={selected.has(item.id)}
                onToggle={() => toggleSelect(item.id)}
                onPress={() => {
                  setActiveItem(item);
                  setSheetMode('detail');
                }}
              />
            ))}
          </View>
        )}
      </ScrollView>

      {/* ── Bottom sheet modal ─────────────────────────────────────────────── */}
      <Modal
        visible={sheetMode !== 'none'}
        transparent
        animationType="slide"
        onRequestClose={() => setSheetMode('none')}
      >
        {/* Scrim */}
        <Pressable
          className="flex-1 bg-scrim"
          onPress={() => setSheetMode('none')}
        />
        {/* Sheet content */}
        <View className="bg-transparent">
          {sheetMode === 'detail' && activeItem ? (
            <DetailSheet
              item={activeItem}
              onClose={() => setSheetMode('none')}
              onVerify={() => void doVerify(activeItem.id)}
              onReject={() => setSheetMode('reject')}
            />
          ) : sheetMode === 'reject' && activeItem ? (
            <RejectSheet
              onClose={() => setSheetMode('detail')}
              onSubmit={(reason) => void doReject(activeItem.id, reason)}
              loading={rejectMut.isPending}
            />
          ) : sheetMode === 'massal' ? (
            <MassalSheet
              count={selected.size}
              description={massalDesc}
              onClose={() => setSheetMode('none')}
              onSubmit={(note) => void doBulkVerify(note)}
              loading={bulkVerifyMut.isPending}
            />
          ) : null}
        </View>
      </Modal>
    </>
  );
}
