package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/booking/domain"
	peopleapp "github.com/trandinh0506/hotel-management/backend/internal/modules/people/application"
	pricingapp "github.com/trandinh0506/hotel-management/backend/internal/modules/pricing/application"
	propertyapp "github.com/trandinh0506/hotel-management/backend/internal/modules/property/application"
	rbacapp "github.com/trandinh0506/hotel-management/backend/internal/modules/rbac/application"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/clock"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/db"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/id"
)

// Exported application-level aliases for cross-module consumers.
type (
	BookingStatus         = domain.BookingStatus
	GroupStatus           = domain.GroupStatus
	HoldMode              = domain.HoldMode
	HoldStatus            = domain.HoldStatus
	StayRange             = domain.StayRange
	QuoteItemRequest      = domain.QuoteItemRequest
	AvailabilityQuoteItem = domain.AvailabilityQuoteItem
	AvailabilityQuote     = domain.AvailabilityQuote
	ReservationHold       = domain.ReservationHold
	HoldRoomSummary       = domain.HoldRoomSummary
	HoldResult            = domain.HoldResult
)

// HoldStore defines the Redis Lua persistence contract for atomic room/group holds.
type HoldStore interface {
	AcquireHoldKeys(ctx context.Context, keys []string, token string, ttlSeconds int) (bool, error)
	ReleaseHoldKeys(ctx context.Context, keys []string, token string) (int64, error)
	ScanHoldKeys(ctx context.Context) (map[string]string, error)
}

// Repository defines the PostgreSQL persistence contract for the booking module.
type Repository interface {
	ListUnavailableRoomIDs(ctx context.Context, roomIDs []uuid.UUID, checkIn, checkOut time.Time, now time.Time) (map[uuid.UUID]struct{}, error)
	HasActiveBookingForCustomer(ctx context.Context, customerID uuid.UUID) (bool, error)
	TryAdvisoryXactLockRoom(ctx context.Context, roomID uuid.UUID) (bool, error)
	InsertReservationHold(ctx context.Context, h domain.ReservationHold) (domain.ReservationHold, error)
	GetReservationHoldByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.ReservationHold, error)
	ListReservationHoldsByGroupIDForUpdate(ctx context.Context, groupID uuid.UUID) ([]domain.ReservationHold, error)
	UpdateReservationHoldsStatus(ctx context.Context, holdIDs []uuid.UUID, status domain.HoldStatus, reason string, now time.Time) error
}

// QuoteAvailabilityParams defines inputs for POST /api/v1/availability/quote.
type QuoteAvailabilityParams struct {
	CheckIn  string
	CheckOut string
	Items    []domain.QuoteItemRequest
	Adults   int
	Children int
}

// Service implements booking availability, holds, individual/group booking, and cancellation workflows.
type Service struct {
	txRunner  db.TxRunner
	repo      Repository
	holdStore HoldStore
	settings  propertyapp.SettingsReader
	catalog   propertyapp.RoomCatalogReader
	pricing   pricingapp.QuoteCalculator
	customers peopleapp.CustomerReader
	authz     rbacapp.Authorizer
	audit     auditapp.Recorder
	outbox    auditapp.OutboxEnqueuer
	clk       clock.Clock
	ids       id.Generator
	logger    *slog.Logger
}

// Compile-time interface assertions.
var (
	_ propertyapp.AvailabilityReader   = (*Service)(nil)
	_ peopleapp.CustomerArchiveChecker = (*Service)(nil)
)

// NewService constructs a booking application service.
func NewService(
	txRunner db.TxRunner,
	repo Repository,
	settings propertyapp.SettingsReader,
	catalog propertyapp.RoomCatalogReader,
	pricing pricingapp.QuoteCalculator,
	customers peopleapp.CustomerReader,
	authz rbacapp.Authorizer,
	audit auditapp.Recorder,
	outbox auditapp.OutboxEnqueuer,
	clk clock.Clock,
	ids id.Generator,
) *Service {
	if clk == nil {
		clk = clock.NewSystem()
	}
	if ids == nil {
		ids = id.NewUUIDGenerator()
	}
	return &Service{
		txRunner:  txRunner,
		repo:      repo,
		settings:  settings,
		catalog:   catalog,
		pricing:   pricing,
		customers: customers,
		authz:     authz,
		audit:     audit,
		outbox:    outbox,
		clk:       clk,
		ids:       ids,
		logger:    slog.Default(),
	}
}

