import { AlertTriangle, Inbox, Loader2, Lock } from 'lucide-react';
import type * as React from 'react';
import { cn } from '../lib/cn.ts';
import { Button } from '../primitives/button.tsx';

/**
 * StateView (molecule) — the canonical async surface states (ENGINEERING.md B2 / G4):
 * loading · empty · error/retry · no-permission. Every data view renders one of these
 * instead of nothing, so "no dead-flow" is structural. Maps to DESIGN-SYSTEM §6.
 */
type StateKind = 'loading' | 'empty' | 'error' | 'no-permission';

const icon: Record<StateKind, React.ComponentType<{ className?: string }>> = {
  loading: Loader2,
  empty: Inbox,
  error: AlertTriangle,
  'no-permission': Lock,
};

export interface StateViewProps {
  kind: StateKind;
  title: string;
  description?: string;
  onRetry?: () => void;
  retryLabel?: string;
  className?: string;
}

export function StateView({
  kind,
  title,
  description,
  onRetry,
  retryLabel = 'Coba lagi',
  className,
}: StateViewProps) {
  const Icon = icon[kind];
  return (
    <div
      className={cn(
        'flex flex-col items-center justify-center gap-3 rounded-lg border border-border bg-surface px-6 py-12 text-center',
        className,
      )}
      role={kind === 'error' ? 'alert' : 'status'}
    >
      <Icon className={cn('size-8 text-text-3', kind === 'loading' && 'animate-spin')} />
      <div>
        <p className="font-semibold text-text">{title}</p>
        {description && <p className="mt-1 text-sm text-text-2">{description}</p>}
      </div>
      {kind === 'error' && onRetry && (
        <Button variant="secondary" size="sm" onClick={onRetry}>
          {retryLabel}
        </Button>
      )}
    </div>
  );
}
