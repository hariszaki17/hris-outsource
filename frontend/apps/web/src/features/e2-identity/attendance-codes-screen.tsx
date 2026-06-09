import { applyFieldErrors, classifyError } from '@/lib/api-error.ts';
import { zodResolver } from '@hookform/resolvers/zod';
import {
  type AttendanceCode,
  type AttendanceCodeId,
  AttendanceCodeStatus,
  type AttendanceCodeWriteRequest,
  type ListAttendanceCodes200,
  type ListAttendanceCodesParams,
  useCreateAttendanceCode,
  useListAttendanceCodes,
  useSoftDeleteAttendanceCode,
  useUpdateAttendanceCode,
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
  StateView,
  StatusBadge,
  Toggle,
  useToast,
} from '@swp/ui';
import { Link } from '@tanstack/react-router';
import { ArrowLeft, Check, Clock3, Pencil, Plus, Trash2 } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';

/**
 * E2 · Kode Kehadiran — list + Tambah/Edit modal (color picker + flag toggles) + soft-delete confirm.
 * Built from .pen frames `R5xoi` (list), `u8eXaW` (modal).
 *
 * Color note: the attendance code's `color` field is a data value. It is rendered as a small
 * data-driven swatch using inline style background — the ONE allowed dynamic color per task spec.
 * All status UI (Aktif/Nonaktif pills) use StatusBadge + tokens only.
 * DESIGN-SYSTEM §2: present status color is teal (#0F8B8D), NOT brand green.
 *
 * Routes: /master-data/attendance-codes — consumers must register in the router.
 * Refs: AC-1 (create), AC-2 (is_billable), AC-3 (needs_verification), MD-1 (soft-delete), MD-2 (unique code).
 */

const PAGE_SIZE = 200;

// ---------------------------------------------------------------------------
// Zod schema (hand-written — E2 Zod deferred)
// ---------------------------------------------------------------------------

const attendanceCodeSchema = z.object({
  code: z
    .string()
    .min(1, 'Kode wajib diisi')
    .max(30)
    .regex(/^[A-Z0-9_]+$/, 'Kode harus huruf kapital/angka/underscore'),
  label: z.string().min(1, 'Label wajib diisi').max(60),
  description: z.string().optional(),
  color: z.string().min(1, 'Warna wajib dipilih'),
  is_workday: z.boolean(),
  is_paid: z.boolean(),
  is_billable: z.boolean(),
  needs_verification: z.boolean(),
});

type AttendanceCodeFormValues = z.infer<typeof attendanceCodeSchema>;

// Color palette. Teal (#0F8B8D) is "present" per DESIGN-SYSTEM §2 (NOT brand green #188E4D).
const CODE_COLOR_SWATCHES = [
  '#0F8B8D', // teal — "present" per DESIGN-SYSTEM §2
  '#22c55e', // ok green
  '#ef4444', // bad red
  '#f59e0b', // warn amber
  '#3b82f6', // info blue
  '#8b5cf6', // accent purple
  '#64748b', // neutral slate
  '#1e293b', // dark slate
];

// ---------------------------------------------------------------------------
// AttendanceCodeModal — Tambah/Edit
// ---------------------------------------------------------------------------

interface AttendanceCodeModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  editing: AttendanceCode | null;
  onDone: () => void;
}

type FlagKey = 'is_workday' | 'is_paid' | 'is_billable' | 'needs_verification';

