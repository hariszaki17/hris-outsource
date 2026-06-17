/**
 * Combobox — generic async-search single-select primitive.
 *
 * Design source: .pen comp masters ZOZ5x / GpyLu / vkwQo / Nz6iR / fg4kI
 * (all share the same trigger + popover shell pattern — 520px modal variant).
 * This component captures the open-inline-popover form of the same visual
 * language: trigger box with a search input, scrollable option list, empty /
 * loading rows. Domain pickers (`EmployeePicker`, etc.) wrap this.
 *
 * Tokens used: bg-surface, border-border, text-text / text-text-2 / text-text-3,
 *   bg-surface-2, bg-primary-soft, text-primary, shadow-overlay, rounded-{8,10}.
 * No raw hex. No new npm deps. Outside-click: document mousedown (ENGINEERING.md pattern).
 *
 * The popover is rendered in a portal (document.body) with fixed positioning derived
 * from the trigger rect, so an `overflow-hidden` ancestor (e.g. a form card) can't clip
 * it. `allowCreate` turns the box into a free-text combobox: when the typed query has no
 * exact match, a "use «query»" row commits the raw string as the value.
 */

import { ChevronDown, Loader2, Plus, Search, X } from 'lucide-react';
import * as React from 'react';
import { createPortal } from 'react-dom';
import { cn } from '../lib/cn.ts';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface ComboboxOption {
  value: string;
  label: string;
  sublabel?: string;
  meta?: string;
}

export interface ComboboxProps {
  value: string | null;
  onChange: (value: string | null) => void;
  options: ComboboxOption[];
  onSearch: (q: string) => void;
  isLoading?: boolean;
  placeholder?: string;
  disabled?: boolean;
  emptyText?: string;
  error?: boolean;
  /** aria-label for the clear (×) button shown when a value is selected. */
  clearLabel?: string;
  renderOption?: (option: ComboboxOption, selected: boolean) => React.ReactNode;
  /**
   * Free-text mode: when the typed query has no exact (case-insensitive) match among
   * `options`, a create row is shown that commits the raw trimmed query as the value.
   * Used by free-text fields like the position picker.
   */
  allowCreate?: boolean;
  /** Label for the create row (defaults to the raw query). Receives the trimmed query. */
  createLabel?: (query: string) => string;
}

// ---------------------------------------------------------------------------
// Combobox
// ---------------------------------------------------------------------------

