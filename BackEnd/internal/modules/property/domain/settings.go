package domain

import (
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
)

const (
	MinIndividualHoldTTLMinutes = 1
	MaxIndividualHoldTTLMinutes = 120
	MinGroupHoldTTLMinutes      = 1
	MaxGroupHoldTTLMinutes      = 180
	MinFreeCancelHours          = 0
	MaxFreeCancelHours          = 8760
)

var (
	currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
	hhmmPattern     = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)
)

// MediaItem represents a resolved image asset linked to a room, room type, or hotel logo.
type MediaItem struct {
	MediaID   uuid.UUID `json:"mediaId"`
	URL       string    `json:"url"`
	IsCover   bool      `json:"isCover"`
	SortOrder int       `json:"sortOrder"`
	MimeType  string    `json:"mimeType,omitempty"`
	Width     *int      `json:"width,omitempty"`
	Height    *int      `json:"height,omitempty"`
}

// HotelSettings holds the single-hotel operational configuration stored in PostgreSQL.
type HotelSettings struct {
	Name                     string     `json:"name"`
	Address                  string     `json:"address"`
	Phone                    string     `json:"phone"`
	Email                    string     `json:"email"`
	Timezone                 string     `json:"timezone"`
	Currency                 string     `json:"currency"`
	CheckInTime              string     `json:"checkInTime"`
	CheckOutTime             string     `json:"checkOutTime"`
	IndividualHoldTTLMinutes int        `json:"individualHoldTtlMinutes"`
	GroupHoldTTLMinutes      int        `json:"groupHoldTtlMinutes"`
	FreeCancelHours          int        `json:"freeCancelHours"`
	LogoMediaID              *uuid.UUID `json:"logoMediaId,omitempty"`
	LogoURL                  string     `json:"logoUrl,omitempty"`
	UpdatedAt                time.Time  `json:"updatedAt"`
	Version                  int        `json:"version"`
}

// PublicHotelSettings exposes non-administrative hotel display and booking policy fields.
type PublicHotelSettings struct {
	Name                     string `json:"name"`
	Address                  string `json:"address"`
	Phone                    string `json:"phone"`
	Email                    string `json:"email"`
	Timezone                 string `json:"timezone"`
	Currency                 string `json:"currency"`
	CheckInTime              string `json:"checkInTime"`
	CheckOutTime             string `json:"checkOutTime"`
	IndividualHoldTTLMinutes int    `json:"individualHoldTtlMinutes"`
	GroupHoldTTLMinutes      int    `json:"groupHoldTtlMinutes"`
	FreeCancelHours          int    `json:"freeCancelHours"`
	LogoURL                  string `json:"logoUrl,omitempty"`
}

// ToPublic projects HotelSettings into PublicHotelSettings.
func (s HotelSettings) ToPublic() PublicHotelSettings {
	return PublicHotelSettings{
		Name:                     s.Name,
		Address:                  s.Address,
		Phone:                    s.Phone,
		Email:                    s.Email,
		Timezone:                 s.Timezone,
		Currency:                 s.Currency,
		CheckInTime:              s.CheckInTime,
		CheckOutTime:             s.CheckOutTime,
		IndividualHoldTTLMinutes: s.IndividualHoldTTLMinutes,
		GroupHoldTTLMinutes:      s.GroupHoldTTLMinutes,
		FreeCancelHours:          s.FreeCancelHours,
		LogoURL:                  s.LogoURL,
	}
}

// IndividualHoldTTL returns the configured individual hold lifetime as time.Duration.
func (s HotelSettings) IndividualHoldTTL() time.Duration {
	return time.Duration(s.IndividualHoldTTLMinutes) * time.Minute
}

// GroupHoldTTL returns the configured group hold lifetime as time.Duration.
func (s HotelSettings) GroupHoldTTL() time.Duration {
	return time.Duration(s.GroupHoldTTLMinutes) * time.Minute
}

// FreeCancelWindow returns the configured free cancellation lead time as time.Duration.
func (s HotelSettings) FreeCancelWindow() time.Duration {
	return time.Duration(s.FreeCancelHours) * time.Hour
}

