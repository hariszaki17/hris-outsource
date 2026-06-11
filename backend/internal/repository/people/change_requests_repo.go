// Package people (repository) — ChangeRequestRepo implements the
// ChangeRequestRepository interface over sqlc-generated queries.
// Mirrors the pattern of Repository (employees_repo.go) and AgreementRepo
// (agreements_repo.go) in the same package.
package people

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/people"
)

// ChangeRequestRepo is the sqlc-backed implementation of svc.ChangeRequestRepository.
type ChangeRequestRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

// compile-time check: ChangeRequestRepo satisfies the service port.
var _ svc.ChangeRequestRepository = (*ChangeRequestRepo)(nil)

// NewChangeRequestRepo returns a new ChangeRequestRepo backed by pool.
func NewChangeRequestRepo(pool *db.Pool) *ChangeRequestRepo {
	return &ChangeRequestRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

// ListChangeRequests returns a cursor-paginated page of change requests.
func (r *ChangeRequestRepo) ListChangeRequests(ctx context.Context, f domain.ChangeRequestFilter) ([]domain.ChangeRequest, error) {
	rows, err := r.q.ListChangeRequests(ctx, sqlcgen.ListChangeRequestsParams{
		Status:            f.Status,
		EmployeeID:        f.EmployeeID,
		RequestType:       f.RequestType,
		CursorSubmittedAt: f.CursorSubmittedAt,
		CursorID:          f.CursorID,
		RowLimit:          int32(f.Limit),
	})
	if err != nil {
		return nil, err
	}

	out := make([]domain.ChangeRequest, 0, len(rows))
	for _, row := range rows {
		cr, err := mapChangeRequest(crRowFromList(row))
		if err != nil {
			return nil, err
		}
		out = append(out, cr)
	}
	return out, nil
}

// GetChangeRequestByID fetches a single change request by SWP-CHG id.
func (r *ChangeRequestRepo) GetChangeRequestByID(ctx context.Context, id string) (domain.ChangeRequest, error) {
	row, err := r.q.GetChangeRequestByID(ctx, id)
	if err != nil {
		return domain.ChangeRequest{}, mapErr(err)
	}
	return mapChangeRequest(crRowFromGet(row))
}

// GetEmployeeByID fetches a single employee by id (needed for the diff in detail + approve).
// Reuses mapEmployeeFromGetByID defined in employees_repo.go (same package).
func (r *ChangeRequestRepo) GetEmployeeByID(ctx context.Context, id string) (domain.Employee, error) {
	row, err := r.q.GetEmployeeByID(ctx, id)
	if err != nil {
		return domain.Employee{}, mapErr(err)
	}
	return mapEmployeeFromGetByID(row), nil
}

// GetEmployeeCompanyID returns the employee's current (non-terminal) client
// company id, used for shift-leader company-scope routing on approve/reject.
// Reuses the E3 GetActivePlacementForEmployee query (ACTIVE/EXPIRING/
// PENDING_START). Returns "" (no error) when the employee has no active
// placement — GuardCompany then rejects a leader (empty != their company) and
// passes HR/super through.
func (r *ChangeRequestRepo) GetEmployeeCompanyID(ctx context.Context, employeeID string) (string, error) {
	row, err := r.q.GetActivePlacementForEmployee(ctx, employeeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return row.ClientCompanyID, nil
}

// UpdateEmployee applies the whitelisted change-request fields to an employee (in tx).
// Reuses the same sqlcgen.UpdateEmployee query and mapEmployeeFromUpdate mapper as
// employees_repo.go — both are in the same package so they share the helpers.
func (r *ChangeRequestRepo) UpdateEmployee(ctx context.Context, tx pgx.Tx, p svc.UpdateEmployeeParams) (domain.Employee, error) {
	joinAt := dateToPgtype(p.JoinAt)
	var birthDate pgtype.Date
	if p.BirthDate != nil {
		birthDate = dateToPgtype(*p.BirthDate)
	}

	row, err := r.q.WithTx(tx).UpdateEmployee(ctx, sqlcgen.UpdateEmployeeParams{
		ID:                    p.ID,
		FullName:              p.FullName,
		Nik:                   p.NIK,
		Nip:                   p.NIP,
		JoinAt:                joinAt,
		Gender:                nullStr(p.Gender),
		BirthDate:             birthDate,
		BirthPlace:            nullStr(p.BirthPlace),
		Phone:                 nullStr(p.Phone),
		EmailPersonal:         nullStr(p.EmailPersonal),
		Address:               nullStr(p.Address),
		Npwp:                  nullStr(p.NPWP),
		BpjsKesehatan:         nullStr(p.BPJSKesehatan),
		BpjsKetenagakerjaan:   nullStr(p.BPJSKetenagakerjaan),
		BankName:              nullStr(p.BankName),
		BankAccountNumber:     nullStr(p.BankAccountNumber),
		BankAccountHolderName: nullStr(p.BankAccountHolderName),
		EmergencyContactName:  nullStr(p.EmergencyContactName),
		EmergencyContactPhone: nullStr(p.EmergencyContactPhone),
	})
	if err != nil {
		return domain.Employee{}, mapErr(err)
	}
	return mapEmployeeFromUpdate(row), nil
}

// ResolveChangeRequest drives :approve, :reject, and the SL bank-split partial
// apply (status='partially_approved' + bank_pending + field_resolutions). Sets
// status, field_resolutions, bank_pending, and (for terminal resolutions)
// resolved_at/resolved_by/rejection_reason (in tx).
func (r *ChangeRequestRepo) ResolveChangeRequest(ctx context.Context, tx pgx.Tx, p svc.ResolveChangeRequestParams) (domain.ChangeRequest, error) {
	// Marshal the per-field resolutions to jsonb; nil/empty → '{}' so the NOT
	// NULL column always receives a valid object.
	resolutions := []byte("{}")
	if len(p.FieldResolutions) > 0 {
		raw, merr := json.Marshal(p.FieldResolutions)
		if merr != nil {
			return domain.ChangeRequest{}, merr
		}
		resolutions = raw
	}

	row, err := r.q.WithTx(tx).ResolveChangeRequest(ctx, sqlcgen.ResolveChangeRequestParams{
		ID:               p.ID,
		Status:           p.Status,
		FieldResolutions: resolutions,
		BankPending:      p.BankPending,
		ResolvedAt:       p.ResolvedAt,
		ResolvedBy:       p.ResolvedBy,
		RejectionReason:  p.RejectionReason,
	})
	if err != nil {
		return domain.ChangeRequest{}, mapErr(err)
	}
	return mapChangeRequest(crRowFromResolve(row))
}

// CreateChangeRequest inserts a new PENDING change request (in tx). The changes
// value object is marshalled to the jsonb `changes` column; request_type is the
// caller-derived PHONE|EMERGENCY_CONTACT|BANK_ACCOUNT|MULTIPLE. The SWP-CHG id
// and status=pending default are allocated by the query.
func (r *ChangeRequestRepo) CreateChangeRequest(ctx context.Context, tx pgx.Tx, p svc.CreateChangeRequestParams) (domain.ChangeRequest, error) {
	changesJSON, err := json.Marshal(p.Changes)
	if err != nil {
		return domain.ChangeRequest{}, err
	}

	row, err := r.q.WithTx(tx).CreateChangeRequest(ctx, sqlcgen.CreateChangeRequestParams{
		EmployeeID:  p.EmployeeID,
		Changes:     changesJSON,
		RequestType: p.RequestType,
		Note:        p.Note,
	})
	if err != nil {
		return domain.ChangeRequest{}, mapErr(err)
	}
	return mapChangeRequest(crRowFromCreate(row))
}

// --- mapping helpers ---

// crRow is the normalized field set shared by every change_requests query row
// (List/Get/Create/Resolve all RETURNING the same columns). The sqlc-generated
// row types are distinct structs but structurally identical; each call site
// adapts its row into this before mapping (sqlc has no shared row type once the
// queries diverged in column order).
type crRow struct {
	ID               string
	EmployeeID       string
	Status           string
	Changes          []byte
	RequestType      string
	Note             *string
	FieldResolutions []byte
	BankPending      bool
	SubmittedAt      time.Time
	ResolvedAt       *time.Time
	ResolvedBy       *string
	RejectionReason  *string
}

// mapChangeRequest converts a normalized crRow to domain.ChangeRequest. The
// changes + field_resolutions jsonb columns are unmarshalled into the domain
// value objects.
func mapChangeRequest(row crRow) (domain.ChangeRequest, error) {
	var changes domain.ChangeRequestChanges
	if len(row.Changes) > 0 {
		if err := json.Unmarshal(row.Changes, &changes); err != nil {
			return domain.ChangeRequest{}, err
		}
	}
	var fieldResolutions map[string]domain.FieldResolution
	if len(row.FieldResolutions) > 0 {
		if err := json.Unmarshal(row.FieldResolutions, &fieldResolutions); err != nil {
			return domain.ChangeRequest{}, err
		}
	}
	return domain.ChangeRequest{
		ID:               row.ID,
		EmployeeID:       row.EmployeeID,
		Status:           row.Status,
		SubmittedAt:      row.SubmittedAt,
		ResolvedAt:       row.ResolvedAt,
		ResolvedBy:       row.ResolvedBy,
		RejectionReason:  row.RejectionReason,
		Note:             row.Note,
		Changes:          changes,
		RequestType:      row.RequestType,
		FieldResolutions: fieldResolutions,
		BankPending:      row.BankPending,
	}, nil
}

// crRowFromList / *Get / *Resolve / *Create normalize each distinct sqlc row
// type into the shared crRow.
func crRowFromList(r sqlcgen.ListChangeRequestsRow) crRow {
	return crRow{r.ID, r.EmployeeID, r.Status, r.Changes, r.RequestType, r.Note, r.FieldResolutions, r.BankPending, r.SubmittedAt, r.ResolvedAt, r.ResolvedBy, r.RejectionReason}
}

func crRowFromGet(r sqlcgen.GetChangeRequestByIDRow) crRow {
	return crRow{r.ID, r.EmployeeID, r.Status, r.Changes, r.RequestType, r.Note, r.FieldResolutions, r.BankPending, r.SubmittedAt, r.ResolvedAt, r.ResolvedBy, r.RejectionReason}
}

func crRowFromResolve(r sqlcgen.ResolveChangeRequestRow) crRow {
	return crRow{r.ID, r.EmployeeID, r.Status, r.Changes, r.RequestType, r.Note, r.FieldResolutions, r.BankPending, r.SubmittedAt, r.ResolvedAt, r.ResolvedBy, r.RejectionReason}
}

func crRowFromCreate(r sqlcgen.CreateChangeRequestRow) crRow {
	return crRow{r.ID, r.EmployeeID, r.Status, r.Changes, r.RequestType, r.Note, r.FieldResolutions, r.BankPending, r.SubmittedAt, r.ResolvedAt, r.ResolvedBy, r.RejectionReason}
}
