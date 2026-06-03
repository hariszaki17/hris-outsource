import { QueryClient } from '@tanstack/react-query';
import { ApiError } from './errors.ts';

/**
 * Shared QueryClient factory. Defaults tuned for an internal ops tool: don't retry
 * auth/permission/validation failures (they won't fix themselves), retry transient ones.
 */
export function createQueryClient(): QueryClient {
  return new QueryClient({
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
  });
}
