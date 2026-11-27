-- name: CreatePendingMediaAsset :one
INSERT INTO media_assets (
    id,
    object_key,
    status,
    uploaded_by,
    created_at,
    updated_at
) VALUES (
    $1, $2, 'pending', $3, $4, $4
)
RETURNING
    id,
    object_key,
    sha256,
    mime_type,
    byte_size,
    width,
    height,
    status,
    uploaded_by,
    created_at,
    updated_at;

-- name: GetMediaAssetByID :one
SELECT
    id,
    object_key,
    sha256,
    mime_type,
    byte_size,
    width,
    height,
    status,
    uploaded_by,
    created_at,
    updated_at
FROM media_assets
WHERE id = $1;

-- name: GetMediaAssetByIDForUpdate :one
SELECT
    id,
    object_key,
    sha256,
    mime_type,
    byte_size,
    width,
    height,
    status,
    uploaded_by,
    created_at,
    updated_at
FROM media_assets
WHERE id = $1
FOR UPDATE;

-- name: MarkMediaAssetReady :one
UPDATE media_assets
SET
    sha256     = sqlc.arg(sha256),
    mime_type  = sqlc.arg(mime_type),
    byte_size  = sqlc.arg(byte_size),
    width      = sqlc.arg(width),
    height     = sqlc.arg(height),
    status     = 'ready',
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id)
RETURNING
    id,
    object_key,
    sha256,
    mime_type,
    byte_size,
    width,
    height,
    status,
    uploaded_by,
    created_at,
    updated_at;

-- name: MarkMediaAssetDeletionPending :one
UPDATE media_assets
SET
    status     = 'deletion_pending',
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id)
RETURNING
    id,
    object_key,
    sha256,
    mime_type,
    byte_size,
    width,
    height,
    status,
    uploaded_by,
    created_at,
    updated_at;

-- name: MarkMediaAssetDeleted :exec
UPDATE media_assets
SET
    status     = 'deleted',
    updated_at = $2
WHERE id = $1;

-- name: DeleteAllRoomMediaForAsset :exec
DELETE FROM room_media
WHERE media_asset_id = $1;

-- name: ClearRoomCover :exec
UPDATE room_media
SET is_cover = false
WHERE room_id = $1
  AND is_cover = true;

-- name: ClearRoomTypeCover :exec
UPDATE room_media
SET is_cover = false
WHERE room_type_id = $1
  AND is_cover = true;

-- name: UpsertRoomMediaLink :one
INSERT INTO room_media (
    id,
    room_id,
    media_asset_id,
    sort_order,
    is_cover,
    created_at
) VALUES (
    $1, $2, $3, $4, $5, $6
)
ON CONFLICT (room_id, media_asset_id) WHERE room_id IS NOT NULL
DO UPDATE SET
    sort_order = EXCLUDED.sort_order,
    is_cover   = EXCLUDED.is_cover
RETURNING
    id,
    room_id,
    room_type_id,
    media_asset_id,
    sort_order,
    is_cover,
    created_at;

-- name: UpsertRoomTypeMediaLink :one
INSERT INTO room_media (
    id,
    room_type_id,
    media_asset_id,
    sort_order,
    is_cover,
    created_at
) VALUES (
    $1, $2, $3, $4, $5, $6
)
ON CONFLICT (room_type_id, media_asset_id) WHERE room_type_id IS NOT NULL
DO UPDATE SET
    sort_order = EXCLUDED.sort_order,
    is_cover   = EXCLUDED.is_cover
RETURNING
    id,
    room_id,
    room_type_id,
    media_asset_id,
    sort_order,
    is_cover,
    created_at;

-- name: GetRoomMediaLink :one
SELECT
    id,
    room_id,
    room_type_id,
    media_asset_id,
    sort_order,
    is_cover,
    created_at
FROM room_media
WHERE room_id = $1
  AND media_asset_id = $2;

-- name: GetRoomTypeMediaLink :one
SELECT
    id,
    room_id,
    room_type_id,
    media_asset_id,
    sort_order,
    is_cover,
    created_at
FROM room_media
WHERE room_type_id = $1
  AND media_asset_id = $2;

-- name: ListExpiredPendingMediaAssets :many
SELECT
    id,
    object_key,
    created_at
FROM media_assets
WHERE status = 'pending'
  AND created_at < $1
ORDER BY created_at ASC
LIMIT $2;

-- name: ListReadyMediaByRoomTypeIDs :many
SELECT
    rm.id,
    rm.room_type_id,
    rm.media_asset_id,
    rm.sort_order,
    rm.is_cover,
    rm.created_at,
    ma.object_key,
    ma.mime_type,
    ma.width,
    ma.height
FROM room_media rm
JOIN media_assets ma ON ma.id = rm.media_asset_id
WHERE rm.room_type_id = ANY(sqlc.arg(room_type_ids)::uuid[])
  AND ma.status = 'ready'
ORDER BY rm.room_type_id, rm.is_cover DESC, rm.sort_order ASC, rm.created_at ASC;

-- name: ListReadyMediaByRoomIDs :many
SELECT
    rm.id,
    rm.room_id,
    rm.media_asset_id,
    rm.sort_order,
    rm.is_cover,
    rm.created_at,
    ma.object_key,
    ma.mime_type,
    ma.width,
    ma.height
FROM room_media rm
JOIN media_assets ma ON ma.id = rm.media_asset_id
WHERE rm.room_id = ANY(sqlc.arg(room_ids)::uuid[])
  AND ma.status = 'ready'
ORDER BY rm.room_id, rm.is_cover DESC, rm.sort_order ASC, rm.created_at ASC;

-- name: ClearHotelLogoForMediaAsset :exec
UPDATE hotel_settings
SET logo_media_id = NULL
WHERE logo_media_id = $1;