function AttendanceCodeModal({ open, onOpenChange, editing, onDone }: AttendanceCodeModalProps) {
  const { t } = useTranslation();
  const { toast } = useToast();
  const isEdit = editing !== null;

  const createMut = useCreateAttendanceCode();
  const updateMut = useUpdateAttendanceCode();
  const isPending = createMut.isPending || updateMut.isPending;

  const form = useForm<AttendanceCodeFormValues>({
    resolver: zodResolver(attendanceCodeSchema),
    defaultValues: {
      code: '',
      label: '',
      description: '',
      color: CODE_COLOR_SWATCHES[0],
      is_workday: true,
      is_paid: true,
      is_billable: true,
      needs_verification: false,
    },
  });

  const { register, handleSubmit, reset, setValue, watch, setError, formState } = form;
  const selectedColor = watch('color');
  const isWorkday = watch('is_workday');
  const isPaid = watch('is_paid');
  const isBillable = watch('is_billable');
  const needsVerification = watch('needs_verification');

  useEffect(() => {
    if (open) {
      if (editing) {
        reset({
          code: editing.code,
          label: editing.label,
          description: editing.description ?? '',
          color: editing.color,
          is_workday: editing.is_workday,
          is_paid: editing.is_paid,
          is_billable: editing.is_billable,
          needs_verification: editing.needs_verification,
        });
      } else {
        reset({
          code: '',
          label: '',
          description: '',
          color: CODE_COLOR_SWATCHES[0],
          is_workday: true,
          is_paid: true,
          is_billable: true,
          needs_verification: false,
        });
      }
    }
  }, [open, editing, reset]);

  async function onSubmit(values: AttendanceCodeFormValues) {
    const payload: AttendanceCodeWriteRequest = {
      code: values.code,
      label: values.label,
      description: values.description || undefined,
      color: values.color,
      is_workday: values.is_workday,
      is_paid: values.is_paid,
      is_billable: values.is_billable,
      needs_verification: values.needs_verification,
    };

    try {
      if (isEdit && editing) {
        await updateMut.mutateAsync({
          attendanceCodeId: editing.id as AttendanceCodeId,
          data: payload,
        });
        toast({ tone: 'success', title: t('masterData.attendanceCodes.updateSuccess') });
      } else {
        await createMut.mutateAsync({ data: payload });
        toast({ tone: 'success', title: t('masterData.attendanceCodes.createSuccess') });
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

  const flags: { key: FlagKey; value: boolean; label: string; hint: string }[] = [
    {
      key: 'is_workday',
      value: isWorkday,
      label: t('masterData.attendanceCodes.flagWorkday'),
      hint: t('masterData.attendanceCodes.flagWorkdayHint'),
    },
    {
      key: 'is_paid',
      value: isPaid,
      label: t('masterData.attendanceCodes.flagPaid'),
      hint: t('masterData.attendanceCodes.flagPaidHint'),
    },
    {
      key: 'is_billable',
      value: isBillable,
      label: t('masterData.attendanceCodes.flagBillable'),
      hint: t('masterData.attendanceCodes.flagBillableHint'),
    },
    {
      key: 'needs_verification',
      value: needsVerification,
      label: t('masterData.attendanceCodes.flagVerification'),
      hint: t('masterData.attendanceCodes.flagVerificationHint'),
    },
  ];

  return (
    <Modal open={open} onOpenChange={onOpenChange} size="lg">
      <form onSubmit={handleSubmit(onSubmit)} noValidate>
        <ModalHeader
          icon={Clock3}
          tone="brand"
          title={
            isEdit
              ? t('masterData.attendanceCodes.editTitle')
              : t('masterData.attendanceCodes.addTitle')
          }
        />
        <ModalBody>
          <div className="flex flex-col gap-[14px]">
            <p className="text-[12px] text-text-2">
              {t('masterData.attendanceCodes.modalSubtitle')}
            </p>

            {/* Kode + Label row */}
            <div className="flex gap-3">
              <FormField
                htmlFor="ac-code"
                label={t('masterData.attendanceCodes.fieldCode')}
                required
                error={formState.errors.code?.message}
                className="flex-1"
              >
                <input
                  id="ac-code"
                  {...register('code')}
                  className="h-9 w-full rounded-md border border-border bg-surface px-3 font-mono text-[14px] text-text outline-none placeholder:text-text-3 focus:border-primary focus:ring-1 focus:ring-primary"
                  placeholder={t('masterData.attendanceCodes.fieldCodePlaceholder')}
                />
              </FormField>
              <FormField
                htmlFor="ac-label"
                label={t('masterData.attendanceCodes.fieldLabel')}
                required
                error={formState.errors.label?.message}
                className="flex-1"
              >
                <input
                  id="ac-label"
                  {...register('label')}
                  className="h-9 w-full rounded-md border border-border bg-surface px-3 text-[14px] text-text outline-none placeholder:text-text-3 focus:border-primary focus:ring-1 focus:ring-primary"
                  placeholder={t('masterData.attendanceCodes.fieldLabelPlaceholder')}
                />
              </FormField>
            </div>

            {/* Warna — data-driven swatch; inline style allowed only here per spec */}
            <FormField
              htmlFor="ac-color"
              label={t('masterData.attendanceCodes.fieldColor')}
              error={formState.errors.color?.message}
            >
              <div id="ac-color" className="flex items-center gap-2">
                {CODE_COLOR_SWATCHES.map((c, i) => (
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
                {/* Selected color preview */}
                <div
                  className="ml-2 size-6 rounded-full border-2 border-border"
                  style={{ backgroundColor: selectedColor }}
                  aria-hidden
                />
              </div>
            </FormField>

            {/* Flags */}
            <div className="flex flex-col gap-[10px] rounded-lg bg-surface-2 px-[14px] py-3">
              {flags.map(({ key, value, label, hint }) => (
                <div key={key} className="flex items-center justify-between">
                  <div className="flex flex-col gap-[1px]">
                    <p className="text-[13px] font-medium text-text">{label}</p>
                    <p className="text-[11px] text-text-3">{hint}</p>
                  </div>
                  <Toggle
                    checked={value}
                    onCheckedChange={(v) => setValue(key, v)}
                    aria-label={label}
                  />
                </div>
              ))}
            </div>
          </div>
        </ModalBody>
        <ModalFooter>
          <p className="mr-auto text-[11px] text-text-3">
            {t('masterData.attendanceCodes.auditHint')}
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
// AttendanceCodesScreen
// ---------------------------------------------------------------------------

export function AttendanceCodesScreen() {
  const { t } = useTranslation();

  const [statusFilter, setStatusFilter] = useState<AttendanceCodeStatus | undefined>(undefined);
  const [modalOpen, setModalOpen] = useState(false);
  const [editingItem, setEditingItem] = useState<AttendanceCode | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<AttendanceCode | null>(null);

  const { toast } = useToast();

  const params: ListAttendanceCodesParams = {
    limit: PAGE_SIZE,
    status: statusFilter,
  };

  const query = useListAttendanceCodes(params);
  const deleteMut = useSoftDeleteAttendanceCode();

  const hasFilters = Boolean(statusFilter);

  function openAdd() {
    setEditingItem(null);
    setModalOpen(true);
  }

  function openEdit(item: AttendanceCode) {
    setEditingItem(item);
    setModalOpen(true);
  }

  function handleDone() {
    void query.refetch();
  }

  async function handleDelete() {
    if (!deleteTarget) return;
    try {
      await deleteMut.mutateAsync({
        attendanceCodeId: deleteTarget.id as AttendanceCodeId,
      });
      toast({ tone: 'success', title: t('masterData.attendanceCodes.deleteSuccess') });
      void query.refetch();
    } catch (err) {
      const { message } = classifyError(err);
      toast({ tone: 'error', title: t(message) });
    } finally {
      setDeleteTarget(null);
    }
  }

  const columns: Column<AttendanceCode>[] = [
    {
      id: 'code',
      header: t('masterData.attendanceCodes.colCode'),
      width: 130,
      cell: (row) => (
        <div className="flex items-center">
          <span className="rounded-md bg-surface-2 px-[10px] py-1 font-mono text-[11px] font-bold tracking-wide text-text-2">
            {row.code}
          </span>
        </div>
      ),
    },
    {
      id: 'label',
      header: t('masterData.attendanceCodes.colLabel'),
      width: 240,
      cell: (row) => <span className="text-[13px] text-text">{row.label}</span>,
    },
    {
      id: 'is_workday',
      header: t('masterData.attendanceCodes.colWorkday'),
      width: 120,
      cell: (row) =>
        row.is_workday ? (
          <Check className="size-4 text-ok-tx" aria-label="Ya" />
        ) : (
          <span className="text-[13px] text-text-3">—</span>
        ),
    },
    {
      id: 'is_paid',
      header: t('masterData.attendanceCodes.colPaid'),
      width: 110,
      cell: (row) =>
        row.is_paid ? (
          <Check className="size-4 text-ok-tx" aria-label="Ya" />
        ) : (
          <span className="text-[13px] text-text-3">—</span>
        ),
    },
    {
      id: 'is_billable',
      header: t('masterData.attendanceCodes.colBillable'),
      width: 110,
      cell: (row) =>
        row.is_billable ? (
          <Check className="size-4 text-ok-tx" aria-label="Ya" />
        ) : (
          <span className="text-[13px] text-text-3">—</span>
        ),
    },
    {
      id: 'needs_verification',
      header: t('masterData.attendanceCodes.colVerification'),
      width: 120,
      cell: (row) =>
        row.needs_verification ? (
          <Check className="size-4 text-ok-tx" aria-label="Ya" />
        ) : (
          <span className="text-[13px] text-text-3">—</span>
        ),
    },
    {
      id: 'color',
      header: t('masterData.attendanceCodes.colColor'),
      width: 100,
      cell: (row) => (
        /* Data-driven swatch — inline style is the ONE allowed dynamic color per task spec */
        <div
          className="size-[14px] rounded-full"
          style={{ backgroundColor: row.color }}
          aria-label={t('masterData.colorLabel')}
        />
      ),
    },
    {
      id: 'status',
      header: t('masterData.attendanceCodes.colStatus'),
      width: 110,
      cell: (row) => (
        <StatusBadge dot tone={row.status === AttendanceCodeStatus.ACTIVE ? 'ok' : 'bad'}>
          {row.status === AttendanceCodeStatus.ACTIVE
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
            title={t('masterData.attendanceCodes.errorTitle')}
            description={t('errors.network')}
            onRetry={() => query.refetch()}
            retryLabel={t('common.retry')}
          />
        )}
      </div>
    );
  }

  const page = query.data?.data as ListAttendanceCodes200 | undefined;
  const rows = (page?.data ?? []) as AttendanceCode[];

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
            {t('masterData.attendanceCodes.title')}
          </h1>
          <p className="text-[13px] text-text-2">{t('masterData.attendanceCodes.subtitle')}</p>
        </div>
        <Button type="button" onClick={openAdd}>
          <Plus aria-hidden />
          {t('masterData.attendanceCodes.add')}
        </Button>
      </div>

      {/* Table card */}
      <div className="overflow-hidden rounded-xl border border-border bg-surface">
        <div className="border-b border-border-soft px-[18px] py-[14px]">
          <FilterSelect
            aria-label={t('masterData.filterStatus')}
            value={statusFilter ?? ''}
            onChange={(e) => setStatusFilter((e.target.value as AttendanceCodeStatus) || undefined)}
          >
            <option value="">{t('masterData.filterAllStatus')}</option>
            <option value={AttendanceCodeStatus.ACTIVE}>{t('masterData.statusActive')}</option>
            <option value={AttendanceCodeStatus.INACTIVE}>{t('masterData.statusInactive')}</option>
          </FilterSelect>
        </div>

        <DataTable
          aria-label={t('masterData.attendanceCodes.title')}
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
                title={t('masterData.attendanceCodes.emptyTitle')}
                description={t('masterData.attendanceCodes.emptyBody')}
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

      {/* Modal */}
      <AttendanceCodeModal
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
        description={t('masterData.attendanceCodes.deleteBody', {
          code: deleteTarget?.code ?? '',
        })}
        confirmLabel={t('masterData.deleteConfirm')}
        cancelLabel={t('common.cancel')}
        loading={deleteMut.isPending}
        onConfirm={handleDelete}
      />
    </div>
  );
}
