// Package overtime (repository) — implements the E7 overtime + holiday service
// ports over the 09-01 sqlc queries. Reads on the pool; locked re-checks + writes
// via q.WithTx(tx). Date columns convert pgtype.Date ↔ time.Time (Phase-5/6/8
// pattern); reference_multiplier converts pgtype.Numeric ↔ *float64; text[] maps
// natively to []string; pgx.ErrNoRows → domain.ErrNotFound. int32 ↔ int cast at
// the boundary.
package overtime

import (
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/overtime"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
)

// --- pg type helpers ---

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

func timeToPgDate(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: true}
}

func pgDateToTime(d pgtype.Date) time.Time {
	if !d.Valid {
		return time.Time{}
	}
	return d.Time
}

func i32(n int) int32 { return int32(n) }

// numericToFloatPtr converts a nullable pgtype.Numeric into a *float64 (nil when
// NULL). reference_multiplier is STORED reference only (INV-2) — never applied.
func numericToFloatPtr(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	v := f.Float64
	return &v
}

// strptr collapses an empty string pointer to nil for narg filters.
func strptr(p *string) *string {
	if p == nil || *p == "" {
		return nil
	}
	return p
}

// --- overtime mappers ---

func mapOvertimeFromList(r sqlcgen.ListOvertimeRow) dom.Overtime {
	return dom.Overtime{
		ID:                   r.ID,
		EmployeeID:           r.EmployeeID,
		EmployeeName:         r.EmployeeName,
		CompanyID:            r.CompanyID,
		CompanyName:          r.CompanyName,
		PlacementID:          r.PlacementID,
		AttendanceID:         r.AttendanceID,
		ServiceLineID:        r.ServiceLineID,
		WorkDate:             pgDateToTime(r.WorkDate),
		PlannedStartTime:     r.PlannedStartTime,
		PlannedEndTime:       r.PlannedEndTime,
		ActualStartTime:      r.ActualStartTime,
		ActualEndTime:        r.ActualEndTime,
		CrossMidnight:        r.CrossMidnight,
		Source:               dom.OvertimeSource(r.Source),
		Status:               dom.OvertimeStatus(r.Status),
		DayType:              dom.OvertimeTier(r.DayType),
		WorkedMinutes:        int(r.WorkedMinutes),
		CountedMinutes:       int(r.CountedMinutes),
		MinMinutesThreshold:  int(r.MinMinutesThreshold),
		SkippedTooShort:      r.SkippedTooShort,
		ReferenceMultiplier:  numericToFloatPtr(r.ReferenceMultiplier),
		OvertimeRuleID:       r.OvertimeRuleID,
		HolidayID:            r.HolidayID,
		FlaggedNoPreapproval: r.FlaggedNoPreapproval,
		Reason:               r.Reason,
		CreatedBy:            r.CreatedBy,
		CreatedAt:            r.CreatedAt,
		UpdatedAt:            r.UpdatedAt,
	}
}

func mapOvertimeFromGet(r sqlcgen.GetOvertimeRow) dom.Overtime {
	return dom.Overtime{
		ID:                   r.ID,
		EmployeeID:           r.EmployeeID,
		EmployeeName:         r.EmployeeName,
		CompanyID:            r.CompanyID,
		CompanyName:          r.CompanyName,
		PlacementID:          r.PlacementID,
		AttendanceID:         r.AttendanceID,
		ServiceLineID:        r.ServiceLineID,
		WorkDate:             pgDateToTime(r.WorkDate),
		PlannedStartTime:     r.PlannedStartTime,
		PlannedEndTime:       r.PlannedEndTime,
		ActualStartTime:      r.ActualStartTime,
		ActualEndTime:        r.ActualEndTime,
		CrossMidnight:        r.CrossMidnight,
		Source:               dom.OvertimeSource(r.Source),
		Status:               dom.OvertimeStatus(r.Status),
		DayType:              dom.OvertimeTier(r.DayType),
		WorkedMinutes:        int(r.WorkedMinutes),
		CountedMinutes:       int(r.CountedMinutes),
		MinMinutesThreshold:  int(r.MinMinutesThreshold),
		SkippedTooShort:      r.SkippedTooShort,
		ReferenceMultiplier:  numericToFloatPtr(r.ReferenceMultiplier),
		OvertimeRuleID:       r.OvertimeRuleID,
		HolidayID:            r.HolidayID,
		FlaggedNoPreapproval: r.FlaggedNoPreapproval,
		Reason:               r.Reason,
		CreatedBy:            r.CreatedBy,
		CreatedAt:            r.CreatedAt,
		UpdatedAt:            r.UpdatedAt,
	}
}

