-- 008_device_tokens.sql
-- Stores FCM/APNs device tokens for push notifications.

CREATE TABLE IF NOT EXISTS device_tokens (
    id          SERIAL PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id),
    token       TEXT NOT NULL,
    platform    TEXT NOT NULL DEFAULT 'ios',  -- 'ios' or 'android'
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, token)
);

CREATE INDEX IF NOT EXISTS idx_device_tokens_user ON device_tokens(user_id);
