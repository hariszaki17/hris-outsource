// Package placement — unit tests for GetMyActivePlacement.
//
// Fakes implement only the PlacementRepository methods invoked on these paths;
// all unused methods panic so any unexpected call fails loudly.
package placement

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
)

// ── fake tx / runner (satisfies TxRunner; audit.Record's Exec is a no-op) ──

type fakeTx struct{}

func (fakeTx) Begin(context.Context) (pgx.Tx, error)                                     { return fakeTx{}, nil }
func (fakeTx) Commit(context.Context) error                                               { return nil }
func (fakeTx) Rollback(context.Context) error                                             { return nil }
func (fakeTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error)           { return pgconn.CommandTag{}, nil }
func (fakeTx) Query(context.Context, string, ...any) (pgx.Rows, error)                  { panic("unused") }
func (fakeTx) QueryRow(context.Context, string, ...any) pgx.Row                          { panic("unused") }
func (fakeTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	panic("unused")
}
func (fakeTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { panic("unused") }
func (fakeTx) LargeObjects() pgx.LargeObjects                         { panic("unused") }
func (fakeTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	panic("unused")
}
func (fakeTx) Conn() *pgx.Conn { return nil }

type fakeRunner struct{}

func (fakeRunner) InTx(ctx context.Context, fn func(tx pgx.Tx) error) error { return fn(fakeTx{}) }

// ── fake repo ──────────────────────────────────────────────────────────────

// fakeActivePlacementRepo is the minimal fake for GetMyActivePlacement paths.
// It controls GetActivePlacementForEmployee, GetPlacementByID and GetPlacementChain;
// every other method panics so unexpected calls fail loudly.
type fakeActivePlacementRepo struct {
	// GetActivePlacementForEmployee result
	activePlacement domain.Placement
	activeErr       error

	// GetPlacementByID result (indexed by id for the happy path)
	byIDPlacement domain.Placement
	byIDErr       error

	// GetPlacementChain result
	chain    []domain.Placement
	chainErr error
}

func (f *fakeActivePlacementRepo) GetActivePlacementForEmployee(_ context.Context, _ string) (domain.Placement, error) {
	return f.activePlacement, f.activeErr
}

func (f *fakeActivePlacementRepo) GetPlacementByID(_ context.Context, _ string) (domain.Placement, error) {
	return f.byIDPlacement, f.byIDErr
}

func (f *fakeActivePlacementRepo) GetPlacementChain(_ context.Context, _ string) ([]domain.Placement, error) {
	return f.chain, f.chainErr
}

// ── unused interface stubs (panic on call) ─────────────────────────────────

func (f *fakeActivePlacementRepo) ListPlacements(_ context.Context, _ domain.PlacementFilter) ([]domain.Placement, error) {
	panic("not expected")
}
func (f *fakeActivePlacementRepo) ListExpiringPlacements(_ context.Context, _ domain.ExpiringFilter) ([]domain.Placement, error) {
	panic("not expected")
}
func (f *fakeActivePlacementRepo) PlacementStats(_ context.Context, _ *string) (domain.PlacementStats, error) {
	panic("not expected")
}
func (f *fakeActivePlacementRepo) SearchPositions(_ context.Context, _ string) ([]string, error) {
	panic("not expected")
}
func (f *fakeActivePlacementRepo) GetEmployeeByID(_ context.Context, _ string) (domain.Employee, error) {
	panic("not expected")
}
func (f *fakeActivePlacementRepo) GetClientCompany(_ context.Context, _ string) (CompanyRef, error) {
	panic("not expected")
}
func (f *fakeActivePlacementRepo) GetSite(_ context.Context, _ string) (SiteRef, error) {
	panic("not expected")
}
func (f *fakeActivePlacementRepo) GetAgreement(_ context.Context, _ string) (AgreementRef, error) {
	panic("not expected")
}
func (f *fakeActivePlacementRepo) GetActivePlacementForEmployeeAtCompany(_ context.Context, _ pgx.Tx, _, _ string) (domain.Placement, error) {
	panic("not expected")
}
func (f *fakeActivePlacementRepo) LockEmployeePlacements(_ context.Context, _ pgx.Tx, _ string) ([]domain.Placement, error) {
	panic("not expected")
}
func (f *fakeActivePlacementRepo) CreatePlacement(_ context.Context, _ pgx.Tx, _ CreatePlacementParams) (domain.Placement, error) {
	panic("not expected")
}
func (f *fakeActivePlacementRepo) UpdatePlacementFields(_ context.Context, _ pgx.Tx, _ UpdatePlacementParams) (domain.Placement, error) {
	panic("not expected")
}
func (f *fakeActivePlacementRepo) SetPlacementAgreement(_ context.Context, _ pgx.Tx, _ SetAgreementParams) (domain.Placement, error) {
	panic("not expected")
}
func (f *fakeActivePlacementRepo) SetPlacementLifecycle(_ context.Context, _ pgx.Tx, _ SetLifecycleParams) (domain.Placement, error) {
	panic("not expected")
}
func (f *fakeActivePlacementRepo) SetPlacementSuccessor(_ context.Context, _ pgx.Tx, _ string, _ *string) error {
	panic("not expected")
}
func (f *fakeActivePlacementRepo) InsertPlacementHistory(_ context.Context, _ pgx.Tx, _ PlacementHistoryParams) error {
	panic("not expected")
}

// ── context helpers ────────────────────────────────────────────────────────

// agentCtx returns a context with an agent principal whose EmployeeID matches empID.
func agentCtx(empID string) context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{
		UserID:     "SWP-USR-99",
		EmployeeID: empID,
		Role:       auth.RoleAgent,
	})
}

