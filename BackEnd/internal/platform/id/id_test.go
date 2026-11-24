package id_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/id"
)

func TestUUIDGenerator(t *testing.T) {
	g := id.NewUUIDGenerator()
	a := g.NewUUID()
	b := g.NewUUID()
	if a == uuid.Nil || b == uuid.Nil || a == b {
		t.Fatalf("expected distinct non-nil UUIDs, got %s and %s", a, b)
	}
}

func TestSequenceGenerator(t *testing.T) {
	first := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	second := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	g := id.NewSequence(first, second)

	if got := g.NewUUID(); got != first {
		t.Fatalf("first = %s, want %s", got, first)
	}
	if got := g.NewUUID(); got != second {
		t.Fatalf("second = %s, want %s", got, second)
	}
	if got := g.NewUUID(); got == uuid.Nil {
		t.Fatal("fallback UUID should not be nil")
	}
}
