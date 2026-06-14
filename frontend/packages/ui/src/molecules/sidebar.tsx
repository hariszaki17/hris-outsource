import { Slot, Slottable } from '@radix-ui/react-slot';
import type { LucideIcon } from 'lucide-react';
import type * as React from 'react';
import { cn } from '../lib/cn.ts';

/**
 * Sidebar (molecule) — dark vertical navigation shell.
 * Design contract: comp/Sidebar `.pen` frame `iCqTB` (ENGINEERING.md G4 — 1:1 design→code).
 * Presentational and data-driven: nav config (items, icons, hrefs) and active state are
 * supplied entirely by the app shell from the x-rbac permission map. No Bahasa copy is
 * hardcoded here; all labels come from the consumer via `children` / props.
 */

// ---------------------------------------------------------------------------
// Sidebar — the outer <aside> container
// ---------------------------------------------------------------------------

export interface SidebarProps {
  className?: string;
  children?: React.ReactNode;
}

/** Dark 240 px nav container. Use `h-screen` or `h-full` from the parent layout. */
export function Sidebar({ className, children }: SidebarProps) {
  return (
    <aside
      className={cn(
        // w-60 = 240 px; bg-sidebar = #18181B token; overflow-hidden clips the active border
        'flex h-full w-60 flex-col overflow-hidden bg-sidebar',
        className,
      )}
    >
      {children}
    </aside>
  );
}

// ---------------------------------------------------------------------------
// SidebarBrand — logo mark + title/subtitle block
// ---------------------------------------------------------------------------

export interface SidebarBrandProps {
  /** Optional logo node rendered inside the 32×32 white mark square. */
  logo?: React.ReactNode;
  /** Product / system name — shown as bold white title. */
  title: string;
  /** Optional second line below the title (muted, smaller). */
  subtitle?: string;
  className?: string;
}

/**
 * Brand block: 32×32 white rounded mark + vertical title/subtitle text.
 * Spec: px-5 py-5, gap-3, items-center (horizontal row).
 */
export function SidebarBrand({ logo, title, subtitle, className }: SidebarBrandProps) {
  return (
    <div className={cn('flex items-center gap-3 px-5 py-5', className)}>
      {/* 32×32 white rounded mark — holds logo image or first-char fallback */}
      <div
        className="flex size-8 shrink-0 items-center justify-center rounded-md bg-white p-1"
        aria-hidden="true"
      >
        {logo ?? (
          <span className="text-[13px] font-bold leading-none text-primary">
            {title.charAt(0).toUpperCase()}
          </span>
        )}
      </div>

      {/* Vertical title + subtitle */}
      <div className="flex flex-col gap-px">
        <span className="text-[15px] font-bold leading-tight text-white">{title}</span>
        {subtitle && (
          <span className="text-[11px] font-medium leading-tight text-sidebar-text">
            {subtitle}
          </span>
        )}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// SidebarSectionLabel — "MENU" / section caption
// ---------------------------------------------------------------------------

export interface SidebarSectionLabelProps {
  children?: React.ReactNode;
  className?: string;
}

/**
 * All-caps muted section caption.
 * Spec: text-[11px] font-semibold tracking-[0.6px] uppercase text-text-2; px-5 pt-3.5 pb-2.
 */
export function SidebarSectionLabel({ children, className }: SidebarSectionLabelProps) {
  return (
    <p
      className={cn(
        'px-5 pb-2 pt-3.5 text-[11px] font-semibold uppercase tracking-[0.6px] text-text-2',
        className,
      )}
    >
      {children}
    </p>
  );
}

// ---------------------------------------------------------------------------
// SidebarNavItem — individual nav row (default / active)
// ---------------------------------------------------------------------------

export interface SidebarNavItemProps extends React.AnchorHTMLAttributes<HTMLAnchorElement> {
  /** Lucide icon component — rendered at 18×18. */
  icon: LucideIcon;
  /** Whether this item is the current active route. */
  active?: boolean;
  /**
   * When true, the underlying element is replaced by the direct child via
   * `@radix-ui/react-slot`, exactly like `Button`. Pass a TanStack Router
   * `<Link>` as the single child to keep full router integration.
   */
  asChild?: boolean;
}

/**
 * Single nav row: icon + label.
 * Default  — icon + text in `text-sidebar-text`.
 * Active   — `bg-sidebar-hover` + 3 px left `border-primary`; icon + text go white.
 * `asChild` delegates the anchor element to the child (e.g. TanStack Router `<Link>`).
 */
export function SidebarNavItem({
  icon: Icon,
  active = false,
  asChild = false,
  className,
  children,
  ...props
}: SidebarNavItemProps) {
  const Comp = asChild ? Slot : 'a';

  return (
    <Comp
      className={cn(
        // Base row — no border-radius per spec. Label text styles live on the row so the
        // label inherits them whether it is a bare text node or a slotted <Link> child.
        'flex w-full items-center gap-3 px-5 py-2.5 text-sm transition-colors',
        // Default state
        !active && 'font-medium text-sidebar-text hover:bg-sidebar-hover',
        // Active state: filled bg + 3 px left accent border
        active && 'border-l-[3px] border-primary bg-sidebar-hover font-semibold text-white',
        className,
      )}
      aria-current={active ? 'page' : undefined}
      {...props}
    >
      {/* Decorative icon — aria-hidden because the label already conveys meaning. Kept as a
          sibling of the slotted child via <Slottable>: with `asChild`, Radix Slot merges row
          props onto the consumer's <Link> and injects the icon inside it, so a single element
          child is preserved (avoids React.Children.only). */}
      <Icon
        className={cn('size-[18px] shrink-0', active ? 'text-white' : 'text-sidebar-text')}
        aria-hidden="true"
      />
      <Slottable>{children}</Slottable>
    </Comp>
  );
}

// ---------------------------------------------------------------------------
// SidebarSpacer — flex-1 gap pusher
// ---------------------------------------------------------------------------

export interface SidebarSpacerProps {
  className?: string;
}

/** Flex-grow spacer that pushes `SidebarFooter` to the bottom of the sidebar. */
export function SidebarSpacer({ className }: SidebarSpacerProps) {
  return <div className={cn('flex-1', className)} aria-hidden="true" />;
}

// ---------------------------------------------------------------------------
// SidebarFooter — bottom-anchored wrapper with top divider
// ---------------------------------------------------------------------------

export interface SidebarFooterProps {
  children?: React.ReactNode;
  className?: string;
}

/**
 * Footer wrapper with a 1 px top divider (`border-sidebar-hover`).
 * Spec: border-t border-sidebar-hover; inner item uses px-5 pt-3.5 pb-[18px].
 * The consumer places a `SidebarNavItem` (or any node) inside.
 */
export function SidebarFooter({ children, className }: SidebarFooterProps) {
  return <div className={cn('border-t border-sidebar-hover', className)}>{children}</div>;
}
