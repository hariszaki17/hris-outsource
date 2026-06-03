/**
 * FilterSelect (atom) — maps to .pen `comp/FilterSelect` (frame t60nEC).
 * Styled native <select> with an absolutely-positioned chevron icon.
 * Native select: zero new deps, full a11y, keyboard-accessible out of the box.
 * ENGINEERING.md G4: no dead-flow states; all copy via props / children.
 */
import { ChevronDown } from 'lucide-react';
import type * as React from 'react';
import { cn } from '../lib/cn.ts';

export interface FilterSelectProps extends React.SelectHTMLAttributes<HTMLSelectElement> {
  /** Extra classes applied to the outer container box. */
  containerClassName?: string;
}

export function FilterSelect({
  containerClassName,
  className,
  children,
  ...props
}: FilterSelectProps) {
  return (
    <div
      className={cn(
        'relative w-44 rounded-md border border-border bg-surface px-3 py-2',
        containerClassName,
      )}
    >
      <select
        className={cn(
          'w-full appearance-none border-0 bg-transparent pr-6 text-[13px] font-medium text-text-2',
          'outline-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
          'disabled:cursor-not-allowed disabled:opacity-50',
          className,
        )}
        {...props}
      >
        {children}
      </select>
      <ChevronDown
        className="pointer-events-none absolute right-3 top-1/2 size-[15px] -translate-y-1/2 text-text-3"
        aria-hidden="true"
      />
    </div>
  );
}
