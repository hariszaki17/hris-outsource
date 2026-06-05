// Package overtime — OvertimeService: the two-level OT approval state machine
// (PENDING_AGENT_CONFIRM → PENDING_L1 → PENDING_HR → APPROVED; reject → REJECTED;
// withdraw → WITHDRAWN), the bulk approve/reject partial-success engine, OT_BELOW_MIN
// enforcement against the EXISTING E2 overtime_rules, day_type classification
// (schedule + holiday calendar with HOLIDAY>RESTDAY>WORKDAY precedence), GuardCompany
// scope (OUT_OF_SCOPE) + SELF_APPROVAL_FORBIDDEN, audit-in-tx + notify stub.
//
// V1 records HOURS/MINUTES ONLY (INV-2): the reference multiplier from the applied
// rule is exposed in the calculation block as reference metadata, NEVER applied to
// any money figure.
//
// Mirrors the Phase-8 leave service (two-level approval) + Phase-7 attendance service
// (bulk partial-success) EXACTLY.
package overtime

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/overtime"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
)

// OvertimeService implements the OT approval business logic.
type OvertimeService struct {
	repo     OvertimeRepository
	rules    RuleRepository
	holidays HolidayRepository
	schedule SchedulePort
	txm      TxRunner
	now      Clock
}

// NewOvertimeService wires the overtime service. holidays + schedule feed the
// day_type classification; rules feed OT_BELOW_MIN + the reference multiplier.
func NewOvertimeService(repo OvertimeRepository, rules RuleRepository, holidays HolidayRepository, schedule SchedulePort, txm TxRunner) *OvertimeService {
	return &OvertimeService{repo: repo, rules: rules, holidays: holidays, schedule: schedule, txm: txm, now: time.Now}
}

// SetClock overrides the time source (tests only).
func (s *OvertimeService) SetClock(c Clock) { s.now = c }

// --- calculation block (computed at read time; INV-2 reference-only multiplier) ---

// TierBreakdown is one resolved tier slice in the calculation block.
type TierBreakdown struct {
	Tier         dom.OvertimeTier
	Minutes      int
	Multiplier   float64
	OvertimeRule *string
	Supersedes   *dom.OvertimeTier // names the winning tier when this entry was downgraded
}

// Calculation is the server-computed OvertimeCalculation block (never persisted
// stale). The React client renders it directly.
type Calculation struct {
	WorkedMinutes       int
	CountedMinutes      int
	MinMinutesThreshold int
	SkippedTooShort     bool
	TierBreakdown       []TierBreakdown
}

// computeCalculation builds the calculation block from the stored OT + the applied
// rule (looked up by service line). Single-tier breakdown (supersedes null) — the
// resolved day_type IS the effective tier (precedence already applied at record
// time). The multiplier is REFERENCE metadata only (INV-2).
func (s *OvertimeService) computeCalculation(ctx context.Context, ot dom.Overtime) Calculation {
	counted := ot.CountedMinutes
	if counted == 0 {
		counted = ot.CountedFromWorked()
	}
	threshold := ot.MinMinutesThreshold
	if threshold == 0 {
		threshold = 30
	}
	calc := Calculation{
		WorkedMinutes:       ot.WorkedMinutes,
		CountedMinutes:      counted,
		MinMinutesThreshold: threshold,
		SkippedTooShort:     ot.SkippedTooShort || counted < threshold,
	}
	mult := s.tierMultiplier(ctx, ot)
	calc.TierBreakdown = []TierBreakdown{{
		Tier:         ot.DayType,
		Minutes:      counted,
		Multiplier:   mult,
		OvertimeRule: ot.OvertimeRuleID,
		Supersedes:   nil, // single-tier resolution: this is the effective tier
	}}
	return calc
}

