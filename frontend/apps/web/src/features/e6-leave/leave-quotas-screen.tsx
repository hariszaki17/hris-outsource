/**
 * E6 · Kuota & Hibah Cuti (HR)
 *
 * .pen frames:
 *   P6HZ7E  Kuota & Hibah Cuti (HR) — main list
 *   CGCnL   Sesuaikan Kuota (modal) — single-employee adjust with reason (LQ-6)
 *   W2zYM   Terbitkan Kuota Tahunan (modal) — bulk grant with preview step (LQ-1)
 *
 * F6.1: per-employee leave quota list (employee, leave type, entitled/used/remaining)
 * + Sesuaikan Kuota adjust modal + Terbitkan Kuota Tahunan bulk-grant modal (preview→apply).
 *
 * ENGINEERING.md D1 — typed URL search params + cursor pagination.
 * ENGINEERING.md B1 — classifyError / applyFieldErrors.
 * LQ-6: manual adjustment requires reason, recorded in audit log.
 */

import { applyFieldErrors, classifyError } from '@/lib/api-error.ts';
import { zodResolver } from '@hookform/resolvers/zod';
import {
  type LeaveQuota,
  type LeaveQuotaAdjustRequest,
  type LeaveQuotaBulkGrantRequest,
  type LeaveQuotaBulkGrantResponse,
  type ListLeaveQuotasParams,
  useAdjustLeaveQuota,
  useBulkGrantLeaveQuotas,
  useListLeaveQuotas,
} from '@swp/api-client/e6';
import {
  Avatar,
  Button,
  type Column,
  CursorPagination,
  DataTable,
  EmptyState,
  FilterSelect,
  FormField,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  SearchField,
  StateView,
  useToast,
} from '@swp/ui';
import { useNavigate, useSearch } from '@tanstack/react-router';
import {
  CalendarPlus,
  CheckCircle2,
  Download,
  Info,
  Settings2,
  TriangleAlert,
  Users,
} from 'lucide-react';
import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';

// ---------------------------------------------------------------------------
// Search params type (D1 — typed URL search params)
// ---------------------------------------------------------------------------

export type LeaveQuotasSearch = {
  q?: string;
  period?: number;
  leave_type_id?: string;
  cursor?: string;
};

// ---------------------------------------------------------------------------
// Zod schemas (hand-written — not from codegen)
// ---------------------------------------------------------------------------

const adjustSchema = z.object({
  delta: z
    .number({ invalid_type_error: 'leaveQuotas.errors.deltaRequired' })
    .int('leaveQuotas.errors.deltaInt')
    .refine((v) => v !== 0, 'leaveQuotas.errors.deltaNonZero'),
  reason: z
    .string()
    .min(5, 'leaveQuotas.errors.reasonMin')
    .max(500, 'leaveQuotas.errors.reasonMax'),
});
type AdjustFormValues = z.infer<typeof adjustSchema>;

const bulkGrantSchema = z.object({
  leave_type_id: z.string().min(1, 'leaveQuotas.errors.leaveTypeRequired'),
  period: z.number().int().min(2000).max(2100),
  default_entitlement_days: z.number().int().min(1).max(365).optional(),
  pro_rate: z.boolean(),
});
type BulkGrantFormValues = z.infer<typeof bulkGrantSchema>;

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const PAGE_SIZE = 50;
const CURRENT_YEAR = new Date().getFullYear();

// ---------------------------------------------------------------------------
// Sesuaikan Kuota Modal (CGCnL)
// ---------------------------------------------------------------------------

