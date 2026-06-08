// Package scheduling — schedule-entry service (F4.2/F4.3 / SA-*). Grid read,
// single-cell create/update/delete, the side-effect-free :check dry-run, and
// the per-cell-atomic :bulk-apply. Every write runs the shared conflict engine
// before persist, audits in-tx, and stubs the auto-publish notification
// (TODO Phase-11). Mirrors the placement service structure.
package scheduling

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
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
)

// --- schedule repository port ---

// ScheduleRepository is the data dependency for the schedule service. It embeds
// ConflictRepo (the engine's read surface) and adds the grid read + locked
// re-check + writes. Writes take a pgx.Tx.
type ScheduleRepository interface {
	ConflictRepo

	ListSchedule(ctx context.Context, f domain.ScheduleFilter) ([]domain.ScheduleEntry, error)
	// ListScheduleByAgent reads one agent's schedule across all placements (F4.3).
	ListScheduleByAgent(ctx context.Context, employeeID string, start, end time.Time) ([]domain.ScheduleEntry, error)
	// GetActivePlacementCompanyForEmployee resolves the agent's current company
	// (shift-leader scope check for by-agent). domain.ErrNotFound when unplaced.
	GetActivePlacementCompanyForEmployee(ctx context.Context, employeeID string) (string, error)
	GetScheduleEntry(ctx context.Context, id string) (domain.ScheduleEntry, error)
	GetScheduleEntryForUpdate(ctx context.Context, tx pgx.Tx, id string) (domain.ScheduleEntry, error)
	// FindLiveEntryForAgentDateTx re-checks DOUBLE_SHIFT under the row lock.
	FindLiveEntryForAgentDateTx(ctx context.Context, tx pgx.Tx, employeeID string, date time.Time) (LiveEntry, error)
	CreateScheduleEntry(ctx context.Context, tx pgx.Tx, p CreateScheduleEntryParams) (domain.ScheduleEntry, error)
	UpdateScheduleEntry(ctx context.Context, tx pgx.Tx, p UpdateScheduleEntryParams) (domain.ScheduleEntry, error)
	SoftDeleteScheduleEntry(ctx context.Context, tx pgx.Tx, id string) (int64, error)
}

// CreateScheduleEntryParams carries the insert columns.
type CreateScheduleEntryParams struct {
	EmployeeID      string
	PlacementID     string
	ServiceLineID   *string
	ShiftMasterID   *string
	StartTime       *string
	EndTime         *string
	CrossMidnight   bool
	WorkDate        time.Time
	Status          string
	IsDayOff        bool
	ReplacedEntryID *string
	CreatedBy       *string
}

// UpdateScheduleEntryParams carries the update columns.
type UpdateScheduleEntryParams struct {
	ID              string
	ShiftMasterID   *string
	ServiceLineID   *string
	StartTime       *string
	EndTime         *string
	CrossMidnight   bool
	Status          string
	IsDayOff        bool
	ReplacedEntryID *string
}

// ScheduleService implements the schedule business logic.
type ScheduleService struct {
	repo ScheduleRepository
	txm  TxRunner
	now  Clock
}

// NewScheduleService wires the schedule service.
func NewScheduleService(repo ScheduleRepository, txm TxRunner) *ScheduleService {
	return &ScheduleService{repo: repo, txm: txm, now: time.Now}
}

// SetClock overrides the time source (tests only).
func (s *ScheduleService) SetClock(c Clock) { s.now = c }

// today returns the current calendar date in Asia/Jakarta (mirrors placement).
func (s *ScheduleService) today() time.Time {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	n := s.now().In(loc)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc)
}

// --- grid read ---

// ListSchedule returns the company × date-range grid (not paginated; the grid
// always loads a bounded window). Leader scope is enforced on the company.
func (s *ScheduleService) ListSchedule(ctx context.Context, f domain.ScheduleFilter) ([]domain.ScheduleEntry, error) {
	// Shift-leader auto-scope (openapi E4 /schedule): pin company_id to the leader's
	// own company and ignore any client-supplied value, so a leader can never read
	// another company's grid. HR/super-admin keep the requested company_id.
	if p, ok := auth.PrincipalFrom(ctx); ok && p.Role == auth.RoleShiftLeader && p.CompanyID != "" {
		f.CompanyID = p.CompanyID
	}
	if serr := rbac.GuardCompany(ctx, f.CompanyID); serr != nil {
		return nil, serr
	}
	rows, err := s.repo.ListSchedule(ctx, f)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return rows, nil
}

