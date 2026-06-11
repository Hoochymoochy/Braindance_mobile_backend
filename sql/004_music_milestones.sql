CREATE TABLE music_milestones (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    milestone_type VARCHAR(64) NOT NULL,    -- e.g., 'first_genre', 'top_artist', 'milestone_track'
    milestone_key VARCHAR(255) NOT NULL,    -- e.g., 'techno', '1000th_track'
    milestone_value TEXT,                   -- JSON payload with details
    achieved_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_milestones_user_id ON music_milestones(user_id);
CREATE UNIQUE INDEX idx_milestones_user_type_key ON music_milestones(user_id, milestone_type, milestone_key);
