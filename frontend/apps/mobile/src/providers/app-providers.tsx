// Single provider stack for the app: configures the api-client (side-effect import), then wraps
// children in Query, i18n, safe-area, and session providers.
import './../lib/api'; // side-effect: configureApiClient
import { createQueryClient } from '@swp/api-client';
import { QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { I18nextProvider } from 'react-i18next';
import { SafeAreaProvider } from 'react-native-safe-area-context';
import i18n from '../lib/i18n';
import { SessionProvider } from './session';

const queryClient = createQueryClient();

export function AppProviders({ children }: { children: ReactNode }) {
  return (
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>
        <SafeAreaProvider>
          <SessionProvider>{children}</SessionProvider>
        </SafeAreaProvider>
      </I18nextProvider>
    </QueryClientProvider>
  );
}
