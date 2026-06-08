// Package attendance — E5 agent clock-in/out (F5.1 / SWP-ATT-*). The MOBILE surface
// (agent, scope:self): an agent clocks in (GPS + geofence + lateness eval, creates a
// PENDING/AUTO_APPROVED record) and clocks out (worked_minutes + early eval, closes
// the open record). Distinct from attendance_service.go which is the WEB
// verify/reject/corrections surface.
//
// Decisions (openapi docs/api/E5-attendance):
//   - GPS_UNAVAILABLE (422) when gps_available=false — required true to clock.
//   - NO_ACTIVE_PLACEMENT (422) when the agent has no active placement (can't resolve
//     site/company/service_line). [code not enumerated in the cross-cutting set; chosen
//     422 RULE per CONVENTIONS — the request is well-formed but unsatisfiable.]
//   - OUT_OF_GEOFENCE (422, fields distance_m/radius_m/company_id) on clock-IN only,
//     UNLESS force_outside_geofence=true → persist with OUTSIDE_GEOFENCE flag. Clock-OUT
//     never blocks (leaving the site is normal) — just flags.
//   - ALREADY_CLOCKED_IN (409, field open_attendance_id) when an open record exists;
//     NOT_CLOCKED_IN (409) on clock-out with no open record.
//   - Lateness: within 15-min grace ⇒ late_minutes=0 / PRESENT; strictly after ⇒
//     LATE flag + status LATE. EARLY (clock-out > 15 min before shift end) ⇒ flag.
//   - verification: any flag ⇒ PENDING (enters the leader queue); none ⇒ AUTO_APPROVED
//     (clock-in) / VERIFIED (clock-out, INV-3 exceptions-only).
package attendance

