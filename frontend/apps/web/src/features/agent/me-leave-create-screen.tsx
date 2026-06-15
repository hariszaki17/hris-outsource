/**
 * Ajukan Cuti — modal form to submit a new leave request (opened from /me/leave).
 *
 * F6.1 agent self-service create. Mirrors mobile validation (DATE_RE, badDate, badRange,
 * badReason) and server-error mapping (OVERLAPPING_LEAVE, QUOTA_EXCEEDED, BACKDATED_LEAVE,
 * MISSING_REQUIRED_DOCUMENT, INVALID_DATE_RANGE). Document-required types are excluded from
 * the picker (no upload support yet, LR-2 deferred). Rendered as a Modal (not a routed page) —
 * matches docs/design/brainstorm.pen "Agen Web · Ajukan Cuti (modal)".
 */
import { useCurrentUser } from '@/lib/use-auth.ts';
import { ApiError } from '@swp/api-client';
import { type ScheduleEntry, useGetScheduleByAgent } from '@swp/api-client/e4';
import {
  type LeaveTypeBalance,
  useCreateLeaveRequest,
  useGetEmployeeTypeBalances,
} from '@swp/api-client/e6';
import { addCalendarDays } from '@swp/shared/datetime';
import { statutoryFixedDays } from '@swp/shared/leave';
import {
  Button,
  FilterSelect,
  FormField,
  Input,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  StateView,
  useToast,
} from '@swp/ui';
import { Plane } from 'lucide-react';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

// ---------------------------------------------------------------------------
// Error code → i18n key map (mirrors mobile leave-new.tsx)
// ---------------------------------------------------------------------------

const SERVER_ERROR_MAP: Record<string, string> = {
  OVERLAPPING_LEAVE: 'leaveOverlap',
  QUOTA_EXCEEDED: 'leaveQuota',
  BACKDATED_LEAVE: 'leaveBackdated',
  MISSING_REQUIRED_DOCUMENT: 'leaveNeedDoc',
  INVALID_DATE_RANGE: 'leaveBadRange',
};

// ---------------------------------------------------------------------------
// Modal
// ---------------------------------------------------------------------------

export interface AgentLeaveCreateModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Called after a successful submit so the list can refetch. */
  onSuccess?: () => void;
}

