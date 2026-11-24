package id

import (
	"sync"

	"github.com/google/uuid"
)

// Generator creates unique identifiers for domain aggregates and events.
type Generator interface {
	NewUUID() uuid.UUID
}

// IDGenerator is an alias for Generator.
type IDGenerator = Generator

// UUIDGenerator generates random UUIDv4 identifiers.
type UUIDGenerator struct{}

// NewUUIDGenerator returns a production UUIDv4 generator.
func NewUUIDGenerator() Generator {
	return UUIDGenerator{}
}

// NewUUID returns a new random UUIDv4.
func (UUIDGenerator) NewUUID() uuid.UUID {
	return uuid.New()
}

// SequenceGenerator yields deterministic UUIDs in order for tests, falling back to uuid.New() when exhausted.
type SequenceGenerator struct {
	mu  sync.Mutex
	ids []uuid.UUID
	idx int
}

// NewSequence constructs a deterministic SequenceGenerator.
func NewSequence(ids ...uuid.UUID) *SequenceGenerator {
	cp := make([]uuid.UUID, len(ids))
	copy(cp, ids)
	return &SequenceGenerator{ids: cp}
}

// NewUUID returns the next configured UUID or a fresh random UUID once exhausted.
func (s *SequenceGenerator) NewUUID() uuid.UUID {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.idx < len(s.ids) {
		out := s.ids[s.idx]
		s.idx++
		return out
	}
	return uuid.New()
}