import (
	"context"
	"math"
	"time"

	"github.com/jackc/pgx/v5"

	att "github.com/hariszaki17/hris-outsource/backend/internal/domain/attendance"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

// defaultGrace is the lateness / early-leave tolerance (EPICS §8: 15 min).
const defaultGrace = 15 * time.Minute

// earthRadiusM is the mean Earth radius used by the haversine geofence distance.
const earthRadiusM = 6371000.0

// --- ports + params ---

// PlacementInfo is the active-placement projection the clock service needs to stamp
// the denormalized company/site/position/service_line columns on a new record.
type PlacementInfo struct {
	PlacementID string
	CompanyID   string
	SiteID      string
	PositionID  string
	ServiceLine string // attendance text enum (facility_services|building_management|parking)
}

// ClockInParams is the decoded clock-in body (agent is always self; employee_id from
// the request is ignored).
type ClockInParams struct {
	Lat                  float64
	Lng                  float64
	GPSAvailable         bool
	WFO                  bool
	PhotoID              *string
	ForceOutsideGeofence bool
}

// ClockOutParams is the decoded clock-out body.
type ClockOutParams struct {
	Lat          float64
	Lng          float64
	GPSAvailable bool
	PhotoID      *string
}

// ClockInRow is the INSERT payload for one clock-in record (one-shot, in-tx).
type ClockInRow struct {
	EmployeeID         string
	PlacementID        string
	ScheduleID         *string
	CompanyID          string
	ServiceLine        string
	SiteID             string
	PositionID         string
	ShiftStartAt       *time.Time
	ShiftEndAt         *time.Time
	CheckInAt          time.Time
	LatIn              float64
	LngIn              float64
	PhotoInID          *string
	WFO                bool
	IsLate             bool
	LateMinutes        int
	InGeofence         *bool
	InDistanceM        *int
	GeofenceRadiusM    int
	Status             string
	VerificationStatus string
	Flags              []string
}

// ClockOutRow is the UPDATE payload for closing one open record (in-tx).
type ClockOutRow struct {
	ID                 string
	CheckOutAt         time.Time
	LatOut             float64
	LngOut             float64
	PhotoOutID         *string
	OutGeofence        *bool
	OutDistanceM       *int
	WorkedMinutes      int
	Flags              []string
	Status             string
	VerificationStatus string
}

// ClockRepository is the data dependency for the clock service. ClockIn returns
// created=false (no error) when ON CONFLICT DO NOTHING no-ops (the schedule already
// has a row) — the service maps that to ALREADY_CLOCKED_IN.
type ClockRepository interface {
	GetActivePlacement(ctx context.Context, employeeID string) (PlacementInfo, bool, error)
	GetSite(ctx context.Context, siteID string) (lat, lng *float64, radiusM int, found bool, err error)
	GetTodaySchedule(ctx context.Context, employeeID string, now time.Time) (scheduleID string, start, end time.Time, found bool, err error)
	GetOpenAttendance(ctx context.Context, employeeID string) (id string, found bool, err error)
	ClockIn(ctx context.Context, tx pgx.Tx, p ClockInRow) (id string, created bool, err error)
	ClockOut(ctx context.Context, tx pgx.Tx, p ClockOutRow) (id string, err error)
	GetAttendance(ctx context.Context, id string) (att.Attendance, error)
}

// ClockService implements the agent clock-in/out business logic.
type ClockService struct {
	repo  ClockRepository
	txm   TxRunner
	now   Clock
	grace time.Duration
}

// NewClockService wires the clock service with the 15-min default grace.
func NewClockService(repo ClockRepository, txm TxRunner) *ClockService {
	return &ClockService{repo: repo, txm: txm, now: time.Now, grace: defaultGrace}
}

// SetClock overrides the time source (tests only).
func (s *ClockService) SetClock(c Clock) { s.now = c }

// --- clock-in ---

// ClockIn creates the caller's attendance record (GPS + geofence + lateness).
func (s *ClockService) ClockIn(ctx context.Context, req ClockInParams) (att.Attendance, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return att.Attendance{}, apperr.Unauthenticated()
	}
	employeeID := p.EmployeeID
	if employeeID == "" {
		return att.Attendance{}, apperr.OutOfScope()
	}
	if !req.GPSAvailable {
		return att.Attendance{}, apperr.Rule("GPS_UNAVAILABLE", nil)
	}

	pl, found, err := s.repo.GetActivePlacement(ctx, employeeID)
	if err != nil {
		return att.Attendance{}, apperr.Internal(err)
	}
	if !found {
		return att.Attendance{}, apperr.Rule("NO_ACTIVE_PLACEMENT", nil)
	}

	siteLat, siteLng, radiusM, _, err := s.repo.GetSite(ctx, pl.SiteID)
	if err != nil {
		return att.Attendance{}, apperr.Internal(err)
	}
	inside, distanceM, haveGeo := evalGeofence(req.Lat, req.Lng, siteLat, siteLng, radiusM)

	// Out-of-geofence blocks clock-in unless the agent explicitly overrides.
	if haveGeo && !inside && !req.ForceOutsideGeofence {
		return att.Attendance{}, apperr.Rule("OUT_OF_GEOFENCE", map[string]string{
			"distance_m": itoa(distanceM),
			"radius_m":   itoa(radiusM),
			"company_id": pl.CompanyID,
		})
	}

	// Reject a second open record before doing more work (CI-5).
	if openID, openFound, oerr := s.repo.GetOpenAttendance(ctx, employeeID); oerr != nil {
		return att.Attendance{}, apperr.Internal(oerr)
	} else if openFound {
		return att.Attendance{}, alreadyClockedIn(openID)
	}

	now := s.now()

	// Today's schedule (lateness eval + schedule_id). Absent ⇒ unscheduled.
	schedID, shiftStart, shiftEnd, schedFound, serr := s.repo.GetTodaySchedule(ctx, employeeID, now)
	if serr != nil {
		return att.Attendance{}, apperr.Internal(serr)
	}

	var (
		flags         []string
		isLate        bool
		lateMinutes   int
		status        = string(att.StatusPresent)
		schedulePtr   *string
		shiftStartPtr *time.Time
		shiftEndPtr   *time.Time
	)
	if schedFound {
		ss := shiftStart
		se := shiftEnd
		schedulePtr = &schedID
		shiftStartPtr = &ss
		shiftEndPtr = &se
		if mins := int(now.Sub(shiftStart).Minutes()); mins > int(s.grace.Minutes()) {
			isLate = true
			lateMinutes = mins
			flags = append(flags, string(att.FlagLate))
			status = string(att.StatusLate)
		}
	} else {
		flags = append(flags, string(att.FlagUnscheduled))
	}
	if haveGeo && !inside {
		flags = append(flags, string(att.FlagOutsideGeofence))
	}

	verification := string(att.VerificationAutoApproved)
	if len(flags) > 0 {
		verification = string(att.VerificationPending)
	}

	var inGeofencePtr *bool
	var inDistancePtr *int
	if haveGeo {
		ig := inside
		dm := distanceM
		inGeofencePtr = &ig
		inDistancePtr = &dm
	}

	row := ClockInRow{
		EmployeeID:         employeeID,
		PlacementID:        pl.PlacementID,
		ScheduleID:         schedulePtr,
		CompanyID:          pl.CompanyID,
		ServiceLine:        pl.ServiceLine,
		SiteID:             pl.SiteID,
		PositionID:         pl.PositionID,
		ShiftStartAt:       shiftStartPtr,
		ShiftEndAt:         shiftEndPtr,
		CheckInAt:          now,
		LatIn:              req.Lat,
		LngIn:              req.Lng,
		PhotoInID:          req.PhotoID,
		WFO:                req.WFO,
		IsLate:             isLate,
		LateMinutes:        lateMinutes,
		InGeofence:         inGeofencePtr,
		InDistanceM:        inDistancePtr,
		GeofenceRadiusM:    radiusM,
		Status:             status,
		VerificationStatus: verification,
		Flags:              flags,
	}

	var newID string
	txErr := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		id, created, ierr := s.repo.ClockIn(ctx, tx, row)
		if ierr != nil {
			return ierr
		}
		if !created {
			// ON CONFLICT no-op: a row already exists for this schedule (a concurrent
			// clock-in / absence-sweep won the race) → ALREADY_CLOCKED_IN.
			return alreadyClockedIn("")
		}
		newID = id
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionCreate,
			EntityType: "attendance",
			EntityID:   id,
			Before:     nil,
			After: map[string]any{
				"employee_id":         employeeID,
				"schedule_id":         ptrStr(schedulePtr),
				"status":              status,
				"verification_status": verification,
				"flags":               flags,
				"source":              "clock_in",
			},
		})
	})
	if txErr != nil {
		return att.Attendance{}, asAppErr(txErr)
	}
	return s.rereadClock(ctx, newID)
}

