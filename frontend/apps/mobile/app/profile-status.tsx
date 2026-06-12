// E2 · Status Pengajuan — change-request status screen (status-pengajuan.png).
// Shows: success banner (when just submitted), current PENDING request with field diffs,
// "RIWAYAT PENGAJUAN" list (APPROVED / REJECTED history items with status badges).
//
// Spec refs: F2.x · EP-5 · useListOwnChangeRequests.
// Routing: replace-pushed from /profile-change-request on success; also reachable
// from profile as a direct navigation.
import {
  type ChangeRequest,
  ChangeRequestStatus,
  useListOwnChangeRequests,
} from '@swp/api-client/e2';
import { color } from '@swp/design-tokens';
import { ArrowLeft } from 'lucide-react-native';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, ScrollView, View } from 'react-native';
import { useSession } from '../src/providers/session';
import { Card } from '../src/ui/Card';
import { Screen } from '../src/ui/Screen';
import { Text } from '../src/ui/Text';
import { Button } from '../src/ui/Button';

// ─── helpers ────────────────────────────────────────────────────────────────

/** Format ISO date string to "d MMM yyyy" in id-ID. */
function fmtDate(iso: string): string {
  try {
    return new Date(iso).toLocaleDateString('id-ID', {
      day: 'numeric',
      month: 'short',
      year: 'numeric',
    });
  } catch {
    return iso;
  }
}

/** Derive a human-readable request type label from the changed fields. */
function requestTypeLabel(
  req: ChangeRequest,
  t: (k: string) => string,
): string {
  const c = req.changes;
  const hasPhone = !!c.phone;
  const hasBank = !!c.bank_account;
  if (hasPhone && hasBank) return t('m:statusPengajuan.typeKontakBank');
  if (hasPhone) return t('m:statusPengajuan.typeTelepon');
  if (hasBank) return t('m:statusPengajuan.typeBank');
  return t('m:statusPengajuan.typeGeneral');
}

/** Map ChangeRequestStatus to display tone. */
type Tone = 'warn' | 'ok' | 'bad' | 'info';
function statusTone(status: ChangeRequest['status']): Tone {
  switch (status) {
    case ChangeRequestStatus.PENDING:
      return 'warn';
    case ChangeRequestStatus.APPROVED:
    case ChangeRequestStatus.PARTIALLY_APPROVED:
      return 'ok';
    case ChangeRequestStatus.REJECTED:
      return 'bad';
    default:
      return 'info';
  }
}

const toneBg: Record<Tone, string> = {
  warn: 'bg-warn-bg border-warn-border',
  ok: 'bg-ok-bg border-ok-border',
  bad: 'bg-bad-bg border-bad-border',
  info: 'bg-info-bg border-info-border',
};
const toneText: Record<Tone, string> = {
  warn: 'text-warn-text',
  ok: 'text-ok-text',
  bad: 'text-bad-text',
  info: 'text-info-text',
};
const toneDot: Record<Tone, string> = {
  warn: 'bg-warn-text',
  ok: 'bg-ok-text',
  bad: 'bg-bad-text',
  info: 'bg-info-text',
};

function StatusPill({
  label,
  tone,
}: {
  label: string;
  tone: Tone;
}) {
  return (
    <View
      className={`flex-row items-center gap-1 rounded-pill border px-3 py-1 ${toneBg[tone]}`}
    >
      <View className={`h-2 w-2 rounded-full ${toneDot[tone]}`} />
      <Text className={`text-xs font-semibold ${toneText[tone]}`}>{label}</Text>
    </View>
  );
}

// ─── sub-components ─────────────────────────────────────────────────────────

/** Field diff row: old → new. */
function DiffRow({
  label,
  oldVal,
  newVal,
}: {
  label: string;
  oldVal?: string;
  newVal?: string;
}) {
  if (!oldVal && !newVal) return null;
  return (
    <View className="gap-0.5">
      <Text variant="caption">{label}</Text>
      <View className="flex-row flex-wrap items-center gap-1">
        {oldVal ? (
          <Text variant="secondary" className="text-text-3">
            {oldVal}
          </Text>
        ) : null}
        {oldVal && newVal ? (
          <Text variant="secondary" className="text-text-3">
            {'→'}
          </Text>
        ) : null}
        {newVal ? (
          <Text variant="strong" className="text-text">
            {newVal}
          </Text>
        ) : null}
      </View>
    </View>
  );
}

