package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/pricing/domain"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/pricing/persistence/postgres/sqlcgen"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/db"
)

// Repository implements PostgreSQL persistence for VAT rates.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a pricing PostgreSQL repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) queries(ctx context.Context) *sqlcgen.Queries {
	return sqlcgen.New(db.Querier(ctx, r.pool))
}

func (r *Repository) ListVATRates(ctx context.Context) ([]domain.VATRate, error) {
	rows, err := r.queries(ctx).ListVATRates(ctx)
	if err != nil {
		return nil, db.MapPgError(err)
	}
	out := make([]domain.VATRate, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapVATRate(row))
	}
	return out, nil
}

func (r *Repository) GetVATRateByID(ctx context.Context, id uuid.UUID) (domain.VATRate, error) {
	row, err := r.queries(ctx).GetVATRateByID(ctx, id)
	if err != nil {
		return domain.VATRate{}, db.MapPgError(err)
	}
	return mapVATRate(row), nil
}

func (r *Repository) GetVATRateByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.VATRate, error) {
	row, err := r.queries(ctx).GetVATRateByIDForUpdate(ctx, id)
	if err != nil {
		return domain.VATRate{}, db.MapPgError(err)
	}
	return mapVATRate(row), nil
}

func (r *Repository) GetActiveVATRate(ctx context.Context) (*domain.VATRate, error) {
	row, err := r.queries(ctx).GetActiveVATRate(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, db.MapPgError(err)
	}
	mapped := mapVATRate(row)
	return &mapped, nil
}

func (r *Repository) CreateVATRate(ctx context.Context, v domain.VATRate) (domain.VATRate, error) {
	row, err := r.queries(ctx).CreateVATRate(ctx, sqlcgen.CreateVATRateParams{
		ID:        v.ID,
		Code:      v.Code,
		Name:      v.Name,
		Rate:      v.Rate,
		IsActive:  v.IsActive,
		CreatedAt: v.CreatedAt,
	})
	if err != nil {
		if db.IsUniqueViolation(err) {
			return domain.VATRate{}, apperror.Conflict("VAT rate code or active state conflict").WithCause(err)
		}
		return domain.VATRate{}, db.MapPgError(err)
	}
	return mapVATRate(row), nil
}

func (r *Repository) DeactivateOtherVATRates(ctx context.Context, excludeID uuid.UUID, now time.Time) ([]domain.VATRate, error) {
	rows, err := r.queries(ctx).DeactivateOtherVATRates(ctx, sqlcgen.DeactivateOtherVATRatesParams{
		UpdatedAt: now,
		ExcludeID: excludeID,
	})
	if err != nil {
		return nil, db.MapPgError(err)
	}
	out := make([]domain.VATRate, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapVATRate(row))
	}
	return out, nil
}

func (r *Repository) UpdateVATRate(ctx context.Context, id uuid.UUID, name string, rate decimal.Decimal, isActive bool, expectedVersion int, now time.Time) (domain.VATRate, error) {
	row, err := r.queries(ctx).UpdateVATRate(ctx, sqlcgen.UpdateVATRateParams{
		ID:              id,
		Name:            name,
		Rate:            rate,
		IsActive:        isActive,
		UpdatedAt:       now,
		ExpectedVersion: int32(expectedVersion),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.VATRate{}, apperror.VersionConflict("VAT rate version conflict").WithCause(err)
		}
		if db.IsUniqueViolation(err) {
			return domain.VATRate{}, apperror.Conflict("active VAT rate conflict").WithCause(err)
		}
		return domain.VATRate{}, db.MapPgError(err)
	}
	return mapVATRate(row), nil
}

func (r *Repository) DeactivateVATRate(ctx context.Context, id uuid.UUID, now time.Time) (domain.VATRate, error) {
	row, err := r.queries(ctx).DeactivateVATRate(ctx, sqlcgen.DeactivateVATRateParams{
		ID:        id,
		UpdatedAt: now,
	})
	if err != nil {
		return domain.VATRate{}, db.MapPgError(err)
	}
	return mapVATRate(row), nil
}

func mapVATRate(row sqlcgen.VatRate) domain.VATRate {
	return domain.VATRate{
		ID:        row.ID,
		Code:      row.Code,
		Name:      row.Name,
		Rate:      row.Rate,
		IsActive:  row.IsActive,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
		Version:   int(row.Version),
	}
}
