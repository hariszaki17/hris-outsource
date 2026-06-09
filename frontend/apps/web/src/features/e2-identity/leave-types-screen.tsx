import { applyFieldErrors, classifyError } from '@/lib/api-error.ts';
import { zodResolver } from '@hookform/resolvers/zod';
import {
  type LeaveType,
  type LeaveTypeId,
  LeaveTypeStatus,
  type LeaveTypeWriteRequest,
  type ListLeaveTypes200,
  type ListLeaveTypesParams,
  useCreateLeaveType,
  useListLeaveTypes,
  useSoftDeleteLeaveType,
  useUpdateLeaveType,
} from '@swp/api-client/e2';
import {
  Button,
  type Column,
  ConfirmDialog,
  DataTable,
  DropdownMenu,
  DropdownMenuItem,
  EmptyState,
  FilterSelect,
  FormField,
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
import { Link } from '@tanstack/react-router';
import { ArrowLeft, CalendarOff, Check, Minus, Pencil, Plus, Trash2 } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';

/**
 * E2 · Jenis Cuti — list + Tambah/Edit modal + soft-delete confirm.
 * Built from .pen frames `HII8C` (list), `rMNJT` (modal).
 * Refs: LT-1 (create), LT-2 (is_annual), LT-3 (requires_document), MD-1 (soft-delete only), MD-2 (unique name/code).
 * Note: ListLeaveTypesParams has no q/cursor — list is small; fetched in full, filtered client-side.
 * Routes: /master-data/leave-types — consumers must register this route in the router.
 */

const PAGE_SIZE = 200;

// ---------------------------------------------------------------------------
// Zod schema (hand-written — E2 Zod deferred per task spec)
// ---------------------------------------------------------------------------

// RHF stores type="number" input values as numbers (via valueAsNumber), so schemas for
// optional number fields must use z.preprocess to handle both string (from reset/defaultValues)
// and number (from RHF's internal state on submit).
const leaveTypeSchema = z.object({
  name: z.string().min(1, 'Nama wajib diisi').max(100),
  code: z
    .string()
    .min(1, 'Kode wajib diisi')
    .max(30)
    .regex(/^[A-Z0-9_]+$/, 'Kode harus huruf kapital/angka/underscore'),
  // Optional quota: empty string / undefined / null → undefined; otherwise coerce to number.
  default_annual_quota: z.preprocess(
    (v) => (v === '' || v === undefined || v === null ? undefined : Number(v)),
    z.number().min(0).optional(),
  ),
  is_annual: z.boolean(),
  requires_document: z.boolean(),
  color: z.string().optional(),
});

type LeaveTypeFormValues = {
  name: string;
  code: string;
  default_annual_quota: number | string | undefined;
  is_annual: boolean;
  requires_document: boolean;
  color: string | undefined;
};

// Predefined color palette (calendar colors)
const COLOR_SWATCHES = [
  '#3B82F6',
  '#EF4444',
  '#10B981',
  '#F59E0B',
  '#8B5CF6',
  '#F97316',
  '#06B6D4',
  '#EC4899',
];

// ---------------------------------------------------------------------------
// LeaveTypeModal — Tambah/Edit
// ---------------------------------------------------------------------------

interface LeaveTypeModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  editing: LeaveType | null;
  onDone: () => void;
}

