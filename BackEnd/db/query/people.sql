-- name: CreateEmployee :one
INSERT INTO employees (
    id,
    user_id,
    employee_code,
    full_name,
    phone,
    email,
    position_title,
    employment_status,
    created_at,
    updated_at,
    version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $9, 1
)
RETURNING id, user_id, employee_code, full_name, phone, email::text AS email, position_title, employment_status, archived_at, created_at, updated_at, version;

-- name: GetEmployeeByID :one
SELECT
    id,
    user_id,
    employee_code,
    full_name,
    phone,
    email::text AS email,
    position_title,
    employment_status,
    archived_at,
    created_at,
    updated_at,
    version
FROM employees
WHERE id = $1;

-- name: GetEmployeeByIDForUpdate :one
SELECT
    id,
    user_id,
    employee_code,
    full_name,
    phone,
    email::text AS email,
    position_title,
    employment_status,
    archived_at,
    created_at,
    updated_at,
    version
FROM employees
WHERE id = $1
FOR UPDATE;

-- name: GetEmployeeByUserID :one
SELECT
    id,
    user_id,
    employee_code,
    full_name,
    phone,
    email::text AS email,
    position_title,
    employment_status,
    archived_at,
    created_at,
    updated_at,
    version
FROM employees
WHERE user_id = $1;

-- name: ListEmployees :many
SELECT
    id,
    user_id,
    employee_code,
    full_name,
    phone,
    email::text AS email,
    position_title,
    employment_status,
    archived_at,
    created_at,
    updated_at,
    version
FROM employees
WHERE archived_at IS NULL
ORDER BY created_at DESC, id DESC
LIMIT $1;

-- name: UpdateEmployeeOptimistic :one
UPDATE employees
SET full_name = $2,
    phone = $3,
    position_title = $4,
    updated_at = $5,
    version = version + 1
WHERE id = $1
  AND version = $6
  AND archived_at IS NULL
RETURNING id, user_id, employee_code, full_name, phone, email::text AS email, position_title, employment_status, archived_at, created_at, updated_at, version;

-- name: DeactivateEmployee :one
UPDATE employees
SET employment_status = 'inactive',
    updated_at = $2,
    version = version + 1
WHERE id = $1
RETURNING id, user_id, employee_code, full_name, phone, email::text AS email, position_title, employment_status, archived_at, created_at, updated_at, version;

-- name: CreateCustomer :one
INSERT INTO customers (
    id,
    user_id,
    full_name,
    phone,
    email,
    nationality_code,
    government_id,
    address,
    source,
    created_at,
    updated_at,
    version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10, 1
)
RETURNING
    id,
    user_id,
    full_name,
    phone,
    COALESCE(email::text, '')::text AS email,
    COALESCE(nationality_code, '')::text AS nationality_code,
    COALESCE(government_id, '')::text AS government_id,
    COALESCE(address, '')::text AS address,
    source,
    archived_at,
    created_at,
    updated_at,
    version;

-- name: GetCustomerByID :one
SELECT
    id,
    user_id,
    full_name,
    phone,
    COALESCE(email::text, '')::text AS email,
    COALESCE(nationality_code, '')::text AS nationality_code,
    COALESCE(government_id, '')::text AS government_id,
    COALESCE(address, '')::text AS address,
    source,
    archived_at,
    created_at,
    updated_at,
    version
FROM customers
WHERE id = $1;

-- name: GetCustomerByIDForUpdate :one
SELECT
    id,
    user_id,
    full_name,
    phone,
    COALESCE(email::text, '')::text AS email,
    COALESCE(nationality_code, '')::text AS nationality_code,
    COALESCE(government_id, '')::text AS government_id,
    COALESCE(address, '')::text AS address,
    source,
    archived_at,
    created_at,
    updated_at,
    version
FROM customers
WHERE id = $1
FOR UPDATE;

-- name: GetCustomerByUserID :one
SELECT
    id,
    user_id,
    full_name,
    phone,
    COALESCE(email::text, '')::text AS email,
    COALESCE(nationality_code, '')::text AS nationality_code,
    COALESCE(government_id, '')::text AS government_id,
    COALESCE(address, '')::text AS address,
    source,
    archived_at,
    created_at,
    updated_at,
    version
FROM customers
WHERE user_id = $1;

-- name: ListCustomers :many
SELECT
    id,
    user_id,
    full_name,
    phone,
    COALESCE(email::text, '')::text AS email,
    COALESCE(nationality_code, '')::text AS nationality_code,
    COALESCE(government_id, '')::text AS government_id,
    COALESCE(address, '')::text AS address,
    source,
    archived_at,
    created_at,
    updated_at,
    version
FROM customers
WHERE archived_at IS NULL
  AND ($1::text = '' OR full_name ILIKE '%' || $1 || '%' OR phone ILIKE '%' || $1 || '%' OR COALESCE(email::text, '') ILIKE '%' || $1 || '%')
  AND (
      $2::boolean = false
      OR (created_at < $3::timestamptz)
      OR (created_at = $3::timestamptz AND id < $4::uuid)
  )
ORDER BY created_at DESC, id DESC
LIMIT $5;

-- name: UpdateCustomerOptimistic :one
UPDATE customers
SET full_name = $2,
    phone = $3,
    email = $4,
    nationality_code = $5,
    government_id = $6,
    address = $7,
    updated_at = $8,
    version = version + 1
WHERE id = $1
  AND version = $9
  AND archived_at IS NULL
RETURNING
    id,
    user_id,
    full_name,
    phone,
    COALESCE(email::text, '')::text AS email,
    COALESCE(nationality_code, '')::text AS nationality_code,
    COALESCE(government_id, '')::text AS government_id,
    COALESCE(address, '')::text AS address,
    source,
    archived_at,
    created_at,
    updated_at,
    version;

-- name: ArchiveCustomer :one
UPDATE customers
SET archived_at = $2,
    updated_at = $2,
    version = version + 1
WHERE id = $1
  AND archived_at IS NULL
RETURNING
    id,
    user_id,
    full_name,
    phone,
    COALESCE(email::text, '')::text AS email,
    COALESCE(nationality_code, '')::text AS nationality_code,
    COALESCE(government_id, '')::text AS government_id,
    COALESCE(address, '')::text AS address,
    source,
    archived_at,
    created_at,
    updated_at,
    version;
