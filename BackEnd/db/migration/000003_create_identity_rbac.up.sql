CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind varchar(16) NOT NULL CHECK (kind IN ('customer', 'staff', 'admin')),
    email citext NOT NULL UNIQUE,
    username citext UNIQUE,
    display_name text NOT NULL,
    status varchar(24) NOT NULL CHECK (status IN ('pending_activation', 'active', 'disabled')),
    authorization_version int NOT NULL DEFAULT 1 CHECK (authorization_version >= 1),
    last_login_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version int NOT NULL DEFAULT 1 CHECK (version >= 1)
);

CREATE TABLE user_credentials (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE RESTRICT,
    password_hash text NOT NULL,
    password_changed_at timestamptz NOT NULL DEFAULT now(),
    failed_login_count int NOT NULL DEFAULT 0 CHECK (failed_login_count >= 0),
    locked_until timestamptz
);

CREATE TABLE auth_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    refresh_token_hash bytea NOT NULL UNIQUE,
    family_id uuid NOT NULL,
    issued_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    replaced_by uuid REFERENCES auth_sessions(id) ON DELETE SET NULL,
    ip_hash bytea,
    user_agent_hash bytea
);

CREATE INDEX idx_auth_sessions_user_id ON auth_sessions (user_id);
CREATE INDEX idx_auth_sessions_family_id ON auth_sessions (family_id);

CREATE TABLE activation_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    token_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_activation_tokens_user_id ON activation_tokens (user_id);

CREATE TABLE password_reset_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    token_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_password_reset_tokens_user_id ON password_reset_tokens (user_id);

CREATE TABLE roles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code varchar(64) NOT NULL UNIQUE,
    name text NOT NULL,
    description text,
    is_system boolean NOT NULL DEFAULT false,
    is_root boolean NOT NULL DEFAULT false,
    archived_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version int NOT NULL DEFAULT 1 CHECK (version >= 1)
);

CREATE UNIQUE INDEX uq_roles_single_active_root ON roles (is_root) WHERE is_root = true AND archived_at IS NULL;

CREATE TABLE permissions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code varchar(100) NOT NULL UNIQUE,
    name text NOT NULL,
    description text,
    is_system boolean NOT NULL DEFAULT true,
    archived_at timestamptz
);

CREATE TABLE user_roles (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    role_id uuid NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
    granted_by uuid REFERENCES users(id) ON DELETE SET NULL,
    granted_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, role_id)
);

CREATE INDEX idx_user_roles_role_id ON user_roles (role_id);

CREATE TABLE role_permissions (
    role_id uuid NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
    permission_id uuid NOT NULL REFERENCES permissions(id) ON DELETE RESTRICT,
    granted_by uuid REFERENCES users(id) ON DELETE SET NULL,
    granted_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (role_id, permission_id)
);

CREATE INDEX idx_role_permissions_permission_id ON role_permissions (permission_id);
