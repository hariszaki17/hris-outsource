// Package leave — CalendarService: GET /leave-calendar. Returns leave entries
// overlapping the requested range (a month, or all 12 months of the period).
// show_pending toggles PENDING_L1/PENDING_HR visibility (APPROVED always).
// Leader scope is forced to their led company.
package leave

import (
	"context"
	"time"

	dom "github.com/hariszaki17/hris-outsource/backend/internal/domain/leave"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/apperr"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
)

// CalendarService implements the team leave calendar read.
type CalendarService struct {
	repo LeaveRepository
	now  Clock
}

// NewCalendarService wires the calendar service.
func NewCalendarService(repo LeaveRepository) *CalendarService {
	return &CalendarService{repo: repo, now: time.Now}
}

// SetClock overrides the time source (tests only).
func (s *CalendarService) SetClock(c Clock) { s.now = c }

// CalendarResult is the assembled calendar grid (openapi LeaveCalendarResponse).
type CalendarResult struct {
	Period      int
	Month       *int
	ShowPending bool
	Entries     []dom.LeaveCalendarEntry
}

// Get returns the calendar grid for the requested range.
func (s *CalendarService) Get(ctx context.Context, f CalendarFilter) (CalendarResult, error) {
	p, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return CalendarResult{}, apperr.Unauthenticated()
	}
	if p.Role == auth.RoleShiftLeader {
		if f.CompanyID != nil && *f.CompanyID != p.CompanyID {
			return CalendarResult{}, apperr.OutOfScope()
		}
		cid := p.CompanyID
		f.CompanyID = &cid
	}
	if f.Period == 0 {
		f.Period = s.now().UTC().Year()
	}

	from, to := rangeFor(f.Period, f.Month)
	statusIn := []string{string(dom.LeaveStatusApproved)}
	if f.ShowPending {
		statusIn = append(statusIn, string(dom.LeaveStatusPendingL1), string(dom.LeaveStatusPendingHR))
	}

	entries, err := s.repo.ListCalendarEntries(ctx, f, statusIn, from, to)
	if err != nil {
		return CalendarResult{}, apperr.Internal(err)
	}
	return CalendarResult{
		Period:      f.Period,
		Month:       f.Month,
		ShowPending: f.ShowPending,
		Entries:     entries,
	}, nil
}

// rangeFor returns [from,to] for a month (when set) or the whole calendar year.
func rangeFor(period int, month *int) (time.Time, time.Time) {
	if month != nil && *month >= 1 && *month <= 12 {
		from := time.Date(period, time.Month(*month), 1, 0, 0, 0, 0, time.UTC)
		to := from.AddDate(0, 1, -1)
		return from, to
	}
	return time.Date(period, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(period, 12, 31, 0, 0, 0, 0, time.UTC)
}
