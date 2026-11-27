package domain

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
)

// BookingStatus represents the lifecycle state of an individual booking.
type BookingStatus string

const (
	BookingStatusPending    BookingStatus = "pending"
	BookingStatusConfirmed  BookingStatus = "confirmed"
	BookingStatusCheckedIn  BookingStatus = "checked_in"
	BookingStatusCheckedOut BookingStatus = "checked_out"
	BookingStatusCancelled  BookingStatus = "cancelled"
	BookingStatusExpired    BookingStatus = "expired"
)

// GroupStatus represents the aggregate lifecycle state of a booking group.
type GroupStatus string

const (
	GroupStatusHolding            GroupStatus = "holding"
	GroupStatusPending            GroupStatus = "pending"
	GroupStatusPartiallyConfirmed GroupStatus = "partially_confirmed"
	GroupStatusConfirmed          GroupStatus = "confirmed"
	GroupStatusPartiallyCancelled GroupStatus = "partially_cancelled"
	GroupStatusCancelled          GroupStatus = "cancelled"
	GroupStatusExpired            GroupStatus = "expired"
	GroupStatusCompleted          GroupStatus = "completed"
)

// HoldMode indicates whether a hold is for a single room or a multi-room group.
type HoldMode string

const (
	HoldModeIndividual HoldMode = "individual"
	HoldModeGroup      HoldMode = "group"
)

// HoldStatus represents the authoritative PostgreSQL state of a reservation hold.
type HoldStatus string

const (
	HoldStatusActive    HoldStatus = "active"
	HoldStatusConverted HoldStatus = "converted"
	HoldStatusReleased  HoldStatus = "released"
	HoldStatusExpired   HoldStatus = "expired"
)

const (
	DateLayout      = "2006-01-02"
	MaxStayNights   = 365
	MaxQuoteItems   = 20
	MaxBatchCancel  = 50
	DefaultTimezone = "Asia/Ho_Chi_Minh"
)

// StayRange represents a half-open stay interval [CheckIn, CheckOut).
type StayRange struct {
	CheckIn  time.Time
	CheckOut time.Time
}

// Nights returns the number of nights in the stay.
func (s StayRange) Nights() int {
	return int(s.CheckOut.Sub(s.CheckIn).Hours() / 24)
}

// CheckInDate returns YYYY-MM-DD formatted check-in date.
func (s StayRange) CheckInDate() string {
	return s.CheckIn.Format(DateLayout)
}

// CheckOutDate returns YYYY-MM-DD formatted check-out date.
func (s StayRange) CheckOutDate() string {
	return s.CheckOut.Format(DateLayout)
}

// ParseStayRange parses and validates [checkIn, checkOut) against today in the hotel's timezone.
func ParseStayRange(checkInRaw, checkOutRaw string, now time.Time, loc *time.Location) (StayRange, error) {
	if loc == nil {
		var err error
		loc, err = time.LoadLocation(DefaultTimezone)
		if err != nil {
			loc = time.UTC
		}
	}

	var fields []apperror.FieldError
	inDate, inErr := time.Parse(DateLayout, strings.TrimSpace(checkInRaw))
	if inErr != nil {
		fields = append(fields, apperror.FieldError{
			Field:   "checkIn",
			Message: "must be in YYYY-MM-DD format",
		})
	}
	outDate, outErr := time.Parse(DateLayout, strings.TrimSpace(checkOutRaw))
	if outErr != nil {
		fields = append(fields, apperror.FieldError{
			Field:   "checkOut",
			Message: "must be in YYYY-MM-DD format",
		})
	}
	if len(fields) > 0 {
		return StayRange{}, apperror.Validation("invalid stay dates", fields...)
	}

	// Normalize to UTC midnight date-only values for date comparison and PostgreSQL daterange.
	inDate = time.Date(inDate.Year(), inDate.Month(), inDate.Day(), 0, 0, 0, 0, time.UTC)
	outDate = time.Date(outDate.Year(), outDate.Month(), outDate.Day(), 0, 0, 0, 0, time.UTC)

	localNow := now.In(loc)
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.UTC)
	if inDate.Before(today) {
		fields = append(fields, apperror.FieldError{
			Field:   "checkIn",
			Message: "checkIn must not be in the past",
		})
	}
	if !outDate.After(inDate) {
		fields = append(fields, apperror.FieldError{
			Field:   "checkOut",
			Message: "checkOut must be after checkIn",
		})
	} else {
		nights := int(outDate.Sub(inDate).Hours() / 24)
		if nights > MaxStayNights {
			fields = append(fields, apperror.FieldError{
				Field:   "checkOut",
				Message: fmt.Sprintf("stay cannot exceed %d nights", MaxStayNights),
			})
		}
	}
	if len(fields) > 0 {
		return StayRange{}, apperror.Validation("invalid stay date range", fields...)
	}

	return StayRange{CheckIn: inDate, CheckOut: outDate}, nil
}

