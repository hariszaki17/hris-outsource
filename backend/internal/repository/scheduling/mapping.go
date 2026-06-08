// Package scheduling (repository) — sqlc-row → domain mappers + pg type helpers
// for the E4 slice. Mirrors internal/repository/placement/placements_mapping.go:
// date columns surface as pgtype.Date and are converted at this boundary;
// pgx.ErrNoRows → domain.ErrNotFound.
package scheduling

import (
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
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

// --- shift-master mappers ---

func mapShiftMasterFromList(row sqlcgen.ListShiftMastersRow) domain.ShiftMaster {
	return domain.ShiftMaster{
		ID:              row.ID,
		Name:            row.Name,
		StartTime:       row.StartTime,
		EndTime:         row.EndTime,
		BreakStart:      row.BreakStart,
		BreakEnd:        row.BreakEnd,
		ServiceLineID:   row.ServiceLineID,
		ServiceLineName: row.ServiceLineName,
		CrossMidnight:   row.CrossMidnight,
		IsActive:        row.IsActive,
		InUseCount:      row.InUseCount,
		CreatedBy:       row.CreatedBy,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func mapShiftMasterFromGet(row sqlcgen.GetShiftMasterRow) domain.ShiftMaster {
	return domain.ShiftMaster{
		ID:              row.ID,
		Name:            row.Name,
		StartTime:       row.StartTime,
		EndTime:         row.EndTime,
		BreakStart:      row.BreakStart,
		BreakEnd:        row.BreakEnd,
		ServiceLineID:   row.ServiceLineID,
		ServiceLineName: row.ServiceLineName,
		CrossMidnight:   row.CrossMidnight,
		IsActive:        row.IsActive,
		InUseCount:      row.InUseCount,
		CreatedBy:       row.CreatedBy,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func mapShiftMasterFromForUpdate(row sqlcgen.GetShiftMasterForUpdateRow) domain.ShiftMaster {
	return domain.ShiftMaster{
		ID:            row.ID,
		Name:          row.Name,
		StartTime:     row.StartTime,
		EndTime:       row.EndTime,
		BreakStart:    row.BreakStart,
		BreakEnd:      row.BreakEnd,
		ServiceLineID: row.ServiceLineID,
		CrossMidnight: row.CrossMidnight,
		IsActive:      row.IsActive,
		CreatedBy:     row.CreatedBy,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

func mapShiftMasterFromCreate(row sqlcgen.CreateShiftMasterRow) domain.ShiftMaster {
	return domain.ShiftMaster{
		ID:            row.ID,
		Name:          row.Name,
		StartTime:     row.StartTime,
		EndTime:       row.EndTime,
		BreakStart:    row.BreakStart,
		BreakEnd:      row.BreakEnd,
		ServiceLineID: row.ServiceLineID,
		CrossMidnight: row.CrossMidnight,
		IsActive:      row.IsActive,
		CreatedBy:     row.CreatedBy,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

func mapShiftMasterFromUpdate(row sqlcgen.UpdateShiftMasterRow) domain.ShiftMaster {
	return domain.ShiftMaster{
		ID:            row.ID,
		Name:          row.Name,
		StartTime:     row.StartTime,
		EndTime:       row.EndTime,
		BreakStart:    row.BreakStart,
		BreakEnd:      row.BreakEnd,
		ServiceLineID: row.ServiceLineID,
		CrossMidnight: row.CrossMidnight,
		IsActive:      row.IsActive,
		CreatedBy:     row.CreatedBy,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

func mapShiftMasterFromSetActive(row sqlcgen.SetShiftMasterActiveRow) domain.ShiftMaster {
	return domain.ShiftMaster{
		ID:            row.ID,
		Name:          row.Name,
		StartTime:     row.StartTime,
		EndTime:       row.EndTime,
		BreakStart:    row.BreakStart,
		BreakEnd:      row.BreakEnd,
		ServiceLineID: row.ServiceLineID,
		CrossMidnight: row.CrossMidnight,
		IsActive:      row.IsActive,
		CreatedBy:     row.CreatedBy,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

// --- schedule-entry mappers ---

func mapScheduleFromList(row sqlcgen.ListScheduleRow) domain.ScheduleEntry {
	return domain.ScheduleEntry{
		ID:              row.ID,
		EmployeeID:      row.EmployeeID,
		PlacementID:     row.PlacementID,
		CompanyID:       row.CompanyID,
		ServiceLineID:   row.ServiceLineID,
		ShiftMasterID:   row.ShiftMasterID,
		ShiftMasterName: row.ShiftMasterName,
		EmployeeName:    row.EmployeeName,
		CompanyName:     row.CompanyName,
		StartTime:       row.StartTime,
		EndTime:         row.EndTime,
		CrossMidnight:   row.CrossMidnight,
		WorkDate:        pgDateToTime(row.WorkDate),
		Status:          row.Status,
		IsDayOff:        row.IsDayOff,
		ReplacedEntryID: row.ReplacedEntryID,
		CreatedBy:       row.CreatedBy,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func mapScheduleFromByAgent(row sqlcgen.ListScheduleByAgentRow) domain.ScheduleEntry {
	return domain.ScheduleEntry{
		ID:              row.ID,
		EmployeeID:      row.EmployeeID,
		PlacementID:     row.PlacementID,
		CompanyID:       row.CompanyID,
		ServiceLineID:   row.ServiceLineID,
		ShiftMasterID:   row.ShiftMasterID,
		ShiftMasterName: row.ShiftMasterName,
		EmployeeName:    row.EmployeeName,
		CompanyName:     row.CompanyName,
		StartTime:       row.StartTime,
		EndTime:         row.EndTime,
		CrossMidnight:   row.CrossMidnight,
		WorkDate:        pgDateToTime(row.WorkDate),
		Status:          row.Status,
		IsDayOff:        row.IsDayOff,
		ReplacedEntryID: row.ReplacedEntryID,
		CreatedBy:       row.CreatedBy,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func mapScheduleFromGet(row sqlcgen.GetScheduleEntryRow) domain.ScheduleEntry {
	return domain.ScheduleEntry{
		ID:              row.ID,
		EmployeeID:      row.EmployeeID,
		PlacementID:     row.PlacementID,
		CompanyID:       row.CompanyID,
		ServiceLineID:   row.ServiceLineID,
		ShiftMasterID:   row.ShiftMasterID,
		ShiftMasterName: row.ShiftMasterName,
		EmployeeName:    row.EmployeeName,
		CompanyName:     row.CompanyName,
		StartTime:       row.StartTime,
		EndTime:         row.EndTime,
		CrossMidnight:   row.CrossMidnight,
		WorkDate:        pgDateToTime(row.WorkDate),
		Status:          row.Status,
		IsDayOff:        row.IsDayOff,
		ReplacedEntryID: row.ReplacedEntryID,
		CreatedBy:       row.CreatedBy,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func mapScheduleFromForUpdate(row sqlcgen.GetScheduleEntryForUpdateRow) domain.ScheduleEntry {
	return domain.ScheduleEntry{
		ID:              row.ID,
		EmployeeID:      row.EmployeeID,
		PlacementID:     row.PlacementID,
		ServiceLineID:   row.ServiceLineID,
		ShiftMasterID:   row.ShiftMasterID,
		StartTime:       row.StartTime,
		EndTime:         row.EndTime,
		CrossMidnight:   row.CrossMidnight,
		WorkDate:        pgDateToTime(row.WorkDate),
		Status:          row.Status,
		IsDayOff:        row.IsDayOff,
		ReplacedEntryID: row.ReplacedEntryID,
		CreatedBy:       row.CreatedBy,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func mapScheduleFromCreate(row sqlcgen.CreateScheduleEntryRow) domain.ScheduleEntry {
	return domain.ScheduleEntry{
		ID:              row.ID,
		EmployeeID:      row.EmployeeID,
		PlacementID:     row.PlacementID,
		ServiceLineID:   row.ServiceLineID,
		ShiftMasterID:   row.ShiftMasterID,
		StartTime:       row.StartTime,
		EndTime:         row.EndTime,
		CrossMidnight:   row.CrossMidnight,
		WorkDate:        pgDateToTime(row.WorkDate),
		Status:          row.Status,
		IsDayOff:        row.IsDayOff,
		ReplacedEntryID: row.ReplacedEntryID,
		CreatedBy:       row.CreatedBy,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func mapScheduleFromUpdate(row sqlcgen.UpdateScheduleEntryRow) domain.ScheduleEntry {
	return domain.ScheduleEntry{
		ID:              row.ID,
		EmployeeID:      row.EmployeeID,
		PlacementID:     row.PlacementID,
		ServiceLineID:   row.ServiceLineID,
		ShiftMasterID:   row.ShiftMasterID,
		StartTime:       row.StartTime,
		EndTime:         row.EndTime,
		CrossMidnight:   row.CrossMidnight,
		WorkDate:        pgDateToTime(row.WorkDate),
		Status:          row.Status,
		IsDayOff:        row.IsDayOff,
		ReplacedEntryID: row.ReplacedEntryID,
		CreatedBy:       row.CreatedBy,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}
