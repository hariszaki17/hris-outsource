// Package leave — CalendarService: GET /leave-calendar. Returns leave entries
// overlapping the requested range (a month, or all 12 months of the period) plus
// per-day service-line-aware coverage clashes (≥2 same-line agents off at the same
// company). show_pending toggles PENDING_L1/PENDING_HR visibility (APPROVED always).
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
	Clashes     []CalendarClash
}

// CalendarClash is one service-line-aware coverage clash day.
type CalendarClash struct {
	Date            string
	CompanyID       string
	CompanyName     *string
	ServiceLine     string
	AgentCount      int
	LeaveRequestIDs []string
}

// Get returns the calendar grid for the requested range + clash flags.
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
		Clashes:     computeClashes(entries),
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

// computeClashes flags (date, company, service_line) triples where ≥2 same-line
// agents are off at the same client company on the same day.
func computeClashes(entries []dom.LeaveCalendarEntry) []CalendarClash {
	type key struct{ date, company, line string }
	type agg struct {
		companyName *string
		requestIDs  []string
		employees   map[string]bool
	}
	buckets := map[key]*agg{}

	for _, e := range entries {
		company := ""
		if e.CompanyID != nil {
			company = *e.CompanyID
		}
		line := ""
		if e.ServiceLine != nil {
			line = *e.ServiceLine
		}
		if company == "" || line == "" {
			continue
		}
		for d := e.StartDate; !d.After(e.EndDate); d = d.AddDate(0, 0, 1) {
			k := key{date: d.Format("2006-01-02"), company: company, line: line}
			b := buckets[k]
			if b == nil {
				b = &agg{companyName: e.CompanyName, employees: map[string]bool{}}
				buckets[k] = b
			}
			if !b.employees[e.EmployeeID] {
				b.employees[e.EmployeeID] = true
				b.requestIDs = append(b.requestIDs, e.LeaveRequestID)
			}
		}
	}

	var out []CalendarClash
	for k, b := range buckets {
		if len(b.employees) >= 2 {
			out = append(out, CalendarClash{
				Date:            k.date,
				CompanyID:       k.company,
				CompanyName:     b.companyName,
				ServiceLine:     k.line,
				AgentCount:      len(b.employees),
				LeaveRequestIDs: b.requestIDs,
			})
		}
	}
	return out
}
