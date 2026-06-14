// Shared test helpers for mobile component/integration tests.
//
// renderWithProviders wraps the tree in a fresh QueryClientProvider. The E11 screens call
// useQueryClient() (to invalidate after a mutation) even though the data hooks themselves are
// jest.mock'd per-test — so a real client must be in context or those calls throw.
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { type RenderOptions, render } from '@testing-library/react-native';
import type { ReactElement, ReactNode } from 'react';

export function makeQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
}

export function withProviders(client: QueryClient = makeQueryClient()) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  };
}

export function renderWithProviders(ui: ReactElement, options?: RenderOptions) {
  return render(ui, { wrapper: withProviders(), ...options });
}
