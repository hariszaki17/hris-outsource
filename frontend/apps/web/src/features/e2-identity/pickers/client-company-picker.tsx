/**
 * ClientCompanyPicker — FK picker for selecting a ClientCompany.
 *
 * Design source: .pen comp/PickerClientCompany `GpyLu`
 * Lists client companies via `useListClientCompanies` (q-search).
 * Maps: id → value, name → label, address → sublabel.
 *
 * Wraps the generic `Combobox` from @swp/ui.
 * i18n namespace: `pickers`.
 */

import { useListClientCompanies } from '@swp/api-client/e2';
import { Combobox } from '@swp/ui';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface ClientCompanyPickerProps {
  value: string | null;
  onChange: (value: string | null) => void;
  disabled?: boolean;
  error?: boolean;
  placeholder?: string;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function ClientCompanyPicker({
  value,
  onChange,
  disabled,
  error,
  placeholder,
}: ClientCompanyPickerProps) {
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

  const result = useListClientCompanies(
    { q: debouncedQuery || undefined, limit: 50 },
    { query: { staleTime: 30_000 } },
  );

  // Response wrapped: result.data?.data → { data: ClientCompany[], next_cursor, has_more }
  const companies =
    (result.data?.data as { data?: { id: string; name: string; address: string }[] } | undefined)
      ?.data ?? [];

  const options = companies.map((cc) => ({
    value: cc.id,
    label: cc.name,
    sublabel: cc.address,
  }));

  return (
    <Combobox
      value={value}
      onChange={onChange}
      options={options}
      onSearch={setQuery}
      isLoading={result.isLoading}
      placeholder={placeholder ?? t('clientCompany.placeholder')}
      disabled={disabled}
      error={error}
      emptyText={t('clientCompany.empty')}
    />
  );
}
