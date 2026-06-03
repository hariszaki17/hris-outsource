/**
 * E2 · Lini Layanan — Detail (Service Line Detail + Positions)
 *
 * .pen frames:
 *   I8WeKy  E2 · Lini Layanan — Detail (Parking)
 *   hb7vL   E2 · Modal · Tambah/Edit Posisi
 *
 * Refs: F2.x, BR-SL-2, BR-POS-1..3, INV-3 (unique name per service line).
 * Route: /service-lines/$serviceLineId
 */

import { applyFieldErrors, classifyError } from '@/lib/api-error.ts';
import { zodResolver } from '@hookform/resolvers/zod';
import {
  type ListPositionsInServiceLine200,
  type Position,
  PositionStatus,
  type ServiceLine,
  ServiceLineStatus,
  useCreatePosition,
  useDiscontinueServiceLine,
  useGetServiceLine,
  useListPositionsInServiceLine,
  useSoftDeletePosition,
  useUpdatePosition,
  useUpdateServiceLine,
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
  StateView,
  StatusBadge,
  useToast,
} from '@swp/ui';
import { Link } from '@tanstack/react-router';
import {
  ArrowLeft,
  Contact,
  Layers,
  MoreVertical,
  Pencil,
  Plus,
  TriangleAlert,
} from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';

// ---------------------------------------------------------------------------
// Zod schemas (hand-written — E2 Zod deferred per engineering brief)
// ---------------------------------------------------------------------------

const serviceLineEditSchema = z.object({
  name: z.string().min(1, 'Nama wajib diisi').max(100),
});
type ServiceLineEditValues = z.infer<typeof serviceLineEditSchema>;

const positionSchema = z.object({
  name: z.string().min(1, 'Nama posisi wajib diisi').max(100),
  alias: z.string().max(100).optional(),
});
type PositionFormValues = z.infer<typeof positionSchema>;

// ---------------------------------------------------------------------------
// Position row kebab menu
// ---------------------------------------------------------------------------

interface PosRowMenuProps {
  position: Position;
  onEdit: (p: Position) => void;
  onDelete: (p: Position) => void;
}

