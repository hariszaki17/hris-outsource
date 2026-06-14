import { AgentAkunScreen } from '@/features/agent/me-akun-screen.tsx';
import { AgentKehadiranScreen } from '@/features/agent/me-kehadiran-screen.tsx';
import { AgentNotificationsScreen } from '@/features/agent/me-notifications-screen.tsx';
import { AgentPengajuanScreen } from '@/features/agent/me-pengajuan-screen.tsx';
import { ForgotPasswordScreen } from '@/features/auth/forgot-password-screen.tsx';
import { LoginScreen } from '@/features/auth/login-screen.tsx';
import { ResetPasswordScreen } from '@/features/auth/reset-password-screen.tsx';
import { ComponentGallery } from '@/features/dev/component-gallery.tsx';
import { DataComponentsGallery } from '@/features/dev/data-components-gallery.tsx';
import { AuditLogScreen } from '@/features/e1-foundations/audit-log-screen.tsx';
import {
  NoPermissionScreen,
  SessionExpiredScreen,
} from '@/features/e1-foundations/global-states.tsx';
import { SettingsGeneralScreen } from '@/features/e1-foundations/settings-general-screen.tsx';
import { SettingsLayout } from '@/features/e1-foundations/settings-layout.tsx';
import { UsersScreen } from '@/features/e1-foundations/users-screen.tsx';
import { AgreementDetailScreen } from '@/features/e2-identity/agreement-detail-screen.tsx';
import { CreateAgreementScreen } from '@/features/e2-identity/agreement-form.tsx';
import {
  AgreementsScreen,
  type AgreementsSearch,
} from '@/features/e2-identity/agreements-screen.tsx';
import { AttendanceCodesScreen } from '@/features/e2-identity/attendance-codes-screen.tsx';
import {
  ClientCompaniesScreen,
  type ClientCompaniesSearch,
} from '@/features/e2-identity/client-companies-screen.tsx';
import { ClientCompanyDetailScreen } from '@/features/e2-identity/client-company-detail-screen.tsx';
import {
  CreateClientCompanyScreen,
  EditClientCompanyScreen,
} from '@/features/e2-identity/client-company-form.tsx';
import { EmployeeDetailScreen } from '@/features/e2-identity/employee-detail-screen.tsx';
import { CreateEmployeeScreen } from '@/features/e2-identity/employee-form.tsx';
import { EmployeesScreen, type EmployeesSearch } from '@/features/e2-identity/employees-screen.tsx';
import { LeaveTypesScreen } from '@/features/e2-identity/leave-types-screen.tsx';
import { MasterDataHubScreen } from '@/features/e2-identity/master-data-hub-screen.tsx';
import { OvertimeRulesScreen } from '@/features/e2-identity/overtime-rules-screen.tsx';
import {
  CompanyRosterScreen,
  type CompanyRosterSearch,
} from '@/features/e3-placement/company-roster-screen.tsx';
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
import {
  AttendanceDashboardScreen,
  type AttendanceDashboardSearch,
} from '@/features/e5-attendance/attendance-dashboard-screen.tsx';
import { AttendanceDetailScreen } from '@/features/e5-attendance/attendance-detail-screen.tsx';
import { AttendanceVerificationScreen } from '@/features/e5-attendance/attendance-verification-screen.tsx';
import {
  CorrectionsScreen,
  type CorrectionsSearch,
} from '@/features/e5-attendance/corrections-screen.tsx';
import { ManualAttendanceCreateScreen } from '@/features/e5-attendance/manual-attendance-create-screen.tsx';
import { LeaveApprovalsScreen } from '@/features/e6-leave/leave-approvals-screen.tsx';
import { LeaveCalendarScreen } from '@/features/e6-leave/leave-calendar-screen.tsx';
import { LeaveDetailScreen } from '@/features/e6-leave/leave-detail-screen.tsx';
import { LeaveQuotasScreen } from '@/features/e6-leave/leave-quotas-screen.tsx';
import { OvertimeApprovalsScreen } from '@/features/e7-overtime/overtime-approvals-screen.tsx';
import { OvertimeDetailScreen } from '@/features/e7-overtime/overtime-detail-screen.tsx';
import { OvertimeRecordsScreen } from '@/features/e7-overtime/overtime-records-screen.tsx';
import { OvertimeRulesScreen as OvertimeRulesHolidaysScreen } from '@/features/e7-overtime/overtime-rules-screen.tsx';
import { PayslipArchiveScreen } from '@/features/e8-payroll/payslip-archive-screen.tsx';
import { PayslipDetailRoute as PayslipDetailRouteView } from '@/features/e8-payroll/payslip-detail-route.tsx';
import { BillableReportScreen } from '@/features/e10-reporting/billable-report-screen.tsx';
import { DashboardScreen } from '@/features/e10-reporting/dashboard-screen.tsx';
import { NotificationsScreen } from '@/features/e10-reporting/notifications-screen.tsx';
import ApprovalDetailScreen from '@/features/e11-approvals/approval-detail-screen.tsx';
import ApprovalInboxScreen from '@/features/e11-approvals/approval-inbox-screen.tsx';
import ApprovalTemplateEditorScreen from '@/features/e11-approvals/approval-template-editor-screen.tsx';
import { auth } from '@/lib/auth.ts';
import { Role, UserStatus } from '@swp/api-client/e1';
import {
  AgreementStatus,
  AgreementType,
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
  useNavigate,
} from '@tanstack/react-router';
import { hasPermission, routeRequirement } from './nav.ts';
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

