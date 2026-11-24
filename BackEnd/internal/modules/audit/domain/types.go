package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// AuditEntry represents an immutable append-only audit log record.
type AuditEntry struct {
	ID            uuid.UUID
	OccurredAt    time.Time
	ActorUserID   *uuid.UUID
	Action        string
	ResourceType  string
	ResourceID    *uuid.UUID
	RequestID     *uuid.UUID
	CorrelationID *uuid.UUID
	IPHash        []byte
	Reason        string
	BeforeData    map[string]any
	AfterData     map[string]any
}

// OutboxEvent represents a transactional outbox message persisted in the same DB transaction
// as the triggering business write.
type OutboxEvent struct {
	ID            uuid.UUID
	AggregateType string
	AggregateID   uuid.UUID
	EventType     string
	Payload       json.RawMessage
	OccurredAt    time.Time
	AvailableAt   time.Time
	ProcessedAt   *time.Time
	Attempts      int
	MaxAttempts   int
	LastError     string
}
