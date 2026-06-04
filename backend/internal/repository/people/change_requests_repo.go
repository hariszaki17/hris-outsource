// Package people (repository) — ChangeRequestRepo implements the
// ChangeRequestRepository interface over sqlc-generated queries.
// Mirrors the pattern of Repository (employees_repo.go) and AgreementRepo
// (agreements_repo.go) in the same package.
package people

import (
	"context"
	"encoding/json"

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
		cr, err := mapChangeRequest(row)
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
	return mapChangeRequest(row)
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
	})
	if err != nil {
		return domain.Employee{}, mapErr(err)
	}
	return mapEmployeeFromUpdate(row), nil
}

// ResolveChangeRequest drives :approve and :reject — sets status, resolved_at,
// resolved_by, and rejection_reason (in tx).
func (r *ChangeRequestRepo) ResolveChangeRequest(ctx context.Context, tx pgx.Tx, p svc.ResolveChangeRequestParams) (domain.ChangeRequest, error) {
	row, err := r.q.WithTx(tx).ResolveChangeRequest(ctx, sqlcgen.ResolveChangeRequestParams{
		ID:              p.ID,
		Status:          p.Status,
		ResolvedAt:      p.ResolvedAt,
		ResolvedBy:      p.ResolvedBy,
		RejectionReason: p.RejectionReason,
	})
	if err != nil {
		return domain.ChangeRequest{}, mapErr(err)
	}
	return mapChangeRequest(row)
}

// --- mapping helpers ---

// mapChangeRequest converts a sqlcgen.ChangeRequest to domain.ChangeRequest.
// The changes jsonb column is unmarshalled into domain.ChangeRequestChanges.
func mapChangeRequest(row sqlcgen.ChangeRequest) (domain.ChangeRequest, error) {
	var changes domain.ChangeRequestChanges
	if len(row.Changes) > 0 {
		if err := json.Unmarshal(row.Changes, &changes); err != nil {
			return domain.ChangeRequest{}, err
		}
	}
	return domain.ChangeRequest{
		ID:              row.ID,
		EmployeeID:      row.EmployeeID,
		Status:          row.Status,
		SubmittedAt:     row.SubmittedAt,
		ResolvedAt:      row.ResolvedAt,
		ResolvedBy:      row.ResolvedBy,
		RejectionReason: row.RejectionReason,
		Note:            row.Note,
		Changes:         changes,
		RequestType:     row.RequestType,
	}, nil
}
