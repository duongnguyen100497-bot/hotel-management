package db_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/db"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/testutil"
)

func TestMapPgError(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantCode string
	}{
		{"no_rows", pgx.ErrNoRows, apperror.CodeNotFound},
		{"unique_23505", &pgconn.PgError{Code: db.CodeUniqueViolation}, apperror.CodeConflict},
		{"exclusion_23P01", &pgconn.PgError{Code: db.CodeExclusionViolation}, apperror.CodeConflict},
		{"serialization_40001", &pgconn.PgError{Code: db.CodeSerializationFail}, apperror.CodeConflict},
		{"deadlock_40P01", &pgconn.PgError{Code: db.CodeDeadlockDetected}, apperror.CodeConflict},
		{"fk_23503", &pgconn.PgError{Code: db.CodeForeignKeyViolation}, apperror.CodeValidationFailed},
		{"check_23514", &pgconn.PgError{Code: db.CodeCheckViolation}, apperror.CodeValidationFailed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mapped := db.MapPgError(tc.err)
			appErr, ok := apperror.As(mapped)
			if !ok {
				t.Fatalf("expected *apperror.Error, got %T (%v)", mapped, mapped)
			}
			if appErr.Code != tc.wantCode {
				t.Errorf("code = %q, want %q", appErr.Code, tc.wantCode)
			}
		})
	}

	if !db.IsRetryable(&pgconn.PgError{Code: db.CodeSerializationFail}) {
		t.Error("40001 should be retryable")
	}
	if !db.IsRetryable(&pgconn.PgError{Code: db.CodeDeadlockDetected}) {
		t.Error("40P01 should be retryable")
	}
	if !db.IsExclusionViolation(&pgconn.PgError{Code: db.CodeExclusionViolation}) {
		t.Error("23P01 should match IsExclusionViolation")
	}
}

func TestPostgresIntegration_ExtensionsAndTxRunner(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Testcontainers integration test in short mode")
	}

	env := testutil.StartPostgres(t)
	ctx := context.Background()

	// 1. Verify Migration 001 enabled pgcrypto, citext, and btree_gist.
	for _, ext := range []string{"pgcrypto", "citext", "btree_gist"} {
		var installed bool
		err := env.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = $1)`, ext).Scan(&installed)
		if err != nil {
			t.Fatalf("query extension %s: %v", ext, err)
		}
		if !installed {
			t.Fatalf("expected extension %s to be installed", ext)
		}
	}

	// Create a scratch table for transaction commit/rollback tests.
	_, err := env.Pool.Exec(ctx, `CREATE TABLE tx_probe (id INT PRIMARY KEY, note TEXT NOT NULL)`)
	if err != nil {
		t.Fatalf("create scratch table: %v", err)
	}

	// 2. Commit succeeds when fn returns nil (including nested WithinTx reuse).
	err = env.TxRunner.WithinTx(ctx, func(txCtx context.Context) error {
		if !db.InTx(txCtx) {
			t.Fatal("expected InTx to be true inside WithinTx")
		}
		if _, execErr := db.Querier(txCtx, env.Pool).Exec(txCtx, `INSERT INTO tx_probe (id, note) VALUES (1, 'outer')`); execErr != nil {
			return execErr
		}
		return env.TxRunner.WithinTx(txCtx, func(nestedCtx context.Context) error {
			_, nestedErr := db.Querier(nestedCtx, env.Pool).Exec(nestedCtx, `INSERT INTO tx_probe (id, note) VALUES (2, 'nested')`)
			return nestedErr
		})
	})
	if err != nil {
		t.Fatalf("WithinTx commit failed: %v", err)
	}

	var count int
	if err := env.Pool.QueryRow(ctx, `SELECT count(*) FROM tx_probe`).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 2 {
		t.Fatalf("count after commit = %d, want 2", count)
	}

	// 3. Rollback cleanly when fn returns an error (outer + nested both rolled back).
	boomErr := errors.New("simulated failure")
	err = env.TxRunner.WithinTx(ctx, func(txCtx context.Context) error {
		if _, execErr := db.Querier(txCtx, env.Pool).Exec(txCtx, `INSERT INTO tx_probe (id, note) VALUES (3, 'should-rollback')`); execErr != nil {
			return execErr
		}
		_ = env.TxRunner.WithinTx(txCtx, func(nestedCtx context.Context) error {
			_, nestedErr := db.Querier(nestedCtx, env.Pool).Exec(nestedCtx, `INSERT INTO tx_probe (id, note) VALUES (4, 'nested-rollback')`)
			return nestedErr
		})
		return boomErr
	})
	if !errors.Is(err, boomErr) {
		t.Fatalf("expected boomErr, got %v", err)
	}
	if err := env.Pool.QueryRow(ctx, `SELECT count(*) FROM tx_probe`).Scan(&count); err != nil {
		t.Fatalf("count rows after error rollback: %v", err)
	}
	if count != 2 {
		t.Fatalf("count after error rollback = %d, want 2", count)
	}

	// 4. Rollback cleanly when fn panics (and propagate the panic).
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		_ = env.TxRunner.WithinTx(ctx, func(txCtx context.Context) error {
			if _, execErr := db.Querier(txCtx, env.Pool).Exec(txCtx, `INSERT INTO tx_probe (id, note) VALUES (5, 'panic-rollback')`); execErr != nil {
				return execErr
			}
			panic("unexpected boom")
		})
	}()
	if !panicked {
		t.Fatal("expected WithinTx to re-panic")
	}
	if err := env.Pool.QueryRow(ctx, `SELECT count(*) FROM tx_probe`).Scan(&count); err != nil {
		t.Fatalf("count rows after panic rollback: %v", err)
	}
	if count != 2 {
		t.Fatalf("count after panic rollback = %d, want 2", count)
	}
}
