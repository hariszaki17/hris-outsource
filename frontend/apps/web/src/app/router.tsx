import { ForgotPasswordScreen } from '@/features/auth/forgot-password-screen.tsx';
import { LoginScreen } from '@/features/auth/login-screen.tsx';
import { ResetPasswordScreen } from '@/features/auth/reset-password-screen.tsx';
import { DashboardScreen } from '@/features/dashboard/dashboard-screen.tsx';
import { ComponentGallery } from '@/features/dev/component-gallery.tsx';
import { DataComponentsGallery } from '@/features/dev/data-components-gallery.tsx';
import { SettingsLayout } from '@/features/e1-foundations/settings-layout.tsx';
import { UsersScreen } from '@/features/e1-foundations/users-screen.tsx';
import { PlaceholderScreen } from '@/features/placeholder-screen.tsx';
import { auth } from '@/lib/auth.ts';
import { Role, UserStatus } from '@swp/api-client/e1';
import {
  Outlet,
  createRootRoute,
  createRoute,
  createRouter,
  redirect,
} from '@tanstack/react-router';
import { AppShell } from './shell.tsx';

const rootRoute = createRootRoute({ component: () => <Outlet /> });

interface LoginSearch {
  redirect?: string;
  error?: 'invalid' | 'locked' | 'disabled';
}

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  component: LoginScreen,
  // Optional search params: callers may `<Link to="/login">` without passing search.
  validateSearch: (search: Record<string, unknown>): LoginSearch => {
    const out: LoginSearch = {};
    if (typeof search.redirect === 'string') out.redirect = search.redirect;
    if (search.error === 'invalid' || search.error === 'locked' || search.error === 'disabled') {
      out.error = search.error;
    }
    return out;
  },
});

// Dev-only design-system gallery (public, no auth) — visual review surface for the
// Phase-0 component batch. Not a product screen.
const forgotPasswordRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/forgot-password',
  component: ForgotPasswordScreen,
});

const resetPasswordRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/reset-password',
  component: ResetPasswordScreen,
});

const devGalleryRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/dev/components',
  component: ComponentGallery,
});

const devDataGalleryRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/dev/components-data',
  component: DataComponentsGallery,
});

// Authenticated layout — guards every child. Client guard is convenience only; the API
// is the real gate (ENGINEERING.md C1).
const authedRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: 'authed',
  beforeLoad: ({ location }) => {
    if (!auth.isAuthenticated()) {
      throw redirect({ to: '/login', search: { redirect: location.href } });
    }
  },
  component: AppShell,
});

const indexRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/',
  component: DashboardScreen,
});

const placeholder = <P extends string>(path: P, title: string) =>
  createRoute({
    getParentRoute: () => authedRoute,
    path,
    component: () => <PlaceholderScreen title={title} />,
  });

// E1 Settings section: a sub-layout (left SettingsSubnav rail) over its sub-pages.
const settingsRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/settings',
  component: SettingsLayout,
});
const settingsIndexRoute = createRoute({
  getParentRoute: () => settingsRoute,
  path: '/',
  component: () => <PlaceholderScreen title="Ringkasan" />,
});
const usersRoute = createRoute({
  getParentRoute: () => settingsRoute,
  path: 'users',
  component: UsersScreen,
  // Typed filter/cursor search params (D1): shareable views + stable query cache key.
  validateSearch: (
    search: Record<string, unknown>,
  ): {
    q?: string;
    role?: Role;
    status?: UserStatus;
    cursor?: string;
  } => {
    const out: { q?: string; role?: Role; status?: UserStatus; cursor?: string } = {};
    if (typeof search.q === 'string' && search.q) out.q = search.q;
    if (
      typeof search.role === 'string' &&
      (Object.values(Role) as string[]).includes(search.role)
    ) {
      out.role = search.role as Role;
    }
    if (search.status === UserStatus.ACTIVE || search.status === UserStatus.DISABLED) {
      out.status = search.status;
    }
    if (typeof search.cursor === 'string' && search.cursor) out.cursor = search.cursor;
    return out;
  },
});
const settingsAuditRoute = createRoute({
  getParentRoute: () => settingsRoute,
  path: 'audit-log',
  component: () => <PlaceholderScreen title="Audit Log" />,
});
const settingsGeneralRoute = createRoute({
  getParentRoute: () => settingsRoute,
  path: 'general',
  component: () => <PlaceholderScreen title="Umum" />,
});

const routeTree = rootRoute.addChildren([
  loginRoute,
  forgotPasswordRoute,
  resetPasswordRoute,
  devGalleryRoute,
  devDataGalleryRoute,
  authedRoute.addChildren([
    indexRoute,
    placeholder('/employees', 'Karyawan'),
    placeholder('/placements', 'Penempatan'),
    placeholder('/schedule', 'Jadwal'),
    placeholder('/attendance', 'Kehadiran'),
    placeholder('/leave', 'Cuti'),
    placeholder('/overtime', 'Lembur'),
    placeholder('/reports', 'Laporan'),
    settingsRoute.addChildren([
      settingsIndexRoute,
      usersRoute,
      settingsAuditRoute,
      settingsGeneralRoute,
    ]),
  ]),
]);

export const router = createRouter({ routeTree, defaultPreload: 'intent' });

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
