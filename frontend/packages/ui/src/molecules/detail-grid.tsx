import type { ReactNode } from 'react';
import { cn } from '../lib/cn.ts';

interface DetailGridProps {
  children: ReactNode;
  className?: string;
}

export function DetailGrid({ children, className }: DetailGridProps) {
  return (
    <div className={cn('grid grid-cols-1 gap-4 lg:grid-cols-2 lg:gap-5', className)}>
      {children}
    </div>
  );
}
