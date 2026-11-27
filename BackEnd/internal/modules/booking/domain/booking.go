package domain

import (
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
