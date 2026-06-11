/**
 * Ajukan Koreksi — modal to file an attendance correction (opened from a /me/attendance row).
 *
 * Web port of apps/mobile/app/correction.tsx, rendered as a Modal over the attendance list
 * (docs/design/brainstorm.pen "Agen Web · Ajukan Koreksi (modal)"). Receives attendanceId +
 * date from the triggering row. On success → toast `corrSuccess` + close + onSuccess().
 * Error codes: OUTSIDE_CORRECTION_WINDOW → corrOutsideWindow,
 *              CORRECTION_ALREADY_PENDING → corrAlreadyPending, else corrError.
 *
 * F5.6 refs: BR-7, C-2 (7-day window), C-3 (already-pending guard).
 */
import { ApiError } from '@swp/api-client';
import {
  type CorrectionType,
  type CorrectionWriteRequest,
  useCreateCorrection,
} from '@swp/api-client/e5';
import {
  Button,
  FormField,
  Input,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  useToast,
} from '@swp/ui';
import { PencilLine } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

// CODE corrections need an attendance-code picker — deferred; MVP covers the common
// missed/wrong clock-in/out case.
const TYPES: { value: CorrectionType; key: 'corrTypeCheckIn' | 'corrTypeCheckOut' }[] = [
  { value: 'CHECK_IN', key: 'corrTypeCheckIn' },
  { value: 'CHECK_OUT', key: 'corrTypeCheckOut' },
];

export interface AgentCorrectionModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Attendance row being corrected. */
  attendanceId: string;
  /** ISO instant/date of the attendance row (used to build the corrected instant). */
  date?: string;
  /** Called after a successful submit so the list can refetch. */
  onSuccess?: () => void;
}

export function AgentCorrectionModal({
  open,
  onOpenChange,
  attendanceId,
  date,
  onSuccess,
}: AgentCorrectionModalProps) {
  const { t } = useTranslation('agent');
  const { toast } = useToast();
  const create = useCreateCorrection();

  const [type, setType] = useState<CorrectionType>('CHECK_OUT');
  const [time, setTime] = useState('');
  const [reason, setReason] = useState('');
  const [err, setErr] = useState<string | null>(null);

  async function onSubmit() {
    setErr(null);
    if (!/^([01]\d|2[0-3]):([0-5]\d)$/.test(time.trim())) {
      setErr(t('corrBadTime'));
      return;
    }
    if (reason.trim().length < 5) {
      setErr(t('corrBadReason'));
      return;
    }
    // Asia/Jakarta is a fixed +07:00 offset (no DST) — safe to build the instant directly.
    const iso = `${(date ?? '').slice(0, 10)}T${time.trim()}:00+07:00`;
    const data: CorrectionWriteRequest = {
      attendance_id: attendanceId,
      type,
      reason: reason.trim(),
      proposed_check_in_at: type === 'CHECK_IN' ? iso : undefined,
      proposed_check_out_at: type === 'CHECK_OUT' ? iso : undefined,
    };
    try {
      await create.mutateAsync({ data });
      toast({ tone: 'success', title: t('corrSuccess') });
      onOpenChange(false);
      onSuccess?.();
    } catch (e) {
      if (e instanceof ApiError) {
        if (e.code === 'OUTSIDE_CORRECTION_WINDOW' || e.code === 'OUTSIDE_WINDOW') {
          setErr(t('corrOutsideWindow'));
          return;
        }
        if (e.code === 'CORRECTION_ALREADY_PENDING' || e.code === 'ALREADY_PENDING') {
          setErr(t('corrAlreadyPending'));
          return;
        }
      }
      setErr(t('corrError'));
    }
  }

  return (
    <Modal open={open} onOpenChange={onOpenChange} size="lg" className="w-[540px]">
      <ModalHeader icon={PencilLine} tone="brand" title={t('corrTitle')} closeLabel={t('cancel')} />

      <ModalBody className="gap-5">
        {/* Type toggle */}
        <div className="flex flex-col gap-1.5">
          <span className="text-sm font-medium text-text-2">{t('corrTitle')}</span>
          <div className="flex gap-3">
            {TYPES.map((opt) => (
              <button
                key={opt.value}
                type="button"
                onClick={() => setType(opt.value)}
                className={[
                  'flex-1 rounded-md border px-4 py-3 text-sm font-medium transition-colors',
                  type === opt.value
                    ? 'border-primary bg-primary-soft text-primary'
                    : 'border-border bg-surface text-text-2 hover:bg-surface-2',
                ].join(' ')}
              >
                {t(opt.key)}
              </button>
            ))}
          </div>
        </div>

        {/* Time input */}
        <FormField
          label={t('corrTime')}
          htmlFor="correction-time"
          error={err === t('corrBadTime') ? err : undefined}
          required
        >
          <Input
            id="correction-time"
            type="time"
            value={time}
            onChange={(e) => setTime(e.target.value)}
            aria-invalid={err === t('corrBadTime') ? true : undefined}
          />
        </FormField>

        {/* Reason textarea — @swp/ui has no Textarea primitive; styled to match Input. */}
        <FormField
          label={t('corrReason')}
          htmlFor="correction-reason"
          error={err === t('corrBadReason') ? err : undefined}
          required
        >
          <textarea
            id="correction-reason"
            rows={4}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            aria-invalid={err === t('corrBadReason') ? true : undefined}
            className={[
              'flex w-full resize-none rounded-md border border-input bg-background px-3 py-2',
              'text-sm text-text placeholder:text-text-3',
              'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
              'disabled:cursor-not-allowed disabled:opacity-50',
              err === t('corrBadReason') ? 'border-bad-bd ring-bad-bd' : '',
            ].join(' ')}
          />
        </FormField>

        {/* Generic / API error message (banner) */}
        {err && err !== t('corrBadTime') && err !== t('corrBadReason') && (
          <div
            role="alert"
            className="flex items-center gap-2 rounded-md border border-bad-bd bg-bad-bg px-3 py-2.5 text-[13px] font-medium text-bad-tx"
          >
            {err}
          </div>
        )}

        {/* Context hint */}
        <p className="text-xs text-text-3">{t('corrWindowHint')}</p>
      </ModalBody>

      <ModalFooter>
        <Button
          variant="secondary"
          size="sm"
          disabled={create.isPending}
          onClick={() => onOpenChange(false)}
        >
          {t('cancel')}
        </Button>
        <Button
          variant="primary"
          size="sm"
          disabled={create.isPending}
          onClick={() => void onSubmit()}
        >
          {t('corrSubmitBtn')}
        </Button>
      </ModalFooter>
    </Modal>
  );
}
