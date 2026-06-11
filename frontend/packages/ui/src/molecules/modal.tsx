/**
 * Modal / ConfirmDialog family (molecule).
 *
 * .pen node ids covered by this single component family:
 *   comp/ModalReject          `EnabP`
 *   comp/ModalBulkApprove     `r4KZl5`
 *   comp/ModalDestructive     `V4LG8`
 *   comp/ModalDiscardChanges  `z0kH0b`
 *
 * ENGINEERING.md G3 — one canonical Modal, variants via composition (no forked modals).
 * ENGINEERING.md G4 — interaction catalogue → named molecule; ConfirmDialog is the
 *   named confirm-flow molecule that composes Modal + ModalHeader/Body/Footer.
 *
 * Radix Dialog provides focus-trap, ESC close, overlay click-to-close, and full a11y
 * (role="dialog", aria-modal, aria-labelledby, aria-describedby) so we don't hand-roll any
 * of those behaviours.
 */

import * as Dialog from '@radix-ui/react-dialog';
import { X } from 'lucide-react';
import type * as React from 'react';
import { cn } from '../lib/cn.ts';
import { Button } from '../primitives/button.tsx';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type ModalSize = 'sm' | 'md' | 'lg';
export type ModalTone = 'brand' | 'danger' | 'warn' | 'info' | 'neutral';
export type ConfirmTone = 'primary' | 'danger';

// ---------------------------------------------------------------------------
// Size → max-width map (token-compatible, no raw px class used at runtime)
// ---------------------------------------------------------------------------

const sizeClass: Record<ModalSize, string> = {
  sm: 'w-[420px]',
  md: 'w-[480px]',
  lg: 'w-[520px]',
};

// ---------------------------------------------------------------------------
// Tone → IconCircle bg + icon colour  (design-system token classes only)
// ---------------------------------------------------------------------------

const toneCircleClass: Record<ModalTone, string> = {
  brand: 'bg-primary-soft text-primary',
  danger: 'bg-bad-bg text-bad-tx',
  warn: 'bg-warn-bg text-warn-tx',
  info: 'bg-info-bg text-info-tx',
  neutral: 'bg-surface-2 text-text-2',
};

// ---------------------------------------------------------------------------
// Modal — root shell (Dialog.Root + Portal + Overlay + Content)
// ---------------------------------------------------------------------------

export interface ModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  size?: ModalSize;
  children?: React.ReactNode;
  className?: string;
}

