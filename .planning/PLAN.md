# PLAN: F5.6 Manual Attendance — Page-based flow + created_by traceability

## Summary

Transform manual attendance from modal to dedicated page at `/attendance/manual-create` with employee+date selection, server-side autofill of shift/placement data, flexible manual overrides, and `created_by` tracking on attendance records.

## Tasks

### 1. DB: Migration `00046` — add `created_by` column
- `backend/db/migrations/00046_manual_attendance_traceability.sql`
- `ALTER TABLE attendance ADD COLUMN created_by text REFERENCES employees(id);`
- Nullable: system events (auto-close) leave null; organic clock-in sets agent; manual entry sets HR admin.

### 2. SQL: Update queries + add autofill query
- `backend/db/queries/attendance/attendance.sql`
- Update `CreateManualAttendance` INSERT to include `created_by` column
- Add new `GetManualAutofillData` query: returns placement + schedule for (employee_id, date)
  - Joins placements (active, date-range overlap), schedule_entries (by employee + date), client_companies, client_sites, positions
- Regenerate sqlc: `cd backend && sqlc generate`

### 3. Domain: Add `CreatedBy` field
- `backend/internal/domain/attendance/attendance.go`
- Add `CreatedBy *string` to `Attendance` struct

### 4. Repository: Update + add autofill method
- `backend/internal/repository/attendance/attendance_repo.go`
- `CreateManualAttendanceParams` — add `CreatedBy string`
- `CreateManualAttendance` — pass `created_by` to sqlc
- Add `GetManualAutofillData(ctx, employeeID, date string)` → returns placement info + schedule info
- `mapping.go` — update `attendanceCols` + all mappers to include `CreatedBy`

### 5. Service: Update + add autofill method
- `backend/internal/service/attendance/attendance_service.go`
- `ManualCreateRequest` — remove `AttendanceCodeID`, add `CreatedBy string`
- `CreateManualAttendanceParams` — add `CreatedBy string`
- `ManualCreate` — pass `created_by` from req
- Add `ManualAutofillData` struct and `GetManualAutofillData(ctx, employeeID, date)` method
  - Returns: placement info, schedule info (if any), employee name, company name, site name, position name
  - Returns 404 if no active placement

### 6. Handler: Update DTO + add autofill endpoint
- `backend/internal/handler/attendance/attendance_dto.go`
- `manualCreateRequest` — remove `attendance_code_id`
- Add `manualAutofillResponse` DTO
- `backend/internal/handler/attendance/attendance_handler.go`
- `ManualCreate` — extract `created_by` from auth principal, populate `ManualCreateRequest.CreatedBy`
- Add `ManualAutofill` handler: `GET /attendance:manual-autofill?employee_id=xxx&date=yyyy-mm-dd`
- Route: register new endpoint in server.go

### 7. Handler tests: Update + add autofill tests
- `backend/internal/handler/attendance/attendance_handler_test.go`
- Update existing manual create tests (no more `attendance_code_id`, verify `created_by`)
- Add `TestManualAutofill_Success`, `TestManualAutofill_NoPlacement`, `TestManualAutofill_MissingParams`

### 8. Frontend: New page + route + remove modal
- Delete `frontend/apps/web/src/features/e5-attendance/manual-attendance-modal.tsx`
- Create `frontend/apps/web/src/features/e5-attendance/manual-attendance-create-screen.tsx`
  - Step 1: Employee search/select + date picker
  - Step 2: Autofill — fetch `GET /attendance:manual-autofill`, show shift/placement/company/site info
  - Step 3: If autofill finds schedule → prefill; if not → manual selectors for shift start/end
  - Step 4: Check-in/check-out datetime inputs (no kode absensi)
  - Step 5: Note (optional) + Submit
  - On submit: `POST /attendance:manual-create`
- Register route `/attendance/manual-create` in router.tsx
- Update dashboard screen — replace modal trigger with `Link` to `/attendance/manual-create`
- Add i18n keys for page labels
