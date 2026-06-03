import type { LucideIcon } from 'lucide-react';
import type * as React from 'react';
import { cn } from '../lib/cn.ts';

/**
 * NotifCard (molecule) — maps to .pen `comp/NotifCardUnread` `CQBqd` + `comp/NotifCardRead`
 * `zTbmw` (G3 — one canonical card, the read/unread split is a `read` prop, not two components).
 *
 * Unread: left 4px primary border, info-toned icon chip, bold title, trailing unread dot.
 * Read:   1px border, muted icon chip, de-emphasised title, no dot.
 *
 * The card is a real <button> when `onClick` is given (keyboard + a11y for free).
 */
export interface NotifCardProps {
  icon: LucideIcon;
  title: string;
  body: string;
  /** Relative time label (already localised by the caller, e.g. "2 menit lalu"). */
  time: string;
  /** Unread → primary accent + dot; read → muted. Default false (read). */
  unread?: boolean;
  onClick?: () => void;
  className?: string;
}

export function NotifCard({
  icon: Icon,
  title,
  body,
  time,
  unread = false,
  onClick,
  className,
}: NotifCardProps) {
  const Comp: React.ElementType = onClick ? 'button' : 'div';
  return (
    <Comp
      {...(onClick ? { type: 'button', onClick } : {})}
      className={cn(
        'flex w-full items-start gap-3 rounded-xl bg-surface p-3.5 text-left',
        unread ? 'border-l-4 border-primary' : 'border border-border',
        onClick && 'transition-colors hover:bg-surface-2',
        className,
      )}
    >
      <span
        className={cn(
          'flex size-[34px] shrink-0 items-center justify-center rounded-lg',
          unread ? 'bg-info-bg text-info-tx' : 'bg-surface-2 text-text-3',
        )}
      >
        <Icon className="size-[18px]" aria-hidden="true" />
      </span>
      <span className="flex min-w-0 flex-1 flex-col gap-[3px]">
        <span
          className={cn(
            'text-[13px]',
            unread ? 'font-bold text-text' : 'font-semibold text-text-2',
          )}
        >
          {title}
        </span>
        <span className={cn('text-[12px] leading-[1.4]', unread ? 'text-text-2' : 'text-text-3')}>
          {body}
        </span>
        <span className="text-[11px] font-medium text-text-3">{time}</span>
      </span>
      {unread && (
        <span className="mt-1 size-2 shrink-0 rounded-full bg-primary" aria-hidden="true" />
      )}
    </Comp>
  );
}
