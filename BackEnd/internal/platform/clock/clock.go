package clock

import (
	"sync"
	"time"
)

// Clock abstracts wall-clock time so domain services can be tested deterministically.
type Clock interface {
	Now() time.Time
}

// SystemClock returns the current UTC time.
type SystemClock struct{}

// NewSystem returns a Clock backed by time.Now().UTC().
func NewSystem() Clock {
	return SystemClock{}
}

// Now returns the current UTC time.
func (SystemClock) Now() time.Time {
	return time.Now().UTC()
}

// FixedClock is a thread-safe mutable clock for deterministic unit tests.
type FixedClock struct {
	mu  sync.RWMutex
	now time.Time
}

// NewFixed constructs a FixedClock initialized to t in UTC.
func NewFixed(t time.Time) *FixedClock {
	return &FixedClock{now: t.UTC()}
}

// Now returns the fixed timestamp.
func (c *FixedClock) Now() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.now
}

// Set updates the clock's timestamp.
func (c *FixedClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t.UTC()
}

// Advance moves the clock forward by d.
func (c *FixedClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}
