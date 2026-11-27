package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/booking/persistence/postgres/sqlcgen"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/db"
)

// Repository implements PostgreSQL persistence for the booking module.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a PostgreSQL booking repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) queries(ctx context.Context) *sqlcgen.Queries {
	return sqlcgen.New(db.Querier(ctx, r.pool))
}

func toPgDate(t time.Time) pgtype.Date {
	return pgtype.Date{
		Time:  time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC),
		Valid: true,
	}
}

// ListUnavailableRoomIDs returns room IDs among roomIDs that have an overlapping active booking
// or an overlapping non-expired active reservation hold for [checkIn, checkOut).
func (r *Repository) ListUnavailableRoomIDs(ctx context.Context, roomIDs []uuid.UUID, checkIn, checkOut time.Time, now time.Time) (map[uuid.UUID]struct{}, error) {
	result := make(map[uuid.UUID]struct{})
	if len(roomIDs) == 0 {
		return result, nil
	}
	ids, err := r.queries(ctx).ListUnavailableRoomIDsForStay(ctx, sqlcgen.ListUnavailableRoomIDsForStayParams{
		Column1:   roomIDs,
		Column2:   toPgDate(checkIn),
		Column3:   toPgDate(checkOut),
		ExpiresAt: now,
	})
	if err != nil {
		return nil, db.MapPgError(err)
	}
	for _, id := range ids {
		result[id] = struct{}{}
	}
	return result, nil
}

// HasActiveBookingForCustomer reports whether the customer has any pending, confirmed, or checked-in booking.
func (r *Repository) HasActiveBookingForCustomer(ctx context.Context, customerID uuid.UUID) (bool, error) {
	hasActive, err := r.queries(ctx).HasActiveBookingForCustomer(ctx, customerID)
	if err != nil {
		return false, db.MapPgError(err)
	}
	return hasActive, nil
}
