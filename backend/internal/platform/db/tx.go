package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// TxManager runs a function inside a single Postgres transaction. This is the
// backbone of the system's atomic-write guarantee (ENGINEERING / CONVENTIONS
// §16.1): the data write, the audit-log row, and the River job enqueue all
// commit together or not at all.
type TxManager struct {
	pool *Pool
}

func NewTxManager(pool *Pool) *TxManager { return &TxManager{pool: pool} }

// InTx begins a transaction, invokes fn with it, and commits — rolling back on
// any error or panic. fn must use the provided tx for all writes (bind
// repositories with repo.WithTx(tx) and enqueue River jobs with the tx).
func (m *TxManager) InTx(ctx context.Context, fn func(tx pgx.Tx) error) (err error) {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p) // re-raise after cleanup
		}
		if err != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
				err = errors.Join(err, fmt.Errorf("rollback: %w", rbErr))
			}
		}
	}()

	if err = fn(tx); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
