// Package people — white-box unit tests for ChangeRequestService, focused on the
// 2026-06-11 redesign: the emergency-contact tier (address no longer accepted),
// the shift-leader bank-split partial-apply flow, the CanApproveBank gate, the
// GuardCompany company-scope routing, and the un-stubbed notification dispatch.
//
// In-package (white-box) so the service can be constructed directly with an
// in-memory repo, a no-op tx runner, and a fake jobs.Dispatcher that captures the
// notification kind/reason. No HTTP, no Postgres, no River.
package people

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/jobs"
)

// errCode extracts the apperr.Error machine code from a service error ("" if not
// an *apperr.Error).
func errCode(err error) string {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

// crTx is a minimal pgx.Tx whose Exec is a no-op (audit.Record calls Exec).
type crTx struct{}

func (crTx) Begin(context.Context) (pgx.Tx, error)  { return crTx{}, nil }
func (crTx) Commit(context.Context) error           { return nil }
func (crTx) Rollback(context.Context) error         { return nil }
func (crTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (crTx) Query(context.Context, string, ...any) (pgx.Rows, error) { panic("Query") }
func (crTx) QueryRow(context.Context, string, ...any) pgx.Row        { panic("QueryRow") }
func (crTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	panic("CopyFrom")
}
func (crTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { panic("SendBatch") }
func (crTx) LargeObjects() pgx.LargeObjects                         { panic("LargeObjects") }
func (crTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	panic("Prepare")
}
func (crTx) Conn() *pgx.Conn { return nil }

// crTxRunner runs the body with a no-op tx (commit-on-success semantics implicit).
type crTxRunner struct{}

func (crTxRunner) InTx(ctx context.Context, fn func(pgx.Tx) error) error { return fn(crTx{}) }

// crRepo is an in-memory ChangeRequestRepository.
type crRepo struct {
	crs       map[string]domain.ChangeRequest
	employees map[string]domain.Employee
	companyOf map[string]string // employeeID → current client company id ("" = none)
}

func newCRRepo() *crRepo {
	return &crRepo{
		crs:       map[string]domain.ChangeRequest{},
		employees: map[string]domain.Employee{},
		companyOf: map[string]string{},
	}
}

func (r *crRepo) ListChangeRequests(_ context.Context, _ domain.ChangeRequestFilter) ([]domain.ChangeRequest, error) {
	out := make([]domain.ChangeRequest, 0, len(r.crs))
	for _, cr := range r.crs {
		out = append(out, cr)
	}
	return out, nil
}

func (r *crRepo) GetChangeRequestByID(_ context.Context, id string) (domain.ChangeRequest, error) {
	cr, ok := r.crs[id]
	if !ok {
		return domain.ChangeRequest{}, domain.ErrNotFound
	}
	return cr, nil
}

func (r *crRepo) GetEmployeeByID(_ context.Context, id string) (domain.Employee, error) {
	e, ok := r.employees[id]
	if !ok {
		return domain.Employee{}, domain.ErrNotFound
	}
	return e, nil
}

func (r *crRepo) GetEmployeeCompanyID(_ context.Context, employeeID string) (string, error) {
	return r.companyOf[employeeID], nil
}

func (r *crRepo) UpdateEmployee(_ context.Context, _ pgx.Tx, p UpdateEmployeeParams) (domain.Employee, error) {
	e, ok := r.employees[p.ID]
	if !ok {
		return domain.Employee{}, domain.ErrNotFound
	}
	if p.Phone != "" {
		ph := p.Phone
		e.Phone = &ph
	} else {
		e.Phone = nil
	}
	e.BankAccount = domain.BankAccount{
		BankName:          p.BankName,
		AccountNumber:     p.BankAccountNumber,
		AccountHolderName: p.BankAccountHolderName,
	}
	e.EmergencyContact = domain.EmergencyContact{
		Name:  p.EmergencyContactName,
		Phone: p.EmergencyContactPhone,
	}
	e.UpdatedAt = time.Now().UTC()
	r.employees[p.ID] = e
	return e, nil
}

func (r *crRepo) ResolveChangeRequest(_ context.Context, _ pgx.Tx, p ResolveChangeRequestParams) (domain.ChangeRequest, error) {
	cr, ok := r.crs[p.ID]
	if !ok {
		return domain.ChangeRequest{}, domain.ErrNotFound
	}
	cr.Status = p.Status
	cr.FieldResolutions = p.FieldResolutions
	cr.BankPending = p.BankPending
	cr.ResolvedAt = p.ResolvedAt
	cr.ResolvedBy = p.ResolvedBy
	cr.RejectionReason = p.RejectionReason
	r.crs[p.ID] = cr
	return cr, nil
}

func (r *crRepo) CreateChangeRequest(_ context.Context, _ pgx.Tx, p CreateChangeRequestParams) (domain.ChangeRequest, error) {
	cr := domain.ChangeRequest{
		ID:          "SWP-CHG-" + p.EmployeeID,
		EmployeeID:  p.EmployeeID,
		Status:      "pending",
		RequestType: p.RequestType,
		Changes:     p.Changes,
		Note:        p.Note,
		SubmittedAt: time.Now().UTC(),
	}
	r.crs[cr.ID] = cr
	return cr, nil
}

var _ ChangeRequestRepository = (*crRepo)(nil)

// captureDispatcher records every NotificationArgs dispatched (kind + reason).
type captureDispatcher struct {
	calls []jobs.NotificationArgs
}

func (d *captureDispatcher) Dispatch(_ context.Context, _ pgx.Tx, a jobs.NotificationArgs) error {
	d.calls = append(d.calls, a)
	return nil
}

var _ jobs.Dispatcher = (*captureDispatcher)(nil)

// ---------------------------------------------------------------------------
// Fixtures + helpers
// ---------------------------------------------------------------------------

const (
	crCompanyA = "SWP-CMP-A"
	crCompanyB = "SWP-CMP-B"
)

func newCRService(t *testing.T) (*ChangeRequestService, *crRepo, *captureDispatcher) {
	t.Helper()
	repo := newCRRepo()
	svc := NewChangeRequestService(repo, crTxRunner{})
	disp := &captureDispatcher{}
	svc.SetNotifier(disp)
	return svc, repo, disp
}

func seedEmp(repo *crRepo, id, phone, companyID string) {
	ph := phone
	repo.employees[id] = domain.Employee{
		ID:          id,
		FullName:    "Emp " + id,
		NIK:         "NIK-" + id,
		NIP:         "NIP-" + id,
		JoinAt:      time.Now().UTC(),
		Status:      "active",
		Phone:       &ph,
		BankAccount: domain.BankAccount{BankName: "BNI", AccountNumber: "111", AccountHolderName: "Emp"},
	}
	repo.companyOf[id] = companyID
}

func seedCR(repo *crRepo, id, empID, reqType string, changes domain.ChangeRequestChanges) {
	repo.crs[id] = domain.ChangeRequest{
		ID:          id,
		EmployeeID:  empID,
		Status:      "pending",
		RequestType: reqType,
		Changes:     changes,
		SubmittedAt: time.Now().UTC(),
	}
}

func hrCtx() context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{
		UserID: "SWP-USR-HR", EmployeeID: "SWP-EMP-HR", Role: auth.RoleHRAdmin,
	})
}

func slCtx(companyID string) context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{
		UserID: "SWP-USR-SL", EmployeeID: "SWP-EMP-SL", Role: auth.RoleShiftLeader, CompanyID: companyID,
	})
}

