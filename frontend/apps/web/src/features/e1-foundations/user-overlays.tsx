/**
 * E1 · Pengguna & Peran — overlay layer for row actions and user mutations.
 *
 * .pen frames implemented:
 *   Zjzvo  UserRowActionsMenu
 *   FGkC2  CreateUserModal
 *   y4qyuS ChangeRoleModal
 *   xmWHa  EditUserDrawer
 *   oXZNQ  SendResetConfirm
 *   cACO9  DeactivateUserConfirm / ReactivateUserConfirm
 *
 * ENGINEERING.md F1.2 · RB-1..RB-7 · INV-1.
 */

import { applyFieldErrors, classifyError } from '@/lib/api-error.ts';
import { zodResolver } from '@hookform/resolvers/zod';
import {
  Role,
  type User,
  UserStatus,
  useChangeUserRole,
  useCreateUser,
  useDeactivateUser,
  useReactivateUser,
  useSendUserPasswordReset,
  useUpdateUser,
} from '@swp/api-client/e1';
import {
  Avatar,
  Banner,
  Button,
  ConfirmDialog,
  DateText,
  Drawer,
  DrawerBody,
  DrawerFooter,
  DrawerHeader,
  FilterSelect,
  FormField,
  FormSection,
  Input,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  StatusBadge,
  Toggle,
  useToast,
} from '@swp/ui';
import {
  Check,
  Info,
  KeyRound,
  Pencil,
  TriangleAlert,
  UserCheck,
  UserCog,
  UserPlus,
  UserX,
} from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

function initials(name: string): string {
  return name
    .split(' ')
    .slice(0, 2)
    .map((p) => p[0] ?? '')
    .join('')
    .toUpperCase();
}

// Inline cn helper — not re-exporting packages/ui's cn to avoid a dep edge
function cx(...classes: (string | undefined | false | null)[]): string {
  return classes.filter(Boolean).join(' ');
}

// ---------------------------------------------------------------------------
// 1) UserRowActionsMenu  (.pen frame Zjzvo)
// ---------------------------------------------------------------------------

export interface UserRowActionsMenuProps {
  user: User;
  open: boolean;
  anchorRef: React.RefObject<HTMLElement | null>;
  onClose: () => void;
  onEdit: () => void;
  onChangeRole: () => void;
  onSendReset: () => void;
  onToggleStatus: () => void;
}

