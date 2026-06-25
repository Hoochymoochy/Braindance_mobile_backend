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

const geoIndexKey = "user_locations"

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

	// Also add to the GEO index for proximity queries.
	// x = longitude, y = latitude.
	if err := rdb.GeoAdd(ctx, geoIndexKey, &redis.GeoLocation{
		Name:      spotifyID,
		Longitude: x,
		Latitude:  y,
	}).Err(); err != nil {
		return nil, fmt.Errorf("adding to geo index: %w", err)
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
	if err := rdb.Del(ctx, locationKey(spotifyID)).Err(); err != nil {
		return err
	}
	return rdb.ZRem(ctx, geoIndexKey, spotifyID).Err()
}

// NearbyLocation represents a user found via GEORADIUS.
type NearbyLocation struct {
	SpotifyID string
	Distance  float64 // meters
	Longitude float64
	Latitude  float64
}

// NearbyUsers returns active users within radiusMeters of a point.
// Sorted nearest-first. Excludes the requesting user.
func NearbyUsers(excludeSpotifyID string, lng, lat, radiusMeters float64) ([]NearbyLocation, error) {
	results, err := rdb.GeoRadius(ctx, geoIndexKey, lng, lat, &redis.GeoRadiusQuery{
		Radius:    radiusMeters,
		Unit:      "m",
		WithDist:  true,
		WithCoord: true,
		Sort:      "ASC",
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("georadius query: %w", err)
	}

	filtered := make([]NearbyLocation, 0, len(results))
	for _, r := range results {
		if r.Name == excludeSpotifyID {
			continue
		}
		filtered = append(filtered, NearbyLocation{
			SpotifyID: r.Name,
			Distance:  r.Dist,
			Longitude: r.Longitude,
			Latitude:  r.Latitude,
		})
	}
	return filtered, nil
}

// ── Currently Playing Song ───────────────────────────

// songKey builds the Redis key for a user's currently playing track.
func songKey(spotifyID string) string {
	return fmt.Sprintf("song:%s", spotifyID)
}

// SetCurrentSong stores the user's currently playing track in Redis.
func SetCurrentSong(spotifyID string, track *models.Track, ttl time.Duration) error {
	payload, err := json.Marshal(track)
	if err != nil {
		return fmt.Errorf("marshaling track: %w", err)
	}

	return rdb.Set(ctx, songKey(spotifyID), payload, ttl).Err()
}

// ClearCurrentSong clears the user's currently playing song key.
func ClearCurrentSong(spotifyID string) error {
	return rdb.Del(ctx, songKey(spotifyID)).Err()
}

// GetCurrentSong retrieves the cached currently playing track from Redis.
// Returns nil if the key doesn't exist (expired or never set).
func GetCurrentSong(spotifyID string) (*models.Track, error) {
	raw, err := rdb.Get(ctx, songKey(spotifyID)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fetching song from Redis: %w", err)
	}

	var track models.Track
	if err := json.Unmarshal([]byte(raw), &track); err != nil {
		return nil, fmt.Errorf("unmarshaling track: %w", err)
	}

	return &track, nil
}

// ── Reactions ─────────────────────────────────────────

const reactionTTL = 5 * time.Minute

func reactionKey(sessionID string) string {
	return fmt.Sprintf("reactions:%s", sessionID)
}

// AddReaction stores an emoji reaction for the target ephemeral session.
func AddReaction(sessionID, emoji string) error {
	pipe := rdb.Pipeline()
	pipe.RPush(ctx, reactionKey(sessionID), emoji)
	pipe.Expire(ctx, reactionKey(sessionID), reactionTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// ── Visibility ────────────────────────────────────────

func hiddenKey(spotifyID string) string {
	return fmt.Sprintf("hidden:%s", spotifyID)
}

// SetVisibility opts a user in or out of appearing in nearby results.
// Uses opt-out semantics: users are visible by default; calling with visible=false
// makes them hidden. The hidden key persists for 24 hours.
func SetVisibility(spotifyID string, visible bool) error {
	if visible {
		// Remove the hidden marker — user is visible again.
		return rdb.Del(ctx, hiddenKey(spotifyID)).Err()
	}
	return rdb.Set(ctx, hiddenKey(spotifyID), "1", 24*time.Hour).Err()
}

// IsVisible returns whether a user has opted into nearby visibility.
// Defaults to visible (true) unless the user has explicitly opted out.
func IsVisible(spotifyID string) bool {
	exists, _ := rdb.Exists(ctx, hiddenKey(spotifyID)).Result()
	return exists == 0 // visible unless explicitly hidden
}

// RedisExists is a generic Redis EXISTS wrapper used by rate limiters.
func RedisExists(key string) bool {
	exists, _ := rdb.Exists(ctx, key).Result()
	return exists > 0
}

// RedisSetTTL sets a key with a TTL (used for rate-limit and notification markers).
func RedisSetTTL(key, value string, ttl time.Duration) error {
	return rdb.Set(ctx, key, value, ttl).Err()
}

// GeoIndexMembers returns all members in the geo index (active users with known location).
func GeoIndexMembers() ([]string, error) {
	return rdb.ZRange(ctx, geoIndexKey, 0, -1).Result()
}

// DrainReactions fetches and clears all pending reactions for a session.
func DrainReactions(sessionID string) ([]string, error) {
	key := reactionKey(sessionID)
	reactions, err := rdb.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	if len(reactions) > 0 {
		rdb.Del(ctx, key)
	}
	return reactions, nil
}
