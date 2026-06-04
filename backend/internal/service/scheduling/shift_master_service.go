// Package scheduling — shift-master service (F4.1 / SM-*). List/Get/Create/
// Update/Deactivate/Reactivate. cross_midnight is server-derived (end<=start);
// the break window must fall inside the shift window (BREAK_OUTSIDE_WINDOW);
// the live name is unique (DUPLICATE_NAME via 23505); de/reactivate are
// idempotency-guarded (ALREADY_INACTIVE / ALREADY_ACTIVE). Every write is
// audited in-tx. Mirrors the placement service structure.
package scheduling

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
)

// --- shift-master repository port ---

// ShiftMasterRepository is the data dependency for the shift-master service.
// Reads run on the pool; writes take a pgx.Tx. Name-uniqueness 23505 surfaces
// as a sentinel the service maps to DUPLICATE_NAME.
type ShiftMasterRepository interface {
	ListShiftMasters(ctx context.Context, f domain.ShiftMasterFilter) ([]domain.ShiftMaster, error)
	GetShiftMaster(ctx context.Context, id string) (domain.ShiftMaster, error)
	GetShiftMasterForUpdate(ctx context.Context, tx pgx.Tx, id string) (domain.ShiftMaster, error)
	CreateShiftMaster(ctx context.Context, tx pgx.Tx, p CreateShiftMasterParams) (domain.ShiftMaster, error)
	UpdateShiftMaster(ctx context.Context, tx pgx.Tx, p UpdateShiftMasterParams) (domain.ShiftMaster, error)
	SetShiftMasterActive(ctx context.Context, tx pgx.Tx, id string, active bool) (domain.ShiftMaster, error)
}

// CreateShiftMasterParams carries the columns for an insert (cross_midnight
// derived by the service).
type CreateShiftMasterParams struct {
	Name          string
	StartTime     string
	EndTime       string
	BreakStart    *string
	BreakEnd      *string
	ServiceLineID *string
	CrossMidnight bool
	IsActive      bool
	CreatedBy     *string
}

// UpdateShiftMasterParams carries the full editable column set (the service
// builds it from the current row + the PATCH overlay).
type UpdateShiftMasterParams struct {
	ID            string
	Name          string
	StartTime     string
	EndTime       string
	BreakStart    *string
	BreakEnd      *string
	ServiceLineID *string
	CrossMidnight bool
	IsActive      bool
}

// ShiftMasterService implements the shift-master business logic.
type ShiftMasterService struct {
	repo ShiftMasterRepository
	txm  TxRunner
}

// NewShiftMasterService wires the shift-master service.
func NewShiftMasterService(repo ShiftMasterRepository, txm TxRunner) *ShiftMasterService {
	return &ShiftMasterService{repo: repo, txm: txm}
}

// --- list / get ---

type shiftMasterCursor struct {
	ID string `json:"i"`
}

// ListShiftMasters returns a cursor-paginated page (id-desc keyset).
func (s *ShiftMasterService) ListShiftMasters(ctx context.Context, f domain.ShiftMasterFilter) ([]domain.ShiftMaster, *string, error) {
	limit := httpx.ClampLimit(int(f.Limit))
	f.Limit = int32(limit + 1)

	rows, err := s.repo.ListShiftMasters(ctx, f)
	if err != nil {
		return nil, nil, apperr.Internal(err)
	}

	var next *string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		cur, cerr := httpx.EncodeCursor(shiftMasterCursor{ID: last.ID})
		if cerr != nil {
			return nil, nil, apperr.Internal(cerr)
		}
		next = &cur
	}
	return rows, next, nil
}

// GetShiftMaster returns one shift master by id.
func (s *ShiftMasterService) GetShiftMaster(ctx context.Context, id string) (domain.ShiftMaster, error) {
	m, err := s.repo.GetShiftMaster(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ShiftMaster{}, apperr.NotFound()
	}
	if err != nil {
		return domain.ShiftMaster{}, apperr.Internal(err)
	}
	return m, nil
}

// --- create / update ---

// ShiftMasterWrite is the validated write payload (shared by create/update).
type ShiftMasterWrite struct {
	Name          string
	StartTime     string
	EndTime       string
	BreakStart    *string
	BreakEnd      *string
	ServiceLineID *string
	IsActive      *bool
	CreatedBy     *string
}

