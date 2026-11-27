-- name: ListPermissions :many
SELECT
    id,
    code,
    name,
    description,
    is_system,
    archived_at
FROM permissions
WHERE archived_at IS NULL
ORDER BY code ASC;

-- name: GetPermissionByID :one
SELECT
    id,
    code,
    name,
    description,
    is_system,
    archived_at
FROM permissions
WHERE id = $1;

-- name: GetPermissionByCode :one
SELECT
    id,
    code,
    name,
    description,
    is_system,
    archived_at
FROM permissions
WHERE code = $1;

-- name: ListPermissionsByCodes :many
SELECT
    id,
    code,
    name,
    description,
    is_system,
    archived_at
FROM permissions
WHERE code = ANY($1::text[])
  AND archived_at IS NULL;

-- name: ListPermissionsByIDs :many
SELECT
    id,
    code,
    name,
    description,
    is_system,
    archived_at
FROM permissions
WHERE id = ANY($1::uuid[])
  AND archived_at IS NULL;

-- name: CreatePermission :one
INSERT INTO permissions (
    id,
    code,
    name,
    description,
    is_system
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING id, code, name, description, is_system, archived_at;

-- name: UpdatePermission :one
UPDATE permissions
SET name = $2,
    description = $3
WHERE id = $1
RETURNING id, code, name, description, is_system, archived_at;

-- name: ListRoles :many
SELECT
    id,
    code,
    name,
    description,
    is_system,
    is_root,
    archived_at,
    created_at,
    updated_at,
    version
FROM roles
WHERE archived_at IS NULL
ORDER BY is_root DESC, is_system DESC, code ASC;

-- name: GetRoleByID :one
SELECT
    id,
    code,
    name,
    description,
    is_system,
    is_root,
    archived_at,
    created_at,
    updated_at,
    version
FROM roles
WHERE id = $1;

-- name: GetRoleByIDForUpdate :one
SELECT
    id,
    code,
    name,
    description,
    is_system,
    is_root,
    archived_at,
    created_at,
    updated_at,
    version
FROM roles
WHERE id = $1
FOR UPDATE;

-- name: GetRoleByCode :one
SELECT
    id,
    code,
    name,
    description,
    is_system,
    is_root,
    archived_at,
    created_at,
    updated_at,
    version
FROM roles
WHERE code = $1;

-- name: GetRootRole :one
SELECT
    id,
    code,
    name,
    description,
    is_system,
    is_root,
    archived_at,
    created_at,
    updated_at,
    version
FROM roles
WHERE is_root = true
  AND archived_at IS NULL
LIMIT 1;

-- name: ListRolesByIDs :many
SELECT
    id,
    code,
    name,
    description,
    is_system,
    is_root,
    archived_at,
    created_at,
    updated_at,
    version
FROM roles
WHERE id = ANY($1::uuid[])
  AND archived_at IS NULL;

-- name: ListRolesByCodes :many
SELECT
    id,
    code,
    name,
    description,
    is_system,
    is_root,
    archived_at,
    created_at,
    updated_at,
    version
FROM roles
WHERE code = ANY($1::text[])
  AND archived_at IS NULL;

-- name: CreateRole :one
INSERT INTO roles (
    id,
    code,
    name,
    description,
    is_system,
    is_root,
    created_at,
    updated_at,
    version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $7, 1
)
RETURNING id, code, name, description, is_system, is_root, archived_at, created_at, updated_at, version;

-- name: UpdateRole :one
UPDATE roles
SET name = $2,
    description = $3,
    updated_at = $4,
    version = version + 1
WHERE id = $1
  AND archived_at IS NULL
RETURNING id, code, name, description, is_system, is_root, archived_at, created_at, updated_at, version;

-- name: ArchiveRole :one
UPDATE roles
SET archived_at = $2,
    updated_at = $2,
    version = version + 1
WHERE id = $1
  AND archived_at IS NULL
RETURNING id, code, name, description, is_system, is_root, archived_at, created_at, updated_at, version;

-- name: DeleteRole :execrows
DELETE FROM roles
WHERE id = $1
  AND is_system = false
  AND is_root = false;

-- name: ListRolePermissions :many
SELECT
    p.id,
    p.code,
    p.name,
    p.description,
    p.is_system,
    p.archived_at
FROM role_permissions rp
JOIN permissions p ON p.id = rp.permission_id
WHERE rp.role_id = $1
  AND p.archived_at IS NULL
ORDER BY p.code ASC;

-- name: DeleteRolePermissionsByRole :exec
DELETE FROM role_permissions
WHERE role_id = $1;

-- name: InsertRolePermission :exec
INSERT INTO role_permissions (
    role_id,
    permission_id,
    granted_by,
    granted_at
) VALUES (
    $1, $2, $3, $4
)
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- name: ListUserRoles :many
SELECT
    r.id,
    r.code,
    r.name,
    r.description,
    r.is_system,
    r.is_root,
    r.archived_at,
    r.created_at,
    r.updated_at,
    r.version
FROM user_roles ur
JOIN roles r ON r.id = ur.role_id
WHERE ur.user_id = $1
  AND r.archived_at IS NULL
ORDER BY r.is_root DESC, r.code ASC;

-- name: DeleteUserRolesByUser :exec
DELETE FROM user_roles
WHERE user_id = $1;

-- name: InsertUserRole :exec
INSERT INTO user_roles (
    user_id,
    role_id,
    granted_by,
    granted_at
) VALUES (
    $1, $2, $3, $4
)
ON CONFLICT (user_id, role_id) DO NOTHING;

-- name: CountUsersAssignedToRole :one
SELECT COUNT(*)::int
FROM user_roles
WHERE role_id = $1;

-- name: ListUserIDsByRole :many
SELECT user_id
FROM user_roles
WHERE role_id = $1;

-- name: CountActiveRootAdmins :one
SELECT COUNT(DISTINCT u.id)::int
FROM users u
JOIN user_roles ur ON ur.user_id = u.id
JOIN roles r ON r.id = ur.role_id
WHERE r.is_root = true
  AND r.archived_at IS NULL
  AND u.status = 'active';

-- name: LockActiveRootAdminUsers :many
SELECT u.id
FROM users u
JOIN user_roles ur ON ur.user_id = u.id
JOIN roles r ON r.id = ur.role_id
WHERE r.is_root = true
  AND r.archived_at IS NULL
  AND u.status = 'active'
ORDER BY u.id
FOR UPDATE OF u;

-- name: UserHasActiveRootRole :one
SELECT EXISTS (
    SELECT 1
    FROM users u
    JOIN user_roles ur ON ur.user_id = u.id
    JOIN roles r ON r.id = ur.role_id
    WHERE u.id = $1
      AND u.status = 'active'
      AND r.is_root = true
      AND r.archived_at IS NULL
) AS is_root_user;

-- name: UserHasEffectivePermission :one
SELECT EXISTS (
    SELECT 1
    FROM users u
    JOIN user_roles ur ON ur.user_id = u.id
    JOIN roles r ON r.id = ur.role_id
    JOIN role_permissions rp ON rp.role_id = r.id
    JOIN permissions p ON p.id = rp.permission_id
    WHERE u.id = $1
      AND u.status = 'active'
      AND r.archived_at IS NULL
      AND p.archived_at IS NULL
      AND p.code = $2
) AS has_permission;

-- name: ListUserEffectivePermissionCodes :many
SELECT DISTINCT p.code
FROM user_roles ur
JOIN roles r ON r.id = ur.role_id
JOIN role_permissions rp ON rp.role_id = r.id
JOIN permissions p ON p.id = rp.permission_id
WHERE ur.user_id = $1
  AND r.archived_at IS NULL
  AND p.archived_at IS NULL
ORDER BY p.code ASC;

-- name: ListUsersWithRoles :many
SELECT
    u.id,
    u.kind,
    u.email::text AS email,
    COALESCE(u.username::text, '')::text AS username,
    u.display_name,
    u.status,
    u.authorization_version,
    u.last_login_at,
    u.created_at,
    u.updated_at,
    u.version
FROM users u
ORDER BY u.created_at DESC, u.id DESC
LIMIT $1;
