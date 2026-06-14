// Package attendance (repository) — sqlc-row → domain mappers + pg type helpers
// for the E5 slice. Mirrors internal/repository/scheduling/mapping.go:
//   - jsonb original_snapshot surfaces as []byte → json.Unmarshal into map[string]any.
//   - attendance_shift_date surfaces as pgtype.Date → converted at this boundary.
//   - integer columns surface as int32 → converted to int / *int.
//   - flags text[] → []domain.Flag.
//   - geofence stored columns assembled into *domain.GeofenceCheck (nil when the
//     inside flag is absent).
//
// pgx.ErrNoRows → domain.ErrNotFound.
package attendance

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	domain "github.com/hariszaki17/hris-outsource/backend/internal/domain"
	att "github.com/hariszaki17/hris-outsource/backend/internal/domain/attendance"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
)

// --- pg type / error helpers ---

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

func pgDateToTime(d pgtype.Date) time.Time {
	if !d.Valid {
		return time.Time{}
	}
	return d.Time
}

func i32ToInt(v int32) int { return int(v) }

func i32PtrToIntPtr(v *int32) *int {
	if v == nil {
		return nil
	}
	n := int(*v)
	return &n
}

// flagsFrom casts the text[] flags column to the domain Flag slice.
func flagsFrom(raw []string) []att.Flag {
	out := make([]att.Flag, 0, len(raw))
	for _, f := range raw {
		out = append(out, att.Flag(f))
	}
	return out
}

// geofenceFrom assembles a *GeofenceCheck from the stored columns. nil when the
// inside flag is absent (NULL) — matches the "no geofence captured" case.
func geofenceFrom(inside *bool, distanceM *int32, radiusM int32) *att.GeofenceCheck {
	if inside == nil {
		return nil
	}
	g := att.GeofenceCheck{Inside: *inside, RadiusM: int(radiusM)}
	if distanceM != nil {
		g.DistanceM = int(*distanceM)
	}
	return &g
}

// snapshotFrom unmarshals the jsonb original_snapshot ([]byte) to map[string]any.
func snapshotFrom(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return map[string]any{}
	}
	return m
}

// --- attendance row shape ---

// attendanceCols is the common column set every attendance Row shares (list,
// get, forUpdate, verify, reject, apply all RETURN the same columns). We map via
// a normalized struct to avoid per-Row duplication.
type attendanceCols struct {
	ID                 string
	EmployeeID         string
	PlacementID        string
	ScheduleID         *string
	CompanyID          string
	SiteID             string
	Position           string
	AttendanceCodeID   *string
	ShiftStartAt       *time.Time
	ShiftEndAt         *time.Time
	CheckInAt          *time.Time
	CheckOutAt         *time.Time
	LatIn              *float64
	LngIn              *float64
	LatOut             *float64
	LngOut             *float64
	PhotoInID          *string
	PhotoOutID         *string
	Wfo                bool
	IsLate             bool
	LateMinutes        int32
	WorkedMinutes      *int32
	AutoClosed         bool
	InGeofence         *bool
	InDistanceM        *int32
	OutGeofence        *bool
	OutDistanceM       *int32
	GeofenceRadiusM    int32
	Status             string
	VerificationStatus string
	Flags              []string
	VerifiedBy         *string
	VerifiedAt         *time.Time
	RejectedBy         *string
	RejectedAt         *time.Time
	RejectReason       *string
	LastCorrectionID   *string
	CreatedBy          *string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	EmployeeName       *string // only present on list/get (LEFT JOIN); nil otherwise
	CompanyName        *string
	SiteName           *string
}

