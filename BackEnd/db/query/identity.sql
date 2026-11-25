-- name: CreateUser :one
INSERT INTO users (
    id,
    kind,
    email,
    username,
    display_name,
    status,
    authorization_version,
    created_at,
    updated_at,
    version
) VALUES (
    $1, $2, $3, $4, $5, $6, 1, $7, $7, 1
)
RETURNING id, kind, email::text AS email, COALESCE(username::text, '')::text AS username, display_name, status, authorization_version, last_login_at, created_at, updated_at, version;

-- name: GetUserByID :one
SELECT
    id,
    kind,
    email::text AS email,
    COALESCE(username::text, '')::text AS username,
    display_name,
    status,
    authorization_version,
    last_login_at,
    created_at,
    updated_at,
    version
FROM users
WHERE id = $1;

-- name: GetUserByIDForUpdate :one
SELECT
    id,
    kind,
    email::text AS email,
    COALESCE(username::text, '')::text AS username,
    display_name,
    status,
    authorization_version,
    last_login_at,
    created_at,
    updated_at,
    version
FROM users
WHERE id = $1
FOR UPDATE;

-- name: GetUserByEmail :one
SELECT
    id,
    kind,
    email::text AS email,
    COALESCE(username::text, '')::text AS username,
    display_name,
    status,
    authorization_version,
    last_login_at,
    created_at,
    updated_at,
    version
FROM users
WHERE email = $1;

-- name: GetUserByEmailForUpdate :one
SELECT
    id,
    kind,
    email::text AS email,
    COALESCE(username::text, '')::text AS username,
    display_name,
    status,
    authorization_version,
    last_login_at,
    created_at,
    updated_at,
    version
FROM users
WHERE email = $1
FOR UPDATE;

-- name: UpdateUserStatus :one
UPDATE users
SET status = $2,
    authorization_version = authorization_version + 1,
    updated_at = $3,
    version = version + 1
WHERE id = $1
RETURNING id, kind, email::text AS email, COALESCE(username::text, '')::text AS username, display_name, status, authorization_version, last_login_at, created_at, updated_at, version;

-- name: UpdateUserDisplayName :one
UPDATE users
SET display_name = $2,
    updated_at = $3,
    version = version + 1
WHERE id = $1
RETURNING id, kind, email::text AS email, COALESCE(username::text, '')::text AS username, display_name, status, authorization_version, last_login_at, created_at, updated_at, version;

-- name: TouchUserLastLogin :exec
UPDATE users
SET last_login_at = $2,
    updated_at = $2
WHERE id = $1;

-- name: BumpUserAuthorizationVersion :exec
UPDATE users
SET authorization_version = authorization_version + 1,
    updated_at = $2,
    version = version + 1
WHERE id = $1;

-- name: BumpUsersAuthorizationVersion :exec
UPDATE users
SET authorization_version = authorization_version + 1,
    updated_at = $2,
    version = version + 1
WHERE id = ANY($1::uuid[]);

-- name: UpsertUserCredential :exec
INSERT INTO user_credentials (
    user_id,
    password_hash,
    password_changed_at,
    failed_login_count,
    locked_until
) VALUES (
    $1, $2, $3, 0, NULL
)
ON CONFLICT (user_id) DO UPDATE
SET password_hash = EXCLUDED.password_hash,
    password_changed_at = EXCLUDED.password_changed_at,
    failed_login_count = 0,
    locked_until = NULL;

-- name: GetUserCredential :one
SELECT
    user_id,
    password_hash,
    password_changed_at,
    failed_login_count,
    locked_until
FROM user_credentials
WHERE user_id = $1;

-- name: GetUserCredentialForUpdate :one
SELECT
    user_id,
    password_hash,
    password_changed_at,
    failed_login_count,
    locked_until
FROM user_credentials
WHERE user_id = $1
FOR UPDATE;

-- name: RecordFailedLogin :exec
UPDATE user_credentials
SET failed_login_count = $2,
    locked_until = $3
WHERE user_id = $1;

