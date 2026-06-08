/**
 * E6 · Saldo & Hibah Cuti (HR) — reframed from per-type quota to grant-lot ledger
 *
 * .pen frames:
 *   P6HZ7E  Saldo & Hibah Cuti (HR) — main balance ledger (2026-06-08)
 *   CGCnL   Tambah Hibah / Sesuaikan Saldo (modal) — grant-lot form + adjust (LQ-6)
 *
 * F6.1 — grant-lot ledger (resolved 2026-06-08): balance = per-employee POOL of LeaveGrant lots,
 * each with its own expires_at. earmark=null → general pool (FIFO); non-null → purpose-restricted.
 *
 * ENGINEERING.md D1 — typed URL search params + cursor pagination.
 * ENGINEERING.md B1 — classifyError / applyFieldErrors.
 * LQ-6: manual grant/adjust requires remark, recorded in audit log.
 */

import { applyFieldErrors, classifyError } from '@/lib/api-error.ts';
import { zodResolver } from '@hookform/resolvers/zod';
import {
  type LeaveGrant,
  type LeaveGrantPatchRequest,
  LeaveGrantSource,
  type LeaveGrantWriteRequest,
  type ListLeaveGrantsParams,
  getGetLeaveBalanceByEmployeeQueryKey,
  getListLeaveGrantsQueryKey,
  useAdjustLeaveGrant,
  useCreateLeaveGrant,
  useGetLeaveBalanceByEmployee,
  useListLeaveGrants,
} from '@swp/api-client/e6';
import {
  Button,
  type Column,
  CursorPagination,
  DataTable,
  DateText,
  EmptyState,
  FilterSelect,
  FormField,
  IdChip,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  SearchField,
  StateView,
  StatusBadge,
  useToast,
} from '@swp/ui';
import { useQueryClient } from '@tanstack/react-query';
import { useNavigate, useSearch } from '@tanstack/react-router';
import { CalendarPlus, Info, PackagePlus, Settings2, Tag } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';

// ---------------------------------------------------------------------------
// Search params type (D1 — typed URL search params)
// ---------------------------------------------------------------------------

export type LeaveQuotasSearch = {
  q?: string;
  employee_id?: string;
  cursor?: string;
};

// ---------------------------------------------------------------------------
// Zod schemas (hand-written — Zod deferred for codegen WEB-STACK §4)
// ---------------------------------------------------------------------------

const grantSchema = z.object({
  employee_id: z.string().min(1, 'Pilih karyawan'),
  amount_days: z
    .number({ invalid_type_error: 'Wajib diisi' })
    .int('Harus bilangan bulat')
    .min(0, 'Minimal 0'),
  expires_at: z.string().min(1, 'Tanggal kedaluwarsa wajib diisi'),
  source: z.nativeEnum(LeaveGrantSource),
  earmark: z.string().optional(),
  remark: z.string().min(5, 'Catatan minimal 5 karakter').max(500, 'Catatan maksimal 500 karakter'),
  effective_from: z.string().optional(),
});
type GrantFormValues = z.infer<typeof grantSchema>;

const adjustSchema = z.object({
  amount_days: z
    .number({ invalid_type_error: 'Wajib diisi' })
    .int('Harus bilangan bulat')
    .min(0, 'Minimal 0')
    .optional(),
  expires_at: z.string().optional(),
  earmark: z.string().optional(),
  remark: z.string().min(5, 'Catatan minimal 5 karakter').max(500, 'Catatan maksimal 500 karakter'),
});
type AdjustFormValues = z.infer<typeof adjustSchema>;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const EARMARK_SOURCES: LeaveGrantSource[] = [
  LeaveGrantSource.MATERNITY,
  LeaveGrantSource.STATUTORY,
];

function sourceLabelKey(source: LeaveGrantSource): string {
  return `leaveQuotas.source.${source}`;
}

function earmarkBadgeTone(earmark: string | null | undefined): 'ok' | 'warn' | 'neutral' {
  if (!earmark) return 'neutral';
  if (earmark === 'MATERNITY') return 'warn';
  return 'ok';
}

const PAGE_SIZE = 50;

// ---------------------------------------------------------------------------
// Grant-lot row: remaining days computation
// ---------------------------------------------------------------------------

