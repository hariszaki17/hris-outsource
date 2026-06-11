// Package attendance (handler) — attendance request/response DTOs + snake_case
// mappers. The Attendance response matches the openapi byte-for-shape: nullable
// required fields (check_out_at, schedule_id, geofence_out, verified_by, …) are
// emitted as JSON `null` (pointer, NO omitempty). Timestamps are UTC RFC3339.
package attendance

import (
	"time"

	att "github.com/hariszaki17/hris-outsource/backend/internal/domain/attendance"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/attendance"
)

// --- request DTOs ---

type verifyRequest struct {
	Note       string  `json:"note"`
	CheckInAt  *string `json:"check_in_at"`
	CheckOutAt *string `json:"check_out_at"`
}

type rejectRequest struct {
	Reason string `json:"reason"`
}

type bulkVerifyRequest struct {
	IDs  []string `json:"ids"`
	Note string   `json:"note"`
}

type bulkRejectRequest struct {
	IDs    []string `json:"ids"`
	Reason string   `json:"reason"`
}


// manualCreateRequest is the openapi ManualCreateRequest (F5.6).
type manualCreateRequest struct {
	EmployeeID  string  `json:"employee_id"`
	CheckInAt   string  `json:"check_in_at"`
	CheckOutAt  *string `json:"check_out_at"`
	Note        string  `json:"note,omitempty"`
}

// autofillResponse is the response for GET /attendance:manual-autofill.
type autofillResponse struct {
	EmployeeName string  `json:"employee_name"`
	CompanyName  string  `json:"company_name"`
	SiteName     *string `json:"site_name"`
	PositionName *string `json:"position_name"`
	ServiceLine  string  `json:"service_line"`
	// Schedule info (null when no schedule for the date).
	ScheduleID   *string `json:"schedule_id"`
	ShiftStartAt *string `json:"shift_start_at"`
	ShiftEndAt   *string `json:"shift_end_at"`
	// Existing attendance for this employee+date (null when none). When present the
	// cron already created a record (e.g. ABSENT/PENDING) — the form redirects the
	// admin to verify/correct it instead of creating a duplicate.
	ExistingAttendanceID   *string `json:"existing_attendance_id"`
	ExistingAttendanceStat *string `json:"existing_attendance_status"`
	ExistingVerification   *string `json:"existing_verification_status"`
}
// clockInRequest is the openapi ClockInRequest. wfo is a *bool so an omitted value
// applies the spec default (true); employee_id is intentionally omitted — the agent is
// always self (server fills from token).
type clockInRequest struct {
	Lat                  float64 `json:"lat"`
	Lng                  float64 `json:"lng"`
	GPSAvailable         bool    `json:"gps_available"`
	WFO                  *bool   `json:"wfo"`
	PhotoID              *string `json:"photo_id"`
	ForceOutsideGeofence bool    `json:"force_outside_geofence"`
}

// wfoOrDefault applies the spec default (true) when wfo is omitted.
func (c clockInRequest) wfoOrDefault() bool {
	if c.WFO == nil {
		return true
	}
	return *c.WFO
}

// clockOutRequest is the openapi ClockOutRequest.
type clockOutRequest struct {
	Lat          float64 `json:"lat"`
	Lng          float64 `json:"lng"`
	GPSAvailable bool    `json:"gps_available"`
	PhotoID      *string `json:"photo_id"`
}

// --- response DTOs ---

// geofenceResponse is the openapi GeofenceCheck (omitted/null when no capture).
type geofenceResponse struct {
	Inside    bool `json:"inside"`
	DistanceM int  `json:"distance_m"`
	RadiusM   int  `json:"radius_m"`
}

