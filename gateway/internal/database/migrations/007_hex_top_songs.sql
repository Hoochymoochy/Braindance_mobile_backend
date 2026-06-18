-- Top songs per hex profile for map detail view (idempotent mirror of Backend/sql/007_hex_top_songs.sql)

ALTER TABLE music_hex_profiles
    ADD COLUMN IF NOT EXISTS top_songs_json JSONB NOT NULL DEFAULT '[]';
