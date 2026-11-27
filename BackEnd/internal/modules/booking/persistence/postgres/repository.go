package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/booking/domain"
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

func fromPgDate(d pgtype.Date) time.Time {
	if !d.Valid {
		return time.Time{}
	}
	return time.Date(d.Time.Year(), d.Time.Month(), d.Time.Day(), 0, 0, 0, 0, time.UTC)
}

func toPgText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

func fromPgText(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
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

// TryAdvisoryXactLockRoom attempts a non-blocking transaction-scoped advisory lock on roomID.
func (r *Repository) TryAdvisoryXactLockRoom(ctx context.Context, roomID uuid.UUID) (bool, error) {
	locked, err := r.queries(ctx).TryAdvisoryXactLockRoom(ctx, roomID.String())
	if err != nil {
		return false, db.MapPgError(err)
	}
	return locked, nil
}

// InsertReservationHold inserts a new authoritative reservation_holds row.
func (r *Repository) InsertReservationHold(ctx context.Context, h domain.ReservationHold) (domain.ReservationHold, error) {
	row, err := r.queries(ctx).InsertReservationHold(ctx, sqlcgen.InsertReservationHoldParams{
		ID:             h.ID,
		CustomerID:     h.CustomerID,
		BookingGroupID: h.BookingGroupID,
		RoomID:         h.RoomID,
		RoomTypeID:     h.RoomTypeID,
		Column6:        toPgDate(h.CheckIn),
		Column7:        toPgDate(h.CheckOut),
		HoldMode:       string(h.HoldMode),
		HoldToken:      h.HoldToken,
		RedisKey:       h.RedisKey,
		TtlSeconds:     int32(h.TTLSeconds),
		Status:         string(h.Status),
		ExpiresAt:      h.ExpiresAt,
		CreatedAt:      h.CreatedAt,
	})
	if err != nil {
		return domain.ReservationHold{}, db.MapPgError(err)
	}
	return mapHoldRow(row.ID, row.CustomerID, row.BookingGroupID, row.RoomID, row.RoomTypeID, row.CheckIn, row.CheckOut, row.HoldMode, row.HoldToken, row.RedisKey, row.TtlSeconds, row.Status, row.ExpiresAt, row.ReleasedReason, row.CreatedAt, row.UpdatedAt), nil
}

// GetReservationHoldByIDForUpdate locks and returns a single hold by its ID.
func (r *Repository) GetReservationHoldByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.ReservationHold, error) {
	row, err := r.queries(ctx).GetReservationHoldByIDForUpdate(ctx, id)
	if err != nil {
		return domain.ReservationHold{}, db.MapPgError(err)
	}
	return mapHoldRow(row.ID, row.CustomerID, row.BookingGroupID, row.RoomID, row.RoomTypeID, row.CheckIn, row.CheckOut, row.HoldMode, row.HoldToken, row.RedisKey, row.TtlSeconds, row.Status, row.ExpiresAt, row.ReleasedReason, row.CreatedAt, row.UpdatedAt), nil
}

// ListReservationHoldsByGroupIDForUpdate locks and returns all holds sharing booking_group_id in UUID order.
func (r *Repository) ListReservationHoldsByGroupIDForUpdate(ctx context.Context, groupID uuid.UUID) ([]domain.ReservationHold, error) {
	rows, err := r.queries(ctx).ListReservationHoldsByGroupIDForUpdate(ctx, &groupID)
	if err != nil {
		return nil, db.MapPgError(err)
	}
	out := make([]domain.ReservationHold, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapHoldRow(row.ID, row.CustomerID, row.BookingGroupID, row.RoomID, row.RoomTypeID, row.CheckIn, row.CheckOut, row.HoldMode, row.HoldToken, row.RedisKey, row.TtlSeconds, row.Status, row.ExpiresAt, row.ReleasedReason, row.CreatedAt, row.UpdatedAt))
	}
	return out, nil
}

// UpdateReservationHoldsStatus updates status and released_reason for the given hold IDs.
func (r *Repository) UpdateReservationHoldsStatus(ctx context.Context, holdIDs []uuid.UUID, status domain.HoldStatus, reason string, now time.Time) error {
	if len(holdIDs) == 0 {
		return nil
	}
	err := r.queries(ctx).UpdateReservationHoldsStatus(ctx, sqlcgen.UpdateReservationHoldsStatusParams{
		Column1:        holdIDs,
		Status:         string(status),
		ReleasedReason: toPgText(reason),
		UpdatedAt:      now,
	})
	return db.MapPgError(err)
}

// ClaimExpiredReservationHolds locks up to limit active holds whose expires_at <= now using SKIP LOCKED.
func (r *Repository) ClaimExpiredReservationHolds(ctx context.Context, now time.Time, limit int) ([]domain.ReservationHold, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.queries(ctx).ClaimExpiredReservationHolds(ctx, sqlcgen.ClaimExpiredReservationHoldsParams{
		ExpiresAt: now,
		Limit:     int32(limit),
	})
	if err != nil {
		return nil, db.MapPgError(err)
	}
	out := make([]domain.ReservationHold, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapHoldRow(row.ID, row.CustomerID, row.BookingGroupID, row.RoomID, row.RoomTypeID, row.CheckIn, row.CheckOut, row.HoldMode, row.HoldToken, row.RedisKey, row.TtlSeconds, row.Status, row.ExpiresAt, row.ReleasedReason, row.CreatedAt, row.UpdatedAt))
	}
	return out, nil
}

