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

var (
	serviceCodePattern     = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{1,63}$`)
	serviceCategoryPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,63}$`)
)

// ServiceCatalogItem represents an ancillary hotel service (laundry, minibar, dining, transport, etc.)
// with a VAT-exclusive unit price.
type ServiceCatalogItem struct {
	ID          uuid.UUID       `json:"id"`
	Code        string          `json:"code"`
	Name        string          `json:"name"`
	Category    string          `json:"category"`
	UnitPrice   decimal.Decimal `json:"unitPrice"`
	Currency    string          `json:"currency"`
	Description string          `json:"description,omitempty"`
	IsActive    bool            `json:"isActive"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
	Version     int             `json:"version"`
}

// NormalizeAndValidate normalizes string fields and validates all ServiceCatalogItem invariants.
func (s *ServiceCatalogItem) NormalizeAndValidate(checkVersion bool, expectedVersion int) error {
	s.Code = strings.ToUpper(strings.TrimSpace(s.Code))
	s.Name = strings.TrimSpace(s.Name)
	s.Category = strings.ToLower(strings.TrimSpace(s.Category))
	s.Currency = strings.ToUpper(strings.TrimSpace(s.Currency))
	if s.Currency == "" {
		s.Currency = "VND"
	}
	s.Description = strings.TrimSpace(s.Description)

	var fields []apperror.FieldError
	if checkVersion && expectedVersion < 1 {
		fields = append(fields, apperror.FieldError{
			Field:   "version",
			Message: "version must be a positive integer",
		})
	}
	if !serviceCodePattern.MatchString(s.Code) {
		fields = append(fields, apperror.FieldError{
			Field:   "code",
			Message: "must be 2..64 uppercase alphanumeric characters with optional _ or -",
		})
	}
	if s.Name == "" {
		fields = append(fields, apperror.FieldError{
			Field:   "name",
			Message: "name is required",
		})
	}
	if !serviceCategoryPattern.MatchString(s.Category) {
		fields = append(fields, apperror.FieldError{
			Field:   "category",
			Message: "must be 2..64 lowercase alphanumeric characters with optional _ or - (e.g. dining, laundry, minibar)",
		})
	}
	if !currencyPattern.MatchString(s.Currency) {
		fields = append(fields, apperror.FieldError{
			Field:   "currency",
			Message: "must be a 3-letter uppercase ISO currency code",
		})
	}
	if s.UnitPrice.IsNegative() {
		fields = append(fields, apperror.FieldError{
			Field:   "unitPrice",
			Message: "must be non-negative (>= 0)",
		})
	} else if !s.UnitPrice.Equal(s.UnitPrice.Round(money.Scale)) || s.UnitPrice.GreaterThan(maxNightlyRate) {
		fields = append(fields, apperror.FieldError{
			Field:   "unitPrice",
			Message: "must fit numeric(14,2) with at most 2 decimal places",
		})
	} else if s.Currency == "VND" && !s.UnitPrice.Equal(s.UnitPrice.Round(0)) {
		fields = append(fields, apperror.FieldError{
			Field:   "unitPrice",
			Message: "VND unitPrice must be a whole integer amount",
		})
	}

	if len(fields) > 0 {
		return apperror.Validation("invalid service catalog item", fields...)
	}
	return nil
}
