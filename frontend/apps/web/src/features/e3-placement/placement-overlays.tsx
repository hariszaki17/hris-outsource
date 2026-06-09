/**
 * E3 · Penempatan — overlay/modal layer for placement lifecycle actions +
 * shift-leader assignment flows.
 *
 * .pen frames:
 *   BMENY  E3 · Overlay — Transfer Penempatan      (ModalTransfer)
 *   JSO5b  E3 · Overlay — Transfer + Replacement   (ModalReplacement)
 *   hwFaA  E3 · Overlay — Perpanjang Penempatan    (ModalRenew)
 *   comp/ModalDestructive V4LG8 via ConfirmDialog tone="danger"
 *
 * Exports:
 *   TransferModal        — useTransferPlacement     vars { id, data }
 *   RenewModal           — useRenewPlacement         vars { id, data }
 *   EndConfirm           — useEndPlacement           vars { id, data }
 *   TerminateConfirm     — useTerminatePlacement     vars { id, data } + company-name retype
 *   ResignModal          — useResignPlacement        vars { id, data }
 *   ShiftLeaderAssignModal   — useCreateShiftLeaderAssignment   vars { data }
 *   ShiftLeaderReplaceModal  — useReplaceShiftLeaderAssignment  vars { id, data }
 *   ShiftLeaderEndConfirm    — useEndShiftLeaderAssignment      vars { id, data }
 *
 * INV-2/3/4 violations → inline Banner (kind 'invariant'|'conflict') via classifyError.
 * i18n namespace: placementDetail.
 */

