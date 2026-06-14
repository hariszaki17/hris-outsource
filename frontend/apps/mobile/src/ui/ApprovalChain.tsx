// Approval chain timeline — vertical per-line steps with connectors.
// Design source: brainstorm.pen frame PGrLa (Status Pengajuan — rantai · Agen mobile),
// Card 2 "Rantai Persetujuan". Spec: F11.3 · IB-4 (chain rendering).
//
// READ-ONLY rendering primitive. Each line (Baris N) shows its OR-set members and a
// per-line state derived by the caller:
//   - done    → teal-filled check node + "Selesai" pill + teal result line (the OR-clearer)
//   - current → warn-outlined numbered node + "Menunggu" pill + muted result
//   - rejected→ bad-filled X node + "Ditolak" pill + red result line (reason)
//   - upcoming→ neutral numbered node + "Menunggu" pill + muted "belum tercapai" result
// Connector line drawn between every adjacent pair (PGrLa `conn`).
import { color } from '@swp/design-tokens';
import { Check, X } from 'lucide-react-native';
import { Fragment } from 'react';
import { View } from 'react-native';
import { Text } from './Text';

export type LineState = 'done' | 'current' | 'rejected' | 'upcoming';

export interface ApprovalChainStep {
  /** 1-based line number (Baris N). */
  lineNo: number;
  state: LineState;
  /** OR-set member display names (e.g. "Rudi Wijaya · Sari"). */
  members: string;
  /** Pill label: "Selesai" | "Menunggu" | "Ditolak". */
  statusLabel: string;
  /** Result line: who acted + time, or "Menunggu keputusan." / reason. */
  result: string;
}

const pill: Record<LineState, { bg: string; tx: string; bd: string }> = {
  done: { bg: 'bg-ok-bg', tx: 'text-ok-text', bd: 'border-ok-border' },
  current: { bg: 'bg-warn-bg', tx: 'text-warn-text', bd: 'border-warn-border' },
  rejected: { bg: 'bg-bad-bg', tx: 'text-bad-text', bd: 'border-bad-border' },
  upcoming: { bg: 'bg-warn-bg', tx: 'text-warn-text', bd: 'border-warn-border' },
};

const resultColor: Record<LineState, string> = {
  done: 'text-ok-text',
  current: 'text-text-3',
  rejected: 'text-bad-text',
  upcoming: 'text-text-3',
};

function Node({ step }: { step: ApprovalChainStep }) {
  const nodeTestID = `chain-node-${step.lineNo}-${step.state}`;
  if (step.state === 'done') {
    return (
      <View
        testID={nodeTestID}
        className="h-[26px] w-[26px] items-center justify-center rounded-full border"
        style={{ backgroundColor: color.ok.text, borderColor: color.ok.text }}
      >
        <Check size={14} color={color.surface} />
      </View>
    );
  }
  if (step.state === 'rejected') {
    return (
      <View
        testID={nodeTestID}
        className="h-[26px] w-[26px] items-center justify-center rounded-full border"
        style={{ backgroundColor: color.bad.text, borderColor: color.bad.text }}
      >
        <X size={14} color={color.surface} />
      </View>
    );
  }
  // current → warn-outlined; upcoming → neutral-outlined. Both show the line number.
  const isCurrent = step.state === 'current';
  return (
    <View
      testID={nodeTestID}
      className="h-[26px] w-[26px] items-center justify-center rounded-full"
      style={{
        backgroundColor: isCurrent ? color.warn.bg : color.surface,
        borderWidth: isCurrent ? 2 : 1,
        borderColor: isCurrent ? color.warn.text : color.border,
      }}
    >
      <Text variant="badge" weight="bold" className={isCurrent ? 'text-warn-text' : 'text-text-3'}>
        {String(step.lineNo)}
      </Text>
    </View>
  );
}

export function ApprovalChain({ steps }: { steps: ApprovalChainStep[] }) {
  return (
    <View className="w-full">
      {steps.map((step, i) => {
        const p = pill[step.state];
        return (
          <Fragment key={step.lineNo}>
            <View testID={`chain-line-${step.lineNo}`} className="w-full flex-row gap-3 pt-3.5">
              <Node step={step} />
              <View className="flex-1 gap-1">
                <View className="flex-row items-center gap-2">
                  <Text variant="strong" weight="bold">
                    {`Baris ${step.lineNo}`}
                  </Text>
                  <View className={`rounded-pill border px-2 py-0.5 ${p.bg} ${p.bd}`}>
                    <Text variant="micro" weight="bold" className={p.tx}>
                      {step.statusLabel}
                    </Text>
                  </View>
                </View>
                <Text variant="caption" className="text-text-2">
                  {step.members}
                </Text>
                <Text variant="caption" className={resultColor[step.state]}>
                  {step.result}
                </Text>
              </View>
            </View>
            {/* connector between adjacent steps (PGrLa `conn`) */}
            {i < steps.length - 1 ? (
              <View className="pl-3 pt-2">
                <View className="h-4 w-0.5" style={{ backgroundColor: color.border }} />
              </View>
            ) : null}
          </Fragment>
        );
      })}
    </View>
  );
}
