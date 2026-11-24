package platformhttp

import (
	"context"
	"testing"
	"time"
)

func TestInMemoryRateLimiter_SweepsExpiredKeys(t *testing.T) {
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	l := NewInMemoryRateLimiter(1, time.Minute)
	l.nowFunc = func() time.Time { return now }

	for _, ip := range []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"} {
		if ok, _, _ := l.Allow(context.Background(), ip); !ok {
			t.Fatalf("first request from %s should be allowed", ip)
		}
	}
	if ok, retry, _ := l.Allow(context.Background(), "10.0.0.1"); ok || retry != time.Minute {
		t.Fatalf("second request: allowed=%v retry=%v, want false 1m", ok, retry)
	}

	// After the window passes, one request from a new key sweeps all expired keys.
	now = now.Add(time.Minute)
	if ok, _, _ := l.Allow(context.Background(), "10.0.0.9"); !ok {
		t.Fatal("request after window should be allowed")
	}
	if got := len(l.entries); got != 1 {
		t.Fatalf("entries after sweep = %d, want 1", got)
	}
}