// --- clock-out ---

// ClockOut closes the caller's open record (worked_minutes + early eval). Out-of-
// geofence on clock-out is flagged, never blocked.
func (s *ClockService) ClockOut(ctx context.Context, req ClockOutParams) (att.Attendance, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return att.Attendance{}, apperr.Unauthenticated()
	}
	employeeID := p.EmployeeID
	if employeeID == "" {
		return att.Attendance{}, apperr.OutOfScope()
	}
	if !req.GPSAvailable {
		return att.Attendance{}, apperr.Rule("GPS_UNAVAILABLE", nil)
	}

	openID, found, err := s.repo.GetOpenAttendance(ctx, employeeID)
	if err != nil {
		return att.Attendance{}, apperr.Internal(err)
	}
	if !found {
		return att.Attendance{}, apperr.Conflict("NOT_CLOCKED_IN")
	}

	rec, err := s.repo.GetAttendance(ctx, openID)
	if err != nil {
		return att.Attendance{}, apperr.Internal(err)
	}

	now := s.now()

	// Geofence-out (advisory only — never blocks). Reuse the stored radius.
	radiusM := 0
	if rec.GeofenceIn != nil {
		radiusM = rec.GeofenceIn.RadiusM
	}
	siteLat, siteLng, siteRadius, siteFound, gerr := s.repo.GetSite(ctx, rec.SiteID)
	if gerr != nil {
		return att.Attendance{}, apperr.Internal(gerr)
	}
	if siteFound && siteRadius > 0 {
		radiusM = siteRadius
	}
	outInside, outDistance, haveGeo := evalGeofence(req.Lat, req.Lng, siteLat, siteLng, radiusM)

	// Start from the existing flags (preserve LATE etc.), then add EARLY / OUTSIDE.
	flags := flagStrings(rec.Flags)
	if haveGeo && !outInside {
		flags = appendUnique(flags, string(att.FlagOutsideGeofence))
	}
	if rec.ShiftEndAt != nil && now.Before(rec.ShiftEndAt.Add(-s.grace)) {
		flags = appendUnique(flags, string(att.FlagEarly))
	}

	workedMinutes := 0
	if rec.CheckInAt != nil {
		workedMinutes = int(now.Sub(*rec.CheckInAt).Minutes())
	}
	if workedMinutes < 0 {
		workedMinutes = 0
	}

	// status: keep LATE if the record was late on clock-in, else PRESENT.
	status := string(att.StatusPresent)
	if rec.IsLate {
		status = string(att.StatusLate)
	}

	// verification: no flags ⇒ VERIFIED (auto-approved); any flag ⇒ keep the existing
	// queue state (PENDING) if already pending, else PENDING (a new EARLY/OUTSIDE flag
	// escalates an auto-approved record into the queue).
	verification := string(att.VerificationVerified)
	if len(flags) > 0 {
		if rec.VerificationStatus == att.VerificationPending || rec.VerificationStatus == att.VerificationEscalated {
			verification = string(rec.VerificationStatus)
		} else {
			verification = string(att.VerificationPending)
		}
	}

	var outGeofencePtr *bool
	var outDistancePtr *int
	if haveGeo {
		oi := outInside
		od := outDistance
		outGeofencePtr = &oi
		outDistancePtr = &od
	}

	row := ClockOutRow{
		ID:                 openID,
		CheckOutAt:         now,
		LatOut:             req.Lat,
		LngOut:             req.Lng,
		PhotoOutID:         req.PhotoID,
		OutGeofence:        outGeofencePtr,
		OutDistanceM:       outDistancePtr,
		WorkedMinutes:      workedMinutes,
		Flags:              flags,
		Status:             status,
		VerificationStatus: verification,
	}

	txErr := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		_, uerr := s.repo.ClockOut(ctx, tx, row)
		if uerr != nil {
			return uerr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionUpdate,
			EntityType: "attendance",
			EntityID:   openID,
			Before:     map[string]any{"check_out_at": nil, "verification_status": string(rec.VerificationStatus)},
			After: map[string]any{
				"check_out_at":        now.UTC(),
				"worked_minutes":      workedMinutes,
				"status":              status,
				"verification_status": verification,
				"flags":               flags,
				"source":              "clock_out",
			},
		})
	})
	if txErr != nil {
		return att.Attendance{}, asAppErr(txErr)
	}
	return s.rereadClock(ctx, openID)
}

