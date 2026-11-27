-- 000005_create_settings_vat.up.sql
-- Single-hotel operational settings (including configurable individual/group hold TTLs and free cancellation window)
-- and dynamic VAT rates registry (at most one active VAT rate at a time).

CREATE TABLE hotel_settings (
    id                          boolean PRIMARY KEY DEFAULT true CHECK (id),
    name                        text NOT NULL,
    address                     text NOT NULL,
    phone                       text NOT NULL,
    email                       text NOT NULL,
    timezone                    text NOT NULL DEFAULT 'Asia/Ho_Chi_Minh',
    currency                    char(3) NOT NULL DEFAULT 'VND',
    check_in_time               time NOT NULL DEFAULT '14:00',
    check_out_time              time NOT NULL DEFAULT '12:00',
    individual_hold_ttl_minutes integer NOT NULL DEFAULT 15 CHECK (individual_hold_ttl_minutes BETWEEN 1 AND 120),
    group_hold_ttl_minutes      integer NOT NULL DEFAULT 30 CHECK (group_hold_ttl_minutes BETWEEN 1 AND 180),
    free_cancel_hours           integer NOT NULL DEFAULT 48 CHECK (free_cancel_hours >= 0),
    logo_media_id               uuid NULL,
    updated_at                  timestamptz NOT NULL DEFAULT now(),
    version                     integer NOT NULL DEFAULT 1 CHECK (version >= 1)
);

INSERT INTO hotel_settings (
    id,
    name,
    address,
    phone,
    email,
    timezone,
    currency,
    check_in_time,
    check_out_time,
    individual_hold_ttl_minutes,
    group_hold_ttl_minutes,
    free_cancel_hours
) VALUES (
    true,
    'Grand Saigon Hotel',
    '1 Dong Khoi, Ben Nghe, District 1, Ho Chi Minh City',
    '+84-28-3823-0000',
    'contact@hotel.local',
    'Asia/Ho_Chi_Minh',
    'VND',
    '14:00',
    '12:00',
    15,
    30,
    48
) ON CONFLICT (id) DO NOTHING;

CREATE TABLE vat_rates (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code       varchar(32) NOT NULL UNIQUE,
    name       text NOT NULL,
    rate       numeric(5,4) NOT NULL CHECK (rate >= 0 AND rate <= 1),
    is_active  boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version    integer NOT NULL DEFAULT 1 CHECK (version >= 1)
);

CREATE UNIQUE INDEX uq_vat_rates_single_active
    ON vat_rates ((true))
    WHERE is_active = true;

INSERT INTO vat_rates (code, name, rate, is_active)
SELECT
    'VAT_10',
    'Thuế GTGT tiêu chuẩn 10%',
    0.1000,
    NOT EXISTS (SELECT 1 FROM vat_rates WHERE is_active = true)
WHERE NOT EXISTS (SELECT 1 FROM vat_rates WHERE code = 'VAT_10');
