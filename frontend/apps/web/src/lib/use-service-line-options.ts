/**
 * Shared service-line filter options — one source for every service-line dropdown
 * (billable report, …). Reuses the E2 `useListServiceLines` hook so a single fetch+cache backs
 * all service-line filters instead of each screen shipping an empty placeholder-only select
 * (the dead-filter bug class). Mirrors `use-company-options.ts`.
 */
import { useListServiceLines } from '@swp/api-client/e2';
import { useMemo } from 'react';

export interface ServiceLineOption {
  value: string;
  label: string;
}

interface UseServiceLineOptionsArgs {
  /** Skip the fetch when the control is not shown. Default true. */
  enabled?: boolean;
}

/** Active service lines as `{ value: id, label: name }` options for a filter select. */
export function useServiceLineOptions({ enabled = true }: UseServiceLineOptionsArgs = {}): {
  options: ServiceLineOption[];
  isLoading: boolean;
} {
  const query = useListServiceLines({ limit: 200 }, { query: { enabled, staleTime: 60_000 } });

  const options = useMemo<ServiceLineOption[]>(() => {
    if (!enabled) return [];
    const data =
      (query.data?.data as { data?: { id: string; name: string }[] } | undefined)?.data ?? [];
    return data.map((s) => ({ value: s.id, label: s.name }));
  }, [enabled, query.data]);

  return { options, isLoading: query.isLoading };
}
