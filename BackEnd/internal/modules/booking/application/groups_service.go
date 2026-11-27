package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/booking/domain"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/db"
	platformhttp "github.com/trandinh0506/hotel-management/backend/internal/platform/http"
)

// RoomGuestInput specifies optional per-room guest overrides when converting a group hold.
type RoomGuestInput struct {
	HoldID              *uuid.UUID           `json:"holdId,omitempty"`
	RoomID              *uuid.UUID           `json:"roomId,omitempty"`
	Adults              int                  `json:"adults,omitempty"`
	Children            int                  `json:"children,omitempty"`
	SpecialRequest      string               `json:"specialRequest,omitempty"`
	RepresentativeGuest domain.GuestSnapshot `json:"representativeGuest"`
}

// CreateBookingGroupParams defines inputs for POST /api/v1/booking-groups.
type CreateBookingGroupParams struct {
	IdempotencyKey      string
	CustomerID          *uuid.UUID
	GroupHoldID         uuid.UUID
	HoldToken           string
	Adults              int
	Children            int
	SpecialRequest      string
	RepresentativeGuest *domain.GuestSnapshot
	RoomGuests          []RoomGuestInput
}

// CreateBookingGroup atomically converts an active group hold into 1 booking_groups aggregate
// and N child bookings + N guest snapshots in a single PostgreSQL transaction.
// Any room conflict rolls back the entire transaction (0 booking_groups, 0 bookings persisted).
// Returns (group, replayed, err).
func (s *Service) CreateBookingGroup(ctx context.Context, actor auth.Principal, params CreateBookingGroupParams) (domain.BookingGroup, bool, error) {
	idempKey := strings.TrimSpace(params.IdempotencyKey)
	if idempKey == "" {
		return domain.BookingGroup{}, false, apperror.Validation("Idempotency-Key header is required", apperror.FieldError{
			Field:   "Idempotency-Key",
			Message: "Idempotency-Key header is required",
		})
	}
	if len(idempKey) > 128 {
		return domain.BookingGroup{}, false, apperror.Validation("Idempotency-Key is too long", apperror.FieldError{
			Field:   "Idempotency-Key",
			Message: "must be at most 128 characters",
		})
	}
	if params.GroupHoldID == uuid.Nil {
		return domain.BookingGroup{}, false, apperror.Validation("groupHoldId is required", apperror.FieldError{
			Field:   "groupHoldId",
			Message: "must be a valid UUID",
		})
	}
	holdToken := strings.TrimSpace(params.HoldToken)
	if holdToken == "" {
		return domain.BookingGroup{}, false, apperror.Validation("holdToken is required", apperror.FieldError{
			Field:   "holdToken",
			Message: "holdToken is required",
		})
	}

	customerID, err := s.resolveBookingCustomer(ctx, actor, params.CustomerID, "booking.create")
	if err != nil {
		return domain.BookingGroup{}, false, err
	}

	// Fast idempotency check before touching holds.
	if existing, err := s.repo.FindBookingGroupByCustomerIdempotencyKey(ctx, customerID, idempKey); err != nil {
		return domain.BookingGroup{}, false, err
	} else if existing != nil {
		children, err := s.repo.ListBookingsByGroupID(ctx, existing.ID)
		if err != nil {
			return domain.BookingGroup{}, false, err
		}
		existing.Bookings = children
		existing.PopulateTotals()
		return *existing, true, nil
	}

	// Validate common representative guest if provided.
	var commonGuest *domain.GuestSnapshot
	if params.RepresentativeGuest != nil {
		g := *params.RepresentativeGuest
		if err := g.NormalizeAndValidate("representativeGuest"); err != nil {
			return domain.BookingGroup{}, false, err
		}
		commonGuest = &g
	}

	// Validate per-room guest overrides if provided.
	guestByHoldID := make(map[uuid.UUID]RoomGuestInput, len(params.RoomGuests))
	guestByRoomID := make(map[uuid.UUID]RoomGuestInput, len(params.RoomGuests))
	for i := range params.RoomGuests {
		rg := params.RoomGuests[i]
		if err := rg.RepresentativeGuest.NormalizeAndValidate(fmt.Sprintf("roomGuests[%d].representativeGuest", i)); err != nil {
			return domain.BookingGroup{}, false, err
		}
		if rg.HoldID != nil && *rg.HoldID != uuid.Nil {
			guestByHoldID[*rg.HoldID] = rg
		}
		if rg.RoomID != nil && *rg.RoomID != uuid.Nil {
			guestByRoomID[*rg.RoomID] = rg
		}
	}
	if commonGuest == nil && len(params.RoomGuests) == 0 {
		return domain.BookingGroup{}, false, apperror.Validation("representative guest is required", apperror.FieldError{
			Field:   "representativeGuest",
			Message: "provide representativeGuest or roomGuests",
		})
	}

	now := s.clk.Now()
	var (
		created             domain.BookingGroup
		replayed            bool
		hadExclusionFailure bool
		conflictRedisKeys   []string
	)

	txErr := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		// Re-check idempotency inside transaction.
		if dup, err := s.repo.FindBookingGroupByCustomerIdempotencyKey(txCtx, customerID, idempKey); err != nil {
			return err
		} else if dup != nil {
			children, err := s.repo.ListBookingsByGroupID(txCtx, dup.ID)
			if err != nil {
				return err
			}
			dup.Bookings = children
			dup.PopulateTotals()
			created = *dup
			replayed = true
			return nil
		}

		// Lock all child reservation holds in sorted UUID order (ORDER BY id ASC FOR UPDATE).
		holds, err := s.repo.ListReservationHoldsByGroupIDForUpdate(txCtx, params.GroupHoldID)
		if err != nil {
			return err
		}
		if len(holds) == 0 {
			return apperror.NotFound("group reservation hold not found")
		}

		conflictRedisKeys = make([]string, 0, len(holds)+1)
		conflictRedisKeys = append(conflictRedisKeys, domain.GroupHoldRedisKey(params.GroupHoldID))

		holdIDs := make([]uuid.UUID, 0, len(holds))
		for _, h := range holds {
			if h.CustomerID != customerID {
				return apperror.Forbidden("you do not own this group hold")
			}
			if h.HoldMode != domain.HoldModeGroup {
				return apperror.Conflict("hold is not a group hold")
			}
			if h.HoldToken != holdToken {
				return apperror.Validation("invalid holdToken", apperror.FieldError{
					Field:   "holdToken",
					Message: "holdToken does not match",
				})
			}
			if h.Status != domain.HoldStatusActive || !h.ExpiresAt.After(now) {
				return apperror.Conflict("group reservation hold is expired or no longer active")
			}
			holdIDs = append(holdIDs, h.ID)
			conflictRedisKeys = append(conflictRedisKeys, h.RedisKey)
		}

		groupExpiresAt := holds[0].ExpiresAt
		groupID := params.GroupHoldID

		grp, err := s.repo.InsertBookingGroup(txCtx, domain.BookingGroup{
			ID:             groupID,
			ReferenceCode:  domain.GenerateReferenceCode("GRP", now),
			CustomerID:     customerID,
			Status:         domain.GroupStatusPending,
			ExpiresAt:      &groupExpiresAt,
			IdempotencyKey: idempKey,
			CreatedAt:      now,
		})
		if err != nil {
			return err
		}

		var actorID *uuid.UUID
		if actor.UserID != uuid.Nil {
			uid := actor.UserID
			actorID = &uid
		}

		children := make([]domain.Booking, 0, len(holds))
		for idx, h := range holds {
			rt, err := s.catalog.GetRoomTypeByID(txCtx, h.RoomTypeID)
			if err != nil {
				return err
			}

			// Resolve guest & occupancy for this room.
			var (
				guest      domain.GuestSnapshot
				adults     int
				childrenCt int
				specReq    string
			)
			if rg, ok := guestByHoldID[h.ID]; ok {
				guest = rg.RepresentativeGuest
				adults = rg.Adults
				childrenCt = rg.Children
				specReq = rg.SpecialRequest
			} else if rg, ok := guestByRoomID[h.RoomID]; ok {
				guest = rg.RepresentativeGuest
				adults = rg.Adults
				childrenCt = rg.Children
				specReq = rg.SpecialRequest
			} else if idx < len(params.RoomGuests) && params.RoomGuests[idx].HoldID == nil && params.RoomGuests[idx].RoomID == nil {
				rg := params.RoomGuests[idx]
				guest = rg.RepresentativeGuest
				adults = rg.Adults
				childrenCt = rg.Children
				specReq = rg.SpecialRequest
			} else if commonGuest != nil {
				guest = *commonGuest
				specReq = params.SpecialRequest
			} else {
				return apperror.Validation("missing guest information for room in group", apperror.FieldError{
					Field:   fmt.Sprintf("roomGuests[%d]", idx),
					Message: "provide representativeGuest or matching roomGuests entry for every room",
				})
			}

			if adults <= 0 {
				if params.Adults > 0 && params.Adults <= rt.Capacity {
					adults = params.Adults
				} else {
					adults = 1
				}
			}
			if childrenCt < 0 {
				childrenCt = 0
			}
			if adults+childrenCt > rt.Capacity {
				return apperror.Validation("guest count exceeds room capacity", apperror.FieldError{
					Field:   "adults",
					Message: fmt.Sprintf("room guests (%d) exceed capacity (%d)", adults+childrenCt, rt.Capacity),
				})
			}

			stay := domain.StayRange{CheckIn: h.CheckIn, CheckOut: h.CheckOut}
			subtotal := rt.BaseNightlyRate.Mul(decimal.NewFromInt(int64(stay.Nights())))
			snap, err := s.pricing.Calculate(txCtx, subtotal)
			if err != nil {
				return err
			}
			if snap.VatRateID == nil || *snap.VatRateID == uuid.Nil {
				return apperror.Conflict("no active VAT rate configured")
			}

			childID := s.ids.NewUUID()
			b, err := s.repo.InsertBooking(txCtx, domain.Booking{
				ID:                  childID,
				ReferenceCode:       domain.GenerateReferenceCode("BK", now),
				BookingGroupID:      &groupID,
				CustomerID:          customerID,
				RoomID:              h.RoomID,
				RoomTypeID:          h.RoomTypeID,
				CheckInTime:         h.CheckIn,
				CheckOutTime:        h.CheckOut,
				Adults:              adults,
				Children:            childrenCt,
				Status:              domain.BookingStatusPending,
				NightlyRateSnapshot: rt.BaseNightlyRate,
				VatRateID:           *snap.VatRateID,
				VatRateSnapshot:     snap.VatRate,
				SubtotalAmount:      snap.Subtotal,
				VatAmount:           snap.VatAmount,
				TotalAmount:         snap.TotalAmount,
				Currency:            snap.Currency,
				SpecialRequest:      strings.TrimSpace(specReq),
				ExpiresAt:           &groupExpiresAt,
				CreatedAt:           now,
			})
			if err != nil {
				if db.IsExclusionViolation(err) {
					hadExclusionFailure = true
				}
				return err
			}

			guest.CreatedAt = now
			savedGuest, err := s.repo.InsertBookingGuestSnapshot(txCtx, childID, guest)
			if err != nil {
				return err
			}
			b.RepresentativeGuest = &savedGuest

			if err := s.repo.InsertBookingStatusHistory(txCtx, domain.BookingStatusHistoryEntry{
				ID:            s.ids.NewUUID(),
				BookingID:     childID,
				FromStatus:    nil,
				ToStatus:      domain.BookingStatusPending,
				ActorUserID:   actorID,
				Reason:        "group_booking_created",
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				OccurredAt:    now,
			}); err != nil {
				return err
			}

			children = append(children, b)
		}

		if err := s.repo.UpdateReservationHoldsStatus(txCtx, holdIDs, domain.HoldStatusConverted, "converted_to_group_booking", now); err != nil {
			return err
		}

		grp.Bookings = children
		grp.PopulateTotals()

		if s.audit != nil {
			if err := s.audit.Record(txCtx, auditapp.AuditEntry{
				ActorUserID:   actorID,
				Action:        "booking.group_created",
				ResourceType:  "booking_group",
				ResourceID:    &grp.ID,
				RequestID:     platformhttp.RequestIDUUIDFromContext(txCtx),
				CorrelationID: platformhttp.CorrelationIDUUIDFromContext(txCtx),
				AfterData: map[string]any{
					"reference_code": grp.ReferenceCode,
					"customer_id":    grp.CustomerID.String(),
					"room_count":     len(children),
					"status":         string(grp.Status),
					"total_amount":   grp.TotalAmount.String(),
					"currency":       grp.Currency,
				},
			}); err != nil {
				return err
			}
		}

		created = grp
		return nil
	})

	if txErr != nil {
		if hadExclusionFailure && s.holdStore != nil && len(conflictRedisKeys) > 0 {
			_, _ = s.holdStore.ReleaseHoldKeys(context.Background(), conflictRedisKeys, holdToken)
		}
		return domain.BookingGroup{}, false, txErr
	}

	return created, replayed, nil
}