// CreateShiftMaster inserts a new template, deriving cross_midnight and
// validating the break window (BREAK_OUTSIDE_WINDOW) + unique name (DUPLICATE_NAME).
func (s *ShiftMasterService) CreateShiftMaster(ctx context.Context, w ShiftMasterWrite) (domain.ShiftMaster, error) {
	if err := validateShiftWindow(w); err != nil {
		return domain.ShiftMaster{}, err
	}
	cross := crossMidnight(w.StartTime, w.EndTime)
	active := true
	if w.IsActive != nil {
		active = *w.IsActive
	}

	var created domain.ShiftMaster
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		var inErr error
		created, inErr = s.repo.CreateShiftMaster(ctx, tx, CreateShiftMasterParams{
			Name:          w.Name,
			StartTime:     w.StartTime,
			EndTime:       w.EndTime,
			BreakStart:    w.BreakStart,
			BreakEnd:      w.BreakEnd,
			ServiceLineID: w.ServiceLineID,
			CrossMidnight: cross,
			IsActive:      active,
			CreatedBy:     w.CreatedBy,
		})
		if inErr != nil {
			if isUniqueViolation(inErr) {
				return duplicateNameErr(w.Name)
			}
			return inErr
		}
		// TODO(Phase-11): dispatch "Shift master created" notification.
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionCreate,
			EntityType: "shift_master",
			EntityID:   created.ID,
			After:      map[string]any{"name": created.Name, "is_active": created.IsActive},
		})
	}); err != nil {
		return domain.ShiftMaster{}, asAppErr(err)
	}
	return created, nil
}

// UpdateShiftMaster overwrites the editable fields (PATCH overlay onto the
// current row), re-deriving cross_midnight and re-validating the break window.
func (s *ShiftMasterService) UpdateShiftMaster(ctx context.Context, id string, p ShiftMasterPatch) (domain.ShiftMaster, error) {
	var updated domain.ShiftMaster
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		cur, inErr := s.repo.GetShiftMasterForUpdate(ctx, tx, id)
		if inErr != nil {
			if errors.Is(inErr, domain.ErrNotFound) {
				return apperr.NotFound()
			}
			return inErr
		}

		// Overlay PATCH fields onto the current row.
		w := ShiftMasterWrite{
			Name:          cur.Name,
			StartTime:     cur.StartTime,
			EndTime:       cur.EndTime,
			BreakStart:    cur.BreakStart,
			BreakEnd:      cur.BreakEnd,
			ServiceLineID: cur.ServiceLineID,
		}
		if p.Name != nil {
			w.Name = *p.Name
		}
		if p.StartTime != nil {
			w.StartTime = *p.StartTime
		}
		if p.EndTime != nil {
			w.EndTime = *p.EndTime
		}
		if p.BreakStartSet {
			w.BreakStart = p.BreakStart
		}
		if p.BreakEndSet {
			w.BreakEnd = p.BreakEnd
		}
		if p.ServiceLineIDSet {
			w.ServiceLineID = p.ServiceLineID
		}
		isActive := cur.IsActive
		if p.IsActive != nil {
			isActive = *p.IsActive
		}

		if verr := validateShiftWindow(w); verr != nil {
			return verr
		}
		cross := crossMidnight(w.StartTime, w.EndTime)

		updated, inErr = s.repo.UpdateShiftMaster(ctx, tx, UpdateShiftMasterParams{
			ID:            id,
			Name:          w.Name,
			StartTime:     w.StartTime,
			EndTime:       w.EndTime,
			BreakStart:    w.BreakStart,
			BreakEnd:      w.BreakEnd,
			ServiceLineID: w.ServiceLineID,
			CrossMidnight: cross,
			IsActive:      isActive,
		})
		if inErr != nil {
			if isUniqueViolation(inErr) {
				return duplicateNameErr(w.Name)
			}
			return inErr
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     audit.ActionUpdate,
			EntityType: "shift_master",
			EntityID:   updated.ID,
			Before:     map[string]any{"name": cur.Name, "start_time": cur.StartTime, "end_time": cur.EndTime},
			After:      map[string]any{"name": updated.Name, "start_time": updated.StartTime, "end_time": updated.EndTime},
		})
	}); err != nil {
		return domain.ShiftMaster{}, asAppErr(err)
	}
	// Re-read for the denormalized service_line_name + in_use_count on the DTO.
	full, gerr := s.repo.GetShiftMaster(ctx, updated.ID)
	if gerr == nil {
		return full, nil
	}
	return updated, nil
}

// ShiftMasterPatch is the partial-update payload. The *Set flags distinguish an
// explicit JSON null (clear) from an absent field (keep).
type ShiftMasterPatch struct {
	Name             *string
	StartTime        *string
	EndTime          *string
	BreakStart       *string
	BreakStartSet    bool
	BreakEnd         *string
	BreakEndSet      bool
	ServiceLineID    *string
	ServiceLineIDSet bool
	IsActive         *bool
}

// --- deactivate / reactivate ---

