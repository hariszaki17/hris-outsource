// Package people (repository) — SelfProfileRepo implements the
// SelfProfileRepository service port over sqlc. Distinct from Repository so its
// mappers surface the self-service fields (emergency_contact / app_language /
// photo_object_key) that the staff employee mappers intentionally omit.
package people

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/people"
)

// SelfProfileRepo is the sqlc-backed implementation of svc.SelfProfileRepository.
type SelfProfileRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

// compile-time check: SelfProfileRepo satisfies the service port.
var _ svc.SelfProfileRepository = (*SelfProfileRepo)(nil)

// NewSelfProfileRepo returns a SelfProfileRepo backed by pool.
func NewSelfProfileRepo(pool *db.Pool) *SelfProfileRepo {
	return &SelfProfileRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

// GetEmployeeByID fetches the caller's own employee record with the self-service
// fields populated (used to validate + return the current state).
func (r *SelfProfileRepo) GetEmployeeByID(ctx context.Context, id string) (domain.Employee, error) {
	row, err := r.q.GetEmployeeByID(ctx, id)
	if err != nil {
		return domain.Employee{}, mapErr(err)
	}
	return mapEmployeeFromGetByID(row), nil
}

// UpdateEmployeeSelfInstant applies the instant-tier fields (COALESCE) and
// returns the updated employee with the self-service fields populated.
func (r *SelfProfileRepo) UpdateEmployeeSelfInstant(ctx context.Context, tx pgx.Tx, p svc.UpdateEmployeeSelfInstantParams) (domain.Employee, error) {
	row, err := r.q.WithTx(tx).UpdateEmployeeSelfInstant(ctx, sqlcgen.UpdateEmployeeSelfInstantParams{
		ID:             p.ID,
		Address:        p.Address,
		AppLanguage:    p.AppLanguage,
		PhotoObjectKey: p.PhotoObjectKey,
	})
	if err != nil {
		return domain.Employee{}, mapErr(err)
	}
	return mapEmployeeFromSelfInstant(row), nil
}

// mapEmployeeFromSelfInstant maps the UpdateEmployeeSelfInstant row to the domain
// entity, including the self-service fields (emergency_contact / app_language /
// photo_object_key).
func mapEmployeeFromSelfInstant(row sqlcgen.UpdateEmployeeSelfInstantRow) domain.Employee {
	return domain.Employee{
		ID:                  row.ID,
		UserID:              row.UserID,
		FullName:            row.FullName,
		NIK:                 row.Nik,
		NIP:                 row.Nip,
		JoinAt:              pgtypeToTime(row.JoinAt),
		Gender:              row.Gender,
		BirthDate:           pgtypeDateToPtr(row.BirthDate),
		BirthPlace:          row.BirthPlace,
		Phone:               row.Phone,
		EmailPersonal:       row.EmailPersonal,
		Address:             row.Address,
		NPWP:                row.Npwp,
		BPJSKesehatan:       row.BpjsKesehatan,
		BPJSKetenagakerjaan: row.BpjsKetenagakerjaan,
		BankAccount: domain.BankAccount{
			BankName:          derefStr(row.BankName),
			AccountNumber:     derefStr(row.BankAccountNumber),
			AccountHolderName: derefStr(row.BankAccountHolderName),
		},
		EmergencyContact: domain.EmergencyContact{
			Name:  derefStr(row.EmergencyContactName),
			Phone: derefStr(row.EmergencyContactPhone),
		},
		AppLanguage:    row.AppLanguage,
		PhotoObjectKey: row.PhotoObjectKey,
		Status:         row.Status,
		HasLogin:       row.UserID != nil,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		CreatedBy:      row.CreatedBy,
	}
}
