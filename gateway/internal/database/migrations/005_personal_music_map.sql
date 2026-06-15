-- Personal Music Map (idempotent mirror of Backend/sql/005_personal_music_map.sql)

CREATE TABLE IF NOT EXISTS music_events (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    track_id VARCHAR(255) NOT NULL,
    track_name VARCHAR(512) NOT NULL,
    artist_name VARCHAR(512) NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL,
    latitude DOUBLE PRECISION NOT NULL,
    longitude DOUBLE PRECISION NOT NULL,
    h3_index VARCHAR(32) NOT NULL,
    energy DOUBLE PRECISION,
    danceability DOUBLE PRECISION,
    valence DOUBLE PRECISION,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_music_events_user_id ON music_events(user_id);
CREATE INDEX IF NOT EXISTS idx_music_events_h3_index ON music_events(h3_index);
CREATE INDEX IF NOT EXISTS idx_music_events_user_h3 ON music_events(user_id, h3_index);
CREATE INDEX IF NOT EXISTS idx_music_events_timestamp ON music_events(timestamp DESC);

CREATE TABLE IF NOT EXISTS music_hex_profiles (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    h3_index VARCHAR(32) NOT NULL,
    event_count INTEGER NOT NULL DEFAULT 0,
    listening_minutes DOUBLE PRECISION NOT NULL DEFAULT 0,
    top_genres_json JSONB NOT NULL DEFAULT '[]',
    top_artists_json JSONB NOT NULL DEFAULT '[]',
    avg_energy DOUBLE PRECISION,
    avg_danceability DOUBLE PRECISION,
    discovery_score DOUBLE PRECISION,
    repeat_score DOUBLE PRECISION,
    territory_name VARCHAR(128),
    last_updated TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, h3_index)
);

CREATE INDEX IF NOT EXISTS idx_music_hex_profiles_user_id ON music_hex_profiles(user_id);

CREATE TABLE IF NOT EXISTS music_insights (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    insight_type VARCHAR(64) NOT NULL,
    h3_index VARCHAR(32),
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_music_insights_user_id ON music_insights(user_id);
