package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/booking/domain"
	propertyapp "github.com/trandinh0506/hotel-management/backend/internal/modules/property/application"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/db"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
)

var _ BookingStatusService = (*Service)(nil)

// CreateBookingParams defines inputs for POST /api/v1/bookings.
// Supports both hold-based booking (HoldID + HoldToken) and staff walk-in booking (RoomTypeID/RoomID + CheckIn + CheckOut).
type CreateBookingParams struct {
	IdempotencyKey      string
	CustomerID          *uuid.UUID
	HoldID              *uuid.UUID
	HoldToken           string
	RoomTypeID          *uuid.UUID
	RoomID              *uuid.UUID
	CheckIn             string
	CheckOut            string
	Adults              int
	Children            int
	SpecialRequest      string
	RepresentativeGuest domain.GuestSnapshot
}

// ListBookingsFilter defines filter parameters for GET /api/v1/bookings and GET /api/v1/me/bookings.
type ListBookingsFilter struct {
	CustomerID     *uuid.UUID
	BookingGroupID *uuid.UUID
	Status         string
	Cursor         string
	Limit          int
}

// CreateBooking converts an active individual hold (or creates a staff walk-in booking)
// into a durable pending booking with immutable guest and pricing/VAT snapshots.
// Returns (booking, replayed, err).
func (s *Service) CreateBooking(ctx context.Context, actor auth.Principal, params CreateBookingParams) (domain.Booking, bool, error) {
	idempKey := strings.TrimSpace(params.IdempotencyKey)
	if idempKey == "" {
		return domain.Booking{}, false, apperror.Validation("Idempotency-Key header is required", apperror.FieldError{
			Field:   "Idempotency-Key",
			Message: "Idempotency-Key header is required",
		})
	}
	if len(idempKey) > 128 {
		return domain.Booking{}, false, apperror.Validation("Idempotency-Key is too long", apperror.FieldError{
			Field:   "Idempotency-Key",
			Message: "must be at most 128 characters",
		})
	}

	customerID, err := s.resolveBookingCustomer(ctx, actor, params.CustomerID, "booking.create")
	if err != nil {
		return domain.Booking{}, false, err
	}

	// Fast idempotency check before validating hold (since the hold is already converted on replay).
	existing, err := s.repo.FindBookingByCustomerIdempotencyKey(ctx, customerID, idempKey)
	if err != nil {
		return domain.Booking{}, false, err
	}
	if existing != nil {
		return *existing, true, nil
	}

	guest := params.RepresentativeGuest
	if err := guest.NormalizeAndValidate("representativeGuest"); err != nil {
		return domain.Booking{}, false, err
	}
	if params.Adults <= 0 {
		return domain.Booking{}, false, apperror.Validation("invalid guest count", apperror.FieldError{
			Field:   "adults",
			Message: "must be greater than 0",
		})
	}
	if params.Children < 0 {
		return domain.Booking{}, false, apperror.Validation("invalid guest count", apperror.FieldError{
			Field:   "children",
			Message: "must be non-negative",
		})
	}

	isWalkIn := params.HoldID == nil || *params.HoldID == uuid.Nil
	if isWalkIn && actor.Kind != "staff" {
		return domain.Booking{}, false, apperror.Validation("holdId is required", apperror.FieldError{
			Field:   "holdId",
			Message: "holdId and holdToken are required for customer bookings",
		})
	}

	now := s.clk.Now()
	var (
		created             domain.Booking
		replayed            bool
		conflictRedisKey    string
		conflictHoldToken   string
		hadExclusionFailure bool
	)

	txErr := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		// Re-check idempotency inside transaction for concurrent replay safety.
		if dup, err := s.repo.FindBookingByCustomerIdempotencyKey(txCtx, customerID, idempKey); err != nil {
			return err
		} else if dup != nil {
			created = *dup
			replayed = true
			return nil
		}

		var (
			roomID     uuid.UUID
			roomTypeID uuid.UUID
			stay       domain.StayRange
			expiresAt  time.Time
			holdToConv *domain.ReservationHold
		)

		if !isWalkIn {
			hold, err := s.repo.GetReservationHoldByIDForUpdate(txCtx, *params.HoldID)
			if err != nil {
				return err
			}
			if hold.CustomerID != customerID {
				return apperror.Forbidden("you do not own this reservation hold")
			}
			if hold.HoldMode != domain.HoldModeIndividual {
				return apperror.Validation("group hold must be converted via POST /api/v1/booking-groups", apperror.FieldError{
					Field:   "holdId",
					Message: "cannot convert a group hold as an individual booking",
				})
			}
			if strings.TrimSpace(params.HoldToken) == "" || hold.HoldToken != strings.TrimSpace(params.HoldToken) {
				return apperror.Validation("invalid holdToken", apperror.FieldError{
					Field:   "holdToken",
					Message: "holdToken does not match",
				})
			}
			if hold.Status != domain.HoldStatusActive || !hold.ExpiresAt.After(now) {
				return apperror.Conflict("reservation hold is expired or no longer active")
			}
			roomID = hold.RoomID
			roomTypeID = hold.RoomTypeID
			stay = domain.StayRange{CheckIn: hold.CheckIn, CheckOut: hold.CheckOut}
			expiresAt = hold.ExpiresAt
			holdToConv = &hold
			conflictRedisKey = hold.RedisKey
			conflictHoldToken = hold.HoldToken
		} else {
			// Staff walk-in booking: validate dates and allocate an available room.
			cfg, err := s.settings.GetSettings(txCtx)
			if err != nil {
				return err
			}
			loc, locErr := cfg.Location()
			if locErr != nil {
				loc = time.UTC
			}
			stay, err = domain.ParseStayRange(params.CheckIn, params.CheckOut, now, loc)
			if err != nil {
				return err
			}
			expiresAt = now.Add(cfg.IndividualHoldTTL())

			if params.RoomID != nil && *params.RoomID != uuid.Nil {
				rm, err := s.catalog.GetRoomByID(txCtx, *params.RoomID)
				if err != nil {
					return err
				}
				if rm.ArchivedAt != nil || rm.OperationalStatus == "maintenance" {
					return apperror.Conflict("selected room is not bookable")
				}
				roomID = rm.ID
				roomTypeID = rm.RoomTypeID
			} else if params.RoomTypeID != nil && *params.RoomTypeID != uuid.Nil {
				rooms, err := s.catalog.ListBookableRoomsByType(txCtx, *params.RoomTypeID)
				if err != nil {
					return err
				}
				sort.Slice(rooms, func(i, j int) bool { return rooms[i].ID.String() < rooms[j].ID.String() })
				ids := make([]uuid.UUID, 0, len(rooms))
				for _, rm := range rooms {
					ids = append(ids, rm.ID)
				}
				busyMap, err := s.repo.ListUnavailableRoomIDs(txCtx, ids, stay.CheckIn, stay.CheckOut, now)
				if err != nil {
					return err
				}
				var picked *propertyapp.Room
				for i := range rooms {
					if _, busy := busyMap[rooms[i].ID]; busy {
						continue
					}
					locked, err := s.repo.TryAdvisoryXactLockRoom(txCtx, rooms[i].ID)
					if err != nil {
						return err
					}
					if !locked {
						continue
					}
					picked = &rooms[i]
					break
				}
				if picked == nil {
					return apperror.Conflict("no available room for the selected room type and dates")
				}
				roomID = picked.ID
				roomTypeID = picked.RoomTypeID
			} else {
				return apperror.Validation("roomId or roomTypeId is required for walk-in booking", apperror.FieldError{
					Field:   "roomTypeId",
					Message: "provide roomId or roomTypeId",
				})
			}
		}

		rt, err := s.catalog.GetRoomTypeByID(txCtx, roomTypeID)
		if err != nil {
			return err
		}
		if params.Adults+params.Children > rt.Capacity {
			return apperror.Validation("guest count exceeds room capacity", apperror.FieldError{
				Field:   "adults",
				Message: fmt.Sprintf("total guests (%d) exceeds room capacity (%d)", params.Adults+params.Children, rt.Capacity),
			})
		}

		nights := stay.Nights()
		subtotal := rt.BaseNightlyRate.Mul(decimal.NewFromInt(int64(nights)))
		snap, err := s.pricing.Calculate(txCtx, subtotal)
		if err != nil {
			return err
		}
		if snap.VatRateID == nil || *snap.VatRateID == uuid.Nil {
			return apperror.Conflict("no active VAT rate configured")
		}

		bookingID := s.ids.NewUUID()
		b, err := s.repo.InsertBooking(txCtx, domain.Booking{
			ID:                  bookingID,
			ReferenceCode:       domain.GenerateReferenceCode("BK", now),
			CustomerID:          customerID,
			RoomID:              roomID,
			RoomTypeID:          roomTypeID,
			CheckInTime:         stay.CheckIn,
			CheckOutTime:        stay.CheckOut,
			Adults:              params.Adults,
			Children:            params.Children,
			Status:              domain.BookingStatusPending,
			NightlyRateSnapshot: rt.BaseNightlyRate,
			VatRateID:           *snap.VatRateID,
			VatRateSnapshot:     snap.VatRate,
			SubtotalAmount:      snap.Subtotal,
			VatAmount:           snap.VatAmount,
			TotalAmount:         snap.TotalAmount,
			Currency:            snap.Currency,
			SpecialRequest:      strings.TrimSpace(params.SpecialRequest),
			ExpiresAt:           &expiresAt,
			IdempotencyKey:      idempKey,
			CreatedAt:           now,
		})
		if err != nil {
			if db.IsExclusionViolation(err) {
				hadExclusionFailure = true
			}
			return err
		}

		guest.CreatedAt = now
		savedGuest, err := s.repo.InsertBookingGuestSnapshot(txCtx, bookingID, guest)
		if err != nil {
			return err
		}
		b.RepresentativeGuest = &savedGuest

		var actorID *uuid.UUID
		if actor.UserID != uuid.Nil {
			uid := actor.UserID
			actorID = &uid
		}
		if err := s.repo.InsertBookingStatusHistory(txCtx, domain.BookingStatusHistoryEntry{
			ID:            s.ids.NewUUID(),
			BookingID:     bookingID,
			FromStatus:    nil,
			ToStatus:      domain.BookingStatusPending,
			ActorUserID:   actorID,
			Reason:        "booking_created",
			CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
			OccurredAt:    now,
		}); err != nil {
			return err
		}

		if holdToConv != nil {
			if err := s.repo.UpdateReservationHoldsStatus(txCtx, []uuid.UUID{holdToConv.ID}, domain.HoldStatusConverted, "converted_to_booking", now); err != nil {
				return err
			}
		}

		if s.audit != nil {
			if err := s.audit.Record(txCtx, auditapp.AuditEntry{
				ActorUserID:   actorID,
				Action:        "booking.created",
				ResourceType:  "booking",
				ResourceID:    &b.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				AfterData: map[string]any{
					"reference_code": b.ReferenceCode,
					"customer_id":    b.CustomerID.String(),
					"room_id":        b.RoomID.String(),
					"check_in":       b.CheckIn,
					"check_out":      b.CheckOut,
					"status":         string(b.Status),
					"total_amount":   b.TotalAmount.String(),
					"currency":       b.Currency,
				},
			}); err != nil {
				return err
			}
		}

		created = b
		return nil
	})

	if txErr != nil {
		if hadExclusionFailure && s.holdStore != nil && conflictRedisKey != "" && conflictHoldToken != "" {
			_, _ = s.holdStore.ReleaseHoldKeys(context.Background(), []string{conflictRedisKey}, conflictHoldToken)
		}
		return domain.Booking{}, false, txErr
	}

	return created, replayed, nil
}