func mapAttendance(c attendanceCols) att.Attendance {
	a := att.Attendance{
		ID:                 c.ID,
		EmployeeID:         c.EmployeeID,
		PlacementID:        c.PlacementID,
		ScheduleID:         c.ScheduleID,
		CompanyID:          c.CompanyID,
		SiteID:             c.SiteID,
		Position:           c.Position,
		AttendanceCodeID:   c.AttendanceCodeID,
		ShiftStartAt:       c.ShiftStartAt,
		ShiftEndAt:         c.ShiftEndAt,
		CheckInAt:          c.CheckInAt,
		CheckOutAt:         c.CheckOutAt,
		LatIn:              c.LatIn,
		LngIn:              c.LngIn,
		LatOut:             c.LatOut,
		LngOut:             c.LngOut,
		PhotoInID:          c.PhotoInID,
		PhotoOutID:         c.PhotoOutID,
		WFO:                c.Wfo,
		IsLate:             c.IsLate,
		LateMinutes:        i32ToInt(c.LateMinutes),
		WorkedMinutes:      i32PtrToIntPtr(c.WorkedMinutes),
		AutoClosed:         c.AutoClosed,
		GeofenceIn:         geofenceFrom(c.InGeofence, c.InDistanceM, c.GeofenceRadiusM),
		GeofenceOut:        geofenceFrom(c.OutGeofence, c.OutDistanceM, c.GeofenceRadiusM),
		Status:             att.AttendanceStatus(c.Status),
		VerificationStatus: att.VerificationStatus(c.VerificationStatus),
		Flags:              flagsFrom(c.Flags),
		VerifiedBy:         c.VerifiedBy,
		VerifiedAt:         c.VerifiedAt,
		RejectedBy:         c.RejectedBy,
		RejectedAt:         c.RejectedAt,
		RejectReason:       c.RejectReason,
		LastCorrectionID:   c.LastCorrectionID,
		CreatedBy:          c.CreatedBy,
		CreatedAt:          c.CreatedAt,
		UpdatedAt:          c.UpdatedAt,
		EmployeeName:       c.EmployeeName,
		CompanyName:        c.CompanyName,
		SiteName:           c.SiteName,
	}
	return a
}

func mapAttendanceFromList(r sqlcgen.ListAttendanceRow) att.Attendance {
	return mapAttendance(attendanceCols{
		ID: r.ID, EmployeeID: r.EmployeeID, PlacementID: r.PlacementID, ScheduleID: r.ScheduleID,
		CompanyID: r.CompanyID, SiteID: r.SiteID, Position: r.Position, AttendanceCodeID: r.AttendanceCodeID,
		ShiftStartAt: r.ShiftStartAt, ShiftEndAt: r.ShiftEndAt, CheckInAt: r.CheckInAt, CheckOutAt: r.CheckOutAt,
		LatIn: r.LatIn, LngIn: r.LngIn, LatOut: r.LatOut, LngOut: r.LngOut, PhotoInID: r.PhotoInID, PhotoOutID: r.PhotoOutID,
		Wfo: r.Wfo, IsLate: r.IsLate, LateMinutes: r.LateMinutes, WorkedMinutes: r.WorkedMinutes, AutoClosed: r.AutoClosed,
		InGeofence: r.InGeofence, InDistanceM: r.InDistanceM, OutGeofence: r.OutGeofence, OutDistanceM: r.OutDistanceM, GeofenceRadiusM: r.GeofenceRadiusM,
		Status: r.Status, VerificationStatus: r.VerificationStatus, Flags: r.Flags,
		VerifiedBy: r.VerifiedBy, VerifiedAt: r.VerifiedAt, RejectedBy: r.RejectedBy, RejectedAt: r.RejectedAt,
		RejectReason: r.RejectReason, LastCorrectionID: r.LastCorrectionID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		EmployeeName: r.EmployeeName, CompanyName: r.CompanyName, SiteName: r.SiteName,
	})
}

