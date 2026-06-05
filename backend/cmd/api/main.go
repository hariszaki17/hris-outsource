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
	peoplehttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/people"
	placementhttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/placement"
	schedulinghttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/scheduling"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/config"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/idempotency"
	applog "github.com/hariszaki17/hris-outsource/backend/internal/platform/log"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/obs"
	attendancerepo "github.com/hariszaki17/hris-outsource/backend/internal/repository/attendance"
	foundationsrepo "github.com/hariszaki17/hris-outsource/backend/internal/repository/foundations"
	identityrepo "github.com/hariszaki17/hris-outsource/backend/internal/repository/identity"
	leaverepo "github.com/hariszaki17/hris-outsource/backend/internal/repository/leave"
	orgrepo "github.com/hariszaki17/hris-outsource/backend/internal/repository/org"
	peoplerepo "github.com/hariszaki17/hris-outsource/backend/internal/repository/people"
	placementrepo "github.com/hariszaki17/hris-outsource/backend/internal/repository/placement"
	schedulingrepo "github.com/hariszaki17/hris-outsource/backend/internal/repository/scheduling"
	"github.com/hariszaki17/hris-outsource/backend/internal/server"
	attendancesvc "github.com/hariszaki17/hris-outsource/backend/internal/service/attendance"
	foundationssvc "github.com/hariszaki17/hris-outsource/backend/internal/service/foundations"
	identitysvc "github.com/hariszaki17/hris-outsource/backend/internal/service/identity"
	leavesvc "github.com/hariszaki17/hris-outsource/backend/internal/service/leave"
	orgsvc "github.com/hariszaki17/hris-outsource/backend/internal/service/org"
	peoplesvc "github.com/hariszaki17/hris-outsource/backend/internal/service/people"
	placementsvc "github.com/hariszaki17/hris-outsource/backend/internal/service/placement"
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

	// Auth primitives.
	issuer, err := auth.NewIssuer(cfg.Auth.JWTPrivateKey, cfg.Auth.JWTPublicKey, cfg.Auth.AccessTTL)
	if err != nil {
		return err
	}
	authn := auth.NewAuthenticator(issuer)

	// Identity slice: repository -> service -> handler.
	idRepo := identityrepo.New(pool)
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
	correctionSvc := attendancesvc.NewCorrectionService(correctionRepo, attendanceRepo, txm)
	attendanceHandler := attendancehttp.NewHandler(attendanceSvc, correctionSvc)

	// Leave slice (08-02): E6 two-level approval + quotas + calendar (F6.1/F6.2/F6.3).
	// The leave service's INV-3 loop-closer reuses the EXISTING scheduling repo
	// (scheduleRepo above) as its SchedulePort — cancelling overlapping schedule
	// entries + populating approved_leave_days in the approval tx.
	leaveRepo := leaverepo.NewLeaveRepo(pool)
	quotaRepo := leaverepo.NewQuotaRepo(pool)
	leaveSvc := leavesvc.NewLeaveService(leaveRepo, quotaRepo, scheduleRepo, txm)
	quotaSvc := leavesvc.NewQuotaService(quotaRepo, txm)
	calendarSvc := leavesvc.NewCalendarService(leaveRepo)
	leaveHandler := leavehttp.NewHandler(leaveSvc, quotaSvc, calendarSvc)

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