func mapOvertimeFromForUpdate(r sqlcgen.GetOvertimeForUpdateRow) dom.Overtime {
	return dom.Overtime{
		ID:                   r.ID,
		EmployeeID:           r.EmployeeID,
		CompanyID:            r.CompanyID,
		PlacementID:          r.PlacementID,
		AttendanceID:         r.AttendanceID,
		ServiceLineID:        r.ServiceLineID,
		WorkDate:             pgDateToTime(r.WorkDate),
		PlannedStartTime:     r.PlannedStartTime,
		PlannedEndTime:       r.PlannedEndTime,
		ActualStartTime:      r.ActualStartTime,
		ActualEndTime:        r.ActualEndTime,
		CrossMidnight:        r.CrossMidnight,
		Source:               dom.OvertimeSource(r.Source),
		Status:               dom.OvertimeStatus(r.Status),
		DayType:              dom.OvertimeTier(r.DayType),
		WorkedMinutes:        int(r.WorkedMinutes),
		CountedMinutes:       int(r.CountedMinutes),
		MinMinutesThreshold:  int(r.MinMinutesThreshold),
		SkippedTooShort:      r.SkippedTooShort,
		ReferenceMultiplier:  numericToFloatPtr(r.ReferenceMultiplier),
		OvertimeRuleID:       r.OvertimeRuleID,
		HolidayID:            r.HolidayID,
		FlaggedNoPreapproval: r.FlaggedNoPreapproval,
		Reason:               r.Reason,
		CreatedBy:            r.CreatedBy,
		CreatedAt:            r.CreatedAt,
		UpdatedAt:            r.UpdatedAt,
	}
}

func mapOvertimeFromUpdate(r sqlcgen.UpdateOvertimeStatusRow) dom.Overtime {
	return dom.Overtime{
		ID:                   r.ID,
		EmployeeID:           r.EmployeeID,
		CompanyID:            r.CompanyID,
		PlacementID:          r.PlacementID,
		AttendanceID:         r.AttendanceID,
		ServiceLineID:        r.ServiceLineID,
		WorkDate:             pgDateToTime(r.WorkDate),
		PlannedStartTime:     r.PlannedStartTime,
		PlannedEndTime:       r.PlannedEndTime,
		ActualStartTime:      r.ActualStartTime,
		ActualEndTime:        r.ActualEndTime,
		CrossMidnight:        r.CrossMidnight,
		Source:               dom.OvertimeSource(r.Source),
		Status:               dom.OvertimeStatus(r.Status),
		DayType:              dom.OvertimeTier(r.DayType),
		WorkedMinutes:        int(r.WorkedMinutes),
		CountedMinutes:       int(r.CountedMinutes),
		MinMinutesThreshold:  int(r.MinMinutesThreshold),
		SkippedTooShort:      r.SkippedTooShort,
		ReferenceMultiplier:  numericToFloatPtr(r.ReferenceMultiplier),
		OvertimeRuleID:       r.OvertimeRuleID,
		HolidayID:            r.HolidayID,
		FlaggedNoPreapproval: r.FlaggedNoPreapproval,
		Reason:               r.Reason,
		CreatedBy:            r.CreatedBy,
		CreatedAt:            r.CreatedAt,
		UpdatedAt:            r.UpdatedAt,
	}
}

