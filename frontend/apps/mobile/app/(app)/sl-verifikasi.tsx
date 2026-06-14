/**
 * SL Persetujuan + Verifikasi Kehadiran (Shift Leader tab).
 *
 * Design reference:
 *   brainstorm.pen DxK66 — E11 · Kotak Masuk Persetujuan (SL approver inbox)
 *   brainstorm.pen viUFF — E11 · Sheet — Setujui / Tolak (approve/reject bottom sheet)
 *   brainstorm.pen fdVo7 — comp/SLMobileNav (routing/tab context; nav is owned by _layout.tsx)
 *   sl-verifikasi-queue.png / sl-detail-verifikasi-sheet.png / sl-tolak-verifikasi-sheet.png /
 *   sl-verifikasi-massal-sheet.png — E5 attendance-verification queue (PRESERVED, see below)
 * Feature: F11.3 (approval inbox) · F11.2 (approval execution) · E5 F5.3 (attendance verification).
 *
 * IA NOTE — two concerns on one tab:
 *   The E11 approver inbox (DxK66) and the E5 attendance-exception verification queue are
 *   distinct concerns. This screen is mounted at the SL tab slot by app/(app)/_layout.tsx (which
 *   we do NOT edit), and leader-beranda's "Verifikasi Kehadiran" action routes here. To honour
 *   DxK66 WITHOUT dropping E5, the screen hosts BOTH as a top segment toggle:
 *     • "Persetujuan" (default) — the E11 approvals inbox (this PRD's surface).
 *     • "Kehadiran"            — the original E5 attendance verification queue, intact.
 *   See the return summary for the flagged routing/tab decision.
 */
import { ApiError } from '@swp/api-client';
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
import {
  type ApprovalInstance,
  type ApproveApprovalInstanceBody,
  type DecisionReason,
  RequestType,
  useApproveApprovalInstance,
  useListApprovalInstances,
  useRejectApprovalInstance,
} from '@swp/api-client/e11';
import { color } from '@swp/design-tokens';
import { formatInstant } from '@swp/shared/datetime';
import { useQueryClient } from '@tanstack/react-query';
import { Check, Inbox } from 'lucide-react-native';
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
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import {
  ApprovalActionSheet,
  type ApprovalSheetMode,
  type ApprovalSheetTarget,
} from '../../src/ui/ApprovalActionSheet';
import { Button } from '../../src/ui/Button';
import { Card } from '../../src/ui/Card';
import { StatusBadge } from '../../src/ui/StatusBadge';
import { Text } from '../../src/ui/Text';

// ── Shared helpers ─────────────────────────────────────────────────────────────

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

// ════════════════════════════════════════════════════════════════════════════════
//  E11 · APPROVALS INBOX (DxK66)
// ════════════════════════════════════════════════════════════════════════════════

/** Best-effort relative "X lalu" caption for the submitted-at instant (Asia/Jakarta date layer). */
function relativeTime(t: (k: string, o?: Record<string, unknown>) => string, iso?: string): string {
  if (!iso) return '';
  const then = new Date(iso).getTime();
  const diffMin = Math.round((Date.now() - then) / 60000);
  if (diffMin < 1) return t('m:approvals.justNow');
  if (diffMin < 60) return t('m:approvals.minsAgo', { count: diffMin });
  const diffHr = Math.round(diffMin / 60);
  if (diffHr < 24) return t('m:approvals.hoursAgo', { count: diffHr });
  const diffDay = Math.round(diffHr / 24);
  if (diffDay === 1) return t('m:approvals.yesterday');
  // Fall back to a WIB-formatted date for older items.
  return formatInstant(iso, { day: 'numeric', month: 'short' });
}

/** Human summary for an instance. Prefer the server `summary`; else derive from type + id. */
function instanceSummary(
  t: (k: string, o?: Record<string, unknown>) => string,
  inst: ApprovalInstance,
): string {
  if (inst.summary) return inst.summary;
  const typeLabel =
    inst.request_type === RequestType.LEAVE
      ? t('m:approvals.typeCuti')
      : t('m:approvals.typeLembur');
  return t('m:approvals.summaryFallback', { type: typeLabel, id: inst.request_id });
}

function requesterName(inst: ApprovalInstance): string {
  // No display_name on the instance; fall back to the requester id (resolver TBD — contract gap).
  return inst.requester_id ?? '—';
}

