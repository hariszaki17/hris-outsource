/**
 * E2 · Lini Layanan — Daftar (Service Lines list)
 *
 * .pen frames:
 *   vV79c  E2 · Lini Layanan — Daftar  (list screen)
 *   IwKfo  E2 · Modal · Tambah/Edit Lini Layanan
 *
 * Refs: F2.x (service-line master data), BR-SL-1..4, INV-3.
 * Route: /service-lines  (validateSearch: { status?, cursor? })
 */

import { applyFieldErrors, classifyError } from '@/lib/api-error.ts';
import { zodResolver } from '@hookform/resolvers/zod';
import {
  type ListServiceLines200,
  type ServiceLine,
  ServiceLineStatus,
  useCreateServiceLine,
  useDiscontinueServiceLine,
  useListServiceLines,
} from '@swp/api-client/e2';
import {
  Button,
  type Column,
  ConfirmDialog,
  CursorPagination,
  DataTable,
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
  useToast,
} from '@swp/ui';
import { Link, useNavigate } from '@tanstack/react-router';
import { Info, Layers, MoreVertical, Plus, Sparkles, Trash2 } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { useFieldArray, useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';

// ---------------------------------------------------------------------------
// Zod schema (hand-written — E2 Zod deferred per engineering brief)
// ---------------------------------------------------------------------------

const serviceLineSchema = z.object({
  name: z.string().min(1, 'Nama wajib diisi').max(100),
  positions: z
    .array(
      z.object({
        name: z.string().min(1, 'Nama posisi wajib diisi').max(100),
        alias: z.string().max(100).optional(),
      }),
    )
    .default([]),
});
type ServiceLineFormValues = z.infer<typeof serviceLineSchema>;

// ---------------------------------------------------------------------------
// ServiceLineIcon — colored icon badge per service line name heuristic
// ---------------------------------------------------------------------------

function serviceLineIconBg(name: string): { bg: string; color: string; icon: string } {
  const lower = name.toLowerCase();
  if (lower.includes('facility') || lower.includes('fasilitas')) {
    return { bg: 'bg-info-bg', color: 'text-info-tx', icon: 'sparkles' };
  }
  if (lower.includes('building') || lower.includes('gedung')) {
    return { bg: 'bg-primary-soft', color: 'text-primary', icon: 'building-2' };
  }
  if (lower.includes('parking') || lower.includes('parkir')) {
    return { bg: 'bg-warn-bg', color: 'text-warn-tx', icon: 'square-parking' };
  }
  return { bg: 'bg-surface-2', color: 'text-text-2', icon: 'layers' };
}

// ---------------------------------------------------------------------------
// Row kebab menu
// ---------------------------------------------------------------------------

interface RowMenuProps {
  row: ServiceLine;
  onEdit: (row: ServiceLine) => void;
  onDiscontinue: (row: ServiceLine) => void;
}

function RowMenu({ row, onEdit, onDiscontinue }: RowMenuProps) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function handleClick(e: MouseEvent) {
      if (
        menuRef.current &&
        !menuRef.current.contains(e.target as Node) &&
        triggerRef.current &&
        !triggerRef.current.contains(e.target as Node)
      ) {
        setOpen(false);
      }
    }
    function handleKey(e: KeyboardEvent) {
      if (e.key === 'Escape') setOpen(false);
    }
    document.addEventListener('mousedown', handleClick);
    document.addEventListener('keydown', handleKey);
    return () => {
      document.removeEventListener('mousedown', handleClick);
      document.removeEventListener('keydown', handleKey);
    };
  }, [open]);

  const itemBase =
    'flex w-full items-center gap-[10px] rounded-[7px] px-3 py-[10px] text-[13px] text-text hover:bg-surface-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring';

  return (
    <div className="relative">
      <button
        ref={triggerRef}
        type="button"
        aria-label={t('serviceLines.rowActions')}
        className="flex size-8 items-center justify-center rounded-md text-text-2 hover:bg-surface-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        onClick={(e) => {
          e.stopPropagation();
          setOpen((v) => !v);
        }}
      >
        <MoreVertical className="size-[18px]" aria-hidden />
      </button>

      {open && (
        <div
          ref={menuRef}
          role="menu"
          className="absolute right-0 z-50 w-[220px] rounded-[10px] border border-border bg-surface p-1.5 shadow-overlay"
          style={{ top: '100%' }}
        >
          <button
            type="button"
            role="menuitem"
            className={itemBase}
            onClick={() => {
              setOpen(false);
              onEdit(row);
            }}
          >
            {t('serviceLines.menuEdit')}
          </button>
          {row.status === ServiceLineStatus.ACTIVE && (
            <button
              type="button"
              role="menuitem"
              className={`${itemBase} text-bad-tx`}
              onClick={() => {
                setOpen(false);
                onDiscontinue(row);
              }}
            >
              {t('serviceLines.menuDiscontinue')}
            </button>
          )}
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// AddModal — Tambah Lini Layanan (create only; .pen IwKfo)
// Edit + position management live on the detail hub (/service-lines/$id).
// ---------------------------------------------------------------------------

interface AddModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

function AddEditModal({ open, onClose, onSuccess }: AddModalProps) {
  const { t } = useTranslation();
  const { toast } = useToast();

  const createMutation = useCreateServiceLine();

  const {
    register,
    handleSubmit,
    reset,
    control,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<ServiceLineFormValues>({
    resolver: zodResolver(serviceLineSchema),
    defaultValues: { name: '', positions: [] },
  });

  const { fields, append, remove } = useFieldArray({ control, name: 'positions' });

  useEffect(() => {
    if (open) {
      reset({ name: '', positions: [] });
    }
  }, [open, reset]);

  function handleClose() {
    reset({ name: '', positions: [] });
    onClose();
  }

  async function onSubmit(values: ServiceLineFormValues) {
    try {
      const positions = values.positions
        .filter((p) => p.name.trim())
        .map((p) => ({ name: p.name, alias: p.alias?.trim() ? p.alias : undefined }));
      await createMutation.mutateAsync({
        data: { name: values.name, positions: positions.length ? positions : undefined },
      });
      toast({ tone: 'success', title: t('common.save') });
      handleClose();
      onSuccess();
    } catch (err) {
      if (applyFieldErrors(err, setError)) return;
      const { message } = classifyError(err);
      toast({ tone: 'error', title: t(message) });
    }
  }

  return (
    <Modal open={open} onOpenChange={(v) => !v && handleClose()} size="lg">
      <form onSubmit={handleSubmit(onSubmit)} noValidate>
        <ModalHeader
          icon={Layers}
          tone="brand"
          title={t('serviceLines.addModal.title')}
          closeLabel={t('common.close')}
        />
        <ModalBody>
          <div className="flex flex-col gap-4">
            <FormField
              label={t('serviceLines.addModal.fieldName')}
              htmlFor="sl-name"
              error={errors.name?.message}
              required
              span={2}
            >
              <Input
                id="sl-name"
                placeholder={t('serviceLines.addModal.fieldNamePlaceholder')}
                {...register('name')}
                aria-describedby={errors.name ? 'sl-name-error' : undefined}
              />
            </FormField>

            {/* Initial positions (optional) — created atomically with the line (SP-3) */}
            <div className="flex flex-col gap-3 rounded-lg border border-border bg-surface-2/40 p-3">
              <div className="flex flex-col gap-0.5">
                <span className="text-sm font-semibold text-text">
                  {t('serviceLines.addModal.positionsLabel')}
                </span>
                <span className="text-xs text-text-3">
                  {t('serviceLines.addModal.positionsHint')}
                </span>
              </div>

              {fields.length > 0 && (
                <div className="flex flex-col gap-2">
                  {fields.map((field, index) => {
                    const nameError = errors.positions?.[index]?.name?.message;
                    const aliasError = errors.positions?.[index]?.alias?.message;
                    return (
                      <div key={field.id} className="flex items-start gap-2">
                        <div className="flex flex-1 flex-col gap-1">
                          <Input
                            placeholder={t('serviceLines.addModal.posNamePlaceholder')}
                            aria-label={t('serviceLines.addModal.posNamePlaceholder')}
                            {...register(`positions.${index}.name`)}
                          />
                          {nameError && (
                            <p role="alert" className="text-xs text-bad-tx">
                              {nameError}
                            </p>
                          )}
                        </div>
                        <div className="flex flex-1 flex-col gap-1">
                          <Input
                            placeholder={t('serviceLines.addModal.posAliasPlaceholder')}
                            aria-label={t('serviceLines.addModal.posAliasPlaceholder')}
                            {...register(`positions.${index}.alias`)}
                          />
                          {aliasError && (
                            <p role="alert" className="text-xs text-bad-tx">
                              {aliasError}
                            </p>
                          )}
                        </div>
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          className="shrink-0 text-text-3 hover:text-bad-tx"
                          aria-label={t('serviceLines.addModal.removePosition')}
                          onClick={() => remove(index)}
                        >
                          <Trash2 className="size-4" aria-hidden />
                        </Button>
                      </div>
                    );
                  })}
                </div>
              )}

              <div>
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  onClick={() => append({ name: '', alias: '' })}
                >
                  <Plus className="size-[14px]" aria-hidden />
                  {t('serviceLines.addModal.addPositionRow')}
                </Button>
              </div>
            </div>
          </div>
        </ModalBody>
        <ModalFooter className="justify-between">
          <p className="text-[11px] text-text-3">{t('serviceLines.addModal.auditHint')}</p>
          <div className="flex items-center gap-[10px]">
            <Button type="button" variant="ghost" onClick={handleClose}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {t('serviceLines.addModal.save')}
            </Button>
          </div>
        </ModalFooter>
      </form>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// ServiceLinesScreen (.pen vV79c)
// ---------------------------------------------------------------------------

export function ServiceLinesScreen() {
  const { t } = useTranslation();
  const { toast } = useToast();
  const navigate = useNavigate();

  // Filter / pagination state
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState<string>('');
  const [cursor, setCursor] = useState<string | undefined>(undefined);
  const [prevCursors, setPrevCursors] = useState<string[]>([]);

  // Modal state (create only — edit + positions live on the detail hub)
  const [addEditOpen, setAddEditOpen] = useState(false);
  const [discontinueTarget, setDiscontinueTarget] = useState<ServiceLine | null>(null);

  const queryParams = {
    ...(statusFilter ? { status: statusFilter as 'ACTIVE' | 'INACTIVE' } : undefined),
    ...(cursor ? { cursor } : undefined),
  };

  const query = useListServiceLines(queryParams);
  const discontinueMutation = useDiscontinueServiceLine();

  // Orval wraps response: query.data is { data, status, headers }; body lives at .data
  const page = query.data?.data as ListServiceLines200 | undefined;
  const rows: ServiceLine[] = (page?.data ?? []) as ServiceLine[];
  const hasMore = page?.has_more ?? false;
  const nextCursor = page?.next_cursor ?? null;

  // Client-side name filter (API has no q= param per spec)
  const filtered = search
    ? rows.filter((r) => r.name.toLowerCase().includes(search.toLowerCase()))
    : rows;

  function handleEdit(row: ServiceLine) {
    // Edit (name + positions) is the detail hub, not a rename-only modal.
    void navigate({ to: '/service-lines/$serviceLineId', params: { serviceLineId: row.id } });
  }

  function handleDiscontinue(row: ServiceLine) {
    setDiscontinueTarget(row);
  }

  async function confirmDiscontinue() {
    if (!discontinueTarget) return;
    try {
      await discontinueMutation.mutateAsync({ serviceLineId: discontinueTarget.id });
      toast({ tone: 'success', title: t('serviceLines.menuDiscontinue') });
      setDiscontinueTarget(null);
      void query.refetch();
    } catch (err) {
      const { message } = classifyError(err);
      toast({ tone: 'error', title: t(message) });
    }
  }

  function handleNext() {
    if (nextCursor) {
      setPrevCursors((p) => [...p, cursor ?? '']);
      setCursor(nextCursor);
    }
  }

  function handlePrev() {
    const prev = prevCursors[prevCursors.length - 1];
    setPrevCursors((p) => p.slice(0, -1));
    setCursor(prev || undefined);
  }

  // DataTable column definitions
  const columns: Column<ServiceLine>[] = [
    {
      id: 'name',
      header: (
        <span className="text-[11px] font-semibold uppercase tracking-[0.5px] text-text-3">
          {t('serviceLines.colName')}
        </span>
      ),
      width: 360,
      cell: (row: ServiceLine) => {
        const { bg, color } = serviceLineIconBg(row.name);
        return (
          <div className="flex items-center gap-3 py-[14px] pl-4 pr-4">
            <div
              className={`flex size-9 shrink-0 items-center justify-center rounded-lg ${bg}`}
              aria-hidden
            >
              <Sparkles className={`size-[18px] ${color}`} aria-hidden />
            </div>
            <div className="flex flex-col gap-0.5">
              <Link
                to="/service-lines/$serviceLineId"
                params={{ serviceLineId: row.id }}
                className="text-[14px] font-medium text-text hover:underline focus-visible:outline-none focus-visible:underline"
              >
                {row.name}
              </Link>
            </div>
          </div>
        );
      },
    },
    {
      id: 'positions',
      header: (
        <span className="text-[11px] font-semibold uppercase tracking-[0.5px] text-text-3">
          {t('serviceLines.colPositions')}
        </span>
      ),
      width: 140,
      cell: (row: ServiceLine) => (
        <span className="py-[14px] px-4 text-[13px] text-text">
          {t('serviceLines.positionsCount', { count: row.position_count ?? 0 })}
        </span>
      ),
    },
    {
      id: 'status',
      header: (
        <span className="text-[11px] font-semibold uppercase tracking-[0.5px] text-text-3">
          {t('serviceLines.colStatus')}
        </span>
      ),
      width: 140,
      cell: (row: ServiceLine) => (
        <div className="py-[14px] px-4">
          <StatusBadge tone={row.status === ServiceLineStatus.ACTIVE ? 'ok' : 'bad'} dot>
            {row.status === ServiceLineStatus.ACTIVE
              ? t('serviceLines.statusActive')
              : t('serviceLines.statusInactive')}
          </StatusBadge>
        </div>
      ),
    },
  ];

  // Loading / error states
  if (query.isError) {
    const { kind } = classifyError(query.error);
    if (kind === 'forbidden') {
      return (
        <StateView
          kind="no-permission"
          title={t('serviceLines.errorTitle')}
          description={t('errors.forbidden')}
        />
      );
    }
    return (
      <StateView
        kind="error"
        title={t('serviceLines.errorTitle')}
        description={t('serviceLines.errorBody')}
        onRetry={() => void query.refetch()}
        retryLabel={t('common.retry')}
      />
    );
  }

  return (
    <div className="flex flex-col gap-[18px]">
      {/* Title band (.pen TitleBand MmZx4) */}
      <div className="flex items-center justify-between rounded-xl border border-border bg-surface px-5 py-[18px]">
        <div className="flex flex-col gap-1">
          <h1 className="text-[20px] font-semibold text-text">{t('serviceLines.title')}</h1>
          <p className="text-[13px] text-text-2">{t('serviceLines.subtitle')}</p>
        </div>
        <Button type="button" onClick={() => setAddEditOpen(true)}>
          <Plus className="size-[15px]" aria-hidden />
          {t('serviceLines.addButton')}
        </Button>
      </div>

      {/* Role note banner (.pen RoleNote edaeM) */}
      <div className="flex items-center gap-[9px] rounded-lg border border-l-[3px] border-border bg-surface px-[14px] py-[10px]">
        <Info className="size-[15px] shrink-0 text-text-3" aria-hidden />
        <p className="text-[12px] text-text-2">{t('serviceLines.roleNote')}</p>
      </div>

      {/* Table card (.pen TableCard bM6mE) */}
      <div className="flex flex-col overflow-hidden rounded-xl border border-border bg-surface">
        {/* Filter row (.pen FilterRow FLIPT) */}
        <div className="flex items-center gap-[10px] border-b border-border-soft px-[18px] py-[14px]">
          <SearchField
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t('serviceLines.searchPlaceholder')}
            aria-label={t('serviceLines.searchPlaceholder')}
            className="w-[280px]"
          />
          <FilterSelect
            aria-label={t('serviceLines.filterStatus')}
            value={statusFilter}
            onChange={(e) => {
              setStatusFilter(e.target.value);
              setCursor(undefined);
              setPrevCursors([]);
            }}
          >
            <option value="">{t('serviceLines.filterStatus')}</option>
            <option value="ACTIVE">{t('serviceLines.filterStatusActive')}</option>
            <option value="INACTIVE">{t('serviceLines.filterStatusInactive')}</option>
          </FilterSelect>
        </div>

        {/* DataTable */}
        <DataTable
          columns={columns}
          data={filtered}
          getRowId={(r) => r.id}
          isLoading={query.isLoading}
          skeletonRows={6}
          aria-label={t('serviceLines.title')}
          empty={
            search || statusFilter ? (
              <EmptyState
                variant="filtered"
                title={t('serviceLines.filteredTitle')}
                description={t('serviceLines.filteredBody')}
              />
            ) : (
              <EmptyState
                variant="fresh"
                title={t('serviceLines.emptyTitle')}
                description={t('serviceLines.emptyBody')}
                action={
                  <Button type="button" onClick={() => setAddEditOpen(true)}>
                    <Plus className="size-[14px]" aria-hidden />
                    {t('serviceLines.addButton')}
                  </Button>
                }
              />
            )
          }
          rowActions={(row) => (
            <RowMenu row={row} onEdit={handleEdit} onDiscontinue={handleDiscontinue} />
          )}
          footer={
            <CursorPagination
              rangeLabel={t('serviceLines.resultCount', { count: filtered.length })}
              hasPrev={prevCursors.length > 0}
              hasNext={hasMore}
              onPrev={handlePrev}
              onNext={handleNext}
              prevLabel={t('common.prev')}
              nextLabel={t('common.next')}
            />
          }
        />
      </div>

      {/* Modals */}
      <AddEditModal
        open={addEditOpen}
        onClose={() => setAddEditOpen(false)}
        onSuccess={() => void query.refetch()}
      />

      <ConfirmDialog
        open={discontinueTarget !== null}
        onOpenChange={(v) => !v && setDiscontinueTarget(null)}
        icon={Layers}
        tone="danger"
        confirmTone="danger"
        title={t('serviceLines.discontinueConfirm.title')}
        description={t('serviceLines.discontinueConfirm.body')}
        confirmLabel={t('serviceLines.discontinueConfirm.confirm')}
        cancelLabel={t('common.cancel')}
        onConfirm={confirmDiscontinue}
        loading={discontinueMutation.isPending}
      />
    </div>
  );
}
