/**
 * Ajukan Koreksi — modal to file an attendance correction.
 *
 * Two modes:
 *   • EXISTING row (CHECK_IN / CHECK_OUT): correct the clock-in / clock-out time on an
 *     attendance record. Opened from a /me/attendance row or the searchable picker — receives
 *     `attendanceId` + `date`.
 *   • NEW_ENTRY: create an attendance record for a day with none (forgot to clock in/out, or a
 *     day with no configured shift). Opened from the picker's "Buat untuk tanggal tanpa jadwal"
 *     affordance — receives a `mode='new-entry'`; the agent fills work_date + jam masuk (+ keluar).
 *
 * On success → toast `corrSuccess` + close + onSuccess(). Error codes map to i18n:
 *   OUTSIDE_CORRECTION_WINDOW → corrOutsideWindow · CORRECTION_ALREADY_PENDING → corrAlreadyPending
 *   ATTENDANCE_ALREADY_EXISTS → corrAlreadyExists · NO_ACTIVE_PLACEMENT → corrNoPlacement · else corrError.
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
// missed/wrong clock-in/out case + NEW_ENTRY (a record for a day with none).
const EXISTING_TYPES: { value: CorrectionType; key: 'corrTypeCheckIn' | 'corrTypeCheckOut' }[] = [
  { value: 'CHECK_IN', key: 'corrTypeCheckIn' },
  { value: 'CHECK_OUT', key: 'corrTypeCheckOut' },
];

const TIME_RE = /^([01]\d|2[0-3]):([0-5]\d)$/;
const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

/** Asia/Jakarta is a fixed +07:00 offset (no DST) — safe to build the instant directly. */
function buildInstant(dateIso: string, time: string): string {
  return `${dateIso.slice(0, 10)}T${time.trim()}:00+07:00`;
}

export interface AgentCorrectionModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /**
   * 'existing' (default) corrects an attendance row (CHECK_IN/CHECK_OUT).
   * 'new-entry' creates a record for a day with none (NEW_ENTRY).
   */
  mode?: 'existing' | 'new-entry';
  /** Attendance row being corrected (existing mode only). */
  attendanceId?: string;
  /** ISO instant/date of the attendance row (used to build the corrected instant — existing mode). */
  date?: string;
  /** Called after a successful submit so the list can refetch. */
  onSuccess?: () => void;
}