// WithHoldStore attaches the Redis Lua hold store.
func (s *Service) WithHoldStore(store HoldStore) *Service {
	s.holdStore = store
	return s
}

// WithLogger attaches a structured logger.
func (s *Service) WithLogger(logger *slog.Logger) *Service {
	if logger != nil {
		s.logger = logger
	}
	return s
}

// EnsureCustomerCanBeArchived blocks archiving a customer who has active bookings.
func (s *Service) EnsureCustomerCanBeArchived(ctx context.Context, customerID uuid.UUID) error {
	hasActive, err := s.repo.HasActiveBookingForCustomer(ctx, customerID)
	if err != nil {
		return err
	}
	if hasActive {
		return apperror.Conflict("cannot archive customer with active bookings")
	}
	return nil
}

// CountAvailableRoomsByType returns the number of bookable rooms available for each room type
// during the half-open stay interval [checkIn, checkOut).
func (s *Service) CountAvailableRoomsByType(ctx context.Context, roomTypeIDs []uuid.UUID, checkIn, checkOut time.Time) (map[uuid.UUID]int, error) {
	counts := make(map[uuid.UUID]int, len(roomTypeIDs))
	if len(roomTypeIDs) == 0 {
		return counts, nil
	}

	roomsByType := make(map[uuid.UUID][]propertyapp.Room, len(roomTypeIDs))
	allRoomIDs := make([]uuid.UUID, 0, len(roomTypeIDs)*4)
	seenTypes := make(map[uuid.UUID]struct{}, len(roomTypeIDs))

	for _, rtID := range roomTypeIDs {
		if _, seen := seenTypes[rtID]; seen {
			continue
		}
		seenTypes[rtID] = struct{}{}
		rooms, err := s.catalog.ListBookableRoomsByType(ctx, rtID)
		if err != nil {
			return nil, err
		}
		roomsByType[rtID] = rooms
		for _, rm := range rooms {
			allRoomIDs = append(allRoomIDs, rm.ID)
		}
	}

	unavailable, err := s.repo.ListUnavailableRoomIDs(ctx, allRoomIDs, checkIn, checkOut, s.clk.Now())
	if err != nil {
		return nil, err
	}

	for rtID, rooms := range roomsByType {
		avail := 0
		for _, rm := range rooms {
			if _, busy := unavailable[rm.ID]; !busy {
				avail++
			}
		}
		counts[rtID] = avail
	}
	return counts, nil
}

