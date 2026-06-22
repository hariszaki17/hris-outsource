/**
 * MobileSidebarDrawer (molecule) — left slide-over drawer for mobile sidebar.
 *
 * Wraps `<Sidebar>` in a Radix Dialog when the viewport is <1024px. On desktop the
 * sidebar is rendered inline by the app shell (hidden lg:flex).
 *
 * ENGINEERING.md G4 — interaction catalogue: mobile navigation overlay.
 * Radix Dialog supplies focus-trap, ESC close, overlay click-to-close, a11y.
 */

import * as Dialog from '@radix-ui/react-dialog';
import type * as React from 'react';

export interface MobileSidebarDrawerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  children: React.ReactNode;
}

export function MobileSidebarDrawer({ open, onOpenChange, children }: MobileSidebarDrawerProps) {
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-scrim" />

        <Dialog.Content
          aria-describedby={undefined}
          className="fixed inset-y-0 left-0 z-50 flex w-[280px] flex-col bg-sidebar shadow-overlay"
        >
          <Dialog.Title className="sr-only">Menu Navigasi</Dialog.Title>

          {children}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