function LeaveTypeModal({ open, onOpenChange, editing, onDone }: LeaveTypeModalProps) {
  const { t } = useTranslation();
  const { toast } = useToast();
  const isEdit = editing !== null;

  const createMut = useCreateLeaveType();
  const updateMut = useUpdateLeaveType();
  const isPending = createMut.isPending || updateMut.isPending;

  const form = useForm<LeaveTypeFormValues>({
    resolver: zodResolver(leaveTypeSchema),
    defaultValues: {
      name: '',
      code: '',
      default_annual_quota: '',
      is_annual: false,
      requires_document: false,
      color: COLOR_SWATCHES[0],
    },
  });

  const { register, handleSubmit, reset, setValue, watch, setError, formState } = form;
  const selectedColor = watch('color');

  useEffect(() => {
    if (open) {
      if (editing) {
        reset({
          name: editing.name,
          code: editing.code,
          // Store as number if defined (RHF will store it as number for type="number" inputs)
          default_annual_quota: editing.default_annual_quota ?? '',
          is_annual: editing.is_annual,
          requires_document: editing.requires_document,
          color: editing.color ?? COLOR_SWATCHES[0],
        });
      } else {
        reset({
          name: '',
          code: '',
          default_annual_quota: '',
          is_annual: false,
          requires_document: false,
          color: COLOR_SWATCHES[0],
        });
      }
    }
  }, [open, editing, reset]);

  async function onSubmit(values: LeaveTypeFormValues) {
    const parsed = leaveTypeSchema.parse(values);
    const payload: LeaveTypeWriteRequest = {
      name: parsed.name,
      code: parsed.code,
      default_annual_quota: parsed.default_annual_quota,
      is_annual: parsed.is_annual,
      requires_document: parsed.requires_document,
      color: parsed.color,
    };

    try {
      if (isEdit && editing) {
        await updateMut.mutateAsync({ leaveTypeId: editing.id as LeaveTypeId, data: payload });
        toast({ tone: 'success', title: t('masterData.leaveTypes.updateSuccess') });
      } else {
        await createMut.mutateAsync({ data: payload });
        toast({ tone: 'success', title: t('masterData.leaveTypes.createSuccess') });
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

  const isAnnual = watch('is_annual');
  const requiresDocument = watch('requires_document');

  return (
    <Modal open={open} onOpenChange={onOpenChange} size="lg">
      <form onSubmit={handleSubmit(onSubmit)} noValidate>
        <ModalHeader
          icon={CalendarOff}
          tone="info"
          title={
            isEdit ? t('masterData.leaveTypes.editTitle') : t('masterData.leaveTypes.addTitle')
          }
        />
        <ModalBody>
          <div className="flex flex-col gap-[14px]">
            <p className="text-[12px] text-text-2">{t('masterData.leaveTypes.modalSubtitle')}</p>

            {/* Nama */}
            <FormField
              htmlFor="lt-name"
              label={t('masterData.leaveTypes.fieldName')}
              required
              error={formState.errors.name?.message}
            >
              <input
                id="lt-name"
                {...register('name')}
                className="h-9 w-full rounded-md border border-border bg-surface px-3 text-[14px] text-text outline-none placeholder:text-text-3 focus:border-primary focus:ring-1 focus:ring-primary"
                placeholder={t('masterData.leaveTypes.fieldNamePlaceholder')}
              />
            </FormField>

            {/* Kode */}
            <FormField
              htmlFor="lt-code"
              label={t('masterData.leaveTypes.fieldCode')}
              required
              error={formState.errors.code?.message}
            >
              <input
                id="lt-code"
                {...register('code')}
                className="h-9 w-full rounded-md border border-border bg-surface px-3 font-mono text-[14px] text-text outline-none placeholder:text-text-3 focus:border-primary focus:ring-1 focus:ring-primary"
                placeholder={t('masterData.leaveTypes.fieldCodePlaceholder')}
              />
            </FormField>

            {/* Kuota tahunan */}
            <FormField
              htmlFor="lt-quota"
              label={t('masterData.leaveTypes.fieldQuota')}
              error={formState.errors.default_annual_quota?.message}
            >
              <input
                id="lt-quota"
                {...register('default_annual_quota')}
                type="number"
                min={0}
                className="h-9 w-full rounded-md border border-border bg-surface px-3 text-[14px] text-text outline-none placeholder:text-text-3 focus:border-primary focus:ring-1 focus:ring-primary"
                placeholder={t('masterData.leaveTypes.fieldQuotaPlaceholder')}
              />
            </FormField>

            {/* Toggle group */}
            <div className="flex flex-col gap-[10px] rounded-lg bg-surface-2 px-3 py-[10px]">
              <div className="flex items-center justify-between">
                <div className="flex flex-col gap-[1px]">
                  <p className="text-[13px] font-medium text-text">
                    {t('masterData.leaveTypes.toggleAnnual')}
                  </p>
                  <p className="text-[11px] text-text-3">
                    {t('masterData.leaveTypes.toggleAnnualHint')}
                  </p>
                </div>
                <Toggle
                  checked={isAnnual}
                  onCheckedChange={(v) => setValue('is_annual', v)}
                  aria-label={t('masterData.leaveTypes.toggleAnnual')}
                />
              </div>
              <div className="flex items-center justify-between">
                <div className="flex flex-col gap-[1px]">
                  <p className="text-[13px] font-medium text-text">
                    {t('masterData.leaveTypes.toggleDocument')}
                  </p>
                  <p className="text-[11px] text-text-3">
                    {t('masterData.leaveTypes.toggleDocumentHint')}
                  </p>
                </div>
                <Toggle
                  checked={requiresDocument}
                  onCheckedChange={(v) => setValue('requires_document', v)}
                  aria-label={t('masterData.leaveTypes.toggleDocument')}
                />
              </div>
            </div>

            {/* Warna */}
            <FormField htmlFor="lt-color" label={t('masterData.leaveTypes.fieldColor')}>
              <div id="lt-color" className="flex items-center gap-2">
                {COLOR_SWATCHES.map((c, i) => (
                  <button
                    key={c}
                    type="button"
                    aria-label={t('masterData.colorSwatch', { n: i + 1 })}
                    aria-pressed={selectedColor === c}
                    onClick={() => setValue('color', c)}
                    className="flex size-6 items-center justify-center rounded-full transition-transform hover:scale-110 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    style={{ backgroundColor: c }}
                  >
                    {selectedColor === c && <Check className="size-3.5 text-white" aria-hidden />}
                  </button>
                ))}
              </div>
            </FormField>
          </div>
        </ModalBody>
        <ModalFooter>
          <p className="mr-auto text-[11px] text-text-3">{t('masterData.auditHint')}</p>
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
// Row kebab menu — Edit + Deactivate (MD-1 soft-delete)
// ---------------------------------------------------------------------------

interface RowActionsMenuProps {
  onEdit: () => void;
  onDeactivate: () => void;
}

function RowActionsMenu({ onEdit, onDeactivate }: RowActionsMenuProps) {
  const { t } = useTranslation();

  return (
    <DropdownMenu triggerLabel={t('masterData.rowActions')} menuWidth={200}>
      <DropdownMenuItem icon={Pencil} onSelect={onEdit}>
        {t('masterData.menuEdit')}
      </DropdownMenuItem>
      <DropdownMenuItem icon={Trash2} tone="danger" onSelect={onDeactivate}>
        {t('masterData.menuDeactivate')}
      </DropdownMenuItem>
    </DropdownMenu>
  );
}

// ---------------------------------------------------------------------------
// LeaveTypesScreen
// ---------------------------------------------------------------------------

export function LeaveTypesScreen() {
  const { t } = useTranslation();

  const [statusFilter, setStatusFilter] = useState<LeaveTypeStatus | undefined>(undefined);
  const [searchQ, setSearchQ] = useState('');
  const [modalOpen, setModalOpen] = useState(false);
  const [editingItem, setEditingItem] = useState<LeaveType | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<LeaveType | null>(null);

  const { toast } = useToast();

  const params: ListLeaveTypesParams = {
    limit: PAGE_SIZE,
    status: statusFilter,
  };

  const query = useListLeaveTypes(params);
  const deleteMut = useSoftDeleteLeaveType();

  const hasFilters = Boolean(searchQ || statusFilter);

  function openAdd() {
    setEditingItem(null);
    setModalOpen(true);
  }

  function openEdit(item: LeaveType) {
    setEditingItem(item);
    setModalOpen(true);
  }

  function handleDone() {
    void query.refetch();
  }

  async function handleDelete() {
    if (!deleteTarget) return;
    try {
      await deleteMut.mutateAsync({ leaveTypeId: deleteTarget.id as LeaveTypeId });
      toast({ tone: 'success', title: t('masterData.leaveTypes.deleteSuccess') });
      void query.refetch();
    } catch (err) {
      const { message } = classifyError(err);
      toast({ tone: 'error', title: t(message) });
    } finally {
      setDeleteTarget(null);
    }
  }

  const columns: Column<LeaveType>[] = [
    {
      id: 'name',
      header: t('masterData.leaveTypes.colName'),
      width: 280,
      cell: (row) => (
        <div className="flex items-center gap-[10px]">
          {/* Data-driven swatch — inline style is the one allowed dynamic color per spec */}
          <div
            className="size-[14px] shrink-0 rounded-[4px]"
            style={{ backgroundColor: row.color ?? '#94a3b8' }}
          />
          <div className="flex flex-col gap-[2px]">
            <span className="text-[14px] font-medium text-text">{row.name}</span>
            {row.description && <span className="text-[11px] text-text-2">{row.description}</span>}
          </div>
        </div>
      ),
    },
    {
      id: 'code',
      header: t('masterData.leaveTypes.colCode'),
      width: 100,
      cell: (row) => <span className="font-mono text-[12px] text-text-2">{row.code}</span>,
    },
    {
      id: 'color',
      header: t('masterData.leaveTypes.colColor'),
      width: 100,
      cell: (row) => (
        /* Data-driven swatch — inline style allowed per spec */
        <div
          className="size-[14px] rounded-full"
          style={{ backgroundColor: row.color ?? '#94a3b8' }}
        />
      ),
    },
    {
      id: 'quota',
      header: t('masterData.leaveTypes.colQuota'),
      width: 160,
      cell: (row) =>
        row.default_annual_quota != null ? (
          <span className="text-[13px] text-text">
            {row.default_annual_quota} {t('masterData.leaveTypes.days')}
          </span>
        ) : (
          <span className="text-text-3">—</span>
        ),
    },
    {
      id: 'is_annual',
      header: t('masterData.leaveTypes.colAnnual'),
      width: 110,
      cell: (row) =>
        row.is_annual ? (
          <Check className="size-4 text-ok-tx" aria-label="Ya" />
        ) : (
          <Minus className="size-4 text-text-3" aria-label="Tidak" />
        ),
    },
    {
      id: 'requires_document',
      header: t('masterData.leaveTypes.colDocument'),
      width: 170,
      cell: (row) =>
        row.requires_document ? (
          <Check className="size-4 text-ok-tx" aria-label="Ya" />
        ) : (
          <Minus className="size-4 text-text-3" aria-label="Tidak" />
        ),
    },
    {
      id: 'status',
      header: t('masterData.leaveTypes.colStatus'),
      width: 120,
      cell: (row) => (
        <StatusBadge dot tone={row.status === LeaveTypeStatus.ACTIVE ? 'ok' : 'bad'}>
          {row.status === LeaveTypeStatus.ACTIVE
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
            title={t('masterData.leaveTypes.errorTitle')}
            description={t('errors.network')}
            onRetry={() => query.refetch()}
            retryLabel={t('common.retry')}
          />
        )}
      </div>
    );
  }

  const page = query.data?.data as ListLeaveTypes200 | undefined;
  // Client-side search filter — the API ListLeaveTypesParams has no q parameter
  const allRows = (page?.data ?? []) as LeaveType[];
  const rows = searchQ
    ? allRows.filter(
        (r) =>
          r.name.toLowerCase().includes(searchQ.toLowerCase()) ||
          r.code.toLowerCase().includes(searchQ.toLowerCase()),
      )
    : allRows;

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
            {t('masterData.leaveTypes.title')}
          </h1>
          <p className="text-[13px] text-text-2">{t('masterData.leaveTypes.subtitle')}</p>
        </div>
        <Button type="button" onClick={openAdd}>
          <Plus aria-hidden />
          {t('masterData.leaveTypes.add')}
        </Button>
      </div>

      {/* Table card */}
      <div className="overflow-hidden rounded-xl border border-border bg-surface">
        {/* Filter row */}
        <div className="flex items-center gap-[10px] border-b border-border-soft px-[18px] py-[14px]">
          <SearchField
            placeholder={t('masterData.leaveTypes.searchPlaceholder')}
            defaultValue={searchQ}
            containerClassName="w-[280px]"
            onChange={(e) => setSearchQ(e.target.value)}
          />
          <FilterSelect
            aria-label={t('masterData.filterStatus')}
            value={statusFilter ?? ''}
            onChange={(e) => setStatusFilter((e.target.value as LeaveTypeStatus) || undefined)}
          >
            <option value="">{t('masterData.filterAllStatus')}</option>
            <option value={LeaveTypeStatus.ACTIVE}>{t('masterData.statusActive')}</option>
            <option value={LeaveTypeStatus.INACTIVE}>{t('masterData.statusInactive')}</option>
          </FilterSelect>
        </div>

        <DataTable
          aria-label={t('masterData.leaveTypes.title')}
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
                title={t('masterData.leaveTypes.emptyTitle')}
                description={t('masterData.leaveTypes.emptyBody')}
              />
            )
          }
          rowActions={(row) => (
            <RowActionsMenu
              onEdit={() => openEdit(row)}
              onDeactivate={() => setDeleteTarget(row)}
            />
          )}
        />
      </div>

      {/* Tambah / Edit modal */}
      <LeaveTypeModal
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
        description={t('masterData.leaveTypes.deleteBody', { name: deleteTarget?.name ?? '' })}
        confirmLabel={t('masterData.deleteConfirm')}
        cancelLabel={t('common.cancel')}
        loading={deleteMut.isPending}
        onConfirm={handleDelete}
      />
    </div>
  );
}
