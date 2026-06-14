// E11 · Status Pengajuan — rantai (Agent mobile) · READ-ONLY approval chain timeline.
//
// Design source: brainstorm.pen frame PGrLa (E11 · Status Pengajuan — rantai · Agen mobile).
// Spec refs: F11.3 · IB-4 (request-detail chain timeline renders every line, its members,
//            each recorded action, and the current pending line) · INV-3 (agent cannot act).
//
// This is the agent's read-only view of where their LEAVE or OVERTIME request sits in the
// approval chain. The agent never acts here — approve/reject lives in the SL/approver inbox.
//
// ── Route + param contract (for integration to wire from leave.tsx / overtime.tsx) ──
//   router.push({
//     pathname: '/approval-status',
//     params: {
//       approval_instance_id?: string,   // off the request; null/absent ⇒ pre-chain state
//       request_type?: 'LEAVE' | 'OVERTIME',
//       request_label?: string,          // optional header subtitle (e.g. "SWP-LR-1042")
//       request_title?: string,          // optional title override (e.g. "Cuti Tahunan")
//     },
//   })
// If approval_instance_id is null/absent (overtime PENDING_AGENT_CONFIRM, or leave DRAFT) we
// render a "belum masuk antrean persetujuan" pre-chain state — we never error.
import {
  type ApprovalAction,
  ApprovalActionAction,
  type ApprovalInstanceDetail,
  type ApprovalLine,
  InstanceStatus,
  RequestType,
  useGetApprovalInstance,
} from '@swp/api-client/e11';
import { color } from '@swp/design-tokens';
import { formatInstant } from '@swp/shared/datetime';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { ArrowLeft, Ban, Check, FilePlus, Inbox, X } from 'lucide-react-native';
import type { ComponentType } from 'react';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, ScrollView, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import {
  ApprovalChain,
  type ApprovalChainStep,
  type LineState,
} from '../src/ui/ApprovalChain';
import { Card } from '../src/ui/Card';
import { Text } from '../src/ui/Text';

// i18n: keys live under the mobile `m` namespace as `approvalStatus.*`. We pass an inline
// Bahasa `defaultValue` so this screen renders correctly before the catalog bundle is added;
// the flat key→Bahasa list is documented in the agent return. (Copy via i18n — ENGINEERING.md.)
type TFn = (key: string, opts?: Record<string, unknown>) => string;

// ── helpers ───────────────────────────────────────────────────────────────────

