/**
 * AuditTrail family (molecules) — three composable components for surfacing audit history.
 *
 * .pen node ids covered:
 *   comp/AuditTrailViewer  `jzBi0`
 *   comp/AuditTrailDrawer  `BUAHW`
 *   comp/AuditTrailInline  `qtz6q`
 *
 * ENGINEERING.md G4 — each compound interaction pattern (timeline viewer, right-sheet drawer,
 * compact inline card) is a named molecule. G3 — one canonical component per concept; the Drawer
 * composes AuditTrailViewer instead of forking.
 *
 * AuditTrailDrawer is built directly on @radix-ui/react-dialog as a right-side sheet.
 * Follow-up: extract a generic `Drawer` (sheet) molecule and make AuditTrailDrawer compose it,
 * the same way ConfirmDialog composes Modal. (ENGINEERING.md G3)
 *
 * All copy is passed via props (Bahasa strings below are default fallbacks — wire to i18n).
 */

import * as Dialog from '@radix-ui/react-dialog';
import { ArrowRight, Check, Download, History, MessageSquare, Pencil, Plus, X } from 'lucide-react';
import type * as React from 'react';
import { cn } from '../lib/cn.ts';
import { Button } from '../primitives/button.tsx';

// ---------------------------------------------------------------------------
// Shared data model
// ---------------------------------------------------------------------------

export type AuditEventType = 'created' | 'updated' | 'approved' | 'rejected' | 'note';

export interface AuditEntry {
  id: string;
  type: AuditEventType;
  /** Display name of the actor, e.g. "Sari Hadi" */
  actor: string;
  /** Human-readable verb phrase, e.g. "menyetujui", "mengubah shift_id" */
  verb: string;
  /** Pre-formatted relative time, e.g. "5 mnt lalu" */
  time: string;
  /** Optional tinted comment box */
  comment?: {
    tone: 'warn' | 'bad' | 'info';
    text: string;
  };
  /** Optional monospaced field diff */
  diff?: {
    field: string;
    from: string;
    to: string;
  };
}

// ---------------------------------------------------------------------------
// Event-type → visual config (icon + dot tint). Token classes only.
// ---------------------------------------------------------------------------

const eventConfig: Record<
  AuditEventType,
  {
    icon: React.ComponentType<{ className?: string; 'aria-hidden'?: 'true' }>;
    dotClass: string;
    iconClass: string;
  }
> = {
  created: {
    icon: Plus,
    dotClass: 'bg-primary-soft',
    iconClass: 'text-primary',
  },
  updated: {
    icon: Pencil,
    dotClass: 'bg-info-bg',
    iconClass: 'text-info-tx',
  },
  approved: {
    icon: Check,
    dotClass: 'bg-ok-bg',
    iconClass: 'text-ok-tx',
  },
  rejected: {
    icon: X,
    dotClass: 'bg-bad-bg',
    iconClass: 'text-bad-tx',
  },
  note: {
    icon: MessageSquare,
    dotClass: 'bg-warn-bg',
    iconClass: 'text-warn-tx',
  },
};

// ---------------------------------------------------------------------------
// Comment box tone → token classes
// ---------------------------------------------------------------------------

const commentToneClass: Record<'warn' | 'bad' | 'info', string> = {
  warn: 'bg-warn-bg border-warn-bd text-warn-tx',
  bad: 'bg-bad-bg border-bad-bd text-bad-tx',
  info: 'bg-info-bg border-info-bd text-info-tx',
};

// ---------------------------------------------------------------------------
// AuditTrailViewer — full timeline card (comp/AuditTrailViewer `jzBi0`)
// ---------------------------------------------------------------------------

export interface AuditTrailViewerProps {
  title: string;
  entries: AuditEntry[];
  /** Pre-formatted count label, e.g. "12 perubahan" */
  count?: string;
  /** Optional filter-chips slot rendered below the header title row */
  filters?: React.ReactNode;
  /** Optional footer slot (bg-surface-2) */
  footer?: React.ReactNode;
  className?: string;
}

