/**
 * Drawer family (molecule) — right-edge sheet on Radix Dialog.
 *
 * Generic extraction of the `.pen` right-sheet pattern used by:
 *   E1 · Drawer · Edit Pengguna   `xmWHa`  (width 520)
 *   E1 · Drawer · Audit Detail    `x5wrt`  (width 560)
 * …and reused across later epics (placement detail, leave detail, OT detail, audit drawers).
 *
 * ENGINEERING.md G3/G4 — one canonical Drawer; variants via the `width` prop + composition of
 * DrawerHeader/Body/Footer. Radix Dialog supplies focus-trap, ESC close, overlay click-to-close,
 * and full a11y (role="dialog", aria-modal, aria-labelledby) so none of it is hand-rolled.
 *
 * The panel is a flex column pinned to the viewport's right edge at full height; DrawerBody is the
 * single scroll region so header/footer stay fixed.
 */

import * as Dialog from '@radix-ui/react-dialog';
import { X } from 'lucide-react';
import type * as React from 'react';
import { cn } from '../lib/cn.ts';

// ---------------------------------------------------------------------------
// Drawer — root shell (Dialog.Root + Portal + Overlay + right-pinned Content)
// ---------------------------------------------------------------------------

export interface DrawerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Panel width in px (matches the `.pen` frame width; e.g. 520 edit, 560 audit). Default 520. */
  width?: number;
  /** aria-label when the panel has no DrawerHeader title to label it. */
  ariaLabel?: string;
  children?: React.ReactNode;
  className?: string;
}

export function Drawer({
  open,
  onOpenChange,
  width = 520,
  ariaLabel,
  children,
  className,
}: DrawerProps) {
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-scrim" />
        <Dialog.Content
          aria-label={ariaLabel}
          style={{ width, maxWidth: '100vw' }}
          className={cn(
            'fixed inset-y-0 right-0 z-50 flex flex-col border-l border-border bg-surface shadow-overlay',
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
// DrawerHeader — title (+ optional subtitle) + close ×
// ---------------------------------------------------------------------------

export interface DrawerHeaderProps {
  title: string;
  /** Optional second line — mono meta (e.g. "#U-204 · Diperbarui …"). */
  subtitle?: string;
  /** Optional leading slot (icon circle, status pill) rendered before the title block. */
  leading?: React.ReactNode;
  /** Optional trailing slot rendered before the × button (e.g. an action chip). */
  trailing?: React.ReactNode;
  onClose?: () => void;
  /** aria-label for the × button. Wire to i18n `common.close`. Defaults to 'Tutup'. */
  closeLabel?: string;
  className?: string;
}

export function DrawerHeader({
  title,
  subtitle,
  leading,
  trailing,
  onClose,
  closeLabel = 'Tutup',
  className,
}: DrawerHeaderProps) {
  return (
    <div
      className={cn(
        'flex shrink-0 items-center justify-between border-b border-border-soft px-5 py-4',
        className,
      )}
    >
      <div className="flex min-w-0 items-center gap-3">
        {leading}
        <div className="flex min-w-0 flex-col gap-0.5">
          <Dialog.Title className="truncate font-bold text-[17px] text-text leading-snug">
            {title}
          </Dialog.Title>
          {subtitle && (
            <span className="truncate font-mono text-[11px] text-text-3">{subtitle}</span>
          )}
        </div>
      </div>
      <div className="flex shrink-0 items-center gap-2">
        {trailing}
        <Dialog.Close
          onClick={onClose}
          aria-label={closeLabel}
          className={cn(
            'inline-flex size-8 shrink-0 items-center justify-center rounded-md bg-surface-2',
            'text-text-2 transition-colors hover:bg-muted focus-visible:outline-none',
            'focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
          )}
        >
          <X className="size-4" aria-hidden="true" />
        </Dialog.Close>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// DrawerBody — the single scroll region
// ---------------------------------------------------------------------------

export interface DrawerBodyProps {
  children?: React.ReactNode;
  className?: string;
}

export function DrawerBody({ children, className }: DrawerBodyProps) {
  return (
    <div
      className={cn('flex min-h-0 flex-1 flex-col gap-3.5 overflow-y-auto px-5 py-4', className)}
    >
      {children}
    </div>
  );
}

// ---------------------------------------------------------------------------
// DrawerFooter — pinned action/meta bar
// ---------------------------------------------------------------------------

export interface DrawerFooterProps {
  children?: React.ReactNode;
  className?: string;
}

export function DrawerFooter({ children, className }: DrawerFooterProps) {
  return (
    <div
      className={cn(
        'flex shrink-0 items-center justify-end gap-2 border-t border-border-soft bg-surface-2 px-5 py-3.5',
        className,
      )}
    >
      {children}
    </div>
  );
}