// DeactivateShiftMaster sets is_active=false (SM-5). 409 ALREADY_INACTIVE when
// already inactive.
func (s *ShiftMasterService) DeactivateShiftMaster(ctx context.Context, id string) (domain.ShiftMaster, error) {
	return s.setActive(ctx, id, false)
}

// ReactivateShiftMaster sets is_active=true. 409 ALREADY_ACTIVE when already active.
func (s *ShiftMasterService) ReactivateShiftMaster(ctx context.Context, id string) (domain.ShiftMaster, error) {
	return s.setActive(ctx, id, true)
}

func (s *ShiftMasterService) setActive(ctx context.Context, id string, active bool) (domain.ShiftMaster, error) {
	var out domain.ShiftMaster
	if err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		cur, inErr := s.repo.GetShiftMasterForUpdate(ctx, tx, id)
		if inErr != nil {
			if errors.Is(inErr, domain.ErrNotFound) {
				return apperr.NotFound()
			}
			return inErr
		}
		if cur.IsActive == active {
			if active {
				return apperr.Conflict("ALREADY_ACTIVE")
			}
			return apperr.Conflict("ALREADY_INACTIVE")
		}
		out, inErr = s.repo.SetShiftMasterActive(ctx, tx, id, active)
		if inErr != nil {
			return inErr
		}
		// TODO(Phase-11): dispatch "Shift master status changed" notification.
		action := audit.Action("shift_master.deactivate")
		if active {
			action = audit.Action("shift_master.reactivate")
		}
		return audit.Record(ctx, tx, audit.Entry{
			Action:     action,
			EntityType: "shift_master",
			EntityID:   out.ID,
			Before:     map[string]any{"is_active": cur.IsActive},
			After:      map[string]any{"is_active": out.IsActive},
		})
	}); err != nil {
		return domain.ShiftMaster{}, asAppErr(err)
	}
	// Re-read for in_use_count + service_line_name on the response DTO.
	if full, gerr := s.repo.GetShiftMaster(ctx, out.ID); gerr == nil {
		return full, nil
	}
	return out, nil
}

// --- validation helpers ---

// crossMidnight is true when end_time <= start_time (lexicographic HH:MM compare
// is correct for zero-padded 24h strings).
func crossMidnight(start, end string) bool { return end <= start }

// validateShiftWindow enforces the break window inside [start,end] (SM-1).
// Both break_start and break_end must be present together; for a same-day shift
// the break must lie strictly inside the window; cross-midnight shifts skip the
// strict-inside check (the window wraps).
func validateShiftWindow(w ShiftMasterWrite) error {
	if !validHHMM(w.StartTime) {
		return apperr.Rule("BREAK_OUTSIDE_WINDOW", map[string]string{"start_time": "Format jam tidak valid (HH:MM)."})
	}
	if !validHHMM(w.EndTime) {
		return apperr.Rule("BREAK_OUTSIDE_WINDOW", map[string]string{"end_time": "Format jam tidak valid (HH:MM)."})
	}
	hasStart := w.BreakStart != nil && *w.BreakStart != ""
	hasEnd := w.BreakEnd != nil && *w.BreakEnd != ""
	if !hasStart && !hasEnd {
		return nil
	}
	if hasStart != hasEnd {
		return apperr.Rule("BREAK_OUTSIDE_WINDOW", map[string]string{
			"break_start": "Jam mulai dan selesai istirahat harus diisi bersamaan.",
		})
	}
	bs, be := *w.BreakStart, *w.BreakEnd
	if !validHHMM(bs) || !validHHMM(be) || be <= bs {
		return apperr.Rule("BREAK_OUTSIDE_WINDOW", map[string]string{
			"break_start": "Harus berada di antara start_time dan end_time.",
		})
	}
	// Same-day shift: break must lie inside the working window.
	if !crossMidnight(w.StartTime, w.EndTime) {
		if bs < w.StartTime || be > w.EndTime {
			return apperr.Rule("BREAK_OUTSIDE_WINDOW", map[string]string{
				"break_start": "Jendela istirahat di luar jam kerja shift.",
			})
		}
	}
	return nil
}

func validHHMM(s string) bool {
	if len(s) != 5 || s[2] != ':' {
		return false
	}
	h := (s[0]-'0')*10 + (s[1] - '0')
	m := (s[3]-'0')*10 + (s[4] - '0')
	if s[0] < '0' || s[0] > '9' || s[1] < '0' || s[1] > '9' || s[3] < '0' || s[3] > '9' || s[4] < '0' || s[4] > '9' {
		return false
	}
	return h <= 23 && m <= 59
}

func duplicateNameErr(name string) error {
	return &apperr.Error{
		Code:       "DUPLICATE_NAME",
		Fields:     map[string]string{"name": "Sudah ada shift dengan nama '" + name + "'."},
		HTTPStatus: 409,
	}
}