// GetScheduleByAgent returns one agent's schedule across all placements for the
// date window (F4.3 "Jadwal Saya" / E3 placement embed). RBAC scope (SV-1):
//   - agent: ONLY their own employee_id (403 otherwise).
//   - shift_leader: only agents currently placed at the leader's company.
//   - hr_admin / super_admin: any agent.
//
// warnings is empty for the MVP (the envelope is assembled in the handler).
func (s *ScheduleService) GetScheduleByAgent(ctx context.Context, employeeID string, start, end time.Time) ([]domain.ScheduleEntry, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return nil, apperr.Unauthenticated()
	}

	switch p.Role {
	case auth.RoleAgent:
		// SV-1: an agent may only read their own schedule.
		if p.EmployeeID != employeeID {
			return nil, apperr.Forbidden()
		}
	case auth.RoleShiftLeader:
		// Leader is scoped to agents currently placed at their company.
		companyID, err := s.repo.GetActivePlacementCompanyForEmployee(ctx, employeeID)
		if errors.Is(err, domain.ErrNotFound) {
			return nil, apperr.Forbidden()
		}
		if err != nil {
			return nil, apperr.Internal(err)
		}
		if serr := rbac.GuardCompany(ctx, companyID); serr != nil {
			return nil, serr
		}
	default:
		// hr_admin / super_admin: no extra scope.
	}

	rows, err := s.repo.ListScheduleByAgent(ctx, employeeID, start, end)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return rows, nil
}

// --- single-cell create ---

// CreateEntryRequest is the decoded POST /schedule body.
type CreateEntryRequest struct {
	EmployeeID    string
	ShiftMasterID *string
	Date          time.Time
	IsDayOff      bool
	ForceReplace  bool
	CreatedBy     *string
}

// CreateEntry assigns one shift (or a day-off) to one agent on one date. Runs
// the conflict engine; on conflict returns the engine's apperr (code/status/
// details). On force_replace, soft-deletes the existing entry and inserts a
// MODIFIED replacement linked via replaced_entry_id. The DB partial-unique
// index is the race backstop (23505 → DOUBLE_SHIFT).
//
// The over-leave (SHIFT_OVER_LEAVE) check is delivered honestly: Evaluate calls
// repo.FindApprovedLeaveForAgentDate (the real approved_leave_days read source),
// not a faked path. See conflict_engine.go check 5.
func (s *ScheduleService) CreateEntry(ctx context.Context, req CreateEntryRequest) (domain.ScheduleEntry, error) {
	res, err := Evaluate(ctx, s.repo, ConflictInput{
		EmployeeID:    req.EmployeeID,
		ShiftMasterID: req.ShiftMasterID,
		Date:          req.Date,
		IsDayOff:      req.IsDayOff,
		ForceReplace:  req.ForceReplace,
	})
	if err != nil {
		return domain.ScheduleEntry{}, err
	}
	if !res.OK() {
		return domain.ScheduleEntry{}, res.AsError()
	}

	var created domain.ScheduleEntry
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		// Re-check DOUBLE_SHIFT under the partial-unique index predicate.
		if !req.ForceReplace {
			if live, lerr := s.repo.FindLiveEntryForAgentDateTx(ctx, tx, req.EmployeeID, req.Date); lerr == nil {
				details := map[string]any{"existing_entry_id": live.ID}
				if live.ShiftName != nil {
					details["existing_shift_name"] = *live.ShiftName
				}
				return &apperr.Error{Code: "DOUBLE_SHIFT", HTTPStatus: 409,
					Fields: map[string]string{"date": "Agen sudah memiliki shift di tanggal ini."}, Details: details}
			} else if !errors.Is(lerr, domain.ErrNotFound) {
				return lerr
			}
		}

		status := "SCHEDULED"
		var replaced *string
		if res.ExistingEntryID != nil {
			// Replace path: supersede the old entry, mark the new one MODIFIED.
			if _, derr := s.repo.SoftDeleteScheduleEntry(ctx, tx, *res.ExistingEntryID); derr != nil {
				return derr
			}
			status = "MODIFIED"
			replaced = res.ExistingEntryID
		}

		var inErr error
		created, inErr = s.repo.CreateScheduleEntry(ctx, tx, CreateScheduleEntryParams{
			EmployeeID:      req.EmployeeID,
			PlacementID:     res.PlacementID,
			ServiceLineID:   res.ServiceLineID,
			ShiftMasterID:   req.ShiftMasterID,
			StartTime:       res.StartTime,
			EndTime:         res.EndTime,
			CrossMidnight:   res.CrossMidnight,
			WorkDate:        req.Date,
			Status:          status,
			IsDayOff:        req.IsDayOff,
			ReplacedEntryID: replaced,
			CreatedBy:       req.CreatedBy,
		})
		if inErr != nil {
			if isUniqueViolation(inErr) {
				return &apperr.Error{Code: "DOUBLE_SHIFT", HTTPStatus: 409,
					Fields: map[string]string{"date": "Agen sudah memiliki shift di tanggal ini."}}
			}
			return inErr
		}
		// TODO(Phase-11): dispatch "Schedule published" notification (INV-4 auto-publish).
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionCreate,
			EntityType: "schedule_entry",
			EntityID:   created.ID,
			After: map[string]any{
				"employee_id": created.EmployeeID, "work_date": created.WorkDate.Format("2006-01-02"),
				"shift_master_id": created.ShiftMasterID, "status": created.Status,
			},
		})
	}); err != nil {
		return domain.ScheduleEntry{}, asAppErr(err)
	}
	// Re-read for denormalized names (employee/company/shift) on the DTO.
	if full, gerr := s.repo.GetScheduleEntry(ctx, created.ID); gerr == nil {
		return full, nil
	}
	return created, nil
}

