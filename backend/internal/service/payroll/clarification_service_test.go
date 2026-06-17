// Package payroll — F8.5 ClarificationService unit tests (PC-9, PC-13): target
// resolution (SL else agent, C-PC-6), the round-trip (raise -> answer -> resolve),
// answer scope-check (PC-15), and the post-lock adjustment emit (PC-9). Fakes
// implement the clarification repo + target resolver; the fake tx no-ops audit.
package payroll

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

// --- fake clarification repo ---

type fakeClarRepo struct {
	created    CreateClarificationParams
	row        dom.ClarificationRequest
	getRow     dom.ClarificationRequest
	answerOK   bool
	setOK      bool
	adjCreated *CreateAdjustmentParams
}

func (f *fakeClarRepo) CreateClarification(_ context.Context, _ pgx.Tx, p CreateClarificationParams) (dom.ClarificationRequest, error) {
	f.created = p
	f.row = dom.ClarificationRequest{
		ID: "SWP-CLR-1", SourceType: dom.ClarificationSourceType(p.SourceType), SourceID: p.SourceID,
		TargetEmployeeID: p.TargetEmployeeID, Question: p.Question, Status: dom.ClarificationStatusOpen,
	}
	return f.row, nil
}

func (f *fakeClarRepo) GetClarification(_ context.Context, _ string) (dom.ClarificationRequest, error) {
	return f.getRow, nil
}

func (f *fakeClarRepo) GetClarificationForUpdate(_ context.Context, _ pgx.Tx, _ string) (dom.ClarificationRequest, error) {
	return f.getRow, nil
}

func (f *fakeClarRepo) Answer(_ context.Context, _ pgx.Tx, id, answer string, by *string) (dom.ClarificationRequest, bool, error) {
	r := f.getRow
	r.Status = dom.ClarificationStatusAnswered
	r.Answer = &answer
	r.AnsweredBy = by
	return r, f.answerOK, nil
}

func (f *fakeClarRepo) SetStatus(_ context.Context, _ pgx.Tx, id, status string) (dom.ClarificationRequest, bool, error) {
	r := f.getRow
	r.Status = dom.ClarificationStatus(status)
	return r, f.setOK, nil
}

func (f *fakeClarRepo) CreateAdjustment(_ context.Context, _ pgx.Tx, p CreateAdjustmentParams) (dom.PayrollAdjustment, error) {
	f.adjCreated = &p
	return dom.PayrollAdjustment{
		ID: "SWP-PAJ-1", EmployeeID: p.EmployeeID, SourceType: dom.AdjustmentSourceType(p.SourceType),
		SourceID: p.SourceID, OriginYear: p.OriginYear, OriginMonth: p.OriginMonth, Status: dom.AdjustmentStatusPending,
	}, nil
}

// --- fake target resolver ---

type fakeResolver struct {
	target ClarificationTarget
	err    error
}

func (f *fakeResolver) ResolveTarget(_ context.Context, _, _ string) (ClarificationTarget, error) {
	return f.target, f.err
}

// --- raise resolves the SL else the agent (C-PC-6) ---

func TestRaise_TargetsLeaderWhenPresent(t *testing.T) {
	repo := &fakeClarRepo{}
	res := &fakeResolver{target: ClarificationTarget{AgentEmployeeID: "SWP-EMP-AG", CompanyID: "SWP-CMP-1", LeaderEmployeeID: "SWP-EMP-SL"}}
	svc := NewClarificationService(repo, res, fakeRunner{})

	cr, err := svc.Raise(hrCtx(), RaiseParams{SourceType: "ATTENDANCE", SourceID: "SWP-ATT-1", Question: "kenapa telat?"})
	if err != nil {
		t.Fatalf("Raise err = %v", err)
	}
	if repo.created.TargetEmployeeID != "SWP-EMP-SL" {
		t.Fatalf("target = %s, want the shift-leader SWP-EMP-SL (C-PC-6)", repo.created.TargetEmployeeID)
	}
	if cr.Status != dom.ClarificationStatusOpen {
		t.Fatalf("status = %v, want OPEN", cr.Status)
	}
}

func TestRaise_FallsBackToAgentWhenNoLeader(t *testing.T) {
	repo := &fakeClarRepo{}
	res := &fakeResolver{target: ClarificationTarget{AgentEmployeeID: "SWP-EMP-AG", CompanyID: "SWP-CMP-1"}}
	svc := NewClarificationService(repo, res, fakeRunner{})

	if _, err := svc.Raise(hrCtx(), RaiseParams{SourceType: "OVERTIME", SourceID: "SWP-OT-1", Question: "q"}); err != nil {
		t.Fatalf("Raise err = %v", err)
	}
	if repo.created.TargetEmployeeID != "SWP-EMP-AG" {
		t.Fatalf("target = %s, want the agent SWP-EMP-AG (C-PC-6 fallback)", repo.created.TargetEmployeeID)
	}
}