// tierMultiplier resolves the reference multiplier for the OT's day_type from the
// applied rule. Falls back to the stored reference_multiplier, then 0 (INV-2:
// reference only; never applied to money).
func (s *OvertimeService) tierMultiplier(ctx context.Context, ot dom.Overtime) float64 {
	if rule, err := s.rules.FindOvertimeRule(ctx, ot.ServiceLineID); err == nil {
		switch ot.DayType {
		case dom.OvertimeTierHoliday:
			return rule.HolidayRate
		case dom.OvertimeTierRestday:
			return rule.RestdayRate
		default:
			return rule.WeekdayRate
		}
	}
	if ot.ReferenceMultiplier != nil {
		return *ot.ReferenceMultiplier
	}
	return 0
}

// --- list / get ---

// List returns the OT page. Leader scope is forced to their led company (explicit
// cross-company company_id → 403 OUT_OF_SCOPE); agent sees own (employee_id forced);
// HR/super are global. Each row carries the computed calculation block.
func (s *OvertimeService) List(ctx context.Context, f OvertimeFilter) ([]dom.Overtime, []Calculation, *string, bool, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, nil, nil, false, apperr.Unauthenticated()
	}
	switch p.Role {
	case auth.RoleShiftLeader:
		if f.CompanyID != nil && *f.CompanyID != p.CompanyID {
			return nil, nil, nil, false, apperr.OutOfScope()
		}
		cid := p.CompanyID
		f.CompanyID = &cid
	case auth.RoleAgent:
		if p.EmployeeID != "" {
			eid := p.EmployeeID
			f.EmployeeID = &eid
		}
	}
	limit := clampLimit(f.Limit)
	f.Limit = limit + 1
	rows, err := s.repo.ListOvertime(ctx, f)
	if err != nil {
		return nil, nil, nil, false, apperr.Internal(err)
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	var next *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		c, cerr := encodeOvertimeCursor(last.CreatedAt, last.ID)
		if cerr != nil {
			return nil, nil, nil, false, cerr
		}
		next = &c
	}
	calcs := make([]Calculation, len(rows))
	for i := range rows {
		calcs[i] = s.computeCalculation(ctx, rows[i])
	}
	return rows, calcs, next, hasMore, nil
}

// Get loads one OT with its approval timeline + recomputed calculation. Cross-scope
// reads are hidden as 404 (no existence leak).
func (s *OvertimeService) Get(ctx context.Context, id string) (dom.Overtime, Calculation, error) {
	rec, err := s.repo.GetOvertime(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return dom.Overtime{}, Calculation{}, apperr.NotFound()
	}
	if err != nil {
		return dom.Overtime{}, Calculation{}, apperr.Internal(err)
	}
	if serr := rbac.GuardCompany(ctx, deref(rec.CompanyID)); serr != nil {
		return dom.Overtime{}, Calculation{}, apperr.NotFound()
	}
	if approvals, aerr := s.repo.ListOvertimeApprovals(ctx, id); aerr == nil {
		rec.Approvals = approvals
	}
	return rec, s.computeCalculation(ctx, rec), nil
}

// --- transitions ---

// Confirm forwards PENDING_AGENT_CONFIRM → PENDING_L1 (OC-2 / OA-6). Only the OT's
// own agent may confirm (openapi x-rbac agent/self); HR/leader cannot confirm on
// behalf → 403 SELF/scope. Wrong-state → 409.
func (s *OvertimeService) Confirm(ctx context.Context, id, note string) (dom.Overtime, Calculation, error) {
	actor := actorEmployeeID(ctx)
	var out dom.Overtime
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		rec, lerr := s.lock(ctx, tx, id)
		if lerr != nil {
			return lerr
		}
		if rec.Status != dom.OvertimeStatusPendingAgentConfirm {
			return stateConflict(rec.Status)
		}
		// Agent-scope: only the OT's own agent may confirm (openapi x-rbac: agent).
		// The web confirm flow is HR/leader-triggered on seeded candidates per
		// CONTEXT; honor the contract by checking actor == employee when the actor
		// IS an agent. Staff roles (HR/super/leader) pass for the web seam.
		if serr := s.guardConfirmActor(ctx, rec); serr != nil {
			return serr
		}
		updated, uerr := s.repo.UpdateOvertimeStatus(ctx, tx, id, dom.OvertimeStatusPendingL1)
		if uerr != nil {
			return uerr
		}
		out = updated
		// note is appended to the decision trail (level 0 / agent confirm marker).
		if _, aerr := s.repo.InsertOvertimeApproval(ctx, tx, ApprovalRow{
			OvertimeID: id, Level: 1, Decision: "APPROVED",
			ApproverID: actor, ApproverName: actorName(ctx), Reason: strOrNil(note),
		}); aerr != nil {
			return aerr
		}
		// TODO(Phase-11): enqueue NotificationArgs ("OT confirmed → leader queue").
		return audit.Record(ctx, tx, otAudit(id, string(rec.Status), "PENDING_L1", actor, "CONFIRM"))
	})
	if err != nil {
		return dom.Overtime{}, Calculation{}, asAppErr(err)
	}
	return s.reread(ctx, out)
}

