import { Providers } from '@/app/providers.tsx';
import { router } from '@/app/router.tsx';
import { auth, installAuth, tryRestoreSession } from '@/lib/auth.ts';
import { RouterProvider } from '@tanstack/react-router';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import '@/lib/i18n.ts';
import './index.css';

installAuth();
// Re-run route guards (e.g. redirect to /login) whenever auth state changes.
auth.subscribe(() => void router.invalidate());

async function enableMocking() {
  if (import.meta.env.VITE_ENABLE_MSW !== 'true') return;
  const { worker } = await import('@/mocks/browser.ts');
  await worker.start({ onUnhandledRequest: 'bypass' });
}

void enableMocking()
  .then(() => tryRestoreSession())
  .then(() => {
    const root = document.getElementById('root');
    if (!root) throw new Error('#root not found');
    createRoot(root).render(
      <StrictMode>
        <Providers>
          <RouterProvider router={router} />
        </Providers>
      </StrictMode>,
    );
  });