// GetBooking retrieves a single booking, enforcing staff `booking.read` or customer ownership.
func (s *Service) GetBooking(ctx context.Context, actor auth.Principal, bookingID uuid.UUID) (domain.Booking, error) {
	b, err := s.repo.GetBookingByID(ctx, bookingID)
	if err != nil {
		return domain.Booking{}, err
	}
	if actor.Kind == "staff" {
		if s.authz != nil {
			if err := s.authz.Authorize(ctx, actor, "booking.read"); err != nil {
				return domain.Booking{}, err
			}
		}
		return b, nil
	}
	cust, err := s.customers.GetCustomerByUserID(ctx, actor.UserID)
	if err != nil {
		return domain.Booking{}, err
	}
	if b.CustomerID != cust.ID {
		return domain.Booking{}, apperror.Forbidden("you do not have access to this booking")
	}
	return b, nil
}

// ListBookings lists bookings for staff (`booking.read`).
func (s *Service) ListBookings(ctx context.Context, actor auth.Principal, filter ListBookingsFilter) (platformhttp.PageResponse[domain.Booking], error) {
	if s.authz != nil {
		if err := s.authz.Authorize(ctx, actor, "booking.read"); err != nil {
			return platformhttp.PageResponse[domain.Booking]{}, err
		}
	}
	return s.listBookingsInternal(ctx, filter)
}

