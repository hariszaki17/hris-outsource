// Package people (repository) — SelfProfileRepo implements the
// SelfProfileRepository service port over sqlc. Distinct from Repository so its
// mappers surface the self-service fields (emergency_contact / app_language /
// photo_object_key) that the staff employee mappers intentionally omit.
package people

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
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

// GetUserByIdentifier-backed login-phone uniqueness check (E11 instant phone edit).
// Reports whether a non-deleted login user already uses this E.164 phone.
func (r *SelfProfileRepo) UserPhoneTaken(ctx context.Context, phone string) (bool, error) {
	_, err := r.q.GetUserByIdentifier(ctx, phone)
	if err != nil {
		if errors.Is(mapErr(err), domain.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// UpdateEmployeeSelfInstant applies the instant self-edit fields (COALESCE) and
// returns the updated employee with the self-service fields populated. When Phone
// is non-nil (E11), the linked users.phone login identifier is updated in the SAME
// tx so the agent can keep signing in with the new number. There is no sqlc query
// for the users-phone write (it isn't part of any generated query), so it runs as
// a scoped raw exec on the active tx — keyed by employee_id, guarded by the
// users_phone_active_uq partial unique index (a race surfaces as a 23505 → 409).
func (r *SelfProfileRepo) UpdateEmployeeSelfInstant(ctx context.Context, tx pgx.Tx, p svc.UpdateEmployeeSelfInstantParams) (domain.Employee, error) {
	if p.Phone != nil {
		const updateUserPhone = `
			UPDATE users
			SET phone = $1, updated_at = now()
			WHERE employee_id = $2 AND deleted_at IS NULL`
		if _, err := tx.Exec(ctx, updateUserPhone, *p.Phone, p.ID); err != nil {
			return domain.Employee{}, mapUniqueViolation(err)
		}
	}

	row, err := r.q.WithTx(tx).UpdateEmployeeSelfInstant(ctx, sqlcgen.UpdateEmployeeSelfInstantParams{
		ID:                    p.ID,
		Address:               p.Address,
		AppLanguage:           p.AppLanguage,
		PhotoObjectKey:        p.PhotoObjectKey,
		Phone:                 p.Phone,
		EmergencyContactName:  p.EmergencyContactName,
		EmergencyContactPhone: p.EmergencyContactPhone,
		BankName:              p.BankName,
		BankAccountNumber:     p.BankAccountNumber,
		BankAccountHolderName: p.BankAccountHolderName,
	})
	if err != nil {
		return domain.Employee{}, mapErr(err)
	}
	return mapEmployeeFromSelfInstant(row), nil
}

// mapUniqueViolation maps a Postgres unique-violation (23505) on the login-phone
// index to a 409 CONFLICT (phone field) — the race-loser path behind the service's
// UserPhoneTaken pre-check. Anything else passes through unchanged.
func mapUniqueViolation(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return &apperr.Error{
			Code:       "CONFLICT",
			HTTPStatus: 409,
			Fields:     map[string]string{"phone": "Nomor telepon sudah digunakan untuk login lain."},
		}
	}
	return err
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
