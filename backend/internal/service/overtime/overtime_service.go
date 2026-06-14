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
	approval "github.com/hariszaki17/hris-outsource/backend/internal/domain/approval"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/overtime"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/jobs"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"

	reportingdom "github.com/hariszaki17/hris-outsource/backend/internal/domain/reporting"
)

// OvertimeService implements the OT lifecycle. Approval routing is OWNED BY the E11
// engine: Confirm/Create set status=PENDING and create an ApprovalInstance; the engine
// drives the chain and calls OnApproved/OnRejected on terminal transition.
type OvertimeService struct {
	repo     OvertimeRepository
	rules    RuleRepository
	holidays HolidayRepository
	schedule SchedulePort
	txm      TxRunner
	now      Clock
	engine   approval.Engine // E11: creates the approval instance at :create / :confirm
	notifier jobs.Dispatcher // E10 (11-02): transactional-outbox notify seam (nil-safe in unit tests)
}

// NewOvertimeService wires the overtime service. holidays + schedule feed the
// day_type classification; rules feed OT_BELOW_MIN + the reference multiplier.
func NewOvertimeService(repo OvertimeRepository, rules RuleRepository, holidays HolidayRepository, schedule SchedulePort, txm TxRunner) *OvertimeService {
	return &OvertimeService{repo: repo, rules: rules, holidays: holidays, schedule: schedule, txm: txm, now: time.Now}
}

// SetClock overrides the time source (tests only).
func (s *OvertimeService) SetClock(c Clock) { s.now = c }

// SetApprovalEngine wires the E11 approval engine. REQUIRED for Create/Confirm.
func (s *OvertimeService) SetApprovalEngine(e approval.Engine) { s.engine = e }