func mapAttendanceFromGet(r sqlcgen.GetAttendanceRow) att.Attendance {
	return mapAttendance(attendanceCols{
		ID: r.ID, EmployeeID: r.EmployeeID, PlacementID: r.PlacementID, ScheduleID: r.ScheduleID,
		CompanyID: r.CompanyID, SiteID: r.SiteID, Position: r.Position, AttendanceCodeID: r.AttendanceCodeID,
		ShiftStartAt: r.ShiftStartAt, ShiftEndAt: r.ShiftEndAt, CheckInAt: r.CheckInAt, CheckOutAt: r.CheckOutAt,
		LatIn: r.LatIn, LngIn: r.LngIn, LatOut: r.LatOut, LngOut: r.LngOut, PhotoInID: r.PhotoInID, PhotoOutID: r.PhotoOutID,
		Wfo: r.Wfo, IsLate: r.IsLate, LateMinutes: r.LateMinutes, WorkedMinutes: r.WorkedMinutes, AutoClosed: r.AutoClosed,
		InGeofence: r.InGeofence, InDistanceM: r.InDistanceM, OutGeofence: r.OutGeofence, OutDistanceM: r.OutDistanceM, GeofenceRadiusM: r.GeofenceRadiusM,
		Status: r.Status, VerificationStatus: r.VerificationStatus, Flags: r.Flags,
		VerifiedBy: r.VerifiedBy, VerifiedAt: r.VerifiedAt, RejectedBy: r.RejectedBy, RejectedAt: r.RejectedAt,
		RejectReason: r.RejectReason, LastCorrectionID: r.LastCorrectionID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		EmployeeName: r.EmployeeName, CompanyName: r.CompanyName, SiteName: r.SiteName,
	})
}

func mapAttendanceFromForUpdate(r sqlcgen.GetAttendanceForUpdateRow) att.Attendance {
	return mapAttendance(attendanceCols{
		ID: r.ID, EmployeeID: r.EmployeeID, PlacementID: r.PlacementID, ScheduleID: r.ScheduleID,
		CompanyID: r.CompanyID, SiteID: r.SiteID, Position: r.Position, AttendanceCodeID: r.AttendanceCodeID,
		ShiftStartAt: r.ShiftStartAt, ShiftEndAt: r.ShiftEndAt, CheckInAt: r.CheckInAt, CheckOutAt: r.CheckOutAt,
		LatIn: r.LatIn, LngIn: r.LngIn, LatOut: r.LatOut, LngOut: r.LngOut, PhotoInID: r.PhotoInID, PhotoOutID: r.PhotoOutID,
		Wfo: r.Wfo, IsLate: r.IsLate, LateMinutes: r.LateMinutes, WorkedMinutes: r.WorkedMinutes, AutoClosed: r.AutoClosed,
		InGeofence: r.InGeofence, InDistanceM: r.InDistanceM, OutGeofence: r.OutGeofence, OutDistanceM: r.OutDistanceM, GeofenceRadiusM: r.GeofenceRadiusM,
		Status: r.Status, VerificationStatus: r.VerificationStatus, Flags: r.Flags,
		VerifiedBy: r.VerifiedBy, VerifiedAt: r.VerifiedAt, RejectedBy: r.RejectedBy, RejectedAt: r.RejectedAt,
		RejectReason: r.RejectReason, LastCorrectionID: r.LastCorrectionID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	})
}

func mapAttendanceFromVerify(r sqlcgen.VerifyAttendanceRow) att.Attendance {
	return mapAttendance(attendanceCols{
		ID: r.ID, EmployeeID: r.EmployeeID, PlacementID: r.PlacementID, ScheduleID: r.ScheduleID,
		CompanyID: r.CompanyID, SiteID: r.SiteID, Position: r.Position, AttendanceCodeID: r.AttendanceCodeID,
		ShiftStartAt: r.ShiftStartAt, ShiftEndAt: r.ShiftEndAt, CheckInAt: r.CheckInAt, CheckOutAt: r.CheckOutAt,
		LatIn: r.LatIn, LngIn: r.LngIn, LatOut: r.LatOut, LngOut: r.LngOut, PhotoInID: r.PhotoInID, PhotoOutID: r.PhotoOutID,
		Wfo: r.Wfo, IsLate: r.IsLate, LateMinutes: r.LateMinutes, WorkedMinutes: r.WorkedMinutes, AutoClosed: r.AutoClosed,
		InGeofence: r.InGeofence, InDistanceM: r.InDistanceM, OutGeofence: r.OutGeofence, OutDistanceM: r.OutDistanceM, GeofenceRadiusM: r.GeofenceRadiusM,
		Status: r.Status, VerificationStatus: r.VerificationStatus, Flags: r.Flags,
		VerifiedBy: r.VerifiedBy, VerifiedAt: r.VerifiedAt, RejectedBy: r.RejectedBy, RejectedAt: r.RejectedAt,
		RejectReason: r.RejectReason, LastCorrectionID: r.LastCorrectionID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	})
}