/** Request-type badge (Cuti = info, Lembur = orange). */
function TypeBadge({ type }: { type: RequestType }) {
  const isLeave = type === RequestType.LEAVE;
  const { t } = useTranslation();
  return (
    <View
      className={`self-start rounded-pill border px-2.5 py-[3px] ${
        isLeave ? 'border-info-border bg-info-bg' : 'border-orange-border bg-orange-bg'
      }`}
    >
      <Text
        variant="badge"
        weight="semibold"
        className={isLeave ? 'text-info-text' : 'text-orange-text'}
      >
        {isLeave ? t('m:approvals.typeCuti') : t('m:approvals.typeLembur')}
      </Text>
    </View>
  );
}

/** One approval request card (DxK66 request card). */
function ApprovalCard({
  inst,
  onApprove,
  onReject,
}: {
  inst: ApprovalInstance;
  onApprove: () => void;
  onReject: () => void;
}) {
  const { t } = useTranslation();
  const name = requesterName(inst);
  const lineCount = Math.max(inst.line_count ?? 1, inst.current_line, 1);

  return (
    <View
      testID={`approval-card-${inst.id}`}
      className="gap-3 rounded-card border border-border bg-surface p-3.5"
    >
      {/* Row A — requester + type badge */}
      <View className="flex-row items-center justify-between">
        <View className="flex-row items-center gap-2.5">
          <View className="h-[34px] w-[34px] items-center justify-center rounded-pill bg-primary-soft">
            <Text variant="caption" weight="bold" className="text-primary">
              {initials(inst.requester_id)}
            </Text>
          </View>
          <View className="gap-0.5">
            <Text variant="subtitle" className="text-text">
              {name}
            </Text>
            <Text variant="badge" weight="regular" mono className="text-text-3">
              {inst.request_id}
            </Text>
          </View>
        </View>
        <TypeBadge type={inst.request_type} />
      </View>

      {/* Row B — summary line */}
      <Text variant="strong" weight="medium" className="text-text">
        {instanceSummary(t, inst)}
      </Text>

      {/* Row C — chain pill + company·time */}
      <View className="flex-row items-center gap-2">
        <View className="self-start rounded-pill border border-warn-border bg-warn-bg px-2.5 py-[3px]">
          <Text variant="badge" weight="bold" className="text-warn-text">
            {t('m:approvals.barisChain', { current: inst.current_line, total: lineCount })}
          </Text>
        </View>
        <Text variant="caption" className="text-text-3">
          {relativeTime(t, inst.created_at)}
        </Text>
      </View>

      {/* Row D — actions */}
      <View className="flex-row items-center gap-2 pt-1">
        <Pressable
          testID={`approval-card-${inst.id}-approve`}
          onPress={onApprove}
          accessibilityRole="button"
          className="flex-1 flex-row items-center justify-center gap-1.5 rounded-input bg-primary py-2.5"
        >
          <Check size={15} color={'#FFFFFF'} />
          <Text variant="strong" weight="bold" style={{ color: '#FFFFFF' }}>
            {t('m:approvals.setujui')}
          </Text>
        </Pressable>
        <Pressable
          testID={`approval-card-${inst.id}-reject`}
          onPress={onReject}
          accessibilityRole="button"
          className="items-center justify-center rounded-input border border-border bg-surface py-2.5"
          style={{ width: 120 }}
        >
          <Text variant="strong" weight="bold" className="text-bad-text">
            {t('m:approvals.tolak')}
          </Text>
        </Pressable>
      </View>
    </View>
  );
}

