// Package overtime — HolidayService: the HR-maintained public-holiday calendar CRUD
// (HOL-1/HOL-2). Create/update guard HOLIDAY_DATE_CLASH (duplicate date+category);
// delete guards HOLIDAY_IN_USE (referenced by APPROVED OT). Each holiday's
// in_use_by_overtime flag is computed via CountOvertimeUsingHoliday. Every write
// audits in-tx + stubs the notification (TODO Phase-11).
package overtime

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/overtime"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/audit"
)

// HolidayService implements the holiday-calendar CRUD business logic.
type HolidayService struct {
	repo HolidayRepository
	txm  TxRunner
	now  Clock
}

// NewHolidayService wires the holiday service.
func NewHolidayService(repo HolidayRepository, txm TxRunner) *HolidayService {
	return &HolidayService{repo: repo, txm: txm, now: time.Now}
}

// SetClock overrides the time source (tests only).
func (s *HolidayService) SetClock(c Clock) { s.now = c }

// HolidayWriteRequest is the validated create/update payload (openapi
// HolidayWriteRequest). Holidays are GLOBAL ONLY (decision 2026-06-12).
type HolidayWriteRequest struct {
	Name      string
	Date      *time.Time // required on create; optional on partial update
	Category  *string    // required on create; optional on partial update
	Recurring *bool
}

// List returns the holiday page (cursor ASC by date). Each Holiday's
// in_use_by_overtime is computed via CountOvertimeUsingHoliday > 0.
func (s *HolidayService) List(ctx context.Context, f HolidayFilter) ([]dom.Holiday, *string, bool, error) {
	limit := clampLimit(f.Limit)
	f.Limit = limit + 1
	rows, err := s.repo.ListHolidays(ctx, f)
	if err != nil {
		return nil, nil, false, apperr.Internal(err)
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	var next *string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		c, cerr := encodeHolidayCursor(last.Date, last.ID)
		if cerr != nil {
			return nil, nil, false, cerr
		}
		next = &c
	}
	for i := range rows {
		rows[i].InUseByOvertime = s.inUse(ctx, rows[i].ID)
	}
	return rows, next, hasMore, nil
}

// Get loads one holiday with its in_use_by_overtime flag.
func (s *HolidayService) Get(ctx context.Context, id string) (dom.Holiday, error) {
	rec, err := s.repo.GetHoliday(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return dom.Holiday{}, apperr.NotFound()
	}
	if err != nil {
		return dom.Holiday{}, apperr.Internal(err)
	}
	rec.InUseByOvertime = s.inUse(ctx, rec.ID)
	return rec, nil
}

// Create adds a holiday (HOL-2). Validates name (2..120) + date present; pre-checks
// HOLIDAY_DATE_CLASH on (date, category); backstops on the 23505 unique violation.
func (s *HolidayService) Create(ctx context.Context, req HolidayWriteRequest) (dom.Holiday, error) {
	return s.CreateWithID(ctx, nil, req)
}

// CreateWithID is Create with an explicit id (seed/E2E deterministic targets).
func (s *HolidayService) CreateWithID(ctx context.Context, id *string, req HolidayWriteRequest) (dom.Holiday, error) {
	if n := len([]rune(req.Name)); n < 2 || n > 120 {
		return dom.Holiday{}, apperr.Invalid(map[string]string{"name": "Nama wajib 2–120 karakter."})
	}
	if req.Date == nil {
		return dom.Holiday{}, apperr.Invalid(map[string]string{"date": "Tanggal wajib diisi."})
	}
	if req.Category == nil || *req.Category == "" {
		return dom.Holiday{}, apperr.Invalid(map[string]string{"category": "Kategori wajib diisi."})
	}
	// Pre-check clash.
	if _, err := s.repo.GetHolidayByDateCategory(ctx, *req.Date, *req.Category); err == nil {
		return dom.Holiday{}, holidayDateClash()
	} else if !errors.Is(err, domain.ErrNotFound) {
		return dom.Holiday{}, apperr.Internal(err)
	}
	recurring := false
	if req.Recurring != nil {
		recurring = *req.Recurring
	}
	var out dom.Holiday
	err := s.txm.InTx(ctx, func(tx pgx.Tx) error {
		h, ierr := s.repo.InsertHoliday(ctx, tx, HolidayWriteParams{
			ID:        id,
			Name:      req.Name,
			Date:      *req.Date,
			Category:  *req.Category,
			Recurring: recurring,
		})
		if ierr != nil {
			if isUniqueViolation(ierr) {
				return holidayDateClash()
			}
			return ierr
		}
		out = h
		return audit.Record(ctx, tx, holidayAudit(audit.ActionCreate, h.ID, "", "created"))
	})
	if err != nil {
		return dom.Holiday{}, asAppErr(err)
	}
	out.InUseByOvertime = false
	return out, nil
}

