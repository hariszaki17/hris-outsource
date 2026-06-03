/**
 * SearchField (atom) — maps to .pen `comp/SearchField` (frame vJBJZ).
 * Row with a search icon + transparent input inside a bordered surface container.
 * ENGINEERING.md G4: no dead-flow states; all copy via props.
 */
import { Search } from 'lucide-react';
import type * as React from 'react';
import { cn } from '../lib/cn.ts';

export interface SearchFieldProps extends React.InputHTMLAttributes<HTMLInputElement> {
  /** Extra classes applied to the outer container box. */
  containerClassName?: string;
}

export function SearchField({ containerClassName, className, ...props }: SearchFieldProps) {
  return (
    <div
      className={cn(
        'flex w-60 items-center gap-2 rounded-md border border-border bg-surface px-3 py-2',
        containerClassName,
      )}
    >
      <Search className="size-[15px] shrink-0 text-text-3" aria-hidden="true" />
      <input
        type="search"
        className={cn(
          'flex-1 border-0 bg-transparent p-0 text-[13px] text-text outline-none',
          'placeholder:text-text-3',
          'focus-visible:outline-none focus-visible:ring-0',
          className,
        )}
        {...props}
      />
    </div>
  );
}
