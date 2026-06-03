// Package db owns the Postgres connection pool (pgx) and the transaction
// manager. sqlc-generated repositories bind to either the pool (reads) or a
// pgx.Tx (writes); the TxManager is how a service runs a write + its audit row
// + its River job enqueue in one atomic transaction.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool is the shared connection pool. It satisfies sqlc's DBTX, so
// sqlcgen.New(pool) works directly for non-transactional queries.
type Pool struct {
	*pgxpool.Pool
}

// Open creates and verifies a pgx pool.
func Open(ctx context.Context, url string, maxConns int32) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse db url: %w", err)
	}
	if maxConns > 0 {
		cfg.MaxConns = maxConns
	}
	cfg.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect db: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &Pool{Pool: pool}, nil
}
