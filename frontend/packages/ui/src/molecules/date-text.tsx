import { formatDate, formatInstant, formatLocalTime } from '@swp/shared';
import { cn } from '../lib/cn.ts';

/**
 * DateText (molecule) — the ONLY way dates/times render (ENGINEERING.md E4). All formatting
 * goes through @swp/shared's Asia/Jakarta helpers; no `new Date()` in components.
 */
export interface DateTextProps {
  /** kind of value coming from the API (CONVENTIONS §10) */
  kind?: 'instant' | 'date' | 'time';
  value: string;
  className?: string;
  options?: Intl.DateTimeFormatOptions;
}

export function DateText({ kind = 'instant', value, className, options }: DateTextProps) {
  let text: string;
  switch (kind) {
    case 'date':
      text = formatDate(value, options);
      break;
    case 'time':
      text = formatLocalTime(value);
      break;
    default:
      text = formatInstant(value, options);
  }
  return (
    <time
      dateTime={value}
      className={cn('tabular-nums', kind === 'time' && 'font-mono', className)}
    >
      {text}
    </time>
  );
}
