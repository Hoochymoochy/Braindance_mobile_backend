CREATE TABLE listening_history (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    track_id VARCHAR(255) NOT NULL,
    track_name VARCHAR(512) NOT NULL,
    artist_name VARCHAR(512) NOT NULL,
    album_name VARCHAR(512),
    album_art_url TEXT,
    played_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_listening_history_user_id ON listening_history(user_id);
CREATE INDEX idx_listening_history_played_at ON listening_history(played_at DESC);