function ApprovalsSection() {
  const { t } = useTranslation();
  const qc = useQueryClient();

  const [active, setActive] = useState<ApprovalInstance | null>(null);
  const [sheetMode, setSheetMode] = useState<ApprovalSheetMode | null>(null);

  // Inbox = current-line instances where the viewer is a member (server-scoped via mine=true).
  const list = useListApprovalInstances({ mine: true, limit: 50 });
  const items: ApprovalInstance[] = list.data?.data?.data ?? [];

  const approveMut = useApproveApprovalInstance();
  const rejectMut = useRejectApprovalInstance();
  const submitting = approveMut.isPending || rejectMut.isPending;

  async function refetch() {
    await qc.invalidateQueries({ queryKey: ['/approval-instances'] });
  }

  // Map a thrown error to a user-facing toast (handles 403 self-approval, 409 line cleared).
  function alertFor(e: unknown) {
    if (e instanceof ApiError) {
      if (e.status === 403 && e.code === 'SELF_APPROVAL_FORBIDDEN') {
        Alert.alert(t('m:approvals.errTitle'), t('m:approvals.errSelfApproval'));
        return;
      }
      if (e.status === 409 && e.code === 'LINE_ALREADY_CLEARED') {
        Alert.alert(t('m:approvals.errTitle'), t('m:approvals.errLineCleared'));
        return;
      }
    }
    Alert.alert(t('m:approvals.errTitle'), t('m:approvals.errGeneric'));
  }

  async function doApprove(note?: string) {
    if (!active) return;
    try {
      await approveMut.mutateAsync({
        id: active.id,
        data: { note } as ApproveApprovalInstanceBody,
      });
      await refetch();
      setSheetMode(null);
      setActive(null);
      Alert.alert(t('m:approvals.okTitle'), t('m:approvals.okApproved'));
    } catch (e) {
      // 403/409 also drop the item out of the viewer's inbox — refetch to reconcile.
      await refetch();
      setSheetMode(null);
      setActive(null);
      alertFor(e);
    }
  }

  async function doReject(reason: string) {
    if (!active) return;
    try {
      await rejectMut.mutateAsync({ id: active.id, data: { reason } as DecisionReason });
      await refetch();
      setSheetMode(null);
      setActive(null);
      Alert.alert(t('m:approvals.okTitle'), t('m:approvals.okRejected'));
    } catch (e) {
      await refetch();
      setSheetMode(null);
      setActive(null);
      alertFor(e);
    }
  }

  const sheetTarget: ApprovalSheetTarget | null = active
    ? {
        summaryLine: instanceSummary(t, active),
        detailLine: [requesterName(active), active.request_id].filter(Boolean).join(' · '),
        currentLine: active.current_line,
        lineCount: Math.max(active.line_count ?? 1, active.current_line, 1),
      }
    : null;

  return (
    <>
      {list.isLoading ? (
        <View className="items-center py-10">
          <ActivityIndicator />
        </View>
      ) : list.isError ? (
        <Card>
          <Text className="text-danger">{t('m:approvals.loadError')}</Text>
        </Card>
      ) : items.length === 0 ? (
        <View className="items-center gap-2 py-12">
          <Inbox size={28} color={color.text3} />
          <Text variant="strong" weight="semibold" className="text-text-2">
            {t('m:approvals.emptyTitle')}
          </Text>
          <Text variant="caption" className="text-text-3">
            {t('m:approvals.emptyBody')}
          </Text>
        </View>
      ) : (
        <View className="gap-3">
          {items.map((inst) => (
            <ApprovalCard
              key={inst.id}
              inst={inst}
              onApprove={() => {
                setActive(inst);
                setSheetMode('approve');
              }}
              onReject={() => {
                setActive(inst);
                setSheetMode('reject');
              }}
            />
          ))}
        </View>
      )}

      <Modal
        visible={sheetMode !== null && sheetTarget !== null}
        transparent
        animationType="slide"
        onRequestClose={() => {
          setSheetMode(null);
          setActive(null);
        }}
      >
        <Pressable
          className="flex-1 bg-scrim"
          onPress={() => {
            setSheetMode(null);
            setActive(null);
          }}
        />
        <View className="bg-transparent">
          {sheetMode && sheetTarget ? (
            <ApprovalActionSheet
              mode={sheetMode}
              target={sheetTarget}
              submitting={submitting}
              onModeChange={setSheetMode}
              onApprove={(note) => void doApprove(note)}
              onReject={(reason) => void doReject(reason)}
              onClose={() => {
                setSheetMode(null);
                setActive(null);
              }}
            />
          ) : null}
        </View>
      </Modal>
    </>
  );
}

// ════════════════════════════════════════════════════════════════════════════════
//  E5 · ATTENDANCE VERIFICATION (preserved intact)
// ════════════════════════════════════════════════════════════════════════════════

