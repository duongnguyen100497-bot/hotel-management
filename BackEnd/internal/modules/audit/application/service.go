package application

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/audit/domain"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/apperror"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/clock"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/id"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/logging"
)

// Exported application-level aliases so other modules depend ONLY on audit/application.
type (
	AuditEntry  = domain.AuditEntry
	OutboxEvent = domain.OutboxEvent
)

// Recorder appends immutable audit entries. When called inside db.TxRunner.WithinTx,
// the write joins the active transaction and commits or rolls back atomically.
type Recorder interface {
	Record(ctx context.Context, entry AuditEntry) error
}

// OutboxEnqueuer persists domain events into outbox_events within the active DB transaction.
type OutboxEnqueuer interface {
	Enqueue(ctx context.Context, event OutboxEvent) error
}

// OutboxProcessor is consumed by the background notification worker to claim and settle outbox rows.
type OutboxProcessor interface {
	ClaimPending(ctx context.Context, now time.Time, limit int) ([]OutboxEvent, error)
	MarkProcessed(ctx context.Context, id uuid.UUID, processedAt time.Time) error
	MarkFailed(ctx context.Context, id uuid.UUID, lastErr string, nextAvailableAt time.Time) error
}

// Repository is the persistence port implemented by audit/persistence/postgres.
// Note: audit_events is strictly append-only (no update or delete operations exist).
type Repository interface {
	InsertAuditEvent(ctx context.Context, entry AuditEntry) error
	InsertOutboxEvent(ctx context.Context, event OutboxEvent) (OutboxEvent, error)
	ClaimPendingOutboxEvents(ctx context.Context, now time.Time, limit int) ([]OutboxEvent, error)
	MarkOutboxEventProcessed(ctx context.Context, id uuid.UUID, processedAt time.Time) error
	MarkOutboxEventFailed(ctx context.Context, id uuid.UUID, lastErr string, nextAvailableAt time.Time) error
}

// Service implements Recorder, OutboxEnqueuer, and OutboxProcessor.
type Service struct {
	repo Repository
	clk  clock.Clock
	ids  id.Generator
}

// NewService creates an audit & outbox application service.
func NewService(repo Repository, clk clock.Clock, ids id.Generator) *Service {
	if clk == nil {
		clk = clock.NewSystem()
	}
	if ids == nil {
		ids = id.NewUUIDGenerator()
	}
	return &Service{
		repo: repo,
		clk:  clk,
		ids:  ids,
	}
}

// Compile-time interface assertions.
var (
	_ Recorder        = (*Service)(nil)
	_ OutboxEnqueuer  = (*Service)(nil)
	_ OutboxProcessor = (*Service)(nil)
)

// Record validates, redacts sensitive fields, and appends an immutable audit event.
func (s *Service) Record(ctx context.Context, entry AuditEntry) error {
	if entry.Action == "" {
		return apperror.Validation("audit action is required")
	}
	if entry.ResourceType == "" {
		return apperror.Validation("audit resource_type is required")
	}
	if entry.ID == uuid.Nil {
		entry.ID = s.ids.NewUUID()
	}
	if entry.OccurredAt.IsZero() {
		entry.OccurredAt = s.clk.Now()
	}
	if entry.BeforeData != nil {
		entry.BeforeData = logging.RedactMap(entry.BeforeData)
	}
	if entry.AfterData != nil {
		entry.AfterData = logging.RedactMap(entry.AfterData)
	}
	return s.repo.InsertAuditEvent(ctx, entry)
}

// Enqueue validates and inserts a transactional outbox event.
func (s *Service) Enqueue(ctx context.Context, event OutboxEvent) error {
	if event.AggregateType == "" {
		return apperror.Validation("outbox aggregate_type is required")
	}
	if event.AggregateID == uuid.Nil {
		return apperror.Validation("outbox aggregate_id is required")
	}
	if event.EventType == "" {
		return apperror.Validation("outbox event_type is required")
	}
	if len(event.Payload) == 0 {
		event.Payload = json.RawMessage(`{}`)
	}
	if !json.Valid(event.Payload) {
		return apperror.Validation("outbox payload must be valid JSON")
	}

	now := s.clk.Now()
	if event.ID == uuid.Nil {
		event.ID = s.ids.NewUUID()
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = now
	}
	if event.AvailableAt.IsZero() {
		event.AvailableAt = now
	}
	if event.MaxAttempts <= 0 {
		event.MaxAttempts = 5
	}

	_, err := s.repo.InsertOutboxEvent(ctx, event)
	return err
}

// ClaimPending locks and returns up to limit due outbox events using FOR UPDATE SKIP LOCKED.
func (s *Service) ClaimPending(ctx context.Context, now time.Time, limit int) ([]OutboxEvent, error) {
	if limit <= 0 {
		limit = 10
	}
	return s.repo.ClaimPendingOutboxEvents(ctx, now, limit)
}

// MarkProcessed records successful outbox delivery.
func (s *Service) MarkProcessed(ctx context.Context, eventID uuid.UUID, processedAt time.Time) error {
	return s.repo.MarkOutboxEventProcessed(ctx, eventID, processedAt)
}

// MarkFailed increments attempts and schedules the next retry time.
func (s *Service) MarkFailed(ctx context.Context, eventID uuid.UUID, lastErr string, nextAvailableAt time.Time) error {
	return s.repo.MarkOutboxEventFailed(ctx, eventID, lastErr, nextAvailableAt)
}
