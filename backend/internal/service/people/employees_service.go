// Package people is the E2 employees service: employee management (list, get,
// create, update, deactivate, reactivate). Business rules (EP-1..EP-3),
// duplicate-NIK guard (EP-2), status transitions, and audit on every write.
// Mirrors the org service pattern exactly.
package people

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
)

// EmployeeRepository is the data dependency, defined by this consumer (Go idiom).
// The repository layer in internal/repository/people implements it over sqlc.
type EmployeeRepository interface {
	// Reads run on the pool.
	ListEmployees(ctx context.Context, f domain.EmployeeFilter) ([]domain.Employee, error)
	GetEmployeeByID(ctx context.Context, id string) (domain.Employee, error)
	GetEmployeeByNIK(ctx context.Context, nik string) (domain.Employee, error)
	// Writes take the active transaction.
	CreateEmployee(ctx context.Context, tx pgx.Tx, p CreateEmployeeParams) (domain.Employee, error)
	UpdateEmployee(ctx context.Context, tx pgx.Tx, p UpdateEmployeeParams) (domain.Employee, error)
	SetEmployeeStatus(ctx context.Context, tx pgx.Tx, id, status string) (domain.Employee, error)
}

// CreateEmployeeParams carries the fields for inserting a new employee.
type CreateEmployeeParams struct {
	FullName             string
	NIK                  string
	NIP                  string
	JoinAt               time.Time
	Gender               string
	BirthDate            *time.Time
	BirthPlace           string
	Phone                string
	EmailPersonal        string
	Address              string
	NPWP                 string
	BPJSKesehatan        string
	BPJSKetenagakerjaan  string
	BankName             string
	BankAccountNumber    string
	BankAccountHolderName string
	CreatedBy            string
	// Phase-4 stubs: login provisioning deferred.
	// provision_login and login_email are accepted from the request but
	// UserID stays NULL in this milestone (EP-3 is Phase-4 scope).
	// See SUMMARY.md for the stub documentation.
}

// UpdateEmployeeParams carries the fields for updating an employee.
type UpdateEmployeeParams struct {
	ID                    string
	FullName              string
	NIK                   string
	NIP                   string
	JoinAt                time.Time
	Gender                string
	BirthDate             *time.Time
	BirthPlace            string
	Phone                 string
	EmailPersonal         string
	Address               string
	NPWP                  string
	BPJSKesehatan         string
	BPJSKetenagakerjaan   string
	BankName              string
	BankAccountNumber     string
	BankAccountHolderName string
}

// TxRunner is a thin interface over db.TxManager (injectable for tests).
type TxRunner interface {
	InTx(ctx context.Context, fn func(tx pgx.Tx) error) error
}

// Clock is injectable for deterministic tests; defaults to time.Now.
type Clock func() time.Time

// Service implements the E2 employees business logic.
type Service struct {
	repo EmployeeRepository
	txm  TxRunner
	now  Clock
}

// NewService wires the service with its dependencies.
func NewService(repo EmployeeRepository, txm TxRunner) *Service {
	return &Service{repo: repo, txm: txm, now: time.Now}
}

// SetClock overrides the time source (tests only).
func (s *Service) SetClock(c Clock) { s.now = c }

// pageCursor is the opaque JSON payload encoded into the cursor string.
type pageCursor struct {
	CreatedAt time.Time `json:"c"`
	ID        string    `json:"i"`
}

// --- Employees ---

// ListEmployees returns a cursor-paginated page of employees.
func (s *Service) ListEmployees(ctx context.Context, f domain.EmployeeFilter) ([]domain.Employee, *string, error) {
	limit := httpx.ClampLimit(f.Limit)
	f.Limit = limit + 1 // fetch one extra to detect has_more

	// Lowercase the status filter before the query (DB stores lowercase).
	if f.Status != nil {
		lower := strings.ToLower(*f.Status)
		f.Status = &lower
	}

	rows, err := s.repo.ListEmployees(ctx, f)
	if err != nil {
		return nil, nil, apperr.Internal(err)
	}

	var nextCursor *string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		cur, err := httpx.EncodeCursor(pageCursor{CreatedAt: last.CreatedAt, ID: last.ID})
		if err != nil {
			return nil, nil, apperr.Internal(err)
		}
		nextCursor = &cur
	}

	return rows, nextCursor, nil
}

// GetEmployee returns a single employee by id.
func (s *Service) GetEmployee(ctx context.Context, id string) (domain.Employee, error) {
	emp, err := s.repo.GetEmployeeByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Employee{}, apperr.NotFound()
	}
	if err != nil {
		return domain.Employee{}, apperr.Internal(err)
	}
	return emp, nil
}

