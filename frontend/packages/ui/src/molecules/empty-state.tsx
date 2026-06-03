import { Clock3, Inbox, SearchX, ShieldX } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import type * as React from 'react';
import { cn } from '../lib/cn.ts';

/**
 * EmptyState (molecule) — canonical empty / filtered-zero / no-permission / session-expired surface.
 *
 * .pen ids: comp/EmptyState `WTymt` · comp/EmptyFilteredZero `BNr4w` · comp/EmptyFresh `mrACi`
 *           comp/EmptyNoPermission `MRbzz` · comp/EmptySessionExpired `iwcgE`
 *
 * ENGINEERING.md B2 — no dead-flow states: empty/filtered-zero/no-permission/session-expired are
 * first-class states that must always be designed and rendered (never a blank div).
 * ENGINEERING.md G3/G4 — this is the canonical empty/permission/session surface; supersedes
 * state-view's plain `empty` and `no-permission` variants for real screens (state-view retains
 * the loading/error/retry contract).
 *
 * All copy (title / description / hint) is supplied by the consumer — no Bahasa is baked in.
 * The `action` slot accepts any ReactNode (typically a `<Button>`) — no Button import needed here.
 */

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type EmptyVariant = 'default' | 'filtered' | 'fresh' | 'no-permission' | 'session-expired';

export interface EmptyStateProps {
  /** Selects the icon-wrap tint and the default icon. Defaults to `'default'`. */
  variant?: EmptyVariant;
  /** Overrides the default icon for the selected variant. */
  icon?: LucideIcon;
  /** Card heading — required. Supply via i18n; no default copy is baked in. */
  title: string;
  /** Supporting copy below the title. */
  description?: string;
  /** Extra small hint line — intended for `no-permission` but available on all variants. */
  hint?: string;
  /** CTA slot rendered below the description — pass a `<Button>` or any ReactNode. */
  action?: React.ReactNode;
  className?: string;
}

// ---------------------------------------------------------------------------
// Per-variant config: default icon + icon-wrap tint classes
// ---------------------------------------------------------------------------

const variantConfig: Record<EmptyVariant, { defaultIcon: LucideIcon; wrapClass: string }> = {
  default: {
    defaultIcon: Inbox,
    wrapClass: 'bg-surface-2 text-text-3',
  },
  filtered: {
    defaultIcon: SearchX,
    wrapClass: 'bg-surface-2 text-text-3',
  },
  fresh: {
    defaultIcon: Inbox,
    wrapClass: 'bg-primary-soft text-primary',
  },
  'no-permission': {
    defaultIcon: ShieldX,
    wrapClass: 'bg-bad-bg text-bad-tx',
  },
  'session-expired': {
    defaultIcon: Clock3,
    wrapClass: 'bg-warn-bg text-warn-tx',
  },
};

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function EmptyState({
  variant = 'default',
  icon: IconOverride,
  title,
  description,
  hint,
  action,
  className,
}: EmptyStateProps) {
  const { defaultIcon: DefaultIcon, wrapClass } = variantConfig[variant];
  const Icon: LucideIcon = IconOverride ?? DefaultIcon;

  return (
    <div
      className={cn(
        // Card shell — mirrors .pen card spec: bg-surface, rounded-lg, border-border-soft,
        // px-8 py-9 (≈32px / 36px), flex col centered, gap-2.5, text-center.
        // max-w-[360px] mx-auto matches the .pen 360px frame width, centering in a content area.
        'flex flex-col items-center justify-center gap-2.5 rounded-lg border border-border-soft bg-surface px-8 py-9 text-center max-w-[360px] mx-auto',
        className,
      )}
    >
      {/* Icon wrap — size-[52px] rounded-full, tinted per variant; icon is size-6 (24px) */}
      <div
        className={cn(
          'flex size-[52px] shrink-0 items-center justify-center rounded-full',
          wrapClass,
        )}
      >
        <Icon className="size-6" />
      </div>

      {/* Text block */}
      <div className="flex flex-col items-center gap-1">
        <p className="text-sm font-bold text-text">{title}</p>
        {description && <p className="text-xs leading-[1.4] text-text-3">{description}</p>}
        {hint && <p className="text-[11px] font-medium text-text-3">{hint}</p>}
      </div>

      {/* CTA slot — consumer-supplied ReactNode (e.g. <Button>); nullish guard avoids
          rendering falsy non-null values (e.g. 0) that && would leak (rendering-conditional-render) */}
      {action != null ? <div className="flex justify-center">{action}</div> : null}
    </div>
  );
}
