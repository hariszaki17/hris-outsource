/**
 * Detail Koreksi (Correction Detail) — E5 F5.4
 * Design reference: koreksi-detail.png
 *
 * Layout:
 *   ← Detail Koreksi    •••
 *   SWP-COR-xxxxx  [● Ditolak]
 *   [Koreksi Clock-in]
 *
 *   [PERUBAHAN DIAJUKAN card]
 *     Min, 24 Mei 2026 · Plaza Senayan
 *     [Asli 08:00 (bad-bg)] → [Diajukan 06:30 (ok-bg)]
 *
 *   [ALASAN ANDA card]
 *     reason text
 *
 *   [Rejection card (if rejected)] — bad-bg, who + when + note
 *
 *   ─────────────────────
 *   [Tutup]  [Ajukan koreksi baru] (only if rejected)
 */
import {
  type Correction,
  type GetCorrection200,
  useGetCorrection,
} from '@swp/api-client/e5';
import { formatInstant } from '@swp/shared/datetime';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, View } from 'react-native';
import { ScrollView } from 'react-native';
import { Button } from '../src/ui/Button';
import { Card } from '../src/ui/Card';
import { StatusBadge } from '../src/ui/StatusBadge';
import { Text } from '../src/ui/Text';

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

function shortDate(iso?: string | null): string {
  if (!iso) return '—';
  return new Date(iso).toLocaleDateString('id-ID', {
    day: 'numeric',
    month: 'short',
    timeZone: 'Asia/Jakarta',
  });
}

function correctionTypeTitle(type: string): string {
  switch (type) {
    case 'CHECK_IN':
      return 'Koreksi Clock-in';
    case 'CHECK_OUT':
      return 'Koreksi Clock-out';
    case 'CODE':
      return 'Koreksi Status';
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

// ── Screen ────────────────────────────────────────────────────────────────────

export default function CorrectionDetailScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const { id } = useLocalSearchParams<{ id: string }>();

  const query = useGetCorrection(id ?? '');
  const resp = query.data?.data as GetCorrection200 | undefined;
  const item: Correction | undefined = resp?.data;

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

  // Original and proposed values for the diff display
  const snap = item.original_snapshot as Record<string, string | null | undefined>;
  const origTime = item.type === 'CHECK_IN'
    ? timeLabel(snap?.check_in_at)
    : item.type === 'CHECK_OUT'
      ? timeLabel(snap?.check_out_at)
      : '—';

  const propTime = item.type === 'CHECK_IN'
    ? timeLabel(item.proposed_check_in_at)
    : item.type === 'CHECK_OUT'
      ? timeLabel(item.proposed_check_out_at)
      : '—';

  const attendanceDateIso =
    (snap?.check_in_at) ??
    item.proposed_check_in_at ??
    item.proposed_check_out_at ??
    item.created_at;

  return (
    <ScrollView
      className="flex-1 bg-app-bg"
      contentContainerStyle={{ padding: 16, paddingBottom: 100, gap: 16 }}
    >
      {/* Header: ID + type + status */}
      <View>
        <Text variant="caption" className="font-mono text-text-3 text-xs">
          {item.id}
        </Text>
        <View className="mt-1 flex-row items-center justify-between">
          <Text variant="title" className="font-bold text-text">
            {correctionTypeTitle(item.type)}
          </Text>
          <StatusBadge status={bStatus} label={bLabel} />
        </View>
      </View>

      {/* PERUBAHAN DIAJUKAN */}
      <Card>
        <Text variant="caption" className="font-semibold text-text-3 tracking-wide mb-2">
          PERUBAHAN DIAJUKAN
        </Text>
        <Text variant="caption" className="text-text-2 mb-3">
          {dateLabel(attendanceDateIso)}
          {snap?.site_name ? ` · ${snap.site_name}` : ''}
        </Text>
        <View className="flex-row items-center gap-3">
          {/* Asli box — bad-bg */}
          <View className="flex-1 rounded-card border border-bad-border bg-bad-bg items-center py-3">
            <Text variant="caption" className="text-bad-text font-semibold mb-1">
              Asli
            </Text>
            <Text
              className="font-bold text-bad-text"
              style={{ fontSize: 28 }}
            >
              {origTime}
            </Text>
          </View>
          <Text className="text-text-3 font-bold text-lg">→</Text>
          {/* Diajukan box — ok-bg */}
          <View className="flex-1 rounded-card border border-ok-border bg-ok-bg items-center py-3">
            <Text variant="caption" className="text-ok-text font-semibold mb-1">
              Diajukan
            </Text>
            <Text
              className="font-bold text-ok-text"
              style={{ fontSize: 28 }}
            >
              {propTime}
            </Text>
          </View>
        </View>
      </Card>

      {/* ALASAN ANDA */}
      <Card>
        <Text variant="caption" className="font-semibold text-text-3 tracking-wide mb-2">
          ALASAN ANDA
        </Text>
        <Text className="text-text text-sm">{item.reason}</Text>
      </Card>

      {/* Rejection note (if rejected) */}
      {isRejected && item.reject_reason ? (
        <View className="rounded-card border border-bad-border bg-bad-bg p-3 gap-1">
          <View className="flex-row items-center justify-between">
            <Text className="font-semibold text-bad-text text-sm">
              Ditolak oleh Pemimpin Shift
            </Text>
            <Text variant="caption" className="text-bad-text">
              {shortDate(item.decided_at)} ·{' '}
              {item.decided_at
                ? formatInstant(item.decided_at, { timeStyle: 'short' })
                : '—'}
            </Text>
          </View>
          {/* Decided-by name is just an ID in the model; show it as mono */}
          {item.decided_by ? (
            <Text variant="caption" className="font-mono text-bad-text text-xs">
              {item.decided_by}
            </Text>
          ) : null}
          <Text className="mt-1 text-bad-text text-sm">{item.reject_reason}</Text>
        </View>
      ) : null}
    </ScrollView>
  );
}
