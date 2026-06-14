/**
 * E11 · Approve / Reject bottom sheet (SL mobile).
 *
 * Design reference: brainstorm.pen frame viUFF ("E11 · Sheet — Setujui (SL mobile)").
 * Feature: F11.2 (approval execution) · F11.3 (inbox act-in-place).
 *
 * One sheet, two modes:
 *  - 'approve' → note OPTIONAL ("Catatan (opsional)"). Setujui submits with optional note.
 *  - 'reject'  → reason REQUIRED ("Alasan penolakan *"). Tolak submits only when non-empty.
 * Tapping the in-sheet "Tolak" / "Setujui" toggles between the two modes (viUFF §"Approve vs
 * Reject behavior"): the reject variant swaps the field to a required reason + validation.
 *
 * Anatomy (viUFF): grab handle → header (title + X) → summary box (surface-2) → mini chain
 * progress (done node → connector → current node) → dynamic explainer → field → action row.
 * The host (sl-verifikasi.tsx) owns the Modal + scrim; this renders only the sheet body.
 */
import { color } from '@swp/design-tokens';
import { Check, X } from 'lucide-react-native';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Pressable, TextInput, View } from 'react-native';
import { Text } from './Text';

export type ApprovalSheetMode = 'approve' | 'reject';

export interface ApprovalSheetTarget {
  /** "Cuti Tahunan · 3 hari" — `{type} · {duration}`. */
  summaryLine: string;
  /** "Budi Santoso · SWP-EMP-1042 · 12–14 Jul 2026" — `{name} · {empId} · {dates}`. */
  detailLine: string;
  /** 1-based current line the viewer is acting on. */
  currentLine: number;
  /** Total configured lines (>= 1). */
  lineCount: number;
}

/** Mini chain progress: done nodes (teal check) → current node (warn-outlined number). */
function ChainProgress({ currentLine, lineCount }: { currentLine: number; lineCount: number }) {
  const { t } = useTranslation();
  const total = Math.max(lineCount, currentLine, 1);
  const nodes = Array.from({ length: total }, (_, i) => i + 1);

  return (
    <View className="flex-row items-center gap-2">
      {nodes.map((n, idx) => {
        const done = n < currentLine;
        const current = n === currentLine;
        const nodeState = done ? 'done' : current ? 'current' : 'upcoming';
        return (
          <View key={n} className="flex-row items-center" style={{ flex: 1 }}>
            <View
              className="items-center gap-[5px]"
              testID={`sheet-chain-node-${n}`}
              accessibilityLabel={`chain-node-${n}-${nodeState}`}
            >
              {done ? (
                <View
                  className="h-[26px] w-[26px] items-center justify-center rounded-pill border border-border"
                  style={{ backgroundColor: color.ok.text }}
                >
                  <Check size={14} color={'#FFFFFF'} />
                </View>
              ) : current ? (
                <View
                  className="h-[26px] w-[26px] items-center justify-center rounded-pill bg-warn-bg"
                  style={{ borderWidth: 2, borderColor: color.warn.text }}
                >
                  <Text variant="badge" weight="bold" className="text-warn-text">
                    {String(n)}
                  </Text>
                </View>
              ) : (
                <View className="h-[26px] w-[26px] items-center justify-center rounded-pill border border-border bg-surface">
                  <Text variant="badge" weight="bold" className="text-text-3">
                    {String(n)}
                  </Text>
                </View>
              )}
              <Text
                variant="badge"
                weight="semibold"
                className={done ? 'text-ok-text' : current ? 'text-warn-text' : 'text-text-3'}
              >
                {t('m:approvals.barisN', { n })}
              </Text>
            </View>
            {idx < nodes.length - 1 ? (
              <View className="mx-1 h-0.5 flex-1 self-start bg-border" style={{ marginTop: 12 }} />
            ) : null}
          </View>
        );
      })}
    </View>
  );
}