// --- single-cell update ---

// UpdateEntryRequest is the decoded PATCH /schedule/{id} body. *Set flags
// distinguish an explicit null (clear) from an absent field.
type UpdateEntryRequest struct {
	ShiftMasterID    *string
	ShiftMasterIDSet bool
	Date             *time.Time
	IsDayOff         *bool
}

// UpdateEntry re-runs the conflict engine for the changed cell and persists a
// MODIFIED entry. (Same conflict codes as create.)
func (s *ScheduleService) UpdateEntry(ctx context.Context, id string, req UpdateEntryRequest) (domain.ScheduleEntry, error) {
	cur, err := s.repo.GetScheduleEntry(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ScheduleEntry{}, apperr.NotFound()
	}
	if err != nil {
		return domain.ScheduleEntry{}, apperr.Internal(err)
	}

	// Resolve the effective target cell after the patch overlay.
	date := cur.WorkDate
	if req.Date != nil {
		date = *req.Date
	}
	isDayOff := cur.IsDayOff
	if req.IsDayOff != nil {
		isDayOff = *req.IsDayOff
	}
	shiftMaster := cur.ShiftMasterID
	if req.ShiftMasterIDSet {
		shiftMaster = req.ShiftMasterID
	}
	if isDayOff {
		shiftMaster = nil
	}

	res, eerr := Evaluate(ctx, s.repo, ConflictInput{
		EmployeeID:    cur.EmployeeID,
		ShiftMasterID: shiftMaster,
		Date:          date,
		IsDayOff:      isDayOff,
		ForceReplace:  true, // updating the agent's own existing cell — not a double-shift block
	})
	if eerr != nil {
		return domain.ScheduleEntry{}, eerr
	}
	if !res.OK() {
		return domain.ScheduleEntry{}, res.AsError()
	}

	var updated domain.ScheduleEntry
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		locked, inErr := s.repo.GetScheduleEntryForUpdate(ctx, tx, id)
		if inErr != nil {
			if errors.Is(inErr, domain.ErrNotFound) {
				return apperr.NotFound()
			}
			return inErr
		}
		_ = locked

		updated, inErr = s.repo.UpdateScheduleEntry(ctx, tx, UpdateScheduleEntryParams{
			ID:            id,
			ShiftMasterID: shiftMaster,
			ServiceLineID: res.ServiceLineID,
			StartTime:     res.StartTime,
			EndTime:       res.EndTime,
			CrossMidnight: res.CrossMidnight,
			Status:        "MODIFIED",
			IsDayOff:      isDayOff,
		})
		if inErr != nil {
			return inErr
		}
		// TODO(Phase-11): dispatch "Schedule modified" notification (CH-2).
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionUpdate,
			EntityType: "schedule_entry",
			EntityID:   updated.ID,
			Before:     map[string]any{"shift_master_id": cur.ShiftMasterID, "is_day_off": cur.IsDayOff, "status": cur.Status},
			After:      map[string]any{"shift_master_id": updated.ShiftMasterID, "is_day_off": updated.IsDayOff, "status": updated.Status},
		})
	}); err != nil {
		return domain.ScheduleEntry{}, asAppErr(err)
	}
	if full, gerr := s.repo.GetScheduleEntry(ctx, updated.ID); gerr == nil {
		return full, nil
	}
	return updated, nil
}

