-- name: CreateServiceCatalogItem :one
INSERT INTO service_catalog (
    id,
    code,
    name,
    category,
    unit_price,
    currency,
    description,
    is_active,
    created_at,
    updated_at,
    version
) VALUES (
    sqlc.arg(id),
    sqlc.arg(code),
    sqlc.arg(name),
    sqlc.arg(category),
    sqlc.arg(unit_price),
    sqlc.arg(currency),
    sqlc.arg(description),
    sqlc.arg(is_active),
    sqlc.arg(created_at),
    sqlc.arg(created_at),
    1
)
RETURNING
    id,
    code,
    name,
    category,
    unit_price,
    btrim(currency)::text AS currency,
    description,
    is_active,
    created_at,
    updated_at,
    version;

-- name: GetServiceCatalogItemByID :one
SELECT
    id,
    code,
    name,
    category,
    unit_price,
    btrim(currency)::text AS currency,
    description,
    is_active,
    created_at,
    updated_at,
    version
FROM service_catalog
WHERE id = $1;

-- name: GetServiceCatalogItemByIDForUpdate :one
SELECT
    id,
    code,
    name,
    category,
    unit_price,
    btrim(currency)::text AS currency,
    description,
    is_active,
    created_at,
    updated_at,
    version
FROM service_catalog
WHERE id = $1
FOR UPDATE;

-- name: UpdateServiceCatalogItem :one
UPDATE service_catalog
SET
    name        = sqlc.arg(name),
    category    = sqlc.arg(category),
    unit_price  = sqlc.arg(unit_price),
    currency    = sqlc.arg(currency),
    description = sqlc.arg(description),
    is_active   = sqlc.arg(is_active),
    updated_at  = sqlc.arg(updated_at),
    version     = version + 1
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
RETURNING
    id,
    code,
    name,
    category,
    unit_price,
    btrim(currency)::text AS currency,
    description,
    is_active,
    created_at,
    updated_at,
    version;

-- name: DeactivateServiceCatalogItem :one
UPDATE service_catalog
SET
    is_active  = false,
    updated_at = sqlc.arg(updated_at),
    version    = CASE WHEN is_active THEN version + 1 ELSE version END
WHERE id = sqlc.arg(id)
RETURNING
    id,
    code,
    name,
    category,
    unit_price,
    btrim(currency)::text AS currency,
    description,
    is_active,
    created_at,
    updated_at,
    version;