function PosRowMenu({ position, onEdit, onDelete }: PosRowMenuProps) {
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
        aria-label={t('users.rowActions')}
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
          className="absolute right-0 z-50 w-[200px] rounded-[10px] border border-border bg-surface p-1.5 shadow-overlay"
          style={{ top: '100%' }}
        >
          <button
            type="button"
            role="menuitem"
            className={itemBase}
            onClick={() => {
              setOpen(false);
              onEdit(position);
            }}
          >
            <Pencil className="size-[14px] shrink-0 text-text-2" aria-hidden />
            {t('common.save')}
          </button>
          {position.status === PositionStatus.ACTIVE && (
            <button
              type="button"
              role="menuitem"
              className={`${itemBase} text-bad-tx`}
              onClick={() => {
                setOpen(false);
                onDelete(position);
              }}
            >
              {t('serviceLines.deletePositionConfirm.confirm')}
            </button>
          )}
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// EditServiceLineModal (inline edit from header)
// ---------------------------------------------------------------------------

interface EditSLModalProps {
  open: boolean;
  serviceLine: ServiceLine;
  onClose: () => void;
  onSuccess: () => void;
}

function EditServiceLineModal({ open, serviceLine, onClose, onSuccess }: EditSLModalProps) {
  const { t } = useTranslation();
  const { toast } = useToast();
  const updateMutation = useUpdateServiceLine();

  const {
    register,
    handleSubmit,
    reset,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<ServiceLineEditValues>({
    resolver: zodResolver(serviceLineEditSchema),
  });

  useEffect(() => {
    if (open) reset({ name: serviceLine.name });
  }, [open, serviceLine.name, reset]);

  function handleClose() {
    reset();
    onClose();
  }

  async function onSubmit(values: ServiceLineEditValues) {
    try {
      await updateMutation.mutateAsync({
        serviceLineId: serviceLine.id,
        data: { name: values.name },
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
          title={t('serviceLines.addModal.editTitle')}
          closeLabel={t('common.close')}
        />
        <ModalBody>
          <FormField
            label={t('serviceLines.addModal.fieldName')}
            htmlFor="sl-edit-name"
            error={errors.name?.message}
            required
            span={2}
          >
            <Input
              id="sl-edit-name"
              placeholder={t('serviceLines.addModal.fieldNamePlaceholder')}
              {...register('name')}
            />
          </FormField>
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
// AddEditPositionModal (.pen hb7vL)
// ---------------------------------------------------------------------------

interface AddEditPositionModalProps {
  open: boolean;
  serviceLineId: string;
  serviceLineName: string;
  editing: Position | null;
  onClose: () => void;
  onSuccess: () => void;
}

function AddEditPositionModal({
  open,
  serviceLineId,
  serviceLineName,
  editing,
  onClose,
  onSuccess,
}: AddEditPositionModalProps) {
  const { t } = useTranslation();
  const { toast } = useToast();
  const isEdit = editing !== null;

  const createMutation = useCreatePosition();
  const updateMutation = useUpdatePosition();

  const {
    register,
    handleSubmit,
    reset,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<PositionFormValues>({
    resolver: zodResolver(positionSchema),
  });

  useEffect(() => {
    if (open) {
      reset({
        name: editing?.name ?? '',
        alias: editing?.alias ?? '',
      });
    }
  }, [open, editing, reset]);

  function handleClose() {
    reset();
    onClose();
  }

  async function onSubmit(values: PositionFormValues) {
    try {
      if (isEdit && editing) {
        await updateMutation.mutateAsync({
          positionId: editing.id,
          data: { name: values.name, alias: values.alias || undefined },
        });
      } else {
        await createMutation.mutateAsync({
          serviceLineId,
          data: { name: values.name, alias: values.alias || undefined },
        });
      }
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
          title={isEdit ? t('serviceLines.posModal.editTitle') : t('serviceLines.posModal.title')}
          closeLabel={t('common.close')}
        />
        <ModalBody>
          <div className="flex flex-col gap-4">
            {/* Service line read-only display (.pen F Lini Layanan zR43C) */}
            <FormField
              label={t('serviceLines.posModal.fieldServiceLine')}
              htmlFor="pos-sl"
              span={2}
            >
              <FilterSelect id="pos-sl" disabled value={serviceLineId} containerClassName="w-full">
                <option value={serviceLineId}>{serviceLineName}</option>
              </FilterSelect>
            </FormField>

            {/* Position name */}
            <FormField
              label={t('serviceLines.posModal.fieldName')}
              htmlFor="pos-name"
              error={errors.name?.message}
              required
              span={2}
            >
              <Input
                id="pos-name"
                placeholder={t('serviceLines.posModal.fieldNamePlaceholder')}
                {...register('name')}
              />
            </FormField>

            {/* Alias */}
            <FormField
              label={t('serviceLines.posModal.fieldAlias')}
              htmlFor="pos-alias"
              error={errors.alias?.message}
              span={2}
            >
              <Input
                id="pos-alias"
                placeholder={t('serviceLines.posModal.fieldAliasPlaceholder')}
                {...register('alias')}
              />
            </FormField>
          </div>
        </ModalBody>
        <ModalFooter className="justify-between">
          <p className="text-[11px] text-text-3">{t('serviceLines.posModal.auditHint')}</p>
          <div className="flex items-center gap-[10px]">
            <Button type="button" variant="ghost" onClick={handleClose}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {t('serviceLines.posModal.save')}
            </Button>
          </div>
        </ModalFooter>
      </form>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// ServiceLineDetailScreen props
// ---------------------------------------------------------------------------

export interface ServiceLineDetailScreenProps {
  serviceLineId: string;
}

// ---------------------------------------------------------------------------
// ServiceLineDetailScreen (.pen I8WeKy)
// ---------------------------------------------------------------------------

export function ServiceLineDetailScreen({ serviceLineId }: ServiceLineDetailScreenProps) {
  const { t } = useTranslation();
  const { toast } = useToast();

  // Position pagination state
  const [posCursor, setPosCursor] = useState<string | undefined>(undefined);
  const [prevPosCursors, setPrevPosCursors] = useState<string[]>([]);
  const [posStatusFilter, setPosStatusFilter] = useState<string>('');

  // Modal state
  const [editSLOpen, setEditSLOpen] = useState(false);
  const [posModalOpen, setPosModalOpen] = useState(false);
  const [editingPosition, setEditingPosition] = useState<Position | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Position | null>(null);
  const [discontinueSLTarget, setDiscontinueSLTarget] = useState<ServiceLine | null>(null);

  const slQuery = useGetServiceLine(serviceLineId);
  const posQuery = useListPositionsInServiceLine(serviceLineId, {
    ...(posStatusFilter ? { status: posStatusFilter as 'ACTIVE' | 'INACTIVE' } : undefined),
    ...(posCursor ? { cursor: posCursor } : undefined),
  });

  const deleteMutation = useSoftDeletePosition();
  const discontinueMutation = useDiscontinueServiceLine();

  // Orval wraps response: query.data is { data, status, headers }; body lives at .data
  const serviceLine: ServiceLine | undefined = slQuery.data?.data as ServiceLine | undefined;
  const posPage = posQuery.data?.data as ListPositionsInServiceLine200 | undefined;
  const positions: Position[] = (posPage?.data ?? []) as Position[];
  const posHasMore = posPage?.has_more ?? false;
  const posNextCursor = posPage?.next_cursor ?? null;

  function handleEditPosition(p: Position) {
    setEditingPosition(p);
    setPosModalOpen(true);
  }

  async function confirmDeletePosition() {
    if (!deleteTarget) return;
    try {
      await deleteMutation.mutateAsync({ positionId: deleteTarget.id });
      toast({ tone: 'success', title: t('serviceLines.deletePositionConfirm.confirm') });
      setDeleteTarget(null);
      void posQuery.refetch();
    } catch (err) {
      const { message } = classifyError(err);
      toast({ tone: 'error', title: t(message) });
    }
  }

  async function confirmDiscontinueSL() {
    if (!discontinueSLTarget) return;
    try {
      await discontinueMutation.mutateAsync({ serviceLineId: discontinueSLTarget.id });
      toast({ tone: 'success', title: t('serviceLines.menuDiscontinue') });
      setDiscontinueSLTarget(null);
      void slQuery.refetch();
    } catch (err) {
      const { message } = classifyError(err);
      toast({ tone: 'error', title: t(message) });
    }
  }

  function handlePosNext() {
    if (posNextCursor) {
      setPrevPosCursors((p) => [...p, posCursor ?? '']);
      setPosCursor(posNextCursor);
    }
  }

  function handlePosPrev() {
    const prev = prevPosCursors[prevPosCursors.length - 1];
    setPrevPosCursors((p) => p.slice(0, -1));
    setPosCursor(prev || undefined);
  }

  // Position table columns
  const posColumns: Column<Position>[] = [
    {
      id: 'name',
      header: (
        <span className="text-[11px] font-semibold uppercase tracking-[0.5px] text-text-3">
          {t('serviceLines.detail.colPositionName')}
        </span>
      ),
      width: 360,
      cell: (p: Position) => (
        <div className="flex items-center gap-3 py-[14px] pl-4">
          <div
            className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-surface-2"
            aria-hidden
          >
            <Contact className="size-4 text-text-2" aria-hidden />
          </div>
          <span className="text-[14px] font-medium text-text">{p.name}</span>
        </div>
      ),
    },
    {
      id: 'alias',
      header: (
        <span className="text-[11px] font-semibold uppercase tracking-[0.5px] text-text-3">
          {t('serviceLines.detail.colAlias')}
        </span>
      ),
      width: 180,
      cell: (p: Position) => (
        <span className="py-[14px] px-4 font-mono text-[12px] text-text-2">{p.alias ?? '—'}</span>
      ),
    },
    {
      id: 'status',
      header: (
        <span className="text-[11px] font-semibold uppercase tracking-[0.5px] text-text-3">
          {t('serviceLines.detail.colStatus')}
        </span>
      ),
      width: 140,
      cell: (p: Position) => (
        <div className="py-[14px] px-4">
          <StatusBadge tone={p.status === PositionStatus.ACTIVE ? 'ok' : 'bad'} dot>
            {p.status === PositionStatus.ACTIVE
              ? t('serviceLines.statusActive')
              : t('serviceLines.statusInactive')}
          </StatusBadge>
        </div>
      ),
    },
  ];

  // --- Loading / error for the service line header ---
  if (slQuery.isLoading) {
    return <StateView kind="loading" title={t('common.loading')} className="mt-8" />;
  }

  if (slQuery.isError) {
    const { kind } = classifyError(slQuery.error);
    if (kind === 'not-found') {
      return (
        <StateView
          kind="error"
          title={t('serviceLines.errorTitle')}
          description={t('errors.notFound')}
        />
      );
    }
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
        onRetry={() => void slQuery.refetch()}
        retryLabel={t('common.retry')}
      />
    );
  }

  if (!serviceLine) return null;

  return (
    <div className="flex flex-col gap-4">
      {/* Back link (.pen BackRow gk9pH) */}
      <Link
        to="/service-lines"
        className="flex items-center gap-[7px] self-start text-[13px] font-medium text-text-2 hover:underline focus-visible:outline-none focus-visible:underline"
      >
        <ArrowLeft className="size-4" aria-hidden />
        {t('serviceLines.detail.backLabel')}
      </Link>

      {/* Header card (.pen HeaderCard spCWM) */}
      <div className="flex items-center justify-between rounded-xl border border-border bg-surface p-5">
        <div className="flex items-center gap-4">
          <div
            className="flex size-[46px] shrink-0 items-center justify-center rounded-[10px] bg-warn-bg"
            aria-hidden
          >
            <Plus className="size-5 text-warn-tx" aria-hidden />
          </div>
          <div className="flex flex-col gap-0.5">
            <h1 className="text-[20px] font-semibold text-text">{serviceLine.name}</h1>
            <StatusBadge tone={serviceLine.status === ServiceLineStatus.ACTIVE ? 'ok' : 'bad'} dot>
              {serviceLine.status === ServiceLineStatus.ACTIVE
                ? t('serviceLines.statusActive')
                : t('serviceLines.statusInactive')}
            </StatusBadge>
          </div>
        </div>

        <div className="flex items-center gap-[10px]">
          <Button type="button" variant="secondary" onClick={() => setEditSLOpen(true)}>
            <Pencil className="size-[14px]" aria-hidden />
            {t('serviceLines.detail.editButton')}
          </Button>
          {serviceLine.status === ServiceLineStatus.ACTIVE && (
            <Button
              type="button"
              variant="destructive"
              onClick={() => setDiscontinueSLTarget(serviceLine)}
            >
              {t('serviceLines.detail.discontinueButton')}
            </Button>
          )}
        </div>
      </div>

      {/* Positions card (.pen PositionsCard HbzGC) */}
      <div className="flex flex-col overflow-hidden rounded-xl border border-border bg-surface">
        {/* Positions header (.pen PosHeader nivkY) */}
        <div className="flex items-center justify-between border-b border-border-soft px-5 py-[18px]">
          <div className="flex flex-col gap-0.5">
            <h2 className="text-[15px] font-semibold text-text">
              {t('serviceLines.detail.positionsTitle', { name: serviceLine.name })}
            </h2>
            <p className="text-[12px] text-text-2">{t('serviceLines.detail.positionsSubtitle')}</p>
          </div>
          <Button
            type="button"
            onClick={() => {
              setEditingPosition(null);
              setPosModalOpen(true);
            }}
          >
            <Plus className="size-[14px]" aria-hidden />
            {t('serviceLines.detail.addPositionButton')}
          </Button>
        </div>

        {/* Status filter bar */}
        <div className="flex items-center gap-[10px] border-b border-border-soft px-[18px] py-[12px]">
          <FilterSelect
            value={posStatusFilter}
            onChange={(e) => {
              setPosStatusFilter(e.target.value);
              setPosCursor(undefined);
              setPrevPosCursors([]);
            }}
          >
            <option value="">{t('serviceLines.filterStatus')}</option>
            <option value="ACTIVE">{t('serviceLines.filterStatusActive')}</option>
            <option value="INACTIVE">{t('serviceLines.filterStatusInactive')}</option>
          </FilterSelect>
        </div>

        {/* Positions DataTable (.pen PosTHead VrnDV + rows) */}
        <DataTable
          columns={posColumns}
          data={positions}
          getRowId={(p) => p.id}
          isLoading={posQuery.isLoading}
          skeletonRows={4}
          aria-label={t('serviceLines.detail.positionsTitle', { name: serviceLine.name })}
          empty={
            posStatusFilter ? (
              <EmptyState
                variant="filtered"
                title={t('serviceLines.detail.posFilteredTitle')}
                description={t('serviceLines.detail.posFilteredBody')}
              />
            ) : (
              <EmptyState
                variant="fresh"
                title={t('serviceLines.detail.posEmptyTitle')}
                description={t('serviceLines.detail.posEmptyBody')}
                action={
                  <Button
                    type="button"
                    onClick={() => {
                      setEditingPosition(null);
                      setPosModalOpen(true);
                    }}
                  >
                    <Plus className="size-[14px]" aria-hidden />
                    {t('serviceLines.detail.addPositionButton')}
                  </Button>
                }
              />
            )
          }
          rowActions={(p) => (
            <PosRowMenu
              position={p}
              onEdit={handleEditPosition}
              onDelete={(pos) => setDeleteTarget(pos)}
            />
          )}
          footer={
            <CursorPagination
              rangeLabel={t('serviceLines.resultRange', {
                from: 1,
                to: positions.length,
                total: positions.length,
              })}
              hasPrev={prevPosCursors.length > 0}
              hasNext={posHasMore}
              onPrev={handlePosPrev}
              onNext={handlePosNext}
              prevLabel={t('common.prev')}
              nextLabel={t('common.next')}
            />
          }
        />
      </div>

      {/* Modals */}
      {serviceLine && (
        <EditServiceLineModal
          open={editSLOpen}
          serviceLine={serviceLine}
          onClose={() => setEditSLOpen(false)}
          onSuccess={() => void slQuery.refetch()}
        />
      )}

      <AddEditPositionModal
        open={posModalOpen}
        serviceLineId={serviceLineId}
        serviceLineName={serviceLine.name}
        editing={editingPosition}
        onClose={() => setPosModalOpen(false)}
        onSuccess={() => void posQuery.refetch()}
      />

      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(v) => !v && setDeleteTarget(null)}
        icon={TriangleAlert}
        tone="danger"
        confirmTone="danger"
        title={t('serviceLines.deletePositionConfirm.title')}
        description={t('serviceLines.deletePositionConfirm.body')}
        confirmLabel={t('serviceLines.deletePositionConfirm.confirm')}
        cancelLabel={t('common.cancel')}
        onConfirm={confirmDeletePosition}
        loading={deleteMutation.isPending}
      />

      <ConfirmDialog
        open={discontinueSLTarget !== null}
        onOpenChange={(v) => !v && setDiscontinueSLTarget(null)}
        icon={TriangleAlert}
        tone="danger"
        confirmTone="danger"
        title={t('serviceLines.discontinueConfirm.title')}
        description={t('serviceLines.discontinueConfirm.body')}
        confirmLabel={t('serviceLines.discontinueConfirm.confirm')}
        cancelLabel={t('common.cancel')}
        onConfirm={confirmDiscontinueSL}
        loading={discontinueMutation.isPending}
      />
    </div>
  );
}