interface ResetPasswordSearch {
  token?: string;
}

const resetPasswordRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/reset-password',
  component: ResetPasswordScreen,
  validateSearch: (search: Record<string, unknown>): ResetPasswordSearch => {
    const out: ResetPasswordSearch = {};
    if (typeof search.token === 'string' && search.token) out.token = search.token;
    return out;
  },
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
    // Capability guard (NAVIGATION-AND-RBAC §4.4): a permitted-section-but-denied deep link
    // redirects to /forbidden rather than rendering a broken screen. Defense-in-depth only —
    // the Go API is the real gate (ENGINEERING.md C1); SCOPE stays server-side. Skipped while
    // the user object is still loading (token present, /me in flight) — the shell handles that.
    const user = auth.getUser();
    if (user) {
      // Agents have no staff dashboard — send them to their self-service home (/me).
      // docs/eng/AGENT-WEB-ACCESS.md (AW-2/AW-4).
      if (user.role === 'agent' && location.pathname === '/') {
        throw redirect({ to: '/me' });
      }
      const requires = routeRequirement(location.pathname);
      if (requires && !hasPermission(user.permissions, requires)) {
        throw redirect({ to: '/forbidden' });
      }
    }
  },
  component: AppShell,
});

const indexRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/',
  component: DashboardScreen,
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
    if (typeof search.client_company === 'string' && search.client_company)
      out.client_company = search.client_company;
    if (search.tab === 'all' || search.tab === 'active' || search.tab === 'inactive') {
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

// E2 — Perjanjian Kerja (employment agreements list, detail, create)
const agreementsRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/agreements',
  component: AgreementsScreen,
  validateSearch: (search: Record<string, unknown>): AgreementsSearch => {
    const out: AgreementsSearch = {};
    if (typeof search.q === 'string' && search.q) out.q = search.q;
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
const clientCompanyEditRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/client-companies/$clientCompanyId/edit',
  component: function ClientCompanyEditRoute() {
    const { clientCompanyId } = clientCompanyEditRoute.useParams();
    return <EditClientCompanyScreen clientCompanyId={clientCompanyId} />;
  },
});
// E11 — Template Persetujuan, homed under the client-company detail (F11.1). Gated
// `approvals.template.manage` via routeRequirement (nav.ts). The tab lives inside the
// detail screen; this is the deep-linkable child route.
const clientCompanyApprovalTemplateRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/client-companies/$clientCompanyId/approval-template',
  component: function ClientCompanyApprovalTemplateRoute() {
    const { clientCompanyId } = clientCompanyApprovalTemplateRoute.useParams();
    return <ApprovalTemplateEditorScreen companyId={clientCompanyId} />;
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
    if (typeof search.awaiting_agreement === 'boolean')
      out.awaiting_agreement = search.awaiting_agreement;
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
  validateSearch: (search: Record<string, unknown>): CompanyRosterSearch => {
    const out: CompanyRosterSearch = {};
    if (typeof search.q === 'string' && search.q) out.q = search.q;
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
    if (typeof search.include_history === 'boolean') out.include_history = search.include_history;
    if (typeof search.awaiting_agreement === 'boolean')
      out.awaiting_agreement = search.awaiting_agreement;
    if (typeof search.cursor === 'string' && search.cursor) out.cursor = search.cursor;
    return out;
  },
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
  validateSearch: (search: Record<string, unknown>): AttendanceDashboardSearch => {
    const out: AttendanceDashboardSearch = {};
    if (typeof search.q === 'string' && search.q) out.q = search.q;
    if (typeof search.tab === 'string' && ['all', 'present', 'late', 'absent'].includes(search.tab))
      out.tab = search.tab as AttendanceDashboardSearch['tab'];
    if (typeof search.company_id === 'string' && search.company_id)
      out.company_id = search.company_id;
    if (typeof search.site_id === 'string' && search.site_id) out.site_id = search.site_id;
    if (typeof search.position === 'string' && search.position) out.position = search.position;
    if (typeof search.cursor === 'string' && search.cursor) out.cursor = search.cursor;
    if (typeof search.date_from === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(search.date_from))
      out.date_from = search.date_from;
    if (typeof search.date_to === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(search.date_to))
      out.date_to = search.date_to;
    return out;
  },
});
const attendanceManualCreateRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/attendance/manual-create',
  component: ManualAttendanceCreateScreen,
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

