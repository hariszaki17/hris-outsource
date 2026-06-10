// Package people (repository) implements the people service's EmployeeRepository
// over sqlc-generated queries. Mirrors the org repository pattern exactly:
// reads on the pool, writes via r.q.WithTx(tx), pgx.ErrNoRows → domain.ErrNotFound.
package people

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/people"
)

// Repository is the sqlc-backed implementation of svc.EmployeeRepository.
type Repository struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

// compile-time check: Repository satisfies the service port.
var _ svc.EmployeeRepository = (*Repository)(nil)

// New returns a new Repository backed by pool.
func New(pool *db.Pool) *Repository {
	return &Repository{pool: pool, q: sqlcgen.New(pool.Pool)}
}

// ListEmployees returns a page of employees matching the filter.
func (r *Repository) ListEmployees(ctx context.Context, f domain.EmployeeFilter) ([]domain.Employee, error) {
	rows, err := r.q.ListEmployees(ctx, sqlcgen.ListEmployeesParams{
		Status:          f.Status,
		Q:               f.Q,
		Role:            f.Role,
		Assigned:        f.Assigned,
		ClientCompany:   f.ClientCompanyID,
		CursorCreatedAt: f.CursorCreatedAt,
		CursorID:        f.CursorID,
		RowLimit:        int32(f.Limit),
	})
	if err != nil {
		return nil, err
	}

	out := make([]domain.Employee, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapEmployeeFromList(row))
	}
	return out, nil
}

// GetEmployeeByID fetches a single employee by SWP-EMP id.
func (r *Repository) GetEmployeeByID(ctx context.Context, id string) (domain.Employee, error) {
	row, err := r.q.GetEmployeeByID(ctx, id)
	if err != nil {
		return domain.Employee{}, mapErr(err)
	}
	return mapEmployeeFromGetByID(row), nil
}

