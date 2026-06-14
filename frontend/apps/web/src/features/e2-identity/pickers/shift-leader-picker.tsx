/**
 * ShiftLeaderPicker — FK picker for selecting a Shift Leader candidate.
 *
 * Design source: .pen comp/PickerShiftLeader `fg4kI`
 * Deviation: there is no dedicated /shift-leaders endpoint. This picker uses
 * `useListEmployees` with `role=shift_leader` + `assigned=false` (default —
 * shows unassigned leaders first, matching the "Belum ditugaskan" chip in the
 * design). The real shift-leader-eligible filter (`INV-3/4`: agent must have
 * role=shift_leader in E1) is enforced server-side by the E3 placement endpoint.
 *
 * Maps: id → value, full_name → label, nip ?? nik → sublabel,
 *       current_client_company?.name → meta (if already assigned, shown as context).
 *
 * Wraps the generic `Combobox` from @swp/ui.
 * i18n namespace: `pickers`.
 */

import { ListEmployeesRole, useListEmployees } from '@swp/api-client/e2';
import { Combobox } from '@swp/ui';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface ShiftLeaderPickerProps {
  value: string | null;
  onChange: (value: string | null) => void;
  disabled?: boolean;
  error?: boolean;
  placeholder?: string;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function ShiftLeaderPicker({
  value,
  onChange,
  disabled,
  error,
  placeholder,
}: ShiftLeaderPickerProps) {
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

  // role=shift_leader shows only employees with the Shift Leader E1 role.
  // assigned=false filters to leaders not yet assigned to a client company —
  // matches the default "Belum ditugaskan" chip in comp/PickerShiftLeader.
  // The real eligibility guard (INV-3/4) is server-side in E3.
  const result = useListEmployees(
    {
      q: debouncedQuery || undefined,
      role: ListEmployeesRole.shift_leader,
      assigned: false,
      limit: 50,
    },
    { query: { staleTime: 30_000 } },
  );

  // Response wrapped: result.data?.data → { data: Employee[], ... }
  const employees =
    (
      result.data?.data as
        | {
            data?: {
              id: string;
              full_name: string;
              nip?: string;
              nik: string;
              current_client_company?: { id: string; name: string } | null;
            }[];
          }
        | undefined
    )?.data ?? [];

  const options = employees.map((emp) => ({
    value: emp.id,
    label: emp.full_name,
    sublabel: emp.nip ?? emp.nik,
    meta: emp.current_client_company?.name,
  }));

  return (
    <Combobox
      value={value}
      onChange={onChange}
      options={options}
      onSearch={setQuery}
      isLoading={result.isLoading}
      placeholder={placeholder ?? t('shiftLeader.placeholder')}
      disabled={disabled}
      error={error}
      emptyText={t('shiftLeader.empty')}
    />
  );
}
