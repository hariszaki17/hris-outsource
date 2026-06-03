/**
 * Toggle (atom) — maps to .pen `comp/Toggle` (frame Uma0O).
 * Accessible switch: 40×22 track (rounded-full), 18px knob (bg-surface).
 * OFF → bg-control-off. ON → bg-primary, knob translate-x-[18px].
 * ENGINEERING.md G4: no dead-flow states; keyboard-accessible via role="switch".
 */
import { cn } from '../lib/cn.ts';

export interface ToggleProps {
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  disabled?: boolean;
  id?: string;
  'aria-label'?: string;
  className?: string;
}

export function Toggle({
  checked,
  onCheckedChange,
  disabled = false,
  id,
  'aria-label': ariaLabel,
  className,
}: ToggleProps) {
  return (
    <button
      id={id}
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={ariaLabel}
      disabled={disabled}
      onClick={() => onCheckedChange(!checked)}
      className={cn(
        // Track
        'inline-flex h-[22px] w-10 shrink-0 cursor-pointer items-center rounded-full p-0.5',
        'transition-colors duration-200',
        checked ? 'bg-primary' : 'bg-control-off',
        // Focus ring
        'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
        // Disabled
        disabled && 'cursor-not-allowed opacity-50',
        className,
      )}
    >
      {/* Knob */}
      <span
        className={cn(
          'size-[18px] rounded-full bg-surface shadow-sm',
          'transition-transform duration-200',
          checked ? 'translate-x-[18px]' : 'translate-x-0',
        )}
      />
    </button>
  );
}