// E6 — Cuti (leave approvals queue, detail, quotas, calendar)
const leaveApprovalsRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/leave',
  component: LeaveApprovalsScreen,
});
const leaveDetailRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/leave/$leaveRequestId',
  component: function LeaveDetailRoute() {
    const { leaveRequestId } = leaveDetailRoute.useParams();
    return <LeaveDetailScreen leaveRequestId={leaveRequestId} />;
  },
});
const leaveQuotasRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/leave/quotas',
  component: LeaveQuotasScreen,
});
const leaveCalendarRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/leave/calendar',
  component: LeaveCalendarScreen,
});

// E7 — Lembur (Overtime)
const overtimeRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/overtime',
  component: OvertimeApprovalsScreen,
});
const overtimeRekapRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/overtime/rekap',
  component: OvertimeRecordsScreen,
});
const overtimeAturanRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/overtime/aturan',
  component: OvertimeRulesHolidaysScreen,
});
const overtimeDetailRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/overtime/$overtimeId',
  component: function OvertimeDetailRoute() {
    const { overtimeId } = overtimeDetailRoute.useParams();
    return <OvertimeDetailScreen overtimeId={overtimeId} />;
  },
});

// E8 — Payroll (read-only)
const payrollRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/payroll',
  component: function PayrollArchiveRoute() {
    const navigate = useNavigate();
    return (
      <PayslipArchiveScreen
        onRowClick={(payslipId) => navigate({ to: '/payroll/$payslipId', params: { payslipId } })}
      />
    );
  },
});
const payslipDetailRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/payroll/$payslipId',
  component: function PayslipDetailRoute() {
    const { payslipId } = payslipDetailRoute.useParams();
    return <PayslipDetailRouteView payslipId={payslipId} />;
  },
});

// E11 — Approvals: the /inbox now renders the E11 aggregated approval queue
// (ApprovalInboxScreen). The old E10 placeholder InboxScreen (a read-only view over the
// dashboard's pending_approvals_panel) is removed. Attendance verifiers reach /inbox via the
// shared anyOf[approvals.act, attendance.verify] gate; their queue is the standalone
// /attendance/verification screen (linked from the nav + from a banner here), so no
// attendance content is lost. onOpenInstance deep-links into the approval-instance detail.
const inboxRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/inbox',
  component: function InboxRoute() {
    const navigate = useNavigate();
    return (
      <ApprovalInboxScreen
        onOpenInstance={(instanceId) =>
          navigate({ to: '/approval-instances/$instanceId', params: { instanceId } })
        }
      />
    );
  },
});
const approvalInstanceDetailRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/approval-instances/$instanceId',
  component: function ApprovalInstanceDetailRoute() {
    const { instanceId } = approvalInstanceDetailRoute.useParams();
    return <ApprovalDetailScreen instanceId={instanceId} />;
  },
});
const reportsRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/reports',
  component: BillableReportScreen,
});
const notificationsRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/notifications',
  component: NotificationsScreen,
});

const settingsRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/settings',
  component: SettingsLayout,
});
const settingsIndexRoute = createRoute({
  getParentRoute: () => settingsRoute,
  path: '/',
  // Overview is hidden from nav; the section entry (/settings) lands on Role Management.
  beforeLoad: () => {
    throw redirect({ to: '/settings/users' });
  },
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

// Agent self-service (/me/*) — docs/eng/AGENT-WEB-ACCESS.md. Three merged homes gated by `self.*`
// keys via the authedRoute capability guard; rendered inside AppShell with the agent nav backbone:
//   /me           Kehadiran (dashboard + attendance + schedule + live clock/clock-in-out)
//   /me/pengajuan Pengajuan (leave + overtime request tabs)
//   /me/akun      Akun (profile + payslip + tiered Ubah Profil)
const meKehadiranRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/me',
  component: AgentKehadiranScreen,
});
const mePengajuanRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/me/pengajuan',
  component: AgentPengajuanScreen,
});
const meAkunRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/me/akun',
  component: AgentAkunScreen,
});
const meNotificationsRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/me/notifications',
  component: AgentNotificationsScreen,
});

// Legacy /me/* paths kept as redirects so bookmarks/deep links survive the nav merge
// (Plan §E). attendance/schedule → Kehadiran (/me); leave/overtime → Pengajuan; profile/payslip
// → Akun. `beforeLoad` throws the redirect before any component renders.
const meRedirect = (from: string, to: '/me' | '/me/pengajuan' | '/me/akun') =>
  createRoute({
    getParentRoute: () => authedRoute,
    path: from,
    beforeLoad: () => {
      throw redirect({ to });
    },
  });
const meAttendanceRedirectRoute = meRedirect('/me/attendance', '/me');
const meScheduleRedirectRoute = meRedirect('/me/schedule', '/me');
const meLeaveRedirectRoute = meRedirect('/me/leave', '/me/pengajuan');
const meOvertimeRedirectRoute = meRedirect('/me/overtime', '/me/pengajuan');
const meProfileRedirectRoute = meRedirect('/me/profile', '/me/akun');
const mePayslipRedirectRoute = meRedirect('/me/payslip', '/me/akun');
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
    agreementsRoute,
    agreementNewRoute,
    agreementDetailRoute,
    masterDataRoute,
    leaveTypesRoute,
    attendanceCodesRoute,
    overtimeRulesRoute,
    clientCompaniesRoute,
    clientCompanyNewRoute,
    clientCompanyDetailRoute,
    clientCompanyEditRoute,
    clientCompanyApprovalTemplateRoute,
    placementsRoute,
    placementNewRoute,
    placementDetailRoute,
    companyRosterRoute,
    scheduleRoute,
    shiftMastersRoute,
    attendanceDashboardRoute,
    attendanceManualCreateRoute,
    attendanceVerificationRoute,
    attendanceDetailRoute,
    correctionsRoute,
    leaveApprovalsRoute,
    leaveDetailRoute,
    leaveQuotasRoute,
    leaveCalendarRoute,
    overtimeRekapRoute,
    overtimeAturanRoute,
    overtimeDetailRoute,
    overtimeRoute,
    payslipDetailRoute,
    payrollRoute,
    inboxRoute,
    approvalInstanceDetailRoute,
    reportsRoute,
    notificationsRoute,
    // Agent self-service (/me/*) — three merged homes + legacy redirects.
    meKehadiranRoute,
    mePengajuanRoute,
    meAkunRoute,
    meNotificationsRoute,
    meAttendanceRedirectRoute,
    meScheduleRedirectRoute,
    meLeaveRedirectRoute,
    meOvertimeRedirectRoute,
    meProfileRedirectRoute,
    mePayslipRedirectRoute,
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
