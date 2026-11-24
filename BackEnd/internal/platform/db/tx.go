package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX matches sqlc's pgx/v5 query execution contract so repositories can run
// identically on either a *pgxpool.Pool or an active pgx.Tx from context.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// TxRunner runs cross-module commands inside a single READ COMMITTED PostgreSQL transaction.
type TxRunner interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type txContextKey struct{}

// WithTx attaches a pgx.Tx to ctx.
func WithTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}

// TxFromContext extracts the active pgx.Tx from ctx, if any.
func TxFromContext(ctx context.Context) (pgx.Tx, bool) {
	if ctx == nil {
		return nil, false
	}
	tx, ok := ctx.Value(txContextKey{}).(pgx.Tx)
	return tx, ok && tx != nil
}

// InTx reports whether ctx currently executes inside a transaction started by TxRunner.
func InTx(ctx context.Context) bool {
	_, ok := TxFromContext(ctx)
	return ok
}

// Querier returns the active transaction from ctx when present, otherwise fallback (e.g. *pgxpool.Pool).
func Querier(ctx context.Context, fallback DBTX) DBTX {
	if tx, ok := TxFromContext(ctx); ok {
		return tx
	}
	return fallback
}

// PgxTxRunner implements TxRunner on top of *pgxpool.Pool.
type PgxTxRunner struct {
	pool *pgxpool.Pool
}

// NewTxRunner constructs a PgxTxRunner.
func NewTxRunner(pool *pgxpool.Pool) *PgxTxRunner {
	return &PgxTxRunner{pool: pool}
}

// WithinTx executes fn inside a READ COMMITTED PostgreSQL transaction.
// If ctx is already inside a transaction, fn reuses that transaction so nested
// cross-module calls remain atomic without opening disconnected transactions.
func (r *PgxTxRunner) WithinTx(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	if fn == nil {
		return nil
	}
	if _, ok := TxFromContext(ctx); ok {
		return fn(ctx)
	}
	if r == nil || r.pool == nil {
		return errors.New("tx runner has nil pool")
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted,
	})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	committed := false
	defer func() {
		if rec := recover(); rec != nil {
			_ = tx.Rollback(context.Background())
			panic(rec)
		}
		if !committed {
			_ = tx.Rollback(context.Background())
		}
	}()

	txCtx := WithTx(ctx, tx)
	if err = fn(txCtx); err != nil {
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	committed = true
	return nil
}
