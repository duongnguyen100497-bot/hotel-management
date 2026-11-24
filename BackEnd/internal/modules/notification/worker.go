package notification

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/clock"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/db"
)

// HandlerFunc processes a single claimed outbox event.
type HandlerFunc func(ctx context.Context, event auditapp.OutboxEvent) error

// WorkerConfig configures polling, batching, and retry backoff for the outbox dispatcher.
type WorkerConfig struct {
	PollInterval time.Duration
	BatchSize    int
	BaseBackoff  time.Duration
	MaxBackoff   time.Duration
}

// Worker polls outbox_events using FOR UPDATE SKIP LOCKED and dispatches to registered handlers.
type Worker struct {
	txRunner db.TxRunner
	outbox   auditapp.OutboxProcessor
	clk      clock.Clock
	logger   *slog.Logger
	cfg      WorkerConfig

	mu       sync.RWMutex
	handlers map[string]HandlerFunc
}

// NewWorker constructs an Outbox Worker with sensible defaults.
func NewWorker(
	txRunner db.TxRunner,
	outbox auditapp.OutboxProcessor,
	clk clock.Clock,
	logger *slog.Logger,
	cfg WorkerConfig,
) *Worker {
	if clk == nil {
		clk = clock.NewSystem()
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 500 * time.Millisecond
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 10
	}
	if cfg.BaseBackoff <= 0 {
		cfg.BaseBackoff = 2 * time.Second
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = 5 * time.Minute
	}
	return &Worker{
		txRunner: txRunner,
		outbox:   outbox,
		clk:      clk,
		logger:   logger,
		cfg:      cfg,
		handlers: make(map[string]HandlerFunc),
	}
}

// Register binds a handler to an outbox event_type.
func (w *Worker) Register(eventType string, handler HandlerFunc) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.handlers[eventType] = handler
}

func (w *Worker) handlerFor(eventType string) (HandlerFunc, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	h, ok := w.handlers[eventType]
	return h, ok
}

// Run polls and dispatches outbox events until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	for {
		if _, err := w.ProcessOnce(ctx); err != nil && ctx.Err() == nil && w.logger != nil {
			w.logger.Error("outbox poll failed", slog.Any("error", err))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// ProcessOnce claims up to BatchSize due events in a single transaction using FOR UPDATE SKIP LOCKED,
// dispatches each to its handler, and marks each processed or failed with exponential backoff.
func (w *Worker) ProcessOnce(ctx context.Context) (int, error) {
	claimedCount := 0

	err := w.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		now := w.clk.Now()
		events, err := w.outbox.ClaimPending(txCtx, now, w.cfg.BatchSize)
		if err != nil {
			return err
		}
		claimedCount = len(events)

		for _, evt := range events {
			handler, ok := w.handlerFor(evt.EventType)
			var handleErr error
			if !ok {
				handleErr = fmt.Errorf("no outbox handler registered for event_type %q", evt.EventType)
			} else {
				handleErr = handler(txCtx, evt)
			}

			if handleErr == nil {
				if err := w.outbox.MarkProcessed(txCtx, evt.ID, w.clk.Now()); err != nil {
					return err
				}
				continue
			}

			nextAttempts := evt.Attempts + 1
			backoff := w.backoffDuration(evt.Attempts)
			nextAvailable := w.clk.Now().Add(backoff)

			if err := w.outbox.MarkFailed(txCtx, evt.ID, handleErr.Error(), nextAvailable); err != nil {
				return err
			}

			if nextAttempts >= evt.MaxAttempts {
				if w.logger != nil {
					w.logger.Error("outbox event dead_letter",
						slog.String("event_id", evt.ID.String()),
						slog.String("event_type", evt.EventType),
						slog.Int("attempts", nextAttempts),
						slog.Int("max_attempts", evt.MaxAttempts),
						slog.String("last_error", handleErr.Error()),
					)
				}
			} else if w.logger != nil {
				w.logger.Warn("outbox event failed; scheduled retry",
					slog.String("event_id", evt.ID.String()),
					slog.String("event_type", evt.EventType),
					slog.Int("attempts", nextAttempts),
					slog.Time("next_available_at", nextAvailable),
					slog.String("last_error", handleErr.Error()),
				)
			}
		}

		return nil
	})
	if err != nil {
		return 0, err
	}
	return claimedCount, nil
}

func (w *Worker) backoffDuration(priorAttempts int) time.Duration {
	if priorAttempts < 0 {
		priorAttempts = 0
	}
	d := w.cfg.BaseBackoff
	for i := 0; i < priorAttempts; i++ {
		if d >= w.cfg.MaxBackoff/2 {
			return w.cfg.MaxBackoff
		}
		d *= 2
	}
	if d > w.cfg.MaxBackoff {
		return w.cfg.MaxBackoff
	}
	return d
}
