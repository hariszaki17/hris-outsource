/**
 * /me/correction — File an attendance correction (search params: attendanceId, date).
 *
 * Web port of apps/mobile/app/correction.tsx.
 * Receives attendanceId and date via TanStack Router search params (validated in router.tsx).
 * On success → toast `corrSuccess` + navigate to /me/attendance.
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
import { Button, FormField, Input, StateView, useToast } from '@swp/ui';
import { useNavigate, useSearch } from '@tanstack/react-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AgentPage } from './agent-page.tsx';

// CODE corrections need an attendance-code picker — deferred; MVP covers the common
// missed/wrong clock-in/out case.
const TYPES: { value: CorrectionType; key: 'corrTypeCheckIn' | 'corrTypeCheckOut' }[] = [
  { value: 'CHECK_IN', key: 'corrTypeCheckIn' },
  { value: 'CHECK_OUT', key: 'corrTypeCheckOut' },
];

export function AgentCorrectionScreen() {
  const { t } = useTranslation('agent');
  const { toast } = useToast();
  const navigate = useNavigate();

  // Read search params from the validated meCorrectionRoute.
  const { attendanceId, date } = useSearch({ strict: false }) as {
    attendanceId?: string;
    date?: string;
  };

  const create = useCreateCorrection();

  const [type, setType] = useState<CorrectionType>('CHECK_OUT');
  const [time, setTime] = useState('');
  const [reason, setReason] = useState('');
  const [err, setErr] = useState<string | null>(null);

  // Guard: this screen requires attendanceId from navigation state.
  if (!attendanceId) {
    return (
      <AgentPage title={t('corrTitle')} backTo="/me/attendance" backLabel={t('back')}>
        <StateView kind="empty" title={t('corrError')} />
      </AgentPage>
    );
  }

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
      attendance_id: attendanceId!,
      type,
      reason: reason.trim(),
      proposed_check_in_at: type === 'CHECK_IN' ? iso : undefined,
      proposed_check_out_at: type === 'CHECK_OUT' ? iso : undefined,
    };
    try {
      await create.mutateAsync({ data });
      toast({ tone: 'success', title: t('corrSuccess') });
      void navigate({ to: '/me/attendance' });
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
    <AgentPage title={t('corrTitle')} backTo="/me/attendance" backLabel={t('back')}>
      <div className="max-w-2xl rounded-xl border border-border bg-surface p-6">
        <div className="flex flex-col gap-5">
          {/* Type toggle */}
          <div>
            <p className="mb-2 text-sm font-semibold text-text">{t('corrTitle')}</p>
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

          {/* Reason textarea — uses a plain <textarea> styled to match Input since @swp/ui
              does not ship a Textarea primitive; the Input atom wraps <input> only. */}
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

          {/* Generic / API error message */}
          {err && err !== t('corrBadTime') && err !== t('corrBadReason') && (
            <p role="alert" className="text-sm text-bad-tx">
              {err}
            </p>
          )}

          {/* Footer: Cancel + Submit */}
          <div className="flex items-center justify-end gap-3 border-t border-border pt-4">
            <Button variant="secondary" onClick={() => void navigate({ to: '/me/attendance' })}>
              {t('cancel')}
            </Button>
            <Button variant="primary" disabled={create.isPending} onClick={() => void onSubmit()}>
              {t('corrSubmitBtn')}
            </Button>
          </div>
        </div>
      </div>
    </AgentPage>
  );
}
