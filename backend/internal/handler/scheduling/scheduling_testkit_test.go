// Package scheduling_test — E4 shift-master + schedule contract tests.
//
// Shared test plumbing: a fakeTx (Exec no-op so audit.Record works inside InTx),
// a fakeTxRunner, in-memory fake repos implementing the 06-02 service ports, and a
// newHarness that mounts the real scheduling.Service + scheduling.Handler over the
// fakes on a chi.Router with a mutable principal middleware (swap roles per case).
//
// Mirrors internal/handler/placement/placements_handler_test.go EXACTLY: same
// fakeTx shape, same decodeBody helper, same dynamic-principal injection pattern.
package scheduling_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hariszaki17/hris-outsource/backend/internal/domain"
	schedulinghandler "github.com/hariszaki17/hris-outsource/backend/internal/handler/scheduling"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/httpx"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/rbac"
	svc "github.com/hariszaki17/hris-outsource/backend/internal/service/scheduling"
)

// ---------------------------------------------------------------------------
// fakeTx — only Exec is needed (audit.Record); all other methods panic.
// ---------------------------------------------------------------------------

type fakeTx struct{}

func (f *fakeTx) Begin(_ context.Context) (pgx.Tx, error) { return f, nil }
func (f *fakeTx) Commit(_ context.Context) error          { return nil }
func (f *fakeTx) Rollback(_ context.Context) error        { return nil }
func (f *fakeTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (f *fakeTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	panic("fakeTx: Query not implemented")
}
func (f *fakeTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	panic("fakeTx: QueryRow not implemented")
}
func (f *fakeTx) CopyFrom(_ context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
	panic("fakeTx: CopyFrom not implemented")
}
func (f *fakeTx) SendBatch(_ context.Context, _ *pgx.Batch) pgx.BatchResults {
	panic("fakeTx: SendBatch not implemented")
}
func (f *fakeTx) LargeObjects() pgx.LargeObjects {
	panic("fakeTx: LargeObjects not implemented")
}
func (f *fakeTx) Prepare(_ context.Context, _, _ string) (*pgconn.StatementDescription, error) {
	panic("fakeTx: Prepare not implemented")
}
func (f *fakeTx) Conn() *pgx.Conn { return nil }

var _ pgx.Tx = (*fakeTx)(nil)

type fakeTxRunner struct{}

func (f *fakeTxRunner) InTx(_ context.Context, fn func(pgx.Tx) error) error {
	return fn(&fakeTx{})
}

// ---------------------------------------------------------------------------
// shared helpers
// ---------------------------------------------------------------------------

func decodeBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&m); err != nil {
		t.Fatalf("decode response body: %v\nbody: %s", err, rr.Body.String())
	}
	return m
}

func errObject(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	e, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("response has no error object: %v", body)
	}
	return e
}

func errCode(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	return strOf(errObject(t, decodeBody(t, rr))["code"])
}

func strOf(v any) string {
	s, _ := v.(string)
	return s
}

func strp(s string) *string { return &s }

func itoa(n int) string { return strconv.Itoa(n) }

// fixedNow is the deterministic clock for all scheduling tests (Asia/Jakarta-safe;
// 12:00 WIB on 2026-06-04 — mirrors the placement test clock).
var fixedNow = time.Date(2026, 6, 4, 5, 0, 0, 0, time.UTC)

