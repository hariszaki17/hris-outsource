/**
 * E6 · "Hak Cuti" — per-employee leave entitlement assignment (HR/admin).
 *
 * PRD: leave-entitlement-assignment §7.2. Mounted on the employee detail screen (E2) as a tab.
 * The assignment IS the eligibility decision (gates dropped, ELE-8) — INV-7 superseded: only an
 * active `employee_leave_entitlement` makes a type requestable (ELE-1).
 *
 * Table of the employee's ASSIGNED types (useListEmployeeLeaveEntitlements):
 *   Jenis Cuti · Kuota (inline-editable entitled_days; NULL = "Sesuai ketentuan") · Reset · [remove]
 * "Tambah Jenis Cuti" picks an un-assigned catalog type (useListLeaveTypes) → assign (ELE-6).
 * Remove deactivates (active=false) via a ConfirmDialog (ELE-6).
 *
 * Writes invalidate/refetch the list on success (ENGINEERING B3 — invariant-ish write, reconcile
 * on success, no optimistic UI). All copy via i18n; all dates via the @swp/shared TZ layer.
 */
import { ApiError } from '@swp/api-client';
import {
  type LeaveEntitlement,
  type LeaveType,
  LeaveTypeStatus,
  useAssignEmployeeLeaveEntitlement,
  useListEmployeeLeaveEntitlements,
  useListLeaveTypes,
  useRemoveEmployeeLeaveEntitlement,
  useUpdateEmployeeLeaveEntitlement,
} from '@swp/api-client/e6';
import { resetCadenceKey } from '@swp/shared/leave';
import {
  Button,
  type Column,
  ConfirmDialog,
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
  useToast,
} from '@swp/ui';
import { Check, Plus, Trash2, X } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** PER_EVENT / UNCAPPED types carry no fixed day quota — HR toggles them on, entitled_days is null. */
function isUncapped(capBasis: string | undefined): boolean {
  return capBasis === 'PER_EVENT' || capBasis === 'UNCAPPED';
}

/**
 * The standing per-period quota to prefill when HR assigns a catalog type: the annual pool size for
 * ANNUAL_POOL, else the type's explicit cap_value (statutory days/occurrences). null = uncapped
 * ("sesuai ketentuan") — HR may still enter an arbitrary number.
 */
function standingQuota(lt: LeaveType): number | null {
  if (lt.cap_basis === 'ANNUAL_POOL') return lt.default_annual_quota ?? lt.cap_value ?? null;
  return lt.cap_value ?? null;
}

// ---------------------------------------------------------------------------
// Section (the "Hak Cuti" tab body)
// ---------------------------------------------------------------------------