// --- helpers ---

// rereadClock re-loads the record for the denormalized names + assembled geofence.
func (s *ClockService) rereadClock(ctx context.Context, id string) (att.Attendance, error) {
	rec, err := s.repo.GetAttendance(ctx, id)
	if err != nil {
		return att.Attendance{}, apperr.Internal(err)
	}
	return rec, nil
}

// alreadyClockedIn builds the 409 with the open record id (empty when the conflict was
// detected only at the ON CONFLICT no-op, where the id is not available).
func alreadyClockedIn(openID string) error {
	e := apperr.Conflict("ALREADY_CLOCKED_IN")
	if openID != "" {
		e.Fields = map[string]string{"open_attendance_id": openID}
	}
	return e
}

// evalGeofence computes inside / distance from the agent point to the site center.
// haveGeo=false (and inside=true) when the site has no coordinates — geofence is
// skipped, the clock-in proceeds without a geofence_in capture.
func evalGeofence(lat, lng float64, siteLat, siteLng *float64, radiusM int) (inside bool, distanceM int, haveGeo bool) {
	if siteLat == nil || siteLng == nil {
		return true, 0, false
	}
	d := haversine(lat, lng, *siteLat, *siteLng)
	return d <= radiusM, d, true
}

// haversine returns the great-circle distance in whole meters between two points.
func haversine(lat1, lng1, lat2, lng2 float64) int {
	rad := math.Pi / 180
	φ1 := lat1 * rad
	φ2 := lat2 * rad
	dφ := (lat2 - lat1) * rad
	dλ := (lng2 - lng1) * rad
	a := math.Sin(dφ/2)*math.Sin(dφ/2) +
		math.Cos(φ1)*math.Cos(φ2)*math.Sin(dλ/2)*math.Sin(dλ/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return int(math.Round(earthRadiusM * c))
}

// flagStrings copies the domain flags into a fresh []string (for in-place dedupe).
func flagStrings(in []att.Flag) []string {
	out := make([]string, 0, len(in))
	for _, f := range in {
		out = append(out, string(f))
	}
	return out
}

// appendUnique appends v only if not already present.
func appendUnique(in []string, v string) []string {
	for _, x := range in {
		if x == v {
			return in
		}
	}
	return append(in, v)
}

// itoa formats a non-negative int for the OUT_OF_GEOFENCE error fields (string map).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
