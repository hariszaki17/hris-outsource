import type * as React from 'react';
import { cn } from '../lib/cn.ts';

/** Input (atom). Token-driven; error state via `aria-invalid`. */
export type InputProps = React.InputHTMLAttributes<HTMLInputElement>;

export function Input({ className, ...props }: InputProps) {
  return (
    <input
      className={cn(
        'flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm text-text',
        'placeholder:text-text-3 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
        'disabled:cursor-not-allowed disabled:opacity-50',
        'aria-[invalid=true]:border-bad-bd aria-[invalid=true]:ring-bad-bd',
        className,
      )}
      {...props}
    />
  );
}
