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
 */

import { ChevronDown, Loader2, Search } from 'lucide-react';
import * as React from 'react';
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
  renderOption?: (option: ComboboxOption, selected: boolean) => React.ReactNode;
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
      renderOption,
    },
    ref,
  ) => {
    const [open, setOpen] = React.useState(false);
    const [query, setQuery] = React.useState('');
    const containerRef = React.useRef<HTMLDivElement>(null);
    const inputRef = React.useRef<HTMLInputElement>(null);
    const listRef = React.useRef<HTMLUListElement>(null);
    const [focusedIndex, setFocusedIndex] = React.useState(-1);

    const selectedOption = options.find((o) => o.value === value) ?? null;

    // Merge forwarded ref
    React.useImperativeHandle(ref, () => containerRef.current as HTMLDivElement);

    // Outside-click to close (ENGINEERING.md pattern — document mousedown)
    React.useEffect(() => {
      if (!open) return;
      const handle = (e: MouseEvent) => {
        if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
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
      if (e.key === 'Enter' && focusedIndex >= 0) {
        e.preventDefault();
        const opt = options[focusedIndex];
        if (opt) {
          onChange(opt.value === value ? null : opt.value);
          setOpen(false);
        }
      }
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
        {/* Trigger */}
        <button
          type="button"
          aria-haspopup="listbox"
          aria-expanded={open}
          aria-disabled={disabled}
          disabled={disabled}
          onClick={handleTriggerClick}
          className={cn(
            'flex w-full items-center gap-2 rounded-lg border px-3 py-2.5 text-sm',
            'bg-surface transition-colors',
            error
              ? 'border-bad-tx focus-within:ring-bad-tx/30'
              : 'border-border focus-within:border-primary focus-within:ring-2 focus-within:ring-primary/20',
            disabled && 'cursor-not-allowed opacity-50',
          )}
        >
          <Search aria-hidden className="h-4 w-4 shrink-0 text-text-3" />
          <span
            className={cn(
              'flex-1 truncate text-left',
              selectedOption ? 'text-text' : 'text-text-3',
            )}
          >
            {selectedOption ? selectedOption.label : placeholder}
          </span>
          <ChevronDown
            aria-hidden
            className={cn(
              'h-4 w-4 shrink-0 text-text-3 transition-transform',
              open && 'rotate-180',
            )}
          />
        </button>

        {/* Popover */}
        {open && (
          <div
            className={cn(
              'absolute left-0 right-0 z-50 mt-1.5',
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

              {!isLoading && options.length === 0 && (
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
          </div>
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
