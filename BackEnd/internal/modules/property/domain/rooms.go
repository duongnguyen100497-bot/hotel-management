package domain

import (
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/money"
)

const (
	RoomStatusAvailable   = "available"
	RoomStatusOccupied    = "occupied"
	RoomStatusCleaning    = "cleaning"
	RoomStatusMaintenance = "maintenance"
)

var (
	roomTypeCodePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{1,63}$`)
	roomCodePattern     = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{0,31}$`)
	maxNightlyRate      = decimal.RequireFromString("999999999999.99")
	maxAreaSqm          = decimal.RequireFromString("9999.99")
)

// RoomType represents a hotel room category and its base VAT-exclusive nightly rate.
type RoomType struct {
	ID                   uuid.UUID        `json:"id"`
	Code                 string           `json:"code"`
	Name                 string           `json:"name"`
	Description          string           `json:"description,omitempty"`
	BaseNightlyRate      decimal.Decimal  `json:"baseNightlyRate"`
	Currency             string           `json:"currency"`
	Capacity             int              `json:"capacity"`
	AreaSqm              *decimal.Decimal `json:"areaSqm,omitempty"`
	BedType              string           `json:"bedType,omitempty"`
	Amenities            []string         `json:"amenities"`
	IsBookable           bool             `json:"isBookable"`
	OperationalRoomCount int              `json:"operationalRoomCount,omitempty"`
	AvailableRoomCount   *int             `json:"availableRoomCount,omitempty"`
	CoverURL             string           `json:"coverUrl,omitempty"`
	Media                []MediaItem      `json:"media,omitempty"`
	ArchivedAt           *time.Time       `json:"archivedAt,omitempty"`
	CreatedAt            time.Time        `json:"createdAt"`
	UpdatedAt            time.Time        `json:"updatedAt"`
	Version              int              `json:"version"`
}

// Room represents an individual physical hotel room.
type Room struct {
	ID                uuid.UUID   `json:"id"`
	Code              string      `json:"code"`
	Name              string      `json:"name"`
	RoomTypeID        uuid.UUID   `json:"roomTypeId"`
	RoomTypeCode      string      `json:"roomTypeCode,omitempty"`
	RoomTypeName      string      `json:"roomTypeName,omitempty"`
	Floor             int         `json:"floor"`
	Capacity          int         `json:"capacity"`
	OperationalStatus string      `json:"operationalStatus"`
	Description       string      `json:"description,omitempty"`
	CoverURL          string      `json:"coverUrl,omitempty"`
	Media             []MediaItem `json:"media,omitempty"`
	ArchivedAt        *time.Time  `json:"archivedAt,omitempty"`
	CreatedAt         time.Time   `json:"createdAt"`
	UpdatedAt         time.Time   `json:"updatedAt"`
	Version           int         `json:"version"`
}

