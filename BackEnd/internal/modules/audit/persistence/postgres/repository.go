package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/audit/domain"
	"github.com/trandinh0506/hotel-management/backend/internal/modules/audit/persistence/postgres/sqlcgen"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/db"
)

// Repository persists append-only audit_events and transactional outbox_events in PostgreSQL.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a PostgreSQL audit/outbox repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

var _ auditapp.Repository = (*Repository)(nil)

func (r *Repository) queries(ctx context.Context) *sqlcgen.Queries {
	return sqlcgen.New(db.Querier(ctx, r.pool))
}

func (r *Repository) InsertAuditEvent(ctx context.Context, entry domain.AuditEntry) error {
	var beforeJSON, afterJSON []byte
	var err error
	if entry.BeforeData != nil {
		beforeJSON, err = json.Marshal(entry.BeforeData)
		if err != nil {
			return fmt.Errorf("marshal before_data: %w", err)
		}
	}
	if entry.AfterData != nil {
		afterJSON, err = json.Marshal(entry.AfterData)
		if err != nil {
			return fmt.Errorf("marshal after_data: %w", err)
		}
	}

	var reason pgtype.Text
	if entry.Reason != "" {
		reason = pgtype.Text{String: entry.Reason, Valid: true}
	}

	_, err = r.queries(ctx).InsertAuditEvent(ctx, sqlcgen.InsertAuditEventParams{
		ID:            entry.ID,
		OccurredAt:    entry.OccurredAt,
		ActorUserID:   entry.ActorUserID,
		Action:        entry.Action,
		ResourceType:  entry.ResourceType,
		ResourceID:    entry.ResourceID,
		RequestID:     entry.RequestID,
		CorrelationID: entry.CorrelationID,
		IpHash:        entry.IPHash,
		Reason:        reason,
		BeforeData:    beforeJSON,
		AfterData:     afterJSON,
	})
	return db.MapPgError(err)
}

func (r *Repository) InsertOutboxEvent(ctx context.Context, event domain.OutboxEvent) (domain.OutboxEvent, error) {
	row, err := r.queries(ctx).InsertOutboxEvent(ctx, sqlcgen.InsertOutboxEventParams{
		ID:            event.ID,
		AggregateType: event.AggregateType,
		AggregateID:   event.AggregateID,
		EventType:     event.EventType,
		Payload:       []byte(event.Payload),
		OccurredAt:    event.OccurredAt,
		AvailableAt:   event.AvailableAt,
		MaxAttempts:   int32(event.MaxAttempts),
	})
	if err != nil {
		return domain.OutboxEvent{}, db.MapPgError(err)
	}
	return mapOutboxRow(row), nil
}

func (r *Repository) ClaimPendingOutboxEvents(ctx context.Context, now time.Time, limit int) ([]domain.OutboxEvent, error) {
	rows, err := r.queries(ctx).ClaimPendingOutboxEvents(ctx, sqlcgen.ClaimPendingOutboxEventsParams{
		AvailableAt: now,
		Limit:       int32(limit),
	})
	if err != nil {
		return nil, db.MapPgError(err)
	}
	out := make([]domain.OutboxEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapOutboxRow(row))
	}
	return out, nil
}

func (r *Repository) MarkOutboxEventProcessed(ctx context.Context, eventID uuid.UUID, processedAt time.Time) error {
	ts := processedAt.UTC()
	err := r.queries(ctx).MarkOutboxEventProcessed(ctx, sqlcgen.MarkOutboxEventProcessedParams{
		ID:          eventID,
		ProcessedAt: &ts,
	})
	return db.MapPgError(err)
}

func (r *Repository) MarkOutboxEventFailed(ctx context.Context, eventID uuid.UUID, lastErr string, nextAvailableAt time.Time) error {
	err := r.queries(ctx).MarkOutboxEventFailed(ctx, sqlcgen.MarkOutboxEventFailedParams{
		ID:          eventID,
		LastError:   pgtype.Text{String: lastErr, Valid: true},
		AvailableAt: nextAvailableAt.UTC(),
	})
	return db.MapPgError(err)
}

func mapOutboxRow(row sqlcgen.OutboxEvent) domain.OutboxEvent {
	lastErr := ""
	if row.LastError.Valid {
		lastErr = row.LastError.String
	}
	return domain.OutboxEvent{
		ID:            row.ID,
		AggregateType: row.AggregateType,
		AggregateID:   row.AggregateID,
		EventType:     row.EventType,
		Payload:       json.RawMessage(row.Payload),
		OccurredAt:    row.OccurredAt,
		AvailableAt:   row.AvailableAt,
		ProcessedAt:   row.ProcessedAt,
		Attempts:      int(row.Attempts),
		MaxAttempts:   int(row.MaxAttempts),
		LastError:     lastErr,
	}
}
