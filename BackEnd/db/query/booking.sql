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