// ApproveL1 forwards PENDING_L1 → PENDING_HR (leader / OA-1/2/5). Guards: state (409),
// scope (403 OUT_OF_SCOPE), self-approve (403 SELF_APPROVAL_FORBIDDEN). Records an
// OvertimeApproval at level 1.
func (s *OvertimeService) ApproveL1(ctx context.Context, id, note string) (dom.Overtime, Calculation, error) {
	actor := actorEmployeeID(ctx)
	var out dom.Overtime
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		rec, lerr := s.lock(ctx, tx, id)
		if lerr != nil {
			return lerr
		}
		if rec.Status != dom.OvertimeStatusPendingL1 {
			return stateConflict(rec.Status)
		}
		if serr := rbac.GuardCompany(ctx, deref(rec.CompanyID)); serr != nil {
			return serr
		}
		if serr := guardSelf(ctx, rec); serr != nil {
			return serr
		}
		updated, uerr := s.repo.UpdateOvertimeStatus(ctx, tx, id, dom.OvertimeStatusPendingHR)
		if uerr != nil {
			return uerr
		}
		out = updated
		if _, aerr := s.repo.InsertOvertimeApproval(ctx, tx, ApprovalRow{
			OvertimeID: id, Level: 1, Decision: "APPROVED",
			ApproverID: actor, ApproverName: actorName(ctx), Reason: strOrNil(note),
		}); aerr != nil {
			return aerr
		}
		// TODO(Phase-11): enqueue NotificationArgs ("OT L1-approved → HR queue").
		return audit.Record(ctx, tx, otAudit(id, string(rec.Status), "PENDING_HR", actor, "APPROVE_L1"))
	})
	if err != nil {
		return dom.Overtime{}, Calculation{}, asAppErr(err)
	}
	return s.reread(ctx, out)
}

// ApproveFinal moves PENDING_HR → APPROVED (HR / OA-1). With isOverride, HR may
// bypass L1 from PENDING_L1 (OA-8) — note required, else 422 OVERRIDE_REASON_REQUIRED.
func (s *OvertimeService) ApproveFinal(ctx context.Context, id, note string, isOverride bool) (dom.Overtime, Calculation, error) {
	if isOverride && note == "" {
		return dom.Overtime{}, Calculation{}, apperr.Rule("OVERRIDE_REASON_REQUIRED", map[string]string{"note": "Wajib diisi saat override."})
	}
	actor := actorEmployeeID(ctx)
	var out dom.Overtime
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		rec, lerr := s.lock(ctx, tx, id)
		if lerr != nil {
			return lerr
		}
		legal := rec.Status == dom.OvertimeStatusPendingHR ||
			(isOverride && rec.Status == dom.OvertimeStatusPendingL1)
		if !legal {
			return stateConflict(rec.Status)
		}
		// HR/super only — RequireRole gates the route; defense-in-depth self-guard.
		if serr := guardSelf(ctx, rec); serr != nil {
			return serr
		}
		updated, uerr := s.repo.UpdateOvertimeStatus(ctx, tx, id, dom.OvertimeStatusApproved)
		if uerr != nil {
			return uerr
		}
		out = updated
		decision := "APPROVED"
		action := "APPROVE_FINAL"
		if isOverride {
			decision = "OVERRIDE_APPROVED"
			action = "APPROVE_OVERRIDE"
		}
		if _, aerr := s.repo.InsertOvertimeApproval(ctx, tx, ApprovalRow{
			OvertimeID: id, Level: 2, Decision: decision,
			ApproverID: actor, ApproverName: actorName(ctx), Reason: strOrNil(note),
		}); aerr != nil {
			return aerr
		}
		// TODO(Phase-11): enqueue NotificationArgs ("OT approved" + submitter notify).
		return audit.Record(ctx, tx, otAudit(id, string(rec.Status), "APPROVED", actor, action))
	})
	if err != nil {
		return dom.Overtime{}, Calculation{}, asAppErr(err)
	}
	return s.reread(ctx, out)
}