// ymd is a YYYY-MM-DD date in UTC (matches the seed fixture convention).
func ymd(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// dateStr formats a time as the API "YYYY-MM-DD".
func dateStr(t time.Time) string { return t.Format("2006-01-02") }

// ---------------------------------------------------------------------------
// fakeShiftMasterRepo — in-memory svc.ShiftMasterRepository.
//
// nameIndex backs the live-name uniqueness; a Create/Update colliding on an
// already-used name returns a sentinel whose Error() contains "23505" so the
// service maps it to DUPLICATE_NAME (mirrors the real 23505 path).
// ---------------------------------------------------------------------------

type fakeShiftMasterRepo struct {
	masters   map[string]domain.ShiftMaster
	nameIndex map[string]string // live name -> id
	seq       int

	// sched is the sibling schedule repo (set by newFakeScheduleRepo) so the
	// shift-time propagation can read its entries + attendance and write them back.
	sched *fakeScheduleRepo
}

func newFakeShiftMasterRepo() *fakeShiftMasterRepo {
	return &fakeShiftMasterRepo{
		masters:   map[string]domain.ShiftMaster{},
		nameIndex: map[string]string{},
	}
}

type uniqueViolationErr struct{ msg string }

func (e uniqueViolationErr) Error() string { return e.msg }

func (r *fakeShiftMasterRepo) ListShiftMasters(_ context.Context, f domain.ShiftMasterFilter) ([]domain.ShiftMaster, error) {
	var out []domain.ShiftMaster
	for _, m := range r.masters {
		if f.Status != nil {
			want := *f.Status == "ACTIVE"
			if m.IsActive != want {
				continue
			}
		}
		out = append(out, m)
	}
	// id-desc keyset (mirror the real ListShiftMasters cursor).
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	if f.Cursor != nil {
		var trimmed []domain.ShiftMaster
		for _, m := range out {
			if m.ID < *f.Cursor {
				trimmed = append(trimmed, m)
			}
		}
		out = trimmed
	}
	if f.Limit > 0 && len(out) > int(f.Limit) {
		out = out[:f.Limit]
	}
	return out, nil
}

func (r *fakeShiftMasterRepo) GetShiftMaster(_ context.Context, id string) (domain.ShiftMaster, error) {
	m, ok := r.masters[id]
	if !ok {
		return domain.ShiftMaster{}, domain.ErrNotFound
	}
	return m, nil
}

func (r *fakeShiftMasterRepo) GetShiftMasterForUpdate(_ context.Context, _ pgx.Tx, id string) (domain.ShiftMaster, error) {
	return r.GetShiftMaster(context.Background(), id)
}

func (r *fakeShiftMasterRepo) CreateShiftMaster(_ context.Context, _ pgx.Tx, p svc.CreateShiftMasterParams) (domain.ShiftMaster, error) {
	if _, taken := r.nameIndex[p.Name]; taken {
		return domain.ShiftMaster{}, uniqueViolationErr{msg: "ERROR: duplicate key value violates unique constraint (SQLSTATE 23505)"}
	}
	r.seq++
	id := "SWP-SHF-" + itoa(7000+r.seq)
	m := domain.ShiftMaster{
		ID:            id,
		Name:          p.Name,
		StartTime:     p.StartTime,
		EndTime:       p.EndTime,
		BreakStart:    p.BreakStart,
		BreakEnd:      p.BreakEnd,
		CrossMidnight: p.CrossMidnight,
		IsActive:      p.IsActive,
		CreatedBy:     p.CreatedBy,
		CreatedAt:     fixedNow,
		UpdatedAt:     fixedNow,
	}
	r.masters[id] = m
	r.nameIndex[p.Name] = id
	return m, nil
}

func (r *fakeShiftMasterRepo) UpdateShiftMaster(_ context.Context, _ pgx.Tx, p svc.UpdateShiftMasterParams) (domain.ShiftMaster, error) {
	cur, ok := r.masters[p.ID]
	if !ok {
		return domain.ShiftMaster{}, domain.ErrNotFound
	}
	if other, taken := r.nameIndex[p.Name]; taken && other != p.ID {
		return domain.ShiftMaster{}, uniqueViolationErr{msg: "ERROR: duplicate key value violates unique constraint (SQLSTATE 23505)"}
	}
	delete(r.nameIndex, cur.Name)
	cur.Name = p.Name
	cur.StartTime = p.StartTime
	cur.EndTime = p.EndTime
	cur.BreakStart = p.BreakStart
	cur.BreakEnd = p.BreakEnd
	cur.CrossMidnight = p.CrossMidnight
	cur.IsActive = p.IsActive
	cur.UpdatedAt = fixedNow
	r.masters[p.ID] = cur
	r.nameIndex[cur.Name] = p.ID
	return cur, nil
}

func (r *fakeShiftMasterRepo) SetShiftMasterActive(_ context.Context, _ pgx.Tx, id string, active bool) (domain.ShiftMaster, error) {
	cur, ok := r.masters[id]
	if !ok {
		return domain.ShiftMaster{}, domain.ErrNotFound
	}
	cur.IsActive = active
	cur.UpdatedAt = fixedNow
	r.masters[id] = cur
	return cur, nil
}

// --- PropagationRepo (SM-2 time-change ripple → entries + attendance) ---

func (r *fakeShiftMasterRepo) ListPropagationCandidates(_ context.Context, masterID string, now time.Time) ([]svc.PropagationCandidate, error) {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	jn := now.In(loc)
	today := time.Date(jn.Year(), jn.Month(), jn.Day(), 0, 0, 0, 0, time.UTC)

	var out []svc.PropagationCandidate
	if r.sched == nil {
		return out, nil
	}
	for _, e := range r.sched.entries {
		if e.ShiftMasterID == nil || *e.ShiftMasterID != masterID {
			continue
		}
		if e.IsDayOff || e.Status == "CANCELLED_BY_LEAVE" {
			continue
		}
		// work_date >= today (Asia/Jakarta). Entries are stored at UTC midnight
		// (ymd helper), so compare dates directly.
		ed := time.Date(e.WorkDate.Year(), e.WorkDate.Month(), e.WorkDate.Day(), 0, 0, 0, 0, time.UTC)
		if ed.Before(today) {
			continue
		}
		st, et := e.StartTime, e.EndTime
		cand := svc.PropagationCandidate{
			EntryID:       e.ID,
			WorkDate:      e.WorkDate,
			StartTime:     st,
			EndTime:       et,
			CrossMidnight: e.CrossMidnight,
		}
		if att, ok := r.sched.attendance[e.ID]; ok {
			cand.CheckInAt = att.checkIn
			cand.CheckOutAt = att.checkOut
		}
		out = append(out, cand)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EntryID < out[j].EntryID })
	return out, nil
}

