-- Top songs per hex profile for map detail view

ALTER TABLE music_hex_profiles
    ADD COLUMN IF NOT EXISTS top_songs_json JSONB NOT NULL DEFAULT '[]';
