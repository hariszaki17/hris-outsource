import { MutationCache, QueryClient } from '@tanstack/react-query';
import { ApiError } from './errors.ts';

/**
 * Shared QueryClient factory. Defaults tuned for an internal ops tool: don't retry
 * auth/permission/validation failures (they won't fix themselves), retry transient ones.
 *
 * Automatic invalidation: a global MutationCache.onSuccess invalidates every query
 * after any successful create/update/delete, so the UI reflects writes without each
 * screen wiring its own invalidation (the TkDodo "automatic invalidation after
 * mutations" pattern). Active queries refetch immediately; inactive ones refetch on
 * next mount. Orval query keys are prefix-shaped (`['/employees', params]`), so a
 * blanket invalidate refreshes all paginated/filtered list variants + detail views.
 * Internal-tool tradeoff: this can over-refetch unrelated open lists, which is
 * acceptable here and far safer than per-call invalidation that silently drifts.
 */
export function createQueryClient(): QueryClient {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        retry: (failureCount, error) => {
          if (error instanceof ApiError) {
            // never retry client-side / auth / business-rule errors
            if (error.status >= 400 && error.status < 500) return false;
          }
          return failureCount < 2;
        },
        refetchOnWindowFocus: false,
      },
      mutations: {
        retry: false,
      },
    },
    mutationCache: new MutationCache({
      // Fire-and-forget: don't return the promise, so the mutation resolves (and the
      // success toast / modal-close fires) immediately while lists refetch in the
      // background. Only on success — a failed write changed nothing.
      onSuccess: () => {
        queryClient.invalidateQueries();
      },
    }),
  });
  return queryClient;
}
