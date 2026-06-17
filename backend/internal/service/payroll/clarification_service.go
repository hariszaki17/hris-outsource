// Package payroll — F8.5 ClarificationService (PC-13) + the PC-9 post-lock
// adjustment-emit helper. Clarification is a thin, notification-backed, one-round
// back-channel (ask -> answer -> resolve/cancel), NOT an approval line — it blocks
// nothing. Raise resolves the target (the record's company shift-leader, else the
// agent; C-PC-6), enqueues an E10 notification (transactional outbox), and audits.
package payroll

import (
	"context"

	"github.com/jackc/pgx/v5"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/payroll"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/jobs"
	reportingdom "github.com/hariszaki17/hris-outsource/backend/internal/domain/reporting"
)

// Clarification audit actions (PC-14).
const (
	ActionClarificationRaised    audit.Action = "CLARIFICATION_RAISED"
	ActionClarificationAnswered  audit.Action = "CLARIFICATION_ANSWERED"
	ActionClarificationResolved  audit.Action = "CLARIFICATION_RESOLVED"
	ActionClarificationCancelled audit.Action = "CLARIFICATION_CANCELLED"
)

// ClarificationService implements the F8.5 clarification + adjustment business logic.
type ClarificationService struct {
	repo     ClarificationRepository
	resolver ClarificationTargetResolver
	txm      TxRunner
	notifier jobs.Dispatcher // E10 notify seam (nil-safe in unit tests)
}

// NewClarificationService wires the clarification service. resolver resolves the
// notification target (SL else agent, C-PC-6); notifier is the E10 dispatcher
// (nil-safe).
func NewClarificationService(repo ClarificationRepository, resolver ClarificationTargetResolver, txm TxRunner) *ClarificationService {
	return &ClarificationService{repo: repo, resolver: resolver, txm: txm}
}

// SetNotifier wires the E10 notification dispatcher. Additive + nil-safe.
func (s *ClarificationService) SetNotifier(d jobs.Dispatcher) { s.notifier = d }

// RaiseParams is the validated raise payload (§9 POST /clarifications). PeriodID is
// optional (the month it surfaced in, for the cockpit "awaiting reply" surface).
type RaiseParams struct {
	SourceType string // ATTENDANCE | OVERTIME | LEAVE
	SourceID   string
	Question   string
	PeriodID   *string
}

// Raise creates a clarification (PC-13): resolves the target (SL else agent),
// inserts the OPEN row, enqueues an E10 notification (transactional outbox), and
// audits — all in one tx.
func (s *ClarificationService) Raise(ctx context.Context, p RaiseParams) (dom.ClarificationRequest, error) {
	if !validClarificationSource(p.SourceType) {
		return dom.ClarificationRequest{}, apperr.Invalid(map[string]string{"source_type": "Tipe sumber tidak valid."})
	}
	if p.SourceID == "" || p.Question == "" {
		return dom.ClarificationRequest{}, apperr.Invalid(map[string]string{"question": "Pertanyaan wajib diisi."})
	}

	// Resolve the target (C-PC-6): the record's company shift-leader, else the agent.
	target, err := s.resolver.ResolveTarget(ctx, p.SourceType, p.SourceID)
	if err != nil {
		return dom.ClarificationRequest{}, asAppErr(err)
	}
	targetEmployeeID := target.LeaderEmployeeID
	if targetEmployeeID == "" {
		targetEmployeeID = target.AgentEmployeeID
	}
	if targetEmployeeID == "" {
		return dom.ClarificationRequest{}, apperr.Rule("CLARIFICATION_NO_TARGET", map[string]string{
			"source_id": "Tidak ada penerima klarifikasi untuk catatan ini.",
		})
	}

	var out dom.ClarificationRequest
	if txErr := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		cr, cerr := s.repo.CreateClarification(ctx, tx, CreateClarificationParams{
			SourceType:       p.SourceType,
			SourceID:         p.SourceID,
			PeriodID:         p.PeriodID,
			RaisedBy:         actorUserID(ctx),
			TargetEmployeeID: targetEmployeeID,
			Question:         p.Question,
		})
		if cerr != nil {
			return cerr
		}
		out = cr

		// E10 notification (transactional outbox). No dedicated clarification kind in
		// the v1 catalog — reuse ATTENDANCE_VERIFY_NEEDED as the nearest honest kind,
		// with clarification-specific copy + a deep link to the source record.
		if nerr := jobs.Dispatch(ctx, s.notifier, tx, jobs.NotificationArgs{
			NotifKind:        string(reportingdom.NotifAttendanceVerifyNeeded),
			RecipientID:      targetEmployeeID,
			Title:            "Permintaan klarifikasi",
			Body:             "HR meminta klarifikasi atas sebuah catatan.",
			DeepLinkEpic:     "E8",
			DeepLinkEntityID: p.SourceID,
			DeepLinkPath:     clarificationDeepLink(p.SourceType, p.SourceID),
			ActorLabel:       "HR",
			IsCritical:       false,
		}); nerr != nil {
			return nerr
		}

		return audit.Record(ctx, tx, audit.Entry{
			Action:     ActionClarificationRaised,
			EntityType: "clarification_request",
			EntityID:   cr.ID,
			After: map[string]any{
				"source_type":        p.SourceType,
				"source_id":          p.SourceID,
				"target_employee_id": targetEmployeeID,
				"status":             string(cr.Status),
			},
		})
	}); txErr != nil {
		return dom.ClarificationRequest{}, asAppErr(txErr)
	}
	return out, nil
}

