import { Check } from 'lucide-react';
import { forwardRef } from 'react';
import type * as React from 'react';
import { cn } from '../lib/cn.ts';

/**
 * Checkbox (atom) — maps to .pen `comp/Checkbox`. 18px, primary-green when checked
 * (DESIGN-SYSTEM §2). Native input for a11y; the box is the visual.
 *
 * forwardRef so callers can set the native `indeterminate` flag (tri-state select-all headers),
 * which has no React prop — e.g. `ref={(el) => { if (el) el.indeterminate = someSelected; }}`.
 */
export interface CheckboxProps
  extends Omit<React.InputHTMLAttributes<HTMLInputElement>, 'type' | 'size'> {
  label?: string;
}

export const Checkbox = forwardRef<HTMLInputElement, CheckboxProps>(function Checkbox(
  { label, className, id, checked, ...props },
  ref,
) {
  return (
    <label htmlFor={id} className={cn('inline-flex cursor-pointer items-center gap-2', className)}>
      <span className="relative inline-flex size-[18px] items-center justify-center">
        <input
          id={id}
          ref={ref}
          type="checkbox"
          checked={checked}
          className="peer size-[18px] cursor-pointer appearance-none rounded-[5px] border border-border bg-surface checked:border-primary checked:bg-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          {...props}
        />
        <Check className="pointer-events-none absolute hidden size-3 text-white peer-checked:block" />
      </span>
      {label && <span className="text-sm text-text-2">{label}</span>}
    </label>
  );
});