// Reject moves PENDING_L1 / PENDING_HR → REJECTED (reason required, min 5). Records
// the rejecting level. Terminal → 409; out-of-scope → 403.
func (s *OvertimeService) Reject(ctx context.Context, id, reason string) (dom.Overtime, Calculation, error) {
	if len([]rune(reason)) < 5 {
		return dom.Overtime{}, Calculation{}, apperr.Invalid(map[string]string{"reason": "Wajib diisi (minimum 5 karakter)."})
	}
	actor := actorEmployeeID(ctx)
	var out dom.Overtime
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		rec, lerr := s.lock(ctx, tx, id)
		if lerr != nil {
			return lerr
		}
		if rec.Status != dom.OvertimeStatusPendingL1 && rec.Status != dom.OvertimeStatusPendingHR {
			return stateConflict(rec.Status)
		}
		if serr := rbac.GuardCompany(ctx, deref(rec.CompanyID)); serr != nil {
			return serr
		}
		level := 2
		if rec.Status == dom.OvertimeStatusPendingL1 {
			level = 1
		}
		updated, uerr := s.repo.UpdateOvertimeStatus(ctx, tx, id, dom.OvertimeStatusRejected)
		if uerr != nil {
			return uerr
		}
		out = updated
		rsn := reason
		if _, aerr := s.repo.InsertOvertimeApproval(ctx, tx, ApprovalRow{
			OvertimeID: id, Level: level, Decision: "REJECTED",
			ApproverID: actor, ApproverName: actorName(ctx), Reason: &rsn,
		}); aerr != nil {
			return aerr
		}
		// TODO(Phase-11): enqueue NotificationArgs ("OT rejected").
		return audit.Record(ctx, tx, otAudit(id, string(rec.Status), "REJECTED", actor, "REJECT"))
	})
	if err != nil {
		return dom.Overtime{}, Calculation{}, asAppErr(err)
	}
	return s.reread(ctx, out)
}

// Withdraw moves PENDING_AGENT_CONFIRM / PENDING_L1 → WITHDRAWN (OA C-3). Not
// withdrawable once APPROVED/REJECTED/WITHDRAWN → 409. No body, 204.
func (s *OvertimeService) Withdraw(ctx context.Context, id string) error {
	actor := actorEmployeeID(ctx)
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		rec, lerr := s.lock(ctx, tx, id)
		if lerr != nil {
			return lerr
		}
		if rec.Status != dom.OvertimeStatusPendingAgentConfirm && rec.Status != dom.OvertimeStatusPendingL1 {
			return stateConflict(rec.Status)
		}
		if _, uerr := s.repo.UpdateOvertimeStatus(ctx, tx, id, dom.OvertimeStatusWithdrawn); uerr != nil {
			return uerr
		}
		// TODO(Phase-11): enqueue NotificationArgs ("OT withdrawn").
		return audit.Record(ctx, tx, otAudit(id, string(rec.Status), "WITHDRAWN", actor, "WITHDRAW"))
	})
	if err != nil {
		return asAppErr(err)
	}
	return nil
}

