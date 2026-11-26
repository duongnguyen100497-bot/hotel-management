-- 000006_create_rooms.up.sql
-- Room types catalog, physical rooms inventory, and immutable room operational state history.

CREATE TABLE room_types (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code              varchar(64) NOT NULL UNIQUE,
    name              text NOT NULL,
    description       text NULL,
    base_nightly_rate numeric(14,2) NOT NULL CHECK (base_nightly_rate > 0),
    currency          char(3) NOT NULL DEFAULT 'VND',
    capacity          smallint NOT NULL CHECK (capacity > 0),
    area_sqm          numeric(6,2) NULL CHECK (area_sqm IS NULL OR area_sqm > 0),
    bed_type          text NULL,
    amenities         jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(amenities) = 'array'),
    is_bookable       boolean NOT NULL DEFAULT true,
    archived_at       timestamptz NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    version           integer NOT NULL DEFAULT 1 CHECK (version >= 1)
);

CREATE INDEX idx_room_types_public_catalog
    ON room_types (base_nightly_rate, capacity)
    WHERE is_bookable = true AND archived_at IS NULL;

CREATE TABLE rooms (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code               varchar(32) NOT NULL UNIQUE,
    name               text NOT NULL,
    room_type_id       uuid NOT NULL REFERENCES room_types(id) ON DELETE RESTRICT,
    floor              smallint NOT NULL,
    capacity           smallint NOT NULL CHECK (capacity > 0),
    operational_status varchar(24) NOT NULL DEFAULT 'available'
        CHECK (operational_status IN ('available', 'occupied', 'cleaning', 'maintenance')),
    description        text NULL,
    archived_at        timestamptz NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    version            integer NOT NULL DEFAULT 1 CHECK (version >= 1)
);

CREATE INDEX idx_rooms_room_type_active
    ON rooms (room_type_id, operational_status)
    WHERE archived_at IS NULL;

CREATE TABLE room_state_history (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id       uuid NOT NULL REFERENCES rooms(id) ON DELETE RESTRICT,
    from_status   varchar(24) NOT NULL,
    to_status     varchar(24) NOT NULL,
    reason        text NULL,
    actor_user_id uuid NULL REFERENCES users(id) ON DELETE SET NULL,
    occurred_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_room_state_history_room_occurred
    ON room_state_history (room_id, occurred_at DESC);