// Update partially edits a holiday. A date/category change that clashes →
// HOLIDAY_DATE_CLASH (409).
func (s *HolidayService) Update(ctx context.Context, id string, req HolidayWriteRequest) (dom.Holiday, error) {
	cur, err := s.repo.GetHoliday(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return dom.Holiday{}, apperr.NotFound()
	}
	if err != nil {
		return dom.Holiday{}, apperr.Internal(err)
	}
	if req.Name != "" {
		if n := len([]rune(req.Name)); n < 2 || n > 120 {
			return dom.Holiday{}, apperr.Invalid(map[string]string{"name": "Nama wajib 2–120 karakter."})
		}
	}
	// Resolve the effective (date, category) after the partial change for the clash check.
	newDate := cur.Date
	if req.Date != nil {
		newDate = *req.Date
	}
	newCat := string(cur.Category)
	if req.Category != nil && *req.Category != "" {
		newCat = *req.Category
	}
	if (req.Date != nil || req.Category != nil) &&
		(!newDate.Equal(cur.Date) || newCat != string(cur.Category)) {
		if existing, cerr := s.repo.GetHolidayByDateCategory(ctx, newDate, newCat); cerr == nil {
			if existing.ID != id {
				return dom.Holiday{}, holidayDateClash()
			}
		} else if !errors.Is(cerr, domain.ErrNotFound) {
			return dom.Holiday{}, apperr.Internal(cerr)
		}
	}
	var out dom.Holiday
	err = s.txm.InTx(ctx, func(tx pgx.Tx) error {
		h, uerr := s.repo.UpdateHoliday(ctx, tx, id, HolidayUpdateParams{
			Name:      strOrNil(req.Name),
			Date:      req.Date,
			Category:  req.Category,
			Recurring: req.Recurring,
		})
		if uerr != nil {
			if isUniqueViolation(uerr) {
				return holidayDateClash()
			}
			return uerr
		}
		out = h
		return audit.Record(ctx, tx, holidayAudit(audit.ActionUpdate, id, "edit", "updated"))
	})
	if err != nil {
		return dom.Holiday{}, asAppErr(err)
	}
	out.InUseByOvertime = s.inUse(ctx, out.ID)
	return out, nil
}

// Delete soft-deletes a holiday. Blocked with HOLIDAY_IN_USE (409) when any APPROVED
// OT references it for its tier-HOLIDAY derivation.
func (s *HolidayService) Delete(ctx context.Context, id string) error {
	if _, err := s.repo.GetHoliday(ctx, id); errors.Is(err, domain.ErrNotFound) {
		return apperr.NotFound()
	} else if err != nil {
		return apperr.Internal(err)
	}
	n, err := s.repo.CountOvertimeUsingHoliday(ctx, id)
	if err != nil {
		return apperr.Internal(err)
	}
	if n > 0 {
		return apperr.Conflict("HOLIDAY_IN_USE")
	}
	err = s.txm.InTx(ctx, func(tx pgx.Tx) error {
		if _, derr := s.repo.SoftDeleteHoliday(ctx, tx, id); derr != nil {
			return derr
		}
		return audit.Record(ctx, tx, holidayAudit(audit.ActionDelete, id, "active", "deleted"))
	})
	if err != nil {
		return asAppErr(err)
	}
	return nil
}

// --- helpers ---

func (s *HolidayService) inUse(ctx context.Context, id string) bool {
	n, err := s.repo.CountOvertimeUsingHoliday(ctx, id)
	return err == nil && n > 0
}

func holidayDateClash() error {
	return &apperr.Error{
		Code:       "HOLIDAY_DATE_CLASH",
		HTTPStatus: 409,
		Message:    "Sudah ada hari libur pada tanggal & kategori ini.",
	}
}

func holidayAudit(action audit.Action, id, before, after string) audit.Entry {
	return audit.Entry{
		Action:     action,
		EntityType: "holiday",
		EntityID:   id,
		Before:     map[string]any{"state": before},
		After:      map[string]any{"state": after},
	}
}

// isUniqueViolation reports whether err is a Postgres 23505 (the
// holidays_date_category_uq backstop for HOLIDAY_DATE_CLASH).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
