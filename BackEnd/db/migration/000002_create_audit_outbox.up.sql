CREATE TABLE audit_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    occurred_at timestamptz NOT NULL DEFAULT now(),
    actor_user_id uuid,
    action varchar(100) NOT NULL,
    resource_type varchar(100) NOT NULL,
    resource_id uuid,
    request_id uuid,
    correlation_id uuid,
    ip_hash bytea,
    reason text,
    before_data jsonb,
    after_data jsonb
);

CREATE INDEX idx_audit_events_occurred_at_brin ON audit_events USING brin (occurred_at);
CREATE INDEX idx_audit_events_resource ON audit_events (resource_type, resource_id);

CREATE TABLE outbox_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type varchar(100) NOT NULL,
    aggregate_id uuid NOT NULL,
    event_type varchar(100) NOT NULL,
    payload jsonb NOT NULL,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    available_at timestamptz NOT NULL DEFAULT now(),
    processed_at timestamptz,
    attempts int NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    max_attempts int NOT NULL DEFAULT 5 CHECK (max_attempts > 0),
    last_error text
);

CREATE INDEX idx_outbox_events_pending_available ON outbox_events (available_at) WHERE processed_at IS NULL;
