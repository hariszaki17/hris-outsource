import { applyFieldErrors, classifyError } from '@/lib/api-error.ts';
import { zodResolver } from '@hookform/resolvers/zod';
import {
  type ListOvertimeRules200,
  type ListOvertimeRulesParams,
  type OvertimeRule,
  type OvertimeRuleId,
  OvertimeRuleStatus,
  type OvertimeRuleWriteRequest,
  useCreateOvertimeRule,
  useListOvertimeRules,
  useSoftDeleteOvertimeRule,
  useUpdateOvertimeRule,
} from '@swp/api-client/e2';
import {
  Button,
  type Column,
  ConfirmDialog,
  DataTable,
  EmptyState,
  FilterSelect,
  FormField,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  StateView,
  StatusBadge,
  Toggle,
  useToast,
} from '@swp/ui';
import { Link } from '@tanstack/react-router';
import { ArrowLeft, Check, Info, MoreVertical, Plus, Timer, Trash2 } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';

/**
 * E2 · Aturan Lembur — list + Tambah/Edit modal + soft-delete confirm.
 * Built from .pen frames `SnXpE` (list), `JYmgi` (modal).
 * D4 (2026-06-02): min_minutes default = 30, minimum = 30 (EPICS §8).
 * OR-1: create, OR-2: service_line_id scoping (null = global), OR-3: rates.
 * Routes: /master-data/overtime-rules — consumers must register in the router.
 * Refs: F7.1 (E7 OT calc consumes this master).
 */

const PAGE_SIZE = 200;

// ---------------------------------------------------------------------------
// Zod schema (hand-written — E2 Zod deferred)
// ---------------------------------------------------------------------------

// RHF stores type="number" input values as numbers (via valueAsNumber), so the schema
// must use z.coerce.number() to accept both the initial string defaultValues and the
// numbers that RHF gives to zodResolver on submit.
const overtimeRuleSchema = z.object({
  name: z.string().min(1, 'Nama wajib diisi').max(100),
  service_line_id: z.string().optional(),
  weekday_rate: z.coerce.number().positive('Harus > 0'),
  restday_rate: z.coerce.number().positive('Harus > 0'),
  holiday_rate: z.coerce.number().positive('Harus > 0'),
  min_minutes: z.coerce.number().int().min(30, 'Minimal 30 menit (D4)'),
  // Optional number: empty string / undefined → undefined; otherwise coerce to positive int.
  max_minutes_per_day: z.preprocess(
    (v) => (v === '' || v === undefined || v === null ? undefined : Number(v)),
    z.number().int().positive().optional(),
  ),
  pre_approval_required: z.boolean(),
});

// Explicit form shape — use number for fields that RHF stores as numbers (type="number" inputs).
type OvertimeRuleFormValues = {
  name: string;
  service_line_id: string;
  weekday_rate: number | string;
  restday_rate: number | string;
  holiday_rate: number | string;
  min_minutes: number | string;
  max_minutes_per_day: number | string | undefined;
  pre_approval_required: boolean;
};

// ---------------------------------------------------------------------------
// OvertimeRuleModal — Tambah/Edit
// ---------------------------------------------------------------------------

interface OvertimeRuleModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  editing: OvertimeRule | null;
  onDone: () => void;
}

