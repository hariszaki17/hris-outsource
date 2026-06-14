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
  type DeactivateEmployeeBodyReason,
  type Employee,
  EmployeeStatus,
  useDeactivateEmployee,
  useReactivateEmployee,
} from '@swp/api-client/e2';
import { ConfirmDialog, FilterSelect, FormField, StatusBadge, useToast } from '@swp/ui';
import {
  Briefcase,
  Check,
  Copy,
  Eye,
  KeyRound,
  Pencil,
  TriangleAlert,
  UserCheck,
  UserMinus,
} from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
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
          <UserMinus className="size-4 shrink-0 text-bad-tx" aria-hidden />
        ) : (
          <UserCheck className="size-4 shrink-0 text-ok-tx" aria-hidden />
        )}
        <span className={`font-medium ${isActive ? 'text-bad-tx' : 'text-ok-tx'}`}>
          {isActive ? t('menuOffboard') : t('menuReactivate')}
        </span>
      </button>
    </div>
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

// ---------------------------------------------------------------------------
// OffboardEmployeeConfirm — F2.7 employment-end + session revocation
//   Reason (OB-3 enum) · note (required for cause) · effective date (OB-7).
//   NOTE: wired to the existing :deactivate mock until the structured
//   `:offboard` endpoint lands (E2 F2.7, P4/BE). The reason/note/date payload
//   is captured client-side for the UI flow; backend persistence follows.
// ---------------------------------------------------------------------------

// MVP offboard reasons — the 4 values the backend agreement closed_reason CHECK
// supports (F2.7 OB-1). RETIRED/DECEASED/ABSCONDED are deferred (need a CHECK
// migration). These map 1:1 to DeactivateEmployeeBodyReason.
const OFFBOARD_REASONS = ['END_OF_TERM', 'RESIGNED', 'TERMINATED', 'OTHER'] as const;
export type OffboardReason = (typeof OFFBOARD_REASONS)[number];

// A note is mandatory when the reason is adverse / catch-all (OB-3).
const NOTE_REQUIRED: Record<OffboardReason, boolean> = {
  END_OF_TERM: false,
  RESIGNED: false,
  TERMINATED: true,
  OTHER: true,
};

// reason → StatusBadge tone (design-system semantic colors only).
const REASON_TONE: Record<OffboardReason, 'neutral' | 'bad' | 'warn' | 'info'> = {
  END_OF_TERM: 'neutral',
  RESIGNED: 'neutral',
  TERMINATED: 'bad',
  OTHER: 'neutral',
};

export interface OffboardEmployeeConfirmProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  employee: Employee | null;
  onDone: () => void;
  /** Pre-seed the reason (e.g. END_OF_TERM when launched from a contract-expiry decision). */
  defaultReason?: OffboardReason;
}