// --- bulk approve / reject (per-item own-tx partial success) ---

// FailedItem is one failed row in the bulk envelope.
type FailedItem struct {
	ID      string
	Code    string
	Message string
}

// BulkResult is the partial-success aggregate (openapi BulkResult {succeeded,failed}).
type BulkResult struct {
	Succeeded []string
	Failed    []FailedItem
}

// BulkApprove approves each id at whichever level the caller is authoritative for
// (leader → L1, HR → final). Each id runs in its OWN tx (Phase-7 atomicity); per-id
// failures (SELF_APPROVAL_FORBIDDEN / OUT_OF_SCOPE / state-409) land in failed[].
func (s *OvertimeService) BulkApprove(ctx context.Context, ids []string, note string) (BulkResult, error) {
	isHR := actorIsHR(ctx)
	var out BulkResult
	for _, id := range ids {
		var err error
		if isHR {
			_, _, err = s.ApproveFinal(ctx, id, note, false)
		} else {
			_, _, err = s.ApproveL1(ctx, id, note)
		}
		if err == nil {
			out.Succeeded = append(out.Succeeded, id)
			continue
		}
		if ae, ok := apperr.As(err); ok {
			out.Failed = append(out.Failed, FailedItem{ID: id, Code: ae.Code, Message: bulkMessage(ae)})
			continue
		}
		return BulkResult{}, err
	}
	return out, nil
}

// BulkReject rejects each id with the shared reason. Same partial-success shape.
func (s *OvertimeService) BulkReject(ctx context.Context, ids []string, reason string) (BulkResult, error) {
	var out BulkResult
	for _, id := range ids {
		_, _, err := s.Reject(ctx, id, reason)
		if err == nil {
			out.Succeeded = append(out.Succeeded, id)
			continue
		}
		if ae, ok := apperr.As(err); ok {
			out.Failed = append(out.Failed, FailedItem{ID: id, Code: ae.Code, Message: bulkMessage(ae)})
			continue
		}
		return BulkResult{}, err
	}
	return out, nil
}

// --- day_type classification + OT_BELOW_MIN (exported seams for 09-03 + seed) ---

// ClassifyDayType resolves the OT tier for (employee, work_date, service line):
// GetHolidayForDate → HOLIDAY (+ holiday id); else a live schedule entry → WORKDAY;
// else RESTDAY. TierPrecedence (HOLIDAY>RESTDAY>WORKDAY) resolves overlaps. Returns
// the resolved tier + the matched holiday id (nil unless HOLIDAY).
func (s *OvertimeService) ClassifyDayType(ctx context.Context, employeeID string, workDate time.Time, serviceLineID *string) (dom.OvertimeTier, *string) {
	scheduleTier := dom.OvertimeTierRestday
	if live, err := s.schedule.FindLiveEntryForAgentDate(ctx, employeeID, workDate); err == nil && live.ID != "" && !live.IsDayOff {
		scheduleTier = dom.OvertimeTierWorkday
	}
	holidayTier := dom.OvertimeTierWorkday // inert when no holiday
	var holidayID *string
	if hol, err := s.holidays.GetHolidayForDate(ctx, workDate); err == nil && hol.ID != "" {
		holidayTier = dom.OvertimeTierHoliday
		hid := hol.ID
		holidayID = &hid
	}
	resolved := dom.TierPrecedence(scheduleTier, holidayTier)
	if resolved != dom.OvertimeTierHoliday {
		holidayID = nil
	}
	return resolved, holidayID
}