function OvertimeRuleModal({ open, onOpenChange, editing, onDone }: OvertimeRuleModalProps) {
  const { t } = useTranslation();
  const { toast } = useToast();
  const isEdit = editing !== null;

  const createMut = useCreateOvertimeRule();
  const updateMut = useUpdateOvertimeRule();
  const isPending = createMut.isPending || updateMut.isPending;

  const form = useForm<OvertimeRuleFormValues>({
    resolver: zodResolver(overtimeRuleSchema),
    defaultValues: {
      name: '',
      service_line_id: '',
      weekday_rate: 1.5,
      restday_rate: 2.0,
      holiday_rate: 3.0,
      min_minutes: 30,
      max_minutes_per_day: '',
      pre_approval_required: false,
    },
  });

  const { register, handleSubmit, reset, setValue, watch, setError, formState } = form;
  const preApproval = watch('pre_approval_required');

  useEffect(() => {
    if (open) {
      if (editing) {
        reset({
          name: editing.name,
          service_line_id: editing.service_line_id ?? '',
          weekday_rate: editing.weekday_rate,
          restday_rate: editing.restday_rate,
          holiday_rate: editing.holiday_rate,
          min_minutes: editing.min_minutes,
          max_minutes_per_day: editing.max_minutes_per_day ?? '',
          pre_approval_required: editing.pre_approval_required,
        });
      } else {
        reset({
          name: '',
          service_line_id: '',
          weekday_rate: 1.5,
          restday_rate: 2.0,
          holiday_rate: 3.0,
          min_minutes: 30,
          max_minutes_per_day: '',
          pre_approval_required: false,
        });
      }
    }
  }, [open, editing, reset]);

  async function onSubmit(values: OvertimeRuleFormValues) {
    // zodResolver already validated + coerced — values are safe here; parse again to get
    // the output type with numbers guaranteed by z.coerce.number().
    const parsed = overtimeRuleSchema.parse(values);
    const payload: OvertimeRuleWriteRequest = {
      name: parsed.name,
      service_line_id: parsed.service_line_id || null,
      weekday_rate: parsed.weekday_rate as number,
      restday_rate: parsed.restday_rate as number,
      holiday_rate: parsed.holiday_rate as number,
      min_minutes: parsed.min_minutes as number,
      max_minutes_per_day: parsed.max_minutes_per_day,
      pre_approval_required: parsed.pre_approval_required,
    };

    try {
      if (isEdit && editing) {
        await updateMut.mutateAsync({
          overtimeRuleId: editing.id as OvertimeRuleId,
          data: payload,
        });
        toast({ tone: 'success', title: t('masterData.overtimeRules.updateSuccess') });
      } else {
        await createMut.mutateAsync({ data: payload });
        toast({ tone: 'success', title: t('masterData.overtimeRules.createSuccess') });
      }
      onDone();
      onOpenChange(false);
    } catch (err) {
      if (!applyFieldErrors(err, setError)) {
        const { message } = classifyError(err);
        toast({ tone: 'error', title: t(message) });
      }
    }
  }

  return (
    <Modal open={open} onOpenChange={onOpenChange} size="lg">
      <form onSubmit={handleSubmit(onSubmit)} noValidate>
        <ModalHeader
          icon={Timer}
          tone="warn"
          title={
            isEdit
              ? t('masterData.overtimeRules.editTitle')
              : t('masterData.overtimeRules.addTitle')
          }
        />
        <ModalBody>
          <div className="flex flex-col gap-[14px]">
            <p className="text-[12px] text-text-2">{t('masterData.overtimeRules.modalSubtitle')}</p>

            {/* Nama Aturan */}
            <FormField
              htmlFor="or-name"
              label={t('masterData.overtimeRules.fieldName')}
              required
              error={formState.errors.name?.message}
            >
              <input
                id="or-name"
                {...register('name')}
                className="h-9 w-full rounded-md border border-border bg-surface px-3 text-[14px] text-text outline-none placeholder:text-text-3 focus:border-primary focus:ring-1 focus:ring-primary"
                placeholder={t('masterData.overtimeRules.fieldNamePlaceholder')}
              />
            </FormField>

            {/* Lini Layanan (optional — null = global) */}
            <FormField
              htmlFor="or-sl"
              label={t('masterData.overtimeRules.fieldServiceLine')}
              error={formState.errors.service_line_id?.message}
            >
              <input
                id="or-sl"
                {...register('service_line_id')}
                className="h-9 w-full rounded-md border border-border bg-surface px-3 text-[14px] text-text outline-none placeholder:text-text-3 focus:border-primary focus:ring-1 focus:ring-primary"
                placeholder={t('masterData.overtimeRules.fieldServiceLinePlaceholder')}
              />
            </FormField>

            {/* Multiplier rates */}
            <div className="flex flex-col gap-[10px] rounded-lg bg-surface-2 px-[14px] py-3">
              <p className="text-[13px] font-semibold text-text">
                {t('masterData.overtimeRules.ratesLabel')}
              </p>
              <div className="flex gap-3">
                <FormField
                  htmlFor="or-weekday"
                  label={t('masterData.overtimeRules.rateWeekday')}
                  required
                  error={formState.errors.weekday_rate?.message}
                  className="flex-1"
                >
                  <input
                    id="or-weekday"
                    {...register('weekday_rate')}
                    type="number"
                    step="0.1"
                    min="0.1"
                    className="h-9 w-full rounded-md border border-border bg-surface px-3 font-mono text-[14px] text-text outline-none placeholder:text-text-3 focus:border-primary focus:ring-1 focus:ring-primary"
                    placeholder="1.5"
                  />
                </FormField>
                <FormField
                  htmlFor="or-restday"
                  label={t('masterData.overtimeRules.rateRestday')}
                  required
                  error={formState.errors.restday_rate?.message}
                  className="flex-1"
                >
                  <input
                    id="or-restday"
                    {...register('restday_rate')}
                    type="number"
                    step="0.1"
                    min="0.1"
                    className="h-9 w-full rounded-md border border-border bg-surface px-3 font-mono text-[14px] text-text outline-none placeholder:text-text-3 focus:border-primary focus:ring-1 focus:ring-primary"
                    placeholder="2.0"
                  />
                </FormField>
                <FormField
                  htmlFor="or-holiday"
                  label={t('masterData.overtimeRules.rateHoliday')}
                  required
                  error={formState.errors.holiday_rate?.message}
                  className="flex-1"
                >
                  <input
                    id="or-holiday"
                    {...register('holiday_rate')}
                    type="number"
                    step="0.1"
                    min="0.1"
                    className="h-9 w-full rounded-md border border-border bg-surface px-3 font-mono text-[14px] text-text outline-none placeholder:text-text-3 focus:border-primary focus:ring-1 focus:ring-primary"
                    placeholder="3.0"
                  />
                </FormField>
              </div>
            </div>

            {/* Min / Max minutes */}
            <div className="flex gap-3">
              <FormField
                htmlFor="or-min"
                label={t('masterData.overtimeRules.fieldMinMinutes')}
                required
                error={formState.errors.min_minutes?.message}
                className="flex-1"
              >
                <input
                  id="or-min"
                  {...register('min_minutes')}
                  type="number"
                  min={30}
                  className="h-9 w-full rounded-md border border-border bg-surface px-3 font-mono text-[14px] text-text outline-none placeholder:text-text-3 focus:border-primary focus:ring-1 focus:ring-primary"
                  placeholder="30"
                />
              </FormField>
              <FormField
                htmlFor="or-max"
                label={t('masterData.overtimeRules.fieldMaxMinutes')}
                error={formState.errors.max_minutes_per_day?.message}
                className="flex-1"
              >
                <input
                  id="or-max"
                  {...register('max_minutes_per_day')}
                  type="number"
                  min={1}
                  className="h-9 w-full rounded-md border border-border bg-surface px-3 font-mono text-[14px] text-text outline-none placeholder:text-text-3 focus:border-primary focus:ring-1 focus:ring-primary"
                  placeholder="240"
                />
              </FormField>
            </div>

            {/* Pre-approval toggle */}
            <div className="flex items-center justify-between rounded-lg bg-surface-2 px-3 py-[10px]">
              <div className="flex flex-col gap-[1px]">
                <p className="text-[13px] font-medium text-text">
                  {t('masterData.overtimeRules.togglePreApproval')}
                </p>
                <p className="text-[11px] text-text-3">
                  {t('masterData.overtimeRules.togglePreApprovalHint')}
                </p>
              </div>
              <Toggle
                checked={preApproval}
                onCheckedChange={(v) => setValue('pre_approval_required', v)}
                aria-label={t('masterData.overtimeRules.togglePreApproval')}
              />
            </div>

            {/* D4 hint */}
            <div className="flex items-start gap-2 rounded-lg border border-info-bd bg-info-bg px-3 py-[10px]">
              <Info className="mt-px size-[14px] shrink-0 text-info-tx" aria-hidden />
              <p className="text-[11px] text-info-tx">{t('masterData.overtimeRules.d4Hint')}</p>
            </div>
          </div>
        </ModalBody>
        <ModalFooter>
          <p className="mr-auto text-[11px] text-text-3">
            {t('masterData.overtimeRules.auditHint')}
          </p>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => onOpenChange(false)}
            disabled={isPending}
          >
            {t('common.cancel')}
          </Button>
          <Button type="submit" size="sm" disabled={isPending}>
            <Check aria-hidden />
            {isPending ? t('common.loading') : t('common.save')}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// OvertimeRulesScreen
// ---------------------------------------------------------------------------

export function OvertimeRulesScreen() {
  const { t } = useTranslation();

  const [statusFilter, setStatusFilter] = useState<OvertimeRuleStatus | undefined>(undefined);
  const [modalOpen, setModalOpen] = useState(false);
  const [editingItem, setEditingItem] = useState<OvertimeRule | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<OvertimeRule | null>(null);

  const { toast } = useToast();

  const params: ListOvertimeRulesParams = {
    limit: PAGE_SIZE,
    status: statusFilter,
  };

  const query = useListOvertimeRules(params);
  const deleteMut = useSoftDeleteOvertimeRule();

  const hasFilters = Boolean(statusFilter);

  function openAdd() {
    setEditingItem(null);
    setModalOpen(true);
  }

  function openEdit(item: OvertimeRule) {
    setEditingItem(item);
    setModalOpen(true);
  }

  function handleDone() {
    void query.refetch();
  }

  async function handleDelete() {
    if (!deleteTarget) return;
    try {
      await deleteMut.mutateAsync({ overtimeRuleId: deleteTarget.id as OvertimeRuleId });
      toast({ tone: 'success', title: t('masterData.overtimeRules.deleteSuccess') });
      void query.refetch();
    } catch (err) {
      const { message } = classifyError(err);
      toast({ tone: 'error', title: t(message) });
    } finally {
      setDeleteTarget(null);
    }
  }

  const columns: Column<OvertimeRule>[] = [
    {
      id: 'name',
      header: t('masterData.overtimeRules.colName'),
      width: 260,
      cell: (row) => (
        <div className="flex items-center gap-3">
          <div className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-warn-bg">
            <Timer className="size-4 text-warn-tx" aria-hidden />
          </div>
          <div className="flex flex-col gap-[2px]">
            <span className="text-[14px] font-medium text-text">{row.name}</span>
            {row.service_line_id && (
              <span className="text-[11px] text-text-2">{row.service_line_id}</span>
            )}
          </div>
        </div>
      ),
    },
    {
      id: 'service_line',
      header: t('masterData.overtimeRules.colServiceLine'),
      width: 160,
      cell: (row) => (
        <span className="text-[13px] text-text">
          {row.service_line_id ?? t('masterData.overtimeRules.global')}
        </span>
      ),
    },
    {
      id: 'weekday_rate',
      header: t('masterData.overtimeRules.colWeekday'),
      width: 110,
      cell: (row) => (
        <span className="font-mono text-[13px] font-medium text-text">{row.weekday_rate}×</span>
      ),
    },
    {
      id: 'restday_rate',
      header: t('masterData.overtimeRules.colRestday'),
      width: 110,
      cell: (row) => (
        <span className="font-mono text-[13px] font-medium text-text">{row.restday_rate}×</span>
      ),
    },
    {
      id: 'holiday_rate',
      header: t('masterData.overtimeRules.colHoliday'),
      width: 110,
      cell: (row) => (
        <span className="font-mono text-[13px] font-medium text-text">{row.holiday_rate}×</span>
      ),
    },
    {
      id: 'min_minutes',
      header: t('masterData.overtimeRules.colMinMinutes'),
      width: 110,
      cell: (row) => (
        <span className="font-mono text-[13px] font-medium text-text">
          {row.min_minutes} {t('masterData.overtimeRules.minutes')}
        </span>
      ),
    },
    {
      id: 'max_minutes_per_day',
      header: t('masterData.overtimeRules.colMaxMinutes'),
      width: 140,
      cell: (row) =>
        row.max_minutes_per_day != null ? (
          <span className="font-mono text-[13px] font-medium text-text">
            {row.max_minutes_per_day} {t('masterData.overtimeRules.minutes')}
          </span>
        ) : (
          <span className="text-text-3">—</span>
        ),
    },
    {
      id: 'status',
      header: t('masterData.overtimeRules.colStatus'),
      width: 110,
      cell: (row) => (
        <StatusBadge dot tone={row.status === OvertimeRuleStatus.ACTIVE ? 'ok' : 'bad'}>
          {row.status === OvertimeRuleStatus.ACTIVE
            ? t('masterData.statusActive')
            : t('masterData.statusInactive')}
        </StatusBadge>
      ),
    },
  ];

  if (query.isError) {
    const { kind } = classifyError(query.error);
    return (
      <div className="flex flex-col gap-4">
        {kind === 'forbidden' || kind === 'unauthenticated' ? (
          <EmptyState
            variant="no-permission"
            title={t('errors.forbidden')}
            description={t('masterData.noPermission')}
          />
        ) : (
          <StateView
            kind="error"
            title={t('masterData.overtimeRules.errorTitle')}
            description={t('errors.network')}
            onRetry={() => query.refetch()}
            retryLabel={t('common.retry')}
          />
        )}
      </div>
    );
  }

  const page = query.data?.data as ListOvertimeRules200 | undefined;
  const rows = (page?.data ?? []) as OvertimeRule[];

  return (
    <div className="flex flex-col gap-4">
      {/* Back link — /master-data route registered when router is wired (RETURN §1) */}
      <Link
        to="/master-data"
        className="flex w-fit items-center gap-[7px] text-[13px] font-medium text-text-2 hover:text-text"
      >
        <ArrowLeft className="size-4" aria-hidden />
        {t('masterData.backToHub')}
      </Link>

      {/* Title band */}
      <div className="flex items-center justify-between rounded-xl border border-border bg-surface px-5 py-[18px]">
        <div className="flex flex-col gap-1">
          <h1 className="text-[20px] font-semibold text-text">
            {t('masterData.overtimeRules.title')}
          </h1>
          <p className="text-[13px] text-text-2">{t('masterData.overtimeRules.subtitle')}</p>
        </div>
        <Button type="button" onClick={openAdd}>
          <Plus aria-hidden />
          {t('masterData.overtimeRules.add')}
        </Button>
      </div>

      {/* Shared-with-E7 note */}
      <div className="flex items-center gap-[9px] rounded-lg border border-info-bd bg-info-bg px-[14px] py-[10px]">
        <Info className="size-[15px] shrink-0 text-info-tx" aria-hidden />
        <p className="text-[12px] text-info-tx">{t('masterData.overtimeRules.sharedNote')}</p>
      </div>

      {/* Table card */}
      <div className="overflow-hidden rounded-xl border border-border bg-surface">
        <div className="border-b border-border-soft px-[18px] py-[14px]">
          <FilterSelect
            aria-label={t('masterData.filterStatus')}
            value={statusFilter ?? ''}
            onChange={(e) => setStatusFilter((e.target.value as OvertimeRuleStatus) || undefined)}
          >
            <option value="">{t('masterData.filterAllStatus')}</option>
            <option value={OvertimeRuleStatus.ACTIVE}>{t('masterData.statusActive')}</option>
            <option value={OvertimeRuleStatus.INACTIVE}>{t('masterData.statusInactive')}</option>
          </FilterSelect>
        </div>

        <DataTable
          aria-label={t('masterData.overtimeRules.title')}
          columns={columns}
          data={rows}
          getRowId={(r) => r.id}
          isLoading={query.isLoading}
          skeletonRows={6}
          empty={
            hasFilters ? (
              <EmptyState
                variant="filtered"
                title={t('masterData.filteredTitle')}
                description={t('masterData.filteredBody')}
              />
            ) : (
              <EmptyState
                variant="fresh"
                title={t('masterData.overtimeRules.emptyTitle')}
                description={t('masterData.overtimeRules.emptyBody')}
              />
            )
          }
          rowActions={(row) => (
            <button
              type="button"
              aria-label={t('masterData.rowActions')}
              aria-haspopup="menu"
              className="flex size-[30px] items-center justify-center rounded-md text-text-2 hover:bg-surface-2"
              onClick={() => openEdit(row)}
            >
              <MoreVertical className="size-4" aria-hidden />
            </button>
          )}
        />
      </div>

      {/* Modal */}
      <OvertimeRuleModal
        open={modalOpen}
        onOpenChange={(open) => {
          if (!open) setModalOpen(false);
        }}
        editing={editingItem}
        onDone={handleDone}
      />

      {/* Soft-delete confirm */}
      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null);
        }}
        icon={Trash2}
        tone="danger"
        title={t('masterData.deleteTitle')}
        description={t('masterData.overtimeRules.deleteBody', {
          name: deleteTarget?.name ?? '',
        })}
        confirmLabel={t('masterData.deleteConfirm')}
        cancelLabel={t('common.cancel')}
        loading={deleteMut.isPending}
        onConfirm={handleDelete}
      />
    </div>
  );
}