func TestRaise_NoTargetRejected(t *testing.T) {
	repo := &fakeClarRepo{}
	res := &fakeResolver{target: ClarificationTarget{}} // no agent, no leader
	svc := NewClarificationService(repo, res, fakeRunner{})

	if _, err := svc.Raise(hrCtx(), RaiseParams{SourceType: "LEAVE", SourceID: "SWP-LR-1", Question: "q"}); err == nil {
		t.Fatalf("Raise with no target allowed, want CLARIFICATION_NO_TARGET")
	} else if ae, _ := apperr.As(err); ae.Code != "CLARIFICATION_NO_TARGET" {
		t.Fatalf("err = %v, want CLARIFICATION_NO_TARGET", err)
	}
}

func TestRaise_InvalidSourceType(t *testing.T) {
	svc := NewClarificationService(&fakeClarRepo{}, &fakeResolver{}, fakeRunner{})
	if _, err := svc.Raise(hrCtx(), RaiseParams{SourceType: "BOGUS", SourceID: "x", Question: "q"}); err == nil {
		t.Fatalf("invalid source_type accepted")
	} else if ae, _ := apperr.As(err); ae.Status() != 400 {
		t.Fatalf("status = %d, want 400", ae.Status())
	}
}

// --- round-trip: raise -> answer -> resolve (§7 "Clarification round-trip") ---

func TestClarification_RoundTrip(t *testing.T) {
	repo := &fakeClarRepo{
		getRow:   dom.ClarificationRequest{ID: "SWP-CLR-1", Status: dom.ClarificationStatusOpen, TargetEmployeeID: "SWP-EMP-SL"},
		answerOK: true, setOK: true,
	}
	svc := NewClarificationService(repo, &fakeResolver{}, fakeRunner{})

	// The SL (the target) answers.
	slCtx := auth.WithPrincipal(context.Background(), auth.Principal{
		UserID: "SWP-USR-SL", EmployeeID: "SWP-EMP-SL", Role: auth.RoleShiftLeader,
	})
	ans, err := svc.Answer(slCtx, "SWP-CLR-1", "dia izin sakit")
	if err != nil {
		t.Fatalf("Answer err = %v", err)
	}
	if ans.Status != dom.ClarificationStatusAnswered {
		t.Fatalf("status = %v, want ANSWERED", ans.Status)
	}

	// HR resolves.
	res, err := svc.Resolve(hrCtx(), "SWP-CLR-1")
	if err != nil {
		t.Fatalf("Resolve err = %v", err)
	}
	if res.Status != dom.ClarificationStatusResolved {
		t.Fatalf("status = %v, want RESOLVED", res.Status)
	}
}

// --- answer scope-check (PC-15): a different agent is out of scope ---

func TestAnswer_ScopeCheck(t *testing.T) {
	repo := &fakeClarRepo{getRow: dom.ClarificationRequest{ID: "SWP-CLR-1", Status: dom.ClarificationStatusOpen, TargetEmployeeID: "SWP-EMP-SL"}, answerOK: true}
	svc := NewClarificationService(repo, &fakeResolver{}, fakeRunner{})

	otherAgent := auth.WithPrincipal(context.Background(), auth.Principal{
		UserID: "SWP-USR-X", EmployeeID: "SWP-EMP-OTHER", Role: auth.RoleAgent,
	})
	if _, err := svc.Answer(otherAgent, "SWP-CLR-1", "x"); err == nil {
		t.Fatalf("a non-target answered, want OUT_OF_SCOPE")
	} else if ae, _ := apperr.As(err); ae.Code != "OUT_OF_SCOPE" {
		t.Fatalf("err = %v, want OUT_OF_SCOPE", err)
	}
}

// --- PC-9: post-lock change emits a PENDING adjustment ---

func TestEmitCorrectionAdjustment(t *testing.T) {
	repo := &fakeClarRepo{}
	svc := NewClarificationService(repo, &fakeResolver{}, fakeRunner{})

	err := svc.EmitCorrectionAdjustment(hrCtx(), fakeTx{}, "SWP-EMP-1", "SWP-ATT-5", 2026, 6)
	if err != nil {
		t.Fatalf("EmitCorrectionAdjustment err = %v", err)
	}
	if repo.adjCreated == nil {
		t.Fatalf("no adjustment emitted (PC-9)")
	}
	if repo.adjCreated.SourceType != string(dom.AdjustmentSourceCorrection) {
		t.Fatalf("source_type = %s, want CORRECTION", repo.adjCreated.SourceType)
	}
	if repo.adjCreated.OriginYear != 2026 || repo.adjCreated.OriginMonth != 6 {
		t.Fatalf("origin = %d-%d, want 2026-6", repo.adjCreated.OriginYear, repo.adjCreated.OriginMonth)
	}
}
