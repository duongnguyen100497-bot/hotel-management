// Package money is the single place that defines how monetary amounts are
// represented: shopspring/decimal values matching PostgreSQL numeric(14,2).
// Never use float64 for money.
package money

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// Scale is the number of fractional digits stored for money (numeric(14,2)).
const Scale = 2

// maxAmount is the largest value numeric(14,2) can hold: 12 integer digits.
var maxAmount = decimal.RequireFromString("999999999999.99")

// Parse converts a decimal string into a money amount. It rejects values with
// more than 2 fractional digits or outside the numeric(14,2) range, instead of
// silently rounding.
func Parse(s string) (decimal.Decimal, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("invalid money amount %q: %w", s, err)
	}
	if !d.Equal(d.Round(Scale)) {
		return decimal.Decimal{}, fmt.Errorf("money amount %q has more than %d decimal places", s, Scale)
	}
	if d.Abs().GreaterThan(maxAmount) {
		return decimal.Decimal{}, fmt.Errorf("money amount %q exceeds numeric(14,2) range", s)
	}
	return d, nil
}
