import { i18n } from '@/lib/i18n.ts';
import { createQueryClient } from '@swp/api-client';
import { ToastProvider, Toaster } from '@swp/ui';
import { QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { I18nextProvider } from 'react-i18next';

const queryClient = createQueryClient();

export function Providers({ children }: { children: ReactNode }) {
  return (
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={queryClient}>
        <ToastProvider>
          {children}
          <Toaster />
        </ToastProvider>
      </QueryClientProvider>
    </I18nextProvider>
  );
}
