/**
 * F8.5 · Clarification Answer (mobile) — E8 Payroll Period Close.
 *
 * Reached from an E10 clarification notification deep-link:
 *   deep_link.epic = 'E8'  /  deep_link.path = '/clarification-answer?id=SWP-CLR-xxx'
 *
 * Route params (useLocalSearchParams):
 *   id       — SWP-CLR-xxx (required, the ClarificationRequest ID)
 *   question — URL-encoded question text (optional, carried by notifications.tsx
 *              from the notification body; shown inline so the user doesn't need a
 *              separate GET. Falls back to a generic placeholder if absent, because
 *              there is no GET /clarifications/{id} endpoint in v1 — see CONTRACT GAP
 *              note in the report.)
 *   source_type — ATTENDANCE | OVERTIME | LEAVE (optional, for context label)
 *   source_id   — the upstream record ID (optional, for context display)
 *
 * CONTRACT GAP (reported): No GET /clarifications/{id} endpoint exists in v1.
 * The screen therefore relies on the notification carrying `question` as a query
 * param (injected by notifications.tsx when routing). If the question is unavailable
 * (direct deep-link without params) the screen still submits correctly via the ID —
 * it just shows a generic "HR has a question for you" placeholder.
 *
 * G0 DESIGN DEBT: No brainstorm.pen frame exists for this screen. F8.5 design was
 * web-only. Screen is built from existing mobile patterns (attendance-detail.tsx,
 * leave-new.tsx). A .pen frame + DESIGN-SYSTEM entry must be ratified before v1 freeze.
 *
 * Hook: useAnswerClarification — mutate { id, data: ClarificationAnswer }
 *   ClarificationAnswer = { answer: string }  (minLength 1, maxLength 4000)
 */
import { useAnswerClarification } from '@swp/api-client/e8';
import { color } from '@swp/design-tokens';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { ChevronLeft, MessageSquare } from 'lucide-react-native';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Pressable, ScrollView, TextInput, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { Button } from '../src/ui/Button';
import { Card } from '../src/ui/Card';
import { Text } from '../src/ui/Text';

const MAX_ANSWER = 4000;

// ── Source-type context label ──────────────────────────────────────────────────

function sourceLabel(sourceType: string | undefined, t: (k: string) => string): string {
  if (!sourceType) return '';
  const map: Record<string, string> = {
    ATTENDANCE: t('m:clarificationAnswer.sourceAttendance'),
    OVERTIME: t('m:clarificationAnswer.sourceOvertime'),
    LEAVE: t('m:clarificationAnswer.sourceLeave'),
  };
  return map[sourceType] ?? sourceType;
}

// ── Status pill ────────────────────────────────────────────────────────────────

type Tone = 'ok' | 'warn' | 'info' | 'neutral';

const statusTone: Record<string, Tone> = {
  OPEN: 'warn',
  ANSWERED: 'ok',
  RESOLVED: 'info',
  CANCELLED: 'neutral',
};

const toneBg: Record<Tone, string> = {
  ok: 'bg-ok-bg',
  warn: 'bg-warn-bg',
  info: 'bg-info-bg',
  neutral: 'bg-surface-2',
};
const toneBorder: Record<Tone, string> = {
  ok: 'border-ok-border',
  warn: 'border-warn-border',
  info: 'border-info-border',
  neutral: 'border-border',
};
const toneText: Record<Tone, string> = {
  ok: 'text-ok-text',
  warn: 'text-warn-text',
  info: 'text-info-text',
  neutral: 'text-text-2',
};
const toneDot: Record<Tone, string> = {
  ok: color.ok.text,
  warn: color.warn.text,
  info: color.info.text,
  neutral: color.text3,
};

function StatusPill({ status, label }: { status: string; label: string }) {
  const tone = statusTone[status] ?? 'neutral';
  return (
    <View
      className={`flex-row items-center gap-1.5 self-start rounded-pill border px-2.5 py-1 ${toneBg[tone]} ${toneBorder[tone]}`}
    >
      <View
        className="rounded-full"
        style={{ width: 7, height: 7, backgroundColor: toneDot[tone] }}
      />
      <Text variant="caption" weight="semibold" className={toneText[tone]}>
        {label}
      </Text>
    </View>
  );
}

// ── Screen ──────────────────────────────────────────────────────────────────────

