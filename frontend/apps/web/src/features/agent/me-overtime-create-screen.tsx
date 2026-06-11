import { useCurrentUser } from '@/lib/use-auth.ts';
/**
 * Ajukan Lembur — modal form to create an overtime request (opened from /me/overtime).
 *
 * F7.1 / OC-1 — agent pre-requests OT (REQUESTED source, skips PENDING_AGENT_CONFIRM).
 * Rendered as a Modal (not a routed page) — matches docs/design/brainstorm.pen
 * "Agen Web · Ajukan Lembur (modal)".
 */
import { ApiError } from '@swp/api-client';
import { useCreateOvertimeRequest } from '@swp/api-client/e7';
import {
  Button,
  FormField,
  FormSection,
  Input,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  useToast,
} from '@swp/ui';
import { Timer } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// Validation constants (mirrors mobile overtime-new.tsx)
// ---------------------------------------------------------------------------

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;
const TIME_RE = /^([01]\d|2[0-3]):([0-5]\d)$/;

// ---------------------------------------------------------------------------
// Modal
// ---------------------------------------------------------------------------

export interface AgentOvertimeCreateModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Called after a successful submit so the list can refetch. */
  onSuccess?: () => void;
}

export function AgentOvertimeCreateModal({
  open,
  onOpenChange,
  onSuccess,
}: AgentOvertimeCreateModalProps) {
  const { t } = useTranslation('agent');
  const { toast } = useToast();
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

  async function onSubmit() {
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
      onOpenChange(false);
      onSuccess?.();
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
    <Modal open={open} onOpenChange={onOpenChange} size="lg" className="w-[560px]">
      <ModalHeader icon={Timer} tone="brand" title={t('otNewBtn')} closeLabel={t('cancel')} />

      <ModalBody>
        <FormSection>
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
            />
          </FormField>

          <FormField label={t('otStartTime')} htmlFor="ot-start" required error={fieldErrors.time}>
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
            />
          </FormField>
        </FormSection>
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
          {create.isPending ? t('loading') : t('otSubmitBtn')}
        </Button>
      </ModalFooter>
    </Modal>
  );
}