// UserEmailTaken reports whether a (non-deleted) user already uses this email (EP-2).
func (r *Repository) UserEmailTaken(ctx context.Context, email string) (bool, error) {
	_, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(mapErr(err), domain.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// UserPhoneTaken reports whether a (non-deleted) user already uses this phone (EP-2).
func (r *Repository) UserPhoneTaken(ctx context.Context, phone string) (bool, error) {
	_, err := r.q.GetUserByIdentifier(ctx, phone)
	if err != nil {
		if errors.Is(mapErr(err), domain.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ProvisionLogin creates the linked agent User (must_change_password=true,
// company_id NULL — agents scope via placement) and links employees.user_id.
// Both writes run in the caller's tx (EP-3). Returns the new SWP-USR id. Phone is
// the required login identifier (D2); Email is optional.
func (r *Repository) ProvisionLogin(ctx context.Context, tx pgx.Tx, p svc.ProvisionLoginRepoParams) (string, error) {
	q := r.q.WithTx(tx)
	user, err := q.CreateUser(ctx, sqlcgen.CreateUserParams{
		Email:              nullStr(p.Email),
		Phone:              nullStr(p.Phone),
		PasswordHash:       p.PasswordHash,
		Role:               "agent",
		EmployeeID:         &p.EmployeeID,
		CompanyID:          nil,
		FullName:           p.FullName,
		MustChangePassword: true,
	})
	if err != nil {
		return "", mapErr(err)
	}
	if err := q.SetEmployeeUserID(ctx, sqlcgen.SetEmployeeUserIDParams{
		UserID: &user.ID,
		ID:     p.EmployeeID,
	}); err != nil {
		return "", mapErr(err)
	}
	return user.ID, nil
}

// RegenerateTempPassword re-issues a temp password hash for an existing login and
// forces a rotation on next login (EP-3 :regenerate-password).
func (r *Repository) RegenerateTempPassword(ctx context.Context, tx pgx.Tx, userID, passwordHash string) error {
	return mapErr(r.q.WithTx(tx).RegenerateTempPassword(ctx, sqlcgen.RegenerateTempPasswordParams{
		PasswordHash: passwordHash,
		ID:           userID,
	}))
}

// DisableUserAndRevoke disables the linked login and revokes its sessions in the
// caller's tx (F2.7): status=disabled + session-epoch bump + refresh revocation.
func (r *Repository) DisableUserAndRevoke(ctx context.Context, tx pgx.Tx, userID string) error {
	q := r.q.WithTx(tx)
	if _, err := q.SetUserStatus(ctx, sqlcgen.SetUserStatusParams{ID: userID, Status: "disabled"}); err != nil {
		return mapErr(err)
	}
	if err := q.BumpTokensValidAfter(ctx, userID); err != nil {
		return mapErr(err)
	}
	return mapErr(q.RevokeAllRefreshForUser(ctx, userID))
}

// EnableUser re-enables a linked login (F2.7 reactivation). Sessions are not
// restored — the epoch already advanced, so the agent must sign in again.
func (r *Repository) EnableUser(ctx context.Context, tx pgx.Tx, userID string) error {
	q := r.q.WithTx(tx)
	if _, err := q.SetUserStatus(ctx, sqlcgen.SetUserStatusParams{ID: userID, Status: "active"}); err != nil {
		return mapErr(err)
	}
	return nil
}

// GetActiveAgreementForEmployee resolves the employee's active employment
// agreement for the offboard cascade (OB-1). found=false when there is no active
// agreement (allowed — the close step is skipped).
func (r *Repository) GetActiveAgreementForEmployee(ctx context.Context, employeeID string) (string, bool, error) {
	row, err := r.q.GetActiveAgreementForEmployee(ctx, employeeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return row.ID, true, nil
}

// CloseAgreement closes an employment agreement in the caller's tx (offboard
// cascade OB-1): status='closed' + closed_reason + closed_at. successor_id stays
// nil — this is a terminal close, not a supersede-on-renew.
func (r *Repository) CloseAgreement(ctx context.Context, tx pgx.Tx, agreementID, closedReason string, closedAt time.Time) error {
	_, err := r.q.WithTx(tx).SetAgreementStatus(ctx, sqlcgen.SetAgreementStatusParams{
		Status:       "closed",
		ClosedReason: &closedReason,
		ClosedAt:     &closedAt,
		SuccessorID:  nil,
		ID:           agreementID,
	})
	return mapErr(err)
}

// EndPlacementsForEmployee ends every non-terminal placement of an employee in
// the caller's tx (offboard cascade OB-1) and returns the ended placement ids
// (for the per-placement audit entry).
func (r *Repository) EndPlacementsForEmployee(ctx context.Context, tx pgx.Tx, employeeID, lifecycleStatus, endedReason string, endedAt time.Time) ([]string, error) {
	rows, err := r.q.WithTx(tx).EndPlacementsForEmployee(ctx, sqlcgen.EndPlacementsForEmployeeParams{
		LifecycleStatus: lifecycleStatus,
		EndedReason:     endedReason,
		EndedAt:         dateToPgtype(endedAt),
		EmployeeID:      employeeID,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids, nil
}

// GetEmployeeByNIK fetches a single employee by NIK (duplicate-NIK pre-check, EP-2).
func (r *Repository) GetEmployeeByNIK(ctx context.Context, nik string) (domain.Employee, error) {
	row, err := r.q.GetEmployeeByNIK(ctx, nik)
	if err != nil {
		return domain.Employee{}, mapErr(err)
	}
	return mapEmployeeFromGetByNIK(row), nil
}

// CreateEmployee inserts a new employee in the given transaction.
func (r *Repository) CreateEmployee(ctx context.Context, tx pgx.Tx, p svc.CreateEmployeeParams) (domain.Employee, error) {
	joinAt := dateToPgtype(p.JoinAt)
	var birthDate pgtype.Date
	if p.BirthDate != nil {
		birthDate = dateToPgtype(*p.BirthDate)
	}

	row, err := r.q.WithTx(tx).CreateEmployee(ctx, sqlcgen.CreateEmployeeParams{
		UserID:                nil, // EP-3 stub: always NULL in Phase 4
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
		CreatedBy:             nullStr(p.CreatedBy),
	})
	if err != nil {
		return domain.Employee{}, mapErr(err)
	}
	return mapEmployeeFromCreate(row), nil
}

// UpdateEmployee patches an employee's mutable fields.
func (r *Repository) UpdateEmployee(ctx context.Context, tx pgx.Tx, p svc.UpdateEmployeeParams) (domain.Employee, error) {
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

// SetEmployeeStatus updates the status of an employee (active/inactive).
func (r *Repository) SetEmployeeStatus(ctx context.Context, tx pgx.Tx, id, status string) (domain.Employee, error) {
	row, err := r.q.WithTx(tx).SetEmployeeStatus(ctx, sqlcgen.SetEmployeeStatusParams{
		ID:     id,
		Status: status,
	})
	if err != nil {
		return domain.Employee{}, mapErr(err)
	}
	return mapEmployeeFromSetStatus(row), nil
}

// --- mapping helpers ---

func mapEmployeeFromList(row sqlcgen.ListEmployeesRow) domain.Employee {
	emp := domain.Employee{
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
		Status:    row.Status,
		HasLogin:  row.UserID != nil,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
		CreatedBy: row.CreatedBy,
	}
	// current_* come from the employee's non-terminal placement (null when unplaced).
	if row.CurrentPositionID != nil {
		emp.CurrentPosition = &domain.PositionRef{ID: *row.CurrentPositionID, Name: derefStr(row.CurrentPositionName)}
	}
	if row.CurrentServiceLineID != nil {
		emp.CurrentServiceLine = &domain.ServiceLineRef{ID: *row.CurrentServiceLineID, Name: derefStr(row.CurrentServiceLineName)}
	}
	if row.CurrentClientCompanyID != nil {
		emp.CurrentClientCompany = &domain.ClientCompanyRef{ID: *row.CurrentClientCompanyID, Name: derefStr(row.CurrentClientCompanyName)}
	}
	return emp
}

func mapEmployeeFromGetByID(row sqlcgen.GetEmployeeByIDRow) domain.Employee {
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
		Status:    row.Status,
		HasLogin:  row.UserID != nil,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
		CreatedBy: row.CreatedBy,
	}
}

func mapEmployeeFromGetByNIK(row sqlcgen.GetEmployeeByNIKRow) domain.Employee {
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
		Status:    row.Status,
		HasLogin:  row.UserID != nil,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
		CreatedBy: row.CreatedBy,
	}
}

func mapEmployeeFromCreate(row sqlcgen.CreateEmployeeRow) domain.Employee {
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
		Status:    row.Status,
		HasLogin:  row.UserID != nil,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
		CreatedBy: row.CreatedBy,
	}
}

func mapEmployeeFromUpdate(row sqlcgen.UpdateEmployeeRow) domain.Employee {
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
		Status:    row.Status,
		HasLogin:  row.UserID != nil,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
		CreatedBy: row.CreatedBy,
	}
}

func mapEmployeeFromSetStatus(row sqlcgen.SetEmployeeStatusRow) domain.Employee {
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
		Status:    row.Status,
		HasLogin:  row.UserID != nil,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
		CreatedBy: row.CreatedBy,
	}
}

// --- error helpers ---

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

// --- type conversion helpers ---

// dateToPgtype converts a Go time.Time to a pgtype.Date (valid=true).
func dateToPgtype(t time.Time) pgtype.Date {
	return pgtype.Date{
		Time:  t,
		Valid: !t.IsZero(),
	}
}

// pgtypeToTime converts a pgtype.Date to time.Time (zero if invalid).
func pgtypeToTime(d pgtype.Date) time.Time {
	if !d.Valid {
		return time.Time{}
	}
	return d.Time
}

// pgtypeDateToPtr converts a pgtype.Date to *time.Time (nil if invalid).
func pgtypeDateToPtr(d pgtype.Date) *time.Time {
	if !d.Valid {
		return nil
	}
	return &d.Time
}

// nullStr returns a *string; returns nil if the string is empty.
func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// derefStr safely dereferences a *string; returns "" if nil.
func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
