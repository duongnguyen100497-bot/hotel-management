package money_test

import (
	"testing"

	"github.com/trandinh0506/hotel-management/backend/internal/platform/money"
)

func TestParse(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "1500000", want: "1500000"},
		{in: "0.10", want: "0.1"},
		{in: "999999999999.99", want: "999999999999.99"},
		{in: "-20.5", want: "-20.5"},
		{in: "0.105", wantErr: true},
		{in: "1000000000000", wantErr: true},
		{in: "abc", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := money.Parse(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q) = %s, want error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tc.in, err)
			}
			if got.String() != tc.want {
				t.Fatalf("Parse(%q) = %s, want %s", tc.in, got, tc.want)
			}
		})
	}

	// 0.1 + 0.2 must be exactly 0.3 (the classic float64 failure).
	a, _ := money.Parse("0.1")
	b, _ := money.Parse("0.2")
	if got := a.Add(b).String(); got != "0.3" {
		t.Fatalf("0.1 + 0.2 = %s, want 0.3", got)
	}
}
