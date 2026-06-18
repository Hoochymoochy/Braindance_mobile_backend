-- Album and artist cover art for map hex profiles (idempotent mirror of Backend/sql/006_hex_cover_art.sql)

ALTER TABLE music_hex_profiles
    ADD COLUMN IF NOT EXISTS top_track_id VARCHAR(255),
    ADD COLUMN IF NOT EXISTS top_track_name VARCHAR(512),
    ADD COLUMN IF NOT EXISTS album_art_url TEXT,
    ADD COLUMN IF NOT EXISTS artist_image_url TEXT;
