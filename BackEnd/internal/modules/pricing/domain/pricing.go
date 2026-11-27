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
	DefaultCurrency = "VND"
	VATRateScale    = 4
)

var (
	vatCodePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_.-]{1,31}$`)
	maxMoneyAmount = decimal.RequireFromString("999999999999.99")
	oneDecimal     = decimal.NewFromInt(1)
)

// VATRate represents a server-managed VAT rate definition.
type VATRate struct {
	ID        uuid.UUID       `json:"id"`
	Code      string          `json:"code"`
	Name      string          `json:"name"`
	Rate      decimal.Decimal `json:"rate"`
	IsActive  bool            `json:"isActive"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
	Version   int             `json:"version"`
}

// PricingSnapshot captures an immutable VAT-exclusive quote breakdown (no separate service fee).
type PricingSnapshot struct {
	VatRateID   *uuid.UUID      `json:"vatRateId,omitempty"`
	VatRate     decimal.Decimal `json:"vatRate"`
	Subtotal    decimal.Decimal `json:"subtotal"`
	VatAmount   decimal.Decimal `json:"vatAmount"`
	TotalAmount decimal.Decimal `json:"totalAmount"`
	Currency    string          `json:"currency"`
}

// NormalizeAndValidateCreate validates fields for creating a new VAT rate.
func NormalizeAndValidateCreate(code, name string, rate decimal.Decimal) (string, string, decimal.Decimal, error) {
	normCode := strings.ToUpper(strings.TrimSpace(code))
	normName := strings.TrimSpace(name)

	var fields []apperror.FieldError
	if !vatCodePattern.MatchString(normCode) {
		fields = append(fields, apperror.FieldError{
			Field:   "code",
			Message: "must be 2..32 uppercase alphanumeric characters with optional _, ., or -",
		})
	}
	if normName == "" {
		fields = append(fields, apperror.FieldError{
			Field:   "name",
			Message: "name is required",
		})
	}
	if err := validateRateDecimal(rate); err != nil {
		fields = append(fields, *err)
	}
	if len(fields) > 0 {
		return "", "", decimal.Decimal{}, apperror.Validation("invalid VAT rate", fields...)
	}
	return normCode, normName, rate.Round(VATRateScale), nil
}

// ValidateUpdate validates fields for updating an existing VAT rate.
func ValidateUpdate(name string, rate decimal.Decimal, expectedVersion int) (string, decimal.Decimal, error) {
	normName := strings.TrimSpace(name)
	var fields []apperror.FieldError
	if expectedVersion < 1 {
		fields = append(fields, apperror.FieldError{
			Field:   "version",
			Message: "version must be a positive integer",
		})
	}
	if normName == "" {
		fields = append(fields, apperror.FieldError{
			Field:   "name",
			Message: "name is required",
		})
	}
	if err := validateRateDecimal(rate); err != nil {
		fields = append(fields, *err)
	}
	if len(fields) > 0 {
		return "", decimal.Decimal{}, apperror.Validation("invalid VAT rate update", fields...)
	}
	return normName, rate.Round(VATRateScale), nil
}

func validateRateDecimal(rate decimal.Decimal) *apperror.FieldError {
	if rate.IsNegative() || rate.GreaterThan(oneDecimal) {
		return &apperror.FieldError{
			Field:   "rate",
			Message: "rate must be between 0 and 1 inclusive (e.g. 0.08 or 0.10)",
		}
	}
	if !rate.Equal(rate.Round(VATRateScale)) {
		return &apperror.FieldError{
			Field:   "rate",
			Message: "rate must have at most 4 decimal places",
		}
	}
	return nil
}

// CalculateQuote computes an exact decimal VAT-exclusive quote:
// vat_amount = round(subtotal * vat_rate), total = subtotal + vat_amount (zero service fee).
// For VND, rounding uses 0 fractional digits (whole đồng); other currencies round to 2 decimal places.
func CalculateQuote(subtotal decimal.Decimal, activeVAT *VATRate, currency string) (PricingSnapshot, error) {
	curr := strings.ToUpper(strings.TrimSpace(currency))
	if curr == "" {
		curr = DefaultCurrency
	}

	if subtotal.IsNegative() {
		return PricingSnapshot{}, apperror.Validation("invalid subtotal", apperror.FieldError{
			Field:   "subtotal",
			Message: "subtotal must be non-negative",
		})
	}
	if !subtotal.Equal(subtotal.Round(money.Scale)) {
		return PricingSnapshot{}, apperror.Validation("invalid subtotal", apperror.FieldError{
			Field:   "subtotal",
			Message: "subtotal must have at most 2 decimal places",
		})
	}
	if curr == DefaultCurrency && !subtotal.Equal(subtotal.Round(0)) {
		return PricingSnapshot{}, apperror.Validation("invalid subtotal", apperror.FieldError{
			Field:   "subtotal",
			Message: "VND subtotal must be a whole integer amount",
		})
	}
	if subtotal.GreaterThan(maxMoneyAmount) {
		return PricingSnapshot{}, apperror.Validation("invalid subtotal", apperror.FieldError{
			Field:   "subtotal",
			Message: "subtotal exceeds numeric(14,2) maximum",
		})
	}

	roundScale := int32(money.Scale)
	if curr == DefaultCurrency {
		roundScale = 0
	}

	normSubtotal := subtotal.Round(roundScale)
	var (
		vatRateID *uuid.UUID
		rate      = decimal.Zero
	)
	if activeVAT != nil {
		if errField := validateRateDecimal(activeVAT.Rate); errField != nil {
			return PricingSnapshot{}, apperror.Validation("invalid active VAT rate", *errField)
		}
		if activeVAT.ID != uuid.Nil {
			idCopy := activeVAT.ID
			vatRateID = &idCopy
		}
		rate = activeVAT.Rate.Round(VATRateScale)
	}

	vatAmount := normSubtotal.Mul(rate).Round(roundScale)
	totalAmount := normSubtotal.Add(vatAmount)
	if totalAmount.GreaterThan(maxMoneyAmount) {
		return PricingSnapshot{}, apperror.Validation("quote total exceeds numeric(14,2) maximum", apperror.FieldError{
			Field:   "totalAmount",
			Message: "exceeds maximum supported monetary amount",
		})
	}

	return PricingSnapshot{
		VatRateID:   vatRateID,
		VatRate:     rate,
		Subtotal:    normSubtotal,
		VatAmount:   vatAmount,
		TotalAmount: totalAmount,
		Currency:    curr,
	}, nil
}
