/**
 * DataTable<T> (molecule) — generic, column-config-driven list table.
 *
 * Derived from: E2 · Karyawan — Daftar `WElYh` (list-table card).
 *
 * ENGINEERING.md D1 (cursor pagination + virtualization-ready):
 *   - Pagination is cursor-based only; pass <CursorPagination> as `footer`.
 *   - Virtualization is intentionally deferred. The data/column abstraction is
 *     virtualization-ready: the body is a plain scroll container that can be
 *     swapped for a virtualized list (e.g. @tanstack/virtual) in a follow-up
 *     without any change to the public API or column definitions.
 *
 * ENGINEERING.md B2 (loading/empty are first-class states):
 *   - isLoading → renders `skeletonRows` × <SkeletonTableRow> (no dead-flow).
 *   - empty     → renders the consumer-supplied `empty` node centered in the body.
 *   - Both states match column count (incl. select/action synthetic columns).
 */

import type * as React from 'react';
import { cn } from '../lib/cn.ts';
import { Checkbox } from '../primitives/checkbox.tsx';
import { SkeletonTableRow } from './skeleton.tsx';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface Column<T> {
  /** Stable identifier — used as React key. */
  id: string;
  /** Rendered in the table header. */
  header: React.ReactNode;
  /**
   * Fixed pixel width. Omit to let the column grow with `flex-1`.
   * Fixed-width columns use `flex-shrink-0`; flex columns share remaining space.
   */
  width?: number;
  /** Horizontal alignment of header + cell content. Default: 'left'. */
  align?: 'left' | 'right' | 'center';
  /** Renders the cell for a given row datum. */
  cell: (row: T) => React.ReactNode;
}

export interface DataTableProps<T> {
  columns: Column<T>[];
  data: T[];
  /** Returns a stable, unique string key for each row. Never use array index. */
  getRowId: (row: T) => string;
  /**
   * When true, renders `skeletonRows` placeholder rows instead of data.
   * Matches column count including synthetic select/action columns.
   */
  isLoading?: boolean;
  /** Number of skeleton placeholder rows. Default: 6. */
  skeletonRows?: number;
  /**
   * Rendered (centered) when `!isLoading && data.length === 0`.
   * Consumer passes e.g. `<EmptyState variant="no-results" … />`.
   */
  empty?: React.ReactNode;
  /**
   * Makes rows interactive: hover highlight, pointer cursor, and role="button"
   * a11y. Called when the user clicks anywhere on the row (excluding the
   * checkbox and row-action cells, which stop propagation).
   */
  onRowClick?: (row: T) => void;
  /**
   * Renders a trailing 52 px centered cell per row (e.g. a kebab menu button).
   * The consumer is responsible for stopping propagation if needed.
   */
  rowActions?: (row: T) => React.ReactNode;
  /**
   * Prepends a 44 px checkbox column. Header checkbox toggles all visible rows.
   * Controlled via `selectedIds` + `onSelectionChange`.
   */
  selectable?: boolean;
  /** Controlled set of selected row IDs. */
  selectedIds?: string[];
  /** Called with the updated full set of selected IDs after any toggle. */
  onSelectionChange?: (ids: string[]) => void;
  /**
   * Rendered below the scroll body inside the card (outside the scroll region).
   * Consumer passes `<CursorPagination … />`.
   */
  footer?: React.ReactNode;
  className?: string;
  'aria-label'?: string;
}

// ---------------------------------------------------------------------------
// Alignment helpers
// ---------------------------------------------------------------------------

const ALIGN_JUSTIFY: Record<NonNullable<Column<unknown>['align']>, string> = {
  left: 'justify-start',
  center: 'justify-center',
  right: 'justify-end',
};

const ALIGN_TEXT: Record<NonNullable<Column<unknown>['align']>, string> = {
  left: 'text-left',
  center: 'text-center',
  right: 'text-right',
};

// ---------------------------------------------------------------------------
// DataTable
// ---------------------------------------------------------------------------

/**
 * Generic list-table component. Pass columns, data, and optionally a
 * `<CursorPagination>` footer. Virtualization is deferred (see doc above).
 */
