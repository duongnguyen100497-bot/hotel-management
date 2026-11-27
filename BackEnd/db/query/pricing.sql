-- name: ListVATRates :many
SELECT
    id,
    code,
    name,
    rate,
    is_active,
    created_at,
    updated_at,
    version
FROM vat_rates
ORDER BY is_active DESC, created_at ASC;

-- name: GetVATRateByID :one
SELECT
    id,
    code,
    name,
    rate,
    is_active,
    created_at,
    updated_at,
    version
FROM vat_rates
WHERE id = $1;

-- name: GetVATRateByIDForUpdate :one
SELECT
    id,
    code,
    name,
    rate,
    is_active,
    created_at,
    updated_at,
    version
FROM vat_rates
WHERE id = $1
FOR UPDATE;

-- name: GetActiveVATRate :one
SELECT
    id,
    code,
    name,
    rate,
    is_active,
    created_at,
    updated_at,
    version
FROM vat_rates
WHERE is_active = true;

-- name: LockActiveVATRate :one
SELECT
    id,
    code,
    name,
    rate,
    is_active,
    created_at,
    updated_at,
    version
FROM vat_rates
WHERE is_active = true
FOR UPDATE;

-- name: CreateVATRate :one
INSERT INTO vat_rates (
    id,
    code,
    name,
    rate,
    is_active,
    created_at,
    updated_at,
    version
) VALUES (
    $1, $2, $3, $4, $5, $6, $6, 1
)
RETURNING
    id,
    code,
    name,
    rate,
    is_active,
    created_at,
    updated_at,
    version;

-- name: DeactivateOtherVATRates :many
UPDATE vat_rates
SET
    is_active  = false,
    updated_at = sqlc.arg(updated_at),
    version    = version + 1
WHERE is_active = true
  AND id <> sqlc.arg(exclude_id)
RETURNING
    id,
    code,
    name,
    rate,
    is_active,
    created_at,
    updated_at,
    version;

-- name: UpdateVATRate :one
UPDATE vat_rates
SET
    name       = sqlc.arg(name),
    rate       = sqlc.arg(rate),
    is_active  = sqlc.arg(is_active),
    updated_at = sqlc.arg(updated_at),
    version    = version + 1
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
RETURNING
    id,
    code,
    name,
    rate,
    is_active,
    created_at,
    updated_at,
    version;

-- name: DeactivateVATRate :one
UPDATE vat_rates
SET
    is_active  = false,
    updated_at = sqlc.arg(updated_at),
    version    = CASE WHEN is_active THEN version + 1 ELSE version END
WHERE id = sqlc.arg(id)
RETURNING
    id,
    code,
    name,
    rate,
    is_active,
    created_at,
    updated_at,
    version;