function exceptionLabel(item: Attendance): { label: string; tone: 'warn' | 'bad' | 'info' } {
  if (item.auto_closed) return { label: 'Auto-tutup (lupa CO)', tone: 'bad' };
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

function DetailSheet({
  item,
  onVerify,
  onReject,
}: {
  item: Attendance;
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
      <View className="mb-4 h-1 w-10 self-center rounded-pill bg-border" />

      <ScrollView className="px-4" contentContainerStyle={{ gap: 12 }}>
        <View className="flex-row items-center justify-between">
          <View className="flex-row items-center gap-3">
            <View className="h-10 w-10 items-center justify-center rounded-pill bg-primary-soft">
              <Text variant="strong" weight="bold" className="text-primary">
                {initials(item.employee_name)}
              </Text>
            </View>
            <View>
              <Text variant="body" weight="bold" className="text-text">
                {item.employee_name ?? item.employee_id}
              </Text>
              <Text variant="caption" className="text-text-3">
                {[item.employee_id, item.position, item.site_name ?? item.company_name]
                  .filter(Boolean)
                  .join(' · ')}
              </Text>
            </View>
          </View>
          <StatusBadge status="LATE" label={excLabel} />
        </View>

        <View>
          <Text variant="caption" className="text-text-3">
            {dateLabel(item.check_in_at ?? item.shift_start_at)}
          </Text>
          <Text variant="section" className="mt-0.5">
            {timeOf(item.check_in_at)} → {timeOf(item.check_out_at)}
          </Text>
          <View className="mt-1 flex-row items-center gap-2">
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

        <View
          className={`flex-row items-start gap-2 rounded-card border px-3 py-2.5 ${toneClass.bg} ${toneClass.border}`}
        >
          <Text variant="strong" className={`flex-1 ${toneClass.text}`}>
            {excLabel}
            {item.late_minutes && item.late_minutes > 0 ? ' · perlu keputusan SL (VF-3)' : ''}
          </Text>
        </View>

        <View className="flex-row gap-3">
          <Card className="flex-1">
            <Text variant="caption" className="text-text-3">
              Geofence
            </Text>
            <Text variant="strong" className="mt-1 text-ok-text">
              {item.geofence_in ? `OK · ${item.geofence_in.distance_m}m` : '—'}
            </Text>
          </Card>
          <Card className="flex-1">
            <Text variant="caption" className="text-text-3">
              Foto
            </Text>
            <Text variant="strong" className="mt-1 text-text-2">
              {item.photo_in_id ? 'Terverifikasi' : '—'}
            </Text>
          </Card>
        </View>

        {item.photo_in_id ? (
          <Pressable className="flex-row items-center justify-between rounded-card border border-border bg-surface px-3 py-3">
            <Text variant="body" className="text-text">
              Lihat foto clock-in &amp; clock-out
            </Text>
            <Text variant="body" className="text-text-3">
              ↗
            </Text>
          </Pressable>
        ) : null}

        <View className="items-center justify-center rounded-card border border-ok-border bg-ok-bg py-6">
          <Text variant="strong" className="text-ok-text">
            {item.site_name ?? item.company_name} ·{' '}
            {item.geofence_in
              ? `${item.geofence_in.distance_m}m dari titik geofence`
              : 'Lokasi tidak tersedia'}
          </Text>
        </View>

        <Button label={t('m:slVerif.verifikasi')} variant="primary" onPress={onVerify} />
        <Button label={t('m:slVerif.tolakKoreksi')} variant="secondary" onPress={onReject} />
      </ScrollView>
    </View>
  );
}

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
    <View className="rounded-t-[20px] bg-surface px-4 pb-8 pt-3">
      <View className="mb-4 h-1 w-10 self-center rounded-pill bg-border" />
      <View className="mb-4 flex-row items-center justify-between">
        <View className="flex-row items-center gap-3">
          <View className="h-10 w-10 items-center justify-center rounded-pill bg-bad-bg">
            <Text variant="body" weight="bold" className="text-bad-text">
              ↩
            </Text>
          </View>
          <Text variant="body" weight="bold" className="text-text">
            {t('m:slVerif.tolakTitle')}
          </Text>
        </View>
        <Pressable onPress={onClose}>
          <Text variant="cardTitle" className="text-text-3">
            ×
          </Text>
        </Pressable>
      </View>

      <Text variant="body" className="mb-4 text-text-2">
        {t('m:slVerif.tolakDesc')}
      </Text>

      <View className="mb-3 gap-1.5">
        <Text variant="caption" weight="semibold" className="text-text-2">
          {t('m:slVerif.tolakAlasan')}
        </Text>
        <TextInput
          value={reason}
          onChangeText={setReason}
          multiline
          numberOfLines={4}
          placeholder={t('m:slVerif.tolakPlaceholder')}
          className="min-h-[80px] rounded-input border border-border bg-surface px-4 py-3 text-sm text-text"
          style={{ textAlignVertical: 'top' }}
        />
      </View>

      <Text variant="caption" className="mb-4 text-text-3">
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
    <View className="rounded-t-[20px] bg-surface px-4 pb-8 pt-3">
      <View className="mb-4 h-1 w-10 self-center rounded-pill bg-border" />
      <View className="mb-4 flex-row items-center justify-between">
        <View className="flex-row items-center gap-3">
          <View className="h-10 w-10 items-center justify-center rounded-pill bg-ok-bg">
            <Text variant="body" weight="bold" className="text-ok-text">
              ✓✓
            </Text>
          </View>
          <Text variant="body" weight="bold" className="text-text">
            {t('m:slVerif.massalTitle', { count })}
          </Text>
        </View>
        <Pressable onPress={onClose}>
          <Text variant="cardTitle" className="text-text-3">
            ×
          </Text>
        </Pressable>
      </View>

      <Text variant="body" className="mb-3 text-text">
        <Text variant="body" weight="bold" className="text-primary">
          {count} catatan
        </Text>{' '}
        akan ditandai Terverifikasi.
      </Text>

      <View className="mb-4 flex-row items-center gap-2 rounded-card border border-border bg-surface-2 px-3 py-2">
        <Text variant="body" className="text-text-3">
          ≡
        </Text>
        <Text variant="body" className="flex-1 text-text-2">
          {t('m:slVerif.massalInclude', { desc: description })}
        </Text>
      </View>

      <View className="mb-3 gap-1.5">
        <Text variant="caption" weight="semibold" className="text-text-2">
          {t('m:slVerif.massalCatatan')}
        </Text>
        <TextInput
          value={note}
          onChangeText={setNote}
          multiline
          numberOfLines={3}
          placeholder={t('m:slVerif.massalPlaceholder')}
          className="min-h-[60px] rounded-input border border-border bg-surface px-4 py-3 text-sm text-text"
          style={{ textAlignVertical: 'top' }}
        />
      </View>

      <Text variant="caption" className="mb-4 text-text-3">
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
  const excBorder = {
    warn: 'border-warn-border',
    bad: 'border-bad-border',
    info: 'border-info-border',
  }[tone];

  return (
    <Pressable
      onPress={onPress}
      className={`rounded-card border ${selected ? 'border-primary' : 'border-border'} bg-surface p-3`}
    >
      <View className="flex-row items-center gap-3">
        <Pressable
          onPress={onToggle}
          className={`h-6 w-6 items-center justify-center rounded-control border ${
            selected ? 'border-primary bg-primary' : 'border-border bg-surface'
          }`}
        >
          {selected ? (
            <Text variant="caption" weight="bold" className="text-surface">
              ✓
            </Text>
          ) : null}
        </Pressable>

        <View className="h-9 w-9 items-center justify-center rounded-pill bg-primary-soft">
          <Text variant="caption" weight="bold" className="text-primary">
            {initials(item.employee_name)}
          </Text>
        </View>

        <View className="flex-1">
          <Text variant="body" weight="bold" className="text-text">
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
              <Text variant="caption" weight="semibold" className={excText}>
                {excLabel}
              </Text>
            </View>
          </View>
        </View>

        <Text variant="cardTitle" className="ml-1 text-text-3">
          ›
        </Text>
      </View>
    </Pressable>
  );
}

