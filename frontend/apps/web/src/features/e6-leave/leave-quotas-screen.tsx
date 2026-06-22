/**
 * E6 · Kuota Cuti (HR) — per-type quota ledger (2026-06-12)
 *
 * .pen frames:
 *   P6HZ7E  E6 · Kuota & Hibah Cuti (HR) — employee directory + per-type quota row
 *   ny4xT   Sesuaikan Kuota (modal) — adjust an existing (employee, type, window) entitlement
 *   qDhTz   Tambah Kuota (modal)   — assign / raise a per-type quota
 *
 * Per-type cap_basis ledger (EPICS §8): leave_type is the cap axis; each type meters a quota
 * window keyed by period_key. The HR directory (canonical DataTable) lists employees (E2) with
 * their leave-rights config status: "Belum dikonfigurasi → Atur Hak Cuti" (routes to the employee
 * Hak Cuti tab) or "{n} jenis cuti aktif → Kelola" (drills into the per-type quota breakdown).
 * Manual entitlement changes go through POST /leave-quotas:adjust-entitled (LQ-6, reason required,
 * audited).
 *
 * ENGINEERING.md D1 — typed URL search params. B1 — classifyError / applyFieldErrors.
 */

import { applyFieldErrors, classifyError } from '@/lib/api-error.ts';
import { zodResolver } from '@hookform/resolvers/zod';
import { useListEmployees } from '@swp/api-client/e2';
import {
  type AdjustTypeQuotaBody,
  type LeaveTypeBalance,
  useAdjustTypeQuota,
  useGetEmployeeTypeBalances,
} from '@swp/api-client/e6';
import { isQuotaBearing } from '@swp/shared/leave';
import {
  Avatar,
  Button,
  type Column,
  Combobox,
  type ComboboxOption,
  CursorPagination,
  DataTable,
  DateText,
  EmptyState,
  FilterRow,
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
import { ArrowRight, ChevronLeft, Info, PackagePlus, Settings2 } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';

// ---------------------------------------------------------------------------
// Search params (D1 — typed URL search params)
// ---------------------------------------------------------------------------

export type LeaveQuotasSearch = {
  /** Free-text employee search (name / NIK / NIP → q param). */
  q?: string;
  /** Employee selected for the per-type drill-in. */
  employee_id?: string;
  cursor?: string;
};

const PAGE_SIZE = 25;
/** Any date inside the current window — the server resolves the window by cap_basis. */
const today = () => new Date().toISOString().slice(0, 10);

// ---------------------------------------------------------------------------
// Envelope unwrap (mutator may single- or double-nest `data`)
// ---------------------------------------------------------------------------

function unwrap<T>(raw: unknown): T | undefined {
  if (raw && typeof raw === 'object' && 'data' in raw) {
    const inner = (raw as { data: unknown }).data;
    if (inner && typeof inner === 'object' && 'data' in inner) {
      return (inner as { data: T }).data;
    }
    return inner as T;
  }
  return raw as T | undefined;
}

type EmployeeRow = { id: string; full_name: string; nik: string; nip?: string };

// ---------------------------------------------------------------------------
// Zod — adjust/add form (hand-written; Zod codegen deferred, WEB-STACK §4)
// ---------------------------------------------------------------------------

const quotaSchema = z.object({
  employee_id: z.string().min(1, 'Pilih karyawan'),
  leave_type_id: z.string().min(1, 'Pilih jenis cuti'),
  delta: z
    .number({ invalid_type_error: 'Wajib diisi' })
    .int('Harus bilangan bulat')
    .refine((v) => v !== 0, 'Tidak boleh 0'),
  reason: z.string().min(5, 'Catatan minimal 5 karakter').max(500, 'Catatan maksimal 500 karakter'),
});
type QuotaFormValues = z.infer<typeof quotaSchema>;

/** The (employee, type, window) being adjusted from the drill-in. */
export interface AdjustTarget {
  employeeId: string;
  employeeName: string;
  leaveTypeId: string;
  code: string;
  name: string;
  entitled: number;
  used: number;
  remaining: number;
}

// ---------------------------------------------------------------------------
// EmployeeCombobox — typeahead resolving employee_id from name/NIK/NIP
// ---------------------------------------------------------------------------

function EmployeeCombobox({
  value,
  onChange,
  error,
  placeholder,
}: {
  value: string | null;
  onChange: (id: string | null) => void;
  error?: string;
  placeholder?: string;
}) {
  const [q, setQ] = useState('');
  const listQuery = useListEmployees(q.length >= 1 ? { q, limit: 20 } : undefined, {
    query: { enabled: q.length >= 1 },
  });
  const employees = unwrap<EmployeeRow[]>(listQuery.data?.data) ?? [];
  const options: ComboboxOption[] = employees.map((e) => ({
    value: e.id,
    label: e.full_name,
    sublabel: e.nip ?? e.nik,
  }));
  const [selected, setSelected] = useState<ComboboxOption | null>(null);
  const merged =
    selected && !options.some((o) => o.value === selected.value) ? [selected, ...options] : options;

  return (
    <Combobox
      value={value}
      onChange={(id) => {
        setSelected(id ? (options.find((o) => o.value === id) ?? selected) : null);
        onChange(id);
      }}
      options={merged}
      onSearch={setQ}
      isLoading={listQuery.isLoading}
      placeholder={placeholder ?? 'Ketik nama, NIK, atau NIP…'}
      emptyText={q.length < 1 ? 'Ketik untuk mencari karyawan' : 'Tidak ada karyawan ditemukan'}
      error={!!error}
    />
  );
}

// ---------------------------------------------------------------------------
// QuotaModal — adjust-entitled (add mode: pick employee+type; adjust mode: fixed target)
// ---------------------------------------------------------------------------

function QuotaModal({
  target,
  open,
  onClose,
  onSuccess,
}: {
  /** Set → "Sesuaikan Kuota" (ny4xT) for a fixed (employee, type). Unset → "Tambah Kuota" (qDhTz). */
  target?: AdjustTarget;
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}) {
  const { t } = useTranslation('leaveQuotas');
  const { toast } = useToast();
  const isAdjust = Boolean(target);
  const mutation = useAdjustTypeQuota();

  const {
    register,
    handleSubmit,
    reset,
    setError,
    setValue,
    watch,
    formState: { errors, isSubmitting },
  } = useForm<QuotaFormValues>({
    resolver: zodResolver(quotaSchema),
    defaultValues: { employee_id: '', leave_type_id: '', delta: 0, reason: '' },
  });

  useEffect(() => {
    if (open) {
      reset({
        employee_id: target?.employeeId ?? '',
        leave_type_id: target?.leaveTypeId ?? '',
        delta: 0,
        reason: '',
      });
    }
  }, [open, target, reset]);

  const employeeId = watch('employee_id');
  const delta = watch('delta');

  // Type options for "Tambah Kuota" come from the selected employee's per-type balances, filtered
  // to quota-bearing cap_basis. PER_EVENT / UNCAPPED types have no adjustable window (the server
  // would 422 with "tidak punya kuota yang bisa disesuaikan"), so they are never offered.
  const typeBalQuery = useGetEmployeeTypeBalances(employeeId, {
    query: { enabled: open && !isAdjust && !!employeeId },
  });
  const quotaBearingTypes = (unwrap<LeaveTypeBalance[]>(typeBalQuery.data?.data) ?? []).filter(
    (b) => isQuotaBearing(b.cap_basis),
  );
  // Adjust mode previews the resulting entitlement/remaining ("Total baru" / "Sisa baru").
  const newEntitled = target ? target.entitled + (Number.isFinite(delta) ? delta : 0) : 0;
  const newRemaining = target ? newEntitled - target.used : 0;

  async function onSubmit(values: QuotaFormValues) {
    try {
      const body: AdjustTypeQuotaBody = {
        employee_id: values.employee_id,
        leave_type_id: values.leave_type_id,
        start_date: today(),
        delta: values.delta,
        reason: values.reason,
      };
      await mutation.mutateAsync({ data: body });
      toast({ tone: 'success', title: t('save.successTitle'), description: t('save.successDesc') });
      onSuccess();
      onClose();
    } catch (err) {
      if (!applyFieldErrors(err, setError)) {
        const { message } = classifyError(err);
        toast({ tone: 'error', title: t('save.errorTitle'), description: message });
      }
    }
  }

  const inputCls =
    'w-full rounded-[8px] border border-border bg-surface py-[10px] px-[12px] text-[14px] text-text focus:outline-none focus:ring-2 focus:ring-primary';

  return (
    <Modal
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
      size="md"
    >
      <form onSubmit={handleSubmit(onSubmit)}>
        <ModalHeader
          icon={isAdjust ? Settings2 : PackagePlus}
          tone={isAdjust ? 'neutral' : 'brand'}
          title={isAdjust ? t('adjust.title') : t('add.title')}
          onClose={onClose}
        />
        <ModalBody>
          {isAdjust && target && (
            <>
              <p className="mb-[14px] text-[12px] text-text-3">
                {target.employeeName} · {target.name} ({target.code})
              </p>
              <div className="grid grid-cols-3 gap-[10px] mb-[14px]">
                {(
                  [
                    { label: t('adjust.entitledCurrent'), value: target.entitled },
                    { label: t('adjust.usedCurrent'), value: target.used },
                    { label: t('adjust.remainingCurrent'), value: target.remaining },
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
            </>
          )}

          {!isAdjust && (
            <>
              <FormField
                label={t('add.employeeLabel')}
                htmlFor="q-employee"
                required
                error={errors.employee_id?.message}
              >
                <EmployeeCombobox
                  value={employeeId || null}
                  onChange={(id) => setValue('employee_id', id ?? '', { shouldValidate: true })}
                  error={errors.employee_id?.message}
                />
              </FormField>

              <FormField
                label={t('add.typeLabel')}
                htmlFor="q-type"
                required
                error={errors.leave_type_id?.message}
              >
                <select
                  id="q-type"
                  className={inputCls}
                  disabled={!employeeId}
                  {...register('leave_type_id')}
                >
                  <option value="">
                    {employeeId ? t('add.typePlaceholder') : t('add.typePlaceholderNoEmployee')}
                  </option>
                  {quotaBearingTypes.map((b) => (
                    <option key={b.leave_type_id} value={b.leave_type_id}>
                      {b.name} ({b.code})
                    </option>
                  ))}
                </select>
              </FormField>
            </>
          )}

          <FormField
            label={isAdjust ? t('adjust.deltaLabel') : t('add.deltaLabel')}
            htmlFor="q-delta"
            required
            error={errors.delta?.message}
          >
            <input
              id="q-delta"
              type="number"
              className={inputCls}
              {...register('delta', { valueAsNumber: true })}
            />
          </FormField>

          {isAdjust && target && (
            <div className="grid grid-cols-2 gap-[10px] mb-[2px]">
              <div className="flex flex-col gap-[2px] rounded-[8px] bg-surface-2 py-[10px] px-[12px]">
                <span className="text-[11px] text-text-3">{t('adjust.newEntitled')}</span>
                <span className="text-[16px] font-bold text-text leading-none">{newEntitled}</span>
              </div>
              <div className="flex flex-col gap-[2px] rounded-[8px] bg-surface-2 py-[10px] px-[12px]">
                <span className="text-[11px] text-text-3">{t('adjust.newRemaining')}</span>
                <span
                  className={`text-[16px] font-bold leading-none ${newRemaining < 0 ? 'text-bad-tx' : 'text-text'}`}
                >
                  {newRemaining}
                </span>
              </div>
            </div>
          )}

          <FormField
            label={t('save.reasonLabel')}
            htmlFor="q-reason"
            required
            error={errors.reason?.message}
          >
            <textarea
              id="q-reason"
              rows={3}
              className={`${inputCls} resize-none`}
              placeholder={t('save.reasonPlaceholder')}
              {...register('reason')}
            />
          </FormField>

          <div className="flex items-start gap-[8px] rounded-[8px] border border-info-bd bg-info-bg py-[10px] px-[12px] mt-[14px]">
            <Info aria-hidden className="h-[14px] w-[14px] shrink-0 text-info-tx mt-[1px]" />
            <span className="text-[12px] text-info-tx leading-[1.4]">{t('save.auditNote')}</span>
          </div>
        </ModalBody>
        <ModalFooter>
          <Button type="button" variant="secondary" onClick={onClose}>
            {t('actions.cancel')}
          </Button>
          <Button type="submit" variant="primary" disabled={isSubmitting}>
            {isAdjust ? t('adjust.saveBtn') : t('add.saveBtn')}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// Directory row — fetches the employee's per-type balances; renders the CT line
// ---------------------------------------------------------------------------

function initials(name: string): string {
  return name
    .split(/\s+/)
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase() ?? '')
    .join('');
}

// LeaveRightsCell is the "Hak Cuti" column body: it fetches the employee's assigned types
// (entitlement-scoped balance, ELE-1) and renders the config status + the navigate action.
// configured → "Kelola" (per-type quota drill-in); unconfigured → "Atur Hak Cuti" (Hak Cuti tab).
function LeaveRightsCell({
  emp,
  onSelect,
  onConfigure,
}: {
  emp: EmployeeRow;
  onSelect: () => void;
  onConfigure: () => void;
}) {
  const { t } = useTranslation('leaveQuotas');
  const balQuery = useGetEmployeeTypeBalances(emp.id);
  const balances = unwrap<LeaveTypeBalance[]>(balQuery.data?.data) ?? [];

  if (balQuery.isLoading) {
    return <span className="inline-block h-[14px] w-[160px] rounded bg-surface-2 animate-pulse" />;
  }
  const configured = balances.length > 0;
  return (
    <button
      type="button"
      onClick={configured ? onSelect : onConfigure}
      className="inline-flex items-center gap-3"
    >
      <StatusBadge dot tone={configured ? 'ok' : 'warn'}>
        {configured ? t('status.configured', { count: balances.length }) : t('status.unconfigured')}
      </StatusBadge>
      <span className="inline-flex items-center gap-1 text-[13px] font-semibold text-primary">
        {configured ? t('actions.manageRights') : t('actions.configureRights')}
        <ArrowRight aria-hidden className="h-[14px] w-[14px]" />
      </span>
    </button>
  );
}

// ---------------------------------------------------------------------------
// Per-type drill-in — full cap_basis breakdown for one employee
// ---------------------------------------------------------------------------

function EmployeeTypeDetail({
  emp,
  onBack,
  onAdjust,
}: {
  emp: EmployeeRow;
  onBack: () => void;
  onAdjust: (target: AdjustTarget) => void;
}) {
  const { t } = useTranslation('leaveQuotas');
  const balQuery = useGetEmployeeTypeBalances(emp.id);
  const balances = unwrap<LeaveTypeBalance[]>(balQuery.data?.data) ?? [];
  const error = balQuery.error ? classifyError(balQuery.error) : null;
  const isEmpty = !balQuery.isLoading && !error && balances.length === 0;

  const columns: Column<LeaveTypeBalance>[] = [
    {
      id: 'type',
      header: t('detail.type'),
      cell: (b) => (
        <div className="flex items-center gap-[8px] min-w-0">
          <span
            className="h-[10px] w-[10px] shrink-0 rounded-full"
            style={{ backgroundColor: b.color }}
          />
          <div className="flex flex-col min-w-0">
            <span className="text-[14px] text-text truncate">{b.name}</span>
            <span className="text-[11px] font-mono text-text-3">{b.code}</span>
          </div>
        </div>
      ),
    },
    {
      id: 'entitled',
      header: t('detail.entitled'),
      width: 110,
      align: 'right',
      cell: (b) =>
        b.has_window || b.entitled_days != null || b.cap_value != null ? (
          <span className="font-mono text-[14px] font-semibold text-text">
            {b.entitled_days ?? b.cap_value ?? 0}
          </span>
        ) : (
          <span className="text-[12px] text-text-3">{t('detail.perRule')}</span>
        ),
    },
    {
      id: 'used',
      header: t('detail.used'),
      width: 90,
      align: 'right',
      cell: (b) => <span className="font-mono text-[14px] text-text-2">{b.used_days}</span>,
    },
    {
      id: 'pending',
      header: t('detail.pending'),
      width: 90,
      align: 'right',
      cell: (b) => (
        <span
          className={`font-mono text-[14px] ${b.pending_days > 0 ? 'text-warn-tx font-medium' : 'text-text-3'}`}
        >
          {b.pending_days}
        </span>
      ),
    },
    {
      id: 'remaining',
      header: t('detail.remaining'),
      width: 90,
      align: 'right',
      cell: (b) =>
        b.remaining_days == null ? (
          <span className="text-[13px] text-text-3">—</span>
        ) : (
          <span
            className={`font-mono text-[14px] font-semibold ${b.remaining_days <= 0 ? 'text-text-3' : 'text-ok-tx'}`}
          >
            {b.remaining_days}
          </span>
        ),
    },
    {
      id: 'expiry',
      header: t('detail.expiry'),
      width: 130,
      cell: (b) =>
        b.expires_at ? (
          <DateText kind="date" value={b.expires_at} className="text-[13px] text-text-2" />
        ) : (
          <span className="text-[13px] text-text-3">—</span>
        ),
    },
    {
      id: 'actions',
      header: '',
      width: 120,
      align: 'right',
      // Every assigned type is adjustable: quota-bearing types adjust their window; an
      // otherwise "sesuai ketentuan" type (PER_EVENT / UNCAPPED) gets its base DEFINED on
      // the first adjust (server upserts the entitlement + opens the lifetime window).
      cell: (b) => (
        <Button
          type="button"
          variant="secondary"
          size="sm"
          onClick={() =>
            onAdjust({
              employeeId: emp.id,
              employeeName: emp.full_name,
              leaveTypeId: b.leave_type_id,
              code: b.code,
              name: b.name,
              entitled: b.entitled_days ?? b.cap_value ?? 0,
              used: b.used_days,
              remaining: b.remaining_days ?? 0,
            })
          }
        >
          {t('actions.adjust')}
        </Button>
      ),
    },
  ];

  return (
    <div className="flex flex-col gap-[16px] w-full">
      <div className="flex items-center gap-[10px]">
        <Button type="button" variant="secondary" size="sm" onClick={onBack}>
          <ChevronLeft aria-hidden className="h-[14px] w-[14px]" />
          {t('actions.back')}
        </Button>
        <div className="flex flex-col gap-[2px]">
          <span className="text-[16px] font-semibold text-text">{emp.full_name}</span>
          <span className="text-[12px] font-mono text-text-3">{emp.id}</span>
        </div>
      </div>

      {error ? (
        <StateView
          kind="error"
          title={t('states.errorTitle')}
          description={error.message}
          onRetry={() => balQuery.refetch()}
          retryLabel={t('actions.retry')}
        />
      ) : (
        <div className="rounded-[12px] border border-border bg-surface overflow-hidden w-full">
          <DataTable
            columns={columns}
            data={balances}
            getRowId={(b) => b.leave_type_id}
            isLoading={balQuery.isLoading}
            empty={
              isEmpty ? (
                <EmptyState
                  variant="fresh"
                  title={t('states.freshTitle')}
                  description={t('states.freshDesc')}
                />
              ) : undefined
            }
          />
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Screen
// ---------------------------------------------------------------------------

export function LeaveQuotasScreen() {
  const { t } = useTranslation('leaveQuotas');
  const navigate = useNavigate();
  const search = useSearch({ strict: false }) as LeaveQuotasSearch;
  const queryClient = useQueryClient();

  const [adjustTarget, setAdjustTarget] = useState<AdjustTarget | null>(null);

  const listQuery = useListEmployees({
    limit: PAGE_SIZE,
    ...(search.q ? { q: search.q } : {}),
    ...(search.cursor ? { cursor: search.cursor } : {}),
  });

  // The employee list envelope is { data: EmployeeRow[], next_cursor, has_more } — a SINGLE
  // pagination wrapper, not the double-nested mutator shape. unwrap<EmployeeRow[]> peels straight
  // to the rows (same as the picker); pagination is read off the body alongside `data`.
  const employees: EmployeeRow[] = unwrap<EmployeeRow[]>(listQuery.data?.data) ?? [];
  const pageBody = listQuery.data?.data as
    | { next_cursor?: string | null; has_more?: boolean }
    | undefined;
  const hasMore = pageBody?.has_more ?? false;
  const nextCursor = pageBody?.next_cursor ?? null;

  type NavFn = (o: {
    to: string;
    params?: Record<string, unknown>;
    search?: Record<string, unknown>;
  }) => void;
  const nav = navigate as unknown as NavFn;

  const selected = search.employee_id
    ? (employees.find((e) => e.id === search.employee_id) ?? null)
    : null;

  function setSearch(patch: Partial<LeaveQuotasSearch>) {
    nav({ to: '/leave/quotas', search: { ...search, ...patch, cursor: undefined } });
  }
  function onSuccess() {
    listQuery.refetch();
    queryClient.invalidateQueries({
      predicate: (q) =>
        q.queryKey.some((k) => typeof k === 'string' && k.includes('leave-balances')),
    });
  }

  const listError = listQuery.error ? classifyError(listQuery.error) : null;
  const isLoading = listQuery.isLoading;
  const hasActiveFilter = !!search.q;
  const isForbidden = listError?.kind === 'forbidden';

  // ── Drill-in ────────────────────────────────────────────────────────────
  if (selected) {
    return (
      <div className="flex flex-col gap-[18px] p-[24px] bg-app-bg min-h-full w-full">
        <div className="flex flex-col gap-[4px]">
          <h1 className="text-[30px] font-bold text-text leading-none">{t('title')}</h1>
          <p className="text-[14px] text-text-3">{t('subtitle')}</p>
        </div>
        <EmployeeTypeDetail
          emp={selected}
          onBack={() => nav({ to: '/leave/quotas', search: { q: search.q } })}
          onAdjust={setAdjustTarget}
        />
        {adjustTarget && (
          <QuotaModal
            target={adjustTarget}
            open
            onClose={() => setAdjustTarget(null)}
            onSuccess={onSuccess}
          />
        )}
      </div>
    );
  }

  // ── Directory ─────────────────────────────────────────────────────────────
  const columns: Column<EmployeeRow>[] = [
    {
      id: 'karyawan',
      header: t('table.employee'),
      priority: 'primary',
      width: 320,
      cell: (e) => (
        <div className="flex items-center gap-[8px]">
          <Avatar initials={initials(e.full_name)} size={32} />
          <div className="flex flex-col gap-[2px]">
            <span className="text-[14px] font-semibold text-text">{e.full_name}</span>
            <IdChip id={e.id} />
          </div>
        </div>
      ),
    },
    {
      id: 'hak-cuti',
      header: t('table.leaveRights'),
      priority: 'secondary',
      align: 'right',
      cell: (e) => (
        <LeaveRightsCell
          emp={e}
          onSelect={() =>
            nav({
              to: '/leave/quotas',
              search: { ...search, employee_id: e.id, cursor: undefined },
            })
          }
          onConfigure={() =>
            nav({
              to: '/employees/$employeeId',
              params: { employeeId: e.id },
              search: { tab: 'hak-cuti' },
            })
          }
        />
      ),
    },
  ];

  return (
    <div className="flex flex-col gap-[18px] p-[24px] bg-app-bg min-h-full w-full">
      <div className="flex items-center justify-between w-full">
        <div className="flex flex-col gap-[4px]">
          <h1 className="text-[30px] font-bold text-text leading-none">{t('title')}</h1>
          <p className="text-[14px] text-text-3">{t('subtitle')}</p>
        </div>
      </div>

      <FilterRow>
        <SearchField
          value={search.q ?? ''}
          onChange={(e) => setSearch({ q: e.target.value || undefined })}
          placeholder={t('filters.searchPlaceholder')}
        />
      </FilterRow>

      {isForbidden ? (
        <EmptyState
          variant="no-permission"
          title={t('states.noPermissionTitle')}
          description={t('states.noPermissionDesc')}
        />
      ) : listError ? (
        <StateView
          kind="error"
          title={t('states.errorTitle')}
          description={listError.message}
          onRetry={() => listQuery.refetch()}
          retryLabel={t('actions.retry')}
        />
      ) : (
        <DataTable
          responsive
          aria-label={t('title')}
          columns={columns}
          data={employees}
          getRowId={(e) => e.id}
          isLoading={isLoading}
          skeletonRows={6}
          empty={
            hasActiveFilter ? (
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
          footer={
            employees.length > 0 ? (
              <CursorPagination
                rangeLabel={t('pagination.rangeLabel', { count: employees.length })}
                hasPrev={!!search.cursor}
                hasNext={hasMore}
                onPrev={() =>
                  nav({ to: '/leave/quotas', search: { ...search, cursor: undefined } })
                }
                onNext={() =>
                  nav({
                    to: '/leave/quotas',
                    search: { ...search, cursor: nextCursor ?? undefined },
                  })
                }
              />
            ) : undefined
          }
        />
      )}
    </div>
  );
}
