-- 000008_create_media.up.sql
-- Media assets backed by MinIO object storage and room / room_type image associations.

CREATE TABLE media_assets (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    object_key  text NOT NULL UNIQUE,
    sha256      bytea NULL,
    mime_type   varchar(64) NULL,
    byte_size   bigint NULL CHECK (byte_size IS NULL OR byte_size > 0),
    width       integer NULL CHECK (width IS NULL OR width > 0),
    height      integer NULL CHECK (height IS NULL OR height > 0),
    status      varchar(24) NOT NULL
        CHECK (status IN ('pending', 'ready', 'deletion_pending', 'deleted')),
    uploaded_by uuid NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_media_assets_pending_created
    ON media_assets (created_at ASC)
    WHERE status = 'pending';

CREATE TABLE room_media (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id        uuid NULL REFERENCES rooms(id) ON DELETE CASCADE,
    room_type_id   uuid NULL REFERENCES room_types(id) ON DELETE CASCADE,
    media_asset_id uuid NOT NULL REFERENCES media_assets(id) ON DELETE CASCADE,
    sort_order     smallint NOT NULL DEFAULT 0,
    is_cover       boolean NOT NULL DEFAULT false,
    created_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chk_room_media_exclusive_owner
        CHECK ((room_id IS NOT NULL) <> (room_type_id IS NOT NULL))
);

CREATE UNIQUE INDEX uq_room_media_room_asset
    ON room_media (room_id, media_asset_id)
    WHERE room_id IS NOT NULL;

CREATE UNIQUE INDEX uq_room_media_room_type_asset
    ON room_media (room_type_id, media_asset_id)
    WHERE room_type_id IS NOT NULL;

CREATE UNIQUE INDEX uq_room_media_single_room_cover
    ON room_media (room_id)
    WHERE room_id IS NOT NULL AND is_cover = true;

CREATE UNIQUE INDEX uq_room_media_single_room_type_cover
    ON room_media (room_type_id)
    WHERE room_type_id IS NOT NULL AND is_cover = true;

CREATE INDEX idx_room_media_room_sort
    ON room_media (room_id, is_cover DESC, sort_order ASC, created_at ASC)
    WHERE room_id IS NOT NULL;

CREATE INDEX idx_room_media_room_type_sort
    ON room_media (room_type_id, is_cover DESC, sort_order ASC, created_at ASC)
    WHERE room_type_id IS NOT NULL;

ALTER TABLE hotel_settings
    ADD CONSTRAINT fk_hotel_settings_logo_media
    FOREIGN KEY (logo_media_id) REFERENCES media_assets(id) ON DELETE SET NULL;
