/**
 * Topbar molecule family — comp/Topbar (.pen frame `caFkE`).
 * ENGINEERING.md G4: breadcrumb and user data are supplied by the app shell;
 * this component is purely presentational.
 *
 * Sub-components (all named exports):
 *   Topbar          — <header> shell (flex, h-16, px-6, border-b)
 *   Breadcrumb      — optional hamburger + crumb list with chevron separators
 *   TopbarSearch    — search box with leading icon + transparent <input>
 *   TopbarIconButton — 36×36 icon-only button (e.g. bell)
 *   TopbarUser      — Avatar + name/role + chevron-down button
 */

import { Bell, ChevronDown, ChevronRight, Menu, Search } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import type * as React from 'react';
import { cn } from '../lib/cn.ts';
import { Avatar } from './avatar.tsx';

// ---------------------------------------------------------------------------
// Topbar — container
// ---------------------------------------------------------------------------

export interface TopbarProps {
  left?: React.ReactNode;
  right?: React.ReactNode;
  className?: string;
}

export function Topbar({ left, right, className }: TopbarProps) {
  return (
    <header
      className={cn(
        'flex h-16 items-center justify-between border-b border-border bg-surface px-6',
        className,
      )}
    >
      {left}
      {right}
    </header>
  );
}

// ---------------------------------------------------------------------------
// Breadcrumb — optional hamburger + crumb items
// ---------------------------------------------------------------------------

export interface BreadcrumbItem {
  label: string;
  current?: boolean;
}

export interface BreadcrumbProps {
  items: BreadcrumbItem[];
  onMenuClick?: () => void;
  /** aria-label for the hamburger button. Default: 'Menu' */
  menuLabel?: string;
  className?: string;
}

export function Breadcrumb({ items, onMenuClick, menuLabel = 'Menu', className }: BreadcrumbProps) {
  return (
    <div className={cn('flex items-center gap-4', className)}>
      {onMenuClick && (
        <button
          type="button"
          aria-label={menuLabel}
          onClick={onMenuClick}
          className="flex items-center justify-center rounded-md p-1 text-text-2 hover:bg-app focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <Menu className="size-5" aria-hidden />
        </button>
      )}

      <nav aria-label="breadcrumb">
        <ol className="flex items-center gap-2">
          {items.map((item, index) => (
            <li key={`${item.label}-${index}`} className="flex items-center gap-2">
              {index > 0 && <ChevronRight className="size-[15px] text-text-3" aria-hidden />}
              <span
                aria-current={item.current ? 'page' : undefined}
                className={cn(
                  'text-[13px]',
                  item.current ? 'font-semibold text-text' : 'font-medium text-text-2',
                )}
              >
                {item.label}
              </span>
            </li>
          ))}
        </ol>
      </nav>
    </div>
  );
}

// ---------------------------------------------------------------------------
// TopbarSearch — search box with leading icon
// ---------------------------------------------------------------------------

export type TopbarSearchProps = React.InputHTMLAttributes<HTMLInputElement>;

export function TopbarSearch({ className, ...inputProps }: TopbarSearchProps) {
  return (
    <div
      className={cn(
        'flex w-[220px] items-center gap-2 rounded-md border border-border bg-app px-3 py-2',
        className,
      )}
    >
      <Search className="size-[15px] shrink-0 text-text-3" aria-hidden />
      <input
        className="bg-transparent text-[13px] text-text outline-none placeholder:text-text-3"
        {...inputProps}
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// TopbarIconButton — 36×36 icon-only button (e.g. notification bell)
// ---------------------------------------------------------------------------

export interface TopbarIconButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  icon: LucideIcon;
  /** Accessible label — required for icon-only buttons. */
  label: string;
}

export function TopbarIconButton({
  icon: Icon,
  label,
  className,
  ...buttonProps
}: TopbarIconButtonProps) {
  return (
    <button
      type="button"
      aria-label={label}
      className={cn(
        'flex size-9 items-center justify-center rounded-md bg-app text-text-2 hover:bg-border focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
        className,
      )}
      {...buttonProps}
    >
      <Icon className="size-[18px]" aria-hidden />
    </button>
  );
}

// ---------------------------------------------------------------------------
// TopbarUser — Avatar + name / role + chevron-down
// ---------------------------------------------------------------------------

export interface TopbarUserProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  name: string;
  /** Secondary line under the name (e.g. the user's RBAC role label). Named `roleLabel`,
   * not `role`, to avoid shadowing the DOM/ARIA `role` attribute on the button. */
  roleLabel?: string;
  initials: string;
}

export function TopbarUser({
  name,
  roleLabel,
  initials,
  className,
  ...buttonProps
}: TopbarUserProps) {
  return (
    <button
      type="button"
      className={cn(
        'flex items-center gap-2.5 rounded-md p-1 hover:bg-app focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
        className,
      )}
      {...buttonProps}
    >
      <Avatar initials={initials} size={34} tone="neutral" shape="circle" />

      <div className="flex flex-col gap-px text-left">
        <span className="text-[13px] font-semibold text-text leading-none">{name}</span>
        {roleLabel && (
          <span className="text-[11px] font-medium text-text-3 leading-none">{roleLabel}</span>
        )}
      </div>

      <ChevronDown className="size-[15px] text-text-3" aria-hidden />
    </button>
  );
}

// ---------------------------------------------------------------------------
// Convenience re-export of Bell for the standard notification button
// ---------------------------------------------------------------------------
export { Bell as TopbarBellIcon };
