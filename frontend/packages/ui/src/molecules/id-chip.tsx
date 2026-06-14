import type * as React from 'react';
import { cn } from '../lib/cn.ts';

/**
 * IdChip (molecule) — renders an opaque SWP-* id in mono (DESIGN-SYSTEM §3: IDs are IBM Plex Mono).
 * IDs are never parsed; this is display-only.
 */
export interface IdChipProps extends React.HTMLAttributes<HTMLSpanElement> {
  id: string;
}

export function IdChip({ id, className, ...props }: IdChipProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-sm bg-surface-2 px-1.5 py-0.5 font-mono text-xs text-text-2',
        className,
      )}
      {...props}
    >
      {id}
    </span>
  );
}