export const Combobox = React.forwardRef<HTMLDivElement, ComboboxProps>(
  (
    {
      value,
      onChange,
      options,
      onSearch,
      isLoading = false,
      placeholder = '—',
      disabled = false,
      emptyText = 'Tidak ada pilihan',
      error = false,
      clearLabel = 'Hapus pilihan',
      renderOption,
      allowCreate = false,
      createLabel,
    },
    ref,
  ) => {
    const [open, setOpen] = React.useState(false);
    const [query, setQuery] = React.useState('');
    const containerRef = React.useRef<HTMLDivElement>(null);
    const popoverRef = React.useRef<HTMLDivElement>(null);
    const inputRef = React.useRef<HTMLInputElement>(null);
    const listRef = React.useRef<HTMLUListElement>(null);
    const [focusedIndex, setFocusedIndex] = React.useState(-1);
    // Trigger rect drives the portalled popover's fixed position so an overflow-hidden
    // ancestor can't clip it.
    const [rect, setRect] = React.useState<{ left: number; top: number; width: number } | null>(
      null,
    );

    const selectedOption = options.find((o) => o.value === value) ?? null;

    // A free-text create row appears when the trimmed query has no exact match.
    const trimmedQuery = query.trim();
    const showCreate =
      allowCreate &&
      trimmedQuery.length > 0 &&
      !options.some((o) => o.value.toLowerCase() === trimmedQuery.toLowerCase());

    // Merge forwarded ref
    React.useImperativeHandle(ref, () => containerRef.current as HTMLDivElement);

    // Recompute the popover position from the trigger rect (open, scroll, resize).
    const updateRect = React.useCallback(() => {
      const el = containerRef.current;
      if (!el) return;
      const r = el.getBoundingClientRect();
      setRect({ left: r.left, top: r.bottom, width: r.width });
    }, []);

    React.useLayoutEffect(() => {
      if (!open) return;
      updateRect();
      window.addEventListener('scroll', updateRect, true);
      window.addEventListener('resize', updateRect);
      return () => {
        window.removeEventListener('scroll', updateRect, true);
        window.removeEventListener('resize', updateRect);
      };
    }, [open, updateRect]);

    // Outside-click to close (ENGINEERING.md pattern — document mousedown). The popover is
    // portalled outside the container, so check both the trigger and the popover.
    React.useEffect(() => {
      if (!open) return;
      const handle = (e: MouseEvent) => {
        const target = e.target as Node;
        if (!containerRef.current?.contains(target) && !popoverRef.current?.contains(target)) {
          setOpen(false);
        }
      };
      document.addEventListener('mousedown', handle);
      return () => document.removeEventListener('mousedown', handle);
    }, [open]);

    // Focus search input when popover opens
    React.useEffect(() => {
      if (open) {
        setFocusedIndex(-1);
        setTimeout(() => inputRef.current?.focus(), 0);
      } else {
        setQuery('');
        onSearch('');
      }
    }, [open, onSearch]);

    // Keyboard navigation
    const handleKeyDown = (e: React.KeyboardEvent) => {
      if (!open) {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          setOpen(true);
        }
        return;
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        setOpen(false);
        return;
      }
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        setFocusedIndex((i) => Math.min(i + 1, options.length - 1));
        return;
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        setFocusedIndex((i) => Math.max(i - 1, 0));
        return;
      }
      if (e.key === 'Enter') {
        if (focusedIndex >= 0) {
          e.preventDefault();
          const opt = options[focusedIndex];
          if (opt) {
            onChange(opt.value === value ? null : opt.value);
            setOpen(false);
          }
          return;
        }
        // No option focused: commit the typed free-text value (create mode).
        if (showCreate) {
          e.preventDefault();
          commitCreate();
        }
      }
    };

    const commitCreate = () => {
      onChange(trimmedQuery);
      setOpen(false);
    };

    const handleQueryChange = (e: React.ChangeEvent<HTMLInputElement>) => {
      setQuery(e.target.value);
      onSearch(e.target.value);
      setFocusedIndex(-1);
    };

    const handleOptionClick = (opt: ComboboxOption) => {
      onChange(opt.value === value ? null : opt.value);
      setOpen(false);
    };

    const handleTriggerClick = () => {
      if (!disabled) setOpen((o) => !o);
    };

    return (
      <div ref={containerRef} className="relative w-full" onKeyDown={handleKeyDown}>
        {/* Trigger. The clear (×) and chevron are absolutely-positioned siblings
            (not nested in the button) — the chevron is pointer-events-none so
            clicks fall through to the trigger; the clear button captures its own. */}
        <button
          type="button"
          aria-haspopup="listbox"
          aria-expanded={open}
          aria-disabled={disabled}
          disabled={disabled}
          onClick={handleTriggerClick}
          className={cn(
            'flex w-full items-center rounded-lg border py-2.5 pl-3 text-sm',
            'bg-surface transition-colors',
            selectedOption && !disabled ? 'pr-14' : 'pr-9',
            error
              ? 'border-bad-tx focus-within:ring-bad-tx/30'
              : 'border-border focus-within:border-primary focus-within:ring-2 focus-within:ring-primary/20',
            disabled && 'cursor-not-allowed opacity-50',
          )}
        >
          <span
            className={cn(
              'min-w-0 flex-1 truncate text-left',
              selectedOption ? 'text-text' : 'text-text-3',
            )}
          >
            {selectedOption ? selectedOption.label : placeholder}
          </span>
        </button>
        {selectedOption && !disabled && (
          <button
            type="button"
            aria-label={clearLabel}
            onClick={(e) => {
              e.stopPropagation();
              onChange(null);
              onSearch('');
              setOpen(false);
            }}
            className="absolute right-8 top-1/2 flex size-5 -translate-y-1/2 items-center justify-center rounded text-text-3 hover:bg-surface-2 hover:text-text"
          >
            <X aria-hidden className="h-4 w-4" />
          </button>
        )}
        <ChevronDown
          aria-hidden
          className={cn(
            'pointer-events-none absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 text-text-3 transition-transform',
            open && 'rotate-180',
          )}
        />

        {/* Popover — portalled to body with fixed positioning so an overflow-hidden
            ancestor (e.g. the form card) can't clip it. */}
        {open &&
          rect &&
          createPortal(
            <div
              ref={popoverRef}
              onKeyDown={handleKeyDown}
              style={{
                position: 'fixed',
                left: rect.left,
                top: rect.top + 6,
                width: rect.width,
              }}
              className={cn(
                'z-50',
                'rounded-xl border border-border bg-surface shadow-overlay',
                'flex flex-col overflow-hidden',
              )}
            >
              {/* Search input inside popover */}
              <div className="border-b border-border-soft px-3 py-2">
                <div className="flex items-center gap-2">
                  <Search aria-hidden className="h-4 w-4 shrink-0 text-text-3" />
                  <input
                    ref={inputRef}
                    type="text"
                    value={query}
                    onChange={handleQueryChange}
                    placeholder={placeholder}
                    className="flex-1 bg-transparent text-sm text-text outline-none placeholder:text-text-3"
                  />
                </div>
              </div>

              {/* Option list. Each option is a real <button> (focusable + keyboard-activatable) and
                the search input above handles ArrowUp/Down/Enter. Formal listbox/option ARIA roles
                are intentionally omitted here — they map poorly to ul/li and the button semantics
                already convey interactivity. (Listbox-role semantics are a follow-up.) */}
              <ul ref={listRef} className="max-h-[280px] overflow-y-auto">
                {isLoading && (
                  <li className="flex items-center gap-2 px-4 py-3 text-sm text-text-3">
                    <Loader2 aria-hidden className="h-4 w-4 animate-spin" />
                    <span>Memuat…</span>
                  </li>
                )}

                {/* Free-text create row: commit the raw query as the value. */}
                {!isLoading && showCreate && (
                  <li>
                    <button
                      type="button"
                      onClick={commitCreate}
                      className={cn(
                        'flex w-full cursor-pointer items-center gap-2 px-4 py-3 text-left text-sm',
                        'border-b border-border-soft last:border-b-0',
                        'bg-surface transition-colors hover:bg-surface-2',
                      )}
                    >
                      <Plus aria-hidden className="h-4 w-4 shrink-0 text-primary" />
                      <span className="truncate text-text">
                        {createLabel ? createLabel(trimmedQuery) : `“${trimmedQuery}”`}
                      </span>
                    </button>
                  </li>
                )}

                {!isLoading && options.length === 0 && !showCreate && (
                  <li className="px-4 py-3 text-sm text-text-3">{emptyText}</li>
                )}

                {!isLoading &&
                  options.map((opt, idx) => {
                    const selected = opt.value === value;
                    const focused = idx === focusedIndex;
                    return (
                      <li key={opt.value}>
                        <button
                          type="button"
                          aria-pressed={selected}
                          onClick={() => handleOptionClick(opt)}
                          className={cn(
                            'flex w-full cursor-pointer items-center gap-3 px-4 py-3 text-left',
                            'border-b border-border-soft last:border-b-0',
                            'transition-colors',
                            selected
                              ? 'bg-primary-soft'
                              : focused
                                ? 'bg-surface-2'
                                : 'bg-surface hover:bg-surface-2',
                          )}
                        >
                          {renderOption ? (
                            renderOption(opt, selected)
                          ) : (
                            <DefaultOptionContent opt={opt} selected={selected} />
                          )}
                        </button>
                      </li>
                    );
                  })}
              </ul>
            </div>,
            document.body,
          )}
      </div>
    );
  },
);

Combobox.displayName = 'Combobox';

// ---------------------------------------------------------------------------
// Default option row — label + sublabel + meta chip
// ---------------------------------------------------------------------------

interface DefaultOptionContentProps {
  opt: ComboboxOption;
  selected: boolean;
}

function DefaultOptionContent({ opt, selected }: DefaultOptionContentProps) {
  return (
    <div className="flex flex-1 items-center gap-3 overflow-hidden">
      <div className="flex flex-1 flex-col gap-0.5 overflow-hidden">
        <span
          className={cn('truncate text-sm font-semibold', selected ? 'text-primary' : 'text-text')}
        >
          {opt.label}
        </span>
        {opt.sublabel && <span className="truncate text-xs text-text-3">{opt.sublabel}</span>}
      </div>
      {opt.meta && (
        <span className="shrink-0 rounded-full bg-surface-2 px-2 py-0.5 text-xs text-text-2">
          {opt.meta}
        </span>
      )}
      {selected && (
        <span aria-hidden className="ml-auto shrink-0 text-primary">
          ✓
        </span>
      )}
    </div>
  );
}
