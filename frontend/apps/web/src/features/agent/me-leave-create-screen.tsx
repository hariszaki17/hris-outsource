/**
 * Ajukan Cuti — modal form to submit a new leave request (opened from /me/leave).
 *
 * F6.1 agent self-service create. Mirrors mobile validation (DATE_RE, badDate, badRange,
 * badReason) and server-error mapping (OVERLAPPING_LEAVE, QUOTA_EXCEEDED, BACKDATED_LEAVE,
 * MISSING_REQUIRED_DOCUMENT, INVALID_DATE_RANGE). Document-required types are excluded from
 * the picker (no upload support yet, LR-2 deferred). Rendered as a Modal (not a routed page) —
 * matches docs/design/brainstorm.pen "Agen Web · Ajukan Cuti (modal)".
 */
import { ApiError } from '@swp/api-client';
import { type LeaveType, useCreateLeaveRequest, useListLeaveTypes } from '@swp/api-client/e6';
import {
  Button,
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
import { useState } from 'react';
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

  const typesQ = useListLeaveTypes();
  const create = useCreateLeaveRequest();

  const allTypes = (typesQ.data?.data as { data?: LeaveType[] } | undefined)?.data ?? [];
  // Exclude document-required types (no attachment upload in v1, LR-2 deferred).
  const types = allTypes.filter((lt) => lt.active && !lt.is_document_required);

  const [typeId, setTypeId] = useState('');
  const [start, setStart] = useState('');
  const [end, setEnd] = useState('');
  const [reason, setReason] = useState('');
  const [fieldErr, setFieldErr] = useState<
    Partial<Record<'type' | 'start' | 'end' | 'reason', string>>
  >({});

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
          {typesQ.isLoading ? (
            <StateView kind="loading" title={t('loading')} />
          ) : typesQ.isError ? (
            <StateView
              kind="error"
              title={t('errorGeneric')}
              onRetry={() => void typesQ.refetch()}
            />
          ) : (
            <div className="flex flex-wrap gap-2">
              {types.map((lt) => (
                <button
                  key={lt.id}
                  type="button"
                  onClick={() => {
                    setTypeId(lt.id);
                    setFieldErr((prev) => ({ ...prev, type: undefined }));
                  }}
                  className={[
                    'rounded-md border px-3 py-2 text-sm font-medium transition-colors',
                    typeId === lt.id
                      ? 'border-primary bg-primary text-primary-foreground'
                      : 'border-border bg-surface text-text-2 hover:bg-muted',
                  ].join(' ')}
                >
                  {lt.name}
                </button>
              ))}
            </div>
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
          disabled={create.isPending}
          onClick={() => void onSubmit()}
        >
          {t('leaveSubmitBtn')}
        </Button>
      </ModalFooter>
    </Modal>
  );
}