interface AdjustModalProps {
  quota: LeaveQuota;
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

function AdjustQuotaModal({ quota, open, onClose, onSuccess }: AdjustModalProps) {
  const { t } = useTranslation('leaveQuotas');
  const { toast } = useToast();
  const adjustMutation = useAdjustLeaveQuota();

  const {
    register,
    handleSubmit,
    reset,
    setError,
    watch,
    formState: { errors, isSubmitting },
  } = useForm<AdjustFormValues>({
    resolver: zodResolver(adjustSchema),
    defaultValues: { delta: 0, reason: '' },
  });

  const deltaValue = watch('delta') || 0;
  const newTotal = quota.total + Number(deltaValue);
  const newRemaining = quota.remaining + Number(deltaValue);

  useEffect(() => {
    if (open) reset({ delta: 0, reason: '' });
  }, [open, reset]);

  async function onSubmit(values: AdjustFormValues) {
    try {
      const body: LeaveQuotaAdjustRequest = { delta: values.delta, reason: values.reason };
      await adjustMutation.mutateAsync({ id: quota.id, data: body });
      toast({
        tone: 'success',
        title: t('adjust.successTitle'),
        description: t('adjust.successDesc'),
      });
      onSuccess();
      onClose();
    } catch (err) {
      if (!applyFieldErrors(err, setError)) {
        const { message } = classifyError(err);
        toast({ tone: 'error', title: t('adjust.errorTitle'), description: message });
      }
    }
  }

  return (
    <Modal
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
      size="md"
    >
      <form onSubmit={handleSubmit(onSubmit)}>
        <ModalHeader icon={Settings2} tone="neutral" title={t('adjust.title')} onClose={onClose} />
        <ModalBody>
          {/* Employee + quota context */}
          <p className="text-[12px] text-text-3 mb-[14px]">
            {quota.employee_name ?? quota.employee_id} &middot;{' '}
            {quota.leave_type_name ?? quota.leave_type_id} {quota.period}
          </p>

          {/* Current stats */}
          <div className="grid grid-cols-3 gap-[10px] mb-[14px]">
            {(
              [
                { label: t('adjust.totalCurrent'), value: quota.total },
                { label: t('adjust.usedCurrent'), value: quota.used },
                { label: t('adjust.remainingCurrent'), value: quota.remaining },
              ] as const
            ).map(({ label, value }) => (
              <div
                key={label}
                className="flex flex-col gap-[2px] items-center rounded-[8px] bg-surface-2 py-[10px] px-[8px]"
              >
                <span className="text-[18px] font-bold text-text leading-none">{value}</span>
                <span className="text-[11px] text-text-3">{label}</span>
              </div>
            ))}
          </div>

          {/* New totals preview */}
          <div className="grid grid-cols-2 gap-[12px] mb-[14px]">
            <div className="flex flex-col gap-[6px]">
              <span className="text-[12px] font-semibold text-text-2">{t('adjust.totalNew')}</span>
              <div className="rounded-[8px] border border-border bg-surface py-[10px] px-[12px] text-[14px] text-text">
                {Number.isNaN(newTotal) ? '—' : newTotal}
              </div>
            </div>
            <div className="flex flex-col gap-[6px]">
              <span className="text-[12px] font-semibold text-text-2">
                {t('adjust.remainingNew')}
              </span>
              <div className="rounded-[8px] border border-border bg-surface py-[10px] px-[12px] text-[14px] text-text">
                {Number.isNaN(newRemaining) ? '—' : newRemaining}
              </div>
            </div>
          </div>

          {/* Delta input */}
          <FormField
            label={t('adjust.deltaLabel')}
            htmlFor="delta"
            required
            error={errors.delta?.message}
          >
            <input
              id="delta"
              type="number"
              className="w-full rounded-[8px] border border-border bg-surface py-[10px] px-[12px] text-[14px] text-text focus:outline-none focus:ring-2 focus:ring-primary"
              placeholder={t('adjust.deltaPlaceholder')}
              {...register('delta', { valueAsNumber: true })}
            />
          </FormField>

          {/* Reason */}
          <FormField
            label={t('adjust.reasonLabel')}
            htmlFor="reason"
            required
            error={errors.reason?.message}
          >
            <textarea
              id="reason"
              rows={3}
              className="w-full rounded-[8px] border border-border bg-surface py-[10px] px-[12px] text-[14px] text-text focus:outline-none focus:ring-2 focus:ring-primary resize-none"
              placeholder={t('adjust.reasonPlaceholder')}
              {...register('reason')}
            />
          </FormField>

          {/* Audit note */}
          <div className="flex items-start gap-[8px] rounded-[8px] border border-info-bd bg-info-bg py-[10px] px-[12px] mt-[14px]">
            <Info aria-hidden className="h-[14px] w-[14px] shrink-0 text-info-tx mt-[1px]" />
            <span className="text-[12px] text-info-tx leading-[1.4]">{t('adjust.auditNote')}</span>
          </div>
        </ModalBody>
        <ModalFooter>
          <Button type="button" variant="secondary" onClick={onClose}>
            {t('actions.cancel')}
          </Button>
          <Button type="submit" variant="primary" disabled={isSubmitting}>
            {t('adjust.saveBtn')}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// Terbitkan Kuota Tahunan Modal (W2zYM) — form → preview → apply
// ---------------------------------------------------------------------------

type BulkGrantStep = 'form' | 'preview';

interface BulkGrantModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

function BulkGrantModal({ open, onClose, onSuccess }: BulkGrantModalProps) {
  const { t } = useTranslation('leaveQuotas');
  const { toast } = useToast();
  const bulkMutation = useBulkGrantLeaveQuotas();

  const [step, setStep] = useState<BulkGrantStep>('form');
  const [previewData, setPreviewData] = useState<LeaveQuotaBulkGrantResponse | null>(null);

  const {
    register,
    handleSubmit,
    reset,
    watch,
    formState: { errors, isSubmitting },
  } = useForm<BulkGrantFormValues>({
    resolver: zodResolver(bulkGrantSchema),
    defaultValues: {
      leave_type_id: 'annual',
      period: CURRENT_YEAR,
      default_entitlement_days: 12,
      pro_rate: true,
    },
  });

  const currentPeriod = watch('period');

  useEffect(() => {
    if (open) {
      reset({
        leave_type_id: 'annual',
        period: CURRENT_YEAR,
        default_entitlement_days: 12,
        pro_rate: true,
      });
      setStep('form');
      setPreviewData(null);
    }
  }, [open, reset]);

  async function onPreview(values: BulkGrantFormValues) {
    try {
      const body: LeaveQuotaBulkGrantRequest = {
        leave_type_id: values.leave_type_id,
        period: values.period,
        default_entitlement_days: values.default_entitlement_days,
        employee_ids: ['all'] as unknown as LeaveQuotaBulkGrantRequest['employee_ids'],
        pro_rate: values.pro_rate,
        preview: true,
      };
      const res = await bulkMutation.mutateAsync({ data: body });
      const responseData = (res as { data: LeaveQuotaBulkGrantResponse }).data;
      setPreviewData(responseData);
      setStep('preview');
    } catch (err) {
      const { message } = classifyError(err);
      toast({ tone: 'error', title: t('bulkGrant.errorTitle'), description: message });
    }
  }

  async function onApply(values: BulkGrantFormValues) {
    try {
      const body: LeaveQuotaBulkGrantRequest = {
        leave_type_id: values.leave_type_id,
        period: values.period,
        default_entitlement_days: values.default_entitlement_days,
        employee_ids: ['all'] as unknown as LeaveQuotaBulkGrantRequest['employee_ids'],
        pro_rate: values.pro_rate,
        preview: false,
      };
      await bulkMutation.mutateAsync({ data: body });
      toast({
        tone: 'success',
        title: t('bulkGrant.successTitle'),
        description: t('bulkGrant.successDesc'),
      });
      onSuccess();
      onClose();
    } catch (err) {
      const { message } = classifyError(err);
      toast({ tone: 'error', title: t('bulkGrant.errorTitle'), description: message });
    }
  }

  return (
    <Modal
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
      size="md"
    >
      <ModalHeader
        icon={CalendarPlus}
        tone="brand"
        title={t('bulkGrant.title')}
        onClose={onClose}
      />

      {step === 'form' && (
        <form onSubmit={handleSubmit(onPreview)}>
          <ModalBody>
            {/* Periode */}
            <FormField
              label={t('bulkGrant.periodLabel')}
              htmlFor="bg-period"
              error={errors.period?.message}
            >
              <div className="flex items-center justify-between rounded-[8px] border border-border bg-surface py-[10px] px-[12px]">
                <span className="text-[14px] text-text">
                  {t('bulkGrant.periodValue', { year: currentPeriod })}
                </span>
              </div>
            </FormField>

            <p className="text-[11px] font-bold tracking-[0.4px] text-text-3 mt-[14px] mb-[10px] uppercase">
              {t('bulkGrant.defaultsHeading')}
            </p>

            {/* Cuti Tahunan row */}
            <div className="flex items-center justify-between rounded-[8px] bg-surface-2 py-[10px] px-[12px] mb-[8px]">
              <span className="text-[14px] text-text">{t('bulkGrant.annualLeaveLabel')}</span>
              <div className="flex items-center gap-[8px] rounded-[6px] border border-border bg-surface py-[5px] px-[10px]">
                <input
                  type="number"
                  aria-label={t('bulkGrant.daysAriaLabel')}
                  className="w-[40px] text-[13px] font-semibold text-text bg-transparent focus:outline-none"
                  {...register('default_entitlement_days', { valueAsNumber: true })}
                />
                <span className="text-[13px] text-text-3">{t('bulkGrant.days')}</span>
              </div>
            </div>

            {/* Cuti Sakit placeholder row */}
            <div className="flex items-center justify-between rounded-[8px] bg-surface-2 py-[10px] px-[12px] mb-[14px]">
              <span className="text-[14px] text-text">{t('bulkGrant.sickLeaveLabel')}</span>
              <div className="flex items-center gap-[8px] rounded-[6px] border border-border bg-surface py-[5px] px-[10px]">
                <span className="text-[13px] font-semibold text-text">12</span>
                <span className="text-[13px] text-text-3">{t('bulkGrant.days')}</span>
              </div>
            </div>

            {/* Pro-rata toggle */}
            <div className="flex items-center justify-between py-[4px] mb-[14px]">
              <div className="flex flex-col gap-[1px]">
                <span className="text-[14px] font-semibold text-text">
                  {t('bulkGrant.proRateLabel')}
                </span>
                <span className="text-[12px] text-text-3">{t('bulkGrant.proRateHint')}</span>
              </div>
              <label className="relative inline-flex items-center cursor-pointer">
                <input
                  type="checkbox"
                  className="sr-only peer"
                  {...register('pro_rate')}
                  defaultChecked
                />
                <div className="w-[44px] h-[24px] bg-border peer-focus:ring-2 peer-focus:ring-primary rounded-full peer peer-checked:bg-primary transition-colors" />
                <div className="absolute left-[2px] top-[2px] bg-white w-[20px] h-[20px] rounded-full shadow transition-transform peer-checked:translate-x-[20px]" />
              </label>
            </div>

            {/* Preview hint */}
            <div className="flex items-center gap-[8px] rounded-[8px] border border-ok-bd bg-ok-bg py-[11px] px-[12px] mb-[14px]">
              <Users aria-hidden className="h-[15px] w-[15px] shrink-0 text-ok-tx" />
              <span className="text-[13px] font-semibold text-ok-tx">
                {t('bulkGrant.previewHint')}
              </span>
            </div>

            {/* Warn note */}
            <div className="flex items-start gap-[8px] rounded-[8px] border border-warn-bd bg-warn-bg py-[10px] px-[12px]">
              <TriangleAlert
                aria-hidden
                className="h-[14px] w-[14px] shrink-0 text-warn-tx mt-[1px]"
              />
              <span className="text-[12px] text-warn-tx leading-[1.4]">
                {t('bulkGrant.warnNote')}
              </span>
            </div>
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="secondary" onClick={onClose}>
              {t('actions.cancel')}
            </Button>
            <Button type="submit" variant="primary" disabled={isSubmitting}>
              {t('bulkGrant.previewBtn')}
            </Button>
          </ModalFooter>
        </form>
      )}

      {step === 'preview' && previewData && (
        <>
          <ModalBody>
            <div className="flex items-center gap-[8px] rounded-[8px] border border-ok-bd bg-ok-bg py-[11px] px-[12px] mb-[14px]">
              <CheckCircle2 aria-hidden className="h-[16px] w-[16px] shrink-0 text-ok-tx" />
              <span className="text-[13px] font-semibold text-ok-tx">
                {t('bulkGrant.previewResult', {
                  count: previewData.total_affected,
                  succeeded: previewData.succeeded.length,
                })}
              </span>
            </div>

            {previewData.failed.length > 0 && (
              <div className="flex items-start gap-[8px] rounded-[8px] border border-warn-bd bg-warn-bg py-[10px] px-[12px] mb-[14px]">
                <TriangleAlert
                  aria-hidden
                  className="h-[14px] w-[14px] shrink-0 text-warn-tx mt-[1px]"
                />
                <span className="text-[12px] text-warn-tx leading-[1.4]">
                  {t('bulkGrant.previewFailed', { count: previewData.failed.length })}
                </span>
              </div>
            )}

            <div className="flex items-start gap-[8px] rounded-[8px] border border-warn-bd bg-warn-bg py-[10px] px-[12px]">
              <TriangleAlert
                aria-hidden
                className="h-[14px] w-[14px] shrink-0 text-warn-tx mt-[1px]"
              />
              <span className="text-[12px] text-warn-tx leading-[1.4]">
                {t('bulkGrant.warnNote')}
              </span>
            </div>
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="secondary" onClick={() => setStep('form')}>
              {t('actions.back')}
            </Button>
            <Button
              type="button"
              variant="primary"
              disabled={bulkMutation.isPending}
              onClick={handleSubmit(onApply)}
            >
              {t('bulkGrant.applyBtn')}
            </Button>
          </ModalFooter>
        </>
      )}
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// LeaveQuotasScreen
// ---------------------------------------------------------------------------

export function LeaveQuotasScreen() {
  const { t } = useTranslation('leaveQuotas');
  const navigate = useNavigate();
  const search = useSearch({ strict: false }) as LeaveQuotasSearch;

  const [adjustTarget, setAdjustTarget] = useState<LeaveQuota | null>(null);
  const [bulkGrantOpen, setBulkGrantOpen] = useState(false);

  const queryParams: ListLeaveQuotasParams = {
    limit: PAGE_SIZE,
    ...(search.q ? { employee_id: search.q } : {}),
    ...(search.period ? { period: search.period } : {}),
    ...(search.leave_type_id ? { leave_type_id: search.leave_type_id } : {}),
    ...(search.cursor ? { cursor: search.cursor } : {}),
  };

  const quotasQuery = useListLeaveQuotas(queryParams);

  type QuotaListData = { data: LeaveQuota[]; next_cursor?: string | null; has_more: boolean };
  const quotaList = (quotasQuery.data as { data: QuotaListData } | undefined)?.data;
  const rows = quotaList?.data ?? [];
  const hasMore = quotaList?.has_more ?? false;
  const nextCursor = quotaList?.next_cursor ?? null;

  type NavFn = (o: { to: string; search?: Record<string, unknown> }) => void;
  const nav = navigate as unknown as NavFn;

  function setSearch(patch: Partial<LeaveQuotasSearch>) {
    nav({ to: '/leave/quotas', search: { ...search, ...patch, cursor: undefined } });
  }

  function onRefetch() {
    quotasQuery.refetch();
  }

  const error = quotasQuery.error ? classifyError(quotasQuery.error) : null;

  // ---------------------------------------------------------------------------
  // Columns
  // ---------------------------------------------------------------------------

  const columns: Column<LeaveQuota>[] = [
    {
      id: 'employee',
      header: t('table.employee'),
      width: 300,
      cell: (row) => (
        <div className="flex items-center gap-[10px]">
          <Avatar
            initials={(row.employee_name ?? row.employee_id).slice(0, 2).toUpperCase()}
            size={32}
          />
          <div className="flex flex-col min-w-0">
            <span className="text-[14px] font-medium text-text truncate">
              {row.employee_name ?? row.employee_id}
            </span>
            <span className="text-[12px] text-text-3 font-mono">{row.employee_id}</span>
          </div>
        </div>
      ),
    },
    {
      id: 'leave_type',
      header: t('table.leaveType'),
      width: 170,
      cell: (row) => (
        <div className="flex items-center gap-[6px]">
          <span className="text-[14px] text-text">{row.leave_type_name ?? row.leave_type_id}</span>
          {row.is_prorated && (
            <span className="text-[11px] font-semibold text-info-tx bg-info-bg rounded-full px-[6px] py-[1px]">
              {t('badge.prorata')}
            </span>
          )}
        </div>
      ),
    },
    {
      id: 'total',
      header: t('table.total'),
      width: 130,
      align: 'right',
      cell: (row) => <span className="text-[14px] font-semibold text-text">{row.total}</span>,
    },
    {
      id: 'used',
      header: t('table.used'),
      width: 100,
      align: 'right',
      cell: (row) => <span className="text-[14px] text-text">{row.used}</span>,
    },
    {
      id: 'pending',
      header: t('table.pending'),
      width: 110,
      align: 'right',
      cell: (row) => (
        <span
          className={`text-[14px] ${row.pending > 0 ? 'text-warn-tx font-medium' : 'text-text-3'}`}
        >
          {row.pending}
        </span>
      ),
    },
    {
      id: 'remaining',
      header: t('table.remaining'),
      width: 140,
      align: 'right',
      cell: (row) => (
        <span
          className={`text-[14px] font-semibold ${
            row.remaining < 0 ? 'text-bad-tx' : row.remaining === 0 ? 'text-text-3' : 'text-ok-tx'
          }`}
        >
          {row.remaining}
        </span>
      ),
    },
    {
      id: 'actions',
      header: '',
      width: 166,
      align: 'right',
      cell: (row) => (
        <Button
          type="button"
          variant="secondary"
          size="sm"
          aria-label={t('actions.adjustAriaLabel', { name: row.employee_name ?? row.employee_id })}
          onClick={() => setAdjustTarget(row)}
        >
          {t('actions.adjust')}
        </Button>
      ),
    },
  ];

  // ---------------------------------------------------------------------------
  // Render state booleans
  // ---------------------------------------------------------------------------

  const isLoading = quotasQuery.isLoading;
  const isEmpty = !isLoading && !error && rows.length === 0;
  const hasActiveFilter = !!(search.q || search.leave_type_id);
  const isForbidden = error?.kind === 'forbidden';

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <div className="flex flex-col gap-[18px] p-[24px] bg-app-bg min-h-full w-full">
      {/* Title band */}
      <div className="flex items-center justify-between w-full">
        <div className="flex flex-col gap-[4px]">
          <h1 className="text-[30px] font-bold text-text leading-none">{t('title')}</h1>
          <p className="text-[14px] text-text-3">{t('subtitle')}</p>
        </div>
        <div className="flex items-center gap-[10px]">
          <Button type="button" variant="secondary">
            <Download aria-hidden className="h-[16px] w-[16px]" />
            {t('actions.export')}
          </Button>
          <Button type="button" variant="primary" onClick={() => setBulkGrantOpen(true)}>
            <CalendarPlus aria-hidden className="h-[16px] w-[16px]" />
            {t('actions.bulkGrant')}
          </Button>
        </div>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-[10px] w-full">
        <SearchField
          value={search.q ?? ''}
          onChange={(e) => setSearch({ q: e.target.value || undefined })}
          placeholder={t('filters.searchPlaceholder')}
        />
        <FilterSelect
          value={String(search.period ?? CURRENT_YEAR)}
          onChange={(e) =>
            setSearch({ period: e.target.value ? Number(e.target.value) : undefined })
          }
        >
          <option value={String(CURRENT_YEAR)}>
            {t('filters.period', { year: CURRENT_YEAR })}
          </option>
          <option value={String(CURRENT_YEAR - 1)}>
            {t('filters.period', { year: CURRENT_YEAR - 1 })}
          </option>
        </FilterSelect>
        <FilterSelect
          value={search.leave_type_id ?? ''}
          onChange={(e) => setSearch({ leave_type_id: e.target.value || undefined })}
        >
          <option value="">{t('filters.allLeaveTypes')}</option>
          <option value="annual">{t('filters.annualLeave')}</option>
          <option value="sick">{t('filters.sickLeave')}</option>
        </FilterSelect>
      </div>

      {/* Table card */}
      {isForbidden ? (
        <EmptyState
          variant="no-permission"
          title={t('states.noPermissionTitle')}
          description={t('states.noPermissionDesc')}
        />
      ) : error ? (
        <StateView
          kind="error"
          title={t('states.errorTitle')}
          description={error.message}
          onRetry={onRefetch}
          retryLabel={t('actions.retry')}
        />
      ) : (
        <div className="rounded-[12px] border border-border bg-surface overflow-hidden w-full">
          <DataTable
            columns={columns}
            data={rows}
            getRowId={(row) => row.id}
            isLoading={isLoading}
            empty={
              isEmpty && hasActiveFilter ? (
                <EmptyState
                  variant="filtered"
                  title={t('states.filteredZeroTitle')}
                  description={t('states.filteredZeroDesc')}
                  action={
                    <Button
                      type="button"
                      variant="secondary"
                      size="sm"
                      onClick={() => nav({ to: '/leave/quotas', search: {} })}
                    >
                      {t('actions.clearFilters')}
                    </Button>
                  }
                />
              ) : (
                <EmptyState
                  variant="fresh"
                  title={t('states.freshTitle')}
                  description={t('states.freshDesc')}
                />
              )
            }
          />
        </div>
      )}

      {/* Footer note (LQ-6) */}
      {!isLoading && !error && rows.length > 0 && (
        <p className="text-[12px] text-text-3">{t('footerNote')}</p>
      )}

      {/* Cursor pagination */}
      {rows.length > 0 && (
        <CursorPagination
          rangeLabel={t('pagination.rangeLabel', { count: rows.length })}
          hasPrev={!!search.cursor}
          hasNext={hasMore}
          onPrev={() => nav({ to: '/leave/quotas', search: { ...search, cursor: undefined } })}
          onNext={() =>
            nav({ to: '/leave/quotas', search: { ...search, cursor: nextCursor ?? undefined } })
          }
        />
      )}

      {/* Sesuaikan Kuota modal (CGCnL) */}
      {adjustTarget && (
        <AdjustQuotaModal
          quota={adjustTarget}
          open
          onClose={() => setAdjustTarget(null)}
          onSuccess={onRefetch}
        />
      )}

      {/* Terbitkan Kuota Tahunan modal (W2zYM) */}
      <BulkGrantModal
        open={bulkGrantOpen}
        onClose={() => setBulkGrantOpen(false)}
        onSuccess={onRefetch}
      />
    </div>
  );
}
