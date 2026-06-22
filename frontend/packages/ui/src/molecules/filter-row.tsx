import type { ReactNode } from 'react';
import { cn } from '../lib/cn.ts';

interface FilterRowProps {
  children: ReactNode;
  className?: string;
}

export function FilterRow({ children, className }: FilterRowProps) {
  return (
    <div
      className={cn(
        'flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-end',
        className,
      )}
    >
      {children}
    </div>
  );
}