type AttSheetMode = 'none' | 'detail' | 'reject' | 'massal';

function KehadiranSection() {
  const { t } = useTranslation();
  const qc = useQueryClient();

  const [filter, setFilter] = useState<FilterKey>('ALL');
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [activeItem, setActiveItem] = useState<Attendance | null>(null);
  const [sheetMode, setSheetMode] = useState<AttSheetMode>('none');

  const list = useListAttendance({ limit: 50 });
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
      await rejectMut.mutateAsync({ id, data: { reason } as RejectAttendanceBody });
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
        data: { ids, note: note || undefined } as BulkVerifyAttendanceBody,
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

  const selectedItems = allItems.filter((a) => selected.has(a.id));
  const lateCount = selectedItems.filter((a) => (a.late_minutes ?? 0) > 0).length;
  const massalDesc =
    lateCount > 0 ? `${lateCount} hadir terlambat <25 menit` : `${selected.size} catatan`;

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
      {companyName ? (
        <Text variant="caption" className="text-text-3">
          {companyName} · hanya pengecualian
        </Text>
      ) : null}

      <ScrollView
        horizontal
        showsHorizontalScrollIndicator={false}
        contentContainerStyle={{ gap: 8 }}
        className="-mx-4 px-4"
      >
        {FILTER_TABS.map((f) => {
          const isActive = filter === f.key;
          return (
            <Pressable
              key={f.key}
              onPress={() => setFilter(f.key)}
              className={`rounded-pill border px-3 py-1.5 ${
                isActive ? 'border-primary bg-primary' : 'border-border bg-surface'
              }`}
            >
              <Text variant="strong" className={isActive ? 'text-surface' : 'text-text-2'}>
                {f.label}
              </Text>
            </Pressable>
          );
        })}
      </ScrollView>

      {selected.size > 0 ? (
        <Pressable
          onPress={() => setSheetMode('massal')}
          className="flex-row items-center justify-between rounded-card border border-ok-border bg-ok-bg px-3 py-2.5"
        >
          <View className="flex-row items-center gap-2">
            <Text variant="body" className="text-ok-text">
              ✓✓
            </Text>
            <Text variant="strong" className="text-ok-text">
              {t('m:slVerif.selected', { count: selected.size })}
            </Text>
          </View>
          <View className="flex-row items-center gap-1">
            <Text variant="strong" className="text-ok-text">
              {t('m:slVerif.setujuiMassal')}
            </Text>
            <Text variant="body" className="text-ok-text">
              →
            </Text>
          </View>
        </Pressable>
      ) : null}

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

      <Modal
        visible={sheetMode !== 'none'}
        transparent
        animationType="slide"
        onRequestClose={() => setSheetMode('none')}
      >
        <Pressable className="flex-1 bg-scrim" onPress={() => setSheetMode('none')} />
        <View className="bg-transparent">
          {sheetMode === 'detail' && activeItem ? (
            <DetailSheet
              item={activeItem}
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

// ════════════════════════════════════════════════════════════════════════════════
//  Screen — AppBar (DxK66) + segment toggle (Persetujuan | Kehadiran)
// ════════════════════════════════════════════════════════════════════════════════

type Segment = 'approvals' | 'attendance';

export default function SlVerifikasiScreen() {
  const { t } = useTranslation();
  const insets = useSafeAreaInsets();
  const [segment, setSegment] = useState<Segment>('approvals');

  // Counter pill (DxK66): number of approvals needing action.
  const approvals = useListApprovalInstances({ mine: true, limit: 50 });
  const pendingCount = approvals.data?.data?.data?.length ?? 0;

  return (
    <View className="flex-1 bg-app-bg">
      {/* AppBar — title + warn counter pill (DxK66) */}
      <View
        className="flex-row items-center justify-between border-b border-border bg-surface px-4 pb-3"
        style={{ paddingTop: insets.top + 8 }}
      >
        <Text variant="screenTitle">{t('m:approvals.title')}</Text>
        {segment === 'approvals' && pendingCount > 0 ? (
          <View className="rounded-pill border border-warn-border bg-warn-bg px-2.5 py-[3px]">
            <Text variant="badge" weight="bold" className="text-warn-text">
              {t('m:approvals.needAction', { count: pendingCount })}
            </Text>
          </View>
        ) : null}
      </View>

      {/* Segment toggle — Persetujuan (E11) | Kehadiran (E5) */}
      <View className="flex-row gap-2 border-b border-border bg-surface px-4 pb-3 pt-1">
        {(['approvals', 'attendance'] as Segment[]).map((s) => {
          const isActive = segment === s;
          return (
            <Pressable
              key={s}
              testID={`segment-${s}`}
              onPress={() => setSegment(s)}
              className={`rounded-pill border px-3 py-1.5 ${
                isActive ? 'border-primary bg-primary' : 'border-border bg-surface'
              }`}
            >
              <Text variant="strong" className={isActive ? 'text-surface' : 'text-text-2'}>
                {s === 'approvals' ? t('m:approvals.segApprovals') : t('m:approvals.segAttendance')}
              </Text>
            </Pressable>
          );
        })}
      </View>

      <ScrollView className="flex-1" contentContainerStyle={{ padding: 16, gap: 12 }}>
        {segment === 'approvals' ? <ApprovalsSection /> : <KehadiranSection />}
      </ScrollView>
    </View>
  );
}