func (r *fakeShiftMasterRepo) UpdateScheduleEntryTimes(_ context.Context, _ pgx.Tx, entryID, startTime, endTime string, cross bool) error {
	if r.sched == nil {
		return nil
	}
	e, ok := r.sched.entries[entryID]
	if !ok {
		return nil
	}
	s, en := startTime, endTime
	e.StartTime = &s
	e.EndTime = &en
	e.CrossMidnight = cross
	r.sched.entries[entryID] = e
	return nil
}

func (r *fakeShiftMasterRepo) SyncOpenAttendanceShiftEnd(_ context.Context, _ pgx.Tx, scheduleID string, shiftEndAt time.Time) error {
	if r.sched == nil {
		return nil
	}
	att, ok := r.sched.attendance[scheduleID]
	if !ok || att.checkOut != nil {
		return nil // no open attendance row → no-op (mirrors the SQL guard)
	}
	end := shiftEndAt
	att.shiftEndAt = &end
	r.sched.attendance[scheduleID] = att
	return nil
}

var _ svc.ShiftMasterRepository = (*fakeShiftMasterRepo)(nil)

// fakeAttendance is the minimal attendance state the propagation tests need: the
// check-in/out instants (drive the freeze branch) + the shift_end_at the E4→E5
// sync writes.
type fakeAttendance struct {
	checkIn    *time.Time
	checkOut   *time.Time
	shiftEndAt *time.Time
}

// ---------------------------------------------------------------------------
// fakeScheduleRepo — in-memory svc.ScheduleRepository (embeds the engine's
// ConflictRepo read surface). Backs placements, masters, approved-leave, and
// live entries so each conflict branch can be triggered deterministically.
// ---------------------------------------------------------------------------

type fakeScheduleRepo struct {
	masters *fakeShiftMasterRepo // shared catalog (engine GetShiftMaster)

	entries map[string]domain.ScheduleEntry // id -> entry

	// placements[employeeID] = the active placement cover.
	placements map[string]svc.PlacementCover
	// approvedLeave[employeeID+"|"+YYYY-MM-DD] = the approved-leave row.
	approvedLeave map[string]svc.ApprovedLeave
	// liveEntry[employeeID+"|"+YYYY-MM-DD] = the existing live entry id.
	liveEntry map[string]svc.LiveEntry
	// attendance[entryID] = the (≤1) attendance row linked to a schedule entry
	// (drives the shift-time propagation freeze branches).
	attendance map[string]fakeAttendance

	seq int
}

func newFakeScheduleRepo(masters *fakeShiftMasterRepo) *fakeScheduleRepo {
	r := &fakeScheduleRepo{
		masters:       masters,
		entries:       map[string]domain.ScheduleEntry{},
		placements:    map[string]svc.PlacementCover{},
		approvedLeave: map[string]svc.ApprovedLeave{},
		liveEntry:     map[string]svc.LiveEntry{},
		attendance:    map[string]fakeAttendance{},
	}
	masters.sched = r // let the master repo's propagation reach the entries/attendance
	return r
}

