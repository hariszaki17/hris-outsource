// Command api is the HTTP API server. It wires config -> logging/otel -> db ->
// platform services -> the identity slice -> the chi router, then serves with
// graceful shutdown. New epics are wired in the same place (build repo+service,
// pass the handler into server.Deps).
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	attendancehttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/attendance"
	foundationshttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/foundations"
	identityhttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/identity"
	leavehttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/leave"
	orghttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/org"
	overtimehttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/overtime"
	payrollhttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/payroll"
	peoplehttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/people"
	placementhttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/placement"
	reportinghttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/reporting"
	schedulinghttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/scheduling"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/config"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/cron"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/crypto"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/idempotency"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/jobs"
	applog "github.com/hariszaki17/hris-outsource/backend/internal/platform/log"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/obs"
	attendancerepo "github.com/hariszaki17/hris-outsource/backend/internal/repository/attendance"
	foundationsrepo "github.com/hariszaki17/hris-outsource/backend/internal/repository/foundations"
	identityrepo "github.com/hariszaki17/hris-outsource/backend/internal/repository/identity"
	leaverepo "github.com/hariszaki17/hris-outsource/backend/internal/repository/leave"
	orgrepo "github.com/hariszaki17/hris-outsource/backend/internal/repository/org"
	overtimerepo "github.com/hariszaki17/hris-outsource/backend/internal/repository/overtime"
	payrollrepo "github.com/hariszaki17/hris-outsource/backend/internal/repository/payroll"
	peoplerepo "github.com/hariszaki17/hris-outsource/backend/internal/repository/people"
	placementrepo "github.com/hariszaki17/hris-outsource/backend/internal/repository/placement"
	reportingrepo "github.com/hariszaki17/hris-outsource/backend/internal/repository/reporting"
	schedulingrepo "github.com/hariszaki17/hris-outsource/backend/internal/repository/scheduling"
	"github.com/hariszaki17/hris-outsource/backend/internal/server"
	attendancesvc "github.com/hariszaki17/hris-outsource/backend/internal/service/attendance"
	foundationssvc "github.com/hariszaki17/hris-outsource/backend/internal/service/foundations"
	identitysvc "github.com/hariszaki17/hris-outsource/backend/internal/service/identity"
	leavesvc "github.com/hariszaki17/hris-outsource/backend/internal/service/leave"
	orgsvc "github.com/hariszaki17/hris-outsource/backend/internal/service/org"
	overtimesvc "github.com/hariszaki17/hris-outsource/backend/internal/service/overtime"
	payrollsvc "github.com/hariszaki17/hris-outsource/backend/internal/service/payroll"
	peoplesvc "github.com/hariszaki17/hris-outsource/backend/internal/service/people"
	placementsvc "github.com/hariszaki17/hris-outsource/backend/internal/service/placement"
	reportingsvc "github.com/hariszaki17/hris-outsource/backend/internal/service/reporting"
	schedulingsvc "github.com/hariszaki17/hris-outsource/backend/internal/service/scheduling"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	applog.Setup(cfg.Env, cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Observability (traces optional, metrics always on).
	observ, err := obs.Setup(ctx, cfg.ServiceName, cfg.OTel.OTLPEndpoint)
	if err != nil {
		return err
	}
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = observ.Shutdown(shutCtx)
	}()

	// Database.
	pool, err := db.Open(ctx, cfg.DB.URL, cfg.DB.MaxConns)
	if err != nil {
		return err
	}
	defer pool.Close()
	txm := db.NewTxManager(pool)

	// Payroll encryption key (E8 / INV-2): the AES-256-GCM cipher that decrypts the
	// *_enc money columns at the read boundary. Fail fast on a set-but-invalid key;
	// warn (not fatal) when empty in dev — the E2E harness always sets it.
	var payrollCipher *crypto.Cipher
	if cfg.Crypto.PayrollKey != "" {
		payrollCipher, err = crypto.NewFromBase64(cfg.Crypto.PayrollKey)
		if err != nil {
			return err
		}
	} else {
		slog.Warn("PAYROLL_ENCRYPTION_KEY unset — payroll endpoints will fail to decrypt money")
	}

	// River insert-only client (the API process inserts jobs; cmd/worker runs them).
	// This is the FIRST wiring of jobs.Client into the API process — the async
	// payslip export EnqueueTx's its job through this client (transactional outbox).
	jobsClient, err := jobs.NewInsertOnlyClient(pool)
	if err != nil {
		return err
	}

	// Auth primitives.
	issuer, err := auth.NewIssuer(cfg.Auth.JWTPrivateKey, cfg.Auth.JWTPublicKey, cfg.Auth.AccessTTL)
	if err != nil {
		return err
	}
	authn := auth.NewAuthenticator(issuer)

	// Identity slice: repository -> service -> handler.
	idRepo := identityrepo.New(pool)

	// F2.7: wire the per-request session-epoch + status check into the authenticator
	// (instant revocation on offboard/disable). One indexed PK read per request.
	authn.WithUserState(func(ctx context.Context, userID string) (string, time.Time, error) {
		u, err := idRepo.GetUserByID(ctx, userID)
		if err != nil {
			return "", time.Time{}, err
		}
		return u.Status, u.TokensValidAfter, nil
	})
	idSvc := identitysvc.NewService(idRepo, txm, issuer, cfg.Auth.RefreshTTL)
	idHandler := identityhttp.NewHandler(idSvc, identityhttp.CookieConfig{
		Domain: cfg.Auth.CookieDomain,
		Secure: cfg.Auth.CookieSecure,
	}, cfg.Auth.AccessTTL)

	// Foundations slice (E1 user management, audit-log, platform settings).
	fndRepo := foundationsrepo.New(pool)
	fndSvc := foundationssvc.NewService(fndRepo, txm)
	fndHandler := foundationshttp.NewHandler(fndSvc)

	// Org slice (03-02): client companies + sites (E2 F2.3 + F2.6).
	orgCompaniesRepo := orgrepo.New(pool)
	orgCompaniesSvc := orgsvc.NewService(orgCompaniesRepo, txm)
	orgCompaniesHandler := orghttp.NewHandler(orgCompaniesSvc)

	// Org slice (03-03): service lines + positions (E2 F2.4).
	orgServiceLinesRepo := orgrepo.NewServiceLineRepo(pool)
	orgServiceLinesSvc := orgsvc.NewServiceLineService(orgServiceLinesRepo, txm)
	orgServiceLinesHandler := orghttp.NewServiceLineHandler(orgServiceLinesSvc)

	// Org slice (03-04): operational master data — leave types, attendance codes, overtime rules.
	orgMasterDataRepo := orgrepo.NewMasterDataRepo(pool)
	orgMasterDataSvc := orgsvc.NewMasterDataService(orgMasterDataRepo, txm)
	orgMasterDataHandler := orghttp.NewMasterDataHandler(orgMasterDataSvc)

	// People slice (04-02): employees (E2 F2.1 / PPL-01).
	// 04-03 (agreements) and 04-04 (change-requests) append their own wiring
	// after this block — see 04-02-SUMMARY.md for the coordination contract.
	peopleRepo := peoplerepo.New(pool)
	peopleSvc := peoplesvc.NewService(peopleRepo, txm)
	peopleHandler := peoplehttp.NewHandler(peopleSvc)

	// People agreements slice (04-03): employment agreements + attachments + file download (PPL-02).
	agreementsRepo := peoplerepo.NewAgreementRepo(pool)
	agreementsSvc := peoplesvc.NewAgreementService(agreementsRepo, txm)
	agreementsHandler := peoplehttp.NewAgreementHandler(agreementsSvc)

	// People change-requests slice (04-04): HR approval queue for agent-submitted
	// profile-change requests (E2 F2.1 EP-5 / PPL-03).
	crRepo := peoplerepo.NewChangeRequestRepo(pool)
	crSvc := peoplesvc.NewChangeRequestService(crRepo, txm)
	crHandler := peoplehttp.NewChangeRequestHandler(crSvc)

	// Placement slice (05-02): E3 placement CRUD + lifecycle + shift-leader + roster.
	// The placement and shift-leader services are mutually referential (the
	// placement service auto-vacates leadership on resolution; both join the
	// current leader), so wire the leader service into the placement service.
	placementRepo := placementrepo.NewPlacementRepo(pool)
	leaderRepo := placementrepo.NewShiftLeaderRepo(pool)

	// GAP 3: derive shift_leader company scope at request time from the live E3
	// leader-assignment (instead of the baked-in JWT `cmp` claim), so reassigning a
	// leader takes effect on their next request. domain.ErrNotFound (leads no company)
	// flows through as an error → the middleware strips scope (fail-safe deny).
	authn.WithCompanyResolver(func(ctx context.Context, employeeID string) (string, error) {
		return leaderRepo.GetActiveLeaderCompanyForEmployee(ctx, employeeID)
	})

	placementSvc := placementsvc.NewPlacementService(placementRepo, txm)
	leaderSvc := placementsvc.NewShiftLeaderService(leaderRepo, txm)
	placementSvc.SetLeaderService(leaderSvc)
	placementHandler := placementhttp.NewHandler(placementSvc, leaderSvc)

	// Scheduling slice (06-02): E4 shift masters + schedule grid + conflict engine.
	shiftMasterRepo := schedulingrepo.NewShiftMasterRepo(pool)
	scheduleRepo := schedulingrepo.NewScheduleRepo(pool)
	shiftMasterSvc := schedulingsvc.NewShiftMasterService(shiftMasterRepo, txm)
	scheduleSvc := schedulingsvc.NewScheduleService(scheduleRepo, txm)
	schedulingHandler := schedulinghttp.NewHandler(shiftMasterSvc, scheduleSvc)

	// Attendance slice (07-02): E5 verify/reject (+bulk) + corrections (F5.3/F5.4).
	// The correction service needs the attendance repo to apply approved
	// corrections to the target record in the same tx.
	attendanceRepo := attendancerepo.NewAttendanceRepo(pool)
	correctionRepo := attendancerepo.NewCorrectionRepo(pool)
	attendanceSvc := attendancesvc.NewAttendanceService(attendanceRepo, txm)
	attendanceSvc.SetNotifier(jobsClient) // E10 (11-02): real notify on verify/reject
	correctionSvc := attendancesvc.NewCorrectionService(correctionRepo, attendanceRepo, txm)
	attendanceHandler := attendancehttp.NewHandler(attendanceSvc, correctionSvc)

	// Absence-sweep cron (07-xx / F5.2): in-process, single-binary job that writes
	// ABSENT rows for scheduled shifts that ended past the grace with no clock-in.
	// The partial unique index on attendance(schedule_id) is the idempotency guard.
	absenceSweepRepo := attendancerepo.NewAbsenceSweepRepo(pool)
	absenceSweepSvc := attendancesvc.NewAbsenceSweepService(absenceSweepRepo, txm, cfg.Cron.AbsenceGrace, 0)

	// Leave slice (08-02): E6 two-level approval + quotas + calendar (F6.1/F6.2/F6.3).
	// The leave service's INV-3 loop-closer reuses the EXISTING scheduling repo
	// (scheduleRepo above) as its SchedulePort — cancelling overlapping schedule
	// entries + populating approved_leave_days in the approval tx.
	leaveRepo := leaverepo.NewLeaveRepo(pool)
	quotaRepo := leaverepo.NewQuotaRepo(pool)
	grantRepo := leaverepo.NewGrantRepo(pool)
	grantSvc := leavesvc.NewGrantService(grantRepo, txm) // F6.1 grant-lot ledger + FIFO allocator
	leaveSvc := leavesvc.NewLeaveService(leaveRepo, grantSvc, scheduleRepo, txm)
	leaveSvc.SetNotifier(jobsClient) // E10 (11-02): real notify on approve-final/reject
	quotaSvc := leavesvc.NewQuotaService(quotaRepo, txm) // DEPRECATED 2026-06-08 — kept for /leave-quotas*
	calendarSvc := leavesvc.NewCalendarService(leaveRepo)
	leaveHandler := leavehttp.NewHandler(leaveSvc, quotaSvc, grantSvc, calendarSvc)

	// Leave-expiry sweep (F6.1): in-process cron that releases dangling pending on
	// lapsed grant-lots (remaining is already 0 for an inactive lot at the read
	// boundary). Mirrors the absence-sweep wiring.
	leaveExpirySvc := leavesvc.NewLeaveExpirySweepService(grantRepo, txm, 0)

	// Overtime slice (09-02): E7 two-level OT approval + holiday calendar
	// (F7.1/F7.3/F7.4). The OT service reuses the EXISTING scheduling repo
	// (scheduleRepo above) as its SchedulePort for WORKDAY/RESTDAY day_type
	// classification, and the overtime repo's FindOvertimeRule reuses the E2
	// overtime_rules master for OT_BELOW_MIN + the reference multiplier (INV-2).
	overtimeRepo := overtimerepo.NewOvertimeRepo(pool)
	holidayRepo := overtimerepo.NewHolidayRepo(pool)
	overtimeSvc := overtimesvc.NewOvertimeService(overtimeRepo, overtimeRepo, holidayRepo, scheduleRepo, txm)
	overtimeSvc.SetNotifier(jobsClient) // E10 (11-02): real notify on approve-final/reject
	holidaySvc := overtimesvc.NewHolidayService(holidayRepo, txm)
	overtimeHandler := overtimehttp.NewHandler(overtimeSvc, holidaySvc)

	// Payroll slice (10-02): E8 historical, read-only payslip archive + audit notes
	// + async Excel export. The payslip service decrypts the *_enc money at the
	// read boundary (payrollCipher); the export service enqueues a real River
	// PayslipExportWorker in the same tx as the export_jobs QUEUED insert
	// (jobsClient). Both reads and the audit-note notify stub share txm.
	payrollRepo := payrollrepo.New(pool)
	payrollExportRepo := payrollrepo.NewExportRepo(pool)
	payslipSvc := payrollsvc.NewPayslipService(payrollRepo, txm, payrollCipher, jobsClient)
	payrollExportSvc := payrollsvc.NewExportService(payrollExportRepo, txm, jobsClient)
	payrollHandler := payrollhttp.NewHandler(payslipSvc, payrollExportSvc)

	// Reporting slice (11-02): E10 notifications (list/mark-read/mark-all-read).
	// scope=self — the service derives the recipient set from the principal. The
	// auto-dispatched notifications (leave/OT/attendance retro-wire) land here via
	// the un-stubbed NotificationWorker (cmd/worker). 11-02b extends this handler
	// with dashboard/billable-report/export methods.
	reportingNotifRepo := reportingrepo.New(pool)
	reportingNotifSvc := reportingsvc.NewNotificationService(reportingNotifRepo)
	// 11-02b: dashboard (role-aware aggregation), billable report (verified-only),
	// and the GENERIC export framework (insert QUEUED + EnqueueTx the
	// ReportExportWorker in one tx; the worker — registered in NewWorkerClient
	// alongside the Phase-10 PayslipExportWorker — flips the job DONE).
	reportingDashboardRepo := reportingrepo.NewDashboardRepo(pool)
	reportingBillableRepo := reportingrepo.NewBillableRepo(pool)
	reportingExportRepo := reportingrepo.NewExportRepo(pool)
	reportingDashboardSvc := reportingsvc.NewDashboardService(reportingDashboardRepo)
	reportingBillableSvc := reportingsvc.NewBillableService(reportingBillableRepo)
	reportingExportSvc := reportingsvc.NewExportService(reportingExportRepo, reportingBillableRepo, txm, jobsClient)
	reportingHandler := reportinghttp.NewHandler(reportingNotifSvc, reportingDashboardSvc, reportingBillableSvc, reportingExportSvc)

	handler := server.New(server.Deps{
		AllowedOrigins:       cfg.HTTP.AllowedOrigins,
		RatePerMinute:        cfg.Rate.PerMinute,
		RateBurst:            cfg.Rate.Burst,
		Auth:                 idHandler,
		Foundations:          fndHandler,
		OrgCompanies:         orgCompaniesHandler,
		OrgServiceLines:      orgServiceLinesHandler,
		OrgMasterData:        orgMasterDataHandler,
		People:               peopleHandler,
		PeopleAgreements:     agreementsHandler,
		PeopleChangeRequests: crHandler,
		Placement:            placementHandler,
		Scheduling:           schedulingHandler,
		Attendance:           attendanceHandler,
		Leave:                leaveHandler,
		Overtime:             overtimeHandler,
		Payroll:              payrollHandler,
		Reporting:            reportingHandler,
		Authn:                authn,
		Idempotency:          idempotency.New(pool),
		Obs:                  observ,
	})

	srv := &http.Server{
		Addr:         cfg.HTTP.Addr,
		Handler:      handler,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
	}

	// In-process cron (single binary): start the absence-sweep runner before serving.
	// It stops on ctx.Done() (the same signal that drives graceful shutdown).
	if cfg.Cron.AbsenceSweepEnabled {
		runner := cron.NewRunner("absence-sweep", cfg.Cron.AbsenceSweepInterval, func(ctx context.Context) error {
			_, err := absenceSweepSvc.Sweep(ctx)
			return err
		})
		go runner.Start(ctx)
		slog.Info("absence-sweep cron started",
			"interval", cfg.Cron.AbsenceSweepInterval.String(), "grace", cfg.Cron.AbsenceGrace.String())
	}

	// Leave-expiry sweep cron (F6.1): releases dangling pending on lapsed grant-lots.
	if cfg.Cron.LeaveExpirySweepEnabled {
		runner := cron.NewRunner("leave-expiry-sweep", cfg.Cron.LeaveExpiryInterval, func(ctx context.Context) error {
			_, err := leaveExpirySvc.Sweep(ctx)
			return err
		})
		go runner.Start(ctx)
		slog.Info("leave-expiry-sweep cron started", "interval", cfg.Cron.LeaveExpiryInterval.String())
	}

	// Serve until signal, then graceful shutdown.
	errCh := make(chan error, 1)
	go func() {
		slog.Info("api listening", "addr", cfg.HTTP.Addr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	}
}