func ptr(s string) *string { return &s }

// ---------------------------------------------------------------------------
// deriveRequestType — emergency-contact tier (address no longer a field)
// ---------------------------------------------------------------------------

func TestDeriveRequestType_EmergencyContact(t *testing.T) {
	got, err := deriveRequestType(domain.ChangeRequestChanges{
		EmergencyContact: &domain.EmergencyContact{Name: "Budi", Phone: "+62811"},
	})
	if err != nil {
		t.Fatalf("deriveRequestType err = %v", err)
	}
	if got != "EMERGENCY_CONTACT" {
		t.Errorf("request_type = %q, want EMERGENCY_CONTACT", got)
	}
}

func TestDeriveRequestType_MultipleAndEmpty(t *testing.T) {
	got, err := deriveRequestType(domain.ChangeRequestChanges{
		Phone:       ptr("+62811"),
		BankAccount: &domain.BankAccount{BankName: "BCA", AccountNumber: "9", AccountHolderName: "x"},
	})
	if err != nil || got != "MULTIPLE" {
		t.Fatalf("request_type = %q err = %v, want MULTIPLE", got, err)
	}
	// An empty change set is a 422 (no address field exists to smuggle in).
	if _, err := deriveRequestType(domain.ChangeRequestChanges{}); err == nil {
		t.Error("deriveRequestType(empty) = nil err, want INVALID")
	}
}

