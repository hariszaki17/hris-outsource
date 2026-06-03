/**
 * CursorPagination (molecule)
 *
 * Derived from: E2 · Karyawan — Daftar `WElYh` (pagination bar at the bottom of the
 * list-table card).
 *
 * ENGINEERING.md D1 (cursor pagination): numbered/offset pages are EXPLICITLY forbidden.
 * The `.pen` mock renders page numbers as a placeholder; this implementation supersedes
 * that with prev/next cursor navigation only.
 *
 * ENGINEERING.md B2 (loading/empty are first-class states): this component is stateless
 * — the parent drives hasPrev/hasNext from its own cursor state so there is never a
 * dead-flow (clicking an exhausted direction is impossible).
 *
 * Default aria-labels: prevLabel='Sebelumnya' / nextLabel='Berikutnya' — wire to i18n
 * keys common.prev / common.next.
 */

import { ChevronLeft, ChevronRight } from 'lucide-react';
import type * as React from 'react';
import { cn } from '../lib/cn.ts';

export interface CursorPaginationProps {
  /** Human-readable range label. E.g. "Menampilkan 1–6 dari 128". */
  rangeLabel: string;
  /** Whether a previous page cursor is available. */
  hasPrev: boolean;
  /** Whether a next page cursor is available. */
  hasNext: boolean;
  /** Called when the user navigates to the previous page. */
  onPrev: () => void;
  /** Called when the user navigates to the next page. */
  onNext: () => void;
  /**
   * Accessible label for the previous button.
   * Default: 'Sebelumnya' (i18n key: common.prev).
   */
  prevLabel?: string;
  /**
   * Accessible label for the next button.
   * Default: 'Berikutnya' (i18n key: common.next).
   */
  nextLabel?: string;
  className?: string;
}

/**
 * Renders a justify-between pagination bar: range label on the left, prev/next
 * icon buttons on the right. Designed to be passed as the `footer` prop of
 * `<DataTable>`.
 */
export function CursorPagination({
  rangeLabel,
  hasPrev,
  hasNext,
  onPrev,
  onNext,
  prevLabel = 'Sebelumnya',
  nextLabel = 'Berikutnya',
  className,
}: CursorPaginationProps): React.ReactElement {
  return (
    <div
      className={cn(
        'flex items-center justify-between border-t border-border-soft px-4 py-3',
        className,
      )}
    >
      {/* Range label */}
      <span className="text-[13px] text-text-2">{rangeLabel}</span>

      {/* Prev / Next controls */}
      <div className="flex items-center gap-1">
        <button
          type="button"
          onClick={onPrev}
          disabled={!hasPrev}
          aria-label={prevLabel}
          className={cn(
            'inline-flex size-8 items-center justify-center rounded-md border border-border bg-surface',
            'transition-opacity focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
            !hasPrev && 'pointer-events-none opacity-50',
          )}
        >
          <ChevronLeft aria-hidden="true" className="size-3.5 text-text-2" />
        </button>

        <button
          type="button"
          onClick={onNext}
          disabled={!hasNext}
          aria-label={nextLabel}
          className={cn(
            'inline-flex size-8 items-center justify-center rounded-md border border-border bg-surface',
            'transition-opacity focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
            !hasNext && 'pointer-events-none opacity-50',
          )}
        >
          <ChevronRight aria-hidden="true" className="size-3.5 text-text-2" />
        </button>
      </div>
    </div>
  );
}