export function EmployeeLeaveEntitlementsSection({
  employeeId,
  canEdit,
}: {
  employeeId: string;
  /** RBAC defense-in-depth (A2): SL/read-only hides write actions. Server is the real gate. */
  canEdit: boolean;
}) {
  const { t } = useTranslation('leaveEntitlements');
  const { toast } = useToast();

  const listQ = useListEmployeeLeaveEntitlements(employeeId);
  const update = useUpdateEmployeeLeaveEntitlement();
  const remove = useRemoveEmployeeLeaveEntitlement();

  const [addOpen, setAddOpen] = useState(false);
  const [removing, setRemoving] = useState<LeaveEntitlement | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editVal, setEditVal] = useState('');

  const entitlements = (listQ.data?.data as { data?: LeaveEntitlement[] } | undefined)?.data ?? [];
  // Only active assignments are shown; removal sets active=false (ELE-6).
  const rows = entitlements.filter((e) => e.active);
  const assignedTypeIds = new Set(entitlements.map((e) => e.leave_type_id));

  function startEdit(row: LeaveEntitlement) {
    setEditingId(row.leave_type_id);
    setEditVal(row.entitled_days == null ? '' : String(row.entitled_days));
  }

  async function saveEdit(row: LeaveEntitlement) {
    const parsed = editVal.trim() === '' ? null : Number(editVal);
    if (parsed != null && (!Number.isInteger(parsed) || parsed < 0)) {
      toast({ tone: 'error', title: t('quotaInvalid') });
      return;
    }
    try {
      await update.mutateAsync({
        employeeId,
        leaveTypeId: row.leave_type_id,
        data: { entitled_days: parsed },
      });
      setEditingId(null);
      await listQ.refetch();
      toast({ tone: 'success', title: t('quotaSaved') });
    } catch (e) {
      toast({ tone: 'error', title: errorTitle(e, t) });
    }
  }

  async function confirmRemove() {
    if (!removing) return;
    try {
      await remove.mutateAsync({ employeeId, leaveTypeId: removing.leave_type_id });
      setRemoving(null);
      await listQ.refetch();
      toast({ tone: 'success', title: t('removeSuccess') });
    } catch (e) {
      toast({ tone: 'error', title: errorTitle(e, t) });
    }
  }

  const columns: Column<LeaveEntitlement>[] = [
    {
      id: 'type',
      header: t('colType'),
      width: 280,
      cell: (r) => (
        <div className="flex items-center gap-2">
          <span
            className="size-2.5 shrink-0 rounded-full"
            style={{ backgroundColor: r.color || 'var(--color-text-3)' }}
            aria-hidden
          />
          <span className="font-medium text-text">{r.name ?? r.leave_type_id}</span>
          {r.code && <span className="font-mono text-[11px] text-text-3">{r.code}</span>}
        </div>
      ),
    },
    {
      id: 'quota',
      header: t('colQuota'),
      width: 200,
      cell: (r) => {
        // NULL entitled_days on event/uncapped types = no day quota → "Sesuai ketentuan", not an input.
        if (isUncapped(r.cap_basis)) {
          return <span className="text-[13px] text-text-2">{t('uncapped')}</span>;
        }
        if (editingId === r.leave_type_id) {
          return (
            <div className="flex items-center gap-1.5">
              <Input
                type="number"
                min={0}
                step={1}
                value={editVal}
                autoFocus
                className="w-20"
                aria-label={t('colQuota')}
                onChange={(e) => setEditVal(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') void saveEdit(r);
                  if (e.key === 'Escape') setEditingId(null);
                }}
              />
              <button
                type="button"
                aria-label={t('save')}
                disabled={update.isPending}
                className="flex size-7 items-center justify-center rounded-md text-ok-tx hover:bg-surface-2 disabled:opacity-50"
                onClick={() => void saveEdit(r)}
              >
                <Check className="size-4" aria-hidden />
              </button>
              <button
                type="button"
                aria-label={t('cancel')}
                className="flex size-7 items-center justify-center rounded-md text-text-3 hover:bg-surface-2"
                onClick={() => setEditingId(null)}
              >
                <X className="size-4" aria-hidden />
              </button>
            </div>
          );
        }
        return canEdit ? (
          <button
            type="button"
            className="rounded-md px-2 py-1 text-[13px] font-medium text-text tabular-nums hover:bg-surface-2"
            onClick={() => startEdit(r)}
          >
            {r.entitled_days == null ? t('uncapped') : t('daysValue', { count: r.entitled_days })}
          </button>
        ) : (
          <span className="text-[13px] text-text tabular-nums">
            {r.entitled_days == null ? t('uncapped') : t('daysValue', { count: r.entitled_days })}
          </span>
        );
      },
    },
    {
      id: 'reset',
      header: t('colReset'),
      cell: (r) => (
        <span className="text-[13px] text-text-2">
          {t(`resetCadence.${resetCadenceKey(r.cap_basis ?? 'UNCAPPED')}`)}
        </span>
      ),
    },
    ...(canEdit
      ? [
          {
            id: 'actions',
            header: '',
            width: 64,
            align: 'right' as const,
            cell: (r: LeaveEntitlement) => (
              <button
                type="button"
                aria-label={t('remove')}
                className="flex size-8 items-center justify-center rounded-md text-text-3 hover:bg-surface-2 hover:text-bad-tx"
                onClick={() => setRemoving(r)}
              >
                <Trash2 className="size-4" aria-hidden />
              </button>
            ),
          },
        ]
      : []),
  ];

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <p className="text-[13px] text-text-3">{t('sectionHint')}</p>
        {canEdit && (
          <Button variant="primary" size="sm" onClick={() => setAddOpen(true)}>
            <Plus className="size-4" aria-hidden />
            {t('addBtn')}
          </Button>
        )}
      </div>

      {listQ.isError ? (
        <StateView kind="error" title={t('errorTitle')} onRetry={() => void listQ.refetch()} />
      ) : (
        <DataTable
          aria-label={t('tableLabel')}
          columns={columns}
          data={rows}
          getRowId={(r) => r.id}
          isLoading={listQ.isLoading}
          skeletonRows={4}
          empty={<EmptyState variant="fresh" title={t('empty')} description={t('emptyHint')} />}
        />
      )}

      {addOpen && (
        <AddEntitlementModal
          employeeId={employeeId}
          assignedTypeIds={assignedTypeIds}
          onOpenChange={setAddOpen}
          onSuccess={() => void listQ.refetch()}
        />
      )}

      {removing && (
        <ConfirmDialog
          open
          onOpenChange={(o) => {
            if (!o) setRemoving(null);
          }}
          icon={Trash2}
          tone="danger"
          confirmTone="danger"
          title={t('removeTitle')}
          description={t('removeBody', { name: removing.name ?? removing.leave_type_id })}
          cancelLabel={t('cancel')}
          confirmLabel={t('remove')}
          loading={remove.isPending}
          onConfirm={() => void confirmRemove()}
        />
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// "Tambah Jenis Cuti" — assign a catalog type not yet assigned (ELE-6)
// ---------------------------------------------------------------------------

function AddEntitlementModal({
  employeeId,
  assignedTypeIds,
  onOpenChange,
  onSuccess,
}: {
  employeeId: string;
  assignedTypeIds: Set<string>;
  onOpenChange: (open: boolean) => void;
  onSuccess: () => void;
}) {
  const { t } = useTranslation('leaveEntitlements');
  const { toast } = useToast();

  const typesQ = useListLeaveTypes();
  const assign = useAssignEmployeeLeaveEntitlement();

  const [typeId, setTypeId] = useState('');
  const [days, setDays] = useState('');
  const [note, setNote] = useState('');

  const allTypes = (typesQ.data?.data as { data?: LeaveType[] } | undefined)?.data ?? [];
  // Every ACTIVE catalog type the employee is NOT already assigned. The document-required hide is
  // an AGENT-request-picker rule (PRD §10), not an HR-assign rule — HR may grant any leave type.
  const available = allTypes.filter(
    (lt) => lt.status === LeaveTypeStatus.ACTIVE && !assignedTypeIds.has(lt.id),
  );

  const selected = available.find((lt) => lt.id === typeId);
  // The quota input is ALWAYS editable: capped types (ANNUAL_POOL / explicit cap_value) prefill
  // their standing quota from the catalog (HR may override); uncapped types ("sesuai ketentuan")
  // start empty but HR may still type an arbitrary number. null entitled_days = honour the type's
  // own cap (event/uncapped) — the backend falls back to cap_value.
  const hasStandingCap = selected != null && standingQuota(selected) != null;

  async function onSubmit() {
    if (!typeId) {
      toast({ tone: 'error', title: t('pickTypeFirst') });
      return;
    }
    const parsed = days.trim() === '' ? null : Number(days);
    if (parsed != null && (!Number.isInteger(parsed) || parsed < 0)) {
      toast({ tone: 'error', title: t('quotaInvalid') });
      return;
    }
    try {
      await assign.mutateAsync({
        employeeId,
        data: {
          leave_type_id: typeId,
          entitled_days: parsed,
          note: note.trim() || undefined,
        },
      });
      toast({ tone: 'success', title: t('addSuccess') });
      onOpenChange(false);
      onSuccess();
    } catch (e) {
      toast({ tone: 'error', title: errorTitle(e, t) });
    }
  }

  return (
    <Modal open onOpenChange={onOpenChange} size="lg" className="w-[520px]">
      <ModalHeader icon={Plus} tone="brand" title={t('addTitle')} closeLabel={t('cancel')} />
      <ModalBody className="gap-5">
        {/* Type picker */}
        <div className="flex flex-col gap-1.5">
          <span className="text-sm font-medium text-text-2">
            {t('fieldType')}
            <span className="ml-0.5 text-bad">*</span>
          </span>
          {typesQ.isLoading ? (
            <StateView kind="loading" title={t('loading')} />
          ) : typesQ.isError ? (
            <StateView kind="error" title={t('errorTitle')} onRetry={() => void typesQ.refetch()} />
          ) : available.length === 0 ? (
            <p className="text-[13px] text-text-3">{t('allAssigned')}</p>
          ) : (
            <FilterSelect
              containerClassName="w-full"
              value={typeId}
              aria-label={t('fieldType')}
              onChange={(e) => {
                const id = e.target.value;
                setTypeId(id);
                const lt = available.find((x) => x.id === id);
                const q = lt ? standingQuota(lt) : null;
                setDays(q != null ? String(q) : '');
              }}
            >
              <option value="" disabled>
                {t('pickTypeFirst')}
              </option>
              {available.map((lt) => (
                <option key={lt.id} value={lt.id}>
                  {lt.name}
                  {lt.code ? ` (${lt.code})` : ''}
                </option>
              ))}
            </FilterSelect>
          )}
        </div>

        {/* Quota — always editable; capped types prefill, uncapped start empty ("sesuai ketentuan") */}
        <FormField
          label={t('fieldQuota')}
          htmlFor="ele-days"
          hint={hasStandingCap ? t('quotaHint') : t('uncapped')}
        >
          <Input
            id="ele-days"
            type="number"
            min={0}
            step={1}
            value={days}
            placeholder={t('uncapped')}
            onChange={(e) => setDays(e.target.value)}
          />
        </FormField>

        {/* Optional HR note */}
        <FormField label={t('fieldNote')} htmlFor="ele-note">
          <textarea
            id="ele-note"
            value={note}
            rows={2}
            onChange={(e) => setNote(e.target.value)}
            className="w-full resize-none rounded-md border border-border bg-surface px-3 py-2 text-sm text-text placeholder:text-text-3 focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2"
          />
        </FormField>
      </ModalBody>
      <ModalFooter>
        <Button
          variant="secondary"
          size="sm"
          disabled={assign.isPending}
          onClick={() => onOpenChange(false)}
        >
          {t('cancel')}
        </Button>
        <Button
          variant="primary"
          size="sm"
          disabled={assign.isPending || !typeId}
          onClick={() => void onSubmit()}
        >
          {t('addConfirm')}
        </Button>
      </ModalFooter>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// Error mapping (ENGINEERING B1)
// ---------------------------------------------------------------------------

function errorTitle(e: unknown, t: (k: string) => string): string {
  if (e instanceof ApiError) {
    // INV-6 no-negative: an adjustment cannot drop entitled below used+pending (ELE-9).
    if (e.code === 'QUOTA_BELOW_USAGE' || e.code === 'UNPROCESSABLE_ENTITY') {
      return t('quotaBelowUsage');
    }
  }
  return t('errorGeneric');
}
