-- 000009_create_bookings_holds.up.sql
-- Booking groups, individual bookings with GiST exclusion anti-double-booking constraint,
-- immutable representative guest snapshots, booking status history, and authoritative DB reservation holds.

CREATE TABLE booking_groups (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    reference_code  varchar(32) NOT NULL UNIQUE,
    customer_id     uuid NOT NULL REFERENCES customers(id) ON DELETE RESTRICT,
    status          varchar(24) NOT NULL
        CHECK (status IN (
            'holding',
            'pending',
            'partially_confirmed',
            'confirmed',
            'partially_cancelled',
            'cancelled',
            'expired',
            'completed'
        )),
    expires_at      timestamptz NULL,
    idempotency_key varchar(128) NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    version         integer NOT NULL DEFAULT 1 CHECK (version >= 1)
);

CREATE UNIQUE INDEX uq_booking_groups_customer_idempotency
    ON booking_groups (customer_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE INDEX idx_booking_groups_customer_created
    ON booking_groups (customer_id, created_at DESC);

CREATE TABLE bookings (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    reference_code        varchar(32) NOT NULL UNIQUE,
    booking_group_id      uuid NULL REFERENCES booking_groups(id) ON DELETE RESTRICT,
    customer_id           uuid NOT NULL REFERENCES customers(id) ON DELETE RESTRICT,
    room_id               uuid NOT NULL REFERENCES rooms(id) ON DELETE RESTRICT,
    room_type_id          uuid NOT NULL REFERENCES room_types(id) ON DELETE RESTRICT,
    stay                  daterange NOT NULL
        CHECK (lower(stay) < upper(stay) AND lower_inc(stay) AND NOT upper_inc(stay)),
    adults                smallint NOT NULL CHECK (adults > 0),
    children              smallint NOT NULL DEFAULT 0 CHECK (children >= 0),
    status                varchar(24) NOT NULL
        CHECK (status IN ('pending', 'confirmed', 'checked_in', 'checked_out', 'cancelled', 'expired')),
    nightly_rate_snapshot numeric(14,2) NOT NULL CHECK (nightly_rate_snapshot > 0),
    vat_rate_id           uuid NOT NULL REFERENCES vat_rates(id) ON DELETE RESTRICT,
    vat_rate_snapshot     numeric(5,4) NOT NULL CHECK (vat_rate_snapshot >= 0 AND vat_rate_snapshot <= 1),
    subtotal_amount       numeric(14,2) NOT NULL CHECK (subtotal_amount >= 0),
    vat_amount            numeric(14,2) NOT NULL CHECK (vat_amount >= 0),
    total_amount          numeric(14,2) NOT NULL CHECK (total_amount >= 0),
    currency              char(3) NOT NULL DEFAULT 'VND',
    special_request       text NULL,
    expires_at            timestamptz NULL,
    cancelled_at          timestamptz NULL,
    cancel_reason         text NULL,
    idempotency_key       varchar(128) NULL,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    version               integer NOT NULL DEFAULT 1 CHECK (version >= 1)
);

ALTER TABLE bookings ADD CONSTRAINT bookings_no_room_overlap
    EXCLUDE USING gist (room_id WITH =, stay WITH &&)
    WHERE (status IN ('pending', 'confirmed', 'checked_in'));

CREATE UNIQUE INDEX uq_bookings_customer_idempotency
    ON bookings (customer_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE INDEX idx_bookings_customer_created
    ON bookings (customer_id, created_at DESC);

CREATE INDEX idx_bookings_group_id
    ON bookings (booking_group_id)
    WHERE booking_group_id IS NOT NULL;

CREATE INDEX idx_bookings_pending_expires
    ON bookings (expires_at)
    WHERE status = 'pending' AND expires_at IS NOT NULL;

CREATE INDEX idx_bookings_status_created
    ON bookings (status, created_at DESC);

CREATE TABLE booking_guest_snapshots (
    booking_id       uuid PRIMARY KEY REFERENCES bookings(id) ON DELETE CASCADE,
    full_name        text NOT NULL,
    phone            text NOT NULL,
    email            text NULL,
    nationality_code varchar(8) NULL,
    special_request  text NULL,
    created_at       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE booking_status_history (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    booking_id     uuid NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
    from_status    varchar(24) NULL,
    to_status      varchar(24) NOT NULL,
    actor_user_id  uuid NULL REFERENCES users(id) ON DELETE SET NULL,
    reason         text NULL,
    correlation_id uuid NULL,
    occurred_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_booking_status_history_booking
    ON booking_status_history (booking_id, occurred_at ASC);

CREATE TABLE reservation_holds (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id      uuid NOT NULL REFERENCES customers(id) ON DELETE RESTRICT,
    booking_group_id uuid NULL,
    room_id          uuid NOT NULL REFERENCES rooms(id) ON DELETE RESTRICT,
    room_type_id     uuid NOT NULL REFERENCES room_types(id) ON DELETE RESTRICT,
    stay             daterange NOT NULL
        CHECK (lower(stay) < upper(stay) AND lower_inc(stay) AND NOT upper_inc(stay)),
    hold_mode        varchar(16) NOT NULL CHECK (hold_mode IN ('individual', 'group')),
    hold_token       varchar(64) NOT NULL,
    redis_key        text NOT NULL,
    ttl_seconds      integer NOT NULL CHECK (ttl_seconds > 0),
    status           varchar(24) NOT NULL CHECK (status IN ('active', 'converted', 'released', 'expired')),
    expires_at       timestamptz NOT NULL,
    released_reason  text NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_reservation_holds_status_expires
    ON reservation_holds (status, expires_at);

CREATE INDEX idx_reservation_holds_group
    ON reservation_holds (booking_group_id)
    WHERE booking_group_id IS NOT NULL;

CREATE INDEX idx_reservation_holds_room_stay_active
    ON reservation_holds USING gist (room_id, stay)
    WHERE status = 'active';
