/**
 * Detail Koreksi (Correction Detail) — E5 F5.6
 * Design reference: koreksi-detail.png
 *
 * Shows the proposed change (before → after where the snapshot is available), the agent's reason,
 * the live approval chain (followed via `approval_instance_id` → useGetApprovalInstance, rendered
 * with the shared ApprovalChain primitive), a payable indicator for applied NEW_ENTRY days, and a
 * withdraw action while still PENDING (useCancelCorrection).
 *
 * Mirrors web me-correction-status-modal.tsx. Corrections are approved in the E11 inbox (HR/SL),
 * NOT here — this screen is read-only except for the agent's own withdraw. No dead-flow states.
 */
import {
  type Correction,
  CorrectionStatus,
  CorrectionType,
  type GetCorrection200,
  useCancelCorrection,
  useGetCorrection,
} from '@swp/api-client/e5';
import {
  ApprovalActionAction,
  type ApprovalInstanceDetail,
  type ApprovalLine,
  InstanceStatus,
  useGetApprovalInstance,
} from '@swp/api-client/e11';
import { color } from '@swp/design-tokens';
import { formatInstant } from '@swp/shared/datetime';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { ChevronLeft } from 'lucide-react-native';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, Alert, ScrollView, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { ApprovalChain, type ApprovalChainStep, type LineState } from '../src/ui/ApprovalChain';
import { Button } from '../src/ui/Button';
import { Card } from '../src/ui/Card';
import { StatusBadge } from '../src/ui/StatusBadge';
import { Text } from '../src/ui/Text';

type TFn = (key: string, opts?: Record<string, unknown>) => string;

// ── Helpers ──────────────────────────────────────────────────────────────────

function timeLabel(iso?: string | null): string {
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
    weekday: 'short',
    day: 'numeric',
    month: 'long',
    year: 'numeric',
    timeZone: 'Asia/Jakarta',
  });
}