// ListMyBookings lists bookings owned by the authenticated customer.
func (s *Service) ListMyBookings(ctx context.Context, actor auth.Principal, filter ListBookingsFilter) (platformhttp.PageResponse[domain.Booking], error) {
	cust, err := s.customers.GetCustomerByUserID(ctx, actor.UserID)
	if err != nil {
		return platformhttp.PageResponse[domain.Booking]{}, err
	}
	filter.CustomerID = &cust.ID
	return s.listBookingsInternal(ctx, filter)
}

func (s *Service) listBookingsInternal(ctx context.Context, filter ListBookingsFilter) (platformhttp.PageResponse[domain.Booking], error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	items, nextCursor, err := s.repo.ListBookings(ctx, filter.CustomerID, filter.BookingGroupID, strings.TrimSpace(filter.Status), filter.Cursor, limit)
	if err != nil {
		return platformhttp.PageResponse[domain.Booking]{}, err
	}
	var curPtr *string
	if filter.Cursor != "" {
		c := filter.Cursor
		curPtr = &c
	}
	return platformhttp.PageResponse[domain.Booking]{
		Data: items,
		Page: platformhttp.PageInfo{
			Limit:      limit,
			Cursor:     curPtr,
			NextCursor: nextCursor,
		},
	}, nil
}

