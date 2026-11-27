-- 000008_create_media.down.sql
ALTER TABLE hotel_settings
    DROP CONSTRAINT IF EXISTS fk_hotel_settings_logo_media;

DROP TABLE IF EXISTS room_media;
DROP TABLE IF EXISTS media_assets;