// CreateChangeRequest no longer accepts address: the domain struct has no Address
// field, so a phone-only request derives PHONE and an emergency-only derives
// EMERGENCY_CONTACT — proving address is structurally impossible.
func TestCreateChangeRequest_EmergencyOnly(t *testing.T) {
	svc, repo, _ := newCRService(t)
	seedEmp(repo, "SWP-EMP-1", "+62800", crCompanyA)
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{
		UserID: "SWP-USR-1", EmployeeID: "SWP-EMP-1", Role: auth.RoleAgent,
	})

	cr, err := svc.CreateChangeRequest(ctx, "SWP-EMP-1", domain.ChangeRequestChanges{
		EmergencyContact: &domain.EmergencyContact{Name: "Siti", Phone: "+62822"},
	}, nil)
	if err != nil {
		t.Fatalf("CreateChangeRequest err = %v", err)
	}
	if cr.RequestType != "EMERGENCY_CONTACT" {
		t.Errorf("request_type = %q, want EMERGENCY_CONTACT", cr.RequestType)
	}
}

// ---------------------------------------------------------------------------
// Approve — emergency-contact derive/apply (HR)
// ---------------------------------------------------------------------------

func TestApprove_EmergencyContact_Applied(t *testing.T) {
	svc, repo, disp := newCRService(t)
	seedEmp(repo, "SWP-EMP-EC", "+62800", crCompanyA)
	seedCR(repo, "SWP-CR-EC", "SWP-EMP-EC", "EMERGENCY_CONTACT", domain.ChangeRequestChanges{
		EmergencyContact: &domain.EmergencyContact{Name: "Rina", Phone: "+62899"},
	})

	cr, err := svc.ApproveChangeRequest(hrCtx(), "SWP-CR-EC", "SWP-USR-HR", "SWP-EMP-HR")
	if err != nil {
		t.Fatalf("Approve err = %v", err)
	}
	if cr.Status != "approved" {
		t.Errorf("status = %q, want approved", cr.Status)
	}
	emp := repo.employees["SWP-EMP-EC"]
	if emp.EmergencyContact.Name != "Rina" || emp.EmergencyContact.Phone != "+62899" {
		t.Errorf("emergency contact not applied: %+v", emp.EmergencyContact)
	}
	// Exactly one approval notification, of the approved kind.
	if len(disp.calls) != 1 {
		t.Fatalf("dispatch calls = %d, want 1", len(disp.calls))
	}
	if disp.calls[0].NotifKind != "CHANGE_REQUEST_APPROVED" {
		t.Errorf("notif kind = %q, want CHANGE_REQUEST_APPROVED", disp.calls[0].NotifKind)
	}
	if disp.calls[0].RecipientID != "SWP-EMP-EC" {
		t.Errorf("notif recipient = %q, want the submitter SWP-EMP-EC", disp.calls[0].RecipientID)
	}
}

// ---------------------------------------------------------------------------
// Bank-split: SL approves a MIXED (phone + bank) request
// ---------------------------------------------------------------------------

