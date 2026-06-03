/**
 * Skeleton family (molecules) — comp/SkeletonLine `jcW4k`, comp/SkeletonAvatar `e3rdpj`,
 * comp/SkeletonCard `NmWCA`, comp/SkeletonTableRow `PRMOL`.
 *
 * ENGINEERING.md B2: loading is a first-class state — no dead-flow; every async surface
 * must render a skeleton while data is in-flight.
 * ENGINEERING.md G4: design-system→code is 1:1; shimmer primitives mirror comp/* exactly.
 *
 * `Skeleton` is the canonical shimmer block. SkeletonLine and SkeletonAvatar are not
 * separate components — they are just className variants of `Skeleton`:
 *   SkeletonLine  → <Skeleton />               (default h-3 w-[200px])
 *   SkeletonAvatar → <Skeleton circle className="size-9" />
 *
 * Tokens only: bg-border-soft for fill, bg-surface for card/row backgrounds,
 * border-border / border-border-soft for outlines. Cell widths use inline style
 * (pixel values that cannot be expressed as Tailwind arbitrary values in a static
 * build without safelisting) — this is the only intentional inline style usage.
 */

import type * as React from 'react';
import { cn } from '../lib/cn.ts';

// ---------------------------------------------------------------------------
// Skeleton — canonical shimmer primitive
// ---------------------------------------------------------------------------

export interface SkeletonProps {
  /** Extra Tailwind classes; use h-* / w-* to set dimensions. */
  className?: string;
  /** Render as a full circle (rounded-full). Default: rounded (4 px). */
  circle?: boolean;
  /** Inline style — used for dynamic pixel dimensions (e.g. table-cell widths). */
  style?: React.CSSProperties;
}

/**
 * Skeleton renders a single animated shimmer block.
 *
 * Default size (when no className sets height/width): h-3 w-[200px] — matches
 * comp/SkeletonLine `jcW4k` (12 px tall, 200 px wide).
 *
 * Pass `circle` + `className="size-9"` to match comp/SkeletonAvatar `e3rdpj`
 * (36×36 px circle).
 */
export function Skeleton({ className, circle = false, style }: SkeletonProps) {
  return (
    <div
      aria-hidden="true"
      style={style}
      className={cn(
        'bg-border-soft animate-pulse',
        circle ? 'rounded-full' : 'rounded',
        // Default line dimensions (SkeletonLine `jcW4k`): overridden by className.
        !className?.includes('h-') && 'h-3',
        !className?.includes('w-') && 'w-[200px]',
        className,
      )}
    />
  );
}

// ---------------------------------------------------------------------------
// SkeletonCard — comp/SkeletonCard `NmWCA`
// ---------------------------------------------------------------------------

export interface SkeletonCardProps {
  className?: string;
}

/**
 * SkeletonCard composes one avatar shimmer + three line shimmers in a card
 * shell. Matches comp/SkeletonCard `NmWCA` exactly:
 *   - frame: w-[520px], rounded-lg, bg-surface, border border-border, p-3.5, gap-3.5
 *   - avatar column: size-9 circle (36×36)
 *   - text column: gap-2.5; lines w-40 h-3 / w-24 h-2.5 / w-full h-2.5
 */
export function SkeletonCard({ className }: SkeletonCardProps) {
  return (
    <div
      aria-hidden="true"
      role="presentation"
      className={cn(
        'flex flex-row items-start gap-3.5 rounded-lg border border-border bg-surface p-3.5',
        'w-[520px]',
        className,
      )}
    >
      {/* Avatar shimmer — comp/SkeletonAvatar `e3rdpj` */}
      <Skeleton circle className="size-9 shrink-0" />

      {/* Text column */}
      <div className="flex flex-1 flex-col gap-2.5">
        {/* Line 1: w-40 h-3 */}
        <Skeleton className="h-3 w-40" />
        {/* Line 2: w-24 h-2.5 */}
        <Skeleton className="h-2.5 w-24" />
        {/* Line 3: full-width h-2.5 */}
        <Skeleton className="h-2.5 w-full" />
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// SkeletonTableRow — comp/SkeletonTableRow `PRMOL`
// ---------------------------------------------------------------------------

/** Default cell widths (px) cycling from comp/SkeletonTableRow `PRMOL`. */
const DEFAULT_CELL_WIDTHS: readonly number[] = [172, 112, 92, 132, 112, 92];

export interface SkeletonTableRowProps {
  /** Number of cells. Default: 6. */
  columns?: number;
  /**
   * Pixel widths for the shimmer line in each cell. Cycles if shorter than
   * `columns`. Default: [172, 112, 92, 132, 112, 92].
   */
  cellWidths?: number[];
  className?: string;
}

/**
 * SkeletonTableRow renders a single placeholder row for a data table.
 * Matches comp/SkeletonTableRow `PRMOL`:
 *   - row: w-full flex bg-surface border-b border-border-soft
 *   - each cell: px-4 py-3.5, holds one h-2.5 shimmer line
 *   - 6 cells with widths [172, 112, 92, 132, 112, 92] px (inline style)
 */
export function SkeletonTableRow({
  columns = 6,
  cellWidths = DEFAULT_CELL_WIDTHS as number[],
  className,
}: SkeletonTableRowProps) {
  return (
    <div
      aria-hidden="true"
      role="presentation"
      className={cn('flex w-full border-b border-border-soft bg-surface', className)}
    >
      {Array.from({ length: columns }, (_, i) => {
        const width = cellWidths[i % cellWidths.length];
        return (
          // biome-ignore lint/suspicious/noArrayIndexKey: static decorative placeholder row, never reordered
          <div key={i} className="flex flex-1 items-center px-4 py-3.5">
            <Skeleton className="h-2.5" style={{ width } as React.CSSProperties} />
          </div>
        );
      })}
    </div>
  );
}