func leaveKey(empID string, date time.Time) string { return empID + "|" + dateStr(date) }

// --- ConflictRepo (engine read surface) ---

func (r *fakeScheduleRepo) FindActivePlacementForAgentDate(_ context.Context, employeeID string, date time.Time) (svc.PlacementCover, error) {
	p, ok := r.placements[employeeID]
	if !ok {
		return svc.PlacementCover{}, domain.ErrNotFound
	}
	// Honour the placement window (INV-2): no cover when date is outside it.
	if date.Before(p.StartDate) {
		return svc.PlacementCover{}, domain.ErrNotFound
	}
	if p.EndDate != nil && date.After(*p.EndDate) {
		return svc.PlacementCover{}, domain.ErrNotFound
	}
	return p, nil
}

func (r *fakeScheduleRepo) GetShiftMaster(ctx context.Context, id string) (domain.ShiftMaster, error) {
	return r.masters.GetShiftMaster(ctx, id)
}

func (r *fakeScheduleRepo) FindApprovedLeaveForAgentDate(_ context.Context, employeeID string, date time.Time) (svc.ApprovedLeave, error) {
	l, ok := r.approvedLeave[leaveKey(employeeID, date)]
	if !ok {
		return svc.ApprovedLeave{}, domain.ErrNotFound
	}
	return l, nil
}

func (r *fakeScheduleRepo) FindLiveEntryForAgentDate(_ context.Context, employeeID string, date time.Time) (svc.LiveEntry, error) {
	e, ok := r.liveEntry[leaveKey(employeeID, date)]
	if !ok {
		return svc.LiveEntry{}, domain.ErrNotFound
	}
	return e, nil
}

// --- ScheduleRepository (grid + writes) ---

