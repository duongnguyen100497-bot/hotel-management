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

-- name: ClaimExpiredReservationHolds :many
SELECT
    id, customer_id, booking_group_id, room_id, room_type_id,
    lower(stay)::date AS check_in, upper(stay)::date AS check_out,
    hold_mode, hold_token, redis_key, ttl_seconds,
    status, expires_at, released_reason, created_at, updated_at
FROM reservation_holds
WHERE status = 'active'
  AND expires_at <= $1
ORDER BY id ASC
LIMIT $2
FOR UPDATE SKIP LOCKED;

-- name: ListActiveNonExpiredHolds :many
SELECT
    id, customer_id, booking_group_id, room_id, room_type_id,
    lower(stay)::date AS check_in, upper(stay)::date AS check_out,
    hold_mode, hold_token, redis_key, ttl_seconds,
    status, expires_at, released_reason, created_at, updated_at
FROM reservation_holds
WHERE status = 'active'
  AND expires_at > $1
ORDER BY created_at ASC, id ASC;

-- name: ClaimExpiredPendingBookings :many
SELECT
    id, reference_code, booking_group_id, customer_id, room_id, room_type_id,
    lower(stay)::date AS check_in, upper(stay)::date AS check_out,
    status, expires_at
FROM bookings
WHERE status = 'pending'
  AND expires_at IS NOT NULL
  AND expires_at <= $1
ORDER BY id ASC
LIMIT $2
FOR UPDATE SKIP LOCKED;

-- name: MarkBookingExpired :exec
UPDATE bookings
SET status = 'expired',
    updated_at = $2,
    version = version + 1
WHERE id = $1
  AND status = 'pending';

-- name: InsertBookingStatusHistory :exec
INSERT INTO booking_status_history (
    id, booking_id, from_status, to_status, actor_user_id, reason, correlation_id, occurred_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
);

-- name: LockBookingGroupForUpdate :one
SELECT id, status, version
FROM booking_groups
WHERE id = $1
FOR UPDATE;

-- name: ListChildBookingStatusesByGroup :many
SELECT status
FROM bookings
WHERE booking_group_id = $1
ORDER BY id ASC;

-- name: UpdateBookingGroupStatus :exec
UPDATE booking_groups
SET status = $2,
    updated_at = $3,
    version = version + 1
WHERE id = $1;
