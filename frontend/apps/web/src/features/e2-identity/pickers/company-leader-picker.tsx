/**
 * CompanyLeaderCandidatePicker — picks a shift-leader candidate scoped to ONE
 * client company.
 *
 * WHY company-scoped (not the global ShiftLeaderPicker): the server enforces
 * INV-4 — a shift leader MUST already have an ACTIVE placement at the company
 * (POST /shift-leader-assignments rejects anyone else with INV_4_VIOLATION,
 * suggested action `assign_after_placement`). The global picker
 * (role=shift_leader&assigned=false) offers people with no placement here, so
 * every pick dead-ends at a 409. This picker sources candidates from the
 * company roster instead, so only assignable employees appear.
 *
 * Candidate set: employees with a currently-working placement at the company
 * (lifecycle ACTIVE | EXTENDED | EXPIRING — PENDING_START does not satisfy
 * INV-4), deduped by employee, minus `excludeEmployeeId` (the current leader on
 * the replace flow). Search filters client-side over the roster page.
 *
 * Maps: employee_id → value, employee_name → label,
 *       position_name → sublabel.
 *
 * Wraps the generic `Combobox` from @swp/ui. i18n namespace: `pickers`.
 */

import { type CompanyRosterResponse, useGetCompanyRoster } from '@swp/api-client/e3';
import { Combobox } from '@swp/ui';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';

// Lifecycle states that satisfy INV-4 (a "currently working" placement).
const ELIGIBLE_LIFECYCLE = new Set(['ACTIVE', 'EXTENDED', 'EXPIRING']);

export interface CompanyLeaderCandidatePickerProps {
  /** Client company to scope candidates to. */
  companyId: string;
  value: string | null;
  onChange: (value: string | null) => void;
  /** Employee to omit from the list (e.g. the current leader on replace). */
  excludeEmployeeId?: string;
  disabled?: boolean;
  error?: boolean;
  placeholder?: string;
}

export function CompanyLeaderCandidatePicker({
  companyId,
  value,
  onChange,
  excludeEmployeeId,
  disabled,
  error,
  placeholder,
}: CompanyLeaderCandidatePickerProps) {
  const { t } = useTranslation('pickers');
  const [query, setQuery] = useState('');

  // Roster carries every placement at the company; one page is enough for a
  // single site's headcount (cursor pagination would need a "load more" UI the
  // picker doesn't have — 200 covers realistic on-site rosters).
  const result = useGetCompanyRoster(companyId, { limit: 200 }, { query: { staleTime: 30_000 } });
  const roster = result.data?.data as CompanyRosterResponse | undefined;

  const options = useMemo(() => {
    const placements = roster?.placements ?? [];
    const seen = new Set<string>();
    const q = query.trim().toLowerCase();
    const opts: { value: string; label: string; sublabel?: string }[] = [];
    for (const p of placements) {
      if (!ELIGIBLE_LIFECYCLE.has(p.lifecycle_status)) continue;
      if (p.employee_id === excludeEmployeeId) continue;
      if (seen.has(p.employee_id)) continue;
      const label = p.employee_name ?? p.employee_id;
      if (q && !label.toLowerCase().includes(q)) continue;
      seen.add(p.employee_id);
      opts.push({
        value: p.employee_id,
        label,
        sublabel: p.position_name || undefined,
      });
    }
    return opts;
  }, [roster, query, excludeEmployeeId]);

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
      emptyText={t('companyLeader.empty')}
    />
  );
}