function fmtEvent(iso?: string): string {
  if (!iso) return '';
  return formatInstant(iso, { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
}

function correctionTypeTitle(type: string): string {
  switch (type) {
    case 'CHECK_IN':
      return 'Koreksi Clock-in';
    case 'CHECK_OUT':
      return 'Koreksi Clock-out';
    case 'CODE':
      return 'Koreksi Status';
    case 'NEW_ENTRY':
      return 'Buat Catatan Kehadiran';
    default:
      return 'Koreksi Lainnya';
  }
}

function corrStatusBadge(status: string): { bStatus: string; label: string } {
  switch (status) {
    case 'PENDING':
      return { bStatus: 'LATE', label: 'Pending' };
    case 'APPROVED':
    case 'APPLIED':
      return { bStatus: 'PRESENT', label: 'Disetujui' };
    case 'REJECTED':
      return { bStatus: 'ABSENT', label: 'Ditolak' };
    case 'CANCELLED':
      return { bStatus: 'ABSENT', label: 'Dibatalkan' };
    default:
      return { bStatus: 'INCOMPLETE', label: status };
  }
}

// ── Approval-chain derivation (mirrors approval-status.tsx buildSteps) ──────────

function memberNames(line: ApprovalLine | undefined): string {
  if (!line || line.members.length === 0) return '';
  return line.members.map((m) => m.display_name ?? m.user_id).join(' · ');
}

function buildSteps(detail: ApprovalInstanceDetail, t: TFn): ApprovalChainStep[] {
  const lines = [...(detail.lines ?? [])].sort((a, b) => a.line_no - b.line_no);
  const actions = detail.actions ?? [];
  const current = detail.current_line;
  const rejected = detail.status === InstanceStatus.REJECTED;
  const approved = detail.status === InstanceStatus.APPROVED;

  return lines.map((line) => {
    const act = actions
      .filter((a) => a.line_no === line.line_no)
      .sort((a, b) => (a.created_at < b.created_at ? 1 : -1))[0];
    const members = memberNames(line);
    const actorName = act
      ? (act.actor_name ??
        line.members.find((m) => m.user_id === act.actor_user_id)?.display_name ??
        act.actor_user_id)
      : '';

    if (rejected && act && act.action === ApprovalActionAction.REJECT) {
      return {
        lineNo: line.line_no,
        state: 'rejected' satisfies LineState,
        members,
        statusLabel: t('m:approvalStatus.lineRejected', { defaultValue: 'Ditolak' }),
        result: t('m:approvalStatus.resultRejected', {
          defaultValue: 'Ditolak oleh {{actor}} · {{time}}{{reason}}',
          actor: actorName,
          time: fmtEvent(act.created_at),
          reason: act.reason ? ` — ${act.reason}` : '',
        }),
      };
    }

    const cleared = !!act && act.action !== ApprovalActionAction.REJECT;
    const isDone = cleared || (approved && line.line_no <= (detail.line_count ?? current));
    if (isDone) {
      const isBypass = act?.action === ApprovalActionAction.BYPASS;
      const result = isBypass
        ? t('m:approvalStatus.resultBypass', {
            defaultValue: 'Disahkan oleh {{actor}} · {{time}} (bypass)',
            actor: actorName,
            time: fmtEvent(act?.created_at),
          })
        : act
          ? t('m:approvalStatus.resultApproved', {
              defaultValue: 'Disetujui oleh {{actor}} · {{time}} (OR)',
              actor: actorName,
              time: fmtEvent(act.created_at),
            })
          : t('m:approvalStatus.resultDone', { defaultValue: 'Selesai.' });
      return {
        lineNo: line.line_no,
        state: 'done' satisfies LineState,
        members,
        statusLabel: t('m:approvalStatus.lineDone', { defaultValue: 'Selesai' }),
        result,
      };
    }

    if (!rejected && !approved && line.line_no === current) {
      return {
        lineNo: line.line_no,
        state: 'current' satisfies LineState,
        members,
        statusLabel: t('m:approvalStatus.lineWaiting', { defaultValue: 'Menunggu' }),
        result: t('m:approvalStatus.resultCurrent', { defaultValue: 'Menunggu keputusan.' }),
      };
    }

    return {
      lineNo: line.line_no,
      state: 'upcoming' satisfies LineState,
      members,
      statusLabel: t('m:approvalStatus.lineWaiting', { defaultValue: 'Menunggu' }),
      result: t('m:approvalStatus.resultUpcoming', { defaultValue: 'Belum tercapai.' }),
    };
  });
}

// ── Screen ────────────────────────────────────────────────────────────────────

export default function CorrectionDetailScreen() {
  const { t } = useTranslation() as { t: TFn };
  const router = useRouter();
  const insets = useSafeAreaInsets();
  const { id } = useLocalSearchParams<{ id: string }>();

  const query = useGetCorrection(id ?? '');
  const resp = query.data?.data as GetCorrection200 | undefined;
  const item: Correction | undefined = resp?.data;

  const status = (item?.status ?? '') as CorrectionStatus;
  const isPending = status === CorrectionStatus.PENDING;
  const instanceId = item?.approval_instance_id ?? '';

  // Follow the E11 approval instance for the live chain (any non-terminal/terminal state).
  const instanceQ = useGetApprovalInstance(instanceId, {
    query: { enabled: Boolean(instanceId) },
  });
  const raw = instanceQ.data?.data;
  const instance: ApprovalInstanceDetail | undefined =
    raw && 'lines' in (raw as object) ? (raw as ApprovalInstanceDetail) : undefined;

  const cancel = useCancelCorrection();

  function onWithdraw() {
    if (!item) return;
    cancel.mutate(
      { id: item.id },
      {
        onSuccess: () => {
          Alert.alert(t('m:koreksi.detailTitle'), t('m:koreksi.withdrawn'));
          router.back();
        },
        onError: () => {
          Alert.alert(t('m:koreksi.detailTitle'), t('m:koreksi.withdrawError'));
        },
      },
    );
  }

  if (query.isLoading) {
    return (
      <View className="flex-1 items-center justify-center bg-app-bg">
        <ActivityIndicator />
      </View>
    );
  }

  if (!item) {
    return (
      <View className="flex-1 items-center justify-center bg-app-bg p-6">
        <Text className="text-danger">{t('m:common.errorGeneric')}</Text>
      </View>
    );
  }

  const { bStatus, label: bLabel } = corrStatusBadge(item.status);
  const isRejected = item.status === 'REJECTED';
  const isNewEntry = item.type === CorrectionType.NEW_ENTRY;

  // Proposed-change diff values.
  const snap = item.original_snapshot as Record<string, string | null | undefined>;
  const origTime =
    item.type === 'CHECK_IN'
      ? timeLabel(snap?.check_in_at)
      : item.type === 'CHECK_OUT'
        ? timeLabel(snap?.check_out_at)
        : '—';
  const propTime =
    item.type === 'CHECK_IN'
      ? timeLabel(item.proposed_check_in_at)
      : item.type === 'CHECK_OUT'
        ? timeLabel(item.proposed_check_out_at)
        : '—';

  const attendanceDateIso =
    item.work_date ??
    snap?.check_in_at ??
    item.proposed_check_in_at ??
    item.proposed_check_out_at ??
    item.created_at;

  // Applied NEW_ENTRY / no-shift days await an HR/SL payable decision (Attendance.is_payable=null).
  const showPayablePending = isNewEntry && item.status === CorrectionStatus.APPLIED;

  const steps = instance ? buildSteps(instance, t) : [];

  return (
    <View className="flex-1 bg-app-bg">
      {/* AppBar */}
      <View
        className="flex-row items-center gap-2 px-4 pb-2"
        style={{ paddingTop: insets.top + 8 }}
      >
        <ChevronLeft size={24} color={color.text} onPress={() => router.back()} />
        <Text variant="screenTitle" className="text-text">
          {t('m:koreksi.detailTitle')}
        </Text>
      </View>

      <ScrollView
        className="flex-1"
        contentContainerStyle={{ padding: 16, paddingBottom: 100, gap: 16 }}
      >
        {/* Header: ID + type + status */}
        <View>
          <Text variant="caption" mono className="text-text-3">
            {item.id}
          </Text>
          <View className="mt-1 flex-row items-center justify-between">
            <Text variant="title" className="text-text">
              {correctionTypeTitle(item.type)}
            </Text>
            <StatusBadge status={bStatus} label={bLabel} />
          </View>
          {showPayablePending ? (
            <View className="mt-2 self-start rounded-pill bg-info-bg px-2 py-1">
              <Text variant="caption" weight="semibold" className="text-info-text">
                {t('m:koreksi.payablePending')}
              </Text>
            </View>
          ) : null}
        </View>

        {/* PERUBAHAN DIAJUKAN */}
        <Card>
          <Text variant="caption" weight="semibold" className="text-text-3 tracking-wide mb-2">
            {t('m:koreksi.perubahan')}
          </Text>
          <Text variant="caption" className="text-text-2 mb-3">
            {dateLabel(attendanceDateIso)}
            {snap?.site_name ? ` · ${snap.site_name}` : ''}
          </Text>

          {isNewEntry ? (
            /* NEW_ENTRY: there is no "before" — show the proposed in/out times. */
            <View className="flex-row gap-3">
              <View className="flex-1 rounded-card border border-ok-border bg-ok-bg items-center py-3">
                <Text variant="caption" weight="semibold" className="text-ok-text mb-1">
                  {t('m:koreksi.labelMasuk')}
                </Text>
                <Text variant="metric" className="text-ok-text">
                  {timeLabel(item.proposed_check_in_at)}
                </Text>
              </View>
              <View className="flex-1 rounded-card border border-border bg-surface items-center py-3">
                <Text variant="caption" weight="semibold" className="text-text-3 mb-1">
                  {t('m:koreksi.labelKeluar')}
                </Text>
                <Text variant="metric" className="text-text">
                  {item.proposed_check_out_at ? timeLabel(item.proposed_check_out_at) : '—'}
                </Text>
              </View>
            </View>
          ) : (
            <View className="flex-row items-center gap-3">
              <View className="flex-1 rounded-card border border-bad-border bg-bad-bg items-center py-3">
                <Text variant="caption" weight="semibold" className="text-bad-text mb-1">
                  {t('m:koreksi.asli')}
                </Text>
                <Text variant="metric" className="text-bad-text">
                  {origTime}
                </Text>
              </View>
              <Text variant="cardTitle" className="text-text-3">
                →
              </Text>
              <View className="flex-1 rounded-card border border-ok-border bg-ok-bg items-center py-3">
                <Text variant="caption" weight="semibold" className="text-ok-text mb-1">
                  {t('m:koreksi.diajukan')}
                </Text>
                <Text variant="metric" className="text-ok-text">
                  {propTime}
                </Text>
              </View>
            </View>
          )}
        </Card>

        {/* ALASAN ANDA */}
        <Card>
          <Text variant="caption" weight="semibold" className="text-text-3 tracking-wide mb-2">
            {t('m:koreksi.alasanAnda')}
          </Text>
          <Text variant="body" className="text-text">
            {item.reason}
          </Text>
        </Card>

        {/* Rantai Persetujuan — live chain via approval_instance_id */}
        <Card>
          <Text variant="subtitle" className="mb-1">
            {t('m:koreksi.chainHeader')}
          </Text>
          {instanceQ.isLoading ? (
            <View className="items-center py-4">
              <ActivityIndicator />
            </View>
          ) : steps.length > 0 ? (
            <ApprovalChain steps={steps} />
          ) : (
            <Text variant="secondary" className="pt-1 text-text-2">
              {t('m:approvalStatus.chainEmpty', {
                defaultValue: 'Rantai persetujuan belum tersedia.',
              })}
            </Text>
          )}
        </Card>

        {/* Rejection note (if rejected) */}
        {isRejected && item.reject_reason ? (
          <View className="rounded-card border border-bad-border bg-bad-bg p-3 gap-1">
            <Text variant="strong" className="text-bad-text">
              {t('m:koreksi.rejectedBy')}
            </Text>
            <Text variant="body" className="mt-1 text-bad-text">
              {item.reject_reason}
            </Text>
          </View>
        ) : null}

        {/* Footer actions */}
        <View className="flex-row gap-3 pt-2">
          <Button
            label={t('m:koreksi.tutup')}
            variant="secondary"
            onPress={() => router.back()}
            className="flex-1"
          />
          {isPending ? (
            <Button
              label={t('m:koreksi.withdraw')}
              variant="danger"
              onPress={onWithdraw}
              loading={cancel.isPending}
              className="flex-1"
            />
          ) : null}
        </View>
      </ScrollView>
    </View>
  );
}