func mapApproval(r sqlcgen.OvertimeApproval) dom.OvertimeApproval {
	return dom.OvertimeApproval{
		Level:        int(r.Level),
		Decision:     r.Decision,
		ApproverID:   r.ApproverID,
		ApproverName: r.ApproverName,
		Reason:       r.Reason,
		DecidedAt:    r.DecidedAt,
	}
}

// --- holiday mappers ---

func mapHolidayFromList(r sqlcgen.ListHolidaysRow) dom.Holiday {
	return dom.Holiday{
		ID:                     r.ID,
		Name:                   r.Name,
		Date:                   pgDateToTime(r.HolidayDate),
		Category:               dom.HolidayCategory(r.Category),
		Recurring:              r.Recurring,
		ApplicableServiceLines: emptyIfNil(r.ApplicableServiceLines),
		CreatedAt:              r.CreatedAt,
		UpdatedAt:              r.UpdatedAt,
	}
}

func mapHolidayFromGet(r sqlcgen.GetHolidayRow) dom.Holiday {
	return dom.Holiday{
		ID:                     r.ID,
		Name:                   r.Name,
		Date:                   pgDateToTime(r.HolidayDate),
		Category:               dom.HolidayCategory(r.Category),
		Recurring:              r.Recurring,
		ApplicableServiceLines: emptyIfNil(r.ApplicableServiceLines),
		CreatedAt:              r.CreatedAt,
		UpdatedAt:              r.UpdatedAt,
	}
}

func mapHolidayFromByDateCategory(r sqlcgen.GetHolidayByDateCategoryRow) dom.Holiday {
	return dom.Holiday{
		ID:                     r.ID,
		Name:                   r.Name,
		Date:                   pgDateToTime(r.HolidayDate),
		Category:               dom.HolidayCategory(r.Category),
		Recurring:              r.Recurring,
		ApplicableServiceLines: emptyIfNil(r.ApplicableServiceLines),
		CreatedAt:              r.CreatedAt,
		UpdatedAt:              r.UpdatedAt,
	}
}

func mapHolidayFromForDate(r sqlcgen.GetHolidayForDateRow) dom.Holiday {
	return dom.Holiday{
		ID:                     r.ID,
		Name:                   r.Name,
		Date:                   pgDateToTime(r.HolidayDate),
		Category:               dom.HolidayCategory(r.Category),
		Recurring:              r.Recurring,
		ApplicableServiceLines: emptyIfNil(r.ApplicableServiceLines),
		CreatedAt:              r.CreatedAt,
		UpdatedAt:              r.UpdatedAt,
	}
}

func mapHolidayFromInsert(r sqlcgen.InsertHolidayRow) dom.Holiday {
	return dom.Holiday{
		ID:                     r.ID,
		Name:                   r.Name,
		Date:                   pgDateToTime(r.HolidayDate),
		Category:               dom.HolidayCategory(r.Category),
		Recurring:              r.Recurring,
		ApplicableServiceLines: emptyIfNil(r.ApplicableServiceLines),
		CreatedAt:              r.CreatedAt,
		UpdatedAt:              r.UpdatedAt,
	}
}

func mapHolidayFromUpdate(r sqlcgen.UpdateHolidayRow) dom.Holiday {
	return dom.Holiday{
		ID:                     r.ID,
		Name:                   r.Name,
		Date:                   pgDateToTime(r.HolidayDate),
		Category:               dom.HolidayCategory(r.Category),
		Recurring:              r.Recurring,
		ApplicableServiceLines: emptyIfNil(r.ApplicableServiceLines),
		CreatedAt:              r.CreatedAt,
		UpdatedAt:              r.UpdatedAt,
	}
}

func emptyIfNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