func TestApprove_SL_Mixed_NonBankAppliedBankEscalated(t *testing.T) {
	svc, repo, disp := newCRService(t)
	seedEmp(repo, "SWP-EMP-MX", "+62800", crCompanyA)
	seedCR(repo, "SWP-CR-MX", "SWP-EMP-MX", "MULTIPLE", domain.ChangeRequestChanges{
		Phone:       ptr("+62811222333"),
		BankAccount: &domain.BankAccount{BankName: "BCA", AccountNumber: "9000", AccountHolderName: "Emp MX"},
	})

	cr, err := svc.ApproveChangeRequest(slCtx(crCompanyA), "SWP-CR-MX", "SWP-USR-SL", "SWP-EMP-SL")
	if err != nil {
		t.Fatalf("SL approve mixed err = %v", err)
	}

	// Status is partially_approved with bank_pending set.
	if cr.Status != "partially_approved" {
		t.Errorf("status = %q, want partially_approved", cr.Status)
	}
	if !cr.BankPending {
		t.Error("bank_pending = false, want true after SL escalates bank")
	}

	// Non-bank field (phone) applied to the employee; bank NOT applied yet.
	emp := repo.employees["SWP-EMP-MX"]
	if emp.Phone == nil || *emp.Phone != "+62811222333" {
		t.Errorf("phone = %v, want applied +62811222333", emp.Phone)
	}
	if emp.BankAccount.AccountNumber == "9000" {
		t.Error("bank account applied by SL — must be escalated, not applied")
	}

	// Per-field resolutions: phone applied (by SL), bank escalated_to_hr.
	if fr := cr.FieldResolutions["phone"]; fr.Status != resolutionApplied || fr.ResolvedBy != "SWP-EMP-SL" {
		t.Errorf("phone resolution = %+v, want applied by SWP-EMP-SL", fr)
	}
	if fr := cr.FieldResolutions["bank_account"]; fr.Status != resolutionEscalated {
		t.Errorf("bank resolution = %+v, want escalated_to_hr", fr)
	}

	// Not finally resolved: resolved_at/by stay nil on a partial.
	if cr.ResolvedAt != nil || cr.ResolvedBy != nil {
		t.Errorf("partial resolve set resolved_at/by: %v %v", cr.ResolvedAt, cr.ResolvedBy)
	}

	// One partial-approval notification dispatched.
	if len(disp.calls) != 1 || disp.calls[0].NotifKind != "CHANGE_REQUEST_APPROVED" {
		t.Fatalf("notif calls = %+v, want one CHANGE_REQUEST_APPROVED", disp.calls)
	}
}

// HR second-approve of the partially_approved request finalizes the bank field.
func TestApprove_HR_FinalizesPartial(t *testing.T) {
	svc, repo, disp := newCRService(t)
	seedEmp(repo, "SWP-EMP-FN", "+62811222333", crCompanyA) // phone already applied by SL
	// Seed the request already partially approved (phone applied, bank escalated).
	now := time.Now().UTC()
	repo.crs["SWP-CR-FN"] = domain.ChangeRequest{
		ID:          "SWP-CR-FN",
		EmployeeID:  "SWP-EMP-FN",
		Status:      "partially_approved",
		RequestType: "MULTIPLE",
		BankPending: true,
		Changes: domain.ChangeRequestChanges{
			Phone:       ptr("+62811222333"),
			BankAccount: &domain.BankAccount{BankName: "BCA", AccountNumber: "9000", AccountHolderName: "Emp FN"},
		},
		FieldResolutions: map[string]domain.FieldResolution{
			"phone":        {Status: resolutionApplied, ResolvedBy: "SWP-EMP-SL", ResolvedAt: &now},
			"bank_account": {Status: resolutionEscalated, ResolvedBy: "SWP-EMP-SL", ResolvedAt: &now},
		},
		SubmittedAt: now,
	}

	cr, err := svc.ApproveChangeRequest(hrCtx(), "SWP-CR-FN", "SWP-USR-HR", "SWP-EMP-HR")
	if err != nil {
		t.Fatalf("HR finalize err = %v", err)
	}
	if cr.Status != "approved" {
		t.Errorf("status = %q, want approved (finalized)", cr.Status)
	}
	if cr.BankPending {
		t.Error("bank_pending still true after HR finalize")
	}
	// Bank now applied to the employee.
	emp := repo.employees["SWP-EMP-FN"]
	if emp.BankAccount.AccountNumber != "9000" {
		t.Errorf("bank account = %q, want finalized 9000", emp.BankAccount.AccountNumber)
	}
	if cr.ResolvedAt == nil || cr.ResolvedBy == nil || *cr.ResolvedBy != "SWP-USR-HR" {
		t.Errorf("finalize did not stamp resolved_at/by: %v %v", cr.ResolvedAt, cr.ResolvedBy)
	}
	if len(disp.calls) != 1 || disp.calls[0].NotifKind != "CHANGE_REQUEST_APPROVED" {
		t.Errorf("notif on finalize = %+v, want one CHANGE_REQUEST_APPROVED", disp.calls)
	}
}