// --- single-cell delete ---

// DeleteEntry hard-removes (soft-deletes) the cell (CH-1). A leader cannot clear
// a past-dated entry (C-5) — attendance (E5) references it.
func (s *ScheduleService) DeleteEntry(ctx context.Context, id string) error {
	cur, err := s.repo.GetScheduleEntry(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return apperr.NotFound()
	}
	if err != nil {
		return apperr.Internal(err)
	}
	// Scope: leader may only act within their own company.
	if serr := rbac.GuardCompany(ctx, cur.CompanyID); serr != nil {
		return serr
	}
	// C-5: a leader cannot clear a past date (HR/super may).
	if p, ok := auth.PrincipalFrom(ctx); ok && p.Role == auth.RoleShiftLeader {
		if cur.WorkDate.Before(s.today()) {
			return apperr.Forbidden()
		}
	}

	return asAppErr(s.txm.InTx(ctx, func(tx pgx.Tx) error {
		n, inErr := s.repo.SoftDeleteScheduleEntry(ctx, tx, id)
		if inErr != nil {
			return inErr
		}
		if n == 0 {
			return apperr.NotFound()
		}
		// TODO(Phase-11): dispatch "Schedule cleared" notification.
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionDelete,
			EntityType: "schedule_entry",
			EntityID:   id,
			Before: map[string]any{
				"employee_id": cur.EmployeeID, "work_date": cur.WorkDate.Format("2006-01-02"),
				"shift_master_id": cur.ShiftMasterID, "status": cur.Status,
			},
		})
	}))
}

// --- :check (dry-run) + :bulk-apply (per-cell atomic) ---

// CellResult is one (agent × date) outcome shared by check + bulk-apply.
type CellResult struct {
	ID         string // set on success
	EmployeeID string
	Date       time.Time
	Status     string // SCHEDULED | MODIFIED (success)
	// Failure:
	Code    string
	Message string
	Details map[string]any
}

// BulkResult bundles the per-cell outcomes.
type BulkResult struct {
	Succeeded []CellResult
	Failed    []CellResult
}

// CheckSingle dry-runs one cell (no writes). Returns the BulkResult envelope.
func (s *ScheduleService) CheckSingle(ctx context.Context, req CreateEntryRequest) (BulkResult, error) {
	return s.checkCells(ctx, []ConflictInput{{
		EmployeeID:    req.EmployeeID,
		ShiftMasterID: req.ShiftMasterID,
		Date:          req.Date,
		IsDayOff:      req.IsDayOff,
		ForceReplace:  req.ForceReplace,
	}})
}

// BulkRequest is the decoded :bulk-apply / bulk :check body.
type BulkRequest struct {
	ShiftMasterID    string
	StartDate        time.Time
	EndDate          time.Time
	EmployeeIDs      []string
	WeekdaysMask     []int // ISO 1=Mon..7=Sun; empty = all
	OverrideExisting bool
	CreatedBy        *string
}

// CheckBulk dry-runs the expanded (employee × date in mask) cells (no writes).
func (s *ScheduleService) CheckBulk(ctx context.Context, req BulkRequest) (BulkResult, error) {
	cells, err := s.expandBulk(req)
	if err != nil {
		return BulkResult{}, err
	}
	return s.checkCells(ctx, cells)
}

// checkCells evaluates each cell with the engine WITHOUT persisting.
func (s *ScheduleService) checkCells(ctx context.Context, cells []ConflictInput) (BulkResult, error) {
	var out BulkResult
	for _, c := range cells {
		res, err := Evaluate(ctx, s.repo, c)
		if err != nil {
			return BulkResult{}, err
		}
		if res.OK() {
			status := "SCHEDULED"
			if res.ExistingEntryID != nil {
				status = "MODIFIED"
			}
			out.Succeeded = append(out.Succeeded, CellResult{
				EmployeeID: c.EmployeeID, Date: c.Date, Status: status,
			})
		} else {
			out.Failed = append(out.Failed, CellResult{
				EmployeeID: c.EmployeeID, Date: c.Date,
				Code: res.Code, Message: conflictMessage(res.Code), Details: res.Details,
			})
		}
	}
	return out, nil
}

