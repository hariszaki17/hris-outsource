# FE-Used Endpoint Inventory (authoritative scope)

**This is the scope contract for the milestone.** The backend implements EXACTLY the
operations below — the ones the web app (`frontend/apps/web/src/features/**`) actually
calls via the Orval-generated `@swp/api-client` hooks. Endpoints in the OpenAPI specs
that the FE does **not** call yet are **out of scope** (defer).

Derived 2026-06-03 by scanning hook usage in feature screens. All paths are under
`/api/v1`. Each operation MUST match its `docs/api/E#-*/openapi.yaml` request/response
schema exactly (the FE client is generated from those specs).

> When implementing a phase, re-verify the live set with a quick grep of the relevant
> `features/<epic>/` folder — the FE may have advanced since this snapshot.

---

## Auth (E1 spec) — gates everything, build first
The FE login screen currently stubs a dev token (TODO in `features/auth/login-screen.tsx`).
Wiring it to the real BE is part of Phase 1.
- `usePostAuthLogin` → `POST /auth/login`
- `usePostAuthRefresh` → `POST /auth/refresh`
- `usePostAuthLogout` → `POST /auth/logout`
- `usePostAuthForgotPassword` → `POST /auth/forgot-password` (forgot-password-screen)
- `usePostAuthResetPassword` → `POST /auth/reset-password` (reset-password-screen)
- `GET /auth/me` (current principal — already in the BE scaffold; FE `SessionUser` source)

## E1 — Foundations & Settings
- `useListUsers` → `GET /users`
- `useCreateUser` → `POST /users`
- `useUpdateUser` → `PATCH /users/{userId}`
- `useChangeUserRole` → `POST /users/{userId}:change-role`
- `useDeactivateUser` → `POST /users/{userId}:deactivate`
- `useReactivateUser` → `POST /users/{userId}:reactivate`
- `useSendUserPasswordReset` → `POST /users/{userId}:send-password-reset`
- `useListAuditLog` → `GET /audit-log`
- `useGetAuditLogEntry` → `GET /audit-log/{auditLogId}`
- `useGetPlatformSettings` → `GET /platform/settings`
- _Unwired screen:_ `settings-overview-screen.tsx` (static; no endpoint yet)

## E2 — Identity, Org & Master Data
**Org & master data:**
- `useListClientCompanies` → `GET /client-companies`
- `useGetClientCompany` → `GET /client-companies/{clientCompanyId}`
- `useCreateClientCompany` → `POST /client-companies`
- `useUpdateClientCompany` → `PATCH /client-companies/{clientCompanyId}`
- `useReactivateClientCompany` → `POST /client-companies/{clientCompanyId}:reactivate`
- `useListSites` → `GET /client-companies/{clientCompanyId}/sites`
- `useCreateSite` → `POST /client-companies/{clientCompanyId}/sites`
- `useUpdateSite` → `PATCH /sites/{siteId}`
- `useListServiceLines` → `GET /service-lines`
- `useGetServiceLine` → `GET /service-lines/{serviceLineId}`
- `useCreateServiceLine` → `POST /service-lines`
- `useUpdateServiceLine` → `PATCH /service-lines/{serviceLineId}`
- `useDiscontinueServiceLine` → `POST /service-lines/{serviceLineId}:discontinue`
- `useListPositionsInServiceLine` → `GET /service-lines/{serviceLineId}/positions`
- `useCreatePosition` → `POST /service-lines/{serviceLineId}/positions`
- `useUpdatePosition` → `PATCH /positions/{positionId}`
- `useSoftDeletePosition` → `DELETE /positions/{positionId}`
- `useListLeaveTypes` → `GET /leave-types`
- `useCreateLeaveType` → `POST /leave-types`
- `useUpdateLeaveType` → `PATCH /leave-types/{leaveTypeId}`
- `useSoftDeleteLeaveType` → `DELETE /leave-types/{leaveTypeId}`
- `useListAttendanceCodes` → `GET /attendance-codes`
- `useCreateAttendanceCode` → `POST /attendance-codes`
- `useUpdateAttendanceCode` → `PATCH /attendance-codes/{attendanceCodeId}`
- `useSoftDeleteAttendanceCode` → `DELETE /attendance-codes/{attendanceCodeId}`
- `useListOvertimeRules` → `GET /overtime-rules`
- `useCreateOvertimeRule` → `POST /overtime-rules`
- `useUpdateOvertimeRule` → `PATCH /overtime-rules/{overtimeRuleId}`
- `useSoftDeleteOvertimeRule` → `DELETE /overtime-rules/{overtimeRuleId}`
- _Unwired screen:_ `master-data-hub-screen.tsx` (static; no endpoint yet)

**People (employees, agreements, change-requests):**
- `useListEmployees` → `GET /employees`
- `useGetEmployee` → `GET /employees/{employeeId}`
- `useCreateEmployee` → `POST /employees`
- `useUpdateEmployee` → `PATCH /employees/{employeeId}`
- `useDeactivateEmployee` → `POST /employees/{employeeId}:deactivate`
- `useReactivateEmployee` → `POST /employees/{employeeId}:reactivate`
- `useListAgreements` → `GET /agreements`
- `useGetAgreement` → `GET /agreements/{agreementId}`
- `useCreateAgreement` → `POST /agreements`
- `useRenewAgreement` → `POST /agreements/{agreementId}:renew`
- `useCloseAgreement` → `POST /agreements/{agreementId}:close`
- `useUploadAgreementAttachment` → `POST /agreements/{agreementId}/attachments` (multipart)
- `useListPendingChangeRequests` → `GET /change-requests`
- `useGetChangeRequest` → `GET /change-requests/{changeRequestId}`
- `useApproveChangeRequest` → `POST /change-requests/{changeRequestId}:approve`
- `useRejectChangeRequest` → `POST /change-requests/{changeRequestId}:reject`