export function OffboardEmployeeConfirm({
  open,
  onOpenChange,
  employee,
  onDone,
  defaultReason,
}: OffboardEmployeeConfirmProps) {
  const { t } = useTranslation('employees');
  const { toast } = useToast();
  const mutation = useDeactivateEmployee();

  const [reason, setReason] = useState<OffboardReason | ''>(defaultReason ?? '');
  const [note, setNote] = useState('');

  // Reset the form each time the dialog opens.
  useEffect(() => {
    if (open) {
      setReason(defaultReason ?? '');
      setNote('');
    }
  }, [open, defaultReason]);

  const noteRequired = reason ? NOTE_REQUIRED[reason] : false;
  const invalid = !reason || (noteRequired && note.trim() === '');

  async function handleConfirm() {
    if (!employee || invalid || !reason) return;
    try {
      // F2.7 OB-1: offboard is immediate (effective-dated scheduling is deferred).
      // The reason drives the traceable agreement+placement cascade server-side.
      await mutation.mutateAsync({
        employeeId: employee.id,
        data: {
          reason: reason as DeactivateEmployeeBodyReason,
          note: note.trim() || undefined,
        },
      });
      toast({ tone: 'success', title: t('offboardSuccess') });
      onDone();
    } catch (err) {
      const { message } = classifyError(err);
      toast({ tone: 'error', title: t('offboardError'), description: message });
    }
  }

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      icon={UserMinus}
      tone="danger"
      confirmTone="danger"
      size="md"
      title={t('offboardTitle')}
      description={
        employee ? t('offboardSubtitle', { name: employee.full_name }) : t('offboardTitle')
      }
      confirmLabel={t('offboardConfirm')}
      cancelLabel={t('cancel')}
      loading={mutation.isPending}
      confirmDisabled={invalid}
      onConfirm={handleConfirm}
    >
      <div className="flex flex-col gap-3.5">
        <FormField label={t('offboardReasonLabel')} htmlFor="offboard-reason" required>
          <FilterSelect
            id="offboard-reason"
            value={reason}
            onChange={(e) => setReason(e.target.value as OffboardReason)}
          >
            <option value="" disabled>
              {t('offboardReasonPlaceholder')}
            </option>
            {OFFBOARD_REASONS.map((r) => (
              <option key={r} value={r}>
                {t(`offboardReason${r}`)}
              </option>
            ))}
          </FilterSelect>
        </FormField>

        <FormField
          label={noteRequired ? t('offboardNoteLabel') : t('offboardNoteLabelOptional')}
          htmlFor="offboard-note"
          required={noteRequired}
        >
          <textarea
            id="offboard-note"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            rows={3}
            placeholder={t('offboardNotePlaceholder')}
            className="w-full resize-none rounded-lg border border-border bg-surface px-3 py-2 text-[13px] text-text placeholder:text-text-3 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          />
        </FormField>

        {reason && (
          <div className="flex items-center gap-2 border-t border-border-soft pt-3">
            <span className="text-[12px] text-text-3">{t('offboardReasonLabel')}:</span>
            <StatusBadge dot tone={REASON_TONE[reason]}>
              {t(`offboardReason${reason}`)}
            </StatusBadge>
          </div>
        )}
      </div>
    </ConfirmDialog>
  );
}

// ---------------------------------------------------------------------------
// TempPasswordModal — EP-3 show-once temporary password (GitHub-secret style).
//   Displayed once after provisioning a login. Copy now; it is never shown
//   again — regenerate if lost.
// ---------------------------------------------------------------------------

export interface TempPasswordModalProps {
  open: boolean;
  onClose: () => void;
  email?: string;
  password: string;
}

export function TempPasswordModal({ open, onClose, email, password }: TempPasswordModalProps) {
  const { t } = useTranslation('employees');
  const { toast } = useToast();
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (open) setCopied(false);
  }, [open]);

  function copy() {
    void navigator.clipboard?.writeText(password);
    setCopied(true);
    toast({ tone: 'success', title: t('tempPwCopied') });
  }

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={(o) => {
        if (!o) onClose();
      }}
      icon={KeyRound}
      tone="brand"
      confirmTone="primary"
      size="md"
      title={t('tempPwTitle')}
      description={email ? t('tempPwDesc', { email }) : t('tempPwDescNoEmail')}
      confirmLabel={t('tempPwDone')}
      cancelLabel={t('cancel')}
      onConfirm={async () => onClose()}
    >
      <div className="flex flex-col gap-2">
        <div className="flex items-center gap-2 rounded-lg border border-border bg-surface-2 px-3 py-2.5">
          <code className="flex-1 select-all break-all font-mono text-[14px] text-text">
            {password}
          </code>
          <button
            type="button"
            aria-label={t('tempPwCopy')}
            title={t('tempPwCopy')}
            onClick={copy}
            className="flex size-8 shrink-0 items-center justify-center rounded-md text-text-2 hover:bg-surface"
          >
            {copied ? (
              <Check className="size-4 text-ok-tx" aria-hidden />
            ) : (
              <Copy className="size-4" aria-hidden />
            )}
          </button>
        </div>
        <p className="flex items-center gap-1.5 text-[12px] text-warn-tx">
          <TriangleAlert className="size-3.5 shrink-0" aria-hidden />
          {t('tempPwWarn')}
        </p>
      </div>
    </ConfirmDialog>
  );
}
