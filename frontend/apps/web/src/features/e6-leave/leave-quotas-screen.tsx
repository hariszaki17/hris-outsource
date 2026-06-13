/**
 * E6 · Kuota Cuti (HR) — per-type quota ledger (2026-06-12)
 *
 * .pen frames:
 *   P6HZ7E  E6 · Kuota & Hibah Cuti (HR) — employee directory + per-type quota row
 *   ny4xT   Sesuaikan Kuota (modal) — adjust an existing (employee, type, window) entitlement
 *   qDhTz   Tambah Kuota (modal)   — assign / raise a per-type quota
 *
 * Per-type cap_basis ledger (EPICS §8): leave_type is the cap axis; each type meters a quota
 * window keyed by period_key. The HR directory lists employees (E2) with their annual CT quota
 * inline (KUOTA CT / TERPAKAI / PENDING / SISA); clicking a row drills into the full per-type
 * breakdown. Manual entitlement changes go through POST /leave-quotas:adjust-entitled (LQ-6,
 * reason required, audited).
 *
 * ENGINEERING.md D1 — typed URL search params. B1 — classifyError / applyFieldErrors.
 */

import { applyFieldErrors, classifyError } from '@/lib/api-error.ts';
import { zodResolver } from '@hookform/resolvers/zod';
import { useListEmployees } from '@swp/api-client/e2';
import {
  type AdjustTypeQuotaBody,
  type LeaveType,
  type LeaveTypeBalance,
  useAdjustTypeQuota,
  useGetEmployeeTypeBalances,
  useListLeaveTypes,
} from '@swp/api-client/e6';
import {
  Button,
  type Column,
  Combobox,
  type ComboboxOption,
  CursorPagination,
  DataTable,
  DateText,
  EmptyState,
  FormField,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  SearchField,
  StateView,
  useToast,
} from '@swp/ui';
import { useQueryClient } from '@tanstack/react-query';
import { useNavigate, useSearch } from '@tanstack/react-router';
import { ChevronLeft, Info, PackagePlus, Settings2 } from 'lucide-react';
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
const CAP_ANNUAL = 'ANNUAL_POOL';
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

  const typesQuery = useListLeaveTypes(undefined, { query: { enabled: open && !isAdjust } });
  const leaveTypes = (unwrap<LeaveType[]>(typesQuery.data?.data) ?? []).filter((lt) => lt.active);

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
                <select id="q-type" className={inputCls} {...register('leave_type_id')}>
                  <option value="">{t('add.typePlaceholder')}</option>
                  {leaveTypes.map((lt) => (
                    <option key={lt.id} value={lt.id}>
                      {lt.name} ({lt.code})
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

const COL = {
  emp: 300,
  ct: 110,
  used: 100,
  pending: 90,
  remaining: 90,
  special: 90,
  expiry: 160,
} as const;

function initials(name: string): string {
  return name
    .split(/\s+/)
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase() ?? '')
    .join('');
}

function EmployeeQuotaRow({ emp, onSelect }: { emp: EmployeeRow; onSelect: () => void }) {
  const balQuery = useGetEmployeeTypeBalances(emp.id);
  const balances = unwrap<LeaveTypeBalance[]>(balQuery.data?.data) ?? [];
  const ct = balances.find((b) => b.cap_basis === CAP_ANNUAL);
  const specialUsed = balances.filter(
    (b) => b.cap_basis !== CAP_ANNUAL && (b.used_days > 0 || b.pending_days > 0),
  ).length;

  const cell = (w: number, node: React.ReactNode, align: 'left' | 'right' = 'right') => (
    <div
      style={{ width: w }}
      className={`shrink-0 flex items-center ${align === 'right' ? 'justify-end' : ''}`}
    >
      {node}
    </div>
  );
  const mono = 'font-mono text-[14px]';
  const dash = <span className="text-[14px] text-text-3">—</span>;

  return (
    <button
      type="button"
      onClick={onSelect}
      className="flex items-center w-full text-left py-[12px] px-[18px] border-b border-border-subtle bg-surface hover:bg-surface-2 transition-colors"
    >
      {cell(
        COL.emp,
        <div className="flex items-center gap-[10px] min-w-0">
          <span className="flex h-[34px] w-[34px] shrink-0 items-center justify-center rounded-full bg-surface-2 text-[13px] font-semibold text-text-2">
            {initials(emp.full_name)}
          </span>
          <div className="flex flex-col gap-[1px] min-w-0">
            <span className="text-[14px] font-semibold text-text truncate">{emp.full_name}</span>
            <span className="text-[11px] font-mono text-text-3">{emp.id}</span>
          </div>
        </div>,
        'left',
      )}
      {balQuery.isLoading
        ? [COL.ct, COL.used, COL.pending, COL.remaining, COL.special, COL.expiry].map((w, i) =>
            cell(
              w,
              <span className="h-[12px] w-[28px] rounded bg-surface-2 animate-pulse" />,
              i === 5 ? 'left' : 'right',
            ),
          )
        : ct
          ? [
              cell(
                COL.ct,
                <span className={`${mono} font-semibold text-text`}>
                  {ct.entitled_days ?? ct.cap_value ?? 0}
                </span>,
              ),
              cell(COL.used, <span className={`${mono} text-text-2`}>{ct.used_days}</span>),
              cell(
                COL.pending,
                <span
                  className={`${mono} ${ct.pending_days > 0 ? 'text-warn-tx font-medium' : 'text-text-3'}`}
                >
                  {ct.pending_days}
                </span>,
              ),
              cell(
                COL.remaining,
                <span
                  className={`${mono} font-semibold ${(ct.remaining_days ?? 0) <= 0 ? 'text-text-3' : 'text-ok-tx'}`}
                >
                  {ct.remaining_days ?? 0}
                </span>,
              ),
              cell(
                COL.special,
                specialUsed > 0 ? (
                  <span className="text-[13px] text-text-2">{specialUsed}</span>
                ) : (
                  dash
                ),
              ),
              cell(
                COL.expiry,
                ct.expires_at ? (
                  <DateText kind="date" value={ct.expires_at} className="text-[13px] text-text-2" />
                ) : (
                  dash
                ),
                'left',
              ),
            ]
          : [
              cell(COL.ct, dash),
              cell(COL.used, dash),
              cell(COL.pending, dash),
              cell(COL.remaining, dash),
              cell(
                COL.special,
                specialUsed > 0 ? (
                  <span className="text-[13px] text-text-2">{specialUsed}</span>
                ) : (
                  dash
                ),
              ),
              cell(COL.expiry, dash, 'left'),
            ]}
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
      width: 260,
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

  const [addOpen, setAddOpen] = useState(false);
  const [adjustTarget, setAdjustTarget] = useState<AdjustTarget | null>(null);

  const listQuery = useListEmployees({
    limit: PAGE_SIZE,
    ...(search.q ? { q: search.q } : {}),
    ...(search.cursor ? { cursor: search.cursor } : {}),
  });

  type EmpList = { data: EmployeeRow[]; next_cursor?: string | null; has_more?: boolean };
  const list = unwrap<EmpList>(listQuery.data?.data);
  const employees: EmployeeRow[] = list?.data ?? [];
  const hasMore = list?.has_more ?? false;
  const nextCursor = list?.next_cursor ?? null;

  type NavFn = (o: { to: string; search?: Record<string, unknown> }) => void;
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
  const isEmpty = !isLoading && !listError && employees.length === 0;
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
  return (
    <div className="flex flex-col gap-[18px] p-[24px] bg-app-bg min-h-full w-full">
      <div className="flex items-center justify-between w-full">
        <div className="flex flex-col gap-[4px]">
          <h1 className="text-[30px] font-bold text-text leading-none">{t('title')}</h1>
          <p className="text-[14px] text-text-3">{t('subtitle')}</p>
        </div>
        <Button type="button" variant="primary" onClick={() => setAddOpen(true)}>
          <PackagePlus aria-hidden className="h-[16px] w-[16px]" />
          {t('actions.addQuota')}
        </Button>
      </div>

      <div className="flex items-center gap-[10px] w-full">
        <SearchField
          value={search.q ?? ''}
          onChange={(e) => setSearch({ q: e.target.value || undefined })}
          placeholder={t('filters.searchPlaceholder')}
        />
      </div>

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
        <div className="rounded-[12px] border border-border bg-surface overflow-hidden w-full">
          {/* Header */}
          <div className="flex items-center py-[11px] px-[18px] bg-surface-2 border-b border-border">
            {(
              [
                [COL.emp, t('table.employee'), 'left'],
                [COL.ct, t('table.quotaCt'), 'right'],
                [COL.used, t('table.used'), 'right'],
                [COL.pending, t('table.pending'), 'right'],
                [COL.remaining, t('table.remaining'), 'right'],
                [COL.special, t('table.special'), 'right'],
                [COL.expiry, t('table.ctExpiry'), 'left'],
              ] as const
            ).map(([w, label, align]) => (
              <div
                key={label}
                style={{ width: w }}
                className={`shrink-0 flex ${align === 'right' ? 'justify-end' : ''}`}
              >
                <span className="text-[11px] font-semibold tracking-[0.5px] text-text-3">
                  {label}
                </span>
              </div>
            ))}
          </div>

          {isLoading ? (
            <div className="p-[18px] text-[13px] text-text-3">{t('states.loading')}</div>
          ) : isEmpty ? (
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
          ) : (
            employees.map((emp) => (
              <EmployeeQuotaRow
                key={emp.id}
                emp={emp}
                onSelect={() =>
                  nav({
                    to: '/leave/quotas',
                    search: { ...search, employee_id: emp.id, cursor: undefined },
                  })
                }
              />
            ))
          )}
        </div>
      )}

      {!isLoading && !listError && employees.length > 0 && (
        <p className="text-[12px] text-text-3">{t('footerNote')}</p>
      )}

      {employees.length > 0 && (
        <CursorPagination
          rangeLabel={t('pagination.rangeLabel', { count: employees.length })}
          hasPrev={!!search.cursor}
          hasNext={hasMore}
          onPrev={() => nav({ to: '/leave/quotas', search: { ...search, cursor: undefined } })}
          onNext={() =>
            nav({ to: '/leave/quotas', search: { ...search, cursor: nextCursor ?? undefined } })
          }
        />
      )}

      <QuotaModal open={addOpen} onClose={() => setAddOpen(false)} onSuccess={onSuccess} />
    </div>
  );
}