// Answer is the target's reply (PC-13): scope-checked to the target employee (the
// resolved SL or agent), guarded OPEN -> ANSWERED. The caller MAY separately file a
// correction (F5.4) which routes normally — that is out of this path.
func (s *ClarificationService) Answer(ctx context.Context, id, answer string) (dom.ClarificationRequest, error) {
	if answer == "" {
		return dom.ClarificationRequest{}, apperr.Invalid(map[string]string{"answer": "Jawaban wajib diisi."})
	}
	p, _ := auth.PrincipalFrom(ctx)

	var out dom.ClarificationRequest
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		cur, cerr := s.repo.GetClarificationForUpdate(ctx, tx, id)
		if cerr != nil {
			return cerr
		}
		// Scope-check (PC-15): only the resolved target may answer. HR/super may also
		// answer on the target's behalf (they raised it / oversee it).
		if !canAnswer(p, cur) {
			return apperr.OutOfScope()
		}

		answered, ok, aerr := s.repo.Answer(ctx, tx, id, answer, actorUserID(ctx))
		if aerr != nil {
			return aerr
		}
		if !ok {
			return apperr.Conflict("CLARIFICATION_NOT_OPEN")
		}
		out = answered
		return audit.Record(ctx, tx, audit.Entry{
			Action:     ActionClarificationAnswered,
			EntityType: "clarification_request",
			EntityID:   id,
			Before:     map[string]any{"status": string(cur.Status)},
			After:      map[string]any{"status": string(answered.Status), "answer": answer},
		})
	}); err != nil {
		return dom.ClarificationRequest{}, asAppErr(err)
	}
	return out, nil
}

// Resolve closes the loop as RESOLVED (PC-13, HR).
func (s *ClarificationService) Resolve(ctx context.Context, id string) (dom.ClarificationRequest, error) {
	return s.close(ctx, id, string(dom.ClarificationStatusResolved), ActionClarificationResolved)
}

// Cancel closes the loop as CANCELLED (PC-13, HR).
func (s *ClarificationService) Cancel(ctx context.Context, id string) (dom.ClarificationRequest, error) {
	return s.close(ctx, id, string(dom.ClarificationStatusCancelled), ActionClarificationCancelled)
}

func (s *ClarificationService) close(ctx context.Context, id, status string, action audit.Action) (dom.ClarificationRequest, error) {
	var out dom.ClarificationRequest
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		cur, cerr := s.repo.GetClarificationForUpdate(ctx, tx, id)
		if cerr != nil {
			return cerr
		}
		closed, ok, serr := s.repo.SetStatus(ctx, tx, id, status)
		if serr != nil {
			return serr
		}
		if !ok {
			return apperr.Conflict("CLARIFICATION_TERMINAL")
		}
		out = closed
		return audit.Record(ctx, tx, audit.Entry{
			Action:     action,
			EntityType: "clarification_request",
			EntityID:   id,
			Before:     map[string]any{"status": string(cur.Status)},
			After:      map[string]any{"status": string(closed.Status)},
		})
	}); err != nil {
		return dom.ClarificationRequest{}, asAppErr(err)
	}
	return out, nil
}