// GetBookingGroup returns a booking group aggregate with all child bookings and totals.
// Enforces staff `booking.read` or customer ownership.
func (s *Service) GetBookingGroup(ctx context.Context, actor auth.Principal, groupID uuid.UUID) (domain.BookingGroup, error) {
	grp, err := s.repo.GetBookingGroupByID(ctx, groupID)
	if err != nil {
		return domain.BookingGroup{}, err
	}
	if actor.Kind == "staff" {
		if s.authz != nil {
			if err := s.authz.Authorize(ctx, actor, "booking.read"); err != nil {
				return domain.BookingGroup{}, err
			}
		}
	} else {
		cust, err := s.customers.GetCustomerByUserID(ctx, actor.UserID)
		if err != nil {
			return domain.BookingGroup{}, err
		}
		if grp.CustomerID != cust.ID {
			return domain.BookingGroup{}, apperror.Forbidden("you do not have access to this booking group")
		}
	}

	children, err := s.repo.ListBookingsByGroupID(ctx, grp.ID)
	if err != nil {
		return domain.BookingGroup{}, err
	}
	grp.Bookings = children
	grp.PopulateTotals()
	return grp, nil
}

// ListMyBookingGroups lists booking groups owned by the authenticated customer.
func (s *Service) ListMyBookingGroups(ctx context.Context, actor auth.Principal, cursor string, limit int) (platformhttp.PageResponse[domain.BookingGroup], error) {
	cust, err := s.customers.GetCustomerByUserID(ctx, actor.UserID)
	if err != nil {
		return platformhttp.PageResponse[domain.BookingGroup]{}, err
	}
	if limit <= 0 {
		limit = 20
	}
	groups, nextCursor, err := s.repo.ListBookingGroupsByCustomer(ctx, cust.ID, cursor, limit)
	if err != nil {
		return platformhttp.PageResponse[domain.BookingGroup]{}, err
	}
	for i := range groups {
		children, err := s.repo.ListBookingsByGroupID(ctx, groups[i].ID)
		if err != nil {
			return platformhttp.PageResponse[domain.BookingGroup]{}, err
		}
		groups[i].Bookings = children
		groups[i].PopulateTotals()
	}

	var curPtr *string
	if cursor != "" {
		c := cursor
		curPtr = &c
	}
	return platformhttp.PageResponse[domain.BookingGroup]{
		Data: groups,
		Page: platformhttp.PageInfo{
			Limit:      limit,
			Cursor:     curPtr,
			NextCursor: nextCursor,
		},
	}, nil
}
