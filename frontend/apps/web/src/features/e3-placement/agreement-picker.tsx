/**
 * AgreementPicker — FK picker for selecting an active Agreement, scoped to an employee.
 *
 * Used on the Create Placement form (g3OzZz) to link a placement to the correct
 * employment agreement (BR-1b). Filters by `employee_id` when provided so only
 * that agent's ACTIVE/EXPIRING agreements appear.
 *
 * Maps: id → value, agreement_no ?? id → label, type + end_date → sublabel.
 * Disabled until an employee is chosen (the form enforces this via the `disabled` prop).
 *
 * i18n namespace: `pickers`.
 */

import { useListAgreements } from '@swp/api-client/e2';
import { Combobox } from '@swp/ui';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface AgreementPickerProps {
  value: string | null;
  onChange: (value: string | null) => void;
  employeeId: string | null;
  disabled?: boolean;
  error?: boolean;
  placeholder?: string;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function AgreementPicker({
  value,
  onChange,
  employeeId,
  disabled,
  error,
  placeholder,
}: AgreementPickerProps) {
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

  // Reset selection when the employee changes — but only on a *real* change, not on
  // first mount (and not on StrictMode's double-invoked effect, which a simple
  // first-run flag wouldn't survive). We track the previous employeeId and reset
  // only when it actually differs, so we never clobber the form's initial value or
  // trigger eager validation before the user interacts.
  const prevEmployeeId = useRef(employeeId);
  // biome-ignore lint/correctness/useExhaustiveDependencies: reset effect keyed on employeeId
  useEffect(() => {
    if (prevEmployeeId.current !== employeeId) {
      prevEmployeeId.current = employeeId;
      onChange(null);
    }
  }, [employeeId]);

  const result = useListAgreements(
    {
      employee_id: employeeId ?? undefined,
      status__in: 'ACTIVE,EXPIRING',
      limit: 50,
    },
    {
      query: {
        enabled: !!employeeId,
        staleTime: 30_000,
      },
    },
  );

  type AgreementItem = {
    id: string;
    agreement_no?: string | null;
    type?: string;
    end_date?: string | null;
  };

  const agreements = (result.data?.data as { data?: AgreementItem[] } | undefined)?.data ?? [];

  const filtered = debouncedQuery
    ? agreements.filter((ag) =>
        (ag.agreement_no ?? ag.id).toLowerCase().includes(debouncedQuery.toLowerCase()),
      )
    : agreements;

  const options = filtered.map((ag) => ({
    value: ag.id,
    label: ag.agreement_no ?? ag.id,
    sublabel:
      [ag.type, ag.end_date ? `s/d ${ag.end_date}` : null].filter(Boolean).join(' · ') || undefined,
  }));

  const isDisabled = disabled || !employeeId;

  return (
    <Combobox
      value={value}
      onChange={onChange}
      options={options}
      onSearch={setQuery}
      isLoading={result.isLoading}
      placeholder={
        placeholder ??
        (!employeeId ? t('agreement.disabledPlaceholder') : t('agreement.placeholder'))
      }
      disabled={isDisabled}
      error={error}
      emptyText={t('agreement.empty')}
    />
  );
}