// ConfirmBooking manually confirms a pending booking (`booking.confirm`, for walk-in / front-desk staff).
func (s *Service) ConfirmBooking(ctx context.Context, actor auth.Principal, bookingID uuid.UUID, reason string) (domain.Booking, error) {
	if actor.Kind != "staff" {
		return domain.Booking{}, apperror.Forbidden("only staff with booking.confirm permission can confirm bookings")
	}
	if s.authz != nil {
		if err := s.authz.Authorize(ctx, actor, "booking.confirm"); err != nil {
			return domain.Booking{}, err
		}
	}
	var actorID *uuid.UUID
	if actor.UserID != uuid.Nil {
		uid := actor.UserID
		actorID = &uid
	}
	if strings.TrimSpace(reason) == "" {
		reason = "staff_confirmed"
	}
	confirmed, err := s.ConfirmPaidBookings(ctx, []uuid.UUID{bookingID}, actorID, reason)
	if err != nil {
		return domain.Booking{}, err
	}
	return confirmed[0], nil
}

// GetBookingByID implements BookingStatusService for internal cross-module callers.
func (s *Service) GetBookingByID(ctx context.Context, bookingID uuid.UUID) (domain.Booking, error) {
	return s.repo.GetBookingByID(ctx, bookingID)
}

// GetPayableTarget implements BookingStatusService for Phase 4 payment intents (single booking or group).
func (s *Service) GetPayableTarget(ctx context.Context, bookingID *uuid.UUID, bookingGroupID *uuid.UUID) (PayableTarget, error) {
	if bookingID != nil && *bookingID != uuid.Nil {
		b, err := s.repo.GetBookingByID(ctx, *bookingID)
		if err != nil {
			return PayableTarget{}, err
		}
		return PayableTarget{
			TargetType:    "booking",
			BookingID:     &b.ID,
			CustomerID:    b.CustomerID,
			TotalAmount:   b.TotalAmount,
			Currency:      b.Currency,
			ExpiresAt:     b.ExpiresAt,
			ChildBookings: []domain.Booking{b},
		}, nil
	}
	if bookingGroupID != nil && *bookingGroupID != uuid.Nil {
		children, err := s.repo.ListBookingsByGroupID(ctx, *bookingGroupID)
		if err != nil {
			return PayableTarget{}, err
		}
		if len(children) == 0 {
			return PayableTarget{}, apperror.NotFound("booking group not found")
		}
		total := decimal.Zero
		var expiresAt *time.Time
		for _, c := range children {
			if c.Status == domain.BookingStatusPending || c.Status == domain.BookingStatusConfirmed {
				total = total.Add(c.TotalAmount)
			}
			if c.ExpiresAt != nil && (expiresAt == nil || c.ExpiresAt.Before(*expiresAt)) {
				t := *c.ExpiresAt
				expiresAt = &t
			}
		}
		gid := *bookingGroupID
		return PayableTarget{
			TargetType:     "booking_group",
			BookingGroupID: &gid,
			CustomerID:     children[0].CustomerID,
			TotalAmount:    total,
			Currency:       children[0].Currency,
			ExpiresAt:      expiresAt,
			ChildBookings:  children,
		}, nil
	}
	return PayableTarget{}, apperror.Validation("either bookingId or bookingGroupId is required")
}

