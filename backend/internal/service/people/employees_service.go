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
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
)

// EmployeeRepository is the data dependency, defined by this consumer (Go idiom).
// The repository layer in internal/repository/people implements it over sqlc.
type EmployeeRepository interface {
	// Reads run on the pool.
	ListEmployees(ctx context.Context, f domain.EmployeeFilter) ([]domain.Employee, error)
	GetEmployeeByID(ctx context.Context, id string) (domain.Employee, error)
	GetEmployeeByNIK(ctx context.Context, nik string) (domain.Employee, error)
	UserEmailTaken(ctx context.Context, email string) (bool, error)
	UserPhoneTaken(ctx context.Context, phone string) (bool, error)
	// Writes take the active transaction.
	CreateEmployee(ctx context.Context, tx pgx.Tx, p CreateEmployeeParams) (domain.Employee, error)
	UpdateEmployee(ctx context.Context, tx pgx.Tx, p UpdateEmployeeParams) (domain.Employee, error)
	SetEmployeeStatus(ctx context.Context, tx pgx.Tx, id, status string) (domain.Employee, error)
	// EP-3 login provisioning. ProvisionLogin creates the linked agent User
	// (must_change_password=true) and returns its id; RegenerateTempPassword
	// re-issues a temp password hash for an existing login.
	ProvisionLogin(ctx context.Context, tx pgx.Tx, p ProvisionLoginRepoParams) (string, error)
	RegenerateTempPassword(ctx context.Context, tx pgx.Tx, userID, passwordHash string) error
	// F2.7: on offboard, disable the linked User + revoke its sessions (instant);
	// on reactivate, re-enable the login (sessions are not restored).
	DisableUserAndRevoke(ctx context.Context, tx pgx.Tx, userID string) error
	EnableUser(ctx context.Context, tx pgx.Tx, userID string) error
	// F2.7 OB-1 offboard cascade: in the same tx, close the active employment
	// agreement and end every non-terminal placement of the employee.
	// GetActiveAgreementForEmployee returns found=false when there is none.
	GetActiveAgreementForEmployee(ctx context.Context, employeeID string) (id string, found bool, err error)
	CloseAgreement(ctx context.Context, tx pgx.Tx, agreementID, closedReason string, closedAt time.Time) error
	EndPlacementsForEmployee(ctx context.Context, tx pgx.Tx, employeeID, lifecycleStatus, endedReason string, endedAt time.Time) ([]string, error)
}

// ProvisionLoginRepoParams carries the fields for creating an agent login + link.
// Phone is the required login identifier (D2); Email is optional.
type ProvisionLoginRepoParams struct {
	EmployeeID   string
	Phone        string
	Email        string
	PasswordHash string
	FullName     string
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
	// Login provisioning is AUTOMATIC at create (D1): a linked agent User is always
	// created in the same tx with a temporary password (show-once). The login
	// identifier is the employee Phone (required, D2); LoginEmail is an optional
	// secondary identifier.
	LoginEmail string
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
// Returns the one-time temporary password (non-nil) when a login was provisioned
// (EP-3 show-once). The caller surfaces it to the admin exactly once.
func (s *Service) CreateEmployee(ctx context.Context, p CreateEmployeeParams) (domain.Employee, *string, error) {
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
	// D2: phone is the required login identifier — every employee auto-provisions
	// a login (D1), and phone is the universal identifier (agents often lack email).
	phone := normalizePhone(p.Phone)
	if phone == "" {
		fields["phone"] = "Wajib diisi (dipakai sebagai identitas login)."
	}
	if len(fields) > 0 {
		return domain.Employee{}, nil, apperr.Invalid(fields)
	}
	p.Phone = phone

	// EP-2: duplicate NIK pre-check (before the tx to give a clean error).
	_, nikErr := s.repo.GetEmployeeByNIK(ctx, p.NIK)
	if nikErr == nil {
		return domain.Employee{}, nil, dupNIKErr()
	}
	if !errors.Is(nikErr, domain.ErrNotFound) {
		return domain.Employee{}, nil, apperr.Internal(nikErr)
	}

	// EP-2: login phone uniqueness pre-check (phone is the primary identifier).
	phoneTaken, err := s.repo.UserPhoneTaken(ctx, phone)
	if err != nil {
		return domain.Employee{}, nil, apperr.Internal(err)
	}
	if phoneTaken {
		return domain.Employee{}, nil, loginPhoneConflictErr()
	}
	// EP-2: login email uniqueness pre-check (only when an email is supplied).
	if strings.TrimSpace(p.LoginEmail) != "" {
		taken, err := s.repo.UserEmailTaken(ctx, p.LoginEmail)
		if err != nil {
			return domain.Employee{}, nil, apperr.Internal(err)
		}
		if taken {
			return domain.Employee{}, nil, loginEmailConflictErr()
		}
	}

	var created domain.Employee
	var tempPw *string
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		created, inErr = s.repo.CreateEmployee(ctx, tx, p)
		if inErr != nil {
			return inErr
		}
		if err := audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionCreate,
			EntityType: "employee",
			EntityID:   created.ID,
			Before:     nil,
			After: map[string]any{
				"full_name": created.FullName,
				"nik":       created.NIK,
				"status":    created.Status,
			},
		}); err != nil {
			return err
		}

		// D1: every employee auto-provisions a login in the same tx.
		plain, userID, err := s.provisionInTx(ctx, tx, created, p.Phone, p.LoginEmail)
		if err != nil {
			return err
		}
		uid := userID
		created.UserID = &uid
		created.HasLogin = true
		tempPw = &plain
		return nil
	}); err != nil {
		return domain.Employee{}, nil, wrapTxErr(err)
	}

	return created, tempPw, nil
}

