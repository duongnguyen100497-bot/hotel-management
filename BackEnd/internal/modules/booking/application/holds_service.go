package application

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/booking/domain"
	propertyapp "github.com/trandinh0506/hotel-management/backend/internal/modules/property/application"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/auth"
)

const maxHoldAcquireRetries = 3

var errRedisKeyConflict = errors.New("redis hold key conflict")

// CreateHoldParams defines inputs for POST /api/v1/holds.
type CreateHoldParams struct {
	CustomerID *uuid.UUID
	CheckIn    string
	CheckOut   string
	Items      []domain.QuoteItemRequest
	Adults     int
	Children   int
}

// ReleaseHoldParams defines inputs for DELETE /api/v1/holds/{id}.
type ReleaseHoldParams struct {
	ID        uuid.UUID
	HoldToken string
}

// CreateHold atomically reserves one room (individual hold) or multiple rooms (group hold)
// across PostgreSQL and Redis using dynamic TTLs from hotel_settings.
// Invariant: if Redis is unavailable, CreateHold fails closed with 503 Service Unavailable.
func (s *Service) CreateHold(ctx context.Context, actor auth.Principal, params CreateHoldParams) (domain.HoldResult, error) {
	if s.holdStore == nil {
		return domain.HoldResult{}, apperror.ServiceUnavailable("hold store is unavailable")
	}

	customerID, err := s.resolveBookingCustomer(ctx, actor, params.CustomerID, "booking.create")
	if err != nil {
		return domain.HoldResult{}, err
	}

	// Validate dates, room types, capacity, and compute authoritative quote first.
	quote, err := s.QuoteAvailability(ctx, QuoteAvailabilityParams{
		CheckIn:  params.CheckIn,
		CheckOut: params.CheckOut,
		Items:    params.Items,
		Adults:   params.Adults,
		Children: params.Children,
	})
	if err != nil {
		return domain.HoldResult{}, err
	}
	if !quote.Available {
		return domain.HoldResult{}, apperror.Conflict("insufficient available rooms for the selected dates")
	}

	cfg, err := s.settings.GetSettings(ctx)
	if err != nil {
		return domain.HoldResult{}, err
	}
	loc, locErr := cfg.Location()
	if locErr != nil {
		loc = time.UTC
	}
	now := s.clk.Now()
	stay, err := domain.ParseStayRange(params.CheckIn, params.CheckOut, now, loc)
	if err != nil {
		return domain.HoldResult{}, err
	}

	totalRooms := 0
	for _, item := range params.Items {
		totalRooms += item.RoomCount
	}
	holdMode := domain.HoldModeIndividual
	ttlDuration := cfg.IndividualHoldTTL()
	var groupHoldID *uuid.UUID
	if totalRooms > 1 {
		holdMode = domain.HoldModeGroup
		ttlDuration = cfg.GroupHoldTTL()
		gid := s.ids.NewUUID()
		groupHoldID = &gid
	}
	ttlSeconds := int(ttlDuration.Seconds())
	if ttlSeconds < 1 {
		ttlSeconds = 60
	}

	// Pre-fetch candidate bookable rooms per room type sorted by UUID ascending for deterministic locking.
	type candidateType struct {
		roomType propertyapp.RoomType
		count    int
		rooms    []propertyapp.Room
	}
	candidates := make([]candidateType, 0, len(params.Items))
	for _, item := range params.Items {
		rt, err := s.catalog.GetRoomTypeByID(ctx, item.RoomTypeID)
		if err != nil {
			return domain.HoldResult{}, err
		}
		rooms, err := s.catalog.ListBookableRoomsByType(ctx, item.RoomTypeID)
		if err != nil {
			return domain.HoldResult{}, err
		}
		sort.Slice(rooms, func(i, j int) bool {
			return rooms[i].ID.String() < rooms[j].ID.String()
		})
		candidates = append(candidates, candidateType{
			roomType: rt,
			count:    item.RoomCount,
			rooms:    rooms,
		})
	}

	excludedRoomIDs := make(map[uuid.UUID]struct{})

	for attempt := 0; attempt < maxHoldAcquireRetries; attempt++ {
		holdToken, err := domain.GenerateHoldToken()
		if err != nil {
			return domain.HoldResult{}, apperror.Internal(err)
		}
		attemptNow := s.clk.Now()
		expiresAt := attemptNow.Add(time.Duration(ttlSeconds) * time.Second)

		var (
			createdHolds []domain.ReservationHold
			summaries    []domain.HoldRoomSummary
			redisKeys    []string
			chosenRooms  []uuid.UUID
			redisLocked  bool
		)

		txErr := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
			selectedByType := make([][]propertyapp.Room, len(candidates))
			for idx, cand := range candidates {
				allIDs := make([]uuid.UUID, 0, len(cand.rooms))
				for _, rm := range cand.rooms {
					if _, skip := excludedRoomIDs[rm.ID]; skip {
						continue
					}
					allIDs = append(allIDs, rm.ID)
				}
				unavailable, err := s.repo.ListUnavailableRoomIDs(txCtx, allIDs, stay.CheckIn, stay.CheckOut, attemptNow)
				if err != nil {
					return err
				}

				picked := make([]propertyapp.Room, 0, cand.count)
				for _, rm := range cand.rooms {
					if _, skip := excludedRoomIDs[rm.ID]; skip {
						continue
					}
					if _, busy := unavailable[rm.ID]; busy {
						continue
					}
					// Non-blocking transaction-scoped advisory lock (SKIP LOCKED semantics without touching property tables).
					locked, err := s.repo.TryAdvisoryXactLockRoom(txCtx, rm.ID)
					if err != nil {
						return err
					}
					if !locked {
						continue
					}
					// Re-check DB availability for this room under the advisory lock.
					recheck, err := s.repo.ListUnavailableRoomIDs(txCtx, []uuid.UUID{rm.ID}, stay.CheckIn, stay.CheckOut, attemptNow)
					if err != nil {
						return err
					}
					if _, busy := recheck[rm.ID]; busy {
						continue
					}
					picked = append(picked, rm)
					if len(picked) == cand.count {
						break
					}
				}
				if len(picked) < cand.count {
					return apperror.Conflict("insufficient available rooms for the selected dates")
				}
				selectedByType[idx] = picked
			}

			redisKeys = make([]string, 0, totalRooms+1)
			createdHolds = make([]domain.ReservationHold, 0, totalRooms)
			summaries = make([]domain.HoldRoomSummary, 0, totalRooms)
			chosenRooms = make([]uuid.UUID, 0, totalRooms)

			if groupHoldID != nil {
				redisKeys = append(redisKeys, domain.GroupHoldRedisKey(*groupHoldID))
			}

			for idx, cand := range candidates {
				for _, rm := range selectedByType[idx] {
					rKey := domain.RoomHoldRedisKey(rm.ID, stay.CheckIn, stay.CheckOut)
					redisKeys = append(redisKeys, rKey)
					chosenRooms = append(chosenRooms, rm.ID)

					h, err := s.repo.InsertReservationHold(txCtx, domain.ReservationHold{
						ID:             s.ids.NewUUID(),
						CustomerID:     customerID,
						BookingGroupID: groupHoldID,
						RoomID:         rm.ID,
						RoomTypeID:     cand.roomType.ID,
						CheckIn:        stay.CheckIn,
						CheckOut:       stay.CheckOut,
						HoldMode:       holdMode,
						HoldToken:      holdToken,
						RedisKey:       rKey,
						TTLSeconds:     ttlSeconds,
						Status:         domain.HoldStatusActive,
						ExpiresAt:      expiresAt,
						CreatedAt:      attemptNow,
					})
					if err != nil {
						return err
					}
					createdHolds = append(createdHolds, h)
					summaries = append(summaries, domain.HoldRoomSummary{
						ID:           h.ID,
						RoomID:       rm.ID,
						RoomCode:     rm.Code,
						RoomTypeID:   cand.roomType.ID,
						RoomTypeCode: cand.roomType.Code,
						RedisKey:     rKey,
					})
				}
			}

			// Acquire all Redis keys atomically via Lua script before committing DB transaction.
			// Fail-closed: any Redis error rolls back the DB transaction and returns 503.
			acquired, err := s.holdStore.AcquireHoldKeys(txCtx, redisKeys, holdToken, ttlSeconds)
			if err != nil {
				return apperror.ServiceUnavailable("hold service temporarily unavailable").WithCause(err)
			}
			if !acquired {
				return errRedisKeyConflict
			}
			redisLocked = true
			return nil
		})

		if txErr == nil {
			primaryHoldID := createdHolds[0].ID
			if groupHoldID != nil {
				primaryHoldID = *groupHoldID
			}
			return domain.HoldResult{
				HoldID:      primaryHoldID,
				GroupHoldID: groupHoldID,
				HoldMode:    holdMode,
				HoldToken:   holdToken,
				TTLSeconds:  ttlSeconds,
				ExpiresAt:   expiresAt,
				CheckIn:     stay.CheckInDate(),
				CheckOut:    stay.CheckOutDate(),
				Nights:      stay.Nights(),
				Holds:       summaries,
				Quote:       quote,
			}, nil
		}

		// If Redis keys were acquired inside the tx but DB commit itself failed, release Redis keys best-effort.
		if redisLocked {
			_, _ = s.holdStore.ReleaseHoldKeys(context.Background(), redisKeys, holdToken)
		}

		if errors.Is(txErr, errRedisKeyConflict) {
			for _, rid := range chosenRooms {
				excludedRoomIDs[rid] = struct{}{}
			}
			continue
		}
		return domain.HoldResult{}, txErr
	}

	return domain.HoldResult{}, apperror.Conflict("selected rooms are currently held by another guest")
}

