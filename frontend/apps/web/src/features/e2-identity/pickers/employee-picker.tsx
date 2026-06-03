/**
 * EmployeePicker — FK picker for selecting an Employee.
 *
 * Design source: .pen comp/PickerEmployee `ZOZ5x`
 * Lists employees via `useListEmployees` (q-search, status=ACTIVE).
 * Maps: id → value, full_name → label, nip ?? nik → sublabel,
 *       current_position.name → meta.
 *
 * Wraps the generic `Combobox` from @swp/ui.
 * i18n namespace: `pickers`.
 */

import { useListEmployees } from '@swp/api-client/e2';
import { Combobox } from '@swp/ui';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface EmployeePickerProps {
  value: string | null;
  onChange: (value: string | null) => void;
  disabled?: boolean;
  error?: boolean;
  placeholder?: string;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function EmployeePicker({
  value,
  onChange,
  disabled,
  error,
  placeholder,
}: EmployeePickerProps) {
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

  const result = useListEmployees(
    { q: debouncedQuery || undefined, limit: 50 },
    { query: { staleTime: 30_000 } },
  );

  // The response is wrapped: result.data?.data → { data: Employee[], next_cursor, has_more }
  const employees =
    (
      result.data?.data as
        | {
            data?: {
              id: string;
              full_name: string;
              nip?: string;
              nik: string;
              current_position?: { id: string; name: string } | null;
            }[];
          }
        | undefined
    )?.data ?? [];

  const options = employees.map((emp) => ({
    value: emp.id,
    label: emp.full_name,
    sublabel: emp.nip ?? emp.nik,
    meta: emp.current_position?.name,
  }));

  return (
    <Combobox
      value={value}
      onChange={onChange}
      options={options}
      onSearch={setQuery}
      isLoading={result.isLoading}
      placeholder={placeholder ?? t('employee.placeholder')}
      disabled={disabled}
      error={error}
      emptyText={t('employee.empty')}
    />
  );
}