-- name: ResetFailedLogin :exec
UPDATE user_credentials
SET failed_login_count = 0,
    locked_until = NULL
WHERE user_id = $1;

-- name: InsertAuthSession :one
INSERT INTO auth_sessions (
    id,
    user_id,
    refresh_token_hash,
    family_id,
    issued_at,
    expires_at,
    ip_hash,
    user_agent_hash
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING id, user_id, refresh_token_hash, family_id, issued_at, expires_at, revoked_at, replaced_by, ip_hash, user_agent_hash;

-- name: GetAuthSessionByHashForUpdate :one
SELECT
    id,
    user_id,
    refresh_token_hash,
    family_id,
    issued_at,
    expires_at,
    revoked_at,
    replaced_by,
    ip_hash,
    user_agent_hash
FROM auth_sessions
WHERE refresh_token_hash = $1
FOR UPDATE;

-- name: GetAuthSessionByID :one
SELECT
    id,
    user_id,
    refresh_token_hash,
    family_id,
    issued_at,
    expires_at,
    revoked_at,
    replaced_by,
    ip_hash,
    user_agent_hash
FROM auth_sessions
WHERE id = $1;

-- name: ListActiveSessionsByUser :many
SELECT
    id,
    user_id,
    refresh_token_hash,
    family_id,
    issued_at,
    expires_at,
    revoked_at,
    replaced_by,
    ip_hash,
    user_agent_hash
FROM auth_sessions
WHERE user_id = $1
  AND revoked_at IS NULL
  AND expires_at > $2
ORDER BY issued_at DESC;

-- name: RotateAuthSession :exec
UPDATE auth_sessions
SET revoked_at = $2,
    replaced_by = $3
WHERE id = $1;

-- name: RevokeAuthSessionByID :execrows
UPDATE auth_sessions
SET revoked_at = $3
WHERE id = $1
  AND user_id = $2
  AND revoked_at IS NULL;

-- name: RevokeAuthSessionFamily :exec
UPDATE auth_sessions
SET revoked_at = $2
WHERE family_id = $1
  AND revoked_at IS NULL;

-- name: RevokeAllUserAuthSessions :exec
UPDATE auth_sessions
SET revoked_at = $2
WHERE user_id = $1
  AND revoked_at IS NULL;

-- name: RevokeOtherUserAuthSessions :exec
UPDATE auth_sessions
SET revoked_at = $3
WHERE user_id = $1
  AND id <> $2
  AND revoked_at IS NULL;

-- name: InsertPasswordResetToken :one
INSERT INTO password_reset_tokens (
    id,
    user_id,
    token_hash,
    expires_at,
    created_at
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING id, user_id, token_hash, expires_at, used_at, created_at;

-- name: GetPasswordResetTokenByHashForUpdate :one
SELECT
    id,
    user_id,
    token_hash,
    expires_at,
    used_at,
    created_at
FROM password_reset_tokens
WHERE token_hash = $1
FOR UPDATE;

-- name: MarkPasswordResetTokenUsed :exec
UPDATE password_reset_tokens
SET used_at = $2
WHERE id = $1;

-- name: InvalidateUnusedPasswordResetTokensByUser :exec
UPDATE password_reset_tokens
SET used_at = $2
WHERE user_id = $1
  AND used_at IS NULL;

-- name: InsertActivationToken :one
INSERT INTO activation_tokens (
    id,
    user_id,
    token_hash,
    expires_at,
    created_at
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING id, user_id, token_hash, expires_at, used_at, created_at;

-- name: GetActivationTokenByHashForUpdate :one
SELECT
    id,
    user_id,
    token_hash,
    expires_at,
    used_at,
    created_at
FROM activation_tokens
WHERE token_hash = $1
FOR UPDATE;

-- name: MarkActivationTokenUsed :exec
UPDATE activation_tokens
SET used_at = $2
WHERE id = $1;

-- name: InvalidateUnusedActivationTokensByUser :exec
UPDATE activation_tokens
SET used_at = $2
WHERE user_id = $1
  AND used_at IS NULL;
