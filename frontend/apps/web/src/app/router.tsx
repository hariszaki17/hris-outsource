import { ForgotPasswordScreen } from '@/features/auth/forgot-password-screen.tsx';
import { LoginScreen } from '@/features/auth/login-screen.tsx';
import { ResetPasswordScreen } from '@/features/auth/reset-password-screen.tsx';
import { DashboardScreen } from '@/features/dashboard/dashboard-screen.tsx';
import { ComponentGallery } from '@/features/dev/component-gallery.tsx';
import { DataComponentsGallery } from '@/features/dev/data-components-gallery.tsx';
import { AuditLogScreen } from '@/features/e1-foundations/audit-log-screen.tsx';
import {
  NoPermissionScreen,
  SessionExpiredScreen,
} from '@/features/e1-foundations/global-states.tsx';
import { SettingsGeneralScreen } from '@/features/e1-foundations/settings-general-screen.tsx';
import { SettingsLayout } from '@/features/e1-foundations/settings-layout.tsx';
import { SettingsOverviewScreen } from '@/features/e1-foundations/settings-overview-screen.tsx';
import { UsersScreen } from '@/features/e1-foundations/users-screen.tsx';
import { AgreementDetailScreen } from '@/features/e2-identity/agreement-detail-screen.tsx';
import { CreateAgreementScreen } from '@/features/e2-identity/agreement-form.tsx';
import {
  AgreementsScreen,
  type AgreementsSearch,
} from '@/features/e2-identity/agreements-screen.tsx';
import { AttendanceCodesScreen } from '@/features/e2-identity/attendance-codes-screen.tsx';
import {
  ChangeRequestsScreen,
  type ChangeRequestsSearch,
} from '@/features/e2-identity/change-requests-screen.tsx';
import {
  ClientCompaniesScreen,
  type ClientCompaniesSearch,
} from '@/features/e2-identity/client-companies-screen.tsx';
import { ClientCompanyDetailScreen } from '@/features/e2-identity/client-company-detail-screen.tsx';
import { CreateClientCompanyScreen } from '@/features/e2-identity/client-company-form.tsx';
import { EmployeeDetailScreen } from '@/features/e2-identity/employee-detail-screen.tsx';
import { CreateEmployeeScreen } from '@/features/e2-identity/employee-form.tsx';
import { EmployeesScreen, type EmployeesSearch } from '@/features/e2-identity/employees-screen.tsx';
import { LeaveTypesScreen } from '@/features/e2-identity/leave-types-screen.tsx';
import { MasterDataHubScreen } from '@/features/e2-identity/master-data-hub-screen.tsx';
import { OvertimeRulesScreen } from '@/features/e2-identity/overtime-rules-screen.tsx';
import { ServiceLineDetailScreen } from '@/features/e2-identity/service-line-detail-screen.tsx';
import { ServiceLinesScreen } from '@/features/e2-identity/service-lines-screen.tsx';
import { CompanyRosterScreen } from '@/features/e3-placement/company-roster-screen.tsx';
import { PlacementDetailScreen } from '@/features/e3-placement/placement-detail-screen.tsx';
import { CreatePlacementScreen } from '@/features/e3-placement/placement-form.tsx';
import {
  PlacementsScreen,
  type PlacementsSearch,
} from '@/features/e3-placement/placements-screen.tsx';
import {
  ScheduleGridScreen,
  type ScheduleGridSearch,
} from '@/features/e4-scheduling/schedule-grid-screen.tsx';
import { ShiftMastersScreen } from '@/features/e4-scheduling/shift-masters-screen.tsx';
import { AttendanceDashboardScreen } from '@/features/e5-attendance/attendance-dashboard-screen.tsx';
import { AttendanceDetailScreen } from '@/features/e5-attendance/attendance-detail-screen.tsx';
import { AttendanceVerificationScreen } from '@/features/e5-attendance/attendance-verification-screen.tsx';
import {
  CorrectionsScreen,
  type CorrectionsSearch,
} from '@/features/e5-attendance/corrections-screen.tsx';
import { PlaceholderScreen } from '@/features/placeholder-screen.tsx';
import { auth } from '@/lib/auth.ts';
import { Role, UserStatus } from '@swp/api-client/e1';
import {
  AgreementStatus,
  AgreementType,
  ChangeRequestRequestType,
  ClientCompanyStatus,
  EmployeeStatus,
} from '@swp/api-client/e2';
import { PlacementLifecycleStatus } from '@swp/api-client/e3';
import { CorrectionStatus, CorrectionType } from '@swp/api-client/e5';
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