// ConfirmPaidBookings atomically locks the given booking IDs in sorted UUID order, transitions pending -> confirmed,
// clears expires_at, recomputes any affected booking_groups, records history/audit/outbox, and releases Redis hold keys post-commit.
func (s *Service) ConfirmPaidBookings(ctx context.Context, bookingIDs []uuid.UUID, actorUserID *uuid.UUID, reason string) ([]domain.Booking, error) {
	if len(bookingIDs) == 0 {
		return nil, apperror.Validation("at least one bookingId is required")
	}

	sortedIDs := dedupeAndSortUUIDs(bookingIDs)
	now := s.clk.Now()
	var (
		confirmedBookings []domain.Booking
		releaseBatches    []redisReleaseBatch
	)

	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		groupSet := make(map[uuid.UUID]struct{})
		fromPending := domain.BookingStatusPending

		for _, bid := range sortedIDs {
			before, err := s.repo.GetBookingByIDForUpdate(txCtx, bid)
			if err != nil {
				return err
			}
			if before.Status != domain.BookingStatusPending {
				return apperror.Conflict(fmt.Sprintf("booking %s cannot be confirmed from status %s", before.ReferenceCode, before.Status))
			}

			updated, err := s.repo.UpdateBookingStatusConfirmed(txCtx, bid, now)
			if err != nil {
				return err
			}

			if err := s.repo.InsertBookingStatusHistory(txCtx, domain.BookingStatusHistoryEntry{
				ID:            s.ids.NewUUID(),
				BookingID:     bid,
				FromStatus:    &fromPending,
				ToStatus:      domain.BookingStatusConfirmed,
				ActorUserID:   actorUserID,
				Reason:        reason,
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				OccurredAt:    now,
			}); err != nil {
				return err
			}

			if s.audit != nil {
				idCopy := bid
				if err := s.audit.Record(txCtx, auditapp.AuditEntry{
					ActorUserID:   actorUserID,
					Action:        "booking.confirmed",
					ResourceType:  "booking",
					ResourceID:    &idCopy,
					RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
					CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
					BeforeData:    map[string]any{"status": string(before.Status)},
					AfterData:     map[string]any{"status": string(updated.Status), "reason": reason},
				}); err != nil {
					return err
				}
			}

			if s.outbox != nil {
				payload, _ := json.Marshal(map[string]any{
					"bookingId":     updated.ID.String(),
					"referenceCode": updated.ReferenceCode,
					"customerId":    updated.CustomerID.String(),
					"checkIn":       updated.CheckIn,
					"checkOut":      updated.CheckOut,
					"totalAmount":   updated.TotalAmount.String(),
					"currency":      updated.Currency,
				})
				if err := s.outbox.Enqueue(txCtx, auditapp.OutboxEvent{
					AggregateType: "booking",
					AggregateID:   updated.ID,
					EventType:     EventBookingConfirmed,
					Payload:       payload,
				}); err != nil {
					return err
				}
			}

			// Collect converted hold keys to release from Redis after commit.
			if holds, err := s.repo.FindConvertedHoldsForRoomStay(txCtx, updated.RoomID, updated.CheckInTime, updated.CheckOutTime); err == nil {
				for _, h := range holds {
					keys := []string{h.RedisKey}
					if h.BookingGroupID != nil {
						keys = append(keys, domain.GroupHoldRedisKey(*h.BookingGroupID))
					}
					releaseBatches = append(releaseBatches, redisReleaseBatch{keys: keys, token: h.HoldToken})
				}
			}

			if updated.BookingGroupID != nil {
				groupSet[*updated.BookingGroupID] = struct{}{}
			}

			full, err := s.repo.GetBookingByID(txCtx, bid)
			if err != nil {
				return err
			}
			confirmedBookings = append(confirmedBookings, full)
		}

		groupIDs := make([]uuid.UUID, 0, len(groupSet))
		for gid := range groupSet {
			groupIDs = append(groupIDs, gid)
		}
		sort.Slice(groupIDs, func(i, j int) bool { return groupIDs[i].String() < groupIDs[j].String() })
		for _, gid := range groupIDs {
			if _, err := s.repo.RecomputeBookingGroupStatus(txCtx, gid, now); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Post-commit best-effort Redis hold key release.
	if s.holdStore != nil {
		for _, b := range releaseBatches {
			if _, relErr := s.holdStore.ReleaseHoldKeys(ctx, b.keys, b.token); relErr != nil {
				s.logger.Warn("post-confirm redis hold release failed", slog.Any("error", relErr))
			}
		}
	}

	return confirmedBookings, nil
}

// MarkCheckedIn transitions a confirmed booking to checked_in (consumed by Phase 5 stayops).
func (s *Service) MarkCheckedIn(ctx context.Context, bookingID uuid.UUID, actorUserID *uuid.UUID, reason string) (domain.Booking, error) {
	now := s.clk.Now()
	var result domain.Booking

	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		before, err := s.repo.GetBookingByIDForUpdate(txCtx, bookingID)
		if err != nil {
			return err
		}
		if before.Status != domain.BookingStatusConfirmed {
			return apperror.Conflict(fmt.Sprintf("booking %s must be confirmed before check-in (current: %s)", before.ReferenceCode, before.Status))
		}
		updated, err := s.repo.UpdateBookingStatusCheckedIn(txCtx, bookingID, now)
		if err != nil {
			return err
		}
		fromConfirmed := domain.BookingStatusConfirmed
		if err := s.repo.InsertBookingStatusHistory(txCtx, domain.BookingStatusHistoryEntry{
			ID:            s.ids.NewUUID(),
			BookingID:     bookingID,
			FromStatus:    &fromConfirmed,
			ToStatus:      domain.BookingStatusCheckedIn,
			ActorUserID:   actorUserID,
			Reason:        reason,
			CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
			OccurredAt:    now,
		}); err != nil {
			return err
		}
		if updated.BookingGroupID != nil {
			if _, err := s.repo.RecomputeBookingGroupStatus(txCtx, *updated.BookingGroupID, now); err != nil {
				return err
			}
		}
		result, err = s.repo.GetBookingByID(txCtx, bookingID)
		return err
	})
	return result, err
}