// ReleaseHold manually releases an active individual hold or group hold.
// Updates PostgreSQL first, then deletes matching Redis keys via Lua script post-commit.
func (s *Service) ReleaseHold(ctx context.Context, actor auth.Principal, params ReleaseHoldParams) error {
	if params.ID == uuid.Nil {
		return apperror.Validation("hold id is required", apperror.FieldError{
			Field:   "id",
			Message: "must be a valid UUID",
		})
	}
	token := strings.TrimSpace(params.HoldToken)
	if token == "" {
		return apperror.Validation("holdToken is required", apperror.FieldError{
			Field:   "holdToken",
			Message: "provide holdToken via X-Hold-Token header, holdToken query param, or JSON body",
		})
	}

	isStaff := actor.Kind == "staff"
	var actorCustomerID uuid.UUID
	if isStaff {
		if s.authz != nil {
			if err := s.authz.Authorize(ctx, actor, "booking.cancel"); err != nil {
				if err2 := s.authz.Authorize(ctx, actor, "booking.create"); err2 != nil {
					return err
				}
			}
		}
	} else {
		cust, err := s.customers.GetCustomerByUserID(ctx, actor.UserID)
		if err != nil {
			return err
		}
		actorCustomerID = cust.ID
	}

	var (
		redisKeys []string
		holdToken string
	)

	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		var holds []domain.ReservationHold
		// Try group hold first; if empty, look up single hold by ID.
		groupHolds, err := s.repo.ListReservationHoldsByGroupIDForUpdate(txCtx, params.ID)
		if err != nil {
			return err
		}
		if len(groupHolds) > 0 {
			holds = groupHolds
		} else {
			single, err := s.repo.GetReservationHoldByIDForUpdate(txCtx, params.ID)
			if err != nil {
				return err
			}
			holds = []domain.ReservationHold{single}
		}

		holdIDs := make([]uuid.UUID, 0, len(holds))
		redisKeys = make([]string, 0, len(holds)+1)
		if holds[0].BookingGroupID != nil {
			redisKeys = append(redisKeys, domain.GroupHoldRedisKey(*holds[0].BookingGroupID))
		}
		for _, h := range holds {
			if !isStaff && h.CustomerID != actorCustomerID {
				return apperror.Forbidden("you do not own this hold")
			}
			if h.HoldToken != token {
				return apperror.Validation("invalid holdToken", apperror.FieldError{
					Field:   "holdToken",
					Message: "holdToken does not match",
				})
			}
			if h.Status != domain.HoldStatusActive {
				return apperror.Conflict("hold is no longer active")
			}
			holdIDs = append(holdIDs, h.ID)
			redisKeys = append(redisKeys, h.RedisKey)
			holdToken = h.HoldToken
		}

		return s.repo.UpdateReservationHoldsStatus(txCtx, holdIDs, domain.HoldStatusReleased, "user_released", s.clk.Now())
	})
	if err != nil {
		return err
	}

	if s.holdStore != nil && len(redisKeys) > 0 {
		if _, relErr := s.holdStore.ReleaseHoldKeys(ctx, redisKeys, holdToken); relErr != nil {
			s.logger.Warn("post-commit redis hold release failed; reconciler will clean up",
				slog.String("hold_id", params.ID.String()),
				slog.Any("error", relErr),
			)
		}
	}
	return nil
}