import { ClientCompanyPicker } from '@/features/e2-identity/pickers/client-company-picker.tsx';
import { CompanyLeaderCandidatePicker } from '@/features/e2-identity/pickers/company-leader-picker.tsx';
import { PositionPicker } from '@/features/e2-identity/pickers/position-picker.tsx';
import { ServiceLinePicker } from '@/features/e2-identity/pickers/service-line-picker.tsx';
import { applyFieldErrors, classifyError } from '@/lib/api-error.ts';
import {
  type EndRequest,
  EndRequestReason,
  type RenewRequest,
  type ResignRequest,
  type ShiftLeaderAssignmentEndRequest,
  type ShiftLeaderAssignmentReplaceRequest,
  type ShiftLeaderAssignmentWriteRequest,
  type TerminateRequest,
  type TransferRequest,
  useCreateShiftLeaderAssignment,
  useEndPlacement,
  useEndShiftLeaderAssignment,
  useRenewPlacement,
  useReplaceShiftLeaderAssignment,
  useResignPlacement,
  useTerminatePlacement,
  useTransferPlacement,
} from '@swp/api-client/e3';
import {
  Banner,
  Button,
  ConfirmDialog,
  FormField,
  FormSection,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  useToast,
} from '@swp/ui';
import { ArrowLeftRight, CheckCircle, RefreshCw, SquareX, UserMinus, UserPlus } from 'lucide-react';
import { useEffect, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

export interface PlacementInfo {
  id: string;
  employee_name: string;
  client_company_id: string;
  client_company_name: string;
  service_line_name: string;
  position_name: string;
  start_date: string;
  end_date?: string | null;
}

// ---------------------------------------------------------------------------
// TransferModal  (.pen BMENY · ModalTransfer)
// ---------------------------------------------------------------------------

export interface TransferModalProps {
  open: boolean;
  onClose: () => void;
  placement: PlacementInfo;
}

interface TransferFormValues {
  new_client_company_id: string;
  new_service_line_id: string;
  new_position_id: string;
  new_start_date: string;
  new_end_date: string;
  transfer_reason: string;
}

export function TransferModal({ open, onClose, placement }: TransferModalProps) {
  const { t } = useTranslation('placementDetail');
  const { toast } = useToast();
  const [bannerMsg, setBannerMsg] = useState<string | null>(null);
  const [bannerTone, setBannerTone] = useState<'warn' | 'bad' | 'info'>('bad');
  // Track selected service line to filter position picker
  const [selectedServiceLineId, setSelectedServiceLineId] = useState<string | null>(null);

  const mutation = useTransferPlacement();

  const {
    register,
    handleSubmit,
    control,
    reset,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<TransferFormValues>({
    defaultValues: {
      new_client_company_id: '',
      new_service_line_id: '',
      new_position_id: '',
      new_start_date: '',
      new_end_date: '',
      transfer_reason: '',
    },
  });

  useEffect(() => {
    if (!open) {
      reset();
      setBannerMsg(null);
    }
  }, [open, reset]);

  function handleClose() {
    reset();
    setBannerMsg(null);
    setSelectedServiceLineId(null);
    onClose();
  }

  async function onSubmit(values: TransferFormValues) {
    setBannerMsg(null);
    const data: TransferRequest = {
      new_client_company_id: values.new_client_company_id,
      new_service_line_id: values.new_service_line_id,
      new_position_id: values.new_position_id,
      new_start_date: values.new_start_date,
      new_end_date: values.new_end_date || null,
      transfer_reason: values.transfer_reason,
    };
    try {
      await mutation.mutateAsync({ id: placement.id, data });
      toast({
        tone: 'success',
        title: t('transfer.successTitle'),
        description: t('transfer.successDesc'),
      });
      handleClose();
    } catch (err) {
      const classified = classifyError(err);
      if (classified.kind === 'invariant' || classified.kind === 'conflict') {
        setBannerTone('warn');
        setBannerMsg(classified.message);
      } else if (classified.kind === 'validation') {
        applyFieldErrors(err, setError as Parameters<typeof applyFieldErrors>[1]);
      } else {
        setBannerTone('bad');
        setBannerMsg(classified.message);
      }
    }
  }

  return (
    <Modal
      open={open}
      onOpenChange={(v) => {
        if (!v) handleClose();
      }}
      size="lg"
    >
      <form onSubmit={handleSubmit(onSubmit)} noValidate>
        <ModalHeader icon={ArrowLeftRight} tone="warn" title={t('transfer.modalTitle')} />
        <ModalBody>
          {/* Current placement summary (.pen xRvs3) */}
          <div className="flex items-center gap-2.5 rounded-xl border border-border-soft bg-surface-2 px-[14px] py-[10px]">
            <ArrowLeftRight className="size-4 shrink-0 text-text-2" aria-hidden="true" />
            <div className="flex flex-col gap-0.5">
              <span className="text-[12px] font-semibold text-text">
                {placement.client_company_name}
              </span>
              <span className="text-[11px] text-text-3">
                {placement.service_line_name} · {placement.position_name}
              </span>
            </div>
          </div>

          {bannerMsg != null && <Banner tone={bannerTone} title={bannerMsg} />}

          <p className="text-[11px] font-bold uppercase tracking-[0.5px] text-text-3">
            {t('transfer.newPlacementLabel')}
          </p>

          <FormSection>
            <FormField
              label={t('transfer.newCompany')}
              htmlFor="tf-company"
              required
              error={errors.new_client_company_id?.message}
            >
              <Controller
                control={control}
                name="new_client_company_id"
                rules={{ required: t('validation.required') }}
                render={({ field }) => (
                  <ClientCompanyPicker
                    value={field.value || null}
                    onChange={(v) => field.onChange(v ?? '')}
                    error={!!errors.new_client_company_id}
                  />
                )}
              />
            </FormField>
            <FormField
              label={t('transfer.newServiceLine')}
              htmlFor="tf-sl"
              required
              error={errors.new_service_line_id?.message}
            >
              <Controller
                control={control}
                name="new_service_line_id"
                rules={{ required: t('validation.required') }}
                render={({ field }) => (
                  <ServiceLinePicker
                    value={field.value || null}
                    onChange={(v) => {
                      field.onChange(v ?? '');
                      setSelectedServiceLineId(v ?? null);
                    }}
                    error={!!errors.new_service_line_id}
                  />
                )}
              />
            </FormField>
            <FormField
              label={t('transfer.newPosition')}
              htmlFor="tf-pos"
              required
              error={errors.new_position_id?.message}
            >
              <Controller
                control={control}
                name="new_position_id"
                rules={{ required: t('validation.required') }}
                render={({ field }) => (
                  <PositionPicker
                    value={field.value || null}
                    onChange={(v) => field.onChange(v ?? '')}
                    serviceLineId={selectedServiceLineId}
                    error={!!errors.new_position_id}
                  />
                )}
              />
            </FormField>
            <FormField
              label={t('transfer.newStartDate')}
              htmlFor="tf-start"
              required
              error={errors.new_start_date?.message}
            >
              <input
                id="tf-start"
                type="date"
                className="h-9 w-full rounded-lg border border-border bg-surface px-3 text-sm text-text focus:outline-none focus:ring-2 focus:ring-ring"
                {...register('new_start_date', { required: t('validation.required') })}
              />
            </FormField>
            <FormField
              label={t('transfer.newEndDate')}
              htmlFor="tf-end"
              error={errors.new_end_date?.message}
            >
              <input
                id="tf-end"
                type="date"
                className="h-9 w-full rounded-lg border border-border bg-surface px-3 text-sm text-text focus:outline-none focus:ring-2 focus:ring-ring"
                {...register('new_end_date')}
              />
            </FormField>
            <FormField
              label={t('transfer.reason')}
              htmlFor="tf-reason"
              required
              error={errors.transfer_reason?.message}
              span={2}
            >
              <textarea
                id="tf-reason"
                rows={3}
                className="w-full resize-none rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text placeholder:text-text-3 focus:outline-none focus:ring-2 focus:ring-ring"
                placeholder={t('transfer.reasonPlaceholder')}
                {...register('transfer_reason', {
                  required: t('validation.required'),
                  minLength: { value: 5, message: t('validation.minLength5') },
                })}
              />
            </FormField>
          </FormSection>

          {/* 1-day buffer hint (.pen bgOHd) */}
          <p className="flex items-center gap-1.5 text-xs text-text-3">
            <span aria-hidden="true">ⓘ</span>
            {t('transfer.bufferHint')}
          </p>
        </ModalBody>
        <ModalFooter>
          <Button type="button" variant="secondary" onClick={handleClose}>
            {t('common.cancel')}
          </Button>
          <Button type="submit" variant="primary" disabled={isSubmitting || mutation.isPending}>
            <ArrowLeftRight className="mr-1.5 size-4" aria-hidden="true" />
            {t('transfer.confirmBtn')}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// RenewModal  (.pen hwFaA · ModalRenew)
// ---------------------------------------------------------------------------

export interface RenewModalProps {
  open: boolean;
  onClose: () => void;
  placement: PlacementInfo;
}

interface RenewFormValues {
  new_start_date: string;
  new_end_date: string;
  notes: string;
}

export function RenewModal({ open, onClose, placement }: RenewModalProps) {
  const { t } = useTranslation('placementDetail');
  const { toast } = useToast();
  const [bannerMsg, setBannerMsg] = useState<string | null>(null);
  const [bannerTone, setBannerTone] = useState<'warn' | 'bad' | 'info'>('bad');

  const mutation = useRenewPlacement();

  const {
    register,
    handleSubmit,
    reset,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<RenewFormValues>({
    defaultValues: { new_start_date: '', new_end_date: '', notes: '' },
  });

  useEffect(() => {
    if (!open) {
      reset();
      setBannerMsg(null);
    }
  }, [open, reset]);

  function handleClose() {
    reset();
    setBannerMsg(null);
    onClose();
  }

  async function onSubmit(values: RenewFormValues) {
    setBannerMsg(null);
    const data: RenewRequest = {
      new_start_date: values.new_start_date,
      new_end_date: values.new_end_date || null,
      notes: values.notes || null,
    };
    try {
      await mutation.mutateAsync({ id: placement.id, data });
      toast({ tone: 'success', title: t('renew.successTitle') });
      handleClose();
    } catch (err) {
      const classified = classifyError(err);
      if (classified.kind === 'invariant' || classified.kind === 'conflict') {
        setBannerTone('warn');
        setBannerMsg(classified.message);
      } else if (classified.kind === 'validation') {
        applyFieldErrors(err, setError as Parameters<typeof applyFieldErrors>[1]);
        setBannerTone('bad');
        setBannerMsg(t('renew.overlapError'));
      } else {
        setBannerTone('bad');
        setBannerMsg(classified.message);
      }
    }
  }

  return (
    <Modal
      open={open}
      onOpenChange={(v) => {
        if (!v) handleClose();
      }}
      size="md"
    >
      <form onSubmit={handleSubmit(onSubmit)} noValidate>
        <ModalHeader icon={RefreshCw} tone="brand" title={t('renew.modalTitle')} />
        <ModalBody>
          {/* Current period card (.pen ySDMz) */}
          <div className="flex items-center gap-2.5 rounded-xl border border-border-soft bg-surface-2 px-[14px] py-[10px]">
            <RefreshCw className="size-4 shrink-0 text-text-2" aria-hidden="true" />
            <div className="flex flex-col gap-0.5">
              <span className="text-[12px] font-semibold text-text">
                {placement.client_company_name}
              </span>
              <span className="text-[11px] text-text-3">
                {placement.start_date} – {placement.end_date ?? t('renew.openEnded')}
              </span>
            </div>
          </div>

          {bannerMsg != null && <Banner tone={bannerTone} title={bannerMsg} />}

          <FormSection>
            <FormField
              label={t('renew.newStartDate')}
              htmlFor="rn-start"
              required
              error={errors.new_start_date?.message}
            >
              <input
                id="rn-start"
                type="date"
                className="h-9 w-full rounded-lg border border-border bg-surface px-3 text-sm text-text focus:outline-none focus:ring-2 focus:ring-ring"
                {...register('new_start_date', { required: t('validation.required') })}
              />
            </FormField>
            <FormField
              label={t('renew.newEndDate')}
              htmlFor="rn-end"
              error={errors.new_end_date?.message}
            >
              <input
                id="rn-end"
                type="date"
                className="h-9 w-full rounded-lg border border-border bg-surface px-3 text-sm text-text focus:outline-none focus:ring-2 focus:ring-ring"
                {...register('new_end_date')}
              />
            </FormField>
            <FormField
              label={t('renew.notes')}
              htmlFor="rn-notes"
              error={errors.notes?.message}
              span={2}
            >
              <textarea
                id="rn-notes"
                rows={3}
                className="w-full resize-none rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text placeholder:text-text-3 focus:outline-none focus:ring-2 focus:ring-ring"
                placeholder={t('renew.notesPlaceholder')}
                {...register('notes')}
              />
            </FormField>
          </FormSection>

          {/* Info banner (.pen P7QN3X) */}
          <Banner tone="info" title={t('renew.infoHint')} />
        </ModalBody>
        <ModalFooter>
          <Button type="button" variant="secondary" onClick={handleClose}>
            {t('common.cancel')}
          </Button>
          <Button type="submit" variant="primary" disabled={isSubmitting || mutation.isPending}>
            <CheckCircle className="mr-1.5 size-4" aria-hidden="true" />
            {t('renew.confirmBtn')}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// EndConfirm  (soft end, reason enum)
// ---------------------------------------------------------------------------

export interface EndConfirmProps {
  open: boolean;
  onClose: () => void;
  placement: PlacementInfo;
}

interface EndFormValues {
  reason: EndRequest['reason'];
  effective_date: string;
  notes: string;
}

const END_REASON_OPTIONS: Array<{ value: EndRequest['reason']; labelKey: string }> = [
  { value: EndRequestReason.END_OF_TERM, labelKey: 'end.reasonEndOfTerm' },
  { value: EndRequestReason.MUTUAL_AGREEMENT, labelKey: 'end.reasonMutual' },
  { value: EndRequestReason.CLIENT_REQUEST, labelKey: 'end.reasonClient' },
  { value: EndRequestReason.OTHER, labelKey: 'end.reasonOther' },
];

export function EndConfirm({ open, onClose, placement }: EndConfirmProps) {
  const { t } = useTranslation('placementDetail');
  const { toast } = useToast();
  const [bannerMsg, setBannerMsg] = useState<string | null>(null);

  const mutation = useEndPlacement();

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<EndFormValues>({
    defaultValues: { reason: EndRequestReason.END_OF_TERM, effective_date: '', notes: '' },
  });

  useEffect(() => {
    if (!open) {
      reset();
      setBannerMsg(null);
    }
  }, [open, reset]);

  function handleClose() {
    reset();
    setBannerMsg(null);
    onClose();
  }

  async function onSubmit(values: EndFormValues) {
    setBannerMsg(null);
    const data: EndRequest = {
      reason: values.reason,
      effective_date: values.effective_date,
      notes: values.notes || null,
    };
    try {
      await mutation.mutateAsync({ id: placement.id, data });
      toast({ tone: 'success', title: t('end.successTitle') });
      handleClose();
    } catch (err) {
      setBannerMsg(classifyError(err).message);
    }
  }

  return (
    <Modal
      open={open}
      onOpenChange={(v) => {
        if (!v) handleClose();
      }}
      size="sm"
    >
      <form onSubmit={handleSubmit(onSubmit)} noValidate>
        <ModalHeader icon={SquareX} tone="warn" title={t('end.modalTitle')} />
        <ModalBody>
          {bannerMsg != null && <Banner tone="bad" title={bannerMsg} />}
          <FormField
            label={t('end.reason')}
            htmlFor="end-reason"
            required
            error={errors.reason?.message}
          >
            <select
              id="end-reason"
              className="h-9 w-full rounded-lg border border-border bg-surface px-3 text-sm text-text focus:outline-none focus:ring-2 focus:ring-ring"
              {...register('reason', { required: t('validation.required') })}
            >
              {END_REASON_OPTIONS.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {t(opt.labelKey as Parameters<typeof t>[0])}
                </option>
              ))}
            </select>
          </FormField>
          <FormField
            label={t('end.effectiveDate')}
            htmlFor="end-date"
            required
            error={errors.effective_date?.message}
          >
            <input
              id="end-date"
              type="date"
              className="h-9 w-full rounded-lg border border-border bg-surface px-3 text-sm text-text focus:outline-none focus:ring-2 focus:ring-ring"
              {...register('effective_date', { required: t('validation.required') })}
            />
          </FormField>
          <FormField label={t('end.notes')} htmlFor="end-notes" error={errors.notes?.message}>
            <textarea
              id="end-notes"
              rows={3}
              className="w-full resize-none rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text placeholder:text-text-3 focus:outline-none focus:ring-2 focus:ring-ring"
              placeholder={t('end.notesPlaceholder')}
              {...register('notes')}
            />
          </FormField>
        </ModalBody>
        <ModalFooter>
          <Button type="button" variant="secondary" onClick={handleClose}>
            {t('common.cancel')}
          </Button>
          <Button type="submit" variant="primary" disabled={isSubmitting || mutation.isPending}>
            {t('end.confirmBtn')}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// TerminateConfirm  (destructive, company-name retype — V4LG8 pattern)
// ---------------------------------------------------------------------------

export interface TerminateConfirmProps {
  open: boolean;
  onClose: () => void;
  placement: PlacementInfo;
}

interface TerminateFormValues {
  termination_reason: string;
  effective_date: string;
  type_company_name_confirm: string;
}

export function TerminateConfirm({ open, onClose, placement }: TerminateConfirmProps) {
  const { t } = useTranslation('placementDetail');
  const { toast } = useToast();
  const [bannerMsg, setBannerMsg] = useState<string | null>(null);

  const mutation = useTerminatePlacement();

  const {
    register,
    handleSubmit,
    watch,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<TerminateFormValues>({
    defaultValues: { termination_reason: '', effective_date: '', type_company_name_confirm: '' },
  });

  const confirmValue = watch('type_company_name_confirm');
  const confirmMatch =
    confirmValue.trim().toLowerCase() === placement.client_company_name.trim().toLowerCase();

  useEffect(() => {
    if (!open) {
      reset();
      setBannerMsg(null);
    }
  }, [open, reset]);

  function handleClose() {
    reset();
    setBannerMsg(null);
    onClose();
  }

  async function onSubmit(values: TerminateFormValues) {
    setBannerMsg(null);
    const data: TerminateRequest = {
      termination_reason: values.termination_reason,
      effective_date: values.effective_date || null,
      type_company_name_confirm: values.type_company_name_confirm,
    };
    try {
      await mutation.mutateAsync({ id: placement.id, data });
      toast({ tone: 'success', title: t('terminate.successTitle') });
      handleClose();
    } catch (err) {
      setBannerMsg(classifyError(err).message);
    }
  }

  return (
    <Modal
      open={open}
      onOpenChange={(v) => {
        if (!v) handleClose();
      }}
      size="sm"
    >
      <form onSubmit={handleSubmit(onSubmit)} noValidate>
        <ModalHeader icon={SquareX} tone="danger" title={t('terminate.modalTitle')} />
        <ModalBody>
          {bannerMsg != null && <Banner tone="bad" title={bannerMsg} />}
          <FormField
            label={t('terminate.reason')}
            htmlFor="term-reason"
            required
            error={errors.termination_reason?.message}
          >
            <textarea
              id="term-reason"
              rows={4}
              className="w-full resize-none rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text placeholder:text-text-3 focus:outline-none focus:ring-2 focus:ring-ring"
              placeholder={t('terminate.reasonPlaceholder')}
              {...register('termination_reason', {
                required: t('validation.required'),
                minLength: { value: 10, message: t('validation.minLength10') },
              })}
            />
          </FormField>
          <FormField
            label={t('terminate.effectiveDate')}
            htmlFor="term-date"
            error={errors.effective_date?.message}
          >
            <input
              id="term-date"
              type="date"
              className="h-9 w-full rounded-lg border border-border bg-surface px-3 text-sm text-text focus:outline-none focus:ring-2 focus:ring-ring"
              {...register('effective_date')}
            />
          </FormField>
          {/* Destructive retype guard (V4LG8 pattern) */}
          <FormField
            label={t('terminate.typeConfirmLabel', { company: placement.client_company_name })}
            htmlFor="term-confirm"
            required
            error={errors.type_company_name_confirm?.message}
          >
            <input
              id="term-confirm"
              type="text"
              autoComplete="off"
              className="h-9 w-full rounded-lg border border-border bg-surface px-3 text-sm text-text placeholder:text-text-3 focus:outline-none focus:ring-2 focus:ring-ring"
              placeholder={placement.client_company_name}
              {...register('type_company_name_confirm', {
                required: t('validation.required'),
                validate: (v) =>
                  v.trim().toLowerCase() === placement.client_company_name.trim().toLowerCase() ||
                  t('terminate.confirmMismatch'),
              })}
            />
          </FormField>
        </ModalBody>
        <ModalFooter>
          <Button type="button" variant="secondary" onClick={handleClose}>
            {t('common.cancel')}
          </Button>
          <Button
            type="submit"
            variant="destructive"
            disabled={isSubmitting || mutation.isPending || !confirmMatch}
          >
            {t('terminate.confirmBtn')}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// ResignModal  (voluntary resign, LC-6)
// ---------------------------------------------------------------------------

export interface ResignModalProps {
  open: boolean;
  onClose: () => void;
  placement: PlacementInfo;
}

interface ResignFormValues {
  resign_at: string;
  resignation_reason: string;
  notes: string;
}

export function ResignModal({ open, onClose, placement }: ResignModalProps) {
  const { t } = useTranslation('placementDetail');
  const { toast } = useToast();
  const [bannerMsg, setBannerMsg] = useState<string | null>(null);

  const mutation = useResignPlacement();

  const {
    register,
    handleSubmit,
    reset,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<ResignFormValues>({
    defaultValues: { resign_at: '', resignation_reason: '', notes: '' },
  });

  useEffect(() => {
    if (!open) {
      reset();
      setBannerMsg(null);
    }
  }, [open, reset]);

  function handleClose() {
    reset();
    setBannerMsg(null);
    onClose();
  }

  async function onSubmit(values: ResignFormValues) {
    setBannerMsg(null);
    const data: ResignRequest = {
      resign_at: values.resign_at,
      resignation_reason: values.resignation_reason,
      notes: values.notes || null,
    };
    try {
      await mutation.mutateAsync({ id: placement.id, data });
      toast({ tone: 'success', title: t('resign.successTitle') });
      handleClose();
    } catch (err) {
      const classified = classifyError(err);
      if (classified.kind === 'validation') {
        applyFieldErrors(err, setError as Parameters<typeof applyFieldErrors>[1]);
      } else {
        setBannerMsg(classified.message);
      }
    }
  }

  return (
    <Modal
      open={open}
      onOpenChange={(v) => {
        if (!v) handleClose();
      }}
      size="sm"
    >
      <form onSubmit={handleSubmit(onSubmit)} noValidate>
        <ModalHeader icon={UserMinus} tone="warn" title={t('resign.modalTitle')} />
        <ModalBody>
          {bannerMsg != null && <Banner tone="bad" title={bannerMsg} />}
          <FormField
            label={t('resign.resignDate')}
            htmlFor="resign-date"
            required
            error={errors.resign_at?.message}
          >
            <input
              id="resign-date"
              type="date"
              className="h-9 w-full rounded-lg border border-border bg-surface px-3 text-sm text-text focus:outline-none focus:ring-2 focus:ring-ring"
              {...register('resign_at', { required: t('validation.required') })}
            />
          </FormField>
          <FormField
            label={t('resign.reason')}
            htmlFor="resign-reason"
            required
            error={errors.resignation_reason?.message}
          >
            <textarea
              id="resign-reason"
              rows={3}
              className="w-full resize-none rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text placeholder:text-text-3 focus:outline-none focus:ring-2 focus:ring-ring"
              placeholder={t('resign.reasonPlaceholder')}
              {...register('resignation_reason', { required: t('validation.required') })}
            />
          </FormField>
          <FormField label={t('resign.notes')} htmlFor="resign-notes" error={errors.notes?.message}>
            <textarea
              id="resign-notes"
              rows={2}
              className="w-full resize-none rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text placeholder:text-text-3 focus:outline-none focus:ring-2 focus:ring-ring"
              placeholder={t('resign.notesPlaceholder')}
              {...register('notes')}
            />
          </FormField>
        </ModalBody>
        <ModalFooter>
          <Button type="button" variant="secondary" onClick={handleClose}>
            {t('common.cancel')}
          </Button>
          <Button type="submit" variant="primary" disabled={isSubmitting || mutation.isPending}>
            {t('resign.confirmBtn')}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// ShiftLeaderAssignModal  — INV-2 (no leader yet), vars { data } (no id)
// ---------------------------------------------------------------------------

export interface ShiftLeaderAssignModalProps {
  open: boolean;
  onClose: () => void;
  companyId: string;
  companyName: string;
}

interface AssignFormValues {
  employee_id: string;
  start_date: string;
  notes: string;
}

export function ShiftLeaderAssignModal({
  open,
  onClose,
  companyId,
  companyName,
}: ShiftLeaderAssignModalProps) {
  const { t } = useTranslation('placementDetail');
  const { toast } = useToast();
  const [bannerMsg, setBannerMsg] = useState<string | null>(null);
  const [bannerTone, setBannerTone] = useState<'warn' | 'bad' | 'info'>('bad');

  const mutation = useCreateShiftLeaderAssignment();

  const {
    register,
    handleSubmit,
    control,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<AssignFormValues>({
    defaultValues: { employee_id: '', start_date: '', notes: '' },
  });

  useEffect(() => {
    if (!open) {
      reset();
      setBannerMsg(null);
    }
  }, [open, reset]);

  function handleClose() {
    reset();
    setBannerMsg(null);
    onClose();
  }

  async function onSubmit(values: AssignFormValues) {
    setBannerMsg(null);
    const data: ShiftLeaderAssignmentWriteRequest = {
      client_company_id: companyId,
      employee_id: values.employee_id,
      start_date: values.start_date,
      replace: false,
      notes: values.notes || null,
    };
    try {
      // useCreateShiftLeaderAssignment vars = { data } (no id)
      await mutation.mutateAsync({ data });
      toast({ tone: 'success', title: t('sl.assignSuccessTitle') });
      handleClose();
    } catch (err) {
      const classified = classifyError(err);
      if (classified.kind === 'invariant' || classified.kind === 'conflict') {
        setBannerTone('warn');
        setBannerMsg(classified.message);
      } else {
        setBannerTone('bad');
        setBannerMsg(classified.message);
      }
    }
  }

  return (
    <Modal
      open={open}
      onOpenChange={(v) => {
        if (!v) handleClose();
      }}
      size="sm"
    >
      <form onSubmit={handleSubmit(onSubmit)} noValidate>
        <ModalHeader icon={UserPlus} tone="brand" title={t('sl.assignTitle')} />
        <ModalBody>
          {bannerMsg != null && <Banner tone={bannerTone} title={bannerMsg} />}
          <p className="text-sm text-text-2">{companyName}</p>
          <FormField
            label={t('sl.selectLeader')}
            htmlFor="sl-assign-emp"
            required
            error={errors.employee_id?.message}
          >
            <Controller
              control={control}
              name="employee_id"
              rules={{ required: t('validation.required') }}
              render={({ field }) => (
                <CompanyLeaderCandidatePicker
                  companyId={companyId}
                  value={field.value || null}
                  onChange={(v) => field.onChange(v ?? '')}
                  error={!!errors.employee_id}
                />
              )}
            />
          </FormField>
          <FormField
            label={t('sl.startDate')}
            htmlFor="sl-assign-start"
            required
            error={errors.start_date?.message}
          >
            <input
              id="sl-assign-start"
              type="date"
              className="h-9 w-full rounded-lg border border-border bg-surface px-3 text-sm text-text focus:outline-none focus:ring-2 focus:ring-ring"
              {...register('start_date', { required: t('validation.required') })}
            />
          </FormField>
          <FormField label={t('sl.notes')} htmlFor="sl-assign-notes" error={errors.notes?.message}>
            <textarea
              id="sl-assign-notes"
              rows={2}
              className="w-full resize-none rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text placeholder:text-text-3 focus:outline-none focus:ring-2 focus:ring-ring"
              {...register('notes')}
            />
          </FormField>
        </ModalBody>
        <ModalFooter>
          <Button type="button" variant="secondary" onClick={handleClose}>
            {t('common.cancel')}
          </Button>
          <Button type="submit" variant="primary" disabled={isSubmitting || mutation.isPending}>
            {t('sl.assignConfirmBtn')}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// ShiftLeaderReplaceModal  — vars { id, data }
// ---------------------------------------------------------------------------

export interface ShiftLeaderReplaceModalProps {
  open: boolean;
  onClose: () => void;
  assignmentId: string;
  companyId: string;
  companyName: string;
  currentLeaderName: string;
  /** Current leader's employee id — omitted from the candidate list. */
  currentLeaderEmployeeId: string;
}

interface ReplaceFormValues {
  new_employee_id: string;
  start_date: string;
  replace_reason: string;
}

export function ShiftLeaderReplaceModal({
  open,
  onClose,
  assignmentId,
  companyId,
  companyName,
  currentLeaderName,
  currentLeaderEmployeeId,
}: ShiftLeaderReplaceModalProps) {
  const { t } = useTranslation('placementDetail');
  const { toast } = useToast();
  const [bannerMsg, setBannerMsg] = useState<string | null>(null);
  const [bannerTone, setBannerTone] = useState<'warn' | 'bad' | 'info'>('bad');

  const mutation = useReplaceShiftLeaderAssignment();

  const {
    register,
    handleSubmit,
    control,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<ReplaceFormValues>({
    defaultValues: { new_employee_id: '', start_date: '', replace_reason: '' },
  });

  useEffect(() => {
    if (!open) {
      reset();
      setBannerMsg(null);
    }
  }, [open, reset]);

  function handleClose() {
    reset();
    setBannerMsg(null);
    onClose();
  }

  async function onSubmit(values: ReplaceFormValues) {
    setBannerMsg(null);
    const data: ShiftLeaderAssignmentReplaceRequest = {
      new_employee_id: values.new_employee_id,
      start_date: values.start_date,
      replace_reason: values.replace_reason,
    };
    try {
      await mutation.mutateAsync({ id: assignmentId, data });
      toast({ tone: 'success', title: t('sl.replaceSuccessTitle') });
      handleClose();
    } catch (err) {
      const classified = classifyError(err);
      if (classified.kind === 'invariant' || classified.kind === 'conflict') {
        setBannerTone('warn');
        setBannerMsg(classified.message);
      } else {
        setBannerTone('bad');
        setBannerMsg(classified.message);
      }
    }
  }

  return (
    <Modal
      open={open}
      onOpenChange={(v) => {
        if (!v) handleClose();
      }}
      size="sm"
    >
      <form onSubmit={handleSubmit(onSubmit)} noValidate>
        <ModalHeader icon={UserPlus} tone="warn" title={t('sl.replaceTitle')} />
        <ModalBody>
          {bannerMsg != null && <Banner tone={bannerTone} title={bannerMsg} />}
          <p className="text-sm text-text-2">
            {companyName} · {t('sl.currentLeader')}: {currentLeaderName}
          </p>
          <FormField
            label={t('sl.newLeader')}
            htmlFor="sl-rep-emp"
            required
            error={errors.new_employee_id?.message}
          >
            <Controller
              control={control}
              name="new_employee_id"
              rules={{ required: t('validation.required') }}
              render={({ field }) => (
                <CompanyLeaderCandidatePicker
                  companyId={companyId}
                  excludeEmployeeId={currentLeaderEmployeeId}
                  value={field.value || null}
                  onChange={(v) => field.onChange(v ?? '')}
                  error={!!errors.new_employee_id}
                />
              )}
            />
          </FormField>
          <FormField
            label={t('sl.startDate')}
            htmlFor="sl-rep-start"
            required
            error={errors.start_date?.message}
          >
            <input
              id="sl-rep-start"
              type="date"
              className="h-9 w-full rounded-lg border border-border bg-surface px-3 text-sm text-text focus:outline-none focus:ring-2 focus:ring-ring"
              {...register('start_date', { required: t('validation.required') })}
            />
          </FormField>
          <FormField
            label={t('sl.replaceReason')}
            htmlFor="sl-rep-reason"
            required
            error={errors.replace_reason?.message}
          >
            <textarea
              id="sl-rep-reason"
              rows={3}
              className="w-full resize-none rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text placeholder:text-text-3 focus:outline-none focus:ring-2 focus:ring-ring"
              placeholder={t('sl.replaceReasonPlaceholder')}
              {...register('replace_reason', { required: t('validation.required') })}
            />
          </FormField>
        </ModalBody>
        <ModalFooter>
          <Button type="button" variant="secondary" onClick={handleClose}>
            {t('common.cancel')}
          </Button>
          <Button type="submit" variant="primary" disabled={isSubmitting || mutation.isPending}>
            {t('sl.replaceConfirmBtn')}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// ShiftLeaderEndConfirm  — manual vacate (vacated_reason = MANUAL), vars { id, data }
// ---------------------------------------------------------------------------

export interface ShiftLeaderEndConfirmProps {
  open: boolean;
  onClose: () => void;
  assignmentId: string;
  companyName: string;
  leaderName: string;
}

export function ShiftLeaderEndConfirm({
  open,
  onClose,
  assignmentId,
  companyName,
  leaderName,
}: ShiftLeaderEndConfirmProps) {
  const { t } = useTranslation('placementDetail');
  const { toast } = useToast();

  const mutation = useEndShiftLeaderAssignment();

  async function handleConfirm() {
    const data: ShiftLeaderAssignmentEndRequest = { reason: null, effective_at: null };
    try {
      await mutation.mutateAsync({ id: assignmentId, data });
      toast({ tone: 'success', title: t('sl.endSuccessTitle') });
      onClose();
    } catch (err) {
      toast({ tone: 'error' as const, title: classifyError(err).message });
    }
  }

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
      icon={UserMinus}
      tone="warn"
      title={t('sl.endTitle')}
      description={t('sl.endDescription', { leader: leaderName, company: companyName })}
      confirmTone="primary"
      confirmLabel={t('sl.endConfirmBtn')}
      cancelLabel={t('common.cancel')}
      onConfirm={() => {
        void handleConfirm();
      }}
      loading={mutation.isPending}
    />
  );
}
