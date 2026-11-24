package db

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
)

// Common PostgreSQL SQLSTATE error codes handled across modules.
const (
	CodeUniqueViolation     = "23505"
	CodeExclusionViolation  = "23P01"
	CodeForeignKeyViolation = "23503"
	CodeCheckViolation      = "23514"
	CodeSerializationFail   = "40001"
	CodeDeadlockDetected    = "40P01"
)

// PgError extracts a *pgconn.PgError from err if present.
func PgError(err error) (*pgconn.PgError, bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr, true
	}
	return nil, false
}

// IsUniqueViolation reports whether err is PostgreSQL SQLSTATE 23505.
func IsUniqueViolation(err error) bool {
	pgErr, ok := PgError(err)
	return ok && pgErr.Code == CodeUniqueViolation
}

// IsExclusionViolation reports whether err is PostgreSQL SQLSTATE 23P01 (e.g. GiST booking overlap).
func IsExclusionViolation(err error) bool {
	pgErr, ok := PgError(err)
	return ok && pgErr.Code == CodeExclusionViolation
}

// IsSerializationFailure reports whether err is PostgreSQL SQLSTATE 40001.
func IsSerializationFailure(err error) bool {
	pgErr, ok := PgError(err)
	return ok && pgErr.Code == CodeSerializationFail
}

// IsDeadlock reports whether err is PostgreSQL SQLSTATE 40P01.
func IsDeadlock(err error) bool {
	pgErr, ok := PgError(err)
	return ok && pgErr.Code == CodeDeadlockDetected
}

// IsRetryable reports whether err is a transient serialization failure or deadlock.
func IsRetryable(err error) bool {
	return IsSerializationFailure(err) || IsDeadlock(err)
}

// MapPgError converts common PostgreSQL/pgx errors into canonical apperror values.
func MapPgError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return apperror.NotFound("resource not found").WithCause(err)
	}
	pgErr, ok := PgError(err)
	if !ok {
		return err
	}
	switch pgErr.Code {
	case CodeUniqueViolation:
		return apperror.Conflict("resource already exists").WithCause(err)
	case CodeExclusionViolation:
		return apperror.Conflict("resource schedule conflict").WithCause(err)
	case CodeSerializationFail, CodeDeadlockDetected:
		return apperror.Conflict("concurrent transaction conflict, please retry").WithCause(err)
	case CodeForeignKeyViolation:
		return apperror.Validation("referenced resource does not exist").WithCause(err)
	case CodeCheckViolation:
		return apperror.Validation("value violates domain constraint").WithCause(err)
	default:
		return apperror.Internal(err)
	}
}