// CreateEmployee creates a new employee record.
// EP-2: pre-checks for duplicate NIK → 409 DUPLICATE_NIK.
// EP-3 (Phase-4 stub): provision_login/login_email fields are accepted but UserID
// stays NULL in this milestone; linked E1 login provisioning is deferred.
func (s *Service) CreateEmployee(ctx context.Context, p CreateEmployeeParams, actorID string) (domain.Employee, error) {
	// Required field validation.
	fields := map[string]string{}
	if strings.TrimSpace(p.FullName) == "" {
		fields["full_name"] = "Wajib diisi."
	}
	if strings.TrimSpace(p.NIK) == "" {
		fields["nik"] = "Wajib diisi."
	}
	if p.JoinAt.IsZero() {
		fields["join_at"] = "Wajib diisi."
	}
	if len(fields) > 0 {
		return domain.Employee{}, apperr.Invalid(fields)
	}

	// EP-2: duplicate NIK pre-check (before the tx to give a clean error).
	_, nikErr := s.repo.GetEmployeeByNIK(ctx, p.NIK)
	if nikErr == nil {
		// NIK already exists → 409 DUPLICATE_NIK.
		return domain.Employee{}, &apperr.Error{
			Code:       "DUPLICATE_NIK",
			HTTPStatus: 409,
			Fields:     map[string]string{"nik": "NIK sudah terdaftar untuk karyawan lain."},
		}
	}
	if !errors.Is(nikErr, domain.ErrNotFound) {
		return domain.Employee{}, apperr.Internal(nikErr)
	}

	var created domain.Employee
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		created, inErr = s.repo.CreateEmployee(ctx, tx, p)
		if inErr != nil {
			return inErr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionCreate,
			EntityType: "employee",
			EntityID:   created.ID,
			Before:     nil,
			After: map[string]any{
				"full_name": created.FullName,
				"nik":       created.NIK,
				"status":    created.Status,
			},
		})
	}); err != nil {
		return domain.Employee{}, apperr.Internal(err)
	}

	return created, nil
}

// UpdateEmployee patches a mutable employee fields.
// EP-2: if NIK changes, re-runs the duplicate check.
func (s *Service) UpdateEmployee(ctx context.Context, p UpdateEmployeeParams, actorID string) (domain.Employee, error) {
	// Load existing (404 if missing).
	current, err := s.repo.GetEmployeeByID(ctx, p.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Employee{}, apperr.NotFound()
	}
	if err != nil {
		return domain.Employee{}, apperr.Internal(err)
	}

	// EP-2: if NIK changed, re-run duplicate check.
	if p.NIK != current.NIK {
		_, nikErr := s.repo.GetEmployeeByNIK(ctx, p.NIK)
		if nikErr == nil {
			return domain.Employee{}, &apperr.Error{
				Code:       "DUPLICATE_NIK",
				HTTPStatus: 409,
				Fields:     map[string]string{"nik": "NIK sudah terdaftar untuk karyawan lain."},
			}
		}
		if !errors.Is(nikErr, domain.ErrNotFound) {
			return domain.Employee{}, apperr.Internal(nikErr)
		}
	}

	var updated domain.Employee
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		updated, inErr = s.repo.UpdateEmployee(ctx, tx, p)
		if inErr != nil {
			return inErr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionUpdate,
			EntityType: "employee",
			EntityID:   p.ID,
			Before:     map[string]any{"full_name": current.FullName, "nik": current.NIK, "status": current.Status},
			After:      map[string]any{"full_name": updated.FullName, "nik": updated.NIK},
		})
	}); err != nil {
		return domain.Employee{}, apperr.Internal(err)
	}

	return updated, nil
}

// DeactivateEmployee sets an employee to inactive.
// Returns CONFLICT (409) if already inactive.
// NOTE: disabling the linked E1 login is deferred to Phase 4 (matches the
// Phase-2 decision "Session revocation on deactivate ... deferred"). The
// employee status is set; auth session remains valid until natural expiry or
// until Phase-4 wires the revocation. See SUMMARY.md.
func (s *Service) DeactivateEmployee(ctx context.Context, id, reason string) (domain.Employee, error) {
	current, err := s.repo.GetEmployeeByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Employee{}, apperr.NotFound()
	}
	if err != nil {
		return domain.Employee{}, apperr.Internal(err)
	}
	if current.Status == "inactive" {
		return domain.Employee{}, apperr.Conflict("CONFLICT")
	}

	var updated domain.Employee
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		updated, inErr = s.repo.SetEmployeeStatus(ctx, tx, id, "inactive")
		if inErr != nil {
			return inErr
		}
		afterSnap := map[string]any{"status": "inactive"}
		if reason != "" {
			afterSnap["reason"] = reason
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("employee.deactivate"),
			EntityType: "employee",
			EntityID:   id,
			Before:     map[string]any{"status": current.Status},
			After:      afterSnap,
		})
	}); err != nil {
		return domain.Employee{}, apperr.Internal(err)
	}

	return updated, nil
}

// ReactivateEmployee sets an employee back to active.
// Returns CONFLICT (409) if already active.
func (s *Service) ReactivateEmployee(ctx context.Context, id string) (domain.Employee, error) {
	current, err := s.repo.GetEmployeeByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Employee{}, apperr.NotFound()
	}
	if err != nil {
		return domain.Employee{}, apperr.Internal(err)
	}
	if current.Status == "active" {
		return domain.Employee{}, apperr.Conflict("CONFLICT")
	}

	var updated domain.Employee
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		updated, inErr = s.repo.SetEmployeeStatus(ctx, tx, id, "active")
		if inErr != nil {
			return inErr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("employee.reactivate"),
			EntityType: "employee",
			EntityID:   id,
			Before:     map[string]any{"status": current.Status},
			After:      map[string]any{"status": "active"},
		})
	}); err != nil {
		return domain.Employee{}, apperr.Internal(err)
	}

	return updated, nil
}
