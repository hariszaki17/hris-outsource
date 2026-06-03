/**
 * PositionPicker — FK picker for selecting a Position within a ServiceLine.
 *
 * Design source: .pen comp/PickerPosition `Nz6iR`
 * Lists positions via `useListPositionsInServiceLine(serviceLineId)`.
 * Disabled until `serviceLineId` is provided (design shows "Posisi tersedia di
 * [service line]" context pill — enforced by disabling the trigger).
 *
 * Maps: id → value, name → label, alias → sublabel (English label).
 *
 * Wraps the generic `Combobox` from @swp/ui.
 * i18n namespace: `pickers`.
 */

import { useListPositionsInServiceLine } from '@swp/api-client/e2';
import { Combobox } from '@swp/ui';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface PositionPickerProps {
  value: string | null;
  onChange: (value: string | null) => void;
  serviceLineId: string | null;
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
  serviceLineId,
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

  // Reset the selected value whenever the service line changes (intentionally keyed on
  // serviceLineId only — `onChange` is a stable callback we don't want to re-trigger on).
  // biome-ignore lint/correctness/useExhaustiveDependencies: reset effect keyed on serviceLineId
  useEffect(() => {
    onChange(null);
  }, [serviceLineId]);

  const result = useListPositionsInServiceLine(
    serviceLineId ?? '',
    { limit: 50 },
    {
      query: {
        enabled: !!serviceLineId,
        staleTime: 60_000,
      },
    },
  );

  // Response wrapped: result.data?.data → { data: Position[], next_cursor, has_more }
  const positions =
    (result.data?.data as { data?: { id: string; name: string; alias?: string }[] } | undefined)
      ?.data ?? [];

  const filtered = debouncedQuery
    ? positions.filter(
        (p) =>
          p.name.toLowerCase().includes(debouncedQuery.toLowerCase()) ||
          (p.alias?.toLowerCase().includes(debouncedQuery.toLowerCase()) ?? false),
      )
    : positions;

  const options = filtered.map((pos) => ({
    value: pos.id,
    label: pos.name,
    sublabel: pos.alias,
  }));

  const isDisabled = disabled || !serviceLineId;

  return (
    <Combobox
      value={value}
      onChange={onChange}
      options={options}
      onSearch={setQuery}
      isLoading={result.isLoading}
      placeholder={
        placeholder ??
        (!serviceLineId ? t('position.disabledPlaceholder') : t('position.placeholder'))
      }
      disabled={isDisabled}
      error={error}
      emptyText={t('position.empty')}
    />
  );
}