export function DataTable<T>({
  columns,
  data,
  getRowId,
  isLoading = false,
  skeletonRows = 6,
  empty,
  onRowClick,
  rowActions,
  selectable = false,
  selectedIds = [],
  onSelectionChange,
  footer,
  className,
  'aria-label': ariaLabel,
}: DataTableProps<T>): React.ReactElement {
  // Total synthetic columns for skeleton column count.
  const totalColumns = columns.length + (selectable ? 1 : 0) + (rowActions ? 1 : 0);

  // Derived selection state.
  const allVisibleIds = data.map(getRowId);
  const selectedSet = new Set(selectedIds);
  const allSelected = allVisibleIds.length > 0 && allVisibleIds.every((id) => selectedSet.has(id));
  const someSelected = !allSelected && allVisibleIds.some((id) => selectedSet.has(id));

  function handleSelectAll(e: React.ChangeEvent<HTMLInputElement>) {
    if (!onSelectionChange) return;
    if (e.target.checked) {
      // Add all visible IDs (union with any pre-existing selection outside this page).
      const merged = Array.from(new Set([...selectedIds, ...allVisibleIds]));
      onSelectionChange(merged);
    } else {
      // Remove all visible IDs, keep anything outside this page.
      const visibleSet = new Set(allVisibleIds);
      onSelectionChange(selectedIds.filter((id) => !visibleSet.has(id)));
    }
  }

  function handleSelectRow(rowId: string, e: React.ChangeEvent<HTMLInputElement>) {
    if (!onSelectionChange) return;
    if (e.target.checked) {
      onSelectionChange([...selectedIds, rowId]);
    } else {
      onSelectionChange(selectedIds.filter((id) => id !== rowId));
    }
  }

  // ---------------------------------------------------------------------------
  // Render helpers
  // ---------------------------------------------------------------------------

  function renderHeaderCell(col: Column<T>) {
    const align = col.align ?? 'left';
    return (
      <div
        key={col.id}
        className={cn(
          'flex items-center px-4 py-[11px]',
          ALIGN_JUSTIFY[align],
          ALIGN_TEXT[align],
          col.width ? 'shrink-0' : 'flex-1',
        )}
        style={col.width ? { width: col.width } : undefined}
      >
        <span className="text-[11px] font-semibold uppercase tracking-[0.5px] text-text-3">
          {col.header}
        </span>
      </div>
    );
  }

  function renderDataCell(col: Column<T>, row: T) {
    const align = col.align ?? 'left';
    return (
      <div
        key={col.id}
        className={cn(
          'flex items-center px-4 py-3 text-[13px] text-text',
          ALIGN_JUSTIFY[align],
          ALIGN_TEXT[align],
          col.width ? 'shrink-0' : 'flex-1',
        )}
        style={col.width ? { width: col.width } : undefined}
      >
        {col.cell(row)}
      </div>
    );
  }

  // ---------------------------------------------------------------------------
  // Body content
  // ---------------------------------------------------------------------------

  let bodyContent: React.ReactNode;

  if (isLoading) {
    bodyContent = Array.from({ length: skeletonRows }, (_, i) => (
      // biome-ignore lint/suspicious/noArrayIndexKey: static placeholder rows — decorative, never reordered
      <SkeletonTableRow key={i} columns={totalColumns} />
    ));
  } else if (data.length === 0) {
    bodyContent = <div className="flex flex-1 items-center justify-center p-8">{empty}</div>;
  } else {
    bodyContent = data.map((row) => {
      const rowId = getRowId(row);
      const isSelected = selectedSet.has(rowId);
      const interactive = !!onRowClick;

      return (
        <div
          key={rowId}
          role={interactive ? 'button' : undefined}
          tabIndex={interactive ? 0 : undefined}
          onClick={interactive ? () => onRowClick(row) : undefined}
          onKeyDown={
            interactive
              ? (e: React.KeyboardEvent<HTMLDivElement>) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    onRowClick(row);
                  }
                }
              : undefined
          }
          className={cn(
            'flex border-b border-border-soft',
            interactive && 'cursor-pointer hover:bg-surface-2',
          )}
        >
          {/* Checkbox cell — stopPropagation lives on the input itself (interactive
              element → no keyboard-handler lint), so a row click isn't triggered. */}
          {selectable && (
            <div className="flex w-11 shrink-0 items-center justify-center px-4 py-3">
              <Checkbox
                checked={isSelected}
                onClick={(e) => e.stopPropagation()}
                onChange={(e) => handleSelectRow(rowId, e)}
                aria-label={`Pilih baris ${rowId}`}
              />
            </div>
          )}

          {/* Data cells */}
          {columns.map((col) => renderDataCell(col, row))}

          {/* Row-action cell. The action node (e.g. a kebab button) should stop its own
              propagation; the wrapper does not intercept clicks. */}
          {rowActions && (
            <div className="flex w-[52px] shrink-0 items-center justify-center">
              {rowActions(row)}
            </div>
          )}
        </div>
      );
    });
  }

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  // Markup is presentational (styled flex grid). Full ARIA-grid semantics (role="grid" with
  // complete row/cell roles) or a native <table> is a deferred a11y follow-up; the column/data
  // abstraction makes either layerable without an API change.
  return (
    <section
      aria-label={ariaLabel}
      className={cn(
        'flex flex-col overflow-hidden rounded-lg border border-border bg-surface',
        className,
      )}
    >
      {/* THead */}
      <div className="sticky top-0 z-10 border-b border-border-soft bg-surface-2">
        <div className="flex">
          {/* Select-all checkbox header */}
          {selectable && (
            <div className="flex w-11 shrink-0 items-center justify-center px-4 py-[11px]">
              <Checkbox
                checked={allSelected}
                aria-checked={someSelected ? 'mixed' : allSelected}
                onChange={handleSelectAll}
                aria-label="Pilih semua baris"
              />
            </div>
          )}

          {/* Column headers */}
          {columns.map((col) => renderHeaderCell(col))}

          {/* Row-action header spacer */}
          {rowActions && <div className="w-[52px] shrink-0 px-4 py-[11px]" aria-hidden="true" />}
        </div>
      </div>

      {/* TBody — scroll container; virtualization-ready (see file doc comment) */}
      <div className="flex flex-col overflow-auto">{bodyContent}</div>

      {/* Footer slot (e.g. <CursorPagination>) — outside the scroll region */}
      {footer && <div>{footer}</div>}
    </section>
  );
}