// --- PC-9 post-lock adjustment emit ---

// EmitAdjustment writes a PENDING, unvalued carry-forward ledger row for a post-lock
// change to a locked-month record (PC-9). It runs INSIDE the caller's tx (the
// correction-apply / attendance-change tx) so the adjustment commits atomically with
// the change. amount is left NULL for the F8.3 run to value. This is the function the
// attendance correction-apply path calls when the record's month is LOCKED.
func (s *ClarificationService) EmitAdjustment(ctx context.Context, tx pgx.Tx, p CreateAdjustmentParams) (dom.PayrollAdjustment, error) {
	if !validAdjustmentSource(p.SourceType) {
		return dom.PayrollAdjustment{}, apperr.Invalid(map[string]string{"source_type": "Tipe sumber penyesuaian tidak valid."})
	}
	adj, err := s.repo.CreateAdjustment(ctx, tx, p)
	if err != nil {
		return dom.PayrollAdjustment{}, asAppErr(err)
	}
	if aerr := audit.Record(ctx, tx, audit.Entry{
		Action:     audit.Action("PAYROLL_ADJUSTMENT_EMITTED"),
		EntityType: "payroll_adjustment",
		EntityID:   adj.ID,
		After: map[string]any{
			"employee_id":  adj.EmployeeID,
			"source_type":  string(adj.SourceType),
			"source_id":    adj.SourceID,
			"origin_year":  adj.OriginYear,
			"origin_month": adj.OriginMonth,
		},
	}); aerr != nil {
		return dom.PayrollAdjustment{}, asAppErr(aerr)
	}
	return adj, nil
}

// EmitCorrectionAdjustment is the attendance.AdjustmentEmitter adapter (PC-9): a
// thin wrapper over EmitAdjustment that fixes source_type = CORRECTION. The
// attendance correction-apply path calls this inside its tx when the record's month
// is LOCKED.
func (s *ClarificationService) EmitCorrectionAdjustment(ctx context.Context, tx pgx.Tx, employeeID, sourceID string, originYear, originMonth int) error {
	_, err := s.EmitAdjustment(ctx, tx, CreateAdjustmentParams{
		EmployeeID:  employeeID,
		SourceType:  string(dom.AdjustmentSourceCorrection),
		SourceID:    sourceID,
		OriginYear:  originYear,
		OriginMonth: originMonth,
	})
	return err
}

// --- helpers ---

func validClarificationSource(s string) bool {
	switch dom.ClarificationSourceType(s) {
	case dom.ClarificationSourceAttendance, dom.ClarificationSourceOvertime, dom.ClarificationSourceLeave:
		return true
	}
	return false
}

func validAdjustmentSource(s string) bool {
	switch dom.AdjustmentSourceType(s) {
	case dom.AdjustmentSourceAttendance, dom.AdjustmentSourceOvertime, dom.AdjustmentSourceCorrection:
		return true
	}
	return false
}

// canAnswer enforces PC-15: the resolved target employee may answer; HR/super may
// answer on their behalf. Everyone else (incl. a different agent/leader) is out of scope.
func canAnswer(p auth.Principal, cr dom.ClarificationRequest) bool {
	if p.Role == auth.RoleHRAdmin || p.Role == auth.RoleSuperAdmin {
		return true
	}
	return p.EmployeeID != "" && p.EmployeeID == cr.TargetEmployeeID
}

// clarificationDeepLink builds the source-record deep link path for the notification.
func clarificationDeepLink(sourceType, sourceID string) string {
	switch dom.ClarificationSourceType(sourceType) {
	case dom.ClarificationSourceAttendance:
		return "/attendance/" + sourceID
	case dom.ClarificationSourceOvertime:
		return "/overtime/" + sourceID
	case dom.ClarificationSourceLeave:
		return "/leave-requests/" + sourceID
	}
	return ""
}