func (r *fakeScheduleRepo) ListSchedule(_ context.Context, f domain.ScheduleFilter) ([]domain.ScheduleEntry, error) {
	var out []domain.ScheduleEntry
	for _, e := range r.entries {
		if e.CompanyID != f.CompanyID {
			continue
		}
		if e.WorkDate.Before(f.StartDate) || e.WorkDate.After(f.EndDate) {
			continue
		}
		if f.EmployeeID != nil && e.EmployeeID != *f.EmployeeID {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *fakeScheduleRepo) ListScheduleByAgent(_ context.Context, employeeID string, start, end time.Time) ([]domain.ScheduleEntry, error) {
	var out []domain.ScheduleEntry
	for _, e := range r.entries {
		if e.EmployeeID != employeeID {
			continue
		}
		if e.WorkDate.Before(start) || e.WorkDate.After(end) {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *fakeScheduleRepo) GetActivePlacementCompanyForEmployee(_ context.Context, employeeID string) (string, error) {
	p, ok := r.placements[employeeID]
	if !ok {
		return "", domain.ErrNotFound
	}
	return p.CompanyID, nil
}

func (r *fakeScheduleRepo) GetScheduleEntry(_ context.Context, id string) (domain.ScheduleEntry, error) {
	e, ok := r.entries[id]
	if !ok {
		return domain.ScheduleEntry{}, domain.ErrNotFound
	}
	return e, nil
}

func (r *fakeScheduleRepo) GetScheduleEntryForUpdate(_ context.Context, _ pgx.Tx, id string) (domain.ScheduleEntry, error) {
	return r.GetScheduleEntry(context.Background(), id)
}

func (r *fakeScheduleRepo) FindLiveEntryForAgentDateTx(_ context.Context, _ pgx.Tx, employeeID string, date time.Time) (svc.LiveEntry, error) {
	return r.FindLiveEntryForAgentDate(context.Background(), employeeID, date)
}

func (r *fakeScheduleRepo) CreateScheduleEntry(_ context.Context, _ pgx.Tx, p svc.CreateScheduleEntryParams) (domain.ScheduleEntry, error) {
	r.seq++
	id := "SWP-SCH-" + itoa(90000+r.seq)
	cover := r.placements[p.EmployeeID]
	e := domain.ScheduleEntry{
		ID:              id,
		EmployeeID:      p.EmployeeID,
		PlacementID:     p.PlacementID,
		CompanyID:       cover.CompanyID,
		ShiftMasterID:   p.ShiftMasterID,
		StartTime:       p.StartTime,
		EndTime:         p.EndTime,
		CrossMidnight:   p.CrossMidnight,
		WorkDate:        p.WorkDate,
		Status:          p.Status,
		IsDayOff:        p.IsDayOff,
		ReplacedEntryID: p.ReplacedEntryID,
		CreatedBy:       p.CreatedBy,
		CreatedAt:       fixedNow,
		UpdatedAt:       fixedNow,
	}
	r.entries[id] = e
	// Register the new entry as the live entry for its (agent, date) so a
	// subsequent same-cell write triggers DOUBLE_SHIFT.
	r.liveEntry[leaveKey(p.EmployeeID, p.WorkDate)] = svc.LiveEntry{
		ID: id, ShiftMasterID: p.ShiftMasterID, Status: p.Status, IsDayOff: p.IsDayOff,
	}
	return e, nil
}

func (r *fakeScheduleRepo) UpdateScheduleEntry(_ context.Context, _ pgx.Tx, p svc.UpdateScheduleEntryParams) (domain.ScheduleEntry, error) {
	cur, ok := r.entries[p.ID]
	if !ok {
		return domain.ScheduleEntry{}, domain.ErrNotFound
	}
	cur.ShiftMasterID = p.ShiftMasterID
	cur.StartTime = p.StartTime
	cur.EndTime = p.EndTime
	cur.CrossMidnight = p.CrossMidnight
	cur.Status = p.Status
	cur.IsDayOff = p.IsDayOff
	cur.UpdatedAt = fixedNow
	r.entries[p.ID] = cur
	return cur, nil
}

func (r *fakeScheduleRepo) SoftDeleteScheduleEntry(_ context.Context, _ pgx.Tx, id string) (int64, error) {
	e, ok := r.entries[id]
	if !ok {
		return 0, nil
	}
	delete(r.entries, id)
	delete(r.liveEntry, leaveKey(e.EmployeeID, e.WorkDate))
	return 1, nil
}

var _ svc.ScheduleRepository = (*fakeScheduleRepo)(nil)

// ---------------------------------------------------------------------------
// harness — mounts the real services + handler over the fakes.
// ---------------------------------------------------------------------------

type harness struct {
	router    *chi.Mux
	masters   *fakeShiftMasterRepo
	schedule  *fakeScheduleRepo
	principal auth.Principal
}

// newHarness builds the scheduling slice. principalRole is the caller's role;
// leaderCompanyID is the shift_leader's scoped company (ignored for staff roles).
func newHarness(t *testing.T, principalRole auth.Role, leaderCompanyID string) *harness {
	t.Helper()
	masters := newFakeShiftMasterRepo()
	sched := newFakeScheduleRepo(masters)

	msvc := svc.NewShiftMasterService(masters, &fakeTxRunner{})
	msvc.SetClock(func() time.Time { return fixedNow })
	ssvc := svc.NewScheduleService(sched, &fakeTxRunner{})
	ssvc.SetClock(func() time.Time { return fixedNow })

	handler := schedulinghandler.NewHandler(msvc, ssvc)

	h := &harness{
		masters:  masters,
		schedule: sched,
		principal: auth.Principal{
			UserID:    "SWP-USR-0001",
			Role:      principalRole,
			CompanyID: leaderCompanyID,
		},
	}

	r := chi.NewRouter()
	r.Use(httpx.RequestIDMiddleware)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := auth.WithPrincipal(req.Context(), h.principal)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})

	// Reads + ALL schedule ops: super_admin, hr_admin, shift_leader (mirror server.go).
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader))
		r.Get("/shift-masters", handler.ListShiftMasters)
		r.Get("/shift-masters/{id}", handler.GetShiftMaster)
		r.Get("/schedule", handler.ListSchedule)
		r.Post("/schedule", handler.CreateScheduleEntry)
		r.Patch("/schedule/{id}", handler.UpdateScheduleEntry)
		r.Delete("/schedule/{id}", handler.DeleteScheduleEntry)
		r.Post("/schedule:check", handler.CheckScheduleConflicts)
		r.Post("/schedule:bulk-apply", handler.BulkApplySchedule)
	})
	// Agent self-schedule: adds RoleAgent (mirror server.go).
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin, auth.RoleShiftLeader, auth.RoleAgent))
		r.Get("/schedule/by-agent/{employee_id}", handler.GetScheduleByAgent)
	})
	// Shift-master writes: super_admin, hr_admin only.
	r.Group(func(r chi.Router) {
		r.Use(rbac.RequireRole(auth.RoleSuperAdmin, auth.RoleHRAdmin))
		r.Post("/shift-masters", handler.CreateShiftMaster)
		r.Patch("/shift-masters/{id}", handler.UpdateShiftMaster)
		r.Post("/shift-masters/{id}:deactivate", handler.DeactivateShiftMaster)
		r.Post("/shift-masters/{id}:reactivate", handler.ReactivateShiftMaster)
	})

	h.router = r
	return h
}