// RegenerateTempPassword re-issues a temp password for an employee that already has
// a login (show-once); forces a rotation on next login (EP-3).
func (s *Service) RegenerateTempPassword(ctx context.Context, employeeID string) (string, error) {
	emp, err := s.repo.GetEmployeeByID(ctx, employeeID)
	if errors.Is(err, domain.ErrNotFound) {
		return "", apperr.NotFound()
	}
	if err != nil {
		return "", apperr.Internal(err)
	}
	if emp.UserID == nil {
		// No login to regenerate.
		return "", apperr.Conflict("CONFLICT")
	}
	plain, hash, err := newTempPassword()
	if err != nil {
		return "", apperr.Internal(err)
	}
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		if err := s.repo.RegenerateTempPassword(ctx, tx, *emp.UserID, hash); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("employee.regenerate_password"),
			EntityType: "employee",
			EntityID:   emp.ID,
			After:      map[string]any{"user_id": *emp.UserID},
		})
	}); err != nil {
		return "", apperr.Internal(err)
	}
	return plain, nil
}

// provisionInTx creates the agent login + link + audit inside an existing tx and
// returns the one-time plaintext temp password and the new user id. Phone is the
// required login identifier (D2); loginEmail is optional.
func (s *Service) provisionInTx(ctx context.Context, tx pgx.Tx, emp domain.Employee, phone, loginEmail string) (string, string, error) {
	plain, hash, err := newTempPassword()
	if err != nil {
		return "", "", err
	}
	userID, err := s.repo.ProvisionLogin(ctx, tx, ProvisionLoginRepoParams{
		EmployeeID:   emp.ID,
		Phone:        phone,
		Email:        loginEmail,
		PasswordHash: hash,
		FullName:     emp.FullName,
	})
	if err != nil {
		return "", "", err
	}
	if err := audit.Record(ctx, tx, audit.Entry{
		Action:     audit.Action("employee.provision_login"),
		EntityType: "employee",
		EntityID:   emp.ID,
		After:      map[string]any{"user_id": userID, "phone": phone, "login_email": loginEmail, "role": "agent"},
	}); err != nil {
		return "", "", err
	}
	return plain, userID, nil
}

// normalizePhone canonicalizes an Indonesian phone number to E.164 (+62…). It
// strips spaces/dashes, converts a leading 0 to +62, and a leading 62 to +62.
// Returns "" for empty/blank input (caller treats that as missing).
func normalizePhone(raw string) string {
	s := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '-', '(', ')', '.':
			return -1
		}
		return r
	}, strings.TrimSpace(raw))
	if s == "" {
		return ""
	}
	switch {
	case strings.HasPrefix(s, "+"):
		return s
	case strings.HasPrefix(s, "0"):
		return "+62" + s[1:]
	case strings.HasPrefix(s, "62"):
		return "+" + s
	default:
		return "+62" + s
	}
}

func newTempPassword() (plain, hash string, err error) {
	plain, err = auth.GenerateTempPassword()
	if err != nil {
		return "", "", err
	}
	hash, err = auth.HashPassword(plain)
	if err != nil {
		return "", "", err
	}
	return plain, hash, nil
}

func dupNIKErr() *apperr.Error {
	return &apperr.Error{
		Code:       "DUPLICATE_NIK",
		HTTPStatus: 409,
		Fields:     map[string]string{"nik": "NIK sudah terdaftar untuk karyawan lain."},
	}
}

func loginEmailConflictErr() *apperr.Error {
	return &apperr.Error{
		Code:       "CONFLICT",
		HTTPStatus: 409,
		Fields:     map[string]string{"login_email": "Email login sudah digunakan."},
	}
}

func loginPhoneConflictErr() *apperr.Error {
	return &apperr.Error{
		Code:       "CONFLICT",
		HTTPStatus: 409,
		Fields:     map[string]string{"phone": "Nomor telepon sudah digunakan untuk login lain."},
	}
}

// wrapTxErr passes through domain apperr (e.g. CONFLICT from a unique-email race)
// and wraps anything else as Internal.
func wrapTxErr(err error) error {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae
	}
	return apperr.Internal(err)
}

