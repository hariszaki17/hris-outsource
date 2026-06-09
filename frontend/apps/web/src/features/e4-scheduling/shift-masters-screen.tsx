/**
 * E4 · Master Shift — Katalog Shift Global
 *
 * .pen frames:
 *   O5JgF  E4 · Master Shift (catalog list)
 *   Mn9ux  E4 · Tambah Shift (modal)
 *
 * Layout: Sidebar + Main (TitleBand → FilterRow → ShiftTable)
 * Modal: Tambah/Edit — Nama, Lini Layanan (opsional), Jam Mulai, Jam Selesai,
 *        Istirahat Mulai/Selesai, cross-midnight note, status toggle.
 * Confirms: Deactivate (tone danger) · Reactivate (tone info).
 *
 * F4.1 — Shift master catalog (SM-1…SM-5).
 * INV-4 — reads cross_midnight from server; do NOT compute client-side.
 * SM-5 — shift in use can only be deactivated, never hard-deleted.
 * ENGINEERING.md D1 — cursor pagination only.
 * ENGINEERING.md B2 — no dead-flow states.
 * ENGINEERING.md A2 — client RBAC is defense-in-depth.
 */

import { ServiceLinePicker } from '@/features/e2-identity/pickers/service-line-picker.tsx';
import { applyFieldErrors, classifyError } from '@/lib/api-error.ts';
import { zodResolver } from '@hookform/resolvers/zod';
import {
  type ListServiceLines200,
  type ServiceLine,
  ServiceLineStatus,
  useListServiceLines,
} from '@swp/api-client/e2';
import {
  type ListShiftMasters200,
  type ListShiftMastersParams,
  type ShiftMaster,
  ShiftMasterStatus,
  type ShiftMasterWriteRequest,
  useCreateShiftMaster,
  useDeactivateShiftMaster,
  useListShiftMasters,
  useReactivateShiftMaster,
  useUpdateShiftMaster,
} from '@swp/api-client/e4';
import {
  Button,
  type Column,
  ConfirmDialog,
  CursorPagination,
  DataTable,
  DropdownMenu,
  DropdownMenuItem,
  EmptyState,
  FilterSelect,
  FormField,
  Input,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  SearchField,
  StateView,
  StatusBadge,
  Toggle,
  useToast,
} from '@swp/ui';
import { AlarmClock, Check, Clock, Info, Moon, Plus, PowerOff, RefreshCw } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const PAGE_SIZE = 50;

/** HH:MM pattern — matches server pattern ^([01][0-9]|2[0-3]):[0-5][0-9]$ */
const TIME_PATTERN = /^([01][0-9]|2[0-3]):[0-5][0-9]$/;

// ---------------------------------------------------------------------------
// Zod schema (hand-written per task spec)
// ---------------------------------------------------------------------------

const shiftMasterSchema = z.object({
  name: z
    .string()
    .min(1, { message: 'shiftMasters.validation.nameRequired' })
    .max(60, { message: 'shiftMasters.validation.nameMaxLength' }),
  start_time: z
    .string()
    .min(1, { message: 'shiftMasters.validation.startTimeRequired' })
    .regex(TIME_PATTERN, { message: 'shiftMasters.validation.timeFormat' }),
  end_time: z
    .string()
    .min(1, { message: 'shiftMasters.validation.endTimeRequired' })
    .regex(TIME_PATTERN, { message: 'shiftMasters.validation.timeFormat' }),
  break_start: z
    .string()
    .regex(TIME_PATTERN, { message: 'shiftMasters.validation.timeFormat' })
    .optional()
    .or(z.literal('')),
  break_end: z
    .string()
    .regex(TIME_PATTERN, { message: 'shiftMasters.validation.timeFormat' })
    .optional()
    .or(z.literal('')),
  service_line_id: z.string().nullable().optional(),
  is_active: z.boolean(),
});

type ShiftMasterFormValues = z.input<typeof shiftMasterSchema>;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Detect cross-midnight locally (mirrors server rule SM-2) — display-only, not sent to server. */
function isCrossMidnight(start: string, end: string): boolean {
  if (!TIME_PATTERN.test(start) || !TIME_PATTERN.test(end)) return false;
  const [sh, sm] = start.split(':').map(Number);
  const [eh, em] = end.split(':').map(Number);
  return (sh ?? 0) * 60 + (sm ?? 0) >= (eh ?? 0) * 60 + (em ?? 0);
}

