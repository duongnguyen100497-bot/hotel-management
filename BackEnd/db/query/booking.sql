-- name: ListUnavailableRoomIDsForStay :many
SELECT DISTINCT room_id
FROM (
    SELECT b.room_id
    FROM bookings b
    WHERE b.room_id = ANY($1::uuid[])
      AND b.status IN ('pending', 'confirmed', 'checked_in')
      AND b.stay && daterange($2::date, $3::date, '[)')
    UNION
    SELECT h.room_id
    FROM reservation_holds h
    WHERE h.room_id = ANY($1::uuid[])
      AND h.status = 'active'
      AND h.expires_at > $4
      AND h.stay && daterange($2::date, $3::date, '[)')
) unavailable;

-- name: HasActiveBookingForCustomer :one
SELECT EXISTS (
    SELECT 1
    FROM bookings
    WHERE customer_id = $1
      AND status IN ('pending', 'confirmed', 'checked_in')
) AS has_active;

-- name: TryAdvisoryXactLockRoom :one
SELECT pg_try_advisory_xact_lock(hashtextextended($1::text, 0)) AS locked;

-- name: InsertReservationHold :one
INSERT INTO reservation_holds (
    id, customer_id, booking_group_id, room_id, room_type_id,
    stay, hold_mode, hold_token, redis_key, ttl_seconds,
    status, expires_at, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5,
    daterange($6::date, $7::date, '[)'), $8, $9, $10, $11,
    $12, $13, $14, $14
)
RETURNING
    id, customer_id, booking_group_id, room_id, room_type_id,
    lower(stay)::date AS check_in, upper(stay)::date AS check_out,
    hold_mode, hold_token, redis_key, ttl_seconds,
    status, expires_at, released_reason, created_at, updated_at;

-- name: GetReservationHoldByIDForUpdate :one
SELECT
    id, customer_id, booking_group_id, room_id, room_type_id,
    lower(stay)::date AS check_in, upper(stay)::date AS check_out,
    hold_mode, hold_token, redis_key, ttl_seconds,
    status, expires_at, released_reason, created_at, updated_at
FROM reservation_holds
WHERE id = $1
FOR UPDATE;

-- name: ListReservationHoldsByGroupIDForUpdate :many
SELECT
    id, customer_id, booking_group_id, room_id, room_type_id,
    lower(stay)::date AS check_in, upper(stay)::date AS check_out,
    hold_mode, hold_token, redis_key, ttl_seconds,
    status, expires_at, released_reason, created_at, updated_at
FROM reservation_holds
WHERE booking_group_id = $1
ORDER BY id ASC
FOR UPDATE;

-- name: UpdateReservationHoldsStatus :exec
UPDATE reservation_holds
SET status = $2,
    released_reason = $3,
    updated_at = $4
WHERE id = ANY($1::uuid[]);