// QuoteItemRequest represents a requested room type and quantity for an availability quote.
type QuoteItemRequest struct {
	RoomTypeID uuid.UUID `json:"roomTypeId"`
	RoomCount  int       `json:"roomCount"`
}

// AvailabilityQuoteItem holds availability and VAT-exclusive pricing breakdown for one room type.
type AvailabilityQuoteItem struct {
	RoomTypeID     uuid.UUID       `json:"roomTypeId"`
	RoomTypeCode   string          `json:"roomTypeCode"`
	RoomTypeName   string          `json:"roomTypeName"`
	Capacity       int             `json:"capacity"`
	RoomCount      int             `json:"roomCount"`
	AvailableCount int             `json:"availableCount"`
	NightlyRate    decimal.Decimal `json:"nightlyRate"`
	Nights         int             `json:"nights"`
	SubtotalAmount decimal.Decimal `json:"subtotalAmount"`
	VatAmount      decimal.Decimal `json:"vatAmount"`
	TotalAmount    decimal.Decimal `json:"totalAmount"`
}

// AvailabilityQuote is the response payload of POST /api/v1/availability/quote.
type AvailabilityQuote struct {
	CheckIn        string                  `json:"checkIn"`
	CheckOut       string                  `json:"checkOut"`
	Nights         int                     `json:"nights"`
	Adults         int                     `json:"adults"`
	Children       int                     `json:"children"`
	Available      bool                    `json:"available"`
	Currency       string                  `json:"currency"`
	VatRateID      *uuid.UUID              `json:"vatRateId,omitempty"`
	VatRate        decimal.Decimal         `json:"vatRate"`
	SubtotalAmount decimal.Decimal         `json:"subtotalAmount"`
	VatAmount      decimal.Decimal         `json:"vatAmount"`
	TotalAmount    decimal.Decimal         `json:"totalAmount"`
	Items          []AvailabilityQuoteItem `json:"items"`
}

