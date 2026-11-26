-- 000007_create_service_catalog.up.sql
-- Ancillary hotel service catalog (VAT-exclusive unit prices; deactivated instead of hard-deleted).

CREATE TABLE service_catalog (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code        varchar(64) NOT NULL UNIQUE,
    name        text NOT NULL,
    category    varchar(64) NOT NULL,
    unit_price  numeric(14,2) NOT NULL CHECK (unit_price >= 0),
    currency    char(3) NOT NULL DEFAULT 'VND',
    description text NULL,
    is_active   boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    version     integer NOT NULL DEFAULT 1 CHECK (version >= 1)
);

CREATE INDEX idx_service_catalog_category_active
    ON service_catalog (category, is_active, created_at DESC);