/** The current pending request card (top of screen). */
function PendingCard({
  req,
  t,
}: {
  req: ChangeRequest;
  t: (k: string, opts?: Record<string, string>) => string;
}) {
  const tone = statusTone(req.status);
  const statusLabel =
    req.status === ChangeRequestStatus.PENDING
      ? t('m:statusPengajuan.statusPending')
      : req.status === ChangeRequestStatus.PARTIALLY_APPROVED
        ? t('m:statusPengajuan.statusPartial')
        : req.status === ChangeRequestStatus.APPROVED
          ? t('m:statusPengajuan.statusApproved')
          : t('m:statusPengajuan.statusRejected');

  return (
    <Card>
      <View className="mb-3 flex-row items-center justify-between">
        <Text variant="strong">{requestTypeLabel(req, t)}</Text>
        <StatusPill label={statusLabel} tone={tone} />
      </View>
      <View className="gap-3 border-t border-border pt-3">
        {req.changes.phone ? (
          <DiffRow label={t('m:statusPengajuan.telepon')} newVal={req.changes.phone} />
        ) : null}
        {req.changes.bank_account?.account_number ? (
          <DiffRow
            label={t('m:statusPengajuan.noRekening')}
            newVal={req.changes.bank_account.account_number}
          />
        ) : null}
      </View>
      <Text variant="caption" className="mt-3 text-text-3">
        {t('m:statusPengajuan.submitted', { date: fmtDate(req.submitted_at) })}
      </Text>
    </Card>
  );
}

/** History item card (APPROVED / REJECTED). */
function HistoryCard({
  req,
  t,
}: {
  req: ChangeRequest;
  t: (k: string, opts?: Record<string, string>) => string;
}) {
  const tone = statusTone(req.status);
  const statusLabel =
    req.status === ChangeRequestStatus.APPROVED ||
    req.status === ChangeRequestStatus.PARTIALLY_APPROVED
      ? t('m:statusPengajuan.statusApproved')
      : t('m:statusPengajuan.statusRejected');

  const dateStr = req.resolved_at
    ? (req.status === ChangeRequestStatus.APPROVED ||
       req.status === ChangeRequestStatus.PARTIALLY_APPROVED
        ? t('m:statusPengajuan.approved', { date: fmtDate(req.resolved_at) })
        : t('m:statusPengajuan.rejected', { date: fmtDate(req.resolved_at) }))
    : '';

  return (
    <Card>
      <View className="flex-row items-center justify-between">
        <View className="flex-1 gap-0.5">
          <Text variant="strong">{requestTypeLabel(req, t)}</Text>
          {dateStr ? (
            <Text variant="secondary">{dateStr}</Text>
          ) : null}
        </View>
        <StatusPill label={statusLabel} tone={tone} />
      </View>
    </Card>
  );
}

// ─── screen ─────────────────────────────────────────────────────────────────

export default function ProfileStatusScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const { user } = useSession();
  const employeeId = user?.employee_id ?? '';

  // "justSubmitted" param: passed as query param from the change-request screen
  // so we show the success banner on first load.
  const params = useLocalSearchParams<{ submitted?: string }>();
  const justSubmitted = params.submitted === '1';

  const q = useListOwnChangeRequests(employeeId, undefined, {
    query: { enabled: !!employeeId },
  });

  const responseData = q.data?.data;
  const allRequests: ChangeRequest[] =
    responseData && 'data' in responseData && Array.isArray(responseData.data)
      ? (responseData.data as ChangeRequest[])
      : [];

  // Split into pending (top card) vs history
  const pending = allRequests.filter((r) => r.status === ChangeRequestStatus.PENDING);
  const history = allRequests.filter(
    (r) =>
      r.status === ChangeRequestStatus.APPROVED ||
      r.status === ChangeRequestStatus.REJECTED ||
      r.status === ChangeRequestStatus.PARTIALLY_APPROVED,
  );

  if (q.isLoading) {
    return (
      <Screen>
        <View className="flex-1 items-center justify-center">
          <ActivityIndicator />
        </View>
      </Screen>
    );
  }

  return (
    <Screen>
      {/* ── Page header ── */}
      <View className="mb-4 flex-row items-center gap-3">
        <Button
          variant="ghost"
          label=""
          onPress={() => router.back()}
          className="w-auto px-0"
          icon={<ArrowLeft size={20} color={color.text} />}
        />
        <Text variant="section">{t('m:statusPengajuan.title')}</Text>
      </View>

      <ScrollView showsVerticalScrollIndicator={false} contentContainerStyle={{ gap: 12 }}>
        {/* ── Success banner (shown right after submission) ── */}
        {justSubmitted ? (
          <View className="flex-row items-center gap-2 rounded-card border border-ok-border bg-ok-bg px-4 py-3">
            <Text className="text-sm text-ok-text">{'✓'}</Text>
            <Text className="text-sm font-semibold text-ok-text">
              {t('m:statusPengajuan.successBanner')}
            </Text>
          </View>
        ) : null}

        {/* ── Pending request (current) ── */}
        {pending.map((req) => (
          <PendingCard key={req.id} req={req} t={t} />
        ))}

        {/* ── History section ── */}
        {history.length > 0 ? (
          <>
            <Text className="text-[11px] font-semibold uppercase tracking-wider text-text-3">
              {t('m:statusPengajuan.sectionHistory')}
            </Text>
            {history.map((req) => (
              <HistoryCard key={req.id} req={req} t={t} />
            ))}
          </>
        ) : null}

        {/* ── Empty state ── */}
        {allRequests.length === 0 ? (
          <View className="items-center py-10">
            <Text variant="secondary">{t('m:statusPengajuan.empty')}</Text>
          </View>
        ) : null}

        <View className="h-4" />
      </ScrollView>
    </Screen>
  );
}