func (s *Service) resolveBookingCustomer(ctx context.Context, actor auth.Principal, requestedCustomerID *uuid.UUID, staffPerm string) (uuid.UUID, error) {
	if actor.Kind == "staff" {
		if s.authz != nil {
			if err := s.authz.Authorize(ctx, actor, staffPerm); err != nil {
				return uuid.Nil, err
			}
		}
		if requestedCustomerID == nil || *requestedCustomerID == uuid.Nil {
			return uuid.Nil, apperror.Validation("customerId is required for staff requests", apperror.FieldError{
				Field:   "customerId",
				Message: "customerId is required",
			})
		}
		cust, err := s.customers.GetCustomerByID(ctx, *requestedCustomerID)
		if err != nil {
			return uuid.Nil, err
		}
		if cust.ArchivedAt != nil {
			return uuid.Nil, apperror.Conflict("customer is archived")
		}
		return cust.ID, nil
	}

	if actor.UserID == uuid.Nil {
		return uuid.Nil, apperror.Unauthenticated("authentication required")
	}
	cust, err := s.customers.GetCustomerByUserID(ctx, actor.UserID)
	if err != nil {
		return uuid.Nil, err
	}
	if requestedCustomerID != nil && *requestedCustomerID != uuid.Nil && *requestedCustomerID != cust.ID {
		return uuid.Nil, apperror.Forbidden("cannot act on behalf of another customer")
	}
	return cust.ID, nil
}