// Location parses Timezone into a *time.Location.
func (s HotelSettings) Location() (*time.Location, error) {
	return time.LoadLocation(s.Timezone)
}

// NormalizeAndValidate normalizes string fields and validates all hotel settings invariants.
func (s *HotelSettings) NormalizeAndValidate(expectedVersion int) error {
	s.Name = strings.TrimSpace(s.Name)
	s.Address = strings.TrimSpace(s.Address)
	s.Phone = strings.TrimSpace(s.Phone)
	s.Email = strings.ToLower(strings.TrimSpace(s.Email))
	s.Timezone = strings.TrimSpace(s.Timezone)
	s.Currency = strings.ToUpper(strings.TrimSpace(s.Currency))
	s.CheckInTime = strings.TrimSpace(s.CheckInTime)
	s.CheckOutTime = strings.TrimSpace(s.CheckOutTime)

	var fields []apperror.FieldError
	if expectedVersion < 1 {
		fields = append(fields, apperror.FieldError{
			Field:   "version",
			Message: "version must be a positive integer",
		})
	}
	if s.Name == "" {
		fields = append(fields, apperror.FieldError{
			Field:   "name",
			Message: "name is required",
		})
	}
	if s.Address == "" {
		fields = append(fields, apperror.FieldError{
			Field:   "address",
			Message: "address is required",
		})
	}
	if s.Phone == "" {
		fields = append(fields, apperror.FieldError{
			Field:   "phone",
			Message: "phone is required",
		})
	}
	if s.Email == "" {
		fields = append(fields, apperror.FieldError{
			Field:   "email",
			Message: "email is required",
		})
	} else if addr, err := mail.ParseAddress(s.Email); err != nil || addr.Address != s.Email {
		fields = append(fields, apperror.FieldError{
			Field:   "email",
			Message: "must be a valid email address",
		})
	}

	if s.Timezone == "" {
		fields = append(fields, apperror.FieldError{
			Field:   "timezone",
			Message: "timezone is required",
		})
	} else if _, err := time.LoadLocation(s.Timezone); err != nil {
		fields = append(fields, apperror.FieldError{
			Field:   "timezone",
			Message: "must be a valid IANA timezone (e.g. Asia/Ho_Chi_Minh)",
		})
	}

	if !currencyPattern.MatchString(s.Currency) {
		fields = append(fields, apperror.FieldError{
			Field:   "currency",
			Message: "must be a 3-letter uppercase ISO currency code (e.g. VND)",
		})
	}
	if !hhmmPattern.MatchString(s.CheckInTime) {
		fields = append(fields, apperror.FieldError{
			Field:   "checkInTime",
			Message: "must be in 24-hour HH:MM format (e.g. 14:00)",
		})
	}
	if !hhmmPattern.MatchString(s.CheckOutTime) {
		fields = append(fields, apperror.FieldError{
			Field:   "checkOutTime",
			Message: "must be in 24-hour HH:MM format (e.g. 12:00)",
		})
	}
	if s.IndividualHoldTTLMinutes < MinIndividualHoldTTLMinutes || s.IndividualHoldTTLMinutes > MaxIndividualHoldTTLMinutes {
		fields = append(fields, apperror.FieldError{
			Field:   "individualHoldTtlMinutes",
			Message: "must be between 1 and 120 minutes",
		})
	}
	if s.GroupHoldTTLMinutes < MinGroupHoldTTLMinutes || s.GroupHoldTTLMinutes > MaxGroupHoldTTLMinutes {
		fields = append(fields, apperror.FieldError{
			Field:   "groupHoldTtlMinutes",
			Message: "must be between 1 and 180 minutes",
		})
	}
	if s.FreeCancelHours < MinFreeCancelHours || s.FreeCancelHours > MaxFreeCancelHours {
		fields = append(fields, apperror.FieldError{
			Field:   "freeCancelHours",
			Message: "must be between 0 and 8760 hours",
		})
	}

	if len(fields) > 0 {
		return apperror.Validation("invalid hotel settings", fields...)
	}
	return nil
}
