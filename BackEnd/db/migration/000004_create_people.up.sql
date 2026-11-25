CREATE TABLE employees (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL UNIQUE REFERENCES users(id) ON DELETE RESTRICT,
    employee_code varchar(32) NOT NULL UNIQUE,
    full_name text NOT NULL,
    phone text NOT NULL,
    email citext NOT NULL UNIQUE,
    position_title text NOT NULL,
    employment_status varchar(24) NOT NULL CHECK (employment_status IN ('active', 'inactive')),
    archived_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version int NOT NULL DEFAULT 1 CHECK (version >= 1)
);

CREATE TABLE customers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid UNIQUE REFERENCES users(id) ON DELETE RESTRICT,
    full_name text NOT NULL,
    phone text NOT NULL,
    email citext,
    nationality_code varchar(8),
    government_id text,
    address text,
    source varchar(24) NOT NULL CHECK (source IN ('online', 'walk_in')),
    archived_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version int NOT NULL DEFAULT 1 CHECK (version >= 1)
);

CREATE UNIQUE INDEX uq_customers_government_id_active
    ON customers (government_id)
    WHERE government_id IS NOT NULL AND archived_at IS NULL;

CREATE INDEX idx_customers_phone ON customers (phone);
CREATE INDEX idx_customers_email ON customers (email);