## E3 — Placement & Deployment
- `useListPlacements` → `GET /placements`
- `useListExpiringPlacements` → `GET /placements` (status/expiring filter)
- `useGetPlacement` → `GET /placements/{id}`
- `useCreatePlacement` → `POST /placements`
- `useRenewPlacement` → `POST /placements/{id}:renew`
- `useTransferPlacement` → `POST /placements/{id}:transfer`
- `useEndPlacement` → `POST /placements/{id}:end`
- `useResignPlacement` → `POST /placements/{id}:resign`
- `useTerminatePlacement` → `POST /placements/{id}:terminate`
- `useGetCompanyRoster` → `GET /client-companies/{companyId}/roster`
- `useCreateShiftLeaderAssignment` → `POST /shift-leader-assignments`
- `useReplaceShiftLeaderAssignment` → `POST /shift-leader-assignments/{id}:replace`
- `useEndShiftLeaderAssignment` → `POST /shift-leader-assignments/{id}:end`
- (`useListAgreements` reused from E2)

## E4 — Schedule & Shifts
- `useListShiftMasters` → `GET /shift-masters`
- `useCreateShiftMaster` → `POST /shift-masters`
- `useUpdateShiftMaster` → `PATCH /shift-masters/{id}`
- `useDeactivateShiftMaster` → `POST /shift-masters/{id}:deactivate`
- `useReactivateShiftMaster` → `POST /shift-masters/{id}:reactivate`
- `useListSchedule` → `GET /schedule`
- `useCreateScheduleEntry` → `POST /schedule`
- `useUpdateScheduleEntry` → `PATCH /schedule/{id}`
- `useDeleteScheduleEntry` → `DELETE /schedule/{id}`
- `useCheckScheduleConflicts` → `POST /schedule:check`
- `useBulkApplySchedule` → `POST /schedule:bulk-apply`

## E5 — Attendance & Verification
- `useListAttendance` → `GET /attendance`
- `useGetAttendance` → `GET /attendance/{id}`
- `useVerifyAttendance` → `POST /attendance/{id}:verify`
- `useRejectAttendance` → `POST /attendance/{id}:reject`
- `useBulkVerifyAttendance` → `POST /attendance:bulk-verify`
- `useBulkRejectAttendance` → `POST /attendance:bulk-reject`
- `useListCorrections` → `GET /corrections`
- `useGetCorrection` → `GET /corrections/{id}`
- `useApproveCorrection` → `POST /corrections/{id}:approve`
- `useRejectCorrection` → `POST /corrections/{id}:reject`

## E6 — Leave Requests & Quotas
- `useListLeaveRequests` → `GET /leave-requests`
- `useGetLeaveRequest` → `GET /leave-requests/{id}`
- `useApproveLeaveRequestL1` → `POST /leave-requests/{id}:approve-l1`
- `useApproveLeaveRequestFinal` → `POST /leave-requests/{id}:approve-final`
- `useApproveLeaveRequestOverride` → `POST /leave-requests/{id}:approve-override`
- `useRejectLeaveRequest` → `POST /leave-requests/{id}:reject`
- `useListLeaveQuotas` → `GET /leave-quotas`
- `useAdjustLeaveQuota` → `POST /leave-quotas/{id}:adjust`
- `useBulkGrantLeaveQuotas` → `POST /leave-quotas:bulk-grant`
- `useGetLeaveCalendar` → `GET /leave-calendar`

## E7 — Overtime & Holidays
- `useListOvertime` → `GET /overtime`
- `useGetOvertime` → `GET /overtime/{id}`
- `useConfirmOvertime` → `POST /overtime/{id}:confirm`
- `useApproveOvertimeL1` → `POST /overtime/{id}:approve-l1`
- `useApproveOvertimeFinal` → `POST /overtime/{id}:approve-final`
- `useRejectOvertime` → `POST /overtime/{id}:reject`
- `useWithdrawOvertime` → `POST /overtime/{id}:withdraw`
- `useBulkApproveOvertime` → `POST /overtime:bulk-approve`
- `useBulkRejectOvertime` → `POST /overtime:bulk-reject`
- `useListHolidays` → `GET /holidays`
- `useCreateHoliday` → `POST /holidays`
- `useUpdateHoliday` → `PATCH /holidays/{id}`
- `useDeleteHoliday` → `DELETE /holidays/{id}`
- (`useListOvertimeRules` reused from E2)

## E8 — Payroll
- `useListPayslips` → `GET /payslips`
- `useGetPayslip` → `GET /payslips/{id}`
- `useExportPayslips` → `POST /payslips:export`
- `useListPayslipAuditNotes` → `GET /payslips/{id}/audit-notes`
- `useCreatePayslipAuditNote` → `POST /payslips/{id}/audit-notes`

## E10 — Reporting, Notifications & Exports
- `useGetMyDashboard` → `GET /dashboards/me`
- `useGetBillableAttendanceReport` → `GET /reports/attendance-billable`
- `useListNotifications` → `GET /notifications`
- `useMarkNotificationRead` → `POST /notifications/{notificationId}:mark-read`
- `useMarkAllNotificationsRead` → `POST /notifications:mark-all-read`
- `useCreateExport` → `POST /exports`
- `useGetExport` → `GET /exports/{exportId}`
- `useCancelExport` → `POST /exports/{exportId}:cancel`