// ReservationHold represents an authoritative PostgreSQL hold record paired with a Redis hold key.
type ReservationHold struct {
	ID             uuid.UUID  `json:"id"`
	CustomerID     uuid.UUID  `json:"customerId"`
	BookingGroupID *uuid.UUID `json:"bookingGroupId,omitempty"`
	RoomID         uuid.UUID  `json:"roomId"`
	RoomTypeID     uuid.UUID  `json:"roomTypeId"`
	CheckIn        time.Time  `json:"-"`
	CheckOut       time.Time  `json:"-"`
	HoldMode       HoldMode   `json:"holdMode"`
	HoldToken      string     `json:"-"`
	RedisKey       string     `json:"redisKey"`
	TTLSeconds     int        `json:"ttlSeconds"`
	Status         HoldStatus `json:"status"`
	ExpiresAt      time.Time  `json:"expiresAt"`
	ReleasedReason string     `json:"releasedReason,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// HoldRoomSummary describes a single room locked inside a hold response.
type HoldRoomSummary struct {
	ID           uuid.UUID `json:"id"`
	RoomID       uuid.UUID `json:"roomId"`
	RoomCode     string    `json:"roomCode"`
	RoomTypeID   uuid.UUID `json:"roomTypeId"`
	RoomTypeCode string    `json:"roomTypeCode"`
	RedisKey     string    `json:"redisKey"`
}

// HoldResult is the response payload of POST /api/v1/holds.
type HoldResult struct {
	HoldID      uuid.UUID         `json:"holdId"`
	GroupHoldID *uuid.UUID        `json:"groupHoldId,omitempty"`
	HoldMode    HoldMode          `json:"holdMode"`
	HoldToken   string            `json:"holdToken"`
	TTLSeconds  int               `json:"ttlSeconds"`
	ExpiresAt   time.Time         `json:"expiresAt"`
	CheckIn     string            `json:"checkIn"`
	CheckOut    string            `json:"checkOut"`
	Nights      int               `json:"nights"`
	Holds       []HoldRoomSummary `json:"holds"`
	Quote       AvailabilityQuote `json:"quote"`
}

// RoomHoldRedisKey returns the zero-PII Redis key for a room hold:
// hold:room:{roomId}:{YYYY-MM-DD}:{YYYY-MM-DD}
func RoomHoldRedisKey(roomID uuid.UUID, checkIn, checkOut time.Time) string {
	return fmt.Sprintf("hold:room:%s:%s:%s", roomID.String(), checkIn.Format(DateLayout), checkOut.Format(DateLayout))
}

// GroupHoldRedisKey returns the zero-PII Redis key for a group hold:
// hold:group:{groupId}
func GroupHoldRedisKey(groupID uuid.UUID) string {
	return fmt.Sprintf("hold:group:%s", groupID.String())
}

// GenerateHoldToken produces a cryptographically random 64-char hex hold token (32 bytes).
func GenerateHoldToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate hold token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// ComputeGroupStatus derives the aggregate booking_groups.status from all child booking statuses.
func ComputeGroupStatus(childStatuses []BookingStatus) GroupStatus {
	if len(childStatuses) == 0 {
		return GroupStatusHolding
	}

	var (
		nPending    int
		nConfirmed  int
		nCheckedIn  int
		nCheckedOut int
		nCancelled  int
		nExpired    int
	)
	for _, st := range childStatuses {
		switch st {
		case BookingStatusPending:
			nPending++
		case BookingStatusConfirmed:
			nConfirmed++
		case BookingStatusCheckedIn:
			nCheckedIn++
		case BookingStatusCheckedOut:
			nCheckedOut++
		case BookingStatusCancelled:
			nCancelled++
		case BookingStatusExpired:
			nExpired++
		}
	}

	total := len(childStatuses)
	switch {
	case nExpired == total:
		return GroupStatusExpired
	case nCancelled == total:
		return GroupStatusCancelled
	case nCheckedOut == total:
		return GroupStatusCompleted
	case nCancelled > 0:
		return GroupStatusPartiallyCancelled
	case nConfirmed+nCheckedIn+nCheckedOut == total:
		return GroupStatusConfirmed
	case (nConfirmed+nCheckedIn+nCheckedOut) > 0 && (nPending > 0 || nExpired > 0):
		return GroupStatusPartiallyConfirmed
	default:
		return GroupStatusPending
	}
}

// ExpiredBookingSummary carries minimal info for an expired pending booking.
type ExpiredBookingSummary struct {
	ID             uuid.UUID
	ReferenceCode  string
	BookingGroupID *uuid.UUID
	CustomerID     uuid.UUID
	RoomID         uuid.UUID
	RoomTypeID     uuid.UUID
	CheckIn        time.Time
	CheckOut       time.Time
}

// BookingStatusHistoryEntry represents an immutable status transition record.
type BookingStatusHistoryEntry struct {
	ID            uuid.UUID
	BookingID     uuid.UUID
	FromStatus    *BookingStatus
	ToStatus      BookingStatus
	ActorUserID   *uuid.UUID
	Reason        string
	CorrelationID *uuid.UUID
	OccurredAt    time.Time
}

// GuestSnapshot captures immutable representative guest information at booking time.
// Updating customers afterwards MUST NEVER mutate this snapshot.
type GuestSnapshot struct {
	FullName        string    `json:"fullName"`
	Phone           string    `json:"phone"`
	Email           string    `json:"email,omitempty"`
	NationalityCode string    `json:"nationalityCode,omitempty"`
	SpecialRequest  string    `json:"specialRequest,omitempty"`
	CreatedAt       time.Time `json:"createdAt,omitempty"`
}

// NormalizeAndValidate validates required guest snapshot fields.
func (g *GuestSnapshot) NormalizeAndValidate(prefix string) error {
	g.FullName = strings.TrimSpace(g.FullName)
	g.Phone = strings.TrimSpace(g.Phone)
	g.Email = strings.TrimSpace(g.Email)
	g.NationalityCode = strings.ToUpper(strings.TrimSpace(g.NationalityCode))
	g.SpecialRequest = strings.TrimSpace(g.SpecialRequest)

	p := ""
	if prefix != "" {
		p = prefix + "."
	}
	var fields []apperror.FieldError
	if g.FullName == "" {
		fields = append(fields, apperror.FieldError{Field: p + "fullName", Message: "fullName is required"})
	}
	if g.Phone == "" {
		fields = append(fields, apperror.FieldError{Field: p + "phone", Message: "phone is required"})
	}
	if len(g.NationalityCode) > 8 {
		fields = append(fields, apperror.FieldError{Field: p + "nationalityCode", Message: "must be at most 8 characters"})
	}
	if len(fields) > 0 {
		return apperror.Validation("invalid representative guest", fields...)
	}
	return nil
}

// Booking represents a persisted room reservation with immutable pricing and guest snapshots.
type Booking struct {
	ID                  uuid.UUID       `json:"id"`
	ReferenceCode       string          `json:"referenceCode"`
	BookingGroupID      *uuid.UUID      `json:"bookingGroupId,omitempty"`
	CustomerID          uuid.UUID       `json:"customerId"`
	RoomID              uuid.UUID       `json:"roomId"`
	RoomTypeID          uuid.UUID       `json:"roomTypeId"`
	CheckIn             string          `json:"checkIn"`
	CheckOut            string          `json:"checkOut"`
	CheckInTime         time.Time       `json:"-"`
	CheckOutTime        time.Time       `json:"-"`
	Nights              int             `json:"nights"`
	Adults              int             `json:"adults"`
	Children            int             `json:"children"`
	Status              BookingStatus   `json:"status"`
	NightlyRateSnapshot decimal.Decimal `json:"nightlyRateSnapshot"`
	VatRateID           uuid.UUID       `json:"vatRateId"`
	VatRateSnapshot     decimal.Decimal `json:"vatRateSnapshot"`
	SubtotalAmount      decimal.Decimal `json:"subtotalAmount"`
	VatAmount           decimal.Decimal `json:"vatAmount"`
	TotalAmount         decimal.Decimal `json:"totalAmount"`
	Currency            string          `json:"currency"`
	SpecialRequest      string          `json:"specialRequest,omitempty"`
	ExpiresAt           *time.Time      `json:"expiresAt,omitempty"`
	CancelledAt         *time.Time      `json:"cancelledAt,omitempty"`
	CancelReason        string          `json:"cancelReason,omitempty"`
	IdempotencyKey      string          `json:"-"`
	RepresentativeGuest *GuestSnapshot  `json:"representativeGuest,omitempty"`
	CreatedAt           time.Time       `json:"createdAt"`
	UpdatedAt           time.Time       `json:"updatedAt"`
	Version             int             `json:"version"`
}

// GenerateReferenceCode builds a human-readable booking/group reference code (e.g. BK-20261201-A1B2C3).
func GenerateReferenceCode(prefix string, now time.Time) string {
	buf := make([]byte, 4)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("%s-%s-%s", prefix, now.UTC().Format("20060102"), strings.ToUpper(hex.EncodeToString(buf)))
}