function remainingDays(lot: LeaveGrant): number {
  return lot.amount_days - lot.consumed_days - lot.pending_days;
}

// ---------------------------------------------------------------------------
// GrantLotModal (CGCnL) — create (POST /leave-grants) or adjust (PATCH)
// ---------------------------------------------------------------------------

interface GrantLotModalProps {
  /** When set, we are adjusting an existing lot (PATCH). When undefined, creating (POST). */
  adjustTarget?: LeaveGrant;
  /** Pre-filled employee_id when known (from context). */
  defaultEmployeeId?: string;
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

function GrantLotModal({
  adjustTarget,
  defaultEmployeeId,
  open,
  onClose,
  onSuccess,
}: GrantLotModalProps) {
  const { t } = useTranslation('leaveQuotas');
  const { toast } = useToast();
  const isAdjust = Boolean(adjustTarget);

  const createMutation = useCreateLeaveGrant();
  const adjustMutation = useAdjustLeaveGrant();

  // ── Grant form (create) ──────────────────────────────────────────────────
  const {
    register: registerGrant,
    handleSubmit: handleGrantSubmit,
    reset: resetGrant,
    setError: setGrantError,
    watch: watchGrant,
    formState: { errors: grantErrors, isSubmitting: grantSubmitting },
  } = useForm<GrantFormValues>({
    resolver: zodResolver(grantSchema),
    defaultValues: {
      employee_id: defaultEmployeeId ?? '',
      amount_days: 0,
      expires_at: '',
      source: LeaveGrantSource.ADJUSTMENT,
      earmark: '',
      remark: '',
      effective_from: '',
    },
  });

  // ── Adjust form (patch) ──────────────────────────────────────────────────
  const {
    register: registerAdjust,
    handleSubmit: handleAdjustSubmit,
    reset: resetAdjust,
    setError: setAdjustError,
    formState: { errors: adjustErrors, isSubmitting: adjustSubmitting },
  } = useForm<AdjustFormValues>({
    resolver: zodResolver(adjustSchema),
    defaultValues: {
      amount_days: adjustTarget?.amount_days,
      expires_at: adjustTarget?.expires_at ?? '',
      earmark: adjustTarget?.earmark ?? '',
      remark: '',
    },
  });

  const grantSource = watchGrant('source');
  const showEarmark = EARMARK_SOURCES.includes(grantSource as LeaveGrantSource);

  useEffect(() => {
    if (open) {
      if (isAdjust && adjustTarget) {
        resetAdjust({
          amount_days: adjustTarget.amount_days,
          expires_at: adjustTarget.expires_at,
          earmark: adjustTarget.earmark ?? '',
          remark: '',
        });
      } else {
        resetGrant({
          employee_id: defaultEmployeeId ?? '',
          amount_days: 0,
          expires_at: '',
          source: LeaveGrantSource.ADJUSTMENT,
          earmark: '',
          remark: '',
          effective_from: '',
        });
      }
    }
  }, [open, isAdjust, adjustTarget, defaultEmployeeId, resetGrant, resetAdjust]);

  async function onGrantSubmit(values: GrantFormValues) {
    try {
      const body: LeaveGrantWriteRequest = {
        employee_id: values.employee_id,
        amount_days: values.amount_days,
        expires_at: values.expires_at,
        source: values.source,
        remark: values.remark,
        ...(showEarmark && values.earmark ? { earmark: values.earmark } : {}),
        ...(values.effective_from ? { effective_from: values.effective_from } : {}),
      };
      await createMutation.mutateAsync({ data: body });
      toast({
        tone: 'success',
        title: t('grant.successTitle'),
        description: t('grant.successDesc'),
      });
      onSuccess();
      onClose();
    } catch (err) {
      if (!applyFieldErrors(err, setGrantError)) {
        const { message } = classifyError(err);
        toast({ tone: 'error', title: t('grant.errorTitle'), description: message });
      }
    }
  }

  async function onAdjustSubmit(values: AdjustFormValues) {
    if (!adjustTarget) return;
    try {
      const body: LeaveGrantPatchRequest = {
        remark: values.remark,
        ...(values.amount_days !== undefined ? { amount_days: values.amount_days } : {}),
        ...(values.expires_at ? { expires_at: values.expires_at } : {}),
        ...(values.earmark !== undefined ? { earmark: values.earmark || null } : {}),
      };
      await adjustMutation.mutateAsync({ id: adjustTarget.id, data: body });
      toast({
        tone: 'success',
        title: t('adjust.successTitle'),
        description: t('adjust.successDesc'),
      });
      onSuccess();
      onClose();
    } catch (err) {
      if (!applyFieldErrors(err, setAdjustError)) {
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
      {isAdjust ? (
        /* ── Adjust form ─────────────────────────────────────────────────── */
        <form onSubmit={handleAdjustSubmit(onAdjustSubmit)}>
          <ModalHeader
            icon={Settings2}
            tone="neutral"
            title={t('adjust.title')}
            onClose={onClose}
          />
          <ModalBody>
            {adjustTarget && (
              <p className="mb-[14px] text-[12px] text-text-3">
                <IdChip id={adjustTarget.id} /> ·{' '}
                {adjustTarget.employee_name ?? adjustTarget.employee_id}
              </p>
            )}

            {/* Current stats */}
            {adjustTarget && (
              <div className="grid grid-cols-3 gap-[10px] mb-[14px]">
                {(
                  [
                    { label: t('adjust.amountCurrent'), value: adjustTarget.amount_days },
                    { label: t('adjust.consumedCurrent'), value: adjustTarget.consumed_days },
                    { label: t('adjust.remainingCurrent'), value: remainingDays(adjustTarget) },
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
            )}

            <FormField
              label={t('adjust.amountLabel')}
              htmlFor="adj-amount"
              error={adjustErrors.amount_days?.message}
            >
              <input
                id="adj-amount"
                type="number"
                className="w-full rounded-[8px] border border-border bg-surface py-[10px] px-[12px] text-[14px] text-text focus:outline-none focus:ring-2 focus:ring-primary"
                {...registerAdjust('amount_days', { valueAsNumber: true })}
              />
            </FormField>

            <FormField
              label={t('adjust.expiresLabel')}
              htmlFor="adj-expires"
              error={adjustErrors.expires_at?.message}
            >
              <input
                id="adj-expires"
                type="date"
                className="w-full rounded-[8px] border border-border bg-surface py-[10px] px-[12px] text-[14px] text-text focus:outline-none focus:ring-2 focus:ring-primary"
                {...registerAdjust('expires_at')}
              />
            </FormField>

            <FormField
              label={t('adjust.remarkLabel')}
              htmlFor="adj-remark"
              required
              error={adjustErrors.remark?.message}
            >
              <textarea
                id="adj-remark"
                rows={3}
                className="w-full rounded-[8px] border border-border bg-surface py-[10px] px-[12px] text-[14px] text-text focus:outline-none focus:ring-2 focus:ring-primary resize-none"
                placeholder={t('adjust.remarkPlaceholder')}
                {...registerAdjust('remark')}
              />
            </FormField>

            <div className="flex items-start gap-[8px] rounded-[8px] border border-info-bd bg-info-bg py-[10px] px-[12px] mt-[14px]">
              <Info aria-hidden className="h-[14px] w-[14px] shrink-0 text-info-tx mt-[1px]" />
              <span className="text-[12px] text-info-tx leading-[1.4]">
                {t('adjust.auditNote')}
              </span>
            </div>
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="secondary" onClick={onClose}>
              {t('actions.cancel')}
            </Button>
            <Button type="submit" variant="primary" disabled={adjustSubmitting}>
              {t('adjust.saveBtn')}
            </Button>
          </ModalFooter>
        </form>
      ) : (
        /* ── Grant (create) form ─────────────────────────────────────────── */
        <form onSubmit={handleGrantSubmit(onGrantSubmit)}>
          <ModalHeader icon={PackagePlus} tone="brand" title={t('grant.title')} onClose={onClose} />
          <ModalBody>
            {/* Employee ID */}
            <FormField
              label={t('grant.employeeLabel')}
              htmlFor="g-employee"
              required
              error={grantErrors.employee_id?.message}
            >
              <input
                id="g-employee"
                type="text"
                className="w-full rounded-[8px] border border-border bg-surface py-[10px] px-[12px] text-[14px] text-text focus:outline-none focus:ring-2 focus:ring-primary"
                placeholder={t('grant.employeePlaceholder')}
                {...registerGrant('employee_id')}
              />
            </FormField>

            {/* Amount days */}
            <FormField
              label={t('grant.amountLabel')}
              htmlFor="g-amount"
              required
              error={grantErrors.amount_days?.message}
            >
              <input
                id="g-amount"
                type="number"
                min={0}
                className="w-full rounded-[8px] border border-border bg-surface py-[10px] px-[12px] text-[14px] text-text focus:outline-none focus:ring-2 focus:ring-primary"
                {...registerGrant('amount_days', { valueAsNumber: true })}
              />
            </FormField>

            {/* Expires at */}
            <FormField
              label={t('grant.expiresLabel')}
              htmlFor="g-expires"
              required
              error={grantErrors.expires_at?.message}
            >
              <input
                id="g-expires"
                type="date"
                className="w-full rounded-[8px] border border-border bg-surface py-[10px] px-[12px] text-[14px] text-text focus:outline-none focus:ring-2 focus:ring-primary"
                {...registerGrant('expires_at')}
              />
            </FormField>

            {/* Source */}
            <FormField
              label={t('grant.sourceLabel')}
              htmlFor="g-source"
              required
              error={grantErrors.source?.message}
            >
              <select
                id="g-source"
                className="w-full rounded-[8px] border border-border bg-surface py-[10px] px-[12px] text-[14px] text-text focus:outline-none focus:ring-2 focus:ring-primary"
                {...registerGrant('source')}
              >
                {Object.values(LeaveGrantSource).map((s) => (
                  <option key={s} value={s}>
                    {t(sourceLabelKey(s))}
                  </option>
                ))}
              </select>
            </FormField>

            {/* Earmark — only for statutory/maternity sources */}
            {showEarmark && (
              <FormField
                label={t('grant.earmarkLabel')}
                htmlFor="g-earmark"
                error={grantErrors.earmark?.message}
              >
                <input
                  id="g-earmark"
                  type="text"
                  className="w-full rounded-[8px] border border-border bg-surface py-[10px] px-[12px] text-[14px] text-text focus:outline-none focus:ring-2 focus:ring-primary"
                  placeholder={t('grant.earmarkPlaceholder')}
                  {...registerGrant('earmark')}
                />
                <p className="mt-[4px] text-[12px] text-text-3">{t('grant.earmarkHint')}</p>
              </FormField>
            )}

            {/* Remark (required) */}
            <FormField
              label={t('grant.remarkLabel')}
              htmlFor="g-remark"
              required
              error={grantErrors.remark?.message}
            >
              <textarea
                id="g-remark"
                rows={3}
                className="w-full rounded-[8px] border border-border bg-surface py-[10px] px-[12px] text-[14px] text-text focus:outline-none focus:ring-2 focus:ring-primary resize-none"
                placeholder={t('grant.remarkPlaceholder')}
                {...registerGrant('remark')}
              />
            </FormField>

            {/* Audit note */}
            <div className="flex items-start gap-[8px] rounded-[8px] border border-info-bd bg-info-bg py-[10px] px-[12px] mt-[14px]">
              <Info aria-hidden className="h-[14px] w-[14px] shrink-0 text-info-tx mt-[1px]" />
              <span className="text-[12px] text-info-tx leading-[1.4]">{t('grant.auditNote')}</span>
            </div>
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="secondary" onClick={onClose}>
              {t('actions.cancel')}
            </Button>
            <Button type="submit" variant="primary" disabled={grantSubmitting}>
              {t('grant.saveBtn')}
            </Button>
          </ModalFooter>
        </form>
      )}
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// EmployeePoolSummary — inline balance summary shown above the lot table
// ---------------------------------------------------------------------------

interface PoolSummaryProps {
  employeeId: string;
}

function EmployeePoolSummary({ employeeId }: PoolSummaryProps) {
  const { t } = useTranslation('leaveQuotas');
  const balanceQuery = useGetLeaveBalanceByEmployee(employeeId, undefined);

  const outer = balanceQuery.data?.data as
    | ({
        pool_remaining: number;
        next_expiry?: string | null;
        earmarked: { earmark: string; remaining_days: number; expires_at: string }[];
      } & { data?: unknown })
    | undefined;
  const balance =
    outer && typeof outer === 'object' && 'data' in outer && outer.data
      ? (outer.data as typeof outer)
      : outer;

  if (balanceQuery.isLoading) {
    return <div className="h-[72px] animate-pulse rounded-[10px] bg-surface-2" />;
  }

  if (!balance) return null;

  return (
    <div className="rounded-[10px] border border-border bg-surface-2 px-[16px] py-[14px] flex items-center gap-[24px]">
      {/* Pool total */}
      <div className="flex flex-col gap-[2px]">
        <span className="text-[28px] font-bold text-ok-tx leading-none">
          {balance.pool_remaining}
        </span>
        <span className="text-[12px] text-text-3">{t('pool.remainingLabel')}</span>
      </div>

      {/* Next expiry hint */}
      {balance.next_expiry && (
        <div className="flex items-center gap-[6px] text-[12px] text-text-3">
          <CalendarPlus aria-hidden className="h-[13px] w-[13px] shrink-0" />
          <span>
            {t('pool.expiryHint')}{' '}
            <DateText kind="date" value={balance.next_expiry} className="font-medium text-text" />
          </span>
        </div>
      )}

      {/* Earmarked lots */}
      {balance.earmarked.length > 0 && (
        <div className="ml-auto flex items-center gap-[8px] flex-wrap">
          {balance.earmarked.map((el) => (
            <div
              key={el.earmark}
              className="flex items-center gap-[4px] rounded-full bg-warn-bg px-[10px] py-[4px]"
            >
              <Tag aria-hidden className="h-[10px] w-[10px] text-warn-tx" />
              <span className="text-[11px] font-semibold text-warn-tx">
                {el.earmark} · {el.remaining_days} {t('pool.days')}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// LeaveQuotasScreen
// ---------------------------------------------------------------------------

export function LeaveQuotasScreen() {
  const { t } = useTranslation('leaveQuotas');
  const navigate = useNavigate();
  const search = useSearch({ strict: false }) as LeaveQuotasSearch;
  const queryClient = useQueryClient();

  const [grantOpen, setGrantOpen] = useState(false);
  const [adjustTarget, setAdjustTarget] = useState<LeaveGrant | null>(null);

  const queryParams: ListLeaveGrantsParams = {
    limit: PAGE_SIZE,
    ...(search.employee_id ? { employee_id: search.employee_id } : {}),
    ...(search.cursor ? { cursor: search.cursor } : {}),
  };

  const grantsQuery = useListLeaveGrants(queryParams);

  type GrantListData = { data: LeaveGrant[]; next_cursor?: string | null; has_more: boolean };
  const grantsOuter = grantsQuery.data?.data as { data: GrantListData } | GrantListData | undefined;
  const grantList =
    grantsOuter &&
    typeof grantsOuter === 'object' &&
    'data' in grantsOuter &&
    typeof (grantsOuter as { data: unknown }).data === 'object' &&
    !Array.isArray((grantsOuter as { data: unknown }).data)
      ? (grantsOuter as { data: GrantListData }).data
      : (grantsOuter as GrantListData | undefined);

  const rows: LeaveGrant[] = grantList?.data ?? [];
  const hasMore = grantList?.has_more ?? false;
  const nextCursor = grantList?.next_cursor ?? null;

  type NavFn = (o: { to: string; search?: Record<string, unknown> }) => void;
  const nav = navigate as unknown as NavFn;

  function setSearch(patch: Partial<LeaveQuotasSearch>) {
    nav({ to: '/leave/quotas', search: { ...search, ...patch, cursor: undefined } });
  }

  function onRefetch() {
    grantsQuery.refetch();
    // Also invalidate balance query if employee filter is active
    if (search.employee_id) {
      queryClient.invalidateQueries({
        queryKey: getGetLeaveBalanceByEmployeeQueryKey(search.employee_id),
      });
    }
    queryClient.invalidateQueries({ queryKey: getListLeaveGrantsQueryKey() });
  }

  const error = grantsQuery.error ? classifyError(grantsQuery.error) : null;

  // ---------------------------------------------------------------------------
  // Columns
  // ---------------------------------------------------------------------------

  const columns: Column<LeaveGrant>[] = [
    {
      id: 'employee',
      header: t('table.employee'),
      width: 220,
      cell: (row) => (
        <div className="flex flex-col min-w-0">
          <span className="text-[14px] font-medium text-text truncate">
            {row.employee_name ?? row.employee_id}
          </span>
          <span className="text-[12px] text-text-3 font-mono">{row.employee_id}</span>
        </div>
      ),
    },
    {
      id: 'source',
      header: t('table.source'),
      width: 130,
      cell: (row) => (
        <span className="text-[13px] text-text-2">
          {t(sourceLabelKey(row.source as LeaveGrantSource))}
        </span>
      ),
    },
    {
      id: 'earmark',
      header: t('table.earmark'),
      width: 150,
      cell: (row) =>
        row.earmark ? (
          <StatusBadge tone={earmarkBadgeTone(row.earmark)} dot={false}>
            {row.earmark}
          </StatusBadge>
        ) : (
          <span className="text-[12px] text-text-3">{t('table.poolGeneral')}</span>
        ),
    },
    {
      id: 'amount',
      header: t('table.amount'),
      width: 100,
      align: 'right',
      cell: (row) => <span className="text-[14px] font-semibold text-text">{row.amount_days}</span>,
    },
    {
      id: 'consumed',
      header: t('table.consumed'),
      width: 100,
      align: 'right',
      cell: (row) => <span className="text-[14px] text-text">{row.consumed_days}</span>,
    },
    {
      id: 'pending',
      header: t('table.pending'),
      width: 100,
      align: 'right',
      cell: (row) => (
        <span
          className={`text-[14px] ${row.pending_days > 0 ? 'text-warn-tx font-medium' : 'text-text-3'}`}
        >
          {row.pending_days}
        </span>
      ),
    },
    {
      id: 'remaining',
      header: t('table.remaining'),
      width: 110,
      align: 'right',
      cell: (row) => {
        const rem = remainingDays(row);
        return (
          <span
            className={`text-[14px] font-semibold ${
              rem < 0 ? 'text-bad-tx' : rem === 0 ? 'text-text-3' : 'text-ok-tx'
            }`}
          >
            {rem}
          </span>
        );
      },
    },
    {
      id: 'expires_at',
      header: t('table.expires'),
      width: 130,
      cell: (row) => (
        <DateText kind="date" value={row.expires_at} className="text-[13px] text-text-2" />
      ),
    },
    {
      id: 'remark',
      header: t('table.remark'),
      width: 200,
      cell: (row) => (
        <span className="text-[12px] text-text-3 truncate" title={row.remark ?? ''}>
          {row.remark ?? '—'}
        </span>
      ),
    },
    {
      id: 'actions',
      header: '',
      width: 110,
      align: 'right',
      cell: (row) => (
        <Button
          type="button"
          variant="secondary"
          size="sm"
          aria-label={t('actions.adjustAriaLabel', { id: row.id })}
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

  const isLoading = grantsQuery.isLoading;
  const isEmpty = !isLoading && !error && rows.length === 0;
  const hasActiveFilter = !!(search.employee_id || search.q);
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
        <Button type="button" variant="primary" onClick={() => setGrantOpen(true)}>
          <PackagePlus aria-hidden className="h-[16px] w-[16px]" />
          {t('actions.grantLot')}
        </Button>
      </div>

      {/* Per-employee pool summary (when filtered to one employee) */}
      {search.employee_id && <EmployeePoolSummary employeeId={search.employee_id} />}

      {/* Filters */}
      <div className="flex items-center gap-[10px] w-full">
        <SearchField
          value={search.employee_id ?? ''}
          onChange={(e) => setSearch({ employee_id: e.target.value || undefined })}
          placeholder={t('filters.searchPlaceholder')}
        />
        <FilterSelect value="" onChange={() => {}}>
          <option value="">{t('filters.allSources')}</option>
          {Object.values(LeaveGrantSource).map((s) => (
            <option key={s} value={s}>
              {t(sourceLabelKey(s))}
            </option>
          ))}
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

      {/* Grant-lot modal (create) */}
      <GrantLotModal
        open={grantOpen}
        defaultEmployeeId={search.employee_id}
        onClose={() => setGrantOpen(false)}
        onSuccess={onRefetch}
      />

      {/* Adjust modal (patch) */}
      {adjustTarget && (
        <GrantLotModal
          adjustTarget={adjustTarget}
          open
          onClose={() => setAdjustTarget(null)}
          onSuccess={onRefetch}
        />
      )}
    </div>
  );
}
