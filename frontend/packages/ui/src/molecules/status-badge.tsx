import { type StatusTone, attendanceTone, placementTone } from '@swp/design-tokens';
import type * as React from 'react';
import { cn } from '../lib/cn.ts';

/**
 * StatusBadge (molecule) — the ONLY sanctioned source of status coloring (ENGINEERING.md G4).
 * Tones map to DESIGN-SYSTEM §2 status palette. Never set status colors inline elsewhere.
 *
 * The `.pen` `comp/StatusPill` `qxONU` (dot + label) is NOT a separate component (G3 — one
 * canonical status concept): it is this badge with `dot` enabled. The dot inherits the tone's
 * text color via `bg-current`.
 */
const toneClass: Record<StatusTone, string> = {
  ok: 'bg-ok-bg text-ok-tx border-ok-bd',
  warn: 'bg-warn-bg text-warn-tx border-warn-bd',
  bad: 'bg-bad-bg text-bad-tx border-bad-bd',
  info: 'bg-info-bg text-info-tx border-info-bd',
  onprogress: 'bg-orange-bg text-orange-tx border-orange-bd',
  neutral: 'bg-surface-2 text-text-2 border-border',
};

export interface StatusBadgeProps extends React.HTMLAttributes<HTMLSpanElement> {
  tone: StatusTone;
  /** Render a leading status dot (the `.pen` `comp/StatusPill` form). */
  dot?: boolean;
}

export function StatusBadge({
  tone,
  dot = false,
  className,
  children,
  ...props
}: StatusBadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 whitespace-nowrap rounded-full border px-2 py-0.5 text-xs font-medium',
        toneClass[tone],
        className,
      )}
      {...props}
    >
      {dot && <span className="size-1.5 shrink-0 rounded-full bg-current" aria-hidden="true" />}
      {children}
    </span>
  );
}

/** Resolve an attendance status enum/label to its tone (DESIGN-SYSTEM §2). */
export function toneForAttendance(status: string): StatusTone {
  return attendanceTone[status] ?? 'neutral';
}

/** Resolve a placement status enum to its tone (E3, DESIGN-SYSTEM §2). */
export function toneForPlacement(status: string): StatusTone {
  return placementTone[status] ?? 'neutral';
}
