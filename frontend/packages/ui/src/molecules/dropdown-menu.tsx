/**
 * DropdownMenu — canonical row/action menu primitive.
 *
 * Why this exists: every list screen hand-rolled a kebab menu as an
 * `absolute right-0 top-full` panel nested inside the row. Inside the
 * `overflow-hidden` table card those panels are CLIPPED — the menu renders
 * invisible / cut off (the Master Shift bug). This primitive portals the panel
 * to `document.body` with FIXED positioning anchored to the trigger, so it
 * escapes every ancestor's `overflow`. One canonical component, promoted to
 * @swp/ui per ENGINEERING.md §6 (reuse before building).
 *
 * Behaviour: outside-click + Escape close (ENGINEERING.md document-mousedown
 * pattern); reposition on scroll/resize (capture scroll to catch inner
 * scroll containers); item click auto-closes via context.
 *
 * Tokens only: bg-surface, border-border, shadow-overlay, text-text / text-2,
 *   bg-surface-2, bad-tx/bad-bg, ok-tx/ok-bg. No raw hex. No new npm deps.
 */

import { MoreVertical } from 'lucide-react';
import * as React from 'react';
import { createPortal } from 'react-dom';
import { cn } from '../lib/cn.ts';

// ---------------------------------------------------------------------------
// Context — lets items close the menu without prop-drilling
// ---------------------------------------------------------------------------

interface DropdownMenuContextValue {
  close: () => void;
}

const DropdownMenuContext = React.createContext<DropdownMenuContextValue | null>(null);

// ---------------------------------------------------------------------------
// DropdownMenu
// ---------------------------------------------------------------------------

export interface DropdownMenuProps {
  /** aria-label for the kebab trigger button. */
  triggerLabel: string;
  /** Menu items — use <DropdownMenuItem>. */
  children: React.ReactNode;
  /** Horizontal alignment of the panel relative to the trigger. Default 'end' (right-aligned). */
  align?: 'start' | 'end';
  /** Panel width in px. Default 200. */
  menuWidth?: number;
  /** Extra classes for the trigger button. */
  triggerClassName?: string;
  /** Extra classes for the menu panel. */
  menuClassName?: string;
  /** Icon for the trigger. Default MoreVertical. */
  triggerIcon?: React.ComponentType<{ className?: string; 'aria-hidden'?: boolean }>;
}

interface Coords {
  top: number;
  left: number;
}

export function DropdownMenu({
  triggerLabel,
  children,
  align = 'end',
  menuWidth = 200,
  triggerClassName,
  menuClassName,
  triggerIcon: TriggerIcon = MoreVertical,
}: DropdownMenuProps) {
  const [open, setOpen] = React.useState(false);
  const [coords, setCoords] = React.useState<Coords | null>(null);
  const triggerRef = React.useRef<HTMLButtonElement>(null);
  const panelRef = React.useRef<HTMLDivElement>(null);

  const close = React.useCallback(() => setOpen(false), []);

  // Measure trigger and place the panel. Fixed coords (viewport-relative) so
  // the portal escapes every ancestor overflow.
  const reposition = React.useCallback(() => {
    const el = triggerRef.current;
    if (!el) return;
    const r = el.getBoundingClientRect();
    const top = r.bottom + 4;
    const left = align === 'end' ? r.right - menuWidth : r.left;
    // Clamp horizontally into the viewport (8px gutter).
    const clampedLeft = Math.max(8, Math.min(left, window.innerWidth - menuWidth - 8));
    setCoords({ top, left: clampedLeft });
  }, [align, menuWidth]);

  // Place on open.
  React.useLayoutEffect(() => {
    if (open) reposition();
  }, [open, reposition]);

  // Outside-click (mousedown), Escape, and reposition on scroll/resize.
  React.useEffect(() => {
    if (!open) return;
    const onPointerDown = (e: MouseEvent) => {
      const target = e.target as Node;
      if (triggerRef.current?.contains(target) || panelRef.current?.contains(target)) return;
      setOpen(false);
    };
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setOpen(false);
        triggerRef.current?.focus();
      }
    };
    const onScrollOrResize = () => reposition();
    document.addEventListener('mousedown', onPointerDown);
    document.addEventListener('keydown', onKeyDown);
    // capture:true so scrolling an inner container (the table) also fires.
    window.addEventListener('scroll', onScrollOrResize, true);
    window.addEventListener('resize', onScrollOrResize);
    return () => {
      document.removeEventListener('mousedown', onPointerDown);
      document.removeEventListener('keydown', onKeyDown);
      window.removeEventListener('scroll', onScrollOrResize, true);
      window.removeEventListener('resize', onScrollOrResize);
    };
  }, [open, reposition]);

  const ctx = React.useMemo<DropdownMenuContextValue>(() => ({ close }), [close]);

  return (
    <>
      <button
        ref={triggerRef}
        type="button"
        aria-label={triggerLabel}
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={(e) => {
          e.stopPropagation();
          setOpen((v) => !v);
        }}
        className={cn(
          'inline-flex size-[30px] items-center justify-center rounded-md text-text-2 transition-colors hover:bg-surface-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
          triggerClassName,
        )}
      >
        <TriggerIcon className="size-4" aria-hidden={true} />
      </button>

      {open &&
        coords !== null &&
        createPortal(
          <div
            ref={panelRef}
            role="menu"
            style={{
              position: 'fixed',
              top: coords.top,
              left: coords.left,
              width: menuWidth,
            }}
            className={cn(
              'z-[60] rounded-[10px] border border-border bg-surface p-1.5 shadow-overlay',
              menuClassName,
            )}
          >
            <DropdownMenuContext.Provider value={ctx}>{children}</DropdownMenuContext.Provider>
          </div>,
          document.body,
        )}
    </>
  );
}

// ---------------------------------------------------------------------------
// DropdownMenuItem
// ---------------------------------------------------------------------------

export type DropdownMenuItemTone = 'default' | 'danger' | 'ok';

export interface DropdownMenuItemProps {
  /** Called after the menu closes. */
  onSelect: () => void;
  children: React.ReactNode;
  /** Optional leading icon. */
  icon?: React.ComponentType<{ className?: string; 'aria-hidden'?: boolean }>;
  /** Visual tone. 'danger' for destructive (deactivate), 'ok' for restorative (reactivate). */
  tone?: DropdownMenuItemTone;
  disabled?: boolean;
  className?: string;
}

const ITEM_TONE: Record<DropdownMenuItemTone, string> = {
  default: 'text-text hover:bg-surface-2',
  danger: 'text-bad-tx hover:bg-bad-bg',
  ok: 'text-ok-tx hover:bg-ok-bg',
};

export function DropdownMenuItem({
  onSelect,
  children,
  icon: Icon,
  tone = 'default',
  disabled = false,
  className,
}: DropdownMenuItemProps) {
  const ctx = React.useContext(DropdownMenuContext);

  return (
    <button
      type="button"
      role="menuitem"
      disabled={disabled}
      onClick={() => {
        ctx?.close();
        onSelect();
      }}
      className={cn(
        'flex w-full items-center gap-2 rounded-[7px] px-3 py-[10px] text-[13px] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
        ITEM_TONE[tone],
        disabled && 'cursor-not-allowed opacity-50',
        className,
      )}
    >
      {Icon && <Icon className="size-[14px]" aria-hidden={true} />}
      {children}
    </button>
  );
}
