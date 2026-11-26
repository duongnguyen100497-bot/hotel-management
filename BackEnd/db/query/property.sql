-- name: GetHotelSettings :one
SELECT
    id,
    name,
    address,
    phone,
    email,
    timezone,
    btrim(currency)::text AS currency,
    to_char(check_in_time, 'HH24:MI')::text AS check_in_time,
    to_char(check_out_time, 'HH24:MI')::text AS check_out_time,
    individual_hold_ttl_minutes,
    group_hold_ttl_minutes,
    free_cancel_hours,
    logo_media_id,
    updated_at,
    version
FROM hotel_settings
WHERE id = true;

-- name: GetHotelSettingsForUpdate :one
SELECT
    id,
    name,
    address,
    phone,
    email,
    timezone,
    btrim(currency)::text AS currency,
    to_char(check_in_time, 'HH24:MI')::text AS check_in_time,
    to_char(check_out_time, 'HH24:MI')::text AS check_out_time,
    individual_hold_ttl_minutes,
    group_hold_ttl_minutes,
    free_cancel_hours,
    logo_media_id,
    updated_at,
    version
FROM hotel_settings
WHERE id = true
FOR UPDATE;

-- name: UpdateHotelSettings :one
UPDATE hotel_settings
SET
    name                        = sqlc.arg(name),
    address                     = sqlc.arg(address),
    phone                       = sqlc.arg(phone),
    email                       = sqlc.arg(email),
    timezone                    = sqlc.arg(timezone),
    currency                    = sqlc.arg(currency),
    check_in_time               = sqlc.arg(check_in_time)::text::time,
    check_out_time              = sqlc.arg(check_out_time)::text::time,
    individual_hold_ttl_minutes = sqlc.arg(individual_hold_ttl_minutes),
    group_hold_ttl_minutes      = sqlc.arg(group_hold_ttl_minutes),
    free_cancel_hours           = sqlc.arg(free_cancel_hours),
    logo_media_id               = sqlc.arg(logo_media_id),
    updated_at                  = sqlc.arg(updated_at),
    version                     = version + 1
WHERE id = true
  AND version = sqlc.arg(expected_version)
RETURNING
    id,
    name,
    address,
    phone,
    email,
    timezone,
    btrim(currency)::text AS currency,
    to_char(check_in_time, 'HH24:MI')::text AS check_in_time,
    to_char(check_out_time, 'HH24:MI')::text AS check_out_time,
    individual_hold_ttl_minutes,
    group_hold_ttl_minutes,
    free_cancel_hours,
    logo_media_id,
    updated_at,
    version;
