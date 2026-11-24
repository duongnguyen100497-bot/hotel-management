package notification_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	auditpg "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/persistence/postgres"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/notification"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/clock"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/id"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/testutil"
)

func TestOutboxWorker_DispatchSuccessRetryBackoffAndDeadLetter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Testcontainers integration test in short mode")
	}

	env := testutil.StartPostgres(t)
	ctx := context.Background()

	start := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(start)
	auditSvc := auditapp.NewService(auditpg.NewRepository(env.Pool), clk, id.NewUUIDGenerator())

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

	worker := notification.NewWorker(env.TxRunner, auditSvc, clk, logger, notification.WorkerConfig{
		BatchSize:   10,
		BaseBackoff: 10 * time.Second,
		MaxBackoff:  60 * time.Second,
	})

	// 1. Successful dispatch marks processed_at and runs exactly once.
	var okCalls atomic.Int32
	worker.Register("booking.confirmed", func(_ context.Context, _ auditapp.OutboxEvent) error {
		okCalls.Add(1)
		return nil
	})

	okEventID := uuid.New()
	if err := auditSvc.Enqueue(ctx, auditapp.OutboxEvent{
		ID:            okEventID,
		AggregateType: "booking",
		AggregateID:   uuid.New(),
		EventType:     "booking.confirmed",
		Payload:       json.RawMessage(`{"code":"BK-001"}`),
	}); err != nil {
		t.Fatalf("enqueue ok event: %v", err)
	}

	n, err := worker.ProcessOnce(ctx)
	if err != nil {
		t.Fatalf("ProcessOnce failed: %v", err)
	}
	if n != 1 || okCalls.Load() != 1 {
		t.Fatalf("claimed=%d calls=%d, want 1 and 1", n, okCalls.Load())
	}

	var processedAt *time.Time
	if err := env.Pool.QueryRow(ctx, `SELECT processed_at FROM outbox_events WHERE id = $1`, okEventID).Scan(&processedAt); err != nil {
		t.Fatalf("query processed_at: %v", err)
	}
	if processedAt == nil {
		t.Fatal("expected processed_at to be set")
	}

	// Second poll should claim nothing.
	n2, err := worker.ProcessOnce(ctx)
	if err != nil || n2 != 0 {
		t.Fatalf("second poll claimed=%d err=%v, want 0 nil", n2, err)
	}

	// 2. Failing handler increments attempts, backs off available_at, and logs dead_letter at max_attempts.
	var failCalls atomic.Int32
	worker.Register("email.flaky", func(_ context.Context, _ auditapp.OutboxEvent) error {
		failCalls.Add(1)
		return errors.New("smtp connection refused")
	})

	flakyID := uuid.New()
	if err := auditSvc.Enqueue(ctx, auditapp.OutboxEvent{
		ID:            flakyID,
		AggregateType: "user",
		AggregateID:   uuid.New(),
		EventType:     "email.flaky",
		Payload:       json.RawMessage(`{"to":"guest@example.com"}`),
		MaxAttempts:   2,
	}); err != nil {
		t.Fatalf("enqueue flaky event: %v", err)
	}

	// Attempt 1 fails -> attempts=1, available_at = start + 10s
	if _, err := worker.ProcessOnce(ctx); err != nil {
		t.Fatalf("attempt 1 error: %v", err)
	}
	var attempts int
	var availableAt time.Time
	var lastErr string
	if err := env.Pool.QueryRow(ctx, `SELECT attempts, available_at, last_error FROM outbox_events WHERE id = $1`, flakyID).
		Scan(&attempts, &availableAt, &lastErr); err != nil {
		t.Fatalf("query flaky event after attempt 1: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
	if !availableAt.Equal(start.Add(10 * time.Second)) {
		t.Fatalf("available_at = %v, want %v", availableAt, start.Add(10*time.Second))
	}
	if !strings.Contains(lastErr, "smtp connection refused") {
		t.Fatalf("last_error = %q", lastErr)
	}

	// Polling before backoff elapses must NOT claim it.
	clk.Advance(5 * time.Second)
	if claimed, err := worker.ProcessOnce(ctx); err != nil || claimed != 0 {
		t.Fatalf("before backoff: claimed=%d err=%v, want 0 nil", claimed, err)
	}

	// Advance past backoff -> Attempt 2 hits max_attempts=2 and logs dead_letter.
	clk.Advance(6 * time.Second)
	if claimed, err := worker.ProcessOnce(ctx); err != nil || claimed != 1 {
		t.Fatalf("attempt 2: claimed=%d err=%v, want 1 nil", claimed, err)
	}
	if !strings.Contains(logBuf.String(), "dead_letter") {
		t.Errorf("expected dead_letter log entry, got: %s", logBuf.String())
	}

	// Further advances must never claim the dead-lettered event, nor block new events.
	clk.Advance(5 * time.Minute)
	newEventID := uuid.New()
	if err := auditSvc.Enqueue(ctx, auditapp.OutboxEvent{
		ID:            newEventID,
		AggregateType: "booking",
		AggregateID:   uuid.New(),
		EventType:     "booking.confirmed",
		Payload:       json.RawMessage(`{"code":"BK-002"}`),
	}); err != nil {
		t.Fatalf("enqueue second ok event: %v", err)
	}
	if claimed, err := worker.ProcessOnce(ctx); err != nil || claimed != 1 {
		t.Fatalf("after dead_letter: claimed=%d err=%v, want 1 nil", claimed, err)
	}
	if failCalls.Load() != 2 || okCalls.Load() != 2 {
		t.Fatalf("failCalls=%d okCalls=%d, want 2 and 2", failCalls.Load(), okCalls.Load())
	}
}

func TestOutboxWorker_ConcurrentWorkersSkipLockedNoDoubleProcessing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Testcontainers integration test in short mode")
	}

	env := testutil.StartPostgres(t)
	ctx := context.Background()

	clk := clock.NewFixed(time.Date(2026, 11, 24, 13, 0, 0, 0, time.UTC))
	auditSvc := auditapp.NewService(auditpg.NewRepository(env.Pool), clk, id.NewUUIDGenerator())

	const totalEvents = 6
	for i := 0; i < totalEvents; i++ {
		if err := auditSvc.Enqueue(ctx, auditapp.OutboxEvent{
			AggregateType: "booking",
			AggregateID:   uuid.New(),
			EventType:     "booking.concurrent",
			Payload:       json.RawMessage(`{}`),
		}); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}

	var mu sync.Mutex
	seen := make(map[uuid.UUID]int)
	var processed atomic.Int32

	makeWorker := func() *notification.Worker {
		w := notification.NewWorker(env.TxRunner, auditSvc, clk, nil, notification.WorkerConfig{
			BatchSize: 1,
		})
		w.Register("booking.concurrent", func(_ context.Context, evt auditapp.OutboxEvent) error {
			// Hold the transaction briefly so both workers overlap on SELECT ... FOR UPDATE SKIP LOCKED.
			time.Sleep(30 * time.Millisecond)
			mu.Lock()
			seen[evt.ID]++
			mu.Unlock()
			processed.Add(1)
			return nil
		})
		return w
	}

	w1 := makeWorker()
	w2 := makeWorker()

	var wg sync.WaitGroup
	runWorker := func(w *notification.Worker) {
		defer wg.Done()
		for {
			n, err := w.ProcessOnce(ctx)
			if err != nil {
				t.Errorf("worker ProcessOnce error: %v", err)
				return
			}
			if n == 0 {
				return
			}
		}
	}

	wg.Add(2)
	go runWorker(w1)
	go runWorker(w2)
	wg.Wait()

	if processed.Load() != totalEvents {
		t.Fatalf("processed = %d, want %d", processed.Load(), totalEvents)
	}
	if len(seen) != totalEvents {
		t.Fatalf("distinct events = %d, want %d", len(seen), totalEvents)
	}
	for id, count := range seen {
		if count != 1 {
			t.Errorf("event %s processed %d times, want 1", id, count)
		}
	}
}
