package postgres

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/booking/domain"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/booking/persistence/postgres/sqlcgen"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
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

type timeIDCursor struct {
	CreatedAt time.Time `json:"t"`
	ID        uuid.UUID `json:"id"`
}

func encodeTimeIDCursor(t time.Time, id uuid.UUID) string {
	raw, _ := json.Marshal(timeIDCursor{CreatedAt: t.UTC(), ID: id})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeTimeIDCursor(cursor string) (*time.Time, uuid.UUID, error) {
	if cursor == "" {
		return nil, uuid.Nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, uuid.Nil, apperror.Validation("invalid cursor", apperror.FieldError{Field: "cursor", Message: "malformed cursor"})
	}
	var c timeIDCursor
	if err := json.Unmarshal(raw, &c); err != nil || c.CreatedAt.IsZero() || c.ID == uuid.Nil {
		return nil, uuid.Nil, apperror.Validation("invalid cursor", apperror.FieldError{Field: "cursor", Message: "malformed cursor"})
	}
	return &c.CreatedAt, c.ID, nil
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

// FindConvertedHoldsForRoomStay returns converted holds matching room_id and stay so post-commit Redis cleanup can release keys.
func (r *Repository) FindConvertedHoldsForRoomStay(ctx context.Context, roomID uuid.UUID, checkIn, checkOut time.Time) ([]domain.ReservationHold, error) {
	rows, err := r.queries(ctx).FindConvertedHoldForRoomStay(ctx, sqlcgen.FindConvertedHoldForRoomStayParams{
		RoomID:  roomID,
		Column2: toPgDate(checkIn),
		Column3: toPgDate(checkOut),
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

// InsertBooking persists a new booking row. Returns raw pgx/mapped error (preserving 23P01 detection via db.IsExclusionViolation).
func (r *Repository) InsertBooking(ctx context.Context, b domain.Booking) (domain.Booking, error) {
	row, err := r.queries(ctx).InsertBooking(ctx, sqlcgen.InsertBookingParams{
		ID:                  b.ID,
		ReferenceCode:       b.ReferenceCode,
		BookingGroupID:      b.BookingGroupID,
		CustomerID:          b.CustomerID,
		RoomID:              b.RoomID,
		RoomTypeID:          b.RoomTypeID,
		Column7:             toPgDate(b.CheckInTime),
		Column8:             toPgDate(b.CheckOutTime),
		Adults:              int16(b.Adults),
		Children:            int16(b.Children),
		Status:              string(b.Status),
		NightlyRateSnapshot: b.NightlyRateSnapshot,
		VatRateID:           b.VatRateID,
		VatRateSnapshot:     b.VatRateSnapshot,
		SubtotalAmount:      b.SubtotalAmount,
		VatAmount:           b.VatAmount,
		TotalAmount:         b.TotalAmount,
		Currency:            b.Currency,
		SpecialRequest:      toPgText(b.SpecialRequest),
		ExpiresAt:           b.ExpiresAt,
		IdempotencyKey:      toPgText(b.IdempotencyKey),
		CreatedAt:           b.CreatedAt,
	})
	if err != nil {
		return domain.Booking{}, db.MapPgError(err)
	}
	return mapBookingCore(row.ID, row.ReferenceCode, row.BookingGroupID, row.CustomerID, row.RoomID, row.RoomTypeID, row.CheckIn, row.CheckOut, row.Adults, row.Children, row.Status, row.NightlyRateSnapshot, row.VatRateID, row.VatRateSnapshot, row.SubtotalAmount, row.VatAmount, row.TotalAmount, row.Currency, row.SpecialRequest, row.ExpiresAt, row.CancelledAt, row.CancelReason, row.IdempotencyKey, row.CreatedAt, row.UpdatedAt, row.Version, nil), nil
}

// InsertBookingGuestSnapshot inserts the immutable representative guest snapshot for a booking.
func (r *Repository) InsertBookingGuestSnapshot(ctx context.Context, bookingID uuid.UUID, g domain.GuestSnapshot) (domain.GuestSnapshot, error) {
	row, err := r.queries(ctx).InsertBookingGuestSnapshot(ctx, sqlcgen.InsertBookingGuestSnapshotParams{
		BookingID:       bookingID,
		FullName:        g.FullName,
		Phone:           g.Phone,
		Email:           toPgText(g.Email),
		NationalityCode: toPgText(g.NationalityCode),
		SpecialRequest:  toPgText(g.SpecialRequest),
		CreatedAt:       g.CreatedAt,
	})
	if err != nil {
		return domain.GuestSnapshot{}, db.MapPgError(err)
	}
	return domain.GuestSnapshot{
		FullName:        row.FullName,
		Phone:           row.Phone,
		Email:           fromPgText(row.Email),
		NationalityCode: fromPgText(row.NationalityCode),
		SpecialRequest:  fromPgText(row.SpecialRequest),
		CreatedAt:       row.CreatedAt,
	}, nil
}

// GetBookingByID fetches a booking with its immutable guest snapshot.
func (r *Repository) GetBookingByID(ctx context.Context, id uuid.UUID) (domain.Booking, error) {
	row, err := r.queries(ctx).GetBookingByID(ctx, id)
	if err != nil {
		return domain.Booking{}, db.MapPgError(err)
	}
	return mapBookingWithGuest(row.ID, row.ReferenceCode, row.BookingGroupID, row.CustomerID, row.RoomID, row.RoomTypeID, row.CheckIn, row.CheckOut, row.Adults, row.Children, row.Status, row.NightlyRateSnapshot, row.VatRateID, row.VatRateSnapshot, row.SubtotalAmount, row.VatAmount, row.TotalAmount, row.Currency, row.SpecialRequest, row.ExpiresAt, row.CancelledAt, row.CancelReason, row.IdempotencyKey, row.CreatedAt, row.UpdatedAt, row.Version, row.GuestFullName, row.GuestPhone, row.GuestEmail, row.GuestNationalityCode, row.GuestSpecialRequest, row.GuestCreatedAt), nil
}

// GetBookingByIDForUpdate locks and returns a booking row.
func (r *Repository) GetBookingByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.Booking, error) {
	row, err := r.queries(ctx).GetBookingByIDForUpdate(ctx, id)
	if err != nil {
		return domain.Booking{}, db.MapPgError(err)
	}
	return mapBookingCore(row.ID, row.ReferenceCode, row.BookingGroupID, row.CustomerID, row.RoomID, row.RoomTypeID, row.CheckIn, row.CheckOut, row.Adults, row.Children, row.Status, row.NightlyRateSnapshot, row.VatRateID, row.VatRateSnapshot, row.SubtotalAmount, row.VatAmount, row.TotalAmount, row.Currency, row.SpecialRequest, row.ExpiresAt, row.CancelledAt, row.CancelReason, row.IdempotencyKey, row.CreatedAt, row.UpdatedAt, row.Version, nil), nil
}

// FindBookingByCustomerIdempotencyKey looks up an existing booking for (customerID, key). Returns (nil, nil) if not found.
func (r *Repository) FindBookingByCustomerIdempotencyKey(ctx context.Context, customerID uuid.UUID, key string) (*domain.Booking, error) {
	if key == "" {
		return nil, nil
	}
	row, err := r.queries(ctx).GetBookingByCustomerIdempotencyKey(ctx, sqlcgen.GetBookingByCustomerIdempotencyKeyParams{
		CustomerID:     customerID,
		IdempotencyKey: toPgText(key),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, db.MapPgError(err)
	}
	b := mapBookingWithGuest(row.ID, row.ReferenceCode, row.BookingGroupID, row.CustomerID, row.RoomID, row.RoomTypeID, row.CheckIn, row.CheckOut, row.Adults, row.Children, row.Status, row.NightlyRateSnapshot, row.VatRateID, row.VatRateSnapshot, row.SubtotalAmount, row.VatAmount, row.TotalAmount, row.Currency, row.SpecialRequest, row.ExpiresAt, row.CancelledAt, row.CancelReason, row.IdempotencyKey, row.CreatedAt, row.UpdatedAt, row.Version, row.GuestFullName, row.GuestPhone, row.GuestEmail, row.GuestNationalityCode, row.GuestSpecialRequest, row.GuestCreatedAt)
	return &b, nil
}

// UpdateBookingStatusConfirmed transitions a pending booking to confirmed and clears expires_at.
func (r *Repository) UpdateBookingStatusConfirmed(ctx context.Context, bookingID uuid.UUID, now time.Time) (domain.Booking, error) {
	row, err := r.queries(ctx).UpdateBookingStatusConfirmed(ctx, sqlcgen.UpdateBookingStatusConfirmedParams{
		ID:        bookingID,
		UpdatedAt: now,
	})
	if err != nil {
		return domain.Booking{}, db.MapPgError(err)
	}
	return mapBookingCore(row.ID, row.ReferenceCode, row.BookingGroupID, row.CustomerID, row.RoomID, row.RoomTypeID, row.CheckIn, row.CheckOut, row.Adults, row.Children, row.Status, row.NightlyRateSnapshot, row.VatRateID, row.VatRateSnapshot, row.SubtotalAmount, row.VatAmount, row.TotalAmount, row.Currency, row.SpecialRequest, row.ExpiresAt, row.CancelledAt, row.CancelReason, row.IdempotencyKey, row.CreatedAt, row.UpdatedAt, row.Version, nil), nil
}

// UpdateBookingStatusCheckedIn transitions a confirmed booking to checked_in.
func (r *Repository) UpdateBookingStatusCheckedIn(ctx context.Context, bookingID uuid.UUID, now time.Time) (domain.Booking, error) {
	row, err := r.queries(ctx).UpdateBookingStatusCheckedIn(ctx, sqlcgen.UpdateBookingStatusCheckedInParams{
		ID:        bookingID,
		UpdatedAt: now,
	})
	if err != nil {
		return domain.Booking{}, db.MapPgError(err)
	}
	return mapBookingCore(row.ID, row.ReferenceCode, row.BookingGroupID, row.CustomerID, row.RoomID, row.RoomTypeID, row.CheckIn, row.CheckOut, row.Adults, row.Children, row.Status, row.NightlyRateSnapshot, row.VatRateID, row.VatRateSnapshot, row.SubtotalAmount, row.VatAmount, row.TotalAmount, row.Currency, row.SpecialRequest, row.ExpiresAt, row.CancelledAt, row.CancelReason, row.IdempotencyKey, row.CreatedAt, row.UpdatedAt, row.Version, nil), nil
}

// UpdateBookingStatusCheckedOut transitions a checked_in booking to checked_out.
func (r *Repository) UpdateBookingStatusCheckedOut(ctx context.Context, bookingID uuid.UUID, now time.Time) (domain.Booking, error) {
	row, err := r.queries(ctx).UpdateBookingStatusCheckedOut(ctx, sqlcgen.UpdateBookingStatusCheckedOutParams{
		ID:        bookingID,
		UpdatedAt: now,
	})
	if err != nil {
		return domain.Booking{}, db.MapPgError(err)
	}
	return mapBookingCore(row.ID, row.ReferenceCode, row.BookingGroupID, row.CustomerID, row.RoomID, row.RoomTypeID, row.CheckIn, row.CheckOut, row.Adults, row.Children, row.Status, row.NightlyRateSnapshot, row.VatRateID, row.VatRateSnapshot, row.SubtotalAmount, row.VatAmount, row.TotalAmount, row.Currency, row.SpecialRequest, row.ExpiresAt, row.CancelledAt, row.CancelReason, row.IdempotencyKey, row.CreatedAt, row.UpdatedAt, row.Version, nil), nil
}

// ListBookings lists bookings with optional filters and cursor pagination.
func (r *Repository) ListBookings(ctx context.Context, customerID, groupID *uuid.UUID, status, cursor string, limit int) ([]domain.Booking, *string, error) {
	if limit <= 0 {
		limit = 20
	}
	cursorTime, cursorID, err := decodeTimeIDCursor(cursor)
	if err != nil {
		return nil, nil, err
	}
	var cursorIDPtr *uuid.UUID
	if cursorID != uuid.Nil {
		idCopy := cursorID
		cursorIDPtr = &idCopy
	}
	rows, err := r.queries(ctx).ListBookingsFiltered(ctx, sqlcgen.ListBookingsFilteredParams{
		CustomerID:      customerID,
		BookingGroupID:  groupID,
		Status:          status,
		CursorCreatedAt: cursorTime,
		CursorID:        cursorIDPtr,
		LimitRows:       int32(limit + 1),
	})
	if err != nil {
		return nil, nil, db.MapPgError(err)
	}

	var nextCursor *string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		nc := encodeTimeIDCursor(last.CreatedAt, last.ID)
		nextCursor = &nc
	}
	out := make([]domain.Booking, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapBookingWithGuest(row.ID, row.ReferenceCode, row.BookingGroupID, row.CustomerID, row.RoomID, row.RoomTypeID, row.CheckIn, row.CheckOut, row.Adults, row.Children, row.Status, row.NightlyRateSnapshot, row.VatRateID, row.VatRateSnapshot, row.SubtotalAmount, row.VatAmount, row.TotalAmount, row.Currency, row.SpecialRequest, row.ExpiresAt, row.CancelledAt, row.CancelReason, row.IdempotencyKey, row.CreatedAt, row.UpdatedAt, row.Version, row.GuestFullName, row.GuestPhone, row.GuestEmail, row.GuestNationalityCode, row.GuestSpecialRequest, row.GuestCreatedAt))
	}
	return out, nextCursor, nil
}

// ListBookingsByGroupID returns all child bookings belonging to a booking group ordered by ID.
func (r *Repository) ListBookingsByGroupID(ctx context.Context, groupID uuid.UUID) ([]domain.Booking, error) {
	rows, err := r.queries(ctx).ListBookingsByGroupID(ctx, &groupID)
	if err != nil {
		return nil, db.MapPgError(err)
	}
	out := make([]domain.Booking, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapBookingWithGuest(row.ID, row.ReferenceCode, row.BookingGroupID, row.CustomerID, row.RoomID, row.RoomTypeID, row.CheckIn, row.CheckOut, row.Adults, row.Children, row.Status, row.NightlyRateSnapshot, row.VatRateID, row.VatRateSnapshot, row.SubtotalAmount, row.VatAmount, row.TotalAmount, row.Currency, row.SpecialRequest, row.ExpiresAt, row.CancelledAt, row.CancelReason, row.IdempotencyKey, row.CreatedAt, row.UpdatedAt, row.Version, row.GuestFullName, row.GuestPhone, row.GuestEmail, row.GuestNationalityCode, row.GuestSpecialRequest, row.GuestCreatedAt))
	}
	return out, nil
}

// InsertBookingGroup inserts a new booking_groups aggregate row.
func (r *Repository) InsertBookingGroup(ctx context.Context, g domain.BookingGroup) (domain.BookingGroup, error) {
	row, err := r.queries(ctx).InsertBookingGroup(ctx, sqlcgen.InsertBookingGroupParams{
		ID:             g.ID,
		ReferenceCode:  g.ReferenceCode,
		CustomerID:     g.CustomerID,
		Status:         string(g.Status),
		ExpiresAt:      g.ExpiresAt,
		IdempotencyKey: toPgText(g.IdempotencyKey),
		CreatedAt:      g.CreatedAt,
	})
	if err != nil {
		return domain.BookingGroup{}, db.MapPgError(err)
	}
	return mapBookingGroupRow(row), nil
}

// GetBookingGroupByID returns a booking group header by ID.
func (r *Repository) GetBookingGroupByID(ctx context.Context, id uuid.UUID) (domain.BookingGroup, error) {
	row, err := r.queries(ctx).GetBookingGroupByID(ctx, id)
	if err != nil {
		return domain.BookingGroup{}, db.MapPgError(err)
	}
	return mapBookingGroupRow(row), nil
}

// FindBookingGroupByCustomerIdempotencyKey returns (nil, nil) if not found.
func (r *Repository) FindBookingGroupByCustomerIdempotencyKey(ctx context.Context, customerID uuid.UUID, key string) (*domain.BookingGroup, error) {
	if key == "" {
		return nil, nil
	}
	row, err := r.queries(ctx).GetBookingGroupByCustomerIdempotencyKey(ctx, sqlcgen.GetBookingGroupByCustomerIdempotencyKeyParams{
		CustomerID:     customerID,
		IdempotencyKey: toPgText(key),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, db.MapPgError(err)
	}
	g := mapBookingGroupRow(row)
	return &g, nil
}

// ListBookingGroupsByCustomer lists booking groups owned by customerID with cursor pagination.
func (r *Repository) ListBookingGroupsByCustomer(ctx context.Context, customerID uuid.UUID, cursor string, limit int) ([]domain.BookingGroup, *string, error) {
	if limit <= 0 {
		limit = 20
	}
	cursorTime, cursorID, err := decodeTimeIDCursor(cursor)
	if err != nil {
		return nil, nil, err
	}
	var cursorIDPtr *uuid.UUID
	if cursorID != uuid.Nil {
		idCopy := cursorID
		cursorIDPtr = &idCopy
	}
	rows, err := r.queries(ctx).ListBookingGroupsByCustomer(ctx, sqlcgen.ListBookingGroupsByCustomerParams{
		CustomerID:      customerID,
		CursorCreatedAt: cursorTime,
		CursorID:        cursorIDPtr,
		LimitRows:       int32(limit + 1),
	})
	if err != nil {
		return nil, nil, db.MapPgError(err)
	}
	var nextCursor *string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		nc := encodeTimeIDCursor(last.CreatedAt, last.ID)
		nextCursor = &nc
	}
	out := make([]domain.BookingGroup, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapBookingGroupRow(row))
	}
	return out, nextCursor, nil
}

func mapBookingGroupRow(row sqlcgen.BookingGroup) domain.BookingGroup {
	return domain.BookingGroup{
		ID:             row.ID,
		ReferenceCode:  row.ReferenceCode,
		CustomerID:     row.CustomerID,
		Status:         domain.GroupStatus(row.Status),
		ExpiresAt:      row.ExpiresAt,
		IdempotencyKey: fromPgText(row.IdempotencyKey),
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Version:        int(row.Version),
	}
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

func mapBookingCore(
	id uuid.UUID,
	refCode string,
	groupID *uuid.UUID,
	customerID, roomID, roomTypeID uuid.UUID,
	checkIn, checkOut pgtype.Date,
	adults, children int16,
	status string,
	nightlyRate decimal.Decimal,
	vatRateID uuid.UUID,
	vatRate, subtotal, vatAmt, totalAmt decimal.Decimal,
	currency string,
	specialReq pgtype.Text,
	expiresAt, cancelledAt *time.Time,
	cancelReason, idempKey pgtype.Text,
	createdAt, updatedAt time.Time,
	version int32,
	guest *domain.GuestSnapshot,
) domain.Booking {
	inTime := fromPgDate(checkIn)
	outTime := fromPgDate(checkOut)
	nights := int(outTime.Sub(inTime).Hours() / 24)
	return domain.Booking{
		ID:                  id,
		ReferenceCode:       refCode,
		BookingGroupID:      groupID,
		CustomerID:          customerID,
		RoomID:              roomID,
		RoomTypeID:          roomTypeID,
		CheckIn:             inTime.Format(domain.DateLayout),
		CheckOut:            outTime.Format(domain.DateLayout),
		CheckInTime:         inTime,
		CheckOutTime:        outTime,
		Nights:              nights,
		Adults:              int(adults),
		Children:            int(children),
		Status:              domain.BookingStatus(status),
		NightlyRateSnapshot: nightlyRate,
		VatRateID:           vatRateID,
		VatRateSnapshot:     vatRate,
		SubtotalAmount:      subtotal,
		VatAmount:           vatAmt,
		TotalAmount:         totalAmt,
		Currency:            currency,
		SpecialRequest:      fromPgText(specialReq),
		ExpiresAt:           expiresAt,
		CancelledAt:         cancelledAt,
		CancelReason:        fromPgText(cancelReason),
		IdempotencyKey:      fromPgText(idempKey),
		RepresentativeGuest: guest,
		CreatedAt:           createdAt,
		UpdatedAt:           updatedAt,
		Version:             int(version),
	}
}

func mapBookingWithGuest(
	id uuid.UUID,
	refCode string,
	groupID *uuid.UUID,
	customerID, roomID, roomTypeID uuid.UUID,
	checkIn, checkOut pgtype.Date,
	adults, children int16,
	status string,
	nightlyRate decimal.Decimal,
	vatRateID uuid.UUID,
	vatRate, subtotal, vatAmt, totalAmt decimal.Decimal,
	currency string,
	specialReq pgtype.Text,
	expiresAt, cancelledAt *time.Time,
	cancelReason, idempKey pgtype.Text,
	createdAt, updatedAt time.Time,
	version int32,
	guestFullName, guestPhone, guestEmail, guestNationality, guestSpecialReq pgtype.Text,
	guestCreatedAt *time.Time,
) domain.Booking {
	var guest *domain.GuestSnapshot
	if guestFullName.Valid {
		g := domain.GuestSnapshot{
			FullName:        guestFullName.String,
			Phone:           fromPgText(guestPhone),
			Email:           fromPgText(guestEmail),
			NationalityCode: fromPgText(guestNationality),
			SpecialRequest:  fromPgText(guestSpecialReq),
		}
		if guestCreatedAt != nil {
			g.CreatedAt = *guestCreatedAt
		}
		guest = &g
	}
	return mapBookingCore(id, refCode, groupID, customerID, roomID, roomTypeID, checkIn, checkOut, adults, children, status, nightlyRate, vatRateID, vatRate, subtotal, vatAmt, totalAmt, currency, specialReq, expiresAt, cancelledAt, cancelReason, idempKey, createdAt, updatedAt, version, guest)
}