func (h *harness) do(method, path string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	return rr
}

// ---------------------------------------------------------------------------
// seed helpers
// ---------------------------------------------------------------------------

// seedMaster inserts a shift master directly (bypasses the create path) so tests
// can pin its id/active flags. The serviceLine arg is retained for call-site
// stability but ignored — shift masters are service-line independent (2026-06-12).
func (h *harness) seedMaster(id, name, start, end string, _ *string, active bool) domain.ShiftMaster {
	m := domain.ShiftMaster{
		ID:            id,
		Name:          name,
		StartTime:     start,
		EndTime:       end,
		CrossMidnight: end <= start,
		IsActive:      active,
		CreatedAt:     fixedNow,
		UpdatedAt:     fixedNow,
	}
	h.masters.masters[id] = m
	h.masters.nameIndex[name] = id
	return m
}

// seedPlacement registers the agent's active placement cover. The serviceLineID
// arg is retained for call-site stability but ignored (service_line removed).
func (h *harness) seedPlacement(empID, placementID, companyID, _ string, start time.Time, end *time.Time) {
	h.schedule.placements[empID] = svc.PlacementCover{
		PlacementID: placementID,
		CompanyID:   companyID,
		StartDate:   start,
		EndDate:     end,
	}
}

// seedApprovedLeave plants an approved-leave day so SHIFT_OVER_LEAVE fires.
func (h *harness) seedApprovedLeave(empID string, date time.Time, lrID, leaveType string) {
	h.schedule.approvedLeave[leaveKey(empID, date)] = svc.ApprovedLeave{
		LeaveRequestID: strp(lrID),
		LeaveType:      strp(leaveType),
	}
}

// seedLiveEntry plants an existing live entry so DOUBLE_SHIFT fires.
func (h *harness) seedLiveEntry(id, empID, companyID string, date time.Time, shiftName string) {
	h.schedule.liveEntry[leaveKey(empID, date)] = svc.LiveEntry{
		ID:        id,
		ShiftName: strp(shiftName),
		Status:    "SCHEDULED",
	}
	h.schedule.entries[id] = domain.ScheduleEntry{
		ID: id, EmployeeID: empID, CompanyID: companyID, WorkDate: date,
		Status: "SCHEDULED", ShiftMasterName: strp(shiftName),
		CreatedAt: fixedNow, UpdatedAt: fixedNow,
	}
}

// seedEntry plants a schedule_entry linked to a master (propagation candidate).
// start/end are the entry's current snapshot; cross is derived (end<=start).
func (h *harness) seedEntry(id, empID, masterID string, date time.Time, start, end string, isDayOff bool, status string) {
	st, et := start, end
	h.schedule.entries[id] = domain.ScheduleEntry{
		ID:            id,
		EmployeeID:    empID,
		ShiftMasterID: strp(masterID),
		StartTime:     &st,
		EndTime:       &et,
		CrossMidnight: end <= start,
		WorkDate:      date,
		Status:        status,
		IsDayOff:      isDayOff,
		CreatedAt:     fixedNow,
		UpdatedAt:     fixedNow,
	}
}

// seedAttendance links an attendance row to an entry. checkIn/checkOut are
// optional (nil = absent). shiftEndAt seeds the pre-sync value so a test can
// assert the E4→E5 push.
func (h *harness) seedAttendance(entryID string, checkIn, checkOut *time.Time, shiftEndAt *time.Time) {
	h.schedule.attendance[entryID] = fakeAttendance{
		checkIn: checkIn, checkOut: checkOut, shiftEndAt: shiftEndAt,
	}
}

// silence unused in case a helper is dropped during edits.
var _ = dateStr