export function ApprovalActionSheet({
  mode,
  target,
  onModeChange,
  onApprove,
  onReject,
  onClose,
  submitting,
}: {
  mode: ApprovalSheetMode;
  target: ApprovalSheetTarget;
  onModeChange: (mode: ApprovalSheetMode) => void;
  onApprove: (note?: string) => void;
  onReject: (reason: string) => void;
  onClose: () => void;
  submitting: boolean;
}) {
  const { t } = useTranslation();
  const [note, setNote] = useState('');
  const [reason, setReason] = useState('');
  const isReject = mode === 'reject';
  const canSubmitReject = reason.trim().length > 0;

  return (
    <View className="rounded-t-[20px] bg-surface px-[18px] pb-[22px] pt-3" style={{ gap: 16 }}>
      {/* 1. grab handle */}
      <View className="self-center pb-1">
        <View className="h-1 w-10 rounded-pill bg-border" />
      </View>

      {/* 2. header */}
      <View className="flex-row items-center justify-between">
        <Text variant="subtitle" className="text-text" style={{ fontSize: 18, lineHeight: 24 }}>
          {isReject ? t('m:approvals.sheetTitleReject') : t('m:approvals.sheetTitleApprove')}
        </Text>
        <Pressable onPress={onClose} hitSlop={8} accessibilityRole="button">
          <X size={18} color={color.text3} />
        </Pressable>
      </View>

      {/* 3. summary box */}
      <View className="gap-[3px] rounded-[10px] bg-surface-2 p-3.5">
        <Text variant="strong" weight="bold" className="text-text">
          {target.summaryLine}
        </Text>
        <Text variant="caption" className="text-text-2">
          {target.detailLine}
        </Text>
      </View>

      {/* 4. mini chain progress */}
      <ChainProgress currentLine={target.currentLine} lineCount={target.lineCount} />

      {/* 5. explainer */}
      <Text variant="caption" className="text-text-2" style={{ lineHeight: 18 }}>
        {t('m:approvals.sheetExplainer', {
          line: target.currentLine,
          total: Math.max(target.lineCount, target.currentLine, 1),
        })}
      </Text>

      {/* 6. field — optional note (approve) / required reason (reject) */}
      <View className="gap-1.5">
        <Text variant="label" className="text-text">
          {isReject ? t('m:approvals.reasonLabel') : t('m:approvals.noteLabel')}
        </Text>
        <TextInput
          testID={isReject ? 'sheet-reason-input' : 'sheet-note-input'}
          value={isReject ? reason : note}
          onChangeText={isReject ? setReason : setNote}
          multiline
          placeholder={
            isReject ? t('m:approvals.reasonPlaceholder') : t('m:approvals.notePlaceholder')
          }
          placeholderTextColor={color.text3}
          className="min-h-[64px] rounded-input border border-border bg-surface-2 p-3 text-text"
          style={{ textAlignVertical: 'top', fontSize: 13, fontFamily: 'Inter_400Regular' }}
        />
      </View>

      {/* 7. action row */}
      <View className="flex-row items-center gap-2.5 pt-1">
        {/* Tolak — fixed 120; danger-ghost. In reject mode it submits, else it switches mode. */}
        <Pressable
          testID="sheet-reject-btn"
          disabled={submitting || (isReject && !canSubmitReject)}
          onPress={() => (isReject ? onReject(reason.trim()) : onModeChange('reject'))}
          accessibilityRole="button"
          className={`items-center justify-center rounded-[9px] border border-border bg-surface py-3 ${
            submitting || (isReject && !canSubmitReject) ? 'opacity-60' : ''
          }`}
          style={{ width: 120 }}
        >
          <Text variant="subtitle" className="text-bad-text">
            {t('m:approvals.tolak')}
          </Text>
        </Pressable>

        {/* Setujui — fills remaining. In approve mode it submits, else it switches back. */}
        <Pressable
          testID="sheet-approve-btn"
          disabled={submitting}
          onPress={() => (isReject ? onModeChange('approve') : onApprove(note.trim() || undefined))}
          accessibilityRole="button"
          className={`flex-1 flex-row items-center justify-center gap-[7px] rounded-[9px] bg-primary py-3 ${
            submitting ? 'opacity-60' : ''
          }`}
        >
          <Check size={16} color={'#FFFFFF'} />
          <Text variant="subtitle" style={{ color: '#FFFFFF' }}>
            {t('m:approvals.setujui')}
          </Text>
        </Pressable>
      </View>
    </View>
  );
}
