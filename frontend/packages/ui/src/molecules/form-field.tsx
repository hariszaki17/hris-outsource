import type * as React from 'react';
import { cn } from '../lib/cn.ts';

/**
 * FormField (molecule) — label + control slot + field error, in the DESIGN-SYSTEM §6
 * 2-column grid rhythm. The `error` prop is fed straight from the API `error.fields`
 * map (ENGINEERING.md B1) or React Hook Form. `span` lets a field occupy the full row.
 */
export interface FormFieldProps {
  label: string;
  htmlFor: string;
  error?: string;
  required?: boolean;
  hint?: string;
  span?: 1 | 2;
  className?: string;
  children: React.ReactNode;
}

export function FormField({
  label,
  htmlFor,
  error,
  required,
  hint,
  span = 1,
  className,
  children,
}: FormFieldProps) {
  return (
    <div className={cn('flex flex-col gap-1.5', span === 2 && 'col-span-2', className)}>
      <label htmlFor={htmlFor} className="text-sm font-semibold text-text">
        {label}
        {required && <span className="ml-0.5 text-bad-tx">*</span>}
      </label>
      {children}
      {hint && !error && <p className="text-xs text-text-3">{hint}</p>}
      {error && (
        <p id={`${htmlFor}-error`} role="alert" className="text-xs text-bad-tx">
          {error}
        </p>
      )}
    </div>
  );
}

/** The 2-column form section wrapper enforcing the DESIGN-SYSTEM §6 grid. */
export function FormSection({
  className,
  children,
}: { className?: string; children: React.ReactNode }) {
  return <div className={cn('grid grid-cols-2 gap-4', className)}>{children}</div>;
}
