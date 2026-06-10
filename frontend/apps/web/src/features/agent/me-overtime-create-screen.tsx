import { useCurrentUser } from '@/lib/use-auth.ts';
/**
 * /me/overtime/new — Create an overtime request. Web port of apps/mobile/app/overtime-new.tsx.
 *
 * F7.1 / OC-1 — agent pre-requests OT (REQUESTED source, skips PENDING_AGENT_CONFIRM).
 * On success: toast + navigate back to /me/overtime.
 */
import { ApiError } from '@swp/api-client';
import { useCreateOvertimeRequest } from '@swp/api-client/e7';
import { Button, FormField, FormSection, Input, useToast } from '@swp/ui';
import { useNavigate } from '@tanstack/react-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AgentPage } from './agent-page.tsx';

// ---------------------------------------------------------------------------
// Validation constants (mirrors mobile overtime-new.tsx)
// ---------------------------------------------------------------------------

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;
const TIME_RE = /^([01]\d|2[0-3]):([0-5]\d)$/;

// ---------------------------------------------------------------------------
// Screen
// ---------------------------------------------------------------------------

export function AgentOvertimeCreateScreen() {
  const { t } = useTranslation('agent');
  const { toast } = useToast();
  const navigate = useNavigate();
  const user = useCurrentUser();
  const create = useCreateOvertimeRequest();

  const [date, setDate] = useState('');
  const [startTime, setStartTime] = useState('');
  const [endTime, setEndTime] = useState('');
  const [reason, setReason] = useState('');
  const [fieldErrors, setFieldErrors] = useState<{
    date?: string;
    time?: string;
    reason?: string;
  }>({});

  function validate(): boolean {
    const errs: typeof fieldErrors = {};
    if (!DATE_RE.test(date)) errs.date = t('otBadDate');
    if (!TIME_RE.test(startTime) || !TIME_RE.test(endTime)) errs.time = t('otBadTime');
    if (reason.trim().length < 5) errs.reason = t('otBadReason');
    setFieldErrors(errs);
    return Object.keys(errs).length === 0;
  }

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!validate()) return;

    try {
      await create.mutateAsync({
        data: {
          employee_id: user?.employeeId ?? '',
          work_date: date,
          planned_start_time: startTime,
          planned_end_time: endTime,
          reason: reason.trim(),
        },
      });
      toast({ tone: 'success', title: t('otSuccess') });
      void navigate({ to: '/me/overtime' });
    } catch (e) {
      if (e instanceof ApiError) {
        if (e.code === 'OT_OVERLAPS_LEAVE') {
          toast({ tone: 'error', title: t('otOverlapLeave') });
          return;
        }
        if (e.code === 'NO_ACTIVE_PLACEMENT') {
          toast({ tone: 'error', title: t('otNoPlacement') });
          return;
        }
      }
      toast({ tone: 'error', title: t('otError') });
    }
  }

  return (
    <AgentPage title={t('otNewBtn')} backTo="/me/overtime" backLabel={t('back')}>
      <form onSubmit={(e) => void onSubmit(e)} noValidate className="max-w-2xl">
        <div className="rounded-xl border border-border bg-surface p-6">
          <FormSection>
            {/* Work date */}
            <FormField
              label={t('otWorkDate')}
              htmlFor="ot-date"
              required
              error={fieldErrors.date}
              span={2}
            >
              <Input
                id="ot-date"
                type="date"
                value={date}
                onChange={(e) => {
                  setDate(e.target.value);
                  setFieldErrors((prev) => ({ ...prev, date: undefined }));
                }}
                aria-invalid={Boolean(fieldErrors.date)}
                aria-describedby={fieldErrors.date ? 'ot-date-error' : undefined}
              />
            </FormField>

            {/* Start time */}
            <FormField
              label={t('otStartTime')}
              htmlFor="ot-start"
              required
              error={fieldErrors.time}
            >
              <Input
                id="ot-start"
                type="time"
                value={startTime}
                onChange={(e) => {
                  setStartTime(e.target.value);
                  setFieldErrors((prev) => ({ ...prev, time: undefined }));
                }}
                aria-invalid={Boolean(fieldErrors.time)}
              />
            </FormField>

            {/* End time */}
            <FormField label={t('otEndTime')} htmlFor="ot-end" required>
              <Input
                id="ot-end"
                type="time"
                value={endTime}
                onChange={(e) => {
                  setEndTime(e.target.value);
                  setFieldErrors((prev) => ({ ...prev, time: undefined }));
                }}
                aria-invalid={Boolean(fieldErrors.time)}
              />
            </FormField>

            {/* Reason */}
            <FormField
              label={t('otReason')}
              htmlFor="ot-reason"
              required
              error={fieldErrors.reason}
              span={2}
            >
              <textarea
                id="ot-reason"
                rows={3}
                className="flex min-h-[80px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm text-text placeholder:text-text-3 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50 aria-[invalid=true]:border-bad-bd aria-[invalid=true]:ring-bad-bd"
                value={reason}
                onChange={(e) => {
                  setReason(e.target.value);
                  setFieldErrors((prev) => ({ ...prev, reason: undefined }));
                }}
                aria-invalid={Boolean(fieldErrors.reason)}
                aria-describedby={fieldErrors.reason ? 'ot-reason-error' : undefined}
              />
            </FormField>
          </FormSection>

          <div className="mt-6 flex items-center justify-end gap-3">
            <Button
              type="button"
              variant="ghost"
              onClick={() => void navigate({ to: '/me/overtime' })}
            >
              {t('cancel')}
            </Button>
            <Button type="submit" variant="primary" disabled={create.isPending}>
              {create.isPending ? t('loading') : t('otSubmitBtn')}
            </Button>
          </div>
        </div>
      </form>
    </AgentPage>
  );
}
