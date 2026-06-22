import type { ReactNode } from 'react';
import { cn } from '../lib/cn.ts';

interface StatCardGridProps {
  children: ReactNode;
  className?: string;
  /** Number of columns on desktop (lg+). Default 4. */
  columns?: 2 | 3 | 4;
}

export function StatCardGrid({ children, className, columns = 4 }: StatCardGridProps) {
  const desktopCols =
    columns === 2 ? 'lg:grid-cols-2' : columns === 3 ? 'lg:grid-cols-3' : 'lg:grid-cols-4';
  return (
    <div className={cn('grid grid-cols-2 gap-3', desktopCols, className)}>
      {children}
    </div>
  );
}