export function AgentLeaveCreateModal({
  open,
  onOpenChange,
  onSuccess,
}: AgentLeaveCreateModalProps) {
  const { t } = useTranslation('agent');
  const { toast } = useToast();

  const user = useCurrentUser();
  const create = useCreateLeaveRequest();

  // The picker is scoped to the employee's HR-ASSIGNED types (ELE-1): the balance endpoint already
  // INNER JOINs the entitlements, so it returns only assigned types + their remaining quota.
  const balancesQ = useGetEmployeeTypeBalances(user?.employeeId ?? '', {
    query: { enabled: !!user?.employeeId },
  });
  const balances = (balancesQ.data?.data as { data?: LeaveTypeBalance[] } | undefined)?.data ?? [];

  // Document-required types stay hidden from the agent picker (upload deferred, PRD §10).
  const types = balances.filter((b) => !b.requires_document && b.applies_to !== 'HEAD_OFFICE');

  const [typeId, setTypeId] = useState('');
  const [start, setStart] = useState('');
  const [end, setEnd] = useState('');
  const [reason, setReason] = useState('');
  const [fieldErr, setFieldErr] = useState<
    Partial<Record<'type' | 'start' | 'end' | 'reason', string>>
  >({});

  const selectedBalance = balances.find((b) => b.leave_type_id === typeId);
  // remaining_days is null for uncapped types ("sesuai ketentuan") — no quota cap then.
  const remaining = selectedBalance?.remaining_days ?? null;

  // Server-authoritative duration = scheduled-shift days in [start,end] (workday OR weekend/holiday
  // shift); days with no shift / day-off are NOT counted. We preview it from the agent's schedule so
  // the agent sees the exact quota charge before submitting (and we block over-quota client-side).
  const rangeValid = DATE_RE.test(start) && DATE_RE.test(end) && end >= start;
  const scheduleQ = useGetScheduleByAgent(
    user?.employeeId ?? '',
    { start_date: start, end_date: end },
    { query: { enabled: !!user?.employeeId && rangeValid } },
  );
  const scheduleEntries =
    (scheduleQ.data?.data as { data?: ScheduleEntry[] } | undefined)?.data ?? [];
  const requestedDays = useMemo(() => {
    const dates = new Set<string>();
    for (const e of scheduleEntries) {
      if (!e.is_day_off && e.status !== 'CANCELLED_BY_LEAVE') dates.add(e.work_date);
    }
    return dates.size;
  }, [scheduleEntries]);

  const durationReady = rangeValid && !scheduleQ.isFetching;
  // Block over-quota (capped types only): requested working-days must fit the remaining balance.
  const overQuota =
    remaining != null && durationReady && requestedDays > remaining && requestedDays > 0;

  // Statutory fixed-duration types (UU 13/2003): pre-fill end_date to the regulated day count when
  // a start date is set. Editable down — the agent may still pick a shorter range.
  const statutoryDays = statutoryFixedDays(balances.find((b) => b.leave_type_id === typeId));
  function autoFillEnd(nextTypeId: string, nextStart: string) {
    const days = statutoryFixedDays(balances.find((b) => b.leave_type_id === nextTypeId));
    if (days != null && DATE_RE.test(nextStart)) {
      setEnd(addCalendarDays(nextStart, days - 1));
      setFieldErr((prev) => ({ ...prev, end: undefined }));
    }
  }

  function validate(): boolean {
    const errs: typeof fieldErr = {};
    if (!typeId) errs.type = t('leavePickType');
    if (!DATE_RE.test(start)) errs.start = t('leaveBadDate');
    if (!DATE_RE.test(end)) errs.end = t('leaveBadDate');
    if (DATE_RE.test(start) && DATE_RE.test(end) && end < start) errs.end = t('leaveBadRange');
    if (reason.trim().length < 5) errs.reason = t('leaveBadReason');
    setFieldErr(errs);
    return Object.keys(errs).length === 0;
  }

  async function onSubmit() {
    if (!validate()) return;
    if (overQuota) {
      toast({ tone: 'error', title: t('leaveOverQuota', { count: remaining ?? 0 }) });
      return;
    }
    try {
      await create.mutateAsync({
        data: {
          leave_type_id: typeId,
          start_date: start,
          end_date: end,
          reason: reason.trim(),
          submit: true,
        },
      });
      toast({ tone: 'success', title: t('leaveSuccess') });
      onOpenChange(false);
      onSuccess?.();
    } catch (e) {
      if (e instanceof ApiError) {
        const key = SERVER_ERROR_MAP[e.code];
        if (key) {
          toast({ tone: 'error', title: t(key) });
          return;
        }
      }
      toast({ tone: 'error', title: t('leaveError') });
    }
  }

  return (
    <Modal open={open} onOpenChange={onOpenChange} size="lg" className="w-[560px]">
      <ModalHeader icon={Plane} tone="brand" title={t('leaveNewBtn')} closeLabel={t('cancel')} />

      <ModalBody className="gap-5">
        {/* Leave type picker */}
        <div className="flex flex-col gap-1.5">
          <span className="text-sm font-medium text-text-2">
            {t('leaveType')}
            <span className="ml-0.5 text-bad">*</span>
          </span>
          {balancesQ.isLoading ? (
            <StateView kind="loading" title={t('loading')} />
          ) : balancesQ.isError ? (
            <StateView
              kind="error"
              title={t('errorGeneric')}
              onRetry={() => void balancesQ.refetch()}
            />
          ) : types.length === 0 ? (
            <p className="text-[13px] text-text-3">{t('leaveNoAssignedTypes')}</p>
          ) : (
            <FilterSelect
              containerClassName="w-full"
              value={typeId}
              aria-label={t('leaveType')}
              onChange={(e) => {
                const id = e.target.value;
                setTypeId(id);
                setFieldErr((prev) => ({ ...prev, type: undefined }));
                autoFillEnd(id, start);
              }}
            >
              <option value="" disabled>
                {t('leavePickType')}
              </option>
              {types.map((b) => (
                <option key={b.leave_type_id} value={b.leave_type_id}>
                  {b.name}
                  {b.code ? ` (${b.code})` : ''}
                </option>
              ))}
            </FilterSelect>
          )}
          {fieldErr.type && (
            <p role="alert" className="text-xs text-bad-tx">
              {fieldErr.type}
            </p>
          )}
        </div>

        {/* Date range */}
        <div className="grid grid-cols-2 gap-4">
          <FormField
            label={t('leaveStartDate')}
            htmlFor="leave-start"
            error={fieldErr.start}
            required
          >
            <Input
              id="leave-start"
              type="date"
              value={start}
              onChange={(e) => {
                setStart(e.target.value);
                setFieldErr((prev) => ({ ...prev, start: undefined }));
                autoFillEnd(typeId, e.target.value);
              }}
            />
          </FormField>

          <FormField label={t('leaveEndDate')} htmlFor="leave-end" error={fieldErr.end} required>
            <Input
              id="leave-end"
              type="date"
              value={end}
              min={start || undefined}
              onChange={(e) => {
                setEnd(e.target.value);
                setFieldErr((prev) => ({ ...prev, end: undefined }));
              }}
            />
          </FormField>
        </div>

        {statutoryDays != null && (
          <p className="-mt-2 text-xs text-text-3">
            {t('leaveStatutoryNote', { count: statutoryDays })}
          </p>
        )}

        {/* Duration (scheduled-shift days) vs remaining quota — over-quota blocks submit. */}
        {typeId && rangeValid && (
          <div className="-mt-2 flex flex-col gap-1 rounded-md border border-border bg-surface-2 px-3 py-2">
            <div className="flex items-center justify-between text-[13px]">
              <span className="text-text-2">{t('leaveDurationLabel')}</span>
              <span className="font-medium text-text tabular-nums">
                {scheduleQ.isFetching
                  ? t('loading')
                  : t('leaveDurationValue', { count: requestedDays })}
              </span>
            </div>
            <div className="flex items-center justify-between text-[13px]">
              <span className="text-text-2">{t('leaveRemainingLabel')}</span>
              <span className="font-medium text-text tabular-nums">
                {remaining == null
                  ? t('leaveUncapped')
                  : t('leaveRemainingValue', { count: remaining })}
              </span>
            </div>
            {overQuota && (
              <p role="alert" className="text-xs text-bad-tx">
                {t('leaveOverQuota', { count: remaining ?? 0 })}
              </p>
            )}
          </div>
        )}

        {/* Reason */}
        <FormField label={t('leaveReason')} htmlFor="leave-reason" error={fieldErr.reason} required>
          <textarea
            id="leave-reason"
            value={reason}
            rows={3}
            onChange={(e) => {
              setReason(e.target.value);
              setFieldErr((prev) => ({ ...prev, reason: undefined }));
            }}
            className="w-full rounded-md border border-border bg-surface px-3 py-2 text-sm text-text placeholder:text-text-3 focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 disabled:opacity-50 resize-none"
          />
        </FormField>
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
          disabled={create.isPending || overQuota}
          onClick={() => void onSubmit()}
        >
          {t('leaveSubmitBtn')}
        </Button>
      </ModalFooter>
    </Modal>
  );
}