func mapAttendanceFromReject(r sqlcgen.RejectAttendanceRow) att.Attendance {
	return mapAttendance(attendanceCols{
		ID: r.ID, EmployeeID: r.EmployeeID, PlacementID: r.PlacementID, ScheduleID: r.ScheduleID,
		CompanyID: r.CompanyID, SiteID: r.SiteID, Position: r.Position, AttendanceCodeID: r.AttendanceCodeID,
		ShiftStartAt: r.ShiftStartAt, ShiftEndAt: r.ShiftEndAt, CheckInAt: r.CheckInAt, CheckOutAt: r.CheckOutAt,
		LatIn: r.LatIn, LngIn: r.LngIn, LatOut: r.LatOut, LngOut: r.LngOut, PhotoInID: r.PhotoInID, PhotoOutID: r.PhotoOutID,
		Wfo: r.Wfo, IsLate: r.IsLate, LateMinutes: r.LateMinutes, WorkedMinutes: r.WorkedMinutes, AutoClosed: r.AutoClosed,
		InGeofence: r.InGeofence, InDistanceM: r.InDistanceM, OutGeofence: r.OutGeofence, OutDistanceM: r.OutDistanceM, GeofenceRadiusM: r.GeofenceRadiusM,
		Status: r.Status, VerificationStatus: r.VerificationStatus, Flags: r.Flags,
		VerifiedBy: r.VerifiedBy, VerifiedAt: r.VerifiedAt, RejectedBy: r.RejectedBy, RejectedAt: r.RejectedAt,
		RejectReason: r.RejectReason, LastCorrectionID: r.LastCorrectionID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	})
}

func mapAttendanceFromApply(r sqlcgen.ApplyCorrectionToAttendanceRow) att.Attendance {
	return mapAttendance(attendanceCols{
		ID: r.ID, EmployeeID: r.EmployeeID, PlacementID: r.PlacementID, ScheduleID: r.ScheduleID,
		CompanyID: r.CompanyID, SiteID: r.SiteID, Position: r.Position, AttendanceCodeID: r.AttendanceCodeID,
		ShiftStartAt: r.ShiftStartAt, ShiftEndAt: r.ShiftEndAt, CheckInAt: r.CheckInAt, CheckOutAt: r.CheckOutAt,
		LatIn: r.LatIn, LngIn: r.LngIn, LatOut: r.LatOut, LngOut: r.LngOut, PhotoInID: r.PhotoInID, PhotoOutID: r.PhotoOutID,
		Wfo: r.Wfo, IsLate: r.IsLate, LateMinutes: r.LateMinutes, WorkedMinutes: r.WorkedMinutes, AutoClosed: r.AutoClosed,
		InGeofence: r.InGeofence, InDistanceM: r.InDistanceM, OutGeofence: r.OutGeofence, OutDistanceM: r.OutDistanceM, GeofenceRadiusM: r.GeofenceRadiusM,
		Status: r.Status, VerificationStatus: r.VerificationStatus, Flags: r.Flags,
		VerifiedBy: r.VerifiedBy, VerifiedAt: r.VerifiedAt, RejectedBy: r.RejectedBy, RejectedAt: r.RejectedAt,
		RejectReason: r.RejectReason, LastCorrectionID: r.LastCorrectionID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	})
}

