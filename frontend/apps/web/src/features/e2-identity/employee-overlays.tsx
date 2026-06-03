/**
 * E2 · Karyawan — overlay layer for row actions and employee mutations.
 *
 * .pen frames:
 *   tNMfN  Row-Kebab Popover (Lihat Detail, Edit Profil, Penempatan, Reset Password, Nonaktifkan)
 *   V4LG8  comp/ModalDestructive (Deactivate / Reactivate confirms)
 *
 * ENGINEERING.md A2 — client RBAC is defense-in-depth; API is the gate.
 */

import { classifyError } from '@/lib/api-error.ts';
import {
  type Employee,
  EmployeeStatus,
  useDeactivateEmployee,
  useReactivateEmployee,
} from '@swp/api-client/e2';
import { ConfirmDialog, useToast } from '@swp/ui';
import { Briefcase, Eye, KeyRound, Pencil, UserCheck, UserX } from 'lucide-react';
import { useEffect, useRef } from 'react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// EmployeeRowActionsMenu — row-kebab popover (.pen tNMfN)
// ---------------------------------------------------------------------------

export interface EmployeeRowActionsMenuProps {
  employee: Employee;
  open: boolean;
  anchorRef: React.RefObject<HTMLElement | null>;
  onClose: () => void;
  onView: () => void;
  onEdit: () => void;
  onToggleStatus: () => void;
}

export function EmployeeRowActionsMenu({
  employee,
  open,
  anchorRef,
  onClose,
  onView,
  onEdit,
  onToggleStatus,
}: EmployeeRowActionsMenuProps) {
  const { t } = useTranslation('employees');
  const panelRef = useRef<HTMLDivElement>(null);

  // Close on outside click
  useEffect(() => {
    if (!open) return;
    function handle(e: MouseEvent) {
      const target = e.target as Node;
      if (
        panelRef.current &&
        !panelRef.current.contains(target) &&
        anchorRef.current &&
        !anchorRef.current.contains(target)
      ) {
        onClose();
      }
    }
    document.addEventListener('mousedown', handle);
    return () => document.removeEventListener('mousedown', handle);
  }, [open, onClose, anchorRef]);

  // Close on Escape
  useEffect(() => {
    if (!open) return;
    function handle(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose();
    }
    document.addEventListener('keydown', handle);
    return () => document.removeEventListener('keydown', handle);
  }, [open, onClose]);

  if (!open) return null;

  const isActive = employee.status === EmployeeStatus.ACTIVE;

  const baseItem =
    'flex w-full items-center gap-[10px] rounded-lg px-3 py-[10px] text-[13px] text-text hover:bg-surface-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring';

  function action(fn: () => void) {
    fn();
    onClose();
  }

  return (
    <div
      ref={panelRef}
      role="menu"
      aria-label={t('rowActionsMenuLabel')}
      className="absolute right-0 z-50 w-[240px] rounded-xl border border-border bg-surface p-1.5 shadow-overlay"
      style={{ top: '100%' }}
    >
      {/* Lihat Detail */}
      <button type="button" role="menuitem" className={baseItem} onClick={() => action(onView)}>
        <Eye className="size-4 shrink-0 text-text-2" aria-hidden />
        <div className="flex flex-col gap-[1px]">
          <span className="font-medium">{t('menuView')}</span>
        </div>
      </button>

      {/* Edit Profil */}
      <button type="button" role="menuitem" className={baseItem} onClick={() => action(onEdit)}>
        <Pencil className="size-4 shrink-0 text-text-2" aria-hidden />
        <div className="flex flex-col gap-[1px]">
          <span className="font-medium">{t('menuEdit')}</span>
        </div>
      </button>

      {/* Penempatan (E3 deep-link — note only in E2) */}
      <button
        type="button"
        role="menuitem"
        className={baseItem}
        onClick={() => {
          onClose();
          // E3 deep-link — placeholder until E3 route is built
        }}
      >
        <Briefcase className="size-4 shrink-0 text-text-2" aria-hidden />
        <div className="flex flex-col gap-[1px]">
          <span className="font-medium">{t('menuPenempatan')}</span>
          <span className="text-[11px] text-text-3">{t('menuPenempatanSub')}</span>
        </div>
      </button>

      {/* Reset Password (E1 invite) */}
      <button
        type="button"
        role="menuitem"
        className={baseItem}
        onClick={() => {
          onClose();
          // Triggers E1 invite — placeholder until E1 ResetPassword flow is wired
        }}
      >
        <KeyRound className="size-4 shrink-0 text-text-2" aria-hidden />
        <div className="flex flex-col gap-[1px]">
          <span className="font-medium">{t('menuResetPassword')}</span>
        </div>
      </button>

      {/* Separator */}
      <div className="my-1 h-px bg-border-soft" aria-hidden />

      {/* (Non)aktifkan */}
      <button
        type="button"
        role="menuitem"
        className={`${baseItem} ${isActive ? 'text-bad-tx' : ''}`}
        onClick={() => action(onToggleStatus)}
      >
        {isActive ? (
          <UserX className="size-4 shrink-0 text-bad-tx" aria-hidden />
        ) : (
          <UserCheck className="size-4 shrink-0 text-ok-tx" aria-hidden />
        )}
        <span className={`font-medium ${isActive ? 'text-bad-tx' : 'text-ok-tx'}`}>
          {isActive ? t('menuDeactivate') : t('menuReactivate')}
        </span>
      </button>
    </div>
  );
}

