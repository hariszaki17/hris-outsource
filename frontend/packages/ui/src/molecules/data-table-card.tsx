/**
 * DataTableCardView<T> (molecule) — mobile card stack alternative to DataTable.
 *
 * Renders each row as a card instead of a table row, suitable for mobile viewports
 * (&lt;640px). Column `priority` controls which columns appear. Designed to be used
 * alongside DataTable via the responsive prop or independently.
 *
 * ENGINEERING.md G4 — interaction catalogue: mobile list → card view.
 */

import type * as React from 'react';
import { cn } from '../lib/cn.ts';
import type { Column } from './data-table.tsx';

export interface DataTableCardViewProps<T> {
  columns: Column<T>[];
  data: T[];
  getRowId: (row: T) => string;
  onRowClick?: (row: T) => void;
  rowActions?: (row: T) => React.ReactNode;
  empty?: React.ReactNode;
  isLoading?: boolean;
  skeletonRows?: number;
  className?: string;
  'aria-label'?: string;
}

export function DataTableCardView<T>({
  columns,
  data,
  getRowId,
  onRowClick,
  rowActions,
  empty,
  isLoading = false,
  skeletonRows = 6,
  className,
  'aria-label': ariaLabel,
}: DataTableCardViewProps<T>): React.ReactElement {
  const cardColumns = columns.filter((col) => col.priority !== 'hidden-mobile');
  const primaryCol = cardColumns.find((col) => col.priority === 'primary') ?? cardColumns[0];
  const secondaryCols = cardColumns.filter(
    (col) => col !== primaryCol && col.priority !== 'hidden-mobile',
  );

  let bodyContent: React.ReactNode;

  if (isLoading) {
    bodyContent = Array.from({ length: skeletonRows }, (_, i) => (
      <div
        // biome-ignore lint/suspicious/noArrayIndexKey: static placeholder — decorative, never reordered
        key={i}
        className="flex flex-col gap-2 rounded-card border border-border bg-surface p-4"
      >
        <div className="h-5 w-2/3 animate-pulse rounded bg-surface-2" />
        <div className="h-4 w-1/2 animate-pulse rounded bg-surface-2" />
        <div className="h-4 w-1/3 animate-pulse rounded bg-surface-2" />
      </div>
    ));
  } else if (data.length === 0) {
    bodyContent = <div className="flex flex-1 items-center justify-center p-8">{empty}</div>;
  } else {
    bodyContent = data.map((row) => {
      const rowId = getRowId(row);
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
            'flex flex-col gap-2.5 rounded-card border border-border bg-surface p-4',
            interactive && 'cursor-pointer hover:bg-surface-2',
          )}
        >
          {/* Primary column — card title */}
          {primaryCol && (
            <div className="text-[15px] font-bold text-text">{primaryCol.cell(row)}</div>
          )}

          {/* Secondary columns — label:value pairs */}
          {secondaryCols.length > 0 && (
            <div className="flex flex-col gap-1.5">
              {secondaryCols.map((col) => (
                <div key={col.id} className="flex flex-col gap-0.5">
                  <span className="text-[11px] font-semibold text-text-3">{col.header}</span>
                  <span className="text-[13px] text-text">{col.cell(row)}</span>
                </div>
              ))}
            </div>
          )}

          {/* Row actions */}
          {rowActions && (
            <div className="flex items-center justify-end border-t border-border-soft pt-2.5 mt-0.5">
              {rowActions(row)}
            </div>
          )}
        </div>
      );
    });
  }

  return (
    <section aria-label={ariaLabel} className={cn('flex flex-col gap-3', className)}>
      {bodyContent}
    </section>
  );
}