// QuoteAvailability validates the requested stay, checks live availability (excluding maintenance rooms,
// overlapping active bookings, and active non-expired holds), and returns a VAT-exclusive price quote
// without creating any hold.
func (s *Service) QuoteAvailability(ctx context.Context, params QuoteAvailabilityParams) (domain.AvailabilityQuote, error) {
	loc := time.UTC
	if s.settings != nil {
		cfg, err := s.settings.GetSettings(ctx)
		if err != nil {
			return domain.AvailabilityQuote{}, err
		}
		if parsedLoc, locErr := cfg.Location(); locErr == nil {
			loc = parsedLoc
		}
	}

	now := s.clk.Now()
	stay, err := domain.ParseStayRange(params.CheckIn, params.CheckOut, now, loc)
	if err != nil {
		return domain.AvailabilityQuote{}, err
	}

	var fields []apperror.FieldError
	if params.Adults <= 0 {
		fields = append(fields, apperror.FieldError{
			Field:   "adults",
			Message: "must be greater than 0",
		})
	}
	if params.Children < 0 {
		fields = append(fields, apperror.FieldError{
			Field:   "children",
			Message: "must be non-negative",
		})
	}
	if len(params.Items) == 0 {
		fields = append(fields, apperror.FieldError{
			Field:   "items",
			Message: "at least one room type item is required",
		})
	} else if len(params.Items) > domain.MaxQuoteItems {
		fields = append(fields, apperror.FieldError{
			Field:   "items",
			Message: fmt.Sprintf("at most %d items are allowed", domain.MaxQuoteItems),
		})
	}

	seenTypes := make(map[uuid.UUID]struct{}, len(params.Items))
	for i, item := range params.Items {
		if item.RoomTypeID == uuid.Nil {
			fields = append(fields, apperror.FieldError{
				Field:   fmt.Sprintf("items[%d].roomTypeId", i),
				Message: "roomTypeId is required",
			})
		} else if _, dup := seenTypes[item.RoomTypeID]; dup {
			fields = append(fields, apperror.FieldError{
				Field:   fmt.Sprintf("items[%d].roomTypeId", i),
				Message: "duplicate roomTypeId in items",
			})
		} else {
			seenTypes[item.RoomTypeID] = struct{}{}
		}
		if item.RoomCount <= 0 {
			fields = append(fields, apperror.FieldError{
				Field:   fmt.Sprintf("items[%d].roomCount", i),
				Message: "must be greater than 0",
			})
		}
	}
	if len(fields) > 0 {
		return domain.AvailabilityQuote{}, apperror.Validation("invalid availability quote request", fields...)
	}

	nights := stay.Nights()
	nightsDec := decimal.NewFromInt(int64(nights))

	roomTypes := make([]propertyapp.RoomType, 0, len(params.Items))
	rtIDs := make([]uuid.UUID, 0, len(params.Items))
	totalCapacity := 0

	for i, item := range params.Items {
		rt, err := s.catalog.GetRoomTypeByID(ctx, item.RoomTypeID)
		if err != nil {
			return domain.AvailabilityQuote{}, err
		}
		if !rt.IsBookable || rt.ArchivedAt != nil {
			return domain.AvailabilityQuote{}, apperror.Validation("room type is not bookable", apperror.FieldError{
				Field:   fmt.Sprintf("items[%d].roomTypeId", i),
				Message: "room type is not currently bookable",
			})
		}
		roomTypes = append(roomTypes, rt)
		rtIDs = append(rtIDs, rt.ID)
		totalCapacity += rt.Capacity * item.RoomCount
	}

	if params.Adults+params.Children > totalCapacity {
		return domain.AvailabilityQuote{}, apperror.Validation("guest count exceeds selected room capacity", apperror.FieldError{
			Field:   "adults",
			Message: fmt.Sprintf("total guests (%d) exceeds total capacity (%d)", params.Adults+params.Children, totalCapacity),
		})
	}

	availCounts, err := s.CountAvailableRoomsByType(ctx, rtIDs, stay.CheckIn, stay.CheckOut)
	if err != nil {
		return domain.AvailabilityQuote{}, err
	}

	quoteItems := make([]domain.AvailabilityQuoteItem, 0, len(params.Items))
	allAvailable := true
	grandSubtotal := decimal.Zero
	grandVat := decimal.Zero
	grandTotal := decimal.Zero
	var (
		vatRateID *uuid.UUID
		vatRate   = decimal.Zero
		currency  = "VND"
	)

	for i, item := range params.Items {
		rt := roomTypes[i]
		avail := availCounts[rt.ID]
		if avail < item.RoomCount {
			allAvailable = false
		}

		// Calculate per-room snapshot then scale by roomCount so group totals match sum of child bookings exactly.
		perRoomSubtotal := rt.BaseNightlyRate.Mul(nightsDec)
		snap, err := s.pricing.Calculate(ctx, perRoomSubtotal)
		if err != nil {
			return domain.AvailabilityQuote{}, err
		}
		countDec := decimal.NewFromInt(int64(item.RoomCount))
		itemSubtotal := snap.Subtotal.Mul(countDec)
		itemVat := snap.VatAmount.Mul(countDec)
		itemTotal := snap.TotalAmount.Mul(countDec)

		if i == 0 {
			vatRateID = snap.VatRateID
			vatRate = snap.VatRate
			currency = snap.Currency
		}

		grandSubtotal = grandSubtotal.Add(itemSubtotal)
		grandVat = grandVat.Add(itemVat)
		grandTotal = grandTotal.Add(itemTotal)

		quoteItems = append(quoteItems, domain.AvailabilityQuoteItem{
			RoomTypeID:     rt.ID,
			RoomTypeCode:   rt.Code,
			RoomTypeName:   rt.Name,
			Capacity:       rt.Capacity,
			RoomCount:      item.RoomCount,
			AvailableCount: avail,
			NightlyRate:    rt.BaseNightlyRate,
			Nights:         nights,
			SubtotalAmount: itemSubtotal,
			VatAmount:      itemVat,
			TotalAmount:    itemTotal,
		})
	}

	return domain.AvailabilityQuote{
		CheckIn:        stay.CheckInDate(),
		CheckOut:       stay.CheckOutDate(),
		Nights:         nights,
		Adults:         params.Adults,
		Children:       params.Children,
		Available:      allAvailable,
		Currency:       currency,
		VatRateID:      vatRateID,
		VatRate:        vatRate,
		SubtotalAmount: grandSubtotal,
		VatAmount:      grandVat,
		TotalAmount:    grandTotal,
		Items:          quoteItems,
	}, nil
}