// Global auth states (public — reachable without the authed shell, e.g. after a 401).
const sessionExpiredRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/session-expired',
  component: SessionExpiredScreen,
});

const forbiddenRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/forbidden',
  component: NoPermissionScreen,
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
// E2 — Karyawan (employee list, detail, create)
const employeesRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/employees',
  component: EmployeesScreen,
  validateSearch: (search: Record<string, unknown>): EmployeesSearch => {
    const out: EmployeesSearch = {};
    if (typeof search.q === 'string' && search.q) out.q = search.q;
    if (search.status === EmployeeStatus.ACTIVE || search.status === EmployeeStatus.INACTIVE) {
      out.status = search.status;
    }
    if (typeof search.service_line === 'string' && search.service_line)
      out.service_line = search.service_line;
    if (typeof search.client_company === 'string' && search.client_company)
      out.client_company = search.client_company;
    if (typeof search.has_login === 'boolean') out.has_login = search.has_login;
    if (
      search.tab === 'all' ||
      search.tab === 'active' ||
      search.tab === 'inactive' ||
      search.tab === 'no-login'
    ) {
      out.tab = search.tab;
    }
    if (typeof search.cursor === 'string' && search.cursor) out.cursor = search.cursor;
    return out;
  },
});

const employeeNewRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/employees/new',
  component: CreateEmployeeScreen,
});

const employeeDetailRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/employees/$employeeId',
  component: EmployeeDetailScreen,
});

// E2 — Lini Layanan (service lines list + detail)
const serviceLinesRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/service-lines',
  component: ServiceLinesScreen,
  validateSearch: (
    search: Record<string, unknown>,
  ): { status?: 'ACTIVE' | 'INACTIVE'; cursor?: string } => {
    const out: { status?: 'ACTIVE' | 'INACTIVE'; cursor?: string } = {};
    if (search.status === 'ACTIVE' || search.status === 'INACTIVE') out.status = search.status;
    if (typeof search.cursor === 'string' && search.cursor) out.cursor = search.cursor;
    return out;
  },
});

const serviceLineDetailRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/service-lines/$serviceLineId',
  component: function ServiceLineDetailRoute() {
    const { serviceLineId } = serviceLineDetailRoute.useParams();
    return <ServiceLineDetailScreen serviceLineId={serviceLineId} />;
  },
});

// E2 — Antrian Persetujuan Perubahan Data (HR change-request approval queue)
const changeRequestsRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/change-requests',
  component: ChangeRequestsScreen,
  validateSearch: (search: Record<string, unknown>): ChangeRequestsSearch => {
    const out: ChangeRequestsSearch = {};
    if (typeof search.q === 'string' && search.q) out.q = search.q;
    if (
      typeof search.request_type === 'string' &&
      (Object.values(ChangeRequestRequestType) as string[]).includes(search.request_type)
    ) {
      out.request_type = search.request_type as ChangeRequestRequestType;
    }
    if (search.tab === 'all' || search.tab === 'profile' || search.tab === 'bank') {
      out.tab = search.tab;
    }
    if (typeof search.cursor === 'string' && search.cursor) out.cursor = search.cursor;
    return out;
  },
});

// E2 — Perjanjian Kerja (employment agreements list, detail, create)
const agreementsRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/agreements',
  component: AgreementsScreen,
  validateSearch: (search: Record<string, unknown>): AgreementsSearch => {
    const out: AgreementsSearch = {};
    if (search.type === AgreementType.PKWT || search.type === AgreementType.PKWTT) {
      out.type = search.type;
    }
    if (
      search.status === AgreementStatus.ACTIVE ||
      search.status === AgreementStatus.EXPIRING ||
      search.status === AgreementStatus.SUPERSEDED ||
      search.status === AgreementStatus.CLOSED
    ) {
      out.status = search.status;
    }
    if (
      search.tab === 'all' ||
      search.tab === 'active' ||
      search.tab === 'expiring' ||
      search.tab === 'superseded' ||
      search.tab === 'closed'
    ) {
      out.tab = search.tab;
    }
    if (typeof search.cursor === 'string' && search.cursor) out.cursor = search.cursor;
    return out;
  },
});

const agreementNewRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/agreements/new',
  component: CreateAgreementScreen,
});

const agreementDetailRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/agreements/$agreementId',
  component: function AgreementDetailRoute() {
    const { agreementId } = agreementDetailRoute.useParams();
    return <AgreementDetailScreen agreementId={agreementId} />;
  },
});

// E2 — Data Master (operational master data: leave types, attendance codes, overtime rules)
const masterDataRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/master-data',
  component: MasterDataHubScreen,
});
const leaveTypesRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/master-data/leave-types',
  component: LeaveTypesScreen,
});
const attendanceCodesRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/master-data/attendance-codes',
  component: AttendanceCodesScreen,
});
const overtimeRulesRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/master-data/overtime-rules',
  component: OvertimeRulesScreen,
});

// E2 — Perusahaan Klien (client companies list, detail, create)
const clientCompaniesRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/client-companies',
  component: ClientCompaniesScreen,
  validateSearch: (search: Record<string, unknown>): ClientCompaniesSearch => {
    const out: ClientCompaniesSearch = {};
    if (typeof search.q === 'string' && search.q) out.q = search.q;
    if (
      search.status === ClientCompanyStatus.ACTIVE ||
      search.status === ClientCompanyStatus.INACTIVE
    ) {
      out.status = search.status;
    }
    if (typeof search.service_line === 'string' && search.service_line)
      out.service_line = search.service_line;
    if (typeof search.cursor === 'string' && search.cursor) out.cursor = search.cursor;
    return out;
  },
});
const clientCompanyNewRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/client-companies/new',
  component: CreateClientCompanyScreen,
});
const clientCompanyDetailRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/client-companies/$clientCompanyId',
  component: function ClientCompanyDetailRoute() {
    const { clientCompanyId } = clientCompanyDetailRoute.useParams();
    return <ClientCompanyDetailScreen clientCompanyId={clientCompanyId} />;
  },
});

// E3 — Penempatan (placements list, create)
const placementsRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/placements',
  component: PlacementsScreen,
  validateSearch: (search: Record<string, unknown>): PlacementsSearch => {
    const out: PlacementsSearch = {};
    if (typeof search.q === 'string' && search.q) out.q = search.q;
    if (typeof search.company_id === 'string' && search.company_id)
      out.company_id = search.company_id;
    if (typeof search.service_line_id === 'string' && search.service_line_id)
      out.service_line_id = search.service_line_id;
    if (
      search.status === PlacementLifecycleStatus.PENDING_START ||
      search.status === PlacementLifecycleStatus.ACTIVE ||
      search.status === PlacementLifecycleStatus.EXTENDED ||
      search.status === PlacementLifecycleStatus.EXPIRING ||
      search.status === PlacementLifecycleStatus.ENDED ||
      search.status === PlacementLifecycleStatus.TRANSFERRED ||
      search.status === PlacementLifecycleStatus.TERMINATED ||
      search.status === PlacementLifecycleStatus.RESIGNED ||
      search.status === PlacementLifecycleStatus.SUPERSEDED
    ) {
      out.status = search.status;
    }
    if (typeof search.expiring_soon === 'boolean') out.expiring_soon = search.expiring_soon;
    if (typeof search.cursor === 'string' && search.cursor) out.cursor = search.cursor;
    return out;
  },
});

const placementNewRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/placements/new',
  component: CreatePlacementScreen,
});

const companyRosterRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/client-companies/$clientCompanyId/roster',
  component: function CompanyRosterRoute() {
    const { clientCompanyId } = companyRosterRoute.useParams();
    return <CompanyRosterScreen clientCompanyId={clientCompanyId} />;
  },
});

const placementDetailRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/placements/$placementId',
  component: function PlacementDetailRoute() {
    const { placementId } = placementDetailRoute.useParams();
    return <PlacementDetailScreen placementId={placementId} />;
  },
});