export function Modal({ open, onOpenChange, size = 'md', children, className }: ModalProps) {
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        {/* Overlay — bg-scrim covers full viewport */}
        <Dialog.Overlay className="fixed inset-0 z-40 bg-scrim" />

        {/* Content panel — centred, panel tokens, width by size */}
        <Dialog.Content
          className={cn(
            // position + centre
            'fixed left-1/2 top-1/2 z-50 -translate-x-1/2 -translate-y-1/2',
            // panel shell
            'flex flex-col overflow-hidden rounded-lg border border-border bg-surface shadow-overlay',
            // responsive clamp so we never overflow small viewports (width AND height)
            'max-h-[calc(100dvh-2rem)] max-w-[calc(100vw-2rem)]',
            sizeClass[size],
            className,
          )}
        >
          {children}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

// ---------------------------------------------------------------------------
// ModalHeader
// ---------------------------------------------------------------------------

export interface ModalHeaderProps {
  /** Any Lucide icon component */
  icon: React.ComponentType<{ className?: string }>;
  tone?: ModalTone;
  title: string;
  /** Called when the close × button is activated; falls back to Dialog.Close behaviour. */
  onClose?: () => void;
  /** aria-label for the × button. Defaults to 'Tutup' — wire to i18n key common.close. */
  closeLabel?: string;
}

export function ModalHeader({
  icon: Icon,
  tone = 'neutral',
  title,
  onClose,
  closeLabel = 'Tutup',
}: ModalHeaderProps) {
  return (
    <div className="flex shrink-0 items-center justify-between border-b border-border-soft px-5 py-4">
      {/* Left: icon circle + title */}
      <div className="flex items-center gap-3">
        {/* IconCircle */}
        <span
          className={cn(
            'inline-flex size-9 shrink-0 items-center justify-center rounded-full',
            toneCircleClass[tone],
          )}
          aria-hidden="true"
        >
          <Icon className="size-[18px]" />
        </span>

        {/* Dialog.Title for a11y (aria-labelledby) */}
        <Dialog.Title className="text-base font-bold text-text leading-snug">{title}</Dialog.Title>
      </div>

      {/* Right: close × button */}
      <Dialog.Close
        onClick={onClose}
        aria-label={closeLabel}
        className={cn(
          'inline-flex size-[30px] shrink-0 items-center justify-center rounded-md bg-surface-2',
          'text-text-2 transition-colors hover:bg-muted focus-visible:outline-none',
          'focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
        )}
      >
        <X className="size-4" aria-hidden="true" />
      </Dialog.Close>
    </div>
  );
}

// ---------------------------------------------------------------------------
// ModalBody
// ---------------------------------------------------------------------------

export interface ModalBodyProps {
  children?: React.ReactNode;
  className?: string;
}

export function ModalBody({ children, className }: ModalBodyProps) {
  return (
    <div
      className={cn('flex min-h-0 flex-1 flex-col gap-3.5 overflow-y-auto px-5 py-5', className)}
    >
      {children}
    </div>
  );
}

// ---------------------------------------------------------------------------
// ModalFooter
// ---------------------------------------------------------------------------

export interface ModalFooterProps {
  children?: React.ReactNode;
  className?: string;
}

export function ModalFooter({ children, className }: ModalFooterProps) {
  return (
    <div
      className={cn(
        'flex shrink-0 justify-end gap-2 border-t border-border-soft bg-surface-2 px-5 py-3.5',
        className,
      )}
    >
      {children}
    </div>
  );
}

// ---------------------------------------------------------------------------
// ConfirmDialog — named confirm-flow molecule (G4)
// ---------------------------------------------------------------------------

export interface ConfirmDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;

  /** Any Lucide icon component */
  icon: React.ComponentType<{ className?: string }>;
  tone?: ModalTone;
  size?: ModalSize;

  title: string;
  /** Short description rendered above `children`; text-[13px] text-text-2. */
  description?: string;
  /** Extra body content (textarea, recap box, type-to-confirm field, …) */
  children?: React.ReactNode;

  cancelLabel: string;
  confirmLabel: string;
  /** Drives the confirm button variant: 'primary' (green) | 'danger' (red). */
  confirmTone?: ConfirmTone;

  onConfirm: () => void;
  /** When true the confirm button shows as loading/disabled. */
  loading?: boolean;
  /** Additional condition that disables confirm (e.g. type-to-confirm gate). */
  confirmDisabled?: boolean;

  /** aria-label for the × close button. Defaults to 'Tutup'. */
  closeLabel?: string;
}

export function ConfirmDialog({
  open,
  onOpenChange,
  icon,
  tone = 'neutral',
  size = 'md',
  title,
  description,
  children,
  cancelLabel,
  confirmLabel,
  confirmTone = 'primary',
  onConfirm,
  loading = false,
  confirmDisabled = false,
  closeLabel,
}: ConfirmDialogProps) {
  const isConfirmDisabled = confirmDisabled || loading;

  return (
    <Modal open={open} onOpenChange={onOpenChange} size={size}>
      <ModalHeader icon={icon} tone={tone} title={title} closeLabel={closeLabel} />

      <ModalBody>
        {description && <p className="text-[13px] leading-[1.5] text-text-2">{description}</p>}
        {children}
      </ModalBody>

      <ModalFooter>
        {/* Cancel — always secondary; Dialog.Close handles dismiss without callback */}
        <Dialog.Close asChild>
          <Button variant="secondary" size="sm">
            {cancelLabel}
          </Button>
        </Dialog.Close>

        {/* Confirm */}
        <Button
          variant={confirmTone === 'danger' ? 'destructive' : 'primary'}
          size="sm"
          disabled={isConfirmDisabled}
          aria-busy={loading}
          onClick={onConfirm}
        >
          {confirmLabel}
        </Button>
      </ModalFooter>
    </Modal>
  );
}
