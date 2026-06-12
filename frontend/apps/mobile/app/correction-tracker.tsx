/**
 * Koreksi Saya (Correction Tracker) — E5 F5.4
 * Design reference: koreksi-tracker.png
 *
 * Layout:
 *   Koreksi Saya          [+ Ajukan]
 *   [Semua 5][Pending 2][Disetujui 2][Ditolak 1]   — filter tabs
 *   List of correction cards: type, date, time arrow, id, status badge, Detail >
 */
import {
  type Correction,
  type CorrectionPage,
  useListCorrections,
} from '@swp/api-client/e5';
import { useRouter } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, Pressable, ScrollView, View } from 'react-native';
import { Card } from '../src/ui/Card';
import { StatusBadge } from '../src/ui/StatusBadge';
import { Text } from '../src/ui/Text';

// ── Helpers ──────────────────────────────────────────────────────────────────

function correctionTypeLabel(type: string): string {
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

function correctionTypeIcon(type: string): string {
  switch (type) {
    case 'CHECK_IN':
      return '→';
    case 'CHECK_OUT':
      return '←';
    case 'CODE':
      return '◎';
    default:
      return '·';
  }
}

function timeSlug(iso?: string | null): string {
  if (!iso) return '—:—';
  return new Date(iso).toLocaleTimeString('id-ID', {
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
    timeZone: 'Asia/Jakarta',
  });
}

function dateLabel(iso: string): string {
  return new Date(iso).toLocaleDateString('id-ID', {
    weekday: 'short',
    day: 'numeric',
    month: 'long',
    year: 'numeric',
    timeZone: 'Asia/Jakarta',
  });
}

// Map CorrectionStatus → StatusBadge status key
function corrStatusBadge(status: string): { bStatus: string; label: string } {
  switch (status) {
    case 'PENDING':
      return { bStatus: 'LATE', label: 'Pending' };       // warn tone
    case 'APPROVED':
    case 'APPLIED':
      return { bStatus: 'PRESENT', label: 'Disetujui' };  // ok/teal tone
    case 'REJECTED':
      return { bStatus: 'ABSENT', label: 'Ditolak' };     // bad tone
    case 'CANCELLED':
      return { bStatus: 'ABSENT', label: 'Dibatalkan' };  // bad tone
    default:
      return { bStatus: 'INCOMPLETE', label: status };
  }
}

// ── Filter tab types ──────────────────────────────────────────────────────────

type FilterKey = 'ALL' | 'PENDING' | 'APPROVED' | 'REJECTED';

const FILTERS: { key: FilterKey; label: string }[] = [
  { key: 'ALL', label: 'Semua' },
  { key: 'PENDING', label: 'Pending' },
  { key: 'APPROVED', label: 'Disetujui' },
  { key: 'REJECTED', label: 'Ditolak' },
];

function filterMatches(item: Correction, key: FilterKey): boolean {
  if (key === 'ALL') return true;
  if (key === 'PENDING') return item.status === 'PENDING';
  if (key === 'APPROVED') return item.status === 'APPROVED' || item.status === 'APPLIED';
  if (key === 'REJECTED') return item.status === 'REJECTED';
  return true;
}

// ── Row ───────────────────────────────────────────────────────────────────────

function CorrectionRow({ item }: { item: Correction }) {
  const router = useRouter();
  const { bStatus, label } = corrStatusBadge(item.status);
  const icon = correctionTypeIcon(item.type);
  const typeLabel = correctionTypeLabel(item.type);

  // Build time arrow display
  const snap = item.original_snapshot as Record<string, string | null | undefined>;
  let timeArrow = '';
  if (item.type === 'CHECK_IN') {
    const orig = timeSlug(snap?.check_in_at);
    const prop = timeSlug(item.proposed_check_in_at);
    timeArrow = `${orig} → ${prop}`;
  } else if (item.type === 'CHECK_OUT') {
    const orig = timeSlug(snap?.check_out_at);
    const prop = timeSlug(item.proposed_check_out_at);
    timeArrow = `${orig} → ${prop}`;
  } else {
    timeArrow = item.type === 'CODE' ? 'Absen → Hadir' : '—';
  }

  return (
    <Card>
      {/* Top row: icon + type label + date + status badge */}
      <View className="flex-row items-start justify-between">
        <View className="flex-row items-center gap-2 flex-1">
          <Text className="text-text-3 font-bold">{icon}</Text>
          <View className="flex-1">
            <Text variant="body" className="font-semibold text-text">
              {typeLabel}
            </Text>
            <Text variant="caption" className="text-text-3">
              {dateLabel(item.created_at)}
            </Text>
          </View>
        </View>
        <StatusBadge status={bStatus} label={label} />
      </View>

      {/* Time arrow */}
      <Text className="mt-1.5 text-text-2 text-sm">{timeArrow}</Text>

      {/* ID + detail link */}
      <View className="mt-2 flex-row items-center justify-between border-t border-border pt-2">
        <Text variant="caption" className="font-mono text-text-3 text-xs">
          {item.id}
        </Text>
        <Pressable
          onPress={() =>
            router.push({ pathname: '/correction-detail', params: { id: item.id } })
          }
          className="flex-row items-center gap-1"
        >
          <Text className="text-primary text-sm font-semibold">Detail</Text>
          <Text className="text-primary text-sm">›</Text>
        </Pressable>
      </View>
    </Card>
  );
}

// ── Screen ────────────────────────────────────────────────────────────────────

export default function CorrectionTrackerScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const [activeFilter, setActiveFilter] = useState<FilterKey>('ALL');

  const list = useListCorrections(undefined);
  const body = list.data?.data as CorrectionPage | undefined;
  const allItems: Correction[] = body?.data ?? [];

  const filtered = allItems.filter((item) => filterMatches(item, activeFilter));

  // Count per filter
  const counts: Record<FilterKey, number> = {
    ALL: allItems.length,
    PENDING: allItems.filter((i) => i.status === 'PENDING').length,
    APPROVED: allItems.filter((i) => i.status === 'APPROVED' || i.status === 'APPLIED').length,
    REJECTED: allItems.filter((i) => i.status === 'REJECTED').length,
  };

  return (
    <ScrollView className="flex-1 bg-app-bg" contentContainerStyle={{ padding: 16, gap: 12 }}>
      {/* Filter tabs */}
      <ScrollView
        horizontal
        showsHorizontalScrollIndicator={false}
        contentContainerStyle={{ gap: 8 }}
        className="-mx-4 px-4"
      >
        {FILTERS.map((f) => {
          const active = activeFilter === f.key;
          return (
            <Pressable
              key={f.key}
              onPress={() => setActiveFilter(f.key)}
              className={`flex-row items-center gap-1.5 rounded-pill border px-3 py-1.5 ${
                active ? 'border-text bg-text' : 'border-border bg-surface'
              }`}
            >
              <Text
                className={`text-sm font-semibold ${active ? 'text-surface' : 'text-text-2'}`}
              >
                {f.label}
              </Text>
              <Text
                className={`text-xs font-bold ${active ? 'text-surface' : 'text-text-3'}`}
              >
                {counts[f.key]}
              </Text>
            </Pressable>
          );
        })}
      </ScrollView>

      {/* List */}
      {list.isLoading ? (
        <View className="items-center py-10">
          <ActivityIndicator />
        </View>
      ) : list.isError ? (
        <Card>
          <Text className="text-danger">{t('m:common.errorGeneric')}</Text>
        </Card>
      ) : filtered.length === 0 ? (
        <Card>
          <Text variant="caption" className="text-text-3">
            {t('m:koreksi.empty')}
          </Text>
        </Card>
      ) : (
        <View className="gap-3">
          {filtered.map((item) => (
            <CorrectionRow key={item.id} item={item} />
          ))}
        </View>
      )}

      {/* New correction CTA */}
      <Pressable
        className="items-center py-3"
        onPress={() =>
          router.push({ pathname: '/correction' })
        }
      >
        <Text className="text-primary font-semibold">
          {t('m:koreksi.ajukan')}
        </Text>
      </Pressable>
    </ScrollView>
  );
}