// ---------------------------------------------------------------------------
// DeactivateEmployeeConfirm — ConfirmDialog (.pen comp/ModalDestructive V4LG8)
// ---------------------------------------------------------------------------

export interface DeactivateEmployeeConfirmProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  employee: Employee | null;
  onDone: () => void;
}

export function DeactivateEmployeeConfirm({
  open,
  onOpenChange,
  employee,
  onDone,
}: DeactivateEmployeeConfirmProps) {
  const { t } = useTranslation('employees');
  const { toast } = useToast();
  const mutation = useDeactivateEmployee();

  async function handleConfirm() {
    if (!employee) return;
    try {
      await mutation.mutateAsync({ employeeId: employee.id, data: {} });
      toast({ tone: 'success', title: t('deactivateSuccess') });
      onDone();
    } catch (err) {
      const { message } = classifyError(err);
      toast({ tone: 'error', title: t('deactivateError'), description: message });
    }
  }

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      icon={UserX}
      tone="danger"
      confirmTone="danger"
      title={t('deactivateTitle')}
      description={
        employee ? t('deactivateDescription', { name: employee.full_name }) : t('deactivateTitle')
      }
      confirmLabel={t('deactivateConfirm')}
      cancelLabel={t('cancel')}
      loading={mutation.isPending}
      onConfirm={handleConfirm}
    />
  );
}

// ---------------------------------------------------------------------------
// ReactivateEmployeeConfirm — ConfirmDialog (primary tone)
// ---------------------------------------------------------------------------

export interface ReactivateEmployeeConfirmProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  employee: Employee | null;
  onDone: () => void;
}

export function ReactivateEmployeeConfirm({
  open,
  onOpenChange,
  employee,
  onDone,
}: ReactivateEmployeeConfirmProps) {
  const { t } = useTranslation('employees');
  const { toast } = useToast();
  const mutation = useReactivateEmployee();

  async function handleConfirm() {
    if (!employee) return;
    try {
      await mutation.mutateAsync({ employeeId: employee.id });
      toast({ tone: 'success', title: t('reactivateSuccess') });
      onDone();
    } catch (err) {
      const { message } = classifyError(err);
      toast({ tone: 'error', title: t('reactivateError'), description: message });
    }
  }

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      icon={UserCheck}
      tone="brand"
      confirmTone="primary"
      title={t('reactivateTitle')}
      description={
        employee ? t('reactivateDescription', { name: employee.full_name }) : t('reactivateTitle')
      }
      confirmLabel={t('reactivateConfirm')}
      cancelLabel={t('cancel')}
      loading={mutation.isPending}
      onConfirm={handleConfirm}
    />
  );
}
