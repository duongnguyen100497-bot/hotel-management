-- name: InsertAuditEvent :one
INSERT INTO audit_events (
    id,
    occurred_at,
    actor_user_id,
    action,
    resource_type,
    resource_id,
    request_id,
    correlation_id,
    ip_hash,
    reason,
    before_data,
    after_data
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
)
RETURNING id, occurred_at;

-- name: InsertOutboxEvent :one
INSERT INTO outbox_events (
    id,
    aggregate_type,
    aggregate_id,
    event_type,
    payload,
    occurred_at,
    available_at,
    max_attempts
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING id, aggregate_type, aggregate_id, event_type, payload, occurred_at, available_at, processed_at, attempts, max_attempts, last_error;

-- name: ClaimPendingOutboxEvents :many
SELECT
    id,
    aggregate_type,
    aggregate_id,
    event_type,
    payload,
    occurred_at,
    available_at,
    processed_at,
    attempts,
    max_attempts,
    last_error
FROM outbox_events
WHERE processed_at IS NULL
  AND available_at <= $1
  AND attempts < max_attempts
ORDER BY available_at ASC, occurred_at ASC
LIMIT $2
FOR UPDATE SKIP LOCKED;

-- name: MarkOutboxEventProcessed :exec
UPDATE outbox_events
SET processed_at = $2,
    last_error = NULL
WHERE id = $1;

-- name: MarkOutboxEventFailed :exec
UPDATE outbox_events
SET attempts = attempts + 1,
    last_error = $2,
    available_at = $3
WHERE id = $1;
