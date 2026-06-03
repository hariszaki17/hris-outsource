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

	identityhttp "github.com/hariszaki17/hris-outsource/backend/internal/handler/identity"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/config"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/idempotency"
	applog "github.com/hariszaki17/hris-outsource/backend/internal/platform/log"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/obs"
	identityrepo "github.com/hariszaki17/hris-outsource/backend/internal/repository/identity"
	"github.com/hariszaki17/hris-outsource/backend/internal/server"
	identitysvc "github.com/hariszaki17/hris-outsource/backend/internal/service/identity"
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

	handler := server.New(server.Deps{
		AllowedOrigins: cfg.HTTP.AllowedOrigins,
		RatePerMinute:  cfg.Rate.PerMinute,
		RateBurst:      cfg.Rate.Burst,
		Auth:           idHandler,
		Authn:          authn,
		Idempotency:    idempotency.New(pool),
		Obs:            observ,
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
