package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"braindance-gateway/internal/models"

	"github.com/redis/go-redis/v9"
)

var rdb *redis.Client
var ctx = context.Background()

// ConnectRedis opens a connection to Redis and verifies it with a PING.
func ConnectRedis() error {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	rdb = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("connecting to Redis: %w", err)
	}

	log.Println("Connected to Redis")
	return nil
}

// CloseRedis shuts down the Redis connection pool.
func CloseRedis() {
	if rdb != nil {
		rdb.Close()
	}
}

// locationKey builds the Redis key for a user's location.
func locationKey(spotifyID string) string {
	return fmt.Sprintf("location:%s", spotifyID)
}

// UpdateLocation stores a user's position in Redis and sets a 2-minute TTL
// so stale locations from disconnected clients are automatically cleaned up.
func UpdateLocation(spotifyID string, x, y, z float64) (*models.LocationUpdate, error) {
	loc := models.LocationUpdate{
		SpotifyID: spotifyID,
		X:         x,
		Y:         y,
		Z:         z,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	payload, err := json.Marshal(loc)
	if err != nil {
		return nil, fmt.Errorf("marshaling location: %w", err)
	}

	// Store with a 2-minute TTL — clients must keep sending updates
	if err := rdb.Set(ctx, locationKey(spotifyID), payload, 2*time.Minute).Err(); err != nil {
		return nil, fmt.Errorf("storing location in Redis: %w", err)
	}

	return &loc, nil
}

// GetLocation retrieves a user's last-known position from Redis.
// Returns nil if the key has expired or doesn't exist.
func GetLocation(spotifyID string) (*models.LocationUpdate, error) {
	raw, err := rdb.Get(ctx, locationKey(spotifyID)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fetching location from Redis: %w", err)
	}

	var loc models.LocationUpdate
	if err := json.Unmarshal([]byte(raw), &loc); err != nil {
		return nil, fmt.Errorf("unmarshaling location: %w", err)
	}

	return &loc, nil
}

// DeleteLocation removes a user's location from Redis (e.g. on disconnect).
func DeleteLocation(spotifyID string) error {
	return rdb.Del(ctx, locationKey(spotifyID)).Err()
}