// --- correction row shape ---

type correctionCols struct {
	ID                       string
	AttendanceID             string
	RequesterID              string
	CompanyID                string
	Type                     string
	ProposedCheckInAt        *time.Time
	ProposedCheckOutAt       *time.Time
	ProposedAttendanceCodeID *string
	Reason                   string
	EvidenceFileID           *string
	Status                   string
	DecidedBy                *string
	DecidedAt                *time.Time
	RejectReason             *string
	OriginalSnapshot         []byte
	AttendanceShiftDate      pgtype.Date
	CreatedAt                time.Time
	UpdatedAt                time.Time
	RequesterName            *string
	CompanyName              *string
}



func mapAttendanceFromCreate(r sqlcgen.CreateManualAttendanceRow) att.Attendance {
	return mapAttendance(attendanceCols{
		ID: r.ID, EmployeeID: r.EmployeeID, PlacementID: r.PlacementID, ScheduleID: r.ScheduleID,
		CompanyID: r.CompanyID, SiteID: r.SiteID, Position: r.Position, AttendanceCodeID: r.AttendanceCodeID,
		ShiftStartAt: r.ShiftStartAt, ShiftEndAt: r.ShiftEndAt, CheckInAt: r.CheckInAt, CheckOutAt: r.CheckOutAt,
		LatIn: r.LatIn, LngIn: r.LngIn, LatOut: r.LatOut, LngOut: r.LngOut, PhotoInID: r.PhotoInID, PhotoOutID: r.PhotoOutID,
		Wfo: r.Wfo, IsLate: r.IsLate, LateMinutes: r.LateMinutes, WorkedMinutes: r.WorkedMinutes, AutoClosed: r.AutoClosed,
		InGeofence: r.InGeofence, InDistanceM: r.InDistanceM, OutGeofence: r.OutGeofence, OutDistanceM: r.OutDistanceM, GeofenceRadiusM: r.GeofenceRadiusM,
		Status: r.Status, VerificationStatus: r.VerificationStatus, Flags: r.Flags,
		VerifiedBy: r.VerifiedBy, VerifiedAt: r.VerifiedAt, RejectedBy: r.RejectedBy, RejectedAt: r.RejectedAt,
		RejectReason: r.RejectReason, LastCorrectionID: r.LastCorrectionID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	})
}

func mapCorrection(c correctionCols) att.Correction {
	return att.Correction{
		ID:                       c.ID,
		AttendanceID:             c.AttendanceID,
		RequesterID:              c.RequesterID,
		CompanyID:                c.CompanyID,
		Type:                     att.CorrectionType(c.Type),
		ProposedCheckInAt:        c.ProposedCheckInAt,
		ProposedCheckOutAt:       c.ProposedCheckOutAt,
		ProposedAttendanceCodeID: c.ProposedAttendanceCodeID,
		Reason:                   c.Reason,
		EvidenceFileID:           c.EvidenceFileID,
		Status:                   att.CorrectionStatus(c.Status),
		DecidedBy:                c.DecidedBy,
		DecidedAt:                c.DecidedAt,
		RejectReason:             c.RejectReason,
		OriginalSnapshot:         snapshotFrom(c.OriginalSnapshot),
		AttendanceShiftDate:      pgDateToTime(c.AttendanceShiftDate),
		CreatedAt:                c.CreatedAt,
		UpdatedAt:                c.UpdatedAt,
		RequesterName:            c.RequesterName,
		CompanyName:              c.CompanyName,
	}
}

func mapCorrectionFromList(r sqlcgen.ListCorrectionsRow) att.Correction {
	return mapCorrection(correctionCols{
		ID: r.ID, AttendanceID: r.AttendanceID, RequesterID: r.RequesterID, CompanyID: r.CompanyID,
		Type: r.Type, ProposedCheckInAt: r.ProposedCheckInAt, ProposedCheckOutAt: r.ProposedCheckOutAt,
		ProposedAttendanceCodeID: r.ProposedAttendanceCodeID, Reason: r.Reason, EvidenceFileID: r.EvidenceFileID,
		Status: r.Status, DecidedBy: r.DecidedBy, DecidedAt: r.DecidedAt, RejectReason: r.RejectReason,
		OriginalSnapshot: r.OriginalSnapshot, AttendanceShiftDate: r.AttendanceShiftDate,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, RequesterName: r.RequesterName, CompanyName: r.CompanyName,
	})
}