// BulkApply applies one shift master across (employee × date in mask). Each cell
// is persisted in its OWN tx — a failing cell does NOT roll back successes.
func (s *ScheduleService) BulkApply(ctx context.Context, req BulkRequest) (BulkResult, error) {
	cells, err := s.expandBulk(req)
	if err != nil {
		return BulkResult{}, err
	}
	var out BulkResult
	for _, c := range cells {
		c.ForceReplace = req.OverrideExisting
		entry, cerr := s.CreateEntry(ctx, CreateEntryRequest{
			EmployeeID:    c.EmployeeID,
			ShiftMasterID: c.ShiftMasterID,
			Date:          c.Date,
			IsDayOff:      c.IsDayOff,
			ForceReplace:  c.ForceReplace,
			CreatedBy:     req.CreatedBy,
		})
		if cerr == nil {
			out.Succeeded = append(out.Succeeded, CellResult{
				ID: entry.ID, EmployeeID: c.EmployeeID, Date: c.Date, Status: entry.Status,
			})
			continue
		}
		// Map the apperr to a failed cell row.
		if ae, ok := apperr.As(cerr); ok {
			out.Failed = append(out.Failed, CellResult{
				EmployeeID: c.EmployeeID, Date: c.Date,
				Code: ae.Code, Message: conflictMessage(ae.Code), Details: detailsMap(ae.Details),
			})
			continue
		}
		return BulkResult{}, cerr
	}
	return out, nil
}

// expandBulk turns a BulkRequest into the per-cell ConflictInput list.
func (s *ScheduleService) expandBulk(req BulkRequest) ([]ConflictInput, error) {
	if req.EndDate.Before(req.StartDate) {
		return nil, apperr.Invalid(map[string]string{"end_date": "Harus >= start_date."})
	}
	if len(req.EmployeeIDs) == 0 {
		return nil, apperr.Invalid(map[string]string{"employee_ids": "Minimal satu agen."})
	}
	mask := map[int]bool{}
	for _, d := range req.WeekdaysMask {
		mask[d] = true
	}
	shiftID := req.ShiftMasterID
	var cells []ConflictInput
	for _, emp := range req.EmployeeIDs {
		for d := req.StartDate; !d.After(req.EndDate); d = d.AddDate(0, 0, 1) {
			if len(mask) > 0 {
				iso := int(d.Weekday())
				if iso == 0 {
					iso = 7 // Sunday → ISO 7
				}
				if !mask[iso] {
					continue
				}
			}
			sid := shiftID
			cells = append(cells, ConflictInput{
				EmployeeID:    emp,
				ShiftMasterID: &sid,
				Date:          d,
				ForceReplace:  req.OverrideExisting,
			})
		}
	}
	return cells, nil
}

// --- shared helpers ---

// conflictMessage returns the Bahasa message for each conflict code.
func conflictMessage(code string) string {
	switch code {
	case "DOUBLE_SHIFT":
		return "Agen sudah memiliki shift di tanggal ini."
	case "SHIFT_OVER_LEAVE":
		return "Agen sedang cuti yang disetujui pada tanggal ini."
	case "OUTSIDE_PLACEMENT_PERIOD":
		return "Tanggal di luar periode penempatan aktif agen."
	case "OUT_OF_SCOPE":
		return "Anda tidak boleh menjadwalkan agen di luar perusahaan yang Anda pimpin."
	case "SHIFT_NOT_FOR_SERVICE_LINE":
		return "Shift dipakai untuk lini layanan lain."
	case "SHIFT_DEACTIVATED":
		return "Shift sudah dinonaktifkan."
	default:
		return "Terjadi konflik penjadwalan."
	}
}

func detailsMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

// isUniqueViolation checks for Postgres error code 23505 (mirrors placement).
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "23505") ||
		strings.Contains(err.Error(), "unique constraint") ||
		strings.Contains(err.Error(), "duplicate key")
}

// asAppErr passes *apperr.Error through, wrapping anything else as 500.
func asAppErr(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := apperr.As(err); ok {
		return err
	}
	return apperr.Internal(err)
}
