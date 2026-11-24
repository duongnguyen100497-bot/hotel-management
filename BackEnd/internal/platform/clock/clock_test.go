package clock_test

import (
	"testing"
	"time"

	"github.com/trandinh0506/hotel-management/backend/internal/platform/clock"
)

func TestFixedClock(t *testing.T) {
	start := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	fc := clock.NewFixed(start)

	if !fc.Now().Equal(start) {
		t.Fatalf("Now() = %v, want %v", fc.Now(), start)
	}
	fc.Advance(15 * time.Minute)
	if !fc.Now().Equal(start.Add(15 * time.Minute)) {
		t.Fatalf("after Advance, Now() = %v", fc.Now())
	}
	next := start.Add(2 * time.Hour)
	fc.Set(next)
	if !fc.Now().Equal(next) {
		t.Fatalf("after Set, Now() = %v", fc.Now())
	}
}

func TestSystemClock(t *testing.T) {
	sc := clock.NewSystem()
	if sc.Now().IsZero() {
		t.Fatal("SystemClock returned zero time")
	}
}