// A bank-ONLY request approved by an SL escalates without touching the employee
// at all (no applicable field → no UpdateEmployee write).
func TestApprove_SL_BankOnly_EscalatesNotApplied(t *testing.T) {
	svc, repo, disp := newCRService(t)
	seedEmp(repo, "SWP-EMP-BK", "+62800", crCompanyA)
	seedCR(repo, "SWP-CR-BK", "SWP-EMP-BK", "BANK_ACCOUNT", domain.ChangeRequestChanges{
		BankAccount: &domain.BankAccount{BankName: "Mandiri", AccountNumber: "7777", AccountHolderName: "Emp BK"},
	})

	cr, err := svc.ApproveChangeRequest(slCtx(crCompanyA), "SWP-CR-BK", "SWP-USR-SL", "SWP-EMP-SL")
	if err != nil {
		t.Fatalf("SL bank-only approve err = %v", err)
	}
	if cr.Status != "partially_approved" || !cr.BankPending {
		t.Errorf("status/bank_pending = %q/%v, want partially_approved/true", cr.Status, cr.BankPending)
	}
	// Bank NOT applied (still the seeded BNI/111).
	emp := repo.employees["SWP-EMP-BK"]
	if emp.BankAccount.AccountNumber != "111" {
		t.Errorf("bank applied by SL on bank-only: %q, want unchanged 111", emp.BankAccount.AccountNumber)
	}
	if fr := cr.FieldResolutions["bank_account"]; fr.Status != resolutionEscalated {
		t.Errorf("bank resolution = %+v, want escalated_to_hr", fr)
	}
	if len(disp.calls) != 1 {
		t.Errorf("notif calls = %d, want 1", len(disp.calls))
	}
}

// An HR approving a bank-containing request applies it directly (CanApproveBank
// true) — no escalation, status APPROVED.
func TestApprove_HR_BankApprovedDirectly(t *testing.T) {
	svc, repo, _ := newCRService(t)
	seedEmp(repo, "SWP-EMP-HRBK", "+62800", crCompanyA)
	seedCR(repo, "SWP-CR-HRBK", "SWP-EMP-HRBK", "BANK_ACCOUNT", domain.ChangeRequestChanges{
		BankAccount: &domain.BankAccount{BankName: "CIMB", AccountNumber: "5555", AccountHolderName: "Emp"},
	})

	cr, err := svc.ApproveChangeRequest(hrCtx(), "SWP-CR-HRBK", "SWP-USR-HR", "SWP-EMP-HR")
	if err != nil {
		t.Fatalf("HR bank approve err = %v", err)
	}
	if cr.Status != "approved" || cr.BankPending {
		t.Errorf("status/bank_pending = %q/%v, want approved/false", cr.Status, cr.BankPending)
	}
	if repo.employees["SWP-EMP-HRBK"].BankAccount.AccountNumber != "5555" {
		t.Error("HR bank approve did not apply the bank account")
	}
}

// ---------------------------------------------------------------------------
// GuardCompany — SL in / out of company scope
// ---------------------------------------------------------------------------

func TestApprove_SL_OutOfCompany_403(t *testing.T) {
	svc, repo, _ := newCRService(t)
	seedEmp(repo, "SWP-EMP-OOS", "+62800", crCompanyB) // employee in company B
	seedCR(repo, "SWP-CR-OOS", "SWP-EMP-OOS", "PHONE", domain.ChangeRequestChanges{Phone: ptr("+62888")})

	// SL scoped to company A may not touch a company-B employee's request.
	_, err := svc.ApproveChangeRequest(slCtx(crCompanyA), "SWP-CR-OOS", "SWP-USR-SL", "SWP-EMP-SL")
	if err == nil {
		t.Fatal("SL out-of-company approve = nil err, want OUT_OF_SCOPE")
	}
	if code := errCode(err); code != "OUT_OF_SCOPE" {
		t.Errorf("err code = %q, want OUT_OF_SCOPE", code)
	}
	// Employee untouched.
	if p := repo.employees["SWP-EMP-OOS"].Phone; p != nil && *p == "+62888" {
		t.Error("phone applied despite out-of-scope rejection")
	}
}

