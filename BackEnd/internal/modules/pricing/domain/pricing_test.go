package domain_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/pricing/domain"
)

func TestCalculateQuote_ExactDecimalNoFloat(t *testing.T) {
	t.Parallel()

	vat0ID := uuid.New()
	vat8ID := uuid.New()
	vat10ID := uuid.New()

	tests := []struct {
		name         string
		subtotal     string
		vat          *domain.VATRate
		currency     string
		wantSubtotal string
		wantVAT      string
		wantTotal    string
	}{
		{
			name:         "no active VAT rate (nil) yields 0% tax",
			subtotal:     "1500000",
			vat:          nil,
			currency:     "VND",
			wantSubtotal: "1500000",
			wantVAT:      "0",
			wantTotal:    "1500000",
		},
		{
			name:     "VAT 0% explicit rate",
			subtotal: "2450000",
			vat: &domain.VATRate{
				ID:   vat0ID,
				Code: "VAT_0",
				Rate: decimal.RequireFromString("0.0000"),
			},
			currency:     "VND",
			wantSubtotal: "2450000",
			wantVAT:      "0",
			wantTotal:    "2450000",
		},
		{
			name:     "VAT 8% exact VND quote without service fee",
			subtotal: "1250000",
			vat: &domain.VATRate{
				ID:   vat8ID,
				Code: "VAT_8",
				Rate: decimal.RequireFromString("0.0800"),
			},
			currency:     "VND",
			wantSubtotal: "1250000",
			wantVAT:      "100000",
			wantTotal:    "1350000",
		},
		{
			name:     "VAT 8% VND half-up rounding on odd subtotal",
			subtotal: "125007",
			vat: &domain.VATRate{
				ID:   vat8ID,
				Code: "VAT_8",
				Rate: decimal.RequireFromString("0.0800"),
			},
			currency:     "VND",
			wantSubtotal: "125007",
			wantVAT:      "10001",
			wantTotal:    "135008",
		},
		{
			name:     "VAT 10% standard VND quote",
			subtotal: "3300000",
			vat: &domain.VATRate{
				ID:   vat10ID,
				Code: "VAT_10",
				Rate: decimal.RequireFromString("0.1000"),
			},
			currency:     "VND",
			wantSubtotal: "3300000",
			wantVAT:      "330000",
			wantTotal:    "3630000",
		},
		{
			name:     "VAT 10% USD rounds to 2 decimals without float drift",
			subtotal: "199.95",
			vat: &domain.VATRate{
				ID:   vat10ID,
				Code: "VAT_10",
				Rate: decimal.RequireFromString("0.1000"),
			},
			currency:     "USD",
			wantSubtotal: "199.95",
			wantVAT:      "20.00",
			wantTotal:    "219.95",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sub := decimal.RequireFromString(tc.subtotal)
			got, err := domain.CalculateQuote(sub, tc.vat, tc.currency)
			if err != nil {
				t.Fatalf("CalculateQuote unexpected error: %v", err)
			}
			if !got.Subtotal.Equal(decimal.RequireFromString(tc.wantSubtotal)) {
				t.Errorf("Subtotal = %s, want %s", got.Subtotal.String(), tc.wantSubtotal)
			}
			if !got.VatAmount.Equal(decimal.RequireFromString(tc.wantVAT)) {
				t.Errorf("VatAmount = %s, want %s", got.VatAmount.String(), tc.wantVAT)
			}
			if !got.TotalAmount.Equal(decimal.RequireFromString(tc.wantTotal)) {
				t.Errorf("TotalAmount = %s, want %s", got.TotalAmount.String(), tc.wantTotal)
			}
			if tc.vat != nil {
				if got.VatRateID == nil || *got.VatRateID != tc.vat.ID {
					t.Errorf("VatRateID = %v, want %v", got.VatRateID, tc.vat.ID)
				}
			} else if got.VatRateID != nil {
				t.Errorf("VatRateID = %v, want nil", got.VatRateID)
			}
		})
	}
}

func TestCalculateQuote_RejectsInvalidSubtotals(t *testing.T) {
	t.Parallel()

	vat10 := &domain.VATRate{
		ID:   uuid.New(),
		Code: "VAT_10",
		Rate: decimal.RequireFromString("0.10"),
	}

	if _, err := domain.CalculateQuote(decimal.RequireFromString("-100"), vat10, "VND"); err == nil {
		t.Fatal("expected error for negative subtotal")
	}
	if _, err := domain.CalculateQuote(decimal.RequireFromString("100.50"), vat10, "VND"); err == nil {
		t.Fatal("expected error for fractional VND subtotal")
	}
	if _, err := domain.CalculateQuote(decimal.RequireFromString("10.123"), vat10, "USD"); err == nil {
		t.Fatal("expected error for >2 decimal places")
	}
}
