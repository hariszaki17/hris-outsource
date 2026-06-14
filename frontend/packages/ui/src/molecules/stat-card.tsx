import type { LucideIcon } from 'lucide-react';
import type * as React from 'react';
import { cn } from '../lib/cn.ts';

/**
 * StatCard (molecule) — metric summary card used in dashboard/list screens.
 * Visual contract: comp/StatCard `lmwet` (.pen). Tokens only (ENGINEERING.md G4).
 * Width is w-full; consumers place the card in a flex/grid cell to control sizing.
 */

export type StatTone = 'brand' | 'ok' | 'warn' | 'bad' | 'info' | 'neutral';

const chipClass: Record<StatTone, string> = {
  brand: 'bg-primary-soft text-primary',
  ok: 'bg-ok-bg text-ok-tx',
  warn: 'bg-warn-bg text-warn-tx',
  bad: 'bg-bad-bg text-bad-tx',
  info: 'bg-info-bg text-info-tx',
  neutral: 'bg-surface-2 text-text-2',
};

export interface StatCardProps {
  label: string;
  value: React.ReactNode;
  sub?: string;
  icon: LucideIcon;
  /** Drives the IconChip tint (background + icon color). Default: 'brand'. */
  tone?: StatTone;
  className?: string;
}

export function StatCard({
  label,
  value,
  sub,
  icon: Icon,
  tone = 'brand',
  className,
}: StatCardProps) {
  return (
    <div
      className={cn(
        'flex w-full flex-col gap-3.5 rounded-lg border border-border bg-surface p-[18px]',
        className,
      )}
    >
      {/* Head row */}
      <div className="flex w-full items-center justify-between">
        <span className="text-[13px] font-medium text-text-2">{label}</span>
        {/* IconChip */}
        <span
          className={cn(
            'flex size-[30px] shrink-0 items-center justify-center rounded-md',
            chipClass[tone],
          )}
          aria-hidden="true"
        >
          <Icon size={17} strokeWidth={2} />
        </span>
      </div>

      {/* Value */}
      <p className="text-[30px] font-bold leading-none text-text">{value}</p>

      {/* Sub */}
      {sub && <p className="text-xs font-medium text-text-3">{sub}</p>}
    </div>
  );
}