func TestApprove_SL_InCompany_NonBank_OK(t *testing.T) {
	svc, repo, _ := newCRService(t)
	seedEmp(repo, "SWP-EMP-IN", "+62800", crCompanyA)
	seedCR(repo, "SWP-CR-IN", "SWP-EMP-IN", "PHONE", domain.ChangeRequestChanges{Phone: ptr("+62812345")})

	cr, err := svc.ApproveChangeRequest(slCtx(crCompanyA), "SWP-CR-IN", "SWP-USR-SL", "SWP-EMP-SL")
	if err != nil {
		t.Fatalf("SL in-company phone approve err = %v", err)
	}
	// Pure non-bank request → fully approved (no escalation).
	if cr.Status != "approved" || cr.BankPending {
		t.Errorf("status/bank_pending = %q/%v, want approved/false", cr.Status, cr.BankPending)
	}
	if p := repo.employees["SWP-EMP-IN"].Phone; p == nil || *p != "+62812345" {
		t.Errorf("phone = %v, want applied", p)
	}
}

func TestReject_SL_OutOfCompany_403(t *testing.T) {
	svc, repo, _ := newCRService(t)
	seedEmp(repo, "SWP-EMP-RJ", "+62800", crCompanyB)
	seedCR(repo, "SWP-CR-RJ", "SWP-EMP-RJ", "PHONE", domain.ChangeRequestChanges{Phone: ptr("+62888")})

	_, err := svc.RejectChangeRequest(slCtx(crCompanyA), "SWP-CR-RJ", "Tidak valid.", "SWP-USR-SL")
	if code := errCode(err); code != "OUT_OF_SCOPE" {
		t.Errorf("reject out-of-company code = %q, want OUT_OF_SCOPE", code)
	}
}

// ---------------------------------------------------------------------------
// Reject — notification carries the reason
// ---------------------------------------------------------------------------

func TestReject_NotifiesWithReason(t *testing.T) {
	svc, repo, disp := newCRService(t)
	seedEmp(repo, "SWP-EMP-RN", "+62800", crCompanyA)
	seedCR(repo, "SWP-CR-RN", "SWP-EMP-RN", "PHONE", domain.ChangeRequestChanges{Phone: ptr("+62888")})

	reason := "Dokumen pendukung tidak sesuai."
	cr, err := svc.RejectChangeRequest(hrCtx(), "SWP-CR-RN", reason, "SWP-USR-HR")
	if err != nil {
		t.Fatalf("Reject err = %v", err)
	}
	if cr.Status != "rejected" || cr.RejectionReason == nil || *cr.RejectionReason != reason {
		t.Errorf("reject result = %+v, want rejected with reason", cr)
	}
	if len(disp.calls) != 1 {
		t.Fatalf("dispatch calls = %d, want 1", len(disp.calls))
	}
	got := disp.calls[0]
	if got.NotifKind != "CHANGE_REQUEST_REJECTED" {
		t.Errorf("notif kind = %q, want CHANGE_REQUEST_REJECTED", got.NotifKind)
	}
	// The reason must be carried in the notification body (un-stubbed dispatch).
	if !contains(got.Body, reason) {
		t.Errorf("notif body = %q, want it to carry the reason %q", got.Body, reason)
	}
}

// ---------------------------------------------------------------------------
// Already-resolved guard
// ---------------------------------------------------------------------------

func TestApprove_AlreadyApproved_Conflict(t *testing.T) {
	svc, repo, _ := newCRService(t)
	seedEmp(repo, "SWP-EMP-DUP", "+62800", crCompanyA)
	now := time.Now().UTC()
	actor := "SWP-USR-HR"
	repo.crs["SWP-CR-DUP"] = domain.ChangeRequest{
		ID: "SWP-CR-DUP", EmployeeID: "SWP-EMP-DUP", Status: "approved",
		RequestType: "PHONE", ResolvedAt: &now, ResolvedBy: &actor, SubmittedAt: now,
	}
	_, err := svc.ApproveChangeRequest(hrCtx(), "SWP-CR-DUP", "SWP-USR-HR", "SWP-EMP-HR")
	if code := errCode(err); code != "CONFLICT" {
		t.Errorf("re-approve code = %q, want CONFLICT", code)
	}
}

// ---------------------------------------------------------------------------
// tiny helpers
// ---------------------------------------------------------------------------

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
