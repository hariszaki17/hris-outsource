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