// MarkCheckedOut transitions a checked_in booking to checked_out (consumed by Phase 5 stayops).
func (s *Service) MarkCheckedOut(ctx context.Context, bookingID uuid.UUID, actorUserID *uuid.UUID, reason string) (domain.Booking, error) {
	now := s.clk.Now()
	var result domain.Booking

	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		before, err := s.repo.GetBookingByIDForUpdate(txCtx, bookingID)
		if err != nil {
			return err
		}
		if before.Status != domain.BookingStatusCheckedIn {
			return apperror.Conflict(fmt.Sprintf("booking %s must be checked_in before check-out (current: %s)", before.ReferenceCode, before.Status))
		}
		updated, err := s.repo.UpdateBookingStatusCheckedOut(txCtx, bookingID, now)
		if err != nil {
			return err
		}
		fromCheckedIn := domain.BookingStatusCheckedIn
		if err := s.repo.InsertBookingStatusHistory(txCtx, domain.BookingStatusHistoryEntry{
			ID:            s.ids.NewUUID(),
			BookingID:     bookingID,
			FromStatus:    &fromCheckedIn,
			ToStatus:      domain.BookingStatusCheckedOut,
			ActorUserID:   actorUserID,
			Reason:        reason,
			CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
			OccurredAt:    now,
		}); err != nil {
			return err
		}
		if updated.BookingGroupID != nil {
			if _, err := s.repo.RecomputeBookingGroupStatus(txCtx, *updated.BookingGroupID, now); err != nil {
				return err
			}
		}
		result, err = s.repo.GetBookingByID(txCtx, bookingID)
		return err
	})
	return result, err
}

func dedupeAndSortUUIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].String() < out[j].String()
	})
	return out
}