func mapCorrectionFromGet(r sqlcgen.GetCorrectionRow) att.Correction {
	return mapCorrection(correctionCols{
		ID: r.ID, AttendanceID: r.AttendanceID, RequesterID: r.RequesterID, CompanyID: r.CompanyID,
		Type: r.Type, ProposedCheckInAt: r.ProposedCheckInAt, ProposedCheckOutAt: r.ProposedCheckOutAt,
		ProposedAttendanceCodeID: r.ProposedAttendanceCodeID, Reason: r.Reason, EvidenceFileID: r.EvidenceFileID,
		Status: r.Status, DecidedBy: r.DecidedBy, DecidedAt: r.DecidedAt, RejectReason: r.RejectReason,
		OriginalSnapshot: r.OriginalSnapshot, AttendanceShiftDate: r.AttendanceShiftDate,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, RequesterName: r.RequesterName, CompanyName: r.CompanyName,
	})
}

func mapCorrectionFromForUpdate(r sqlcgen.GetCorrectionForUpdateRow) att.Correction {
	return mapCorrection(correctionCols{
		ID: r.ID, AttendanceID: r.AttendanceID, RequesterID: r.RequesterID, CompanyID: r.CompanyID,
		Type: r.Type, ProposedCheckInAt: r.ProposedCheckInAt, ProposedCheckOutAt: r.ProposedCheckOutAt,
		ProposedAttendanceCodeID: r.ProposedAttendanceCodeID, Reason: r.Reason, EvidenceFileID: r.EvidenceFileID,
		Status: r.Status, DecidedBy: r.DecidedBy, DecidedAt: r.DecidedAt, RejectReason: r.RejectReason,
		OriginalSnapshot: r.OriginalSnapshot, AttendanceShiftDate: r.AttendanceShiftDate,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	})
}

func mapCorrectionFromApprove(r sqlcgen.ApproveCorrectionRow) att.Correction {
	return mapCorrection(correctionCols{
		ID: r.ID, AttendanceID: r.AttendanceID, RequesterID: r.RequesterID, CompanyID: r.CompanyID,
		Type: r.Type, ProposedCheckInAt: r.ProposedCheckInAt, ProposedCheckOutAt: r.ProposedCheckOutAt,
		ProposedAttendanceCodeID: r.ProposedAttendanceCodeID, Reason: r.Reason, EvidenceFileID: r.EvidenceFileID,
		Status: r.Status, DecidedBy: r.DecidedBy, DecidedAt: r.DecidedAt, RejectReason: r.RejectReason,
		OriginalSnapshot: r.OriginalSnapshot, AttendanceShiftDate: r.AttendanceShiftDate,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	})
}

func mapCorrectionFromReject(r sqlcgen.RejectCorrectionRow) att.Correction {
	return mapCorrection(correctionCols{
		ID: r.ID, AttendanceID: r.AttendanceID, RequesterID: r.RequesterID, CompanyID: r.CompanyID,
		Type: r.Type, ProposedCheckInAt: r.ProposedCheckInAt, ProposedCheckOutAt: r.ProposedCheckOutAt,
		ProposedAttendanceCodeID: r.ProposedAttendanceCodeID, Reason: r.Reason, EvidenceFileID: r.EvidenceFileID,
		Status: r.Status, DecidedBy: r.DecidedBy, DecidedAt: r.DecidedAt, RejectReason: r.RejectReason,
		OriginalSnapshot: r.OriginalSnapshot, AttendanceShiftDate: r.AttendanceShiftDate,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	})
}
