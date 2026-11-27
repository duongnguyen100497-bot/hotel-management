-- name: CreateRoomType :one
INSERT INTO room_types (
    id,
    code,
    name,
    description,
    base_nightly_rate,
    currency,
    capacity,
    area_sqm,
    bed_type,
    amenities,
    is_bookable,
    created_at,
    updated_at,
    version
) VALUES (
    sqlc.arg(id),
    sqlc.arg(code),
    sqlc.arg(name),
    sqlc.arg(description),
    sqlc.arg(base_nightly_rate),
    sqlc.arg(currency),
    sqlc.arg(capacity),
    sqlc.arg(area_sqm),
    sqlc.arg(bed_type),
    sqlc.arg(amenities),
    sqlc.arg(is_bookable),
    sqlc.arg(created_at),
    sqlc.arg(created_at),
    1
)
RETURNING
    id,
    code,
    name,
    description,
    base_nightly_rate,
    btrim(currency)::text AS currency,
    capacity,
    area_sqm,
    bed_type,
    amenities,
    is_bookable,
    archived_at,
    created_at,
    updated_at,
    version;

-- name: GetRoomTypeByID :one
SELECT
    id,
    code,
    name,
    description,
    base_nightly_rate,
    btrim(currency)::text AS currency,
    capacity,
    area_sqm,
    bed_type,
    amenities,
    is_bookable,
    archived_at,
    created_at,
    updated_at,
    version
FROM room_types
WHERE id = $1;

-- name: GetRoomTypeByIDForUpdate :one
SELECT
    id,
    code,
    name,
    description,
    base_nightly_rate,
    btrim(currency)::text AS currency,
    capacity,
    area_sqm,
    bed_type,
    amenities,
    is_bookable,
    archived_at,
    created_at,
    updated_at,
    version
FROM room_types
WHERE id = $1
FOR UPDATE;

-- name: GetPublicRoomTypeByCode :one
SELECT
    rt.id,
    rt.code,
    rt.name,
    rt.description,
    rt.base_nightly_rate,
    btrim(rt.currency)::text AS currency,
    rt.capacity,
    rt.area_sqm,
    rt.bed_type,
    rt.amenities,
    rt.is_bookable,
    rt.archived_at,
    rt.created_at,
    rt.updated_at,
    rt.version,
    (
        SELECT count(*)::int
        FROM rooms r
        WHERE r.room_type_id = rt.id
          AND r.archived_at IS NULL
          AND r.operational_status <> 'maintenance'
    ) AS operational_room_count
FROM room_types rt
WHERE rt.code = $1
  AND rt.is_bookable = true
  AND rt.archived_at IS NULL;

-- name: UpdateRoomType :one
UPDATE room_types
SET
    name              = sqlc.arg(name),
    description       = sqlc.arg(description),
    base_nightly_rate = sqlc.arg(base_nightly_rate),
    currency          = sqlc.arg(currency),
    capacity          = sqlc.arg(capacity),
    area_sqm          = sqlc.arg(area_sqm),
    bed_type          = sqlc.arg(bed_type),
    amenities         = sqlc.arg(amenities),
    is_bookable       = sqlc.arg(is_bookable),
    archived_at       = sqlc.arg(archived_at),
    updated_at        = sqlc.arg(updated_at),
    version           = version + 1
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
RETURNING
    id,
    code,
    name,
    description,
    base_nightly_rate,
    btrim(currency)::text AS currency,
    capacity,
    area_sqm,
    bed_type,
    amenities,
    is_bookable,
    archived_at,
    created_at,
    updated_at,
    version;

-- name: CreateRoom :one
INSERT INTO rooms (
    id,
    code,
    name,
    room_type_id,
    floor,
    capacity,
    operational_status,
    description,
    created_at,
    updated_at,
    version
) VALUES (
    sqlc.arg(id),
    sqlc.arg(code),
    sqlc.arg(name),
    sqlc.arg(room_type_id),
    sqlc.arg(floor),
    sqlc.arg(capacity),
    'available',
    sqlc.arg(description),
    sqlc.arg(created_at),
    sqlc.arg(created_at),
    1
)
RETURNING
    id,
    code,
    name,
    room_type_id,
    floor,
    capacity,
    operational_status,
    description,
    archived_at,
    created_at,
    updated_at,
    version;

-- name: GetRoomByID :one
SELECT
    id,
    code,
    name,
    room_type_id,
    floor,
    capacity,
    operational_status,
    description,
    archived_at,
    created_at,
    updated_at,
    version
FROM rooms
WHERE id = $1;

-- name: GetRoomByIDForUpdate :one
SELECT
    id,
    code,
    name,
    room_type_id,
    floor,
    capacity,
    operational_status,
    description,
    archived_at,
    created_at,
    updated_at,
    version
FROM rooms
WHERE id = $1
FOR UPDATE;

-- name: UpdateRoomMetadata :one
UPDATE rooms
SET
    code         = sqlc.arg(code),
    name         = sqlc.arg(name),
    room_type_id = sqlc.arg(room_type_id),
    floor        = sqlc.arg(floor),
    capacity     = sqlc.arg(capacity),
    description  = sqlc.arg(description),
    updated_at   = sqlc.arg(updated_at),
    version      = version + 1
WHERE id = sqlc.arg(id)
  AND archived_at IS NULL
  AND version = sqlc.arg(expected_version)
RETURNING
    id,
    code,
    name,
    room_type_id,
    floor,
    capacity,
    operational_status,
    description,
    archived_at,
    created_at,
    updated_at,
    version;

-- name: ArchiveRoom :one
UPDATE rooms
SET
    archived_at = sqlc.arg(archived_at),
    updated_at  = sqlc.arg(archived_at),
    version     = version + 1
WHERE id = sqlc.arg(id)
  AND archived_at IS NULL
  AND operational_status <> 'occupied'
RETURNING
    id,
    code,
    name,
    room_type_id,
    floor,
    capacity,
    operational_status,
    description,
    archived_at,
    created_at,
    updated_at,
    version;

-- name: ListBookableRoomsByType :many
SELECT
    id,
    code,
    name,
    room_type_id,
    floor,
    capacity,
    operational_status,
    description,
    archived_at,
    created_at,
    updated_at,
    version
FROM rooms
WHERE room_type_id = $1
  AND archived_at IS NULL
  AND operational_status <> 'maintenance'
ORDER BY floor ASC, code ASC;
