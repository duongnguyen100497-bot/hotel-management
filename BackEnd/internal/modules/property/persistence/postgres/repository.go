package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/property/domain"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/property/persistence/postgres/sqlcgen"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/db"
)

// Repository implements PostgreSQL persistence for the property module.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a property PostgreSQL repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) queries(ctx context.Context) *sqlcgen.Queries {
	return sqlcgen.New(db.Querier(ctx, r.pool))
}

func (r *Repository) GetHotelSettings(ctx context.Context) (domain.HotelSettings, error) {
	row, err := r.queries(ctx).GetHotelSettings(ctx)
	if err != nil {
		return domain.HotelSettings{}, db.MapPgError(err)
	}
	return mapGetHotelSettingsRow(row), nil
}

func (r *Repository) GetHotelSettingsForUpdate(ctx context.Context) (domain.HotelSettings, error) {
	row, err := r.queries(ctx).GetHotelSettingsForUpdate(ctx)
	if err != nil {
		return domain.HotelSettings{}, db.MapPgError(err)
	}
	return domain.HotelSettings{
		Name:                     row.Name,
		Address:                  row.Address,
		Phone:                    row.Phone,
		Email:                    row.Email,
		Timezone:                 row.Timezone,
		Currency:                 row.Currency,
		CheckInTime:              row.CheckInTime,
		CheckOutTime:             row.CheckOutTime,
		IndividualHoldTTLMinutes: int(row.IndividualHoldTtlMinutes),
		GroupHoldTTLMinutes:      int(row.GroupHoldTtlMinutes),
		FreeCancelHours:          int(row.FreeCancelHours),
		LogoMediaID:              row.LogoMediaID,
		UpdatedAt:                row.UpdatedAt,
		Version:                  int(row.Version),
	}, nil
}

func (r *Repository) UpdateHotelSettings(ctx context.Context, settings domain.HotelSettings, expectedVersion int, now time.Time) (domain.HotelSettings, error) {
	row, err := r.queries(ctx).UpdateHotelSettings(ctx, sqlcgen.UpdateHotelSettingsParams{
		Name:                     settings.Name,
		Address:                  settings.Address,
		Phone:                    settings.Phone,
		Email:                    settings.Email,
		Timezone:                 settings.Timezone,
		Currency:                 settings.Currency,
		CheckInTime:              settings.CheckInTime,
		CheckOutTime:             settings.CheckOutTime,
		IndividualHoldTtlMinutes: int32(settings.IndividualHoldTTLMinutes),
		GroupHoldTtlMinutes:      int32(settings.GroupHoldTTLMinutes),
		FreeCancelHours:          int32(settings.FreeCancelHours),
		LogoMediaID:              settings.LogoMediaID,
		UpdatedAt:                now,
		ExpectedVersion:          int32(expectedVersion),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.HotelSettings{}, apperror.VersionConflict("hotel settings version conflict").WithCause(err)
		}
		return domain.HotelSettings{}, db.MapPgError(err)
	}
	return domain.HotelSettings{
		Name:                     row.Name,
		Address:                  row.Address,
		Phone:                    row.Phone,
		Email:                    row.Email,
		Timezone:                 row.Timezone,
		Currency:                 row.Currency,
		CheckInTime:              row.CheckInTime,
		CheckOutTime:             row.CheckOutTime,
		IndividualHoldTTLMinutes: int(row.IndividualHoldTtlMinutes),
		GroupHoldTTLMinutes:      int(row.GroupHoldTtlMinutes),
		FreeCancelHours:          int(row.FreeCancelHours),
		LogoMediaID:              row.LogoMediaID,
		UpdatedAt:                row.UpdatedAt,
		Version:                  int(row.Version),
	}, nil
}

func mapGetHotelSettingsRow(row sqlcgen.GetHotelSettingsRow) domain.HotelSettings {
	return domain.HotelSettings{
		Name:                     row.Name,
		Address:                  row.Address,
		Phone:                    row.Phone,
		Email:                    row.Email,
		Timezone:                 row.Timezone,
		Currency:                 row.Currency,
		CheckInTime:              row.CheckInTime,
		CheckOutTime:             row.CheckOutTime,
		IndividualHoldTTLMinutes: int(row.IndividualHoldTtlMinutes),
		GroupHoldTTLMinutes:      int(row.GroupHoldTtlMinutes),
		FreeCancelHours:          int(row.FreeCancelHours),
		LogoMediaID:              row.LogoMediaID,
		UpdatedAt:                row.UpdatedAt,
		Version:                  int(row.Version),
	}
}