// SetNotifier wires the E10 notification dispatcher (11-02). Additive + nil-safe.
func (s *OvertimeService) SetNotifier(d jobs.Dispatcher) { s.notifier = d }

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
	if rule, err := s.rules.FindOvertimeRule(ctx); err == nil {
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

// --- create (F7.2 agent/leader request path) ---

// CreateOvertimeInput is the decoded createOvertimeRequest body (OvertimeWriteRequest).
type CreateOvertimeInput struct {
	EmployeeID       string // empty for an agent caller (server fills from token)
	WorkDate         time.Time
	PlannedStartTime string // "HH:MM"
	PlannedEndTime   string // "HH:MM"
	Reason           string
}

// Create persists an agent/leader OT request (F7.2 / OC-1). The record starts at
// PENDING_L1 (the requester IS the agent, so PENDING_AGENT_CONFIRM is skipped) with
// source REQUESTED. Scope: an agent caller may only request for themselves (403 if a
// different employee_id is supplied). The work_date must fall within an ACTIVE
// placement (OC-6) and must NOT overlap an APPROVED leave (OT_OVERLAPS_LEAVE / 409).
// Placement/company are denormalized from the resolved placement.
func (s *OvertimeService) Create(ctx context.Context, in CreateOvertimeInput) (dom.Overtime, Calculation, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return dom.Overtime{}, Calculation{}, apperr.Unauthenticated()
	}
	// Resolve the subject employee. Agent → forced to self (403 on a mismatch).
	employeeID := in.EmployeeID
	if p.Role == auth.RoleAgent {
		if p.EmployeeID == "" {
			return dom.Overtime{}, Calculation{}, apperr.Forbidden()
		}
		if employeeID != "" && employeeID != p.EmployeeID {
			return dom.Overtime{}, Calculation{}, apperr.Forbidden()
		}
		employeeID = p.EmployeeID
	}
	if employeeID == "" {
		return dom.Overtime{}, Calculation{}, apperr.Invalid(map[string]string{"employee_id": "Wajib diisi."})
	}
	// reason ≥5 (openapi minLength).
	if len([]rune(in.Reason)) < 5 {
		return dom.Overtime{}, Calculation{}, apperr.Invalid(map[string]string{"reason": "Wajib diisi (minimum 5 karakter)."})
	}
	// Validate planned times (HH:MM). end < start ⇒ cross-midnight.
	startMin, serr := parseHHMM(in.PlannedStartTime)
	if serr != nil {
		return dom.Overtime{}, Calculation{}, apperr.Invalid(map[string]string{"planned_start_time": "Format jam tidak valid (HH:MM)."})
	}
	endMin, eerr := parseHHMM(in.PlannedEndTime)
	if eerr != nil {
		return dom.Overtime{}, Calculation{}, apperr.Invalid(map[string]string{"planned_end_time": "Format jam tidak valid (HH:MM)."})
	}
	crossMidnight := endMin < startMin

	// OC-6: work_date must fall within an ACTIVE placement.
	cover, perr := s.schedule.FindActivePlacementForAgentDate(ctx, employeeID, in.WorkDate)
	if errors.Is(perr, domain.ErrNotFound) {
		return dom.Overtime{}, Calculation{}, &apperr.Error{
			HTTPStatus: 422,
			Code:       "OT_NO_SCHEDULED_SHIFT",
			Message:    "Tidak ada penempatan aktif pada tanggal lembur.",
			Fields:     map[string]string{"work_date": in.WorkDate.Format("2006-01-02")},
		}
	}
	if perr != nil {
		return dom.Overtime{}, Calculation{}, apperr.Internal(perr)
	}
	// Leader scope: a shift leader may only request for an agent at their company.
	if p.Role == auth.RoleShiftLeader && cover.CompanyID != p.CompanyID {
		return dom.Overtime{}, Calculation{}, apperr.OutOfScope()
	}

	// OT_OVERLAPS_LEAVE (409): the work_date may not overlap an APPROVED leave.
	if _, lerr := s.schedule.FindApprovedLeaveForAgentDate(ctx, employeeID, in.WorkDate); lerr == nil {
		return dom.Overtime{}, Calculation{}, &apperr.Error{
			HTTPStatus: 409,
			Code:       "OT_OVERLAPS_LEAVE",
			Message:    "Tanggal lembur bertabrakan dengan cuti yang disetujui.",
			Fields:     map[string]string{"work_date": in.WorkDate.Format("2006-01-02")},
		}
	} else if !errors.Is(lerr, domain.ErrNotFound) {
		return dom.Overtime{}, Calculation{}, apperr.Internal(lerr)
	}

	// Resolve the day_type tier (HOLIDAY/RESTDAY/WORKDAY) for the record.
	tier, holidayID := s.ClassifyDayType(ctx, employeeID, in.WorkDate)

	companyID := &cover.CompanyID
	if cover.CompanyID == "" {
		companyID = nil
	}
	pst := in.PlannedStartTime
	pet := in.PlannedEndTime
	reason := in.Reason

	if s.engine == nil {
		return dom.Overtime{}, Calculation{}, apperr.Rule("RULE_VIOLATION", map[string]string{"approval": "Approval engine tidak aktif."})
	}
	var created dom.Overtime
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		// A directly-requested OT (the requester IS the agent) skips
		// PENDING_AGENT_CONFIRM and enters the approval chain immediately: status
		// PENDING + an E11 ApprovalInstance.
		rec, ierr := s.repo.InsertOvertime(ctx, tx, OvertimeInsertParams{
			EmployeeID:       employeeID,
			CompanyID:        companyID,
			PlacementID:      cover.PlacementID,
			WorkDate:         in.WorkDate,
			PlannedStartTime: &pst,
			PlannedEndTime:   &pet,
			CrossMidnight:    crossMidnight,
			Source:           dom.OvertimeSourceRequested,
			Status:           dom.OvertimeStatusPending,
			DayType:          tier,
			HolidayID:        holidayID,
			Reason:           &reason,
			CreatedBy:        actorUserIDPtr(ctx),
		})
		if ierr != nil {
			return ierr
		}
		created = rec
		instanceID, cerr := s.engine.CreateInstance(ctx, tx, approval.CreateInstanceInput{
			RequestType: approval.RequestTypeOvertime,
			RequestID:   rec.ID,
			CompanyID:   deref(companyID),
			RequesterID: employeeID,
		})
		if cerr != nil {
			return cerr
		}
		if lerr := s.repo.SetApprovalInstanceID(ctx, tx, rec.ID, instanceID); lerr != nil {
			return lerr
		}
		created.ApprovalInstanceID = &instanceID
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionCreate,
			EntityType: "overtime",
			EntityID:   rec.ID,
			After:      map[string]any{"status": string(rec.Status), "employee_id": employeeID, "work_date": in.WorkDate.Format("2006-01-02")},
		})
	})
	if err != nil {
		return dom.Overtime{}, Calculation{}, asAppErr(err)
	}
	// Re-read for the full denormalized record + calculation.
	full, calc, gerr := s.Get(ctx, created.ID)
	if gerr != nil {
		return created, s.computeCalculation(ctx, created), nil
	}
	return full, calc, nil
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
	// Agent self-scope: an agent may only read their OWN OT; a cross-employee read
	// is hidden as 404 (no existence leak). Staff (HR/super/leader) use GuardCompany.
	if p, ok := auth.PrincipalFrom(ctx); ok && p.Role == auth.RoleAgent {
		if p.EmployeeID == "" || p.EmployeeID != rec.EmployeeID {
			return dom.Overtime{}, Calculation{}, apperr.NotFound()
		}
	} else if serr := rbac.GuardCompany(ctx, deref(rec.CompanyID)); serr != nil {
		return dom.Overtime{}, Calculation{}, apperr.NotFound()
	}
	// The approval chain is OWNED BY E11 — the client reads it via ApprovalInstanceID.
	return rec, s.computeCalculation(ctx, rec), nil
}

