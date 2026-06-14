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
  /**
   * Fires alongside onChange with the selected option's display label (the
   * employee's full_name). Lets callers that render their own chips show the name
   * for a fresh pick without a round-trip. `label` is undefined when cleared.
   */
  onPick?: (value: string | null, label?: string) => void;
  disabled?: boolean;
  error?: boolean;
  placeholder?: string;
  /**
   * Which id the picker emits/binds. Default `employee_id` (the Employee FK — most callers).
   * Approval-line membership stores `SWP-USR-…` ids, so the E11 line card passes `user_id`.
   */
  valueField?: 'employee_id' | 'user_id';
}

// ---------------------------------------------------------------------------
// Display-name cache — every option the picker ever loads records its id → label
// so callers that render their own chips (e.g. the E11 template line card) can
// resolve a freshly-picked id to a name without a round-trip, independent of the
// onPick callback. Keyed by BOTH user_id and employee_id (whichever was bound).
// ---------------------------------------------------------------------------
const employeeNameCache = new Map<string, string>();

/** Resolve a previously-loaded employee/user id to its display name, if known. */
export function resolveEmployeeName(id: string | null | undefined): string | undefined {
  return id ? employeeNameCache.get(id) : undefined;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function EmployeePicker({
  value,
  onChange,
  onPick,
  disabled,
  error,
  placeholder,
  valueField = 'employee_id',
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
              user_id?: string;
              full_name: string;
              nip?: string;
              nik: string;
              current_position?: { id: string; name: string } | null;
            }[];
          }
        | undefined
    )?.data ?? [];

  const options = employees.map((emp) => ({
    // Approval-line callers bind `user_id`; default callers bind the Employee FK (`id`).
    value: valueField === 'user_id' ? (emp.user_id ?? emp.id) : emp.id,
    label: emp.full_name,
    sublabel: emp.nip ?? emp.nik,
    meta: emp.current_position?.name,
  }));

  // Record id → name for any caller that resolves a picked id to a name later.
  for (const emp of employees) {
    if (emp.full_name) {
      employeeNameCache.set(emp.id, emp.full_name);
      if (emp.user_id) employeeNameCache.set(emp.user_id, emp.full_name);
    }
  }

  const handleChange = (v: string | null) => {
    onChange(v);
    onPick?.(v, v == null ? undefined : options.find((o) => o.value === v)?.label);
  };

  return (
    <Combobox
      value={value}
      onChange={handleChange}
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