// E4 — Jadwal Shift (weekly schedule grid — Shift Leader & HR/Admin)
const scheduleRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/schedule',
  component: ScheduleGridScreen,
  validateSearch: (search: Record<string, unknown>): ScheduleGridSearch => {
    const out: ScheduleGridSearch = {};
    if (typeof search.company_id === 'string' && search.company_id)
      out.company_id = search.company_id;
    if (typeof search.week === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(search.week))
      out.week = search.week;
    return out;
  },
});

// E4 — Master Shift catalog (HR/Admin)
const shiftMastersRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/shifts',
  component: ShiftMastersScreen,
});

// E5 — Kehadiran (attendance dashboard, verification queue, detail, corrections)
const attendanceDashboardRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/attendance',
  component: AttendanceDashboardScreen,
});
const attendanceVerificationRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/attendance/verification',
  component: AttendanceVerificationScreen,
});
const attendanceDetailRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/attendance/$attendanceId',
  component: function AttendanceDetailRoute() {
    const { attendanceId } = attendanceDetailRoute.useParams();
    return <AttendanceDetailScreen attendanceId={attendanceId} />;
  },
});
const correctionsRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/corrections',
  component: CorrectionsScreen,
  validateSearch: (search: Record<string, unknown>): CorrectionsSearch => {
    const out: CorrectionsSearch = {};
    if (typeof search.q === 'string' && search.q) out.q = search.q;
    if (
      typeof search.status === 'string' &&
      (Object.values(CorrectionStatus) as string[]).includes(search.status)
    ) {
      out.status = search.status as CorrectionStatus;
    }
    if (
      typeof search.type === 'string' &&
      (Object.values(CorrectionType) as string[]).includes(search.type)
    ) {
      out.type = search.type as CorrectionType;
    }
    if (typeof search.cursor === 'string' && search.cursor) out.cursor = search.cursor;
    return out;
  },
});

const settingsRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/settings',
  component: SettingsLayout,
});
const settingsIndexRoute = createRoute({
  getParentRoute: () => settingsRoute,
  path: '/',
  component: SettingsOverviewScreen,
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
  component: AuditLogScreen,
  // Typed filter/cursor search params for the audit-log screen (D1).
  validateSearch: (
    search: Record<string, unknown>,
  ): {
    q?: string;
    entity_type?: string;
    action?: string;
    date_preset?: string;
    cursor?: string;
  } => {
    const out: {
      q?: string;
      entity_type?: string;
      action?: string;
      date_preset?: string;
      cursor?: string;
    } = {};
    if (typeof search.q === 'string' && search.q) out.q = search.q;
    if (typeof search.entity_type === 'string' && search.entity_type)
      out.entity_type = search.entity_type;
    if (typeof search.action === 'string' && search.action) out.action = search.action;
    if (typeof search.date_preset === 'string' && search.date_preset)
      out.date_preset = search.date_preset;
    if (typeof search.cursor === 'string' && search.cursor) out.cursor = search.cursor;
    return out;
  },
});
const settingsGeneralRoute = createRoute({
  getParentRoute: () => settingsRoute,
  path: 'general',
  component: SettingsGeneralScreen,
});

const routeTree = rootRoute.addChildren([
  loginRoute,
  forgotPasswordRoute,
  resetPasswordRoute,
  devGalleryRoute,
  devDataGalleryRoute,
  sessionExpiredRoute,
  forbiddenRoute,
  authedRoute.addChildren([
    indexRoute,
    employeesRoute,
    employeeNewRoute,
    employeeDetailRoute,
    changeRequestsRoute,
    agreementsRoute,
    agreementNewRoute,
    agreementDetailRoute,
    serviceLinesRoute,
    serviceLineDetailRoute,
    masterDataRoute,
    leaveTypesRoute,
    attendanceCodesRoute,
    overtimeRulesRoute,
    clientCompaniesRoute,
    clientCompanyNewRoute,
    clientCompanyDetailRoute,
    placementsRoute,
    placementNewRoute,
    placementDetailRoute,
    companyRosterRoute,
    scheduleRoute,
    shiftMastersRoute,
    attendanceDashboardRoute,
    attendanceVerificationRoute,
    attendanceDetailRoute,
    correctionsRoute,
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
