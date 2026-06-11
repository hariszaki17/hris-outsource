package placement

import (
	"github.com/jackc/pgx/v5/pgtype"
	"time"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
)

// placementCore holds the scalar (non-denormalized) columns shared by every
// placement row type sqlc generates. Each mapper copies its row into this and
// then attaches the denormalized *_name fields when present.
type placementCore struct {
	ID                string
	EmployeeID        string
	AgreementID       *string
	AwaitingAgreement bool
	ClientCompanyID   string
	SiteID            string
	ServiceLineID     string
	PositionID        string
	StartDate         pgtype.Date
	EndDate           pgtype.Date
	Notes             *string
	LifecycleStatus   string
	StatusChangedAt   time.Time
	EndedReason       *string
	EndedAt           pgtype.Date
	TerminationReason *string
	ResignAt          pgtype.Date
	PredecessorID     *string
	SuccessorID       *string
	BackdateReason    *string
	CreatedBy         *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (c placementCore) toDomain() domain.Placement {
	return domain.Placement{
		ID:                c.ID,
		EmployeeID:        c.EmployeeID,
		AgreementID:       c.AgreementID,
		AwaitingAgreement: c.AwaitingAgreement,
		ClientCompanyID:   c.ClientCompanyID,
		SiteID:            c.SiteID,
		ServiceLineID:     c.ServiceLineID,
		PositionID:        c.PositionID,
		StartDate:         pgtypeToTime(c.StartDate),
		EndDate:           pgDateToPtr(c.EndDate),
		Notes:             c.Notes,
		LifecycleStatus:   c.LifecycleStatus,
		StatusChangedAt:   c.StatusChangedAt,
		EndedReason:       c.EndedReason,
		EndedAt:           pgDateToPtr(c.EndedAt),
		TerminationReason: c.TerminationReason,
		ResignAt:          pgDateToPtr(c.ResignAt),
		PredecessorID:     c.PredecessorID,
		SuccessorID:       c.SuccessorID,
		BackdateReason:    c.BackdateReason,
		CreatedBy:         c.CreatedBy,
		CreatedAt:         c.CreatedAt,
		UpdatedAt:         c.UpdatedAt,
	}
}

func mapPlacementFromList(row sqlcgen.ListPlacementsRow) domain.Placement {
	p := placementCore{
		ID: row.ID, EmployeeID: row.EmployeeID, AgreementID: row.AgreementID,
		AwaitingAgreement: row.AwaitingAgreement,
		ClientCompanyID:   row.ClientCompanyID, SiteID: row.SiteID, ServiceLineID: row.ServiceLineID,
		PositionID: row.PositionID, StartDate: row.StartDate, EndDate: row.EndDate,
		Notes: row.Notes, LifecycleStatus: row.LifecycleStatus, StatusChangedAt: row.StatusChangedAt,
		EndedReason: row.EndedReason, EndedAt: row.EndedAt, TerminationReason: row.TerminationReason,
		ResignAt: row.ResignAt, PredecessorID: row.PredecessorID, SuccessorID: row.SuccessorID,
		BackdateReason: row.BackdateReason, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}.toDomain()
	p.EmployeeName = row.EmployeeName
	p.ClientCompanyName = row.ClientCompanyName
	p.SiteName = row.SiteName
	p.ServiceLineName = row.ServiceLineName
	p.PositionName = row.PositionName
	p.AgreementType = row.AgreementType
	return p
}

func mapPlacementFromExpiring(row sqlcgen.ListExpiringPlacementsRow) domain.Placement {
	p := placementCore{
		ID: row.ID, EmployeeID: row.EmployeeID, AgreementID: row.AgreementID,
		AwaitingAgreement: row.AwaitingAgreement,
		ClientCompanyID:   row.ClientCompanyID, SiteID: row.SiteID, ServiceLineID: row.ServiceLineID,
		PositionID: row.PositionID, StartDate: row.StartDate, EndDate: row.EndDate,
		Notes: row.Notes, LifecycleStatus: row.LifecycleStatus, StatusChangedAt: row.StatusChangedAt,
		EndedReason: row.EndedReason, EndedAt: row.EndedAt, TerminationReason: row.TerminationReason,
		ResignAt: row.ResignAt, PredecessorID: row.PredecessorID, SuccessorID: row.SuccessorID,
		BackdateReason: row.BackdateReason, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}.toDomain()
	p.EmployeeName = row.EmployeeName
	p.ClientCompanyName = row.ClientCompanyName
	p.SiteName = row.SiteName
	p.ServiceLineName = row.ServiceLineName
	p.PositionName = row.PositionName
	p.AgreementType = row.AgreementType
	return p
}

func mapPlacementFromGetByID(row sqlcgen.GetPlacementByIDRow) domain.Placement {
	p := placementCore{
		ID: row.ID, EmployeeID: row.EmployeeID, AgreementID: row.AgreementID,
		AwaitingAgreement: row.AwaitingAgreement,
		ClientCompanyID:   row.ClientCompanyID, SiteID: row.SiteID, ServiceLineID: row.ServiceLineID,
		PositionID: row.PositionID, StartDate: row.StartDate, EndDate: row.EndDate,
		Notes: row.Notes, LifecycleStatus: row.LifecycleStatus, StatusChangedAt: row.StatusChangedAt,
		EndedReason: row.EndedReason, EndedAt: row.EndedAt, TerminationReason: row.TerminationReason,
		ResignAt: row.ResignAt, PredecessorID: row.PredecessorID, SuccessorID: row.SuccessorID,
		BackdateReason: row.BackdateReason, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}.toDomain()
	p.EmployeeName = row.EmployeeName
	p.ClientCompanyName = row.ClientCompanyName
	p.SiteName = row.SiteName
	p.ServiceLineName = row.ServiceLineName
	p.PositionName = row.PositionName
	p.AgreementType = row.AgreementType
	return p
}

func mapPlacementFromChain(row sqlcgen.GetPlacementChainRow) domain.Placement {
	p := placementCore{
		ID: row.ID, EmployeeID: row.EmployeeID, AgreementID: row.AgreementID,
		AwaitingAgreement: row.AwaitingAgreement,
		ClientCompanyID:   row.ClientCompanyID, SiteID: row.SiteID, ServiceLineID: row.ServiceLineID,
		PositionID: row.PositionID, StartDate: row.StartDate, EndDate: row.EndDate,
		Notes: row.Notes, LifecycleStatus: row.LifecycleStatus, StatusChangedAt: row.StatusChangedAt,
		EndedReason: row.EndedReason, EndedAt: row.EndedAt, TerminationReason: row.TerminationReason,
		ResignAt: row.ResignAt, PredecessorID: row.PredecessorID, SuccessorID: row.SuccessorID,
		BackdateReason: row.BackdateReason, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}.toDomain()
	p.EmployeeName = row.EmployeeName
	p.ClientCompanyName = row.ClientCompanyName
	p.SiteName = row.SiteName
	p.ServiceLineName = row.ServiceLineName
	p.PositionName = row.PositionName
	p.AgreementType = row.AgreementType
	return p
}

func mapPlacementFromActive(row sqlcgen.GetActivePlacementForEmployeeRow) domain.Placement {
	p := placementCore{
		ID: row.ID, EmployeeID: row.EmployeeID, AgreementID: row.AgreementID,
		AwaitingAgreement: row.AwaitingAgreement,
		ClientCompanyID:   row.ClientCompanyID, SiteID: row.SiteID, ServiceLineID: row.ServiceLineID,
		PositionID: row.PositionID, StartDate: row.StartDate, EndDate: row.EndDate,
		Notes: row.Notes, LifecycleStatus: row.LifecycleStatus, StatusChangedAt: row.StatusChangedAt,
		EndedReason: row.EndedReason, EndedAt: row.EndedAt, TerminationReason: row.TerminationReason,
		ResignAt: row.ResignAt, PredecessorID: row.PredecessorID, SuccessorID: row.SuccessorID,
		BackdateReason: row.BackdateReason, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}.toDomain()
	p.EmployeeName = row.EmployeeName
	p.ClientCompanyName = row.ClientCompanyName
	p.SiteName = row.SiteName
	p.ServiceLineName = row.ServiceLineName
	p.PositionName = row.PositionName
	p.AgreementType = row.AgreementType
	return p
}

func mapPlacementFromAtCompany(row sqlcgen.GetActivePlacementForEmployeeAtCompanyForUpdateRow) domain.Placement {
	return placementCore{
		ID: row.ID, EmployeeID: row.EmployeeID, AgreementID: row.AgreementID,
		AwaitingAgreement: row.AwaitingAgreement,
		ClientCompanyID:   row.ClientCompanyID, SiteID: row.SiteID, ServiceLineID: row.ServiceLineID,
		PositionID: row.PositionID, StartDate: row.StartDate, EndDate: row.EndDate,
		Notes: row.Notes, LifecycleStatus: row.LifecycleStatus, StatusChangedAt: row.StatusChangedAt,
		EndedReason: row.EndedReason, EndedAt: row.EndedAt, TerminationReason: row.TerminationReason,
		ResignAt: row.ResignAt, PredecessorID: row.PredecessorID, SuccessorID: row.SuccessorID,
		BackdateReason: row.BackdateReason, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}.toDomain()
}

func mapPlacementFromLock(row sqlcgen.LockEmployeePlacementsRow) domain.Placement {
	return placementCore{
		ID: row.ID, EmployeeID: row.EmployeeID, AgreementID: row.AgreementID,
		AwaitingAgreement: row.AwaitingAgreement,
		ClientCompanyID:   row.ClientCompanyID, SiteID: row.SiteID, ServiceLineID: row.ServiceLineID,
		PositionID: row.PositionID, StartDate: row.StartDate, EndDate: row.EndDate,
		Notes: row.Notes, LifecycleStatus: row.LifecycleStatus, StatusChangedAt: row.StatusChangedAt,
		EndedReason: row.EndedReason, EndedAt: row.EndedAt, TerminationReason: row.TerminationReason,
		ResignAt: row.ResignAt, PredecessorID: row.PredecessorID, SuccessorID: row.SuccessorID,
		BackdateReason: row.BackdateReason, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}.toDomain()
}

func mapPlacementFromCreate(row sqlcgen.CreatePlacementRow) domain.Placement {
	return placementCore{
		ID: row.ID, EmployeeID: row.EmployeeID, AgreementID: row.AgreementID,
		AwaitingAgreement: row.AwaitingAgreement,
		ClientCompanyID:   row.ClientCompanyID, SiteID: row.SiteID, ServiceLineID: row.ServiceLineID,
		PositionID: row.PositionID, StartDate: row.StartDate, EndDate: row.EndDate,
		Notes: row.Notes, LifecycleStatus: row.LifecycleStatus, StatusChangedAt: row.StatusChangedAt,
		EndedReason: row.EndedReason, EndedAt: row.EndedAt, TerminationReason: row.TerminationReason,
		ResignAt: row.ResignAt, PredecessorID: row.PredecessorID, SuccessorID: row.SuccessorID,
		BackdateReason: row.BackdateReason, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}.toDomain()
}

func mapPlacementFromUpdate(row sqlcgen.UpdatePlacementFieldsRow) domain.Placement {
	return placementCore{
		ID: row.ID, EmployeeID: row.EmployeeID, AgreementID: row.AgreementID,
		AwaitingAgreement: row.AwaitingAgreement,
		ClientCompanyID:   row.ClientCompanyID, SiteID: row.SiteID, ServiceLineID: row.ServiceLineID,
		PositionID: row.PositionID, StartDate: row.StartDate, EndDate: row.EndDate,
		Notes: row.Notes, LifecycleStatus: row.LifecycleStatus, StatusChangedAt: row.StatusChangedAt,
		EndedReason: row.EndedReason, EndedAt: row.EndedAt, TerminationReason: row.TerminationReason,
		ResignAt: row.ResignAt, PredecessorID: row.PredecessorID, SuccessorID: row.SuccessorID,
		BackdateReason: row.BackdateReason, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}.toDomain()
}

func mapPlacementFromSetAgreement(row sqlcgen.SetPlacementAgreementRow) domain.Placement {
	return placementCore{
		ID: row.ID, EmployeeID: row.EmployeeID, AgreementID: row.AgreementID,
		AwaitingAgreement: row.AwaitingAgreement,
		ClientCompanyID:   row.ClientCompanyID, SiteID: row.SiteID, ServiceLineID: row.ServiceLineID,
		PositionID: row.PositionID, StartDate: row.StartDate, EndDate: row.EndDate,
		Notes: row.Notes, LifecycleStatus: row.LifecycleStatus, StatusChangedAt: row.StatusChangedAt,
		EndedReason: row.EndedReason, EndedAt: row.EndedAt, TerminationReason: row.TerminationReason,
		ResignAt: row.ResignAt, PredecessorID: row.PredecessorID, SuccessorID: row.SuccessorID,
		BackdateReason: row.BackdateReason, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}.toDomain()
}

func mapPlacementFromSetLifecycle(row sqlcgen.SetPlacementLifecycleRow) domain.Placement {
	return placementCore{
		ID: row.ID, EmployeeID: row.EmployeeID, AgreementID: row.AgreementID,
		AwaitingAgreement: row.AwaitingAgreement,
		ClientCompanyID:   row.ClientCompanyID, SiteID: row.SiteID, ServiceLineID: row.ServiceLineID,
		PositionID: row.PositionID, StartDate: row.StartDate, EndDate: row.EndDate,
		Notes: row.Notes, LifecycleStatus: row.LifecycleStatus, StatusChangedAt: row.StatusChangedAt,
		EndedReason: row.EndedReason, EndedAt: row.EndedAt, TerminationReason: row.TerminationReason,
		ResignAt: row.ResignAt, PredecessorID: row.PredecessorID, SuccessorID: row.SuccessorID,
		BackdateReason: row.BackdateReason, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}.toDomain()
}