// UpdateEmployee patches a mutable employee fields.
// EP-2: if NIK changes, re-runs the duplicate check.
func (s *Service) UpdateEmployee(ctx context.Context, p UpdateEmployeeParams) (domain.Employee, error) {
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

// OffboardReason is the required offboard reason enum (F2.7 OB-1). It maps 1:1 to
// agreement.closed_reason and is translated to the placement terminal pair.
const (
	offboardReasonResigned  = "RESIGNED"
	offboardReasonTerminated = "TERMINATED"
	offboardReasonEndOfTerm = "END_OF_TERM"
	offboardReasonOther     = "OTHER"
)

// placementTerminalFor maps an offboard reason to the placement terminal pair
// (lifecycle_status, ended_reason) per F2.7 OB-1. The agreement closed_reason is
// always the reason value itself (same enum domain).
func placementTerminalFor(reason string) (lifecycleStatus, endedReason string) {
	switch reason {
	case offboardReasonResigned:
		return "RESIGNED", "RESIGNED"
	case offboardReasonTerminated:
		return "TERMINATED", "TERMINATED"
	case offboardReasonEndOfTerm:
		return "ENDED", "END_OF_TERM"
	default: // OTHER
		return "ENDED", "ENDED"
	}
}

// DeactivateEmployee offboards an employee (F2.7 OB-1): in ONE transaction it
// sets the employee inactive, closes the active employment agreement, ends every
// non-terminal placement, and disables+revokes the linked login. reason is the
// REQUIRED offboard enum (RESIGNED|TERMINATED|END_OF_TERM|OTHER); note is an
// optional free-text. Returns CONFLICT (409) if already inactive.
//
// Traceability: the cascaded agreement/placement audit entries carry the marker
// keys caused_by="employee_offboard" + source_employee_id, distinguishing them
// from direct closes (OB-3). The parent "employee.offboard" entry lists what it
// cascaded.
func (s *Service) DeactivateEmployee(ctx context.Context, id, reason, note string) (domain.Employee, error) {
	switch reason {
	case offboardReasonResigned, offboardReasonTerminated, offboardReasonEndOfTerm, offboardReasonOther:
		// valid
	default:
		return domain.Employee{}, apperr.Invalid(map[string]string{
			"reason": "Wajib diisi salah satu: RESIGNED, TERMINATED, END_OF_TERM, OTHER.",
		})
	}

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

	now := s.now()
	lifecycleStatus, endedReason := placementTerminalFor(reason)

	var updated domain.Employee
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		updated, inErr = s.repo.SetEmployeeStatus(ctx, tx, id, "inactive")
		if inErr != nil {
			return inErr
		}

		// OB-1: close the active employment agreement (if any). closed_reason mirrors
		// the offboard reason. A SEPARATE audit entry records the cascade with the
		// traceability marker so it is distinguishable from a direct agreement close.
		agreementID, found, inErr := s.repo.GetActiveAgreementForEmployee(ctx, id)
		if inErr != nil {
			return inErr
		}
		var cascadedAgreementID any // null in the parent entry when no active agreement
		if found {
			if inErr = s.repo.CloseAgreement(ctx, tx, agreementID, reason, now); inErr != nil {
				return inErr
			}
			cascadedAgreementID = agreementID
			if inErr = audit.Record(ctx, tx, audit.Entry{
				Action:     audit.Action("agreement.close"),
				EntityType: "employment_agreement",
				EntityID:   agreementID,
				After: map[string]any{
					"status":             "closed",
					"closed_reason":      reason,
					"caused_by":          "employee_offboard",
					"source_employee_id": id,
				},
			}); inErr != nil {
				return inErr
			}
		}

		// OB-1: end every non-terminal placement. One audit entry per ended placement,
		// each carrying the traceability marker.
		placementIDs, inErr := s.repo.EndPlacementsForEmployee(ctx, tx, id, lifecycleStatus, endedReason, now)
		if inErr != nil {
			return inErr
		}
		for _, pid := range placementIDs {
			if inErr = audit.Record(ctx, tx, audit.Entry{
				Action:     audit.Action("placement.end"),
				EntityType: "placement",
				EntityID:   pid,
				After: map[string]any{
					"lifecycle_status":   lifecycleStatus,
					"ended_reason":       endedReason,
					"caused_by":          "employee_offboard",
					"source_employee_id": id,
				},
			}); inErr != nil {
				return inErr
			}
		}

		// F2.7: cascade to the linked login — disable it and revoke every session
		// (bump epoch + refresh tokens), so access dies immediately.
		if current.UserID != nil {
			if inErr = s.repo.DisableUserAndRevoke(ctx, tx, *current.UserID); inErr != nil {
				return inErr
			}
		}

		// Parent offboard entry: lists what the cascade closed/ended.
		afterSnap := map[string]any{
			"status":                  "inactive",
			"reason":                  reason,
			"cascaded_agreement_id":   cascadedAgreementID,
			"cascaded_placement_ids":  placementIDs,
		}
		if note != "" {
			afterSnap["note"] = note
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.Action("employee.offboard"),
			EntityType: "employee",
			EntityID:   id,
			Before:     map[string]any{"status": current.Status},
			After:      afterSnap,
		})
	}); err != nil {
		return domain.Employee{}, wrapTxErr(err)
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
		// F2.7: re-enable the linked login (sessions are NOT restored — the agent
		// signs in fresh).
		if current.UserID != nil {
			if inErr = s.repo.EnableUser(ctx, tx, *current.UserID); inErr != nil {
				return inErr
			}
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