// EnforceMinMinutes returns OT_BELOW_MIN (422 with field errors, INV-5) when the
// counted minutes fall below the applicable rule's min_minutes. Exposed as a seam
// for 09-03 contract tests + the seed validation path (the openapi returns this on
// the create path, which is OUT of web scope).
func (s *OvertimeService) EnforceMinMinutes(ctx context.Context, countedMinutes int, serviceLineID *string) error {
	rule, err := s.rules.FindOvertimeRule(ctx, serviceLineID)
	min := 30
	if err == nil && rule.MinMinutes > 0 {
		min = rule.MinMinutes
	}
	if countedMinutes < min {
		return apperr.Rule("OT_BELOW_MIN", map[string]string{
			"counted_minutes": itoa(countedMinutes),
			"min_minutes":     itoa(min),
		})
	}
	return nil
}

// --- helpers ---

func (s *OvertimeService) lock(ctx context.Context, tx pgx.Tx, id string) (dom.Overtime, error) {
	rec, err := s.repo.GetOvertimeForUpdate(ctx, tx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return dom.Overtime{}, apperr.NotFound()
	}
	if err != nil {
		return dom.Overtime{}, err
	}
	return rec, nil
}

// reread re-loads the OT with denormalized names, approvals, and a fresh calculation.
func (s *OvertimeService) reread(ctx context.Context, fallback dom.Overtime) (dom.Overtime, Calculation, error) {
	rec := fallback
	if full, err := s.repo.GetOvertime(ctx, fallback.ID); err == nil {
		rec = full
	}
	if approvals, aerr := s.repo.ListOvertimeApprovals(ctx, rec.ID); aerr == nil {
		rec.Approvals = approvals
	}
	return rec, s.computeCalculation(ctx, rec), nil
}

// guardSelf enforces OA-5: an approver cannot decide their own OT →
// 403 SELF_APPROVAL_FORBIDDEN (struct literal bypasses statusForCode).
func guardSelf(ctx context.Context, rec dom.Overtime) error {
	if p, ok := auth.PrincipalFrom(ctx); ok && p.EmployeeID != "" && p.EmployeeID == rec.EmployeeID {
		return &apperr.Error{
			HTTPStatus: 403,
			Code:       "SELF_APPROVAL_FORBIDDEN",
			Message:    "Tidak dapat menyetujui lembur sendiri.",
		}
	}
	return nil
}

// guardConfirmActor enforces the openapi x-rbac (agent/self) on :confirm: when the
// actor is an agent, they must be the OT's own agent (else 403). Staff roles
// (HR/super/leader) pass for the web-triggered confirm seam per CONTEXT.
func (s *OvertimeService) guardConfirmActor(ctx context.Context, rec dom.Overtime) error {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return apperr.Unauthenticated()
	}
	if p.Role == auth.RoleAgent {
		if p.EmployeeID == "" || p.EmployeeID != rec.EmployeeID {
			return apperr.Forbidden()
		}
	}
	return nil
}

// actorIsHR reports whether the principal is HR/super (final/override authority).
func actorIsHR(ctx context.Context) bool {
	if p, ok := auth.PrincipalFrom(ctx); ok {
		return p.Role == auth.RoleHRAdmin || p.Role == auth.RoleSuperAdmin
	}
	return false
}

func otAudit(id, before, after string, actor *string, action string) audit.Entry {
	return audit.Entry{
		Action:     audit.ActionUpdate,
		EntityType: "overtime",
		EntityID:   id,
		Before:     map[string]any{"status": before},
		After:      map[string]any{"status": after, "decided_by": ptrStr(actor), "decision": action},
	}
}

func strOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// bulkMessage returns the Bahasa message for a failed bulk row, preferring the
// apperr's own message.
func bulkMessage(ae *apperr.Error) string {
	if ae.Message != "" {
		return ae.Message
	}
	switch ae.Code {
	case "SELF_APPROVAL_FORBIDDEN":
		return "Tidak dapat menyetujui lembur sendiri."
	case "OUT_OF_SCOPE":
		return "Lembur berada di luar perusahaan binaan Anda."
	case "CONFLICT":
		return "Lembur sudah pada status lain."
	case "NOT_FOUND":
		return "Lembur tidak ditemukan."
	default:
		return "Tindakan gagal untuk lembur ini."
	}
}