// NormalizeAndValidateRoomType validates a RoomType for create or update.
func (rt *RoomType) NormalizeAndValidateRoomType(checkVersion bool, expectedVersion int) error {
	rt.Code = strings.ToUpper(strings.TrimSpace(rt.Code))
	rt.Name = strings.TrimSpace(rt.Name)
	rt.Description = strings.TrimSpace(rt.Description)
	rt.Currency = strings.ToUpper(strings.TrimSpace(rt.Currency))
	if rt.Currency == "" {
		rt.Currency = "VND"
	}
	rt.BedType = strings.TrimSpace(rt.BedType)

	cleanAmenities := make([]string, 0, len(rt.Amenities))
	seen := make(map[string]struct{}, len(rt.Amenities))
	for _, a := range rt.Amenities {
		trimmed := strings.TrimSpace(a)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		cleanAmenities = append(cleanAmenities, trimmed)
	}
	rt.Amenities = cleanAmenities

	var fields []apperror.FieldError
	if checkVersion && expectedVersion < 1 {
		fields = append(fields, apperror.FieldError{
			Field:   "version",
			Message: "version must be a positive integer",
		})
	}
	if !roomTypeCodePattern.MatchString(rt.Code) {
		fields = append(fields, apperror.FieldError{
			Field:   "code",
			Message: "must be 2..64 uppercase alphanumeric characters with optional _ or -",
		})
	}
	if rt.Name == "" {
		fields = append(fields, apperror.FieldError{
			Field:   "name",
			Message: "name is required",
		})
	}
	if !currencyPattern.MatchString(rt.Currency) {
		fields = append(fields, apperror.FieldError{
			Field:   "currency",
			Message: "must be a 3-letter uppercase ISO currency code",
		})
	}
	if !rt.BaseNightlyRate.IsPositive() {
		fields = append(fields, apperror.FieldError{
			Field:   "baseNightlyRate",
			Message: "must be greater than 0",
		})
	} else if !rt.BaseNightlyRate.Equal(rt.BaseNightlyRate.Round(money.Scale)) || rt.BaseNightlyRate.GreaterThan(maxNightlyRate) {
		fields = append(fields, apperror.FieldError{
			Field:   "baseNightlyRate",
			Message: "must fit numeric(14,2) with at most 2 decimal places",
		})
	} else if rt.Currency == "VND" && !rt.BaseNightlyRate.Equal(rt.BaseNightlyRate.Round(0)) {
		fields = append(fields, apperror.FieldError{
			Field:   "baseNightlyRate",
			Message: "VND baseNightlyRate must be a whole integer amount",
		})
	}
	if rt.Capacity <= 0 || rt.Capacity > 100 {
		fields = append(fields, apperror.FieldError{
			Field:   "capacity",
			Message: "must be between 1 and 100",
		})
	}
	if rt.AreaSqm != nil {
		if !rt.AreaSqm.IsPositive() || !rt.AreaSqm.Equal(rt.AreaSqm.Round(2)) || rt.AreaSqm.GreaterThan(maxAreaSqm) {
			fields = append(fields, apperror.FieldError{
				Field:   "areaSqm",
				Message: "must be positive and fit numeric(6,2)",
			})
		}
	}

	if len(fields) > 0 {
		return apperror.Validation("invalid room type", fields...)
	}
	return nil
}

// NormalizeAndValidateRoom validates physical room fields for create or update.
func (r *Room) NormalizeAndValidateRoom(checkVersion bool, expectedVersion int) error {
	r.Code = strings.ToUpper(strings.TrimSpace(r.Code))
	r.Name = strings.TrimSpace(r.Name)
	r.Description = strings.TrimSpace(r.Description)

	var fields []apperror.FieldError
	if checkVersion && expectedVersion < 1 {
		fields = append(fields, apperror.FieldError{
			Field:   "version",
			Message: "version must be a positive integer",
		})
	}
	if !roomCodePattern.MatchString(r.Code) {
		fields = append(fields, apperror.FieldError{
			Field:   "code",
			Message: "must be 1..32 uppercase alphanumeric characters with optional _ or -",
		})
	}
	if r.Name == "" {
		fields = append(fields, apperror.FieldError{
			Field:   "name",
			Message: "name is required",
		})
	}
	if r.RoomTypeID == uuid.Nil {
		fields = append(fields, apperror.FieldError{
			Field:   "roomTypeId",
			Message: "roomTypeId is required",
		})
	}
	if r.Floor < -10 || r.Floor > 200 {
		fields = append(fields, apperror.FieldError{
			Field:   "floor",
			Message: "must be between -10 and 200",
		})
	}
	if r.Capacity <= 0 || r.Capacity > 100 {
		fields = append(fields, apperror.FieldError{
			Field:   "capacity",
			Message: "must be between 1 and 100",
		})
	}

	if len(fields) > 0 {
		return apperror.Validation("invalid room", fields...)
	}
	return nil
}

// IsValidRoomStatus checks if status is one of the 4 canonical operational states.
func IsValidRoomStatus(status string) bool {
	switch status {
	case RoomStatusAvailable, RoomStatusOccupied, RoomStatusCleaning, RoomStatusMaintenance:
		return true
	default:
		return false
	}
}