export function AuditTrailViewer({
  title,
  entries,
  count,
  filters,
  footer,
  className,
}: AuditTrailViewerProps) {
  return (
    <div
      className={cn(
        'flex flex-col overflow-hidden rounded-lg border border-border bg-surface',
        className,
      )}
    >
      {/* Header */}
      <div className="flex flex-col gap-2.5 border-b border-border-soft px-4 py-3.5">
        {/* Top row: icon + title (left) | count (right) */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-1.5">
            <History className="size-4 text-text-2" aria-hidden="true" />
            <span className="text-sm font-bold text-text">{title}</span>
          </div>
          {count !== undefined && <span className="text-xs text-text-3">{count}</span>}
        </div>

        {/* Optional filter-chips slot */}
        {filters !== undefined && filters}
      </div>

      {/* Timeline */}
      <div className="flex flex-col">
        {entries.map((entry, idx) => {
          const isLast = idx === entries.length - 1;
          const { icon: Icon, dotClass, iconClass } = eventConfig[entry.type];

          return (
            <div
              key={entry.id}
              className={cn('flex gap-3 px-4 py-3.5', !isLast && 'border-b border-border-soft')}
            >
              {/* Rail: dot + connector line */}
              <div className="flex flex-col items-center gap-1">
                {/* Dot */}
                <span
                  className={cn(
                    'inline-flex size-6 shrink-0 items-center justify-center rounded-full',
                    dotClass,
                  )}
                  aria-hidden="true"
                >
                  <Icon className={cn('size-[13px]', iconClass)} aria-hidden="true" />
                </span>

                {/* Connector line — omit on last entry */}
                {!isLast && <span className="w-0.5 flex-1 bg-border-soft" aria-hidden="true" />}
              </div>

              {/* Body */}
              <div className="flex flex-1 flex-col gap-1.5">
                {/* Meta row: actor + verb (left) | time (right) */}
                <div className="flex items-center justify-between">
                  <p className="text-xs text-text-2">
                    <span className="font-semibold text-text">{entry.actor}</span> {entry.verb}
                  </p>
                  <span className="text-[11px] font-medium text-text-3">{entry.time}</span>
                </div>

                {/* Comment box */}
                {entry.comment !== undefined && (
                  <div
                    className={cn(
                      'rounded-md border px-2.5 py-2 text-xs leading-[1.4]',
                      commentToneClass[entry.comment.tone],
                    )}
                  >
                    {entry.comment.text}
                  </div>
                )}

                {/* Diff box */}
                {entry.diff !== undefined && (
                  <div className="flex items-center gap-1.5 rounded-md border border-border-soft bg-surface-2 px-2.5 py-2">
                    <span className="font-mono text-[11px] font-semibold text-text-2">
                      {entry.diff.field}
                    </span>
                    <span className="font-mono text-[11px] text-bad-tx">{entry.diff.from}</span>
                    <ArrowRight className="size-3 shrink-0 text-text-3" aria-hidden="true" />
                    <span className="font-mono text-[11px] font-semibold text-ok-tx">
                      {entry.diff.to}
                    </span>
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>

      {/* Footer slot */}
      {footer !== undefined && (
        <div className="border-t border-border-soft bg-surface-2 px-4 py-2.5">{footer}</div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// AuditTrailInline — compact card (comp/AuditTrailInline `qtz6q`)
// ---------------------------------------------------------------------------

export type AuditEntryCompact = Pick<AuditEntry, 'id' | 'type' | 'actor' | 'verb' | 'time'>;

export interface AuditTrailInlineProps {
  title: string;
  entries: AuditEntryCompact[];
  onViewAll?: () => void;
  /** Label for the "view all" link button. Defaults to 'Lihat semua' — wire to i18n. */
  viewAllLabel?: string;
  className?: string;
}

export function AuditTrailInline({
  title,
  entries,
  onViewAll,
  viewAllLabel = 'Lihat semua',
  className,
}: AuditTrailInlineProps) {
  return (
    <div
      className={cn(
        'flex flex-col overflow-hidden rounded-lg border border-border bg-surface',
        className,
      )}
    >
      {/* Header */}
      <div className="flex items-center justify-between border-b border-border-soft px-3.5 py-3">
        <div className="flex items-center gap-1.5">
          <History className="size-3.5 text-text-2" aria-hidden="true" />
          <span className="text-[13px] font-bold text-text">{title}</span>
        </div>

        {/* "View all" link — only when handler provided */}
        {onViewAll !== undefined && (
          <button
            type="button"
            onClick={onViewAll}
            className="inline-flex items-center gap-1 text-xs font-semibold text-primary hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
          >
            {viewAllLabel}
            <ArrowRight className="size-3 shrink-0 text-primary" aria-hidden="true" />
          </button>
        )}
      </div>

      {/* Compact list */}
      <div className="flex flex-col">
        {entries.map((entry, idx) => {
          const isLast = idx === entries.length - 1;
          const { icon: Icon, dotClass, iconClass } = eventConfig[entry.type];

          return (
            <div
              key={entry.id}
              className={cn('flex gap-2.5 px-3.5 py-2.5', !isLast && 'border-b border-border-soft')}
            >
              {/* Compact dot */}
              <span
                className={cn(
                  'mt-0.5 inline-flex size-5 shrink-0 items-center justify-center rounded-full',
                  dotClass,
                )}
                aria-hidden="true"
              >
                <Icon className={cn('size-[11px]', iconClass)} aria-hidden="true" />
              </span>

              {/* Body */}
              <div className="flex flex-col gap-0.5">
                <p className="text-xs text-text-2">
                  <span className="font-semibold text-text">{entry.actor}</span> {entry.verb}
                </p>
                <span className="text-[11px] text-text-3">{entry.time}</span>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// AuditTrailDrawer — right-side sheet (comp/AuditTrailDrawer `BUAHW`)
//
// Built directly on @radix-ui/react-dialog.  Follow-up: extract a generic
// Drawer (sheet) molecule so AuditTrailDrawer can compose it — same pattern
// as ConfirmDialog composes Modal (ENGINEERING.md G3).
// ---------------------------------------------------------------------------

export interface AuditTrailDrawerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  subtitle?: string;
  entries: AuditEntry[];
  /** Pre-formatted count label forwarded to the inner AuditTrailViewer */
  count?: string;
  onExport?: () => void;
  /** Label for the export button. Defaults to 'Ekspor riwayat' — wire to i18n. */
  exportLabel?: string;
  /** Label for the close button. Defaults to 'Tutup' — wire to i18n. */
  closeLabel?: string;
  className?: string;
}

export function AuditTrailDrawer({
  open,
  onOpenChange,
  title,
  subtitle,
  entries,
  count,
  onExport,
  exportLabel = 'Ekspor riwayat',
  closeLabel = 'Tutup',
  className,
}: AuditTrailDrawerProps) {
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        {/* Overlay — bg-scrim covers full viewport */}
        <Dialog.Overlay className="fixed inset-0 z-40 bg-scrim" />

        {/* Drawer panel — fixed right-side sheet */}
        <Dialog.Content
          className={cn(
            'fixed inset-y-0 right-0 z-50 flex w-[560px] max-w-[calc(100vw-1rem)] flex-col',
            'border-l border-border bg-surface shadow-overlay',
            className,
          )}
        >
          {/* Drawer header */}
          <div className="flex items-center justify-between border-b border-border-soft px-5 py-3.5">
            <div className="flex flex-col gap-0.5">
              <Dialog.Title className="text-base font-bold text-text leading-snug">
                {title}
              </Dialog.Title>
              {subtitle !== undefined && (
                <Dialog.Description className="text-xs text-text-3">{subtitle}</Dialog.Description>
              )}
            </div>

            {/* Close × button */}
            <Dialog.Close
              aria-label={closeLabel}
              className={cn(
                'inline-flex size-8 shrink-0 items-center justify-center rounded-md bg-surface-2',
                'text-text-2 transition-colors hover:bg-muted',
                'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
              )}
            >
              <X className="size-4" aria-hidden="true" />
            </Dialog.Close>
          </div>

          {/* Body — scrollable timeline */}
          <div className="flex-1 overflow-auto px-5 py-4">
            <AuditTrailViewer title={title} entries={entries} count={count} />
          </div>

          {/* Footer */}
          <div className="flex items-center justify-between border-t border-border-soft bg-surface-2 px-5 py-3">
            {/* Export button — only when handler provided */}
            {onExport !== undefined ? (
              <button
                type="button"
                onClick={onExport}
                className={cn(
                  'inline-flex items-center gap-1.5 text-[13px] text-text-2',
                  'hover:text-text transition-colors',
                  'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 rounded-sm',
                )}
              >
                <Download className="size-3.5 shrink-0" aria-hidden="true" />
                {exportLabel}
              </button>
            ) : (
              /* Keep layout stable when no export action */
              <span />
            )}

            {/* Close button */}
            <Dialog.Close asChild>
              <Button variant="secondary" size="sm">
                {closeLabel}
              </Button>
            </Dialog.Close>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
