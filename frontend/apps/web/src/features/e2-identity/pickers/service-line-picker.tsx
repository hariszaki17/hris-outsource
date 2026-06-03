/**
 * ServiceLinePicker — FK picker for selecting a ServiceLine.
 *
 * Design source: .pen comp/PickerServiceLine `vkwQo`
 * Lists service lines via `useListServiceLines` (no search param — small static list).
 * Maps: id → value, name → label, position_count as meta (e.g. "12 posisi").
 *
 * Wraps the generic `Combobox` from @swp/ui.
 * i18n namespace: `pickers`.
 */

import { useListServiceLines } from '@swp/api-client/e2';
import { Combobox } from '@swp/ui';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface ServiceLinePickerProps {
  value: string | null;
  onChange: (value: string | null) => void;
  disabled?: boolean;
  error?: boolean;
  placeholder?: string;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function ServiceLinePicker({
  value,
  onChange,
  disabled,
  error,
  placeholder,
}: ServiceLinePickerProps) {
  const { t } = useTranslation('pickers');
  // Service lines are a small static master list — no server-side search needed.
  // We keep a local query for client-side label filtering if the user types.
  const [query, setQuery] = useState('');

  const result = useListServiceLines({ limit: 50 }, { query: { staleTime: 5 * 60_000 } });

  // Response wrapped: result.data?.data → { data: ServiceLine[], next_cursor, has_more }
  const serviceLines =
    (
      result.data?.data as
        | { data?: { id: string; name: string; position_count?: number }[] }
        | undefined
    )?.data ?? [];

  const filtered = query
    ? serviceLines.filter((sl) => sl.name.toLowerCase().includes(query.toLowerCase()))
    : serviceLines;

  const options = filtered.map((sl) => ({
    value: sl.id,
    label: sl.name,
    meta:
      sl.position_count != null
        ? `${sl.position_count} ${t('serviceLine.positionCountSuffix')}`
        : undefined,
  }));

  return (
    <Combobox
      value={value}
      onChange={onChange}
      options={options}
      onSearch={setQuery}
      isLoading={result.isLoading}
      placeholder={placeholder ?? t('serviceLine.placeholder')}
      disabled={disabled}
      error={error}
      emptyText={t('serviceLine.empty')}
    />
  );
}