// attendanceResponse is the openapi Attendance object. Required-nullable fields
// use pointers WITHOUT omitempty so they serialize as `null`, not absent.
type attendanceResponse struct {
	ID               string  `json:"id"`
	EmployeeID       string  `json:"employee_id"`
	EmployeeName     *string `json:"employee_name,omitempty"`
	PlacementID      string  `json:"placement_id"`
	ScheduleID       *string `json:"schedule_id"`
	CompanyID        string  `json:"company_id"`
	CompanyName      *string `json:"company_name,omitempty"`
	SiteID           string  `json:"site_id"`
	SiteName         *string `json:"site_name,omitempty"`
	ServiceLine      string  `json:"service_line"`
	PositionID       string  `json:"position_id"`
	PositionName     *string `json:"position_name,omitempty"`
	AttendanceCodeID *string `json:"attendance_code_id"`

	ShiftStartAt *string `json:"shift_start_at"`
	ShiftEndAt   *string `json:"shift_end_at"`

	// check_in_at / lat_in / lng_in are nullable: a true ABSENT record has no
	// clock-in (null) — pointer WITHOUT omitempty so they serialize as `null`.
	CheckInAt  *string  `json:"check_in_at"`
	CheckOutAt *string  `json:"check_out_at"`
	LatIn      *float64 `json:"lat_in"`
	LngIn      *float64 `json:"lng_in"`
	LatOut     *float64 `json:"lat_out"`
	LngOut     *float64 `json:"lng_out"`
	PhotoInID  *string  `json:"photo_in_id"`
	PhotoOutID *string  `json:"photo_out_id"`

	WFO           bool `json:"wfo"`
	LateMinutes   int  `json:"late_minutes"`
	WorkedMinutes *int `json:"worked_minutes"`
	AutoClosed    bool `json:"auto_closed"`

	// CanCheckOut is a server-computed display hint for the mobile toggle (F5.1): true
	// only for an OPEN record still within its checkout window (shift_end + grace). An
	// open row past its window reads false → mobile shows "Check In", not "Check Out".
	CanCheckOut bool `json:"can_check_out"`

	Status             string   `json:"status"`
	VerificationStatus string   `json:"verification_status"`
	Flags              []string `json:"flags"`

	GeofenceIn  *geofenceResponse `json:"geofence_in"`
	GeofenceOut *geofenceResponse `json:"geofence_out"`

	VerifiedBy       *string `json:"verified_by"`
	VerifiedAt       *string `json:"verified_at"`
	RejectedBy       *string `json:"rejected_by"`
	RejectedAt       *string `json:"rejected_at"`
	RejectReason     *string `json:"reject_reason"`
	LastCorrectionID *string `json:"last_correction_id"`

	CreatedBy *string `json:"created_by"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// dataResponse[T] is the generic single-object envelope `{ "data": ... }`.
type dataResponse[T any] struct {
	Data T `json:"data"`
}

// clockInResponse is the clock-in envelope (F5.1). It extends the data envelope with
// auto_closed_previous (a stale forgotten clock-out was auto-closed to let this check-in
// through) + an informative message. Additive/backward-compatible: existing clients that
// read only `data` are unaffected.
type clockInResponse struct {
	Data               attendanceResponse `json:"data"`
	AutoClosedPrevious bool               `json:"auto_closed_previous"`
	Message            string             `json:"message"`
}

// clockInMessage is the Bahasa user message for the check-in result.
func clockInMessage(autoClosedPrevious bool) string {
	if autoClosedPrevious {
		return "Berhasil Check In. Absensi sebelumnya belum di-check out dan telah ditutup otomatis oleh sistem."
	}
	return "Berhasil Check In."
}

// bulkActionResponse is the openapi BulkActionResponse (200 ≥1 succeeded / 422 all failed).
type bulkActionResponse struct {
	Succeeded []string         `json:"succeeded"`
	Failed    []bulkFailedItem `json:"failed"`
}

type bulkFailedItem struct {
	ID    string        `json:"id"`
	Error bulkFailedErr `json:"error"`
}

type bulkFailedErr struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// --- mappers ---

func rfc3339(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func rfc3339Ptr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

func toGeofenceResponse(g *att.GeofenceCheck) *geofenceResponse {
	if g == nil {
		return nil
	}
	return &geofenceResponse{Inside: g.Inside, DistanceM: g.DistanceM, RadiusM: g.RadiusM}
}

func toAttendanceResponse(a att.Attendance) attendanceResponse {
	flags := make([]string, 0, len(a.Flags))
	for _, f := range a.Flags {
		flags = append(flags, string(f))
	}
	return attendanceResponse{
		ID:                 a.ID,
		EmployeeID:         a.EmployeeID,
		EmployeeName:       a.EmployeeName,
		PlacementID:        a.PlacementID,
		ScheduleID:         a.ScheduleID,
		CompanyID:          a.CompanyID,
		CompanyName:        a.CompanyName,
		SiteID:             a.SiteID,
		SiteName:           a.SiteName,
		ServiceLine:        a.ServiceLine,
		PositionID:         a.PositionID,
		PositionName:       a.PositionName,
		AttendanceCodeID:   a.AttendanceCodeID,
		ShiftStartAt:       rfc3339Ptr(a.ShiftStartAt),
		ShiftEndAt:         rfc3339Ptr(a.ShiftEndAt),
		CheckInAt:          rfc3339Ptr(a.CheckInAt),
		CheckOutAt:         rfc3339Ptr(a.CheckOutAt),
		LatIn:              a.LatIn,
		LngIn:              a.LngIn,
		LatOut:             a.LatOut,
		LngOut:             a.LngOut,
		PhotoInID:          a.PhotoInID,
		PhotoOutID:         a.PhotoOutID,
		WFO:                a.WFO,
		LateMinutes:        a.LateMinutes,
		WorkedMinutes:      a.WorkedMinutes,
		AutoClosed:         a.AutoClosed,
		CanCheckOut:        att.IsWithinCheckoutWindow(a, time.Now()),
		Status:             string(a.Status),
		VerificationStatus: string(a.VerificationStatus),
		Flags:              flags,
		GeofenceIn:         toGeofenceResponse(a.GeofenceIn),
		GeofenceOut:        toGeofenceResponse(a.GeofenceOut),
		VerifiedBy:         a.VerifiedBy,
		VerifiedAt:         rfc3339Ptr(a.VerifiedAt),
		RejectedBy:         a.RejectedBy,
		RejectedAt:         rfc3339Ptr(a.RejectedAt),
		RejectReason:       a.RejectReason,
		LastCorrectionID:   a.LastCorrectionID,
		CreatedBy:          a.CreatedBy,
		CreatedAt:          rfc3339(a.CreatedAt),
		UpdatedAt:          rfc3339(a.UpdatedAt),
	}
}

func toBulkActionResponse(r svc.BulkResult) bulkActionResponse {
	out := bulkActionResponse{
		Succeeded: make([]string, 0, len(r.Succeeded)),
		Failed:    make([]bulkFailedItem, 0, len(r.Failed)),
	}
	out.Succeeded = append(out.Succeeded, r.Succeeded...)
	for _, f := range r.Failed {
		out.Failed = append(out.Failed, bulkFailedItem{
			ID:    f.ID,
			Error: bulkFailedErr{Code: f.Code, Message: f.Message},
		})
	}
	return out
}
