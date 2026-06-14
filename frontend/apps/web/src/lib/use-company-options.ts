/**
 * Shared client-company filter options — one source for every HR-facing company dropdown
 * (attendance, overtime, leave, billable report, …). Reuses the E2 `useListClientCompanies`
 * hook so a single fetch+cache backs all company filters instead of each screen re-deriving it
 * (or shipping an empty placeholder-only select — the dead-filter bug class).
 *
 * SCOPE note (NAVIGATION-AND-RBAC §4.2): a shift_leader is pinned to ONE company server-side, so
 * the caller passes `enabled: false` for SL and renders a locked control instead — this hook is
 * for the global (HR/super_admin) free-pick case.
 */
import { useListClientCompanies } from '@swp/api-client/e2';
import { useMemo } from 'react';

export interface CompanyOption {
  value: string;
  label: string;
}

interface UseCompanyOptionsArgs {
  /** Skip the fetch (e.g. shift_leader, whose company is locked). Default true. */
  enabled?: boolean;
}

/** Active client companies as `{ value: id, label: name }` options for a filter select. */
export function useCompanyOptions({ enabled = true }: UseCompanyOptionsArgs = {}): {
  options: CompanyOption[];
  isLoading: boolean;
} {
  const query = useListClientCompanies({ limit: 200 }, { query: { enabled, staleTime: 60_000 } });

  const options = useMemo<CompanyOption[]>(() => {
    if (!enabled) return [];
    const data =
      (query.data?.data as { data?: { id: string; name: string }[] } | undefined)?.data ?? [];
    return data.map((c) => ({ value: c.id, label: c.name }));
  }, [enabled, query.data]);

  return { options, isLoading: query.isLoading };
}