// hrCtx returns an HR-admin context (no EmployeeID — drives the empty-EmployeeID test).
func hrCtx() context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{
		UserID: "SWP-USR-1",
		Role:   auth.RoleHRAdmin,
	})
}

// ── test cases ─────────────────────────────────────────────────────────────

// TestGetMyActivePlacement_NoPrincipal — bare context (no principal stored) returns NotFound.
func TestGetMyActivePlacement_NoPrincipal(t *testing.T) {
	svc := NewPlacementService(&fakeActivePlacementRepo{}, fakeRunner{})

	_, err := svc.GetMyActivePlacement(context.Background())
	assertNotFound(t, err)
}

// TestGetMyActivePlacement_EmptyEmployeeID — principal present but EmployeeID = "" returns NotFound.
func TestGetMyActivePlacement_EmptyEmployeeID(t *testing.T) {
	svc := NewPlacementService(&fakeActivePlacementRepo{}, fakeRunner{})

	_, err := svc.GetMyActivePlacement(hrCtx())
	assertNotFound(t, err)
}

// TestGetMyActivePlacement_RepoNotFound — GetActivePlacementForEmployee returns domain.ErrNotFound → service NotFound.
func TestGetMyActivePlacement_RepoNotFound(t *testing.T) {
	repo := &fakeActivePlacementRepo{
		activeErr: domain.ErrNotFound,
	}
	svc := NewPlacementService(repo, fakeRunner{})

	_, err := svc.GetMyActivePlacement(agentCtx("SWP-EMP-1"))
	assertNotFound(t, err)
}

// TestGetMyActivePlacement_RepoGenericError — unexpected repo error → apperr Internal (500).
func TestGetMyActivePlacement_RepoGenericError(t *testing.T) {
	repo := &fakeActivePlacementRepo{
		activeErr: errors.New("connection reset"),
	}
	svc := NewPlacementService(repo, fakeRunner{})

	_, err := svc.GetMyActivePlacement(agentCtx("SWP-EMP-1"))
	ae, ok := apperr.As(err)
	if !ok {
		t.Fatalf("expected *apperr.Error, got %T: %v", err, err)
	}
	if ae.Status() != 500 {
		t.Fatalf("status = %d, want 500", ae.Status())
	}
	if ae.Code != "INTERNAL" {
		t.Fatalf("code = %q, want INTERNAL", ae.Code)
	}
}

// TestGetMyActivePlacement_HappyPath — valid principal + active placement → PlacementDetail with matching ID.
func TestGetMyActivePlacement_HappyPath(t *testing.T) {
	const empID = "SWP-EMP-42"
	const placementID = "SWP-PL-7"

	pl := domain.Placement{
		ID:              placementID,
		EmployeeID:      empID,
		ClientCompanyID: "SWP-CMP-3",
		SiteID:          "SWP-SITE-1",
		LifecycleStatus: "ACTIVE",
		StartDate:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		StatusChangedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	repo := &fakeActivePlacementRepo{
		activePlacement: pl,        // returned by GetActivePlacementForEmployee
		byIDPlacement:   pl,        // returned by GetPlacementByID (delegated via GetPlacement)
		chain:           []domain.Placement{pl}, // returned by GetPlacementChain
	}
	svc := NewPlacementService(repo, fakeRunner{})

	detail, err := svc.GetMyActivePlacement(agentCtx(empID))
	if err != nil {
		t.Fatalf("GetMyActivePlacement err = %v, want nil", err)
	}
	if detail.Placement.ID != placementID {
		t.Fatalf("placement.ID = %q, want %q", detail.Placement.ID, placementID)
	}
	if len(detail.HistoryChain) != 1 {
		t.Fatalf("history chain len = %d, want 1", len(detail.HistoryChain))
	}
}

// ── assertion helpers ──────────────────────────────────────────────────────

func assertNotFound(t *testing.T, err error) {
	t.Helper()
	ae, ok := apperr.As(err)
	if !ok {
		t.Fatalf("expected *apperr.Error, got %T: %v", err, err)
	}
	if ae.Code != "NOT_FOUND" {
		t.Fatalf("code = %q, want NOT_FOUND", ae.Code)
	}
	if ae.Status() != 404 {
		t.Fatalf("status = %d, want 404", ae.Status())
	}
}