export function AgentCorrectionModal({
  open,
  onOpenChange,
  mode = 'existing',
  attendanceId,
  date,
  onSuccess,
}: AgentCorrectionModalProps) {
  const { t } = useTranslation('agent');
  const { toast } = useToast();
  const create = useCreateCorrection();
  const isNewEntry = mode === 'new-entry';

  // Existing-mode: which field to fix. New-entry: both times collected.
  const [type, setType] = useState<CorrectionType>('CHECK_OUT');
  const [workDate, setWorkDate] = useState('');
  const [time, setTime] = useState(''); // existing mode: the single corrected time
  const [checkIn, setCheckIn] = useState(''); // new-entry: jam masuk
  const [checkOut, setCheckOut] = useState(''); // new-entry: jam keluar (optional)
  const [reason, setReason] = useState('');
  const [err, setErr] = useState<string | null>(null);

  function fail(key: string) {
    setErr(t(key));
  }

  async function onSubmit() {
    setErr(null);

    let data: CorrectionWriteRequest;

    if (isNewEntry) {
      if (!DATE_RE.test(workDate.trim())) {
        fail('corrBadDate');
        return;
      }
      if (!TIME_RE.test(checkIn.trim())) {
        fail('corrBadTime');
        return;
      }
      if (checkOut.trim() && !TIME_RE.test(checkOut.trim())) {
        fail('corrBadTime');
        return;
      }
      if (reason.trim().length < 5) {
        fail('corrBadReason');
        return;
      }
      data = {
        type: 'NEW_ENTRY',
        work_date: workDate.trim(),
        reason: reason.trim(),
        proposed_check_in_at: buildInstant(workDate.trim(), checkIn),
        proposed_check_out_at: checkOut.trim()
          ? buildInstant(workDate.trim(), checkOut)
          : undefined,
      };
    } else {
      if (!TIME_RE.test(time.trim())) {
        fail('corrBadTime');
        return;
      }
      if (reason.trim().length < 5) {
        fail('corrBadReason');
        return;
      }
      const iso = buildInstant(date ?? '', time);
      data = {
        attendance_id: attendanceId,
        type,
        reason: reason.trim(),
        proposed_check_in_at: type === 'CHECK_IN' ? iso : undefined,
        proposed_check_out_at: type === 'CHECK_OUT' ? iso : undefined,
      };
    }

    try {
      await create.mutateAsync({ data });
      toast({ tone: 'success', title: t('corrSuccess') });
      onOpenChange(false);
      onSuccess?.();
    } catch (e) {
      if (e instanceof ApiError) {
        switch (e.code) {
          case 'OUTSIDE_CORRECTION_WINDOW':
          case 'OUTSIDE_WINDOW':
            fail('corrOutsideWindow');
            return;
          case 'CORRECTION_ALREADY_PENDING':
          case 'ALREADY_PENDING':
            fail('corrAlreadyPending');
            return;
          case 'ATTENDANCE_ALREADY_EXISTS':
            fail('corrAlreadyExists');
            return;
          case 'NO_ACTIVE_PLACEMENT':
            fail('corrNoPlacement');
            return;
        }
      }
      fail('corrError');
    }
  }

  const title = isNewEntry ? t('corrNewEntryTitle') : t('corrTitle');

  return (
    <Modal open={open} onOpenChange={onOpenChange} size="lg" className="w-[540px]">
      <ModalHeader icon={PencilLine} tone="brand" title={title} closeLabel={t('cancel')} />

      <ModalBody className="gap-5">
        {isNewEntry ? (
          <>
            {/* Work date */}
            <FormField
              label={t('corrWorkDate')}
              htmlFor="correction-work-date"
              error={err === t('corrBadDate') ? err : undefined}
              required
            >
              <Input
                id="correction-work-date"
                type="date"
                value={workDate}
                onChange={(e) => setWorkDate(e.target.value)}
                aria-invalid={err === t('corrBadDate') ? true : undefined}
              />
            </FormField>

            {/* Times: jam masuk (required) + jam keluar (optional) */}
            <div className="flex gap-4">
              <FormField
                label={t('corrCheckInTime')}
                htmlFor="correction-check-in"
                error={err === t('corrBadTime') ? err : undefined}
                required
                className="flex-1"
              >
                <Input
                  id="correction-check-in"
                  type="time"
                  value={checkIn}
                  onChange={(e) => setCheckIn(e.target.value)}
                  aria-invalid={err === t('corrBadTime') ? true : undefined}
                />
              </FormField>
              <FormField
                label={t('corrCheckOutTime')}
                htmlFor="correction-check-out"
                className="flex-1"
              >
                <Input
                  id="correction-check-out"
                  type="time"
                  value={checkOut}
                  onChange={(e) => setCheckOut(e.target.value)}
                />
              </FormField>
            </div>
          </>
        ) : (
          <>
            {/* Type toggle */}
            <div className="flex flex-col gap-1.5">
              <span className="text-sm font-medium text-text-2">{t('corrTitle')}</span>
              <div className="flex gap-3">
                {EXISTING_TYPES.map((opt) => (
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
          </>
        )}

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
        {err &&
          err !== t('corrBadTime') &&
          err !== t('corrBadReason') &&
          err !== t('corrBadDate') && (
            <div
              role="alert"
              className="flex items-center gap-2 rounded-md border border-bad-bd bg-bad-bg px-3 py-2.5 text-[13px] font-medium text-bad-tx"
            >
              {err}
            </div>
          )}

        {/* Context hint */}
        <p className="text-xs text-text-3">
          {isNewEntry ? t('corrNewEntryHint') : t('corrWindowHint')}
        </p>
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
