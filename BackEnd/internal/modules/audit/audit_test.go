package audit_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	auditapp "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/application"
	auditpg "github.com/trandinh0506/hotel-management/backend/internal/modules/audit/persistence/postgres"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/clock"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/id"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/logging"
	"github.com/trandinh0506/hotel-management/backend/internal/platform/testutil"
)

func TestAuditAndOutbox_TransactionalCommitRollbackAndRedaction(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Testcontainers integration test in short mode")
	}

	env := testutil.StartPostgres(t)
	ctx := context.Background()

	fixedNow := time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC)
	svc := auditapp.NewService(auditpg.NewRepository(env.Pool), clock.NewFixed(fixedNow), id.NewUUIDGenerator())

	actorID := uuid.New()
	resourceID := uuid.New()
	reqID := uuid.New()

	// 1. Rollback discards both audit_events and outbox_events atomically.
	rollbackErr := errors.New("business failure")
	err := env.TxRunner.WithinTx(ctx, func(txCtx context.Context) error {
		if err := svc.Record(txCtx, auditapp.AuditEntry{
			ActorUserID:  &actorID,
			Action:       "booking.created",
			ResourceType: "booking",
			ResourceID:   &resourceID,
			RequestID:    &reqID,
			AfterData:    map[string]any{"status": "pending"},
		}); err != nil {
			return err
		}
		if err := svc.Enqueue(txCtx, auditapp.OutboxEvent{
			AggregateType: "booking",
			AggregateID:   resourceID,
			EventType:     "booking.confirmed",
			Payload:       json.RawMessage(`{"bookingId":"test"}`),
		}); err != nil {
			return err
		}
		return rollbackErr
	})
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("expected rollbackErr, got %v", err)
	}

	var auditCount, outboxCount int
	if err := env.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_events`).Scan(&auditCount); err != nil {
		t.Fatalf("count audit_events: %v", err)
	}
	if err := env.Pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events`).Scan(&outboxCount); err != nil {
		t.Fatalf("count outbox_events: %v", err)
	}
	if auditCount != 0 || outboxCount != 0 {
		t.Fatalf("expected 0 rows after rollback, got audit=%d outbox=%d", auditCount, outboxCount)
	}

	// 2. Commit persists both records and redacts sensitive fields in audit_events.
	err = env.TxRunner.WithinTx(ctx, func(txCtx context.Context) error {
		if err := svc.Record(txCtx, auditapp.AuditEntry{
			ActorUserID:  &actorID,
			Action:       "employee.created",
			ResourceType: "employee",
			ResourceID:   &resourceID,
			RequestID:    &reqID,
			Reason:       "onboarding",
			BeforeData:   map[string]any{"password": "old-password", "role": "guest"},
			AfterData: map[string]any{
				"role":             "receptionist",
				"government_id":    "012345678901",
				"activation_token": "tok_secret_xyz",
			},
		}); err != nil {
			return err
		}
		return svc.Enqueue(txCtx, auditapp.OutboxEvent{
			AggregateType: "employee",
			AggregateID:   resourceID,
			EventType:     "identity.employee_activation_requested",
			Payload:       json.RawMessage(`{"employeeId":"123"}`),
		})
	})
	if err != nil {
		t.Fatalf("commit tx failed: %v", err)
	}

	var beforeRaw, afterRaw string
	if err := env.Pool.QueryRow(ctx, `SELECT before_data::text, after_data::text FROM audit_events LIMIT 1`).Scan(&beforeRaw, &afterRaw); err != nil {
		t.Fatalf("query audit_events: %v", err)
	}
	for _, secret := range []string{"old-password", "012345678901", "tok_secret_xyz"} {
		if strings.Contains(beforeRaw, secret) || strings.Contains(afterRaw, secret) {
			t.Fatalf("audit payload leaked secret %q: before=%s after=%s", secret, beforeRaw, afterRaw)
		}
	}
	if !strings.Contains(afterRaw, logging.RedactedValue) {
		t.Errorf("expected redacted marker in after_data, got %s", afterRaw)
	}
	if !strings.Contains(afterRaw, "receptionist") {
		t.Errorf("expected non-sensitive field preserved in after_data, got %s", afterRaw)
	}
}
