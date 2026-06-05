// Package overtime (repository) — HolidayRepo implements svc.HolidayRepository
// over the 09-01 sqlc holidays queries. Reads on the pool; writes via q.WithTx(tx).
package overtime

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/overtime"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	sqlcgen "github.com/hariszaki17/hris-outsource/backend/internal/repository/sqlc"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/overtime"
)

// HolidayRepo is the sqlc-backed implementation of svc.HolidayRepository.
type HolidayRepo struct {
	pool *db.Pool
	q    *sqlcgen.Queries
}

var _ svc.HolidayRepository = (*HolidayRepo)(nil)

// NewHolidayRepo returns a HolidayRepo backed by pool.
func NewHolidayRepo(pool *db.Pool) *HolidayRepo {
	return &HolidayRepo{pool: pool, q: sqlcgen.New(pool.Pool)}
}

// --- list / get ---

func (r *HolidayRepo) ListHolidays(ctx context.Context, f svc.HolidayFilter) ([]dom.Holiday, error) {
	p := sqlcgen.ListHolidaysParams{
		Category:      strptr(f.Category),
		ServiceLineID: strptr(f.ServiceLineID),
		CursorID:      f.CursorID,
		Lim:           i32(f.Limit),
	}
	if f.Year != nil {
		y := int32(*f.Year)
		p.Year = &y
	}
	if f.CursorDate != nil {
		p.CursorDate = timeToPgDate(*f.CursorDate)
	}
	rows, err := r.q.ListHolidays(ctx, p)
	if err != nil {
		return nil, err
	}
	out := make([]dom.Holiday, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapHolidayFromList(row))
	}
	return out, nil
}

func (r *HolidayRepo) GetHoliday(ctx context.Context, id string) (dom.Holiday, error) {
	row, err := r.q.GetHoliday(ctx, id)
	if err != nil {
		return dom.Holiday{}, mapErr(err)
	}
	return mapHolidayFromGet(row), nil
}

func (r *HolidayRepo) GetHolidayByDateCategory(ctx context.Context, date time.Time, category string) (dom.Holiday, error) {
	row, err := r.q.GetHolidayByDateCategory(ctx, sqlcgen.GetHolidayByDateCategoryParams{
		HolidayDate: timeToPgDate(date),
		Category:    category,
	})
	if err != nil {
		return dom.Holiday{}, mapErr(err)
	}
	return mapHolidayFromByDateCategory(row), nil
}

func (r *HolidayRepo) GetHolidayForDate(ctx context.Context, date time.Time) (dom.Holiday, error) {
	row, err := r.q.GetHolidayForDate(ctx, timeToPgDate(date))
	if err != nil {
		return dom.Holiday{}, mapErr(err)
	}
	return mapHolidayFromForDate(row), nil
}

// --- writes ---

func (r *HolidayRepo) InsertHoliday(ctx context.Context, tx pgx.Tx, p svc.HolidayWriteParams) (dom.Holiday, error) {
	lines := p.ApplicableServiceLines
	if lines == nil {
		lines = []string{}
	}
	row, err := r.q.WithTx(tx).InsertHoliday(ctx, sqlcgen.InsertHolidayParams{
		ID:                     p.ID,
		Name:                   p.Name,
		HolidayDate:            timeToPgDate(p.Date),
		Category:               p.Category,
		Recurring:              p.Recurring,
		ApplicableServiceLines: lines,
	})
	if err != nil {
		return dom.Holiday{}, mapErr(err)
	}
	return mapHolidayFromInsert(row), nil
}

func (r *HolidayRepo) UpdateHoliday(ctx context.Context, tx pgx.Tx, id string, p svc.HolidayUpdateParams) (dom.Holiday, error) {
	params := sqlcgen.UpdateHolidayParams{
		Name:                   p.Name,
		Category:               p.Category,
		Recurring:              p.Recurring,
		ApplicableServiceLines: p.ApplicableServiceLines,
		ID:                     id,
	}
	if p.Date != nil {
		params.HolidayDate = timeToPgDate(*p.Date)
	}
	row, err := r.q.WithTx(tx).UpdateHoliday(ctx, params)
	if err != nil {
		return dom.Holiday{}, mapErr(err)
	}
	return mapHolidayFromUpdate(row), nil
}

func (r *HolidayRepo) SoftDeleteHoliday(ctx context.Context, tx pgx.Tx, id string) (string, error) {
	deleted, err := r.q.WithTx(tx).SoftDeleteHoliday(ctx, id)
	if err != nil {
		return "", mapErr(err)
	}
	return deleted, nil
}

func (r *HolidayRepo) CountOvertimeUsingHoliday(ctx context.Context, holidayID string) (int64, error) {
	id := holidayID
	return r.q.CountOvertimeUsingHoliday(ctx, &id)
}