export default function ClarificationAnswer() {
  const { t } = useTranslation();
  const router = useRouter();
  const insets = useSafeAreaInsets();

  const {
    id,
    question,
    source_type,
    source_id,
    status: statusParam,
    existing_answer,
  } = useLocalSearchParams<{
    id: string;
    question?: string;
    source_type?: string;
    source_id?: string;
    status?: string;
    existing_answer?: string;
  }>();

  const [answer, setAnswer] = useState('');
  const [validationErr, setValidationErr] = useState<string | null>(null);

  const answerMutation = useAnswerClarification();

  // Determine whether already answered / non-open (carried by notifications.tsx param).
  const isAnswered = statusParam === 'ANSWERED' || statusParam === 'RESOLVED';
  const isCancelled = statusParam === 'CANCELLED';
  const canAnswer = !isAnswered && !isCancelled && !!id;

  async function onSubmit() {
    setValidationErr(null);
    const trimmed = answer.trim();
    if (trimmed.length < 1) {
      setValidationErr(t('m:clarificationAnswer.answerMinLength'));
      return;
    }
    if (trimmed.length > MAX_ANSWER) {
      setValidationErr(`Maksimal ${MAX_ANSWER} karakter.`);
      return;
    }
    try {
      await answerMutation.mutateAsync({ id, data: { answer: trimmed } });
      Alert.alert(t('m:clarificationAnswer.successTitle'), t('m:clarificationAnswer.successBody'), [
        { text: 'OK', onPress: () => router.back() },
      ]);
    } catch {
      setValidationErr(t('m:clarificationAnswer.errorGeneric'));
    }
  }

  const statusKey = statusParam ?? 'OPEN';
  const statusLabelMap: Record<string, string> = {
    OPEN: t('m:clarificationAnswer.statusOpen'),
    ANSWERED: t('m:clarificationAnswer.statusAnswered'),
    RESOLVED: t('m:clarificationAnswer.statusResolved'),
    CANCELLED: t('m:clarificationAnswer.statusCancelled'),
  };

  const ctx = sourceLabel(source_type, t);
  const inputClass = 'rounded-input border border-border bg-surface px-4 py-3 text-sm text-text';

  return (
    <View className="flex-1 bg-app-bg">
      {/* AppBar — matches attendance-detail.tsx pattern */}
      <View
        className="flex-row items-center gap-2 px-4 pb-3"
        style={{ paddingTop: insets.top + 8 }}
      >
        <Pressable onPress={() => router.back()} hitSlop={8}>
          <ChevronLeft size={22} color={color.text} />
        </Pressable>
        <Text variant="screenTitle" className="text-text">
          {t('m:clarificationAnswer.title')}
        </Text>
      </View>

      <ScrollView
        className="flex-1"
        keyboardShouldPersistTaps="handled"
        contentContainerStyle={{ paddingHorizontal: 16, paddingTop: 6, paddingBottom: 24, gap: 14 }}
      >
        {/* ── Status + context card ── */}
        <Card>
          <View className="flex-row items-start justify-between gap-2">
            <View className="flex-1 gap-1">
              {ctx ? (
                <Text variant="caption" weight="semibold" className="text-text-3 uppercase">
                  {t('m:clarificationAnswer.sectionContext')}
                </Text>
              ) : null}
              {ctx ? (
                <Text variant="body" className="text-text">
                  {ctx}
                </Text>
              ) : null}
              {source_id ? (
                <Text variant="caption" mono className="text-text-3">
                  {source_id}
                </Text>
              ) : null}
            </View>
            <StatusPill status={statusKey} label={statusLabelMap[statusKey] ?? statusKey} />
          </View>
        </Card>

        {/* ── Question card ── */}
        <Card>
          <View className="flex-row items-center gap-2 mb-2">
            <MessageSquare size={16} color={color.primary} />
            <Text variant="caption" weight="semibold" className="text-text-3 uppercase">
              {t('m:clarificationAnswer.sectionQuestion')}
            </Text>
          </View>
          <Text variant="body" className="text-text leading-relaxed">
            {question
              ? decodeURIComponent(question)
              : 'HR meminta klarifikasi atas sebuah catatan.'}
          </Text>
        </Card>

        {/* ── Already-answered state ── */}
        {isAnswered && existing_answer ? (
          <Card>
            <Text variant="caption" weight="semibold" className="text-ok-text mb-1">
              {t('m:clarificationAnswer.statusAnswered')}
            </Text>
            <Text variant="body" className="text-text">
              {decodeURIComponent(existing_answer)}
            </Text>
          </Card>
        ) : null}

        {/* ── Answer form (only when OPEN) ── */}
        {canAnswer ? (
          <View className="gap-3">
            <View>
              <Text variant="caption" weight="semibold" className="mb-1 text-text-2">
                {t('m:clarificationAnswer.answerLabel')}
              </Text>
              <TextInput
                value={answer}
                onChangeText={(v) => {
                  setAnswer(v);
                  if (validationErr) setValidationErr(null);
                }}
                multiline
                numberOfLines={5}
                maxLength={MAX_ANSWER}
                placeholder={t('m:clarificationAnswer.answerPlaceholder')}
                placeholderTextColor={color.text3}
                className={`${inputClass} min-h-[120px]`}
                style={{ textAlignVertical: 'top' }}
              />
              <Text variant="caption" className="mt-1 text-text-3 text-right">
                {answer.length}/{MAX_ANSWER}
              </Text>
            </View>

            {validationErr ? (
              <Text variant="caption" className="text-bad-text">
                {validationErr}
              </Text>
            ) : null}

            <Button
              label={t('m:clarificationAnswer.submit')}
              onPress={() => void onSubmit()}
              loading={answerMutation.isPending}
              disabled={!answer.trim()}
            />
          </View>
        ) : null}

        {/* ── Cancelled / already-answered no-action states ── */}
        {!canAnswer && !isAnswered ? (
          <View className="rounded-input border border-border bg-surface px-4 py-3">
            <Text variant="caption" className="text-text-2">
              {isCancelled
                ? t('m:clarificationAnswer.statusCancelled')
                : t('m:clarificationAnswer.alreadyAnswered')}
            </Text>
          </View>
        ) : null}

        {/* ── Missing ID guard ── */}
        {!id ? (
          <Card>
            <Text className="text-bad-text">{t('m:clarificationAnswer.notFound')}</Text>
          </Card>
        ) : null}
      </ScrollView>
    </View>
  );
}
