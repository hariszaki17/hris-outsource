/**
 * PositionPicker — free-text typeahead for the placement "position" field.
 *
 * Position is FREE-TEXT (no master, no FK, no SWP-POS id) per the locked
 * decision (2026-06-12): the value is the position *string* itself. The picker
 * is a typeahead Combobox over DISTINCT existing values, backed by
 * `useSearchPositions` (GET /positions:search). The user may pick an existing
 * value or type a new one — whatever string they commit becomes the value.
 *
 * No service-line gating, no reset-on-service-line: the field stands alone.
 *
 * The debounced query (`q`) drives the search hook; the Combobox's `onSearch`
 * plumbing supplies the raw query. The current value is always merged into the
 * options so the Combobox trigger can render it even when it isn't in the
 * latest search page.
 *
 * Wraps the generic `Combobox` from @swp/ui. i18n namespace: `pickers`.
 */

import { useSearchPositions } from '@swp/api-client/e2';
import { Combobox } from '@swp/ui';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface PositionPickerProps {
  /** Free-text position string (not an id). */
  value: string | null;
  onChange: (value: string | null) => void;
  disabled?: boolean;
  error?: boolean;
  placeholder?: string;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function PositionPicker({
  value,
  onChange,
  disabled,
  error,
  placeholder,
}: PositionPickerProps) {
  const { t } = useTranslation('pickers');
  const [query, setQuery] = useState('');
  const [debouncedQuery, setDebouncedQuery] = useState('');
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      setDebouncedQuery(query);
    }, 300);
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [query]);

  const result = useSearchPositions(
    { q: debouncedQuery || undefined, limit: 50 },
    {
      query: {
        staleTime: 60_000,
      },
    },
  );

  // Response: result.data?.data → { positions: string[] }
  const positions = (result.data?.data as { positions?: string[] } | undefined)?.positions ?? [];

  // The value is a free-text string. Merge it into the options so the Combobox
  // trigger can render the current selection even when it isn't in the latest
  // search page (and so the user's typed-but-not-listed value stays visible).
  const options = useMemo(() => {
    const opts = positions.map((p) => ({ value: p, label: p }));
    if (value && !opts.some((o) => o.value === value)) {
      return [{ value, label: value }, ...opts];
    }
    return opts;
  }, [positions, value]);

  return (
    <Combobox
      value={value}
      onChange={onChange}
      options={options}
      onSearch={setQuery}
      isLoading={result.isLoading}
      placeholder={placeholder ?? t('position.searchPlaceholder')}
      disabled={disabled}
      error={error}
      emptyText={t('position.empty')}
    />
  );
}