// ---------------------------------------------------------------------------
// Status → tone map (DESIGN-SYSTEM §2 — no raw hex)
// ---------------------------------------------------------------------------

function shiftStatusTone(status: string) {
  return status === ShiftMasterStatus.ACTIVE ? ('ok' as const) : ('neutral' as const);
}

// ---------------------------------------------------------------------------
// ShiftMasterModal — Tambah / Edit (frame Mn9ux)
// ---------------------------------------------------------------------------

interface ShiftMasterModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  editing: ShiftMaster | null;
  onDone: () => void;
}

function ShiftMasterModal({ open, onOpenChange, editing, onDone }: ShiftMasterModalProps) {
  const { t } = useTranslation('shiftMasters');
  const { toast } = useToast();
  const isEdit = editing !== null;

  const createMut = useCreateShiftMaster();
  const updateMut = useUpdateShiftMaster();
  const isPending = createMut.isPending || updateMut.isPending;

  const form = useForm<ShiftMasterFormValues>({
    resolver: zodResolver(shiftMasterSchema),
    defaultValues: {
      name: '',
      start_time: '',
      end_time: '',
      break_start: '',
      break_end: '',
      service_line_id: null,
      is_active: true,
    },
  });

  const { register, handleSubmit, reset, setValue, watch, setError, formState } = form;

  const watchedStart = watch('start_time');
  const watchedEnd = watch('end_time');
  const isActive = watch('is_active');
  const showCrossMidnightNote = isCrossMidnight(watchedStart ?? '', watchedEnd ?? '');

  useEffect(() => {
    if (open) {
      if (editing) {
        reset({
          name: editing.name,
          start_time: editing.start_time,
          end_time: editing.end_time,
          break_start: editing.break_start ?? '',
          break_end: editing.break_end ?? '',
          service_line_id: editing.service_line_id ?? null,
          is_active: editing.is_active,
        });
      } else {
        reset({
          name: '',
          start_time: '',
          end_time: '',
          break_start: '',
          break_end: '',
          service_line_id: null,
          is_active: true,
        });
      }
    }
  }, [open, editing, reset]);

  async function onSubmit(values: ShiftMasterFormValues) {
    const payload: ShiftMasterWriteRequest = {
      name: values.name,
      start_time: values.start_time,
      end_time: values.end_time,
      break_start: values.break_start !== '' ? values.break_start : undefined,
      break_end: values.break_end !== '' ? values.break_end : undefined,
      service_line_id: values.service_line_id ?? undefined,
      is_active: values.is_active,
    };

    try {
      if (isEdit && editing) {
        await updateMut.mutateAsync({ id: editing.id, data: payload });
        toast({ tone: 'success', title: t('updateSuccess') });
      } else {
        await createMut.mutateAsync({ data: payload });
        toast({ tone: 'success', title: t('createSuccess') });
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

  const serviceLineId = watch('service_line_id');

  return (
    <Modal open={open} onOpenChange={onOpenChange} size="lg">
      <form onSubmit={handleSubmit(onSubmit)}>
        {/* Header — mirrors .pen Head: Tambah Shift title + subtitle + × */}
        <ModalHeader
          icon={AlarmClock}
          tone="brand"
          title={isEdit ? t('modal.editTitle') : t('modal.addTitle')}
          closeLabel={t('modal.close')}
        />

        <ModalBody>
          <p className="text-[12px] text-text-3">{t('modal.subtitle')}</p>

          {/* Nama Shift */}
          <FormField
            htmlFor="sm-name"
            label={t('modal.fieldName')}
            required
            error={formState.errors.name?.message ? t(formState.errors.name.message) : undefined}
          >
            <Input
              id="sm-name"
              placeholder={t('modal.fieldNamePlaceholder')}
              {...register('name')}
              aria-invalid={!!formState.errors.name}
              aria-describedby={formState.errors.name ? 'sm-name-error' : undefined}
            />
          </FormField>

          {/* Lini Layanan (opsional) */}
          <FormField htmlFor="sm-service-line" label={t('modal.fieldServiceLine')}>
            <ServiceLinePicker
              value={serviceLineId ?? null}
              onChange={(v) => setValue('service_line_id', v)}
              placeholder={t('modal.fieldServiceLinePlaceholder')}
            />
          </FormField>

          {/* Jam Mulai + Jam Selesai — 2-col row */}
          <div className="grid grid-cols-2 gap-3">
            <FormField
              htmlFor="sm-start-time"
              label={t('modal.fieldStartTime')}
              required
              error={
                formState.errors.start_time?.message
                  ? t(formState.errors.start_time.message)
                  : undefined
              }
            >
              <Input
                id="sm-start-time"
                type="time"
                {...register('start_time')}
                aria-invalid={!!formState.errors.start_time}
              />
            </FormField>

            <FormField
              htmlFor="sm-end-time"
              label={t('modal.fieldEndTime')}
              required
              error={
                formState.errors.end_time?.message
                  ? t(formState.errors.end_time.message)
                  : undefined
              }
            >
              <Input
                id="sm-end-time"
                type="time"
                {...register('end_time')}
                aria-invalid={!!formState.errors.end_time}
              />
            </FormField>
          </div>

          {/* Istirahat Mulai + Istirahat Selesai — 2-col row */}
          <div className="grid grid-cols-2 gap-3">
            <FormField
              htmlFor="sm-break-start"
              label={t('modal.fieldBreakStart')}
              error={
                formState.errors.break_start?.message
                  ? t(formState.errors.break_start.message)
                  : undefined
              }
            >
              <Input
                id="sm-break-start"
                type="time"
                {...register('break_start')}
                aria-invalid={!!formState.errors.break_start}
              />
            </FormField>

            <FormField
              htmlFor="sm-break-end"
              label={t('modal.fieldBreakEnd')}
              error={
                formState.errors.break_end?.message
                  ? t(formState.errors.break_end.message)
                  : undefined
              }
            >
              <Input
                id="sm-break-end"
                type="time"
                {...register('break_end')}
                aria-invalid={!!formState.errors.break_end}
              />
            </FormField>
          </div>

          {/* Cross-midnight note — mirrors .pen `nRkuQ` Note block */}
          {showCrossMidnightNote && (
            <div className="flex items-start gap-2 rounded-lg border border-info-bd bg-info-bg px-3 py-[9px]">
              <Info className="mt-[1px] size-[14px] shrink-0 text-info-tx" aria-hidden="true" />
              <p className="text-[12px] leading-[1.4] text-info-tx">
                {t('modal.crossMidnightNote')}
              </p>
            </div>
          )}

          {/* Status toggle — mirrors .pen `aWara` StatusRow */}
          <div className="flex items-center justify-between">
            <div className="flex flex-col gap-[2px]">
              <span className="text-[14px] font-semibold text-text">
                {t('modal.toggleActiveLabel')}
              </span>
              <span className="text-[12px] text-text-3">{t('modal.toggleActiveHint')}</span>
            </div>
            <Toggle
              checked={isActive}
              onCheckedChange={(v) => setValue('is_active', v)}
              aria-label={t('modal.toggleActiveLabel')}
            />
          </div>
        </ModalBody>

        <ModalFooter>
          <Button
            type="button"
            variant="secondary"
            size="sm"
            onClick={() => onOpenChange(false)}
            disabled={isPending}
          >
            {t('modal.cancel')}
          </Button>
          <Button type="submit" size="sm" disabled={isPending} aria-busy={isPending}>
            <Check className="size-4" aria-hidden="true" />
            {isPending ? t('modal.saving') : t('modal.save')}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// Row actions menu
// ---------------------------------------------------------------------------

interface RowActionsMenuProps {
  shift: ShiftMaster;
  onEdit: (shift: ShiftMaster) => void;
  onDeactivate: (shift: ShiftMaster) => void;
  onReactivate: (shift: ShiftMaster) => void;
}

function RowActionsMenu({ shift, onEdit, onDeactivate, onReactivate }: RowActionsMenuProps) {
  const { t } = useTranslation('shiftMasters');
  const isActive = shift.status === ShiftMasterStatus.ACTIVE;

  return (
    <DropdownMenu triggerLabel={t('rowActions')} menuWidth={180}>
      <DropdownMenuItem icon={Clock} onSelect={() => onEdit(shift)}>
        {t('actions.edit')}
      </DropdownMenuItem>

      {isActive ? (
        <DropdownMenuItem icon={PowerOff} tone="danger" onSelect={() => onDeactivate(shift)}>
          {t('actions.deactivate')}
        </DropdownMenuItem>
      ) : (
        <DropdownMenuItem icon={RefreshCw} tone="ok" onSelect={() => onReactivate(shift)}>
          {t('actions.reactivate')}
        </DropdownMenuItem>
      )}
    </DropdownMenu>
  );
}

// ---------------------------------------------------------------------------
// ShiftMastersScreen (export — frame O5JgF)
// ---------------------------------------------------------------------------

/** Typed filter/cursor search params for `/shift-masters`. */
export type ShiftMastersSearch = {
  q?: string;
  service_line_id?: string;
  status?: ShiftMasterStatus;
  cursor?: string;
};

export function ShiftMastersScreen() {
  const { t } = useTranslation('shiftMasters');
  const { toast } = useToast();

  // ---------------------------------------------------------------------------
  // Filter / pagination state
  // ---------------------------------------------------------------------------

  const [searchQ, setSearchQ] = useState('');
  const [serviceLineFilter, setServiceLineFilter] = useState<string | undefined>(undefined);
  const [statusFilter, setStatusFilter] = useState<ShiftMasterStatus | undefined>(undefined);
  const [cursor, setCursor] = useState<string | undefined>(undefined);
  const [prevCursors, setPrevCursors] = useState<string[]>([]);

  // ---------------------------------------------------------------------------
  // Modal / confirm state
  // ---------------------------------------------------------------------------

  const [modalOpen, setModalOpen] = useState(false);
  const [editingShift, setEditingShift] = useState<ShiftMaster | null>(null);
  const [deactivateTarget, setDeactivateTarget] = useState<ShiftMaster | null>(null);
  const [reactivateTarget, setReactivateTarget] = useState<ShiftMaster | null>(null);

  // ---------------------------------------------------------------------------
  // Data hooks
  // ---------------------------------------------------------------------------

  const params: ListShiftMastersParams = {
    limit: PAGE_SIZE,
    q: searchQ || undefined,
    service_line_id: serviceLineFilter,
    status: statusFilter,
    cursor,
  };

  const query = useListShiftMasters(params);
  const deactivateMut = useDeactivateShiftMaster();
  const reactivateMut = useReactivateShiftMaster();

  // Service lines for the filter dropdown (active only). Global shifts (no line)
  // are matched server-side regardless of the selected line.
  const serviceLinesQuery = useListServiceLines(
    { limit: 50 },
    { query: { staleTime: 5 * 60_000 } },
  );
  const serviceLineOptions = (
    ((serviceLinesQuery.data?.data as ListServiceLines200 | undefined)?.data ?? []) as ServiceLine[]
  ).filter((sl) => sl.status === ServiceLineStatus.ACTIVE);

  const page = query.data?.data as ListShiftMasters200 | undefined;
  const rows: ShiftMaster[] = page?.data ?? [];
  const nextCursor = page?.next_cursor ?? undefined;
  const hasMore = page?.has_more ?? Boolean(nextCursor);
  const hasPrev = prevCursors.length > 0;

  const hasFilters = Boolean(searchQ || serviceLineFilter || statusFilter);

  // ---------------------------------------------------------------------------
  // Handlers
  // ---------------------------------------------------------------------------

  function openAdd() {
    setEditingShift(null);
    setModalOpen(true);
  }

  function openEdit(shift: ShiftMaster) {
    setEditingShift(shift);
    setModalOpen(true);
  }

  function handleDone() {
    void query.refetch();
  }

  function handleNext() {
    if (!nextCursor) return;
    setPrevCursors((prev) => [...prev, cursor ?? '']);
    setCursor(nextCursor);
  }

  function handlePrev() {
    const newPrev = [...prevCursors];
    const prevCursor = newPrev.pop();
    setPrevCursors(newPrev);
    setCursor(prevCursor === '' ? undefined : prevCursor);
  }

  // Reset pagination when filters change
  function handleSearch(q: string) {
    setSearchQ(q);
    setCursor(undefined);
    setPrevCursors([]);
  }

  function handleServiceLineFilter(v: string) {
    setServiceLineFilter(v || undefined);
    setCursor(undefined);
    setPrevCursors([]);
  }

  function handleStatusFilter(v: string) {
    setStatusFilter((v as ShiftMasterStatus) || undefined);
    setCursor(undefined);
    setPrevCursors([]);
  }

  async function handleDeactivate() {
    if (!deactivateTarget) return;
    try {
      await deactivateMut.mutateAsync({ id: deactivateTarget.id });
      toast({ tone: 'success', title: t('deactivateSuccess') });
      void query.refetch();
    } catch (err) {
      const { message } = classifyError(err);
      toast({ tone: 'error', title: t(message) });
    } finally {
      setDeactivateTarget(null);
    }
  }

  async function handleReactivate() {
    if (!reactivateTarget) return;
    try {
      await reactivateMut.mutateAsync({ id: reactivateTarget.id });
      toast({ tone: 'success', title: t('reactivateSuccess') });
      void query.refetch();
    } catch (err) {
      const { message } = classifyError(err);
      toast({ tone: 'error', title: t(message) });
    } finally {
      setReactivateTarget(null);
    }
  }

  // ---------------------------------------------------------------------------
  // Columns (mirrors .pen ShiftTable columns: h0…h5)
  // h0 = Nama Shift (w:310), h1 = Waktu (w:200), h2 = Istirahat (w:190),
  // h3 = Lini Layanan (w:210), h4 = Status (w:150), h5 = actions (w:56)
  // ---------------------------------------------------------------------------

  const columns: Column<ShiftMaster>[] = [
    {
      id: 'name',
      header: t('col.name'),
      width: 310,
      cell: (row) => (
        <div className="flex flex-col gap-[2px]">
          <span className="text-[14px] font-medium text-text">{row.name}</span>
          {row.cross_midnight && (
            <span className="flex items-center gap-[4px] text-[11px] text-info-tx">
              <Moon className="size-[11px]" aria-hidden="true" />
              {t('crossMidnight')}
            </span>
          )}
        </div>
      ),
    },
    {
      id: 'time',
      header: t('col.time'),
      width: 200,
      cell: (row) => (
        <span className="text-[13px] text-text-2">
          {row.start_time} – {row.end_time}
        </span>
      ),
    },
    {
      id: 'break',
      header: t('col.break'),
      width: 190,
      cell: (row) =>
        row.break_start && row.break_end ? (
          <span className="text-[13px] text-text-2">
            {row.break_start} – {row.break_end}
            {row.break_minutes != null && (
              <span className="ml-1 text-text-3">({row.break_minutes} mnt)</span>
            )}
          </span>
        ) : (
          <span className="text-text-3">—</span>
        ),
    },
    {
      id: 'service_line',
      header: t('col.serviceLine'),
      width: 210,
      cell: (row) =>
        row.service_line_name ? (
          <span className="inline-flex rounded-full bg-info-bg px-2 py-0.5 text-[11px] font-medium text-info-tx">
            {row.service_line_name}
          </span>
        ) : (
          <span className="text-[12px] text-text-3">{t('allLines')}</span>
        ),
    },
    {
      id: 'status',
      header: t('col.status'),
      width: 150,
      cell: (row) => (
        <div className="flex flex-col gap-1">
          <StatusBadge dot tone={shiftStatusTone(row.status)}>
            {row.status === ShiftMasterStatus.ACTIVE ? t('statusActive') : t('statusInactive')}
          </StatusBadge>
          {(row.in_use_count ?? 0) > 0 && (
            <span className="text-[11px] text-text-3">
              {t('inUseCount', { count: row.in_use_count })}
            </span>
          )}
        </div>
      ),
    },
    {
      id: 'actions',
      header: '',
      width: 56,
      align: 'right',
      cell: (row) => (
        <RowActionsMenu
          shift={row}
          onEdit={openEdit}
          onDeactivate={(s) => setDeactivateTarget(s)}
          onReactivate={(s) => setReactivateTarget(s)}
        />
      ),
    },
  ];

  // ---------------------------------------------------------------------------
  // Error state — classify and render the right empty / state-view
  // ---------------------------------------------------------------------------

  if (query.isError) {
    const { kind } = classifyError(query.error);
    if (kind === 'forbidden' || kind === 'unauthenticated') {
      return (
        <div className="flex flex-col gap-4">
          <EmptyState
            variant="no-permission"
            title={t('noPermissionTitle')}
            description={t('noPermissionBody')}
          />
        </div>
      );
    }
    return (
      <div className="flex flex-col gap-4">
        <StateView
          kind="error"
          title={t('errorTitle')}
          description={t('errorBody')}
          onRetry={() => query.refetch()}
          retryLabel={t('retry')}
        />
      </div>
    );
  }

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <div className="flex flex-col gap-[18px]">
      {/* TitleBand — mirrors .pen MbRcF */}
      <div className="flex items-start justify-between">
        <div className="flex flex-col gap-1">
          <h1 className="text-[30px] font-bold leading-tight text-text">{t('title')}</h1>
          <p className="text-[14px] text-text-3">{t('subtitle')}</p>
        </div>
        <Button type="button" onClick={openAdd}>
          <Plus className="size-4" aria-hidden="true" />
          {t('addButton')}
        </Button>
      </div>

      {/* Filter row — mirrors .pen xG94u Filters: Search + Lini Layanan + Status */}
      <div className="flex items-center gap-[10px]">
        <SearchField
          placeholder={t('searchPlaceholder')}
          value={searchQ}
          containerClassName="w-[280px]"
          onChange={(e) => handleSearch(e.target.value)}
        />

        {/* Service line filter — active lines from the master list (global shifts
            with no line are matched server-side regardless of selection). */}
        <FilterSelect
          aria-label={t('filterServiceLine')}
          value={serviceLineFilter ?? ''}
          onChange={(e) => handleServiceLineFilter(e.target.value)}
          containerClassName="w-[200px]"
        >
          <option value="">{t('filterAllServiceLines')}</option>
          {serviceLineOptions.map((sl) => (
            <option key={sl.id} value={sl.id}>
              {sl.name}
            </option>
          ))}
        </FilterSelect>

        <FilterSelect
          aria-label={t('filterStatus')}
          value={statusFilter ?? ''}
          onChange={(e) => handleStatusFilter(e.target.value)}
        >
          <option value="">{t('filterAllStatus')}</option>
          <option value={ShiftMasterStatus.ACTIVE}>{t('statusActive')}</option>
          <option value={ShiftMasterStatus.INACTIVE}>{t('statusInactive')}</option>
        </FilterSelect>
      </div>

      {/* ShiftTable card — mirrors .pen I2djIQ ShiftTable */}
      <div className="overflow-hidden rounded-xl border border-border bg-surface">
        <DataTable
          aria-label={t('title')}
          columns={columns}
          data={rows}
          getRowId={(r) => r.id}
          isLoading={query.isLoading}
          skeletonRows={7}
          empty={
            hasFilters ? (
              <EmptyState
                variant="filtered"
                title={t('filteredTitle')}
                description={t('filteredBody')}
              />
            ) : (
              <EmptyState variant="fresh" title={t('emptyTitle')} description={t('emptyBody')} />
            )
          }
          footer={
            (hasPrev || hasMore) && (
              <CursorPagination
                rangeLabel={t('rangeLabel', { count: rows.length })}
                hasPrev={hasPrev}
                hasNext={hasMore}
                onPrev={handlePrev}
                onNext={handleNext}
                prevLabel={t('prev')}
                nextLabel={t('next')}
              />
            )
          }
        />
      </div>

      {/* Tambah / Edit modal — frame Mn9ux */}
      <ShiftMasterModal
        open={modalOpen}
        onOpenChange={(o) => {
          if (!o) setModalOpen(false);
        }}
        editing={editingShift}
        onDone={handleDone}
      />

      {/* Deactivate confirm (SM-5 — in-use shifts can be deactivated, not deleted) */}
      <ConfirmDialog
        open={deactivateTarget !== null}
        onOpenChange={(o) => {
          if (!o) setDeactivateTarget(null);
        }}
        icon={PowerOff}
        tone="danger"
        title={t('confirm.deactivateTitle')}
        description={
          deactivateTarget
            ? t('confirm.deactivateBody', {
                name: deactivateTarget.name,
                inUseCount: deactivateTarget.in_use_count ?? 0,
              })
            : undefined
        }
        confirmLabel={t('confirm.deactivateConfirm')}
        cancelLabel={t('confirm.cancel')}
        confirmTone="danger"
        loading={deactivateMut.isPending}
        onConfirm={handleDeactivate}
      >
        {(deactivateTarget?.in_use_count ?? 0) > 0 && (
          <div className="flex items-start gap-2 rounded-lg border border-warn-bd bg-warn-bg px-3 py-[9px]">
            <Info className="mt-[1px] size-[14px] shrink-0 text-warn-tx" aria-hidden="true" />
            <p className="text-[12px] leading-[1.4] text-warn-tx">
              {t('confirm.inUseWarning', { count: deactivateTarget?.in_use_count })}
            </p>
          </div>
        )}
      </ConfirmDialog>

      {/* Reactivate confirm */}
      <ConfirmDialog
        open={reactivateTarget !== null}
        onOpenChange={(o) => {
          if (!o) setReactivateTarget(null);
        }}
        icon={RefreshCw}
        tone="info"
        title={t('confirm.reactivateTitle')}
        description={
          reactivateTarget
            ? t('confirm.reactivateBody', { name: reactivateTarget.name })
            : undefined
        }
        confirmLabel={t('confirm.reactivateConfirm')}
        cancelLabel={t('confirm.cancel')}
        loading={reactivateMut.isPending}
        onConfirm={handleReactivate}
      />
    </div>
  );
}
