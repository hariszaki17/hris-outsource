import { Slot, Slottable } from '@radix-ui/react-slot';
import type { LucideIcon } from 'lucide-react';
import type * as React from 'react';
import { cn } from '../lib/cn.ts';

/**
 * SettingsSubnav (molecule) — white card secondary navigation for settings screens.
 * Design contract: comp/SettingsSubnav `.pen` frame `WhMQv` (ENGINEERING.md G4 — 1:1 design→code).
 * Presentational and data-driven: nav config (items, icons, hrefs) and active state are
 * supplied entirely by the consuming settings screen. No Bahasa copy is hardcoded here;
 * all labels and the optional cap label come from the consumer via `children` / props.
 */

// ---------------------------------------------------------------------------
// SettingsSubnav — the outer white card container
// ---------------------------------------------------------------------------

export interface SettingsSubnavProps {
  /**
   * Optional all-caps section label rendered above the nav items (e.g. "PENGATURAN").
   * When omitted, no cap label row is rendered.
   */
  label?: string;
  children: React.ReactNode;
  className?: string;
}

/**
 * White 220 px card that wraps settings nav items.
 * Spec: bg-surface, rounded-lg, border border-border, px-3 py-4, flex flex-col, gap-1.
 * Width defaults to w-[220px]; override via `className`.
 */
export function SettingsSubnav({ label, children, className }: SettingsSubnavProps) {
  return (
    <div
      className={cn(
        'flex w-[220px] flex-col gap-1 rounded-lg border border-border bg-surface px-3 py-4',
        className,
      )}
    >
      {label != null && (
        // Cap label row: 11 px, semibold, uppercase, wide tracking, muted text-text-3.
        // Padding: px-2 pt-1.5 pb-2.5 (spec [6,8,10,8] → left/right 8 → px-2; top 6 → pt-1.5; bottom 10 → pb-2.5).
        <p
          className="px-2 pb-2.5 pt-1.5 text-[11px] font-semibold uppercase tracking-[0.6px] text-text-3"
          aria-hidden="true"
        >
          {label}
        </p>
      )}
      {children}
    </div>
  );
}

// ---------------------------------------------------------------------------
// SettingsSubnavItem — individual nav row (default / active)
// ---------------------------------------------------------------------------

export interface SettingsSubnavItemProps extends React.AnchorHTMLAttributes<HTMLAnchorElement> {
  /** Lucide icon component — rendered at 16×16 (size-4). */
  icon: LucideIcon;
  /** Whether this item is the current active route. */
  active?: boolean;
  /**
   * When true, the underlying element is replaced by the direct child via
   * `@radix-ui/react-slot`, exactly like `SidebarNavItem`. Pass a TanStack Router
   * `<Link>` as the single child to keep full router integration.
   */
  asChild?: boolean;
}

/**
 * Single settings nav row: icon + label.
 * Default — icon + label text-text-2, transparent background.
 * Active  — bg-primary-soft + 3 px left border-primary; icon + label text-primary, font-semibold.
 *
 * `asChild` delegates the anchor element to the child (e.g. TanStack Router `<Link>`).
 * The icon is rendered as a sibling of `<Slottable>` so Radix merges all row props onto
 * the consumer's single `<Link>` child and injects the icon inside it — avoids the
 * React.Children.only error that would occur if a raw icon sibling sat next to `{children}`
 * under a bare `<Slot>`. Mirrors `SidebarNavItem` precisely.
 */
export function SettingsSubnavItem({
  icon: Icon,
  active = false,
  asChild = false,
  className,
  children,
  ...props
}: SettingsSubnavItemProps) {
  const Comp = asChild ? Slot : 'a';

  return (
    <Comp
      className={cn(
        // Base row — rounded-md (8 px), gap-2.5 (10 px), px-3 py-2.5 (spec [10,12]).
        // Label text styles live on the row so the label inherits them whether it is a
        // bare text node or a slotted <Link> child — mirrors SidebarNavItem.
        'flex w-full items-center gap-2.5 rounded-md px-3 py-2.5 text-[13px] transition-colors',
        // Default state
        !active && 'font-medium text-text-2 hover:bg-surface',
        // Active state: primary-soft fill + 3 px left accent border + primary text
        active && 'border-l-[3px] border-primary bg-primary-soft font-semibold text-primary',
        className,
      )}
      aria-current={active ? 'page' : undefined}
      {...props}
    >
      {/* Decorative icon — aria-hidden; label conveys meaning. Kept as a sibling of the
          slotted child via <Slottable>: with `asChild`, Radix Slot merges row props onto
          the consumer's <Link> and injects the icon inside it, preserving a single element
          child (avoids React.Children.only). Mirrors SidebarNavItem exactly. */}
      <Icon
        className={cn('size-4 shrink-0', active ? 'text-primary' : 'text-text-2')}
        aria-hidden="true"
      />
      <Slottable>{children}</Slottable>
    </Comp>
  );
}
