package database

import (
	"encoding/json"
	"fmt"
	"time"

	"braindance-gateway/internal/models"

	"github.com/redis/go-redis/v9"
)

const musicSessionTTL = 2 * time.Hour

func musicSessionKey(userID int) string {
	return fmt.Sprintf("music:user:%d:current", userID)
}

// GetMusicSession returns the active listening session for a user.
func GetMusicSession(userID int) (*models.MusicSession, error) {
	raw, err := rdb.Get(ctx, musicSessionKey(userID)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fetching music session: %w", err)
	}

	var session models.MusicSession
	if err := json.Unmarshal([]byte(raw), &session); err != nil {
		return nil, fmt.Errorf("unmarshaling music session: %w", err)
	}
	return &session, nil
}

// SetMusicSession stores the active listening session with a 2-hour TTL.
func SetMusicSession(userID int, session *models.MusicSession) error {
	payload, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshaling music session: %w", err)
	}
	return rdb.Set(ctx, musicSessionKey(userID), payload, musicSessionTTL).Err()
}

// ClearMusicSession removes the active listening session.
func ClearMusicSession(userID int) error {
	return rdb.Del(ctx, musicSessionKey(userID)).Err()
}