// ListActiveNonExpiredHolds returns all active holds whose expires_at > now ordered by created_at ASC, id ASC.
func (r *Repository) ListActiveNonExpiredHolds(ctx context.Context, now time.Time) ([]domain.ReservationHold, error) {
	rows, err := r.queries(ctx).ListActiveNonExpiredHolds(ctx, now)
	if err != nil {
		return nil, db.MapPgError(err)
	}
	out := make([]domain.ReservationHold, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapHoldRow(row.ID, row.CustomerID, row.BookingGroupID, row.RoomID, row.RoomTypeID, row.CheckIn, row.CheckOut, row.HoldMode, row.HoldToken, row.RedisKey, row.TtlSeconds, row.Status, row.ExpiresAt, row.ReleasedReason, row.CreatedAt, row.UpdatedAt))
	}
	return out, nil
}

// ClaimExpiredPendingBookings locks up to limit pending bookings whose expires_at <= now using SKIP LOCKED.
func (r *Repository) ClaimExpiredPendingBookings(ctx context.Context, now time.Time, limit int) ([]domain.ExpiredBookingSummary, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.queries(ctx).ClaimExpiredPendingBookings(ctx, sqlcgen.ClaimExpiredPendingBookingsParams{
		ExpiresAt: &now,
		Limit:     int32(limit),
	})
	if err != nil {
		return nil, db.MapPgError(err)
	}
	out := make([]domain.ExpiredBookingSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.ExpiredBookingSummary{
			ID:             row.ID,
			ReferenceCode:  row.ReferenceCode,
			BookingGroupID: row.BookingGroupID,
			CustomerID:     row.CustomerID,
			RoomID:         row.RoomID,
			RoomTypeID:     row.RoomTypeID,
			CheckIn:        fromPgDate(row.CheckIn),
			CheckOut:       fromPgDate(row.CheckOut),
		})
	}
	return out, nil
}

// MarkBookingExpired transitions a pending booking to expired.
func (r *Repository) MarkBookingExpired(ctx context.Context, bookingID uuid.UUID, now time.Time) error {
	return db.MapPgError(r.queries(ctx).MarkBookingExpired(ctx, sqlcgen.MarkBookingExpiredParams{
		ID:        bookingID,
		UpdatedAt: now,
	}))
}

// InsertBookingStatusHistory appends an immutable booking status transition entry.
func (r *Repository) InsertBookingStatusHistory(ctx context.Context, entry domain.BookingStatusHistoryEntry) error {
	var fromStr string
	if entry.FromStatus != nil {
		fromStr = string(*entry.FromStatus)
	}
	return db.MapPgError(r.queries(ctx).InsertBookingStatusHistory(ctx, sqlcgen.InsertBookingStatusHistoryParams{
		ID:            entry.ID,
		BookingID:     entry.BookingID,
		FromStatus:    toPgText(fromStr),
		ToStatus:      string(entry.ToStatus),
		ActorUserID:   entry.ActorUserID,
		Reason:        toPgText(entry.Reason),
		CorrelationID: entry.CorrelationID,
		OccurredAt:    entry.OccurredAt,
	}))
}

// RecomputeBookingGroupStatus locks the booking group, reads all child booking statuses, and updates its aggregate status.
func (r *Repository) RecomputeBookingGroupStatus(ctx context.Context, groupID uuid.UUID, now time.Time) (domain.GroupStatus, error) {
	if _, err := r.queries(ctx).LockBookingGroupForUpdate(ctx, groupID); err != nil {
		return "", db.MapPgError(err)
	}
	rawStatuses, err := r.queries(ctx).ListChildBookingStatusesByGroup(ctx, &groupID)
	if err != nil {
		return "", db.MapPgError(err)
	}
	statuses := make([]domain.BookingStatus, 0, len(rawStatuses))
	for _, st := range rawStatuses {
		statuses = append(statuses, domain.BookingStatus(st))
	}
	derived := domain.ComputeGroupStatus(statuses)
	if err := r.queries(ctx).UpdateBookingGroupStatus(ctx, sqlcgen.UpdateBookingGroupStatusParams{
		ID:        groupID,
		Status:    string(derived),
		UpdatedAt: now,
	}); err != nil {
		return "", db.MapPgError(err)
	}
	return derived, nil
}

func mapHoldRow(
	id, customerID uuid.UUID,
	groupID *uuid.UUID,
	roomID, roomTypeID uuid.UUID,
	checkIn, checkOut pgtype.Date,
	holdMode, holdToken, redisKey string,
	ttlSeconds int32,
	status string,
	expiresAt time.Time,
	releasedReason pgtype.Text,
	createdAt, updatedAt time.Time,
) domain.ReservationHold {
	return domain.ReservationHold{
		ID:             id,
		CustomerID:     customerID,
		BookingGroupID: groupID,
		RoomID:         roomID,
		RoomTypeID:     roomTypeID,
		CheckIn:        fromPgDate(checkIn),
		CheckOut:       fromPgDate(checkOut),
		HoldMode:       domain.HoldMode(holdMode),
		HoldToken:      holdToken,
		RedisKey:       redisKey,
		TTLSeconds:     int(ttlSeconds),
		Status:         domain.HoldStatus(status),
		ExpiresAt:      expiresAt,
		ReleasedReason: fromPgText(releasedReason),
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}
}