// --- transitions ---

// Confirm forwards PENDING_AGENT_CONFIRM → PENDING (OC-2 / OA-6): the agent confirms
// an auto-detected OT, which enters the E11 approval chain (creates the ApprovalInstance
// + links it). Only the OT's own agent may confirm (openapi x-rbac agent/self); HR/
// leader cannot confirm on behalf → 403. Wrong-state → 409.
func (s *OvertimeService) Confirm(ctx context.Context, id, note string) (dom.Overtime, Calculation, error) {
	_ = note // the agent-confirm marker note is no longer persisted to a decision trail (chain owned by E11)
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
		if serr := s.guardConfirmActor(ctx, rec); serr != nil {
			return serr
		}
		if s.engine == nil {
			return apperr.Rule("RULE_VIOLATION", map[string]string{"approval": "Approval engine tidak aktif."})
		}
		updated, uerr := s.repo.UpdateOvertimeStatus(ctx, tx, id, dom.OvertimeStatusPending)
		if uerr != nil {
			return uerr
		}
		out = updated
		// Confirming enters the E11 chain: create the ApprovalInstance + link it.
		instanceID, cerr := s.engine.CreateInstance(ctx, tx, approval.CreateInstanceInput{
			RequestType: approval.RequestTypeOvertime,
			RequestID:   id,
			CompanyID:   deref(rec.CompanyID),
			RequesterID: rec.EmployeeID,
		})
		if cerr != nil {
			return cerr
		}
		if lerr := s.repo.SetApprovalInstanceID(ctx, tx, id, instanceID); lerr != nil {
			return lerr
		}
		out.ApprovalInstanceID = &instanceID
		return audit.Record(ctx, tx, otAudit(id, string(rec.Status), "PENDING", actor, "CONFIRM"))
	})
	if err != nil {
		return dom.Overtime{}, Calculation{}, asAppErr(err)
	}
	return s.reread(ctx, out)
}

// --- E11 hooks (called by the approval engine on terminal transition, in its tx) ---

// OnApproved finalizes the OT record when its E11 ApprovalInstance reaches a terminal
// APPROVED/BYPASS decision. Runs INSIDE the engine's tx: sets status=APPROVED and
// notifies the submitter. (V1 records HOURS ONLY — INV-2: the reference multiplier is
// never applied; PP35/2021 hour-counting for payroll is deferred to E8, matching the
// prior behavior where day_type/multiplier are stored at create and never recomputed.)
func (s *OvertimeService) OnApproved(ctx context.Context, tx pgx.Tx, requestID string) error {
	rec, lerr := s.repo.GetOvertimeForUpdate(ctx, tx, requestID)
	if errors.Is(lerr, domain.ErrNotFound) {
		return apperr.NotFound()
	}
	if lerr != nil {
		return lerr
	}
	if rec.Status == dom.OvertimeStatusApproved {
		return nil // idempotent
	}
	if rec.Status != dom.OvertimeStatusPending {
		return stateConflict(rec.Status)
	}
	if _, uerr := s.repo.UpdateOvertimeStatus(ctx, tx, requestID, dom.OvertimeStatusApproved); uerr != nil {
		return uerr
	}
	if derr := jobs.Dispatch(ctx, s.notifier, tx, jobs.NotificationArgs{
		NotifKind:        string(reportingdom.NotifOTApproved),
		RecipientID:      rec.EmployeeID,
		Title:            "Lembur disetujui",
		Body:             "Pengajuan lembur Anda (" + rec.WorkDate.Format("2006-01-02") + ") disetujui.",
		DeepLinkEpic:     "E7",
		DeepLinkEntityID: requestID,
		DeepLinkPath:     "/overtime/" + requestID,
		ActorID:          actorUserID(ctx),
		IsCritical:       true,
	}); derr != nil {
		return derr
	}
	return audit.Record(ctx, tx, otAudit(requestID, string(rec.Status), "APPROVED", actorEmployeeID(ctx), "APPROVE"))
}