export function UserRowActionsMenu({
  user,
  open,
  anchorRef,
  onClose,
  onEdit,
  onChangeRole,
  onSendReset,
  onToggleStatus,
}: UserRowActionsMenuProps) {
  const { t } = useTranslation();
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

  const isDisabled = user.status === UserStatus.DISABLED;

  const itemBase =
    'flex w-full items-center gap-[10px] rounded-[7px] px-3 py-[10px] text-[13px] text-text hover:bg-surface-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring';

  function action(fn: () => void) {
    fn();
    onClose();
  }

  return (
    <div
      ref={panelRef}
      role="menu"
      aria-label={t('userOverlays.rowMenuLabel')}
      className="absolute z-50 w-[240px] rounded-[10px] border border-border bg-surface p-1.5 shadow-overlay"
      style={{ top: '100%', right: 0 }}
    >
      <button type="button" role="menuitem" className={itemBase} onClick={() => action(onEdit)}>
        <Pencil className="size-[15px] shrink-0 text-text-2" aria-hidden />
        {t('userOverlays.menuEdit')}
      </button>

      <button
        type="button"
        role="menuitem"
        className={itemBase}
        onClick={() => action(onChangeRole)}
      >
        <UserCog className="size-[15px] shrink-0 text-text-2" aria-hidden />
        {t('userOverlays.menuChangeRole')}
      </button>

      <button
        type="button"
        role="menuitem"
        className={itemBase}
        onClick={() => action(onSendReset)}
      >
        <KeyRound className="size-[15px] shrink-0 text-text-2" aria-hidden />
        {t('userOverlays.menuSendReset')}
      </button>

      <div className="my-1 h-px bg-border-soft" aria-hidden="true" />

      {isDisabled ? (
        <button
          type="button"
          role="menuitem"
          className={cx(itemBase, 'text-ok-tx')}
          onClick={() => action(onToggleStatus)}
        >
          <UserCheck className="size-[15px] shrink-0" aria-hidden />
          {t('userOverlays.menuReactivate')}
        </button>
      ) : (
        <button
          type="button"
          role="menuitem"
          className={cx(itemBase, 'text-bad-tx')}
          onClick={() => action(onToggleStatus)}
        >
          <UserX className="size-[15px] shrink-0" aria-hidden />
          {t('userOverlays.menuDeactivate')}
        </button>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// 2) CreateUserModal  (.pen frame FGkC2)
// ---------------------------------------------------------------------------

const createSchema = z
  .object({
    email: z.string().min(1, 'Email wajib diisi').email('Format email tidak valid'),
    display_name: z.string().optional(),
    role: z.nativeEnum(Role, { required_error: 'Peran wajib dipilih' }),
    initial_status: z.enum(['ACTIVE', 'DISABLED']).default('ACTIVE'),
    employee_id: z.string().optional(),
  })
  .refine(
    (v) => {
      if (v.role === Role.shift_leader && !v.employee_id?.trim()) return false;
      return true;
    },
    { message: 'Tautan karyawan wajib untuk peran shift_leader', path: ['employee_id'] },
  );

type CreateForm = z.infer<typeof createSchema>;

export interface CreateUserModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onDone: () => void;
}

export function CreateUserModal({ open, onOpenChange, onDone }: CreateUserModalProps) {
  const { t } = useTranslation();
  const { toast } = useToast();
  const mutation = useCreateUser();
  const [serverError, setServerError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    setError,
    watch,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<CreateForm>({
    resolver: zodResolver(createSchema),
    defaultValues: { initial_status: 'ACTIVE' },
  });

  const roleValue = watch('role');

  function handleClose() {
    reset();
    setServerError(null);
    onOpenChange(false);
  }

  function onSubmit(values: CreateForm) {
    setServerError(null);
    mutation.mutate(
      {
        data: {
          email: values.email,
          role: values.role,
          employee_id: values.employee_id ?? '',
          send_invitation_email: true,
        },
      },
      {
        onSuccess: () => {
          toast({ tone: 'success', title: t('userOverlays.createSuccess') });
          handleClose();
          onDone();
        },
        onError: (err) => {
          if (!applyFieldErrors(err, setError)) {
            const { message } = classifyError(err);
            setServerError(t(message));
          }
        },
      },
    );
  }

  const saving = isSubmitting || mutation.isPending;

  return (
    <Modal open={open} onOpenChange={handleClose} size="lg">
      <ModalHeader
        icon={UserPlus}
        tone="brand"
        title={t('userOverlays.createTitle')}
        closeLabel={t('common.close')}
      />

      <form onSubmit={handleSubmit(onSubmit)} noValidate>
        <ModalBody>
          <p className="text-[13px] text-text-2">{t('userOverlays.createSubtitle')}</p>

          {serverError && <Banner tone="bad" title={serverError} />}

          <FormSection>
            <FormField
              label={t('userOverlays.fieldEmailLogin')}
              htmlFor="cu-email"
              required
              hint={t('userOverlays.fieldEmailLoginHint')}
              error={errors.email?.message}
            >
              <Input
                id="cu-email"
                type="email"
                placeholder="nama@swp.id"
                aria-invalid={errors.email ? true : undefined}
                aria-describedby={errors.email ? 'cu-email-error' : undefined}
                disabled={saving}
                {...register('email')}
              />
            </FormField>

            <FormField
              label={t('userOverlays.fieldDisplayName')}
              htmlFor="cu-display-name"
              error={errors.display_name?.message}
            >
              <Input
                id="cu-display-name"
                placeholder={t('userOverlays.fieldDisplayNamePlaceholder')}
                disabled={saving}
                {...register('display_name')}
              />
            </FormField>
          </FormSection>

          <FormSection>
            <FormField
              label={t('userOverlays.fieldRole')}
              htmlFor="cu-role"
              required
              error={errors.role?.message}
            >
              <FilterSelect
                id="cu-role"
                aria-invalid={errors.role ? true : undefined}
                disabled={saving}
                {...register('role')}
              >
                <option value="">{t('userOverlays.fieldRolePlaceholder')}</option>
                {Object.values(Role).map((r) => (
                  <option key={r} value={r}>
                    {t(`role.${r}`)}
                  </option>
                ))}
              </FilterSelect>
            </FormField>

            <FormField
              label={t('userOverlays.fieldInitialStatus')}
              htmlFor="cu-status"
              error={errors.initial_status?.message}
            >
              <FilterSelect id="cu-status" disabled={saving} {...register('initial_status')}>
                <option value="ACTIVE">{t('users.statusActive')}</option>
                <option value="DISABLED">{t('users.statusDisabled')}</option>
              </FilterSelect>
            </FormField>
          </FormSection>

          <FormField
            label={t('userOverlays.fieldEmployeeLink')}
            htmlFor="cu-employee-id"
            hint={
              roleValue === Role.shift_leader
                ? t('userOverlays.fieldEmployeeLinkHintRequired')
                : t('userOverlays.fieldEmployeeLinkHint')
            }
            error={errors.employee_id?.message}
            span={2}
          >
            <Input
              id="cu-employee-id"
              placeholder={t('userOverlays.fieldEmployeeLinkPlaceholder')}
              aria-invalid={errors.employee_id ? true : undefined}
              disabled={saving}
              {...register('employee_id')}
            />
          </FormField>

          {/* Audit notice */}
          <div className="flex items-start gap-2 rounded-md border border-info-bd bg-info-bg px-3 py-2.5 text-[13px] text-info-tx">
            <Info className="mt-0.5 size-3.5 shrink-0" aria-hidden />
            <span>{t('userOverlays.createAuditNotice')}</span>
          </div>
        </ModalBody>

        <ModalFooter>
          <Button
            type="button"
            variant="secondary"
            size="sm"
            disabled={saving}
            onClick={handleClose}
          >
            {t('common.cancel')}
          </Button>
          <Button type="submit" variant="primary" size="sm" disabled={saving} aria-busy={saving}>
            <Check aria-hidden />
            {t('userOverlays.createSubmit')}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// 3) ChangeRoleModal  (.pen frame y4qyuS)
// ---------------------------------------------------------------------------

const changeRoleSchema = z.object({
  new_role: z.nativeEnum(Role, { required_error: 'Peran baru wajib dipilih' }),
  reason: z.string().min(1, 'Alasan wajib diisi'),
});

type ChangeRoleForm = z.infer<typeof changeRoleSchema>;

export interface ChangeRoleModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  user: User | null;
  onDone: () => void;
}

export function ChangeRoleModal({ open, onOpenChange, user, onDone }: ChangeRoleModalProps) {
  const { t } = useTranslation();
  const { toast } = useToast();
  const mutation = useChangeUserRole();
  const [serverError, setServerError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    setError,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<ChangeRoleForm>({
    resolver: zodResolver(changeRoleSchema),
  });

  function handleClose() {
    reset();
    setServerError(null);
    onOpenChange(false);
  }

  function onSubmit(values: ChangeRoleForm) {
    if (!user) return;
    setServerError(null);
    mutation.mutate(
      { userId: user.id, data: { new_role: values.new_role, reason: values.reason } },
      {
        onSuccess: () => {
          toast({ tone: 'success', title: t('userOverlays.changeRoleSuccess') });
          handleClose();
          onDone();
        },
        onError: (err) => {
          if (!applyFieldErrors(err, setError)) {
            const { message } = classifyError(err);
            setServerError(t(message));
          }
        },
      },
    );
  }

  const saving = isSubmitting || mutation.isPending;

  return (
    <Modal open={open} onOpenChange={handleClose} size="lg">
      <ModalHeader
        icon={UserCog}
        tone="info"
        title={t('userOverlays.changeRoleTitle')}
        closeLabel={t('common.close')}
      />

      <form onSubmit={handleSubmit(onSubmit)} noValidate>
        <ModalBody>
          {user && (
            <div className="flex items-center gap-3 rounded-lg border border-border-soft bg-surface-2 px-3 py-2.5">
              <Avatar initials={initials(user.full_name)} size={36} />
              <div className="flex min-w-0 flex-col gap-0.5">
                <span className="font-semibold text-sm text-text">{user.full_name}</span>
                <span className="font-mono text-xs text-text-3">
                  {user.email} · #{user.id}
                </span>
              </div>
            </div>
          )}

          {serverError && <Banner tone="bad" title={serverError} />}

          <FormSection>
            <FormField label={t('userOverlays.fieldCurrentRole')} htmlFor="cr-current-role">
              <Input
                id="cr-current-role"
                value={user ? t(`role.${user.role}`) : ''}
                readOnly
                disabled
              />
            </FormField>

            <FormField
              label={t('userOverlays.fieldNewRole')}
              htmlFor="cr-new-role"
              required
              error={errors.new_role?.message}
            >
              <FilterSelect
                id="cr-new-role"
                aria-invalid={errors.new_role ? true : undefined}
                disabled={saving}
                {...register('new_role')}
              >
                <option value="">{t('userOverlays.fieldRolePlaceholder')}</option>
                {Object.values(Role)
                  .filter((r) => r !== user?.role)
                  .map((r) => (
                    <option key={r} value={r}>
                      {t(`role.${r}`)}
                    </option>
                  ))}
              </FilterSelect>
            </FormField>
          </FormSection>

          <FormField
            label={t('userOverlays.fieldChangeReason')}
            htmlFor="cr-reason"
            required
            error={errors.reason?.message}
            span={2}
          >
            <textarea
              id="cr-reason"
              rows={3}
              placeholder={t('userOverlays.fieldChangeReasonPlaceholder')}
              aria-invalid={errors.reason ? true : undefined}
              disabled={saving}
              className="flex w-full resize-y rounded-md border border-input bg-background px-3 py-2 text-sm text-text placeholder:text-text-3 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50 aria-[invalid=true]:border-bad-bd"
              {...register('reason')}
            />
          </FormField>

          {/* Impact warning banner */}
          <div className="flex items-start gap-2 rounded-md border border-warn-bd bg-warn-bg px-3 py-2.5 text-[13px] text-warn-tx">
            <TriangleAlert className="mt-0.5 size-3.5 shrink-0" aria-hidden />
            <span>{t('userOverlays.changeRoleWarning')}</span>
          </div>
        </ModalBody>

        <ModalFooter>
          <Button
            type="button"
            variant="secondary"
            size="sm"
            disabled={saving}
            onClick={handleClose}
          >
            {t('common.cancel')}
          </Button>
          <Button type="submit" variant="primary" size="sm" disabled={saving} aria-busy={saving}>
            <Check aria-hidden />
            {t('userOverlays.changeRoleSubmit')}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  );
}

// ---------------------------------------------------------------------------
// 4) EditUserDrawer  (.pen frame xmWHa)
// ---------------------------------------------------------------------------

const editSchema = z.object({
  email: z.string().min(1, 'Email wajib diisi').email('Format email tidak valid'),
});

type EditForm = z.infer<typeof editSchema>;

export interface EditUserDrawerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  user: User | null;
  onDone: () => void;
  onRequestChangeRole: (user: User) => void;
  onRequestToggleStatus: (user: User) => void;
}

export function EditUserDrawer({
  open,
  onOpenChange,
  user,
  onDone,
  onRequestChangeRole,
  onRequestToggleStatus,
}: EditUserDrawerProps) {
  const { t } = useTranslation();
  const { toast } = useToast();
  const mutation = useUpdateUser();
  const [serverError, setServerError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    setError,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<EditForm>({
    resolver: zodResolver(editSchema),
    values: user ? { email: user.email } : undefined,
  });

  function handleClose() {
    reset();
    setServerError(null);
    onOpenChange(false);
  }

  function onSubmit(values: EditForm) {
    if (!user) return;
    setServerError(null);
    mutation.mutate(
      { userId: user.id, data: { email: values.email } },
      {
        onSuccess: () => {
          toast({ tone: 'success', title: t('userOverlays.editSuccess') });
          handleClose();
          onDone();
        },
        onError: (err) => {
          if (!applyFieldErrors(err, setError)) {
            const { message } = classifyError(err);
            setServerError(t(message));
          }
        },
      },
    );
  }

  const saving = isSubmitting || mutation.isPending;
  const isActive = user?.status === UserStatus.ACTIVE;

  return (
    <Drawer open={open} onOpenChange={handleClose} width={520}>
      <DrawerHeader
        title={t('userOverlays.editTitle')}
        subtitle={user ? `#${user.id}` : undefined}
        closeLabel={t('common.close')}
      />

      <DrawerBody>
        {user && (
          <>
            {/* Profile block */}
            <div className="flex items-center gap-3">
              <Avatar initials={initials(user.full_name)} size={52} />
              <div className="flex flex-col gap-0.5">
                <span className="text-[18px] font-bold text-text">{user.full_name}</span>
                <span className="text-[13px] text-text-2">{user.email}</span>
              </div>
            </div>

            {/* Status row */}
            <div className="flex flex-wrap items-center gap-2">
              <StatusBadge dot tone={isActive ? 'ok' : 'bad'}>
                {isActive ? t('users.statusActive') : t('users.statusDisabled')}
              </StatusBadge>
              <StatusBadge dot tone="info">
                {t(`role.${user.role}`)}
              </StatusBadge>
              <span className="text-xs text-text-3">
                {t('userOverlays.lastLogin')}:{' '}
                {user.last_login_at ? (
                  <DateText kind="instant" value={user.last_login_at} className="inline" />
                ) : (
                  t('users.neverLoggedIn')
                )}
              </span>
            </div>
          </>
        )}

        {serverError && <Banner tone="bad" title={serverError} />}

        <form id="edit-user-form" onSubmit={handleSubmit(onSubmit)} noValidate>
          {/* PROFIL section */}
          <div className="flex flex-col gap-3.5">
            <p className="text-[11px] font-bold uppercase tracking-wider text-text-3">
              {t('userOverlays.sectionProfile')}
            </p>

            <FormField
              label={t('userOverlays.fieldEmailLogin')}
              htmlFor="eu-email"
              hint={t('userOverlays.fieldEmailLoginHint')}
              error={errors.email?.message}
            >
              <Input
                id="eu-email"
                type="email"
                aria-invalid={errors.email ? true : undefined}
                disabled={saving}
                {...register('email')}
              />
            </FormField>

            <FormField label={t('userOverlays.fieldDisplayName')} htmlFor="eu-display-name">
              <Input id="eu-display-name" value={user?.full_name ?? ''} readOnly disabled />
            </FormField>
          </div>

          {/* PERAN & SCOPE section */}
          <div className="mt-5 flex flex-col gap-3.5">
            <p className="text-[11px] font-bold uppercase tracking-wider text-text-3">
              {t('userOverlays.sectionRole')}
            </p>

            <FormField
              label={t('userOverlays.fieldRole')}
              htmlFor="eu-role"
              hint={t('userOverlays.fieldRoleHint')}
            >
              <div className="flex items-center gap-2">
                <Input
                  id="eu-role"
                  value={user ? t(`role.${user.role}`) : ''}
                  readOnly
                  disabled
                  className="flex-1"
                />
                {user && (
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={() => {
                      handleClose();
                      onRequestChangeRole(user);
                    }}
                  >
                    {t('userOverlays.menuChangeRole')}
                  </Button>
                )}
              </div>
            </FormField>

            <FormField
              label={t('userOverlays.fieldCompanyScope')}
              htmlFor="eu-company"
              hint={t('userOverlays.fieldCompanyScopeHint')}
            >
              <Input id="eu-company" value={user?.company_name ?? '—'} readOnly disabled />
            </FormField>
          </div>

          {/* STATUS AKUN section */}
          <div className="mt-5 flex flex-col gap-3.5">
            <p className="text-[11px] font-bold uppercase tracking-wider text-text-3">
              {t('userOverlays.sectionStatus')}
            </p>

            <div className="flex items-center justify-between rounded-md border border-border-soft px-3 py-2.5">
              <div className="flex flex-col gap-0.5">
                <span className="text-sm font-semibold text-text">
                  {t('userOverlays.toggleActiveLabel')}
                </span>
                <span className="text-xs text-text-3">{t('userOverlays.toggleActiveHint')}</span>
              </div>
              {user && (
                <Toggle
                  checked={isActive}
                  onCheckedChange={() => {
                    handleClose();
                    onRequestToggleStatus(user);
                  }}
                  aria-label={t('userOverlays.toggleActiveAriaLabel')}
                />
              )}
            </div>
          </div>
        </form>
      </DrawerBody>

      <DrawerFooter className="justify-between">
        <Button type="button" variant="ghost" size="sm" disabled={saving} onClick={handleClose}>
          {t('common.cancel')}
        </Button>
        <Button
          type="submit"
          form="edit-user-form"
          variant="primary"
          size="sm"
          disabled={saving}
          aria-busy={saving}
        >
          <Check aria-hidden />
          {t('userOverlays.editSubmit')}
        </Button>
      </DrawerFooter>
    </Drawer>
  );
}

// ---------------------------------------------------------------------------
// 5) SendResetConfirm  (.pen frame oXZNQ)
// ---------------------------------------------------------------------------

export interface SendResetConfirmProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  user: User | null;
  onDone: () => void;
}

export function SendResetConfirm({ open, onOpenChange, user, onDone }: SendResetConfirmProps) {
  const { t } = useTranslation();
  const { toast } = useToast();
  const mutation = useSendUserPasswordReset();

  function handleConfirm() {
    if (!user) return;
    mutation.mutate(
      { userId: user.id },
      {
        onSuccess: () => {
          toast({ tone: 'success', title: t('userOverlays.sendResetSuccess') });
          onOpenChange(false);
          onDone();
        },
        onError: (err) => {
          const { message } = classifyError(err);
          toast({ tone: 'error', title: t(message) });
        },
      },
    );
  }

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      icon={KeyRound}
      tone="info"
      title={t('userOverlays.sendResetTitle')}
      description={user ? t('userOverlays.sendResetDescription', { email: user.email }) : undefined}
      cancelLabel={t('common.cancel')}
      confirmLabel={t('userOverlays.sendResetConfirm')}
      confirmTone="primary"
      onConfirm={handleConfirm}
      loading={mutation.isPending}
      closeLabel={t('common.close')}
    >
      <div className="flex items-center gap-1.5 text-[12px] text-info-tx">
        <Info className="size-3.5 shrink-0" aria-hidden />
        <span>{t('userOverlays.auditHint')}</span>
      </div>
    </ConfirmDialog>
  );
}

// ---------------------------------------------------------------------------
// 6a) DeactivateUserConfirm  (.pen frame cACO9)
// ---------------------------------------------------------------------------

export interface DeactivateUserConfirmProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  user: User | null;
  onDone: () => void;
}

export function DeactivateUserConfirm({
  open,
  onOpenChange,
  user,
  onDone,
}: DeactivateUserConfirmProps) {
  const { t } = useTranslation();
  const { toast } = useToast();
  const mutation = useDeactivateUser();

  function handleConfirm() {
    if (!user) return;
    mutation.mutate(
      { userId: user.id, data: {} },
      {
        onSuccess: () => {
          toast({ tone: 'success', title: t('userOverlays.deactivateSuccess') });
          onOpenChange(false);
          onDone();
        },
        onError: (err) => {
          const { message } = classifyError(err);
          toast({ tone: 'error', title: t(message) });
        },
      },
    );
  }

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      icon={UserX}
      tone="danger"
      title={t('userOverlays.deactivateTitle')}
      description={
        user ? t('userOverlays.deactivateDescription', { name: user.full_name }) : undefined
      }
      cancelLabel={t('common.cancel')}
      confirmLabel={t('userOverlays.deactivateConfirm')}
      confirmTone="danger"
      onConfirm={handleConfirm}
      loading={mutation.isPending}
      closeLabel={t('common.close')}
    >
      <div className="flex items-center gap-1.5 text-[12px] text-info-tx">
        <Info className="size-3.5 shrink-0" aria-hidden />
        <span>{t('userOverlays.deactivateAuditHint')}</span>
      </div>
    </ConfirmDialog>
  );
}

// ---------------------------------------------------------------------------
// 6b) ReactivateUserConfirm  (sibling variant of cACO9)
// ---------------------------------------------------------------------------

export interface ReactivateUserConfirmProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  user: User | null;
  onDone: () => void;
}

export function ReactivateUserConfirm({
  open,
  onOpenChange,
  user,
  onDone,
}: ReactivateUserConfirmProps) {
  const { t } = useTranslation();
  const { toast } = useToast();
  const mutation = useReactivateUser();

  function handleConfirm() {
    if (!user) return;
    mutation.mutate(
      { userId: user.id, data: {} },
      {
        onSuccess: () => {
          toast({ tone: 'success', title: t('userOverlays.reactivateSuccess') });
          onOpenChange(false);
          onDone();
        },
        onError: (err) => {
          const { message } = classifyError(err);
          toast({ tone: 'error', title: t(message) });
        },
      },
    );
  }

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      icon={UserCheck}
      tone="info"
      title={t('userOverlays.reactivateTitle')}
      description={
        user ? t('userOverlays.reactivateDescription', { name: user.full_name }) : undefined
      }
      cancelLabel={t('common.cancel')}
      confirmLabel={t('userOverlays.reactivateConfirm')}
      confirmTone="primary"
      onConfirm={handleConfirm}
      loading={mutation.isPending}
      closeLabel={t('common.close')}
    >
      <div className="flex items-center gap-1.5 text-[12px] text-info-tx">
        <Info className="size-3.5 shrink-0" aria-hidden />
        <span>{t('userOverlays.auditHint')}</span>
      </div>
    </ConfirmDialog>
  );
}