/** "9 Jul 14:20" — WIB instant, short. */
function fmtEvent(iso?: string): string {
  if (!iso) return '';
  return formatInstant(iso, { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
}

/** Display name for a user_id, resolved from action/member metadata when present. */
function memberNames(line: ApprovalLine | undefined): string {
  if (!line || line.members.length === 0) return '';
  return line.members.map((m) => m.display_name ?? m.user_id).join(' · ');
}

/** The action (if any) that cleared / decided a given line_no. */
function actionForLine(actions: ApprovalAction[], lineNo: number): ApprovalAction | undefined {
  // Newest decisive action on this line (APPROVE clears, REJECT/BYPASS decide).
  return actions
    .filter((a) => a.line_no === lineNo)
    .sort((a, b) => (a.created_at < b.created_at ? 1 : -1))[0];
}

function actorName(a: ApprovalAction, line: ApprovalLine | undefined): string {
  if (a.actor_name) return a.actor_name;
  const m = line?.members.find((mm) => mm.user_id === a.actor_user_id);
  return m?.display_name ?? a.actor_user_id;
}

// ── chain derivation (PGrLa Card 2 rules) ───────────────────────────────────────

function buildSteps(detail: ApprovalInstanceDetail, t: TFn): ApprovalChainStep[] {
  const lines = [...(detail.lines ?? [])].sort((a, b) => a.line_no - b.line_no);
  const actions = detail.actions ?? [];
  const current = detail.current_line;
  const rejected = detail.status === InstanceStatus.REJECTED;
  const approved = detail.status === InstanceStatus.APPROVED;

  return lines.map((line) => {
    const act = actionForLine(actions, line.line_no);
    const members = memberNames(line);

    // REJECTED instance: the line carrying the REJECT action is the rejected row.
    if (rejected && act && act.action === ApprovalActionAction.REJECT) {
      return {
        lineNo: line.line_no,
        state: 'rejected' satisfies LineState,
        members,
        statusLabel: t('m:approvalStatus.lineRejected', { defaultValue: 'Ditolak' }),
        result: t('m:approvalStatus.resultRejected', {
          defaultValue: 'Ditolak oleh {{actor}} · {{time}}{{reason}}',
          actor: actorName(act, line),
          time: fmtEvent(act.created_at),
          reason: act.reason ? ` — ${act.reason}` : '',
        }),
      };
    }

    // A recorded APPROVE/BYPASS on this line ⇒ done (cleared). Also: a fully-approved
    // instance marks every line done.
    const cleared = !!act && act.action !== ApprovalActionAction.REJECT;
    const isDone = cleared || (approved && line.line_no <= (detail.line_count ?? current));
    if (isDone) {
      const isBypass = act?.action === ApprovalActionAction.BYPASS;
      const resultKey = isBypass
        ? t('m:approvalStatus.resultBypass', {
            defaultValue: 'Disahkan oleh {{actor}} · {{time}} (bypass)',
            actor: act ? actorName(act, line) : '',
            time: fmtEvent(act?.created_at),
          })
        : act
          ? t('m:approvalStatus.resultApproved', {
              defaultValue: 'Disetujui oleh {{actor}} · {{time}} (OR)',
              actor: actorName(act, line),
              time: fmtEvent(act.created_at),
            })
          : t('m:approvalStatus.resultDone', { defaultValue: 'Selesai.' });
      return {
        lineNo: line.line_no,
        state: 'done' satisfies LineState,
        members,
        statusLabel: t('m:approvalStatus.lineDone', { defaultValue: 'Selesai' }),
        result: resultKey,
      };
    }

    // The current line (only when the instance is still pending).
    if (!rejected && !approved && line.line_no === current) {
      return {
        lineNo: line.line_no,
        state: 'current' satisfies LineState,
        members,
        statusLabel: t('m:approvalStatus.lineWaiting', { defaultValue: 'Menunggu' }),
        result: t('m:approvalStatus.resultCurrent', { defaultValue: 'Menunggu keputusan.' }),
      };
    }

    // Everything else is an upcoming, not-yet-reached line.
    return {
      lineNo: line.line_no,
      state: 'upcoming' satisfies LineState,
      members,
      statusLabel: t('m:approvalStatus.lineWaiting', { defaultValue: 'Menunggu' }),
      result: t('m:approvalStatus.resultUpcoming', { defaultValue: 'Belum tercapai.' }),
    };
  });
}

// ── header status pill ──────────────────────────────────────────────────────────

type Tone = 'warn' | 'ok' | 'bad';
const pillBg: Record<Tone, string> = {
  warn: 'bg-warn-bg border-warn-border',
  ok: 'bg-ok-bg border-ok-border',
  bad: 'bg-bad-bg border-bad-border',
};
const pillTx: Record<Tone, string> = {
  warn: 'text-warn-text',
  ok: 'text-ok-text',
  bad: 'text-bad-text',
};
const pillDot: Record<Tone, string> = {
  warn: 'bg-warn-text',
  ok: 'bg-ok-text',
  bad: 'bg-bad-text',
};

function HeaderPill({ label, tone }: { label: string; tone: Tone }) {
  return (
    <View className={`flex-row items-center gap-1.5 rounded-pill border px-2.5 py-1 ${pillBg[tone]}`}>
      <View className={`h-1.5 w-1.5 rounded-full ${pillDot[tone]}`} />
      <Text variant="badge" weight="bold" className={pillTx[tone]}>
        {label}
      </Text>
    </View>
  );
}

// ── trail (Card 3 RIWAYAT) ───────────────────────────────────────────────────────

function TrailRow({
  Icon,
  tint,
  text,
}: {
  Icon: ComponentType<{ size?: number; color?: string }>;
  tint: string;
  text: string;
}) {
  return (
    <View className="flex-row items-center gap-2">
      <Icon size={14} color={tint} />
      <Text variant="caption" className="text-text-2">
        {text}
      </Text>
    </View>
  );
}

// ── screen ────────────────────────────────────────────────────────────────────

export default function ApprovalStatusScreen() {
  const { t } = useTranslation() as { t: TFn };
  const router = useRouter();
  const insets = useSafeAreaInsets();
  const params = useLocalSearchParams<{
    approval_instance_id?: string;
    request_type?: string;
    request_label?: string;
    request_title?: string;
  }>();

  const instanceId = params.approval_instance_id ?? '';
  const hasInstance = instanceId.length > 0;

  const query = useGetApprovalInstance(instanceId, {
    query: { enabled: hasInstance },
  });

  // The response envelope is `{ data: ApprovalInstanceDetail | NotFoundResponse }`.
  const raw = query.data?.data;
  const detail: ApprovalInstanceDetail | undefined =
    raw && 'lines' in (raw as object) ? (raw as ApprovalInstanceDetail) : undefined;
  // current_line is required on ApprovalInstance, so use it as the success discriminator too.
  const instance: ApprovalInstanceDetail | undefined =
    raw && 'current_line' in (raw as object) ? (raw as ApprovalInstanceDetail) : undefined;
  const inst = detail ?? instance;

  const requestType = (params.request_type ?? inst?.request_type) as
    | RequestType
    | undefined;
  const typeTitle =
    params.request_title ??
    (requestType === RequestType.OVERTIME
      ? t('m:approvalStatus.typeOvertime', { defaultValue: 'Lembur' })
      : requestType === RequestType.LEAVE
        ? t('m:approvalStatus.typeLeave', { defaultValue: 'Cuti' })
        : t('m:approvalStatus.typeGeneric', { defaultValue: 'Pengajuan' }));

  // ── chrome: header ──
  const Header = (
    <View
      className="flex-row items-center gap-3 border-b border-border bg-surface px-4 pb-3.5"
      style={{ paddingTop: insets.top + 8 }}
    >
      <ArrowLeft size={20} color={color.text2} onPress={() => router.back()} />
      <Text variant="screenTitle">
        {t('m:approvalStatus.title', { defaultValue: 'Status Pengajuan' })}
      </Text>
    </View>
  );

  // ── pre-chain: no instance id (DRAFT / PENDING_AGENT_CONFIRM) ──
  if (!hasInstance) {
    return (
      <View className="flex-1 bg-app-bg">
        {Header}
        <View className="flex-1 items-center justify-center gap-3 px-8">
          <Inbox size={40} color={color.text3} />
          <Text variant="subtitle" className="text-center">
            {t('m:approvalStatus.preChainTitle', { defaultValue: 'Belum masuk antrean persetujuan' })}
          </Text>
          <Text variant="secondary" className="text-center text-text-2">
            {t('m:approvalStatus.preChainBody', {
              defaultValue:
                'Pengajuan ini belum dikirim ke rantai persetujuan. Selesaikan pengajuan untuk mulai diproses.',
            })}
          </Text>
        </View>
      </View>
    );
  }

  // ── loading ──
  if (query.isLoading) {
    return (
      <View className="flex-1 bg-app-bg">
        {Header}
        <View className="flex-1 items-center justify-center">
          <ActivityIndicator />
        </View>
      </View>
    );
  }

  // ── error / not found ──
  if (query.isError || !inst) {
    return (
      <View className="flex-1 bg-app-bg">
        {Header}
        <View className="flex-1 items-center justify-center px-8">
          <Text variant="secondary" className="text-center text-text-2">
            {t('m:approvalStatus.error', {
              defaultValue: 'Tidak dapat memuat status persetujuan. Coba lagi.',
            })}
          </Text>
        </View>
      </View>
    );
  }

  // ── derive header status pill ──
  const status = inst.status;
  const lineCount = inst.line_count ?? inst.lines?.length ?? 1;
  let pillTone: Tone = 'warn';
  let pillLabel = t('m:approvalStatus.statusPending', {
    defaultValue: 'Menunggu · Baris {{cur}}/{{total}}',
    cur: inst.current_line,
    total: lineCount,
  });
  if (status === InstanceStatus.APPROVED) {
    pillTone = 'ok';
    pillLabel = t('m:approvalStatus.statusApproved', { defaultValue: 'Disetujui' });
  } else if (status === InstanceStatus.REJECTED) {
    pillTone = 'bad';
    pillLabel = t('m:approvalStatus.statusRejected', { defaultValue: 'Ditolak' });
  }

  const steps = buildSteps(inst, t);
  const trail = buildTrail(inst, t);

  return (
    <View className="flex-1 bg-app-bg">
      {Header}
      <ScrollView
        className="flex-1"
        contentContainerStyle={{ padding: 16, paddingBottom: 32, gap: 14 }}
        showsVerticalScrollIndicator={false}
      >
        {/* Card 1 — HeaderCard */}
        <Card className="gap-2.5">
          <View className="flex-row items-center justify-between">
            <Text variant="cardTitle">{typeTitle}</Text>
            <HeaderPill label={pillLabel} tone={pillTone} />
          </View>
          {params.request_label ? (
            <Text variant="secondary" mono className="text-text-2">
              {params.request_label}
            </Text>
          ) : (
            <Text variant="secondary" mono className="text-text-2">
              {inst.request_id}
            </Text>
          )}
        </Card>

        {/* Card 2 — chain timeline */}
        <Card>
          <Text variant="subtitle" className="mb-1">
            {t('m:approvalStatus.chainHeader', { defaultValue: 'Rantai Persetujuan' })}
          </Text>
          {steps.length > 0 ? (
            <ApprovalChain steps={steps} />
          ) : (
            <Text variant="secondary" className="pt-2 text-text-2">
              {t('m:approvalStatus.chainEmpty', {
                defaultValue: 'Rantai persetujuan belum tersedia.',
              })}
            </Text>
          )}
        </Card>

        {/* Card 3 — RIWAYAT trail */}
        {trail.length > 0 ? (
          <Card className="gap-2">
            <Text variant="badge" className="uppercase tracking-wider text-text-3">
              {t('m:approvalStatus.trailHeader', { defaultValue: 'RIWAYAT' })}
            </Text>
            {trail.map((row, i) => (
              <TrailRow key={`${row.text}-${i}`} Icon={row.Icon} tint={row.tint} text={row.text} />
            ))}
          </Card>
        ) : null}
      </ScrollView>
    </View>
  );
}

// ── trail derivation (Card 3) ───────────────────────────────────────────────────

interface TrailItem {
  Icon: ComponentType<{ size?: number; color?: string }>;
  tint: string;
  text: string;
}

function buildTrail(detail: ApprovalInstanceDetail, t: TFn): TrailItem[] {
  const rows: TrailItem[] = [];

  // "Diajukan · {time}" — submission, neutral.
  if (detail.created_at) {
    rows.push({
      Icon: FilePlus,
      tint: color.text2,
      text: t('m:approvalStatus.trailSubmitted', {
        defaultValue: 'Diajukan · {{time}}',
        time: fmtEvent(detail.created_at),
      }),
    });
  }

  // One row per recorded action, oldest→newest. Tint by kind.
  const actions = [...(detail.actions ?? [])].sort((a, b) =>
    a.created_at < b.created_at ? -1 : 1,
  );
  for (const a of actions) {
    if (a.action === ApprovalActionAction.APPROVE) {
      rows.push({
        Icon: Check,
        tint: color.ok.text,
        text: t('m:approvalStatus.trailApproved', {
          defaultValue: 'Baris {{line}} disetujui · {{time}}',
          line: a.line_no,
          time: fmtEvent(a.created_at),
        }),
      });
    } else if (a.action === ApprovalActionAction.REJECT) {
      rows.push({
        Icon: X,
        tint: color.bad.text,
        text: t('m:approvalStatus.trailRejected', {
          defaultValue: 'Baris {{line}} ditolak · {{time}}',
          line: a.line_no,
          time: fmtEvent(a.created_at),
        }),
      });
    } else {
      rows.push({
        Icon: Ban,
        tint: color.accent.purple,
        text: t('m:approvalStatus.trailBypass', {
          defaultValue: 'Baris {{line}} disahkan (bypass) · {{time}}',
          line: a.line_no,
          time: fmtEvent(a.created_at),
        }),
      });
    }
  }

  return rows;
}