// OnRejected finalizes the OT record when its E11 ApprovalInstance reaches a terminal
// REJECTED decision. Runs INSIDE the engine's tx: sets status=REJECTED and notifies.
func (s *OvertimeService) OnRejected(ctx context.Context, tx pgx.Tx, requestID string) error {
	rec, lerr := s.repo.GetOvertimeForUpdate(ctx, tx, requestID)
	if errors.Is(lerr, domain.ErrNotFound) {
		return apperr.NotFound()
	}
	if lerr != nil {
		return lerr
	}
	if rec.Status == dom.OvertimeStatusRejected {
		return nil // idempotent
	}
	if rec.Status != dom.OvertimeStatusPending {
		return stateConflict(rec.Status)
	}
	if _, uerr := s.repo.UpdateOvertimeStatus(ctx, tx, requestID, dom.OvertimeStatusRejected); uerr != nil {
		return uerr
	}
	if derr := jobs.Dispatch(ctx, s.notifier, tx, jobs.NotificationArgs{
		NotifKind:        string(reportingdom.NotifOTRejected),
		RecipientID:      rec.EmployeeID,
		Title:            "Lembur ditolak",
		Body:             "Pengajuan lembur Anda ditolak.",
		DeepLinkEpic:     "E7",
		DeepLinkEntityID: requestID,
		DeepLinkPath:     "/overtime/" + requestID,
		ActorID:          actorUserID(ctx),
		IsCritical:       true,
	}); derr != nil {
		return derr
	}
	return audit.Record(ctx, tx, otAudit(requestID, string(rec.Status), "REJECTED", actorEmployeeID(ctx), "REJECT"))
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
		if rec.Status != dom.OvertimeStatusPendingAgentConfirm && rec.Status != dom.OvertimeStatusPending {
			return stateConflict(rec.Status)
		}
		if _, uerr := s.repo.UpdateOvertimeStatus(ctx, tx, id, dom.OvertimeStatusCancelled); uerr != nil {
			return uerr
		}
		// Agent self-action — actor IS the recipient, so no notification. The E11
		// ApprovalInstance (if one exists for a confirmed OT) is left as-is; a CANCELLED
		// request is terminal on the domain side.
		return audit.Record(ctx, tx, otAudit(id, string(rec.Status), "CANCELLED", actor, "WITHDRAW"))
	})
	if err != nil {
		return asAppErr(err)
	}
	return nil
}

// --- day_type classification + OT_BELOW_MIN (exported seams for 09-03 + seed) ---

// ClassifyDayType resolves the OT tier for (employee, work_date):
// GetHolidayForDate → HOLIDAY (+ holiday id); else a live schedule entry → WORKDAY;
// else RESTDAY. TierPrecedence (HOLIDAY>RESTDAY>WORKDAY) resolves overlaps. Returns
// the resolved tier + the matched holiday id (nil unless HOLIDAY). Holidays are
// GLOBAL (decision 2026-06-12) so there is no per-service-line applicability filter.
func (s *OvertimeService) ClassifyDayType(ctx context.Context, employeeID string, workDate time.Time) (dom.OvertimeTier, *string) {
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
func (s *OvertimeService) EnforceMinMinutes(ctx context.Context, countedMinutes int) error {
	rule, err := s.rules.FindOvertimeRule(ctx)
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

// reread re-loads the OT with denormalized names + a fresh calculation. The approval
// chain is OWNED BY E11 (read via ApprovalInstanceID), not assembled here.
func (s *OvertimeService) reread(ctx context.Context, fallback dom.Overtime) (dom.Overtime, Calculation, error) {
	rec := fallback
	if full, err := s.repo.GetOvertime(ctx, fallback.ID); err == nil {
		rec = full
	}
	return rec, s.computeCalculation(ctx, rec), nil
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

