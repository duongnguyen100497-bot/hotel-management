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

-- name: InsertBooking :one
INSERT INTO bookings (
    id, reference_code, booking_group_id, customer_id, room_id, room_type_id,
    stay, adults, children, status,
    nightly_rate_snapshot, vat_rate_id, vat_rate_snapshot,
    subtotal_amount, vat_amount, total_amount, currency,
    special_request, expires_at, idempotency_key, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6,
    daterange($7::date, $8::date, '[)'), $9, $10, $11,
    $12, $13, $14,
    $15, $16, $17, $18,
    $19, $20, $21, $22, $22
)
RETURNING
    id, reference_code, booking_group_id, customer_id, room_id, room_type_id,
    lower(stay)::date AS check_in, upper(stay)::date AS check_out,
    adults, children, status,
    nightly_rate_snapshot, vat_rate_id, vat_rate_snapshot,
    subtotal_amount, vat_amount, total_amount, currency,
    special_request, expires_at, cancelled_at, cancel_reason,
    idempotency_key, created_at, updated_at, version;

-- name: InsertBookingGuestSnapshot :one
INSERT INTO booking_guest_snapshots (
    booking_id, full_name, phone, email, nationality_code, special_request, created_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING booking_id, full_name, phone, email, nationality_code, special_request, created_at;

-- name: GetBookingByID :one
SELECT
    b.id, b.reference_code, b.booking_group_id, b.customer_id, b.room_id, b.room_type_id,
    lower(b.stay)::date AS check_in, upper(b.stay)::date AS check_out,
    b.adults, b.children, b.status,
    b.nightly_rate_snapshot, b.vat_rate_id, b.vat_rate_snapshot,
    b.subtotal_amount, b.vat_amount, b.total_amount, b.currency,
    b.special_request, b.expires_at, b.cancelled_at, b.cancel_reason,
    b.idempotency_key, b.created_at, b.updated_at, b.version,
    g.full_name AS guest_full_name, g.phone AS guest_phone, g.email AS guest_email,
    g.nationality_code AS guest_nationality_code, g.special_request AS guest_special_request,
    g.created_at AS guest_created_at
FROM bookings b
LEFT JOIN booking_guest_snapshots g ON g.booking_id = b.id
WHERE b.id = $1;

-- name: GetBookingByIDForUpdate :one
SELECT
    id, reference_code, booking_group_id, customer_id, room_id, room_type_id,
    lower(stay)::date AS check_in, upper(stay)::date AS check_out,
    adults, children, status,
    nightly_rate_snapshot, vat_rate_id, vat_rate_snapshot,
    subtotal_amount, vat_amount, total_amount, currency,
    special_request, expires_at, cancelled_at, cancel_reason,
    idempotency_key, created_at, updated_at, version
FROM bookings
WHERE id = $1
FOR UPDATE;

-- name: GetBookingByCustomerIdempotencyKey :one
SELECT
    b.id, b.reference_code, b.booking_group_id, b.customer_id, b.room_id, b.room_type_id,
    lower(b.stay)::date AS check_in, upper(b.stay)::date AS check_out,
    b.adults, b.children, b.status,
    b.nightly_rate_snapshot, b.vat_rate_id, b.vat_rate_snapshot,
    b.subtotal_amount, b.vat_amount, b.total_amount, b.currency,
    b.special_request, b.expires_at, b.cancelled_at, b.cancel_reason,
    b.idempotency_key, b.created_at, b.updated_at, b.version,
    g.full_name AS guest_full_name, g.phone AS guest_phone, g.email AS guest_email,
    g.nationality_code AS guest_nationality_code, g.special_request AS guest_special_request,
    g.created_at AS guest_created_at
FROM bookings b
LEFT JOIN booking_guest_snapshots g ON g.booking_id = b.id
WHERE b.customer_id = $1
  AND b.idempotency_key = $2;

-- name: UpdateBookingStatusConfirmed :one
UPDATE bookings
SET status = 'confirmed',
    expires_at = NULL,
    updated_at = $2,
    version = version + 1
WHERE id = $1
  AND status = 'pending'
RETURNING
    id, reference_code, booking_group_id, customer_id, room_id, room_type_id,
    lower(stay)::date AS check_in, upper(stay)::date AS check_out,
    adults, children, status,
    nightly_rate_snapshot, vat_rate_id, vat_rate_snapshot,
    subtotal_amount, vat_amount, total_amount, currency,
    special_request, expires_at, cancelled_at, cancel_reason,
    idempotency_key, created_at, updated_at, version;

-- name: UpdateBookingStatusCheckedIn :one
UPDATE bookings
SET status = 'checked_in',
    expires_at = NULL,
    updated_at = $2,
    version = version + 1
WHERE id = $1
  AND status = 'confirmed'
RETURNING
    id, reference_code, booking_group_id, customer_id, room_id, room_type_id,
    lower(stay)::date AS check_in, upper(stay)::date AS check_out,
    adults, children, status,
    nightly_rate_snapshot, vat_rate_id, vat_rate_snapshot,
    subtotal_amount, vat_amount, total_amount, currency,
    special_request, expires_at, cancelled_at, cancel_reason,
    idempotency_key, created_at, updated_at, version;

-- name: UpdateBookingStatusCheckedOut :one
UPDATE bookings
SET status = 'checked_out',
    updated_at = $2,
    version = version + 1
WHERE id = $1
  AND status = 'checked_in'
RETURNING
    id, reference_code, booking_group_id, customer_id, room_id, room_type_id,
    lower(stay)::date AS check_in, upper(stay)::date AS check_out,
    adults, children, status,
    nightly_rate_snapshot, vat_rate_id, vat_rate_snapshot,
    subtotal_amount, vat_amount, total_amount, currency,
    special_request, expires_at, cancelled_at, cancel_reason,
    idempotency_key, created_at, updated_at, version;

-- name: ListBookingsFiltered :many
SELECT
    b.id, b.reference_code, b.booking_group_id, b.customer_id, b.room_id, b.room_type_id,
    lower(b.stay)::date AS check_in, upper(b.stay)::date AS check_out,
    b.adults, b.children, b.status,
    b.nightly_rate_snapshot, b.vat_rate_id, b.vat_rate_snapshot,
    b.subtotal_amount, b.vat_amount, b.total_amount, b.currency,
    b.special_request, b.expires_at, b.cancelled_at, b.cancel_reason,
    b.idempotency_key, b.created_at, b.updated_at, b.version,
    g.full_name AS guest_full_name, g.phone AS guest_phone, g.email AS guest_email,
    g.nationality_code AS guest_nationality_code, g.special_request AS guest_special_request,
    g.created_at AS guest_created_at
FROM bookings b
LEFT JOIN booking_guest_snapshots g ON g.booking_id = b.id
WHERE (sqlc.narg('customer_id')::uuid IS NULL OR b.customer_id = sqlc.narg('customer_id')::uuid)
  AND (sqlc.narg('booking_group_id')::uuid IS NULL OR b.booking_group_id = sqlc.narg('booking_group_id')::uuid)
  AND (sqlc.arg('status')::text = '' OR b.status = sqlc.arg('status')::text)
  AND (sqlc.narg('cursor_created_at')::timestamptz IS NULL OR (b.created_at, b.id) < (sqlc.narg('cursor_created_at')::timestamptz, sqlc.narg('cursor_id')::uuid))
ORDER BY b.created_at DESC, b.id DESC
LIMIT sqlc.arg('limit_rows');

-- name: ListBookingsByGroupID :many
SELECT
    b.id, b.reference_code, b.booking_group_id, b.customer_id, b.room_id, b.room_type_id,
    lower(b.stay)::date AS check_in, upper(b.stay)::date AS check_out,
    b.adults, b.children, b.status,
    b.nightly_rate_snapshot, b.vat_rate_id, b.vat_rate_snapshot,
    b.subtotal_amount, b.vat_amount, b.total_amount, b.currency,
    b.special_request, b.expires_at, b.cancelled_at, b.cancel_reason,
    b.idempotency_key, b.created_at, b.updated_at, b.version,
    g.full_name AS guest_full_name, g.phone AS guest_phone, g.email AS guest_email,
    g.nationality_code AS guest_nationality_code, g.special_request AS guest_special_request,
    g.created_at AS guest_created_at
FROM bookings b
LEFT JOIN booking_guest_snapshots g ON g.booking_id = b.id
WHERE b.booking_group_id = $1
ORDER BY b.id ASC;

-- name: FindConvertedHoldForRoomStay :many
SELECT
    id, customer_id, booking_group_id, room_id, room_type_id,
    lower(stay)::date AS check_in, upper(stay)::date AS check_out,
    hold_mode, hold_token, redis_key, ttl_seconds,
    status, expires_at, released_reason, created_at, updated_at
FROM reservation_holds
WHERE room_id = $1
  AND stay = daterange($2::date, $3::date, '[)')
  AND status = 'converted'
ORDER BY created_at DESC
LIMIT 5;
