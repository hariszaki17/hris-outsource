// Command seed inserts deterministic test/dev personas into the database and
// optionally prints a fresh Ed25519 keypair for use by the E2E harness.
//
// Usage:
//
//	seed [-genkeys] [-demo]
//
// Without flags: reads DATABASE_URL from the environment (or .env), connects,
// and calls Seed to upsert the four demo personas + the E2E fixtures. This is
// the path the Playwright E2E harness (frontend/e2e/lib/backend.ts) invokes; its
// behavior MUST stay unchanged.
//
// With -genkeys: prints two base64 (std) lines to stdout — line 1 is the
// Ed25519 private key (64 bytes), line 2 is the public key (32 bytes) — then
// exits 0. No database connection is opened. The harness (lib/backend.ts)
// reads these two lines as AUTH_JWT_PRIVATE_KEY / AUTH_JWT_PUBLIC_KEY.
//
// With -demo: runs the DEFAULT Seed first (so personas/auth/master-data exist),
// THEN layers a production-like demo dataset on top via SeedDemo (~8 companies,
// ~25 sites, ~120 agents/placements, ~30 days of schedules, attendance, leave,
// overtime, payslips, notifications). Additive + idempotent; uses disjoint
// high-band SWP ids so it never collides with the E2E fixtures. This flag is
// NEVER set by the E2E harness.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/hariszaki17/hris-outsource/backend/internal/platform/auth"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/config"
	"github.com/hariszaki17/hris-outsource/backend/internal/platform/db"
)

func main() {
	if err := run(); err != nil {
		slog.Error("seed failed", "err", err)
		os.Exit(1)
	}
}

func run() error {
	genkeys := flag.Bool("genkeys", false, "print a fresh Ed25519 keypair (base64 std) and exit")
	demo := flag.Bool("demo", false, "after the default seed, layer a production-like demo dataset (additive, idempotent)")
	flag.Parse()

	if *genkeys {
		privB64, pubB64, err := auth.GenerateKeypair()
		if err != nil {
			return fmt.Errorf("generate keypair: %w", err)
		}
		// Contract: exactly two lines; no labels. Harness reads line 1 as
		// AUTH_JWT_PRIVATE_KEY, line 2 as AUTH_JWT_PUBLIC_KEY.
		fmt.Println(privB64)
		fmt.Println(pubB64)
		os.Exit(0)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx := context.Background()
	pool, err := db.Open(ctx, cfg.DB.URL, cfg.DB.MaxConns)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer pool.Close()

	if err := Seed(ctx, pool); err != nil {
		return fmt.Errorf("seed: %w", err)
	}

	// -demo: layer the production-like dataset ON TOP of the default seed. The
	// default Seed above already created personas/auth/master-data that SeedDemo
	// reuses (global service lines, leave types, attendance codes, the default
	// overtime rule). This branch is unreachable on the no-flag E2E path.
	if *demo {
		if err := SeedDemo(ctx, pool); err != nil {
			return fmt.Errorf("seed demo: %w", err)
		}
		slog.Info("demo seed complete")
	}

	slog.Info("seed complete")
	return nil
}
