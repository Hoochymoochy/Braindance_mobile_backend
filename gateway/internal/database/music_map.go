package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"braindance-gateway/internal/models"
)

// SaveMusicEvent persists a listening event with H3 index and optional audio features.
func SaveMusicEvent(event *models.MusicEvent) error {
	query := `
		INSERT INTO music_events (
			user_id, track_id, track_name, artist_name, timestamp,
			latitude, longitude, h3_index, energy, danceability, valence
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err := db.Exec(
		query,
		event.UserID,
		event.TrackID,
		event.TrackName,
		event.ArtistName,
		event.Timestamp,
		event.Latitude,
		event.Longitude,
		event.H3Index,
		event.Energy,
		event.Danceability,
		event.Valence,
	)
	if err != nil {
		return fmt.Errorf("saving music event: %w", err)
	}
	return nil
}

// GetUserIDBySpotifyID is an alias helper for music map handlers.
func GetUserIDBySpotifyID(spotifyID string) (int, error) {
	user, err := GetUserBySpotifyID(spotifyID)
	if err != nil {
		return 0, err
	}
	if user == nil {
		return 0, nil
	}
	return user.ID, nil
}

// ListUserIDsWithMusicEvents returns distinct user IDs that have listening events.
func ListUserIDsWithMusicEvents() ([]int, error) {
	rows, err := db.Query(`SELECT DISTINCT user_id FROM music_events`)
	if err != nil {
		return nil, fmt.Errorf("listing users with music events: %w", err)
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

type hexAggregationRow struct {
	H3Index          string
	EventCount       int
	ListeningMinutes float64
	TopGenres        []models.GenreCount
	TopArtists       []models.ArtistCount
	AvgEnergy        *float64
	AvgDanceability  *float64
	DiscoveryScore   float64
	RepeatScore      float64
}

// AggregateMusicEventsForUser computes hex-level stats from raw events.
func AggregateMusicEventsForUser(userID int) ([]hexAggregationRow, error) {
	rows, err := db.Query(`
		SELECT
			h3_index,
			COUNT(*) AS event_count,
			COUNT(*) * 3.5 AS listening_minutes,
			AVG(energy) AS avg_energy,
			AVG(danceability) AS avg_danceability
		FROM music_events
		WHERE user_id = $1
		GROUP BY h3_index
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("aggregating music events: %w", err)
	}
	defer rows.Close()

	var results []hexAggregationRow
	for rows.Next() {
		var row hexAggregationRow
		var avgEnergy, avgDance sql.NullFloat64
		if err := rows.Scan(&row.H3Index, &row.EventCount, &row.ListeningMinutes, &avgEnergy, &avgDance); err != nil {
			return nil, err
		}
		if avgEnergy.Valid {
			row.AvgEnergy = &avgEnergy.Float64
		}
		if avgDance.Valid {
			row.AvgDanceability = &avgDance.Float64
		}

		genres, err := topGenresForHex(userID, row.H3Index)
		if err != nil {
			return nil, err
		}
		artists, err := topArtistsForHex(userID, row.H3Index)
		if err != nil {
			return nil, err
		}
		discovery, repeat, err := discoveryRepeatScores(userID, row.H3Index)
		if err != nil {
			return nil, err
		}

		row.TopGenres = genres
		row.TopArtists = artists
		row.DiscoveryScore = discovery
		row.RepeatScore = repeat
		results = append(results, row)
	}
	return results, rows.Err()
}

func topArtistsForHex(userID int, h3Index string) ([]models.ArtistCount, error) {
	rows, err := db.Query(`
		SELECT artist_name, COUNT(*) AS cnt
		FROM music_events
		WHERE user_id = $1 AND h3_index = $2
		GROUP BY artist_name
		ORDER BY cnt DESC
		LIMIT 5
	`, userID, h3Index)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artists []models.ArtistCount
	for rows.Next() {
		var a models.ArtistCount
		if err := rows.Scan(&a.Artist, &a.Count); err != nil {
			return nil, err
		}
		artists = append(artists, a)
	}
	return artists, rows.Err()
}

func topGenresForHex(userID int, h3Index string) ([]models.GenreCount, error) {
	// Genres are derived from artist names as a proxy when Spotify genre data
	// isn't stored per-event. The aggregation job enriches via Spotify API.
	rows, err := db.Query(`
		SELECT artist_name, COUNT(*) AS cnt
		FROM music_events
		WHERE user_id = $1 AND h3_index = $2
		GROUP BY artist_name
		ORDER BY cnt DESC
		LIMIT 3
	`, userID, h3Index)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var genres []models.GenreCount
	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			return nil, err
		}
		genres = append(genres, models.GenreCount{Genre: name, Count: count})
	}
	return genres, rows.Err()
}

func discoveryRepeatScores(userID int, h3Index string) (discovery float64, repeat float64, err error) {
	var total, unique int
	err = db.QueryRow(`
		SELECT COUNT(*), COUNT(DISTINCT track_id)
		FROM music_events
		WHERE user_id = $1 AND h3_index = $2
	`, userID, h3Index).Scan(&total, &unique)
	if err != nil {
		return 0, 0, err
	}
	if total == 0 {
		return 0, 0, nil
	}
	discovery = float64(unique) / float64(total)
	repeat = 1.0 - discovery
	return discovery, repeat, nil
}

// UpsertMusicHexProfile stores aggregated hex profile data.
func UpsertMusicHexProfile(userID int, row hexAggregationRow, territoryName string) error {
	genresJSON, _ := json.Marshal(row.TopGenres)
	artistsJSON, _ := json.Marshal(row.TopArtists)

	query := `
		INSERT INTO music_hex_profiles (
			user_id, h3_index, event_count, listening_minutes,
			top_genres_json, top_artists_json, avg_energy, avg_danceability,
			discovery_score, repeat_score, territory_name, last_updated
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW())
		ON CONFLICT (user_id, h3_index) DO UPDATE SET
			event_count = EXCLUDED.event_count,
			listening_minutes = EXCLUDED.listening_minutes,
			top_genres_json = EXCLUDED.top_genres_json,
			top_artists_json = EXCLUDED.top_artists_json,
			avg_energy = EXCLUDED.avg_energy,
			avg_danceability = EXCLUDED.avg_danceability,
			discovery_score = EXCLUDED.discovery_score,
			repeat_score = EXCLUDED.repeat_score,
			territory_name = EXCLUDED.territory_name,
			last_updated = NOW()
	`

	_, err := db.Exec(
		query,
		userID,
		row.H3Index,
		row.EventCount,
		row.ListeningMinutes,
		genresJSON,
		artistsJSON,
		row.AvgEnergy,
		row.AvgDanceability,
		row.DiscoveryScore,
		row.RepeatScore,
		territoryName,
	)
	return err
}

// GetMusicHexProfiles returns all hex profiles for a user.
func GetMusicHexProfiles(userID int) ([]models.MusicHexProfile, error) {
	rows, err := db.Query(`
		SELECT id, user_id, h3_index, event_count, listening_minutes,
			top_genres_json, top_artists_json, avg_energy, avg_danceability,
			discovery_score, repeat_score, territory_name, last_updated
		FROM music_hex_profiles
		WHERE user_id = $1
		ORDER BY event_count DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("fetching hex profiles: %w", err)
	}
	defer rows.Close()

	var profiles []models.MusicHexProfile
	for rows.Next() {
		var p models.MusicHexProfile
		var genresJSON, artistsJSON []byte
		var avgEnergy, avgDance, discovery, repeat sql.NullFloat64
		var territory sql.NullString

		if err := rows.Scan(
			&p.ID, &p.UserID, &p.H3Index, &p.EventCount, &p.ListeningMinutes,
			&genresJSON, &artistsJSON, &avgEnergy, &avgDance,
			&discovery, &repeat, &territory, &p.LastUpdated,
		); err != nil {
			return nil, err
		}

		_ = json.Unmarshal(genresJSON, &p.TopGenres)
		_ = json.Unmarshal(artistsJSON, &p.TopArtists)
		if avgEnergy.Valid {
			p.AvgEnergy = &avgEnergy.Float64
		}
		if avgDance.Valid {
			p.AvgDanceability = &avgDance.Float64
		}
		if discovery.Valid {
			p.DiscoveryScore = &discovery.Float64
		}
		if repeat.Valid {
			p.RepeatScore = &repeat.Float64
		}
		if territory.Valid {
			p.TerritoryName = territory.String
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

// ReplaceMusicInsights replaces all insights for a user with freshly generated ones.
func ReplaceMusicInsights(userID int, insights []models.MusicInsight) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM music_insights WHERE user_id = $1`, userID); err != nil {
		return err
	}

	for _, ins := range insights {
		_, err := tx.Exec(`
			INSERT INTO music_insights (user_id, insight_type, h3_index, message, created_at, updated_at)
			VALUES ($1, $2, $3, $4, NOW(), NOW())
		`, userID, ins.InsightType, nullString(ins.H3Index), ins.Message)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// GetMusicInsights returns stored insights for a user.
func GetMusicInsights(userID int) ([]models.MusicInsight, error) {
	rows, err := db.Query(`
		SELECT id, user_id, insight_type, h3_index, message, created_at, updated_at
		FROM music_insights
		WHERE user_id = $1
		ORDER BY updated_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("fetching insights: %w", err)
	}
	defer rows.Close()

	var insights []models.MusicInsight
	for rows.Next() {
		var ins models.MusicInsight
		var h3 sql.NullString
		if err := rows.Scan(
			&ins.ID, &ins.UserID, &ins.InsightType, &h3,
			&ins.Message, &ins.CreatedAt, &ins.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if h3.Valid {
			ins.H3Index = h3.String
		}
		insights = append(insights, ins)
	}
	return insights, rows.Err()
}

// UpdateEventAudioFeatures backfills audio features on events missing them.
func UpdateEventAudioFeatures(userID int, trackID string, features *models.AudioFeatures) error {
	_, err := db.Exec(`
		UPDATE music_events
		SET energy = $3, danceability = $4, valence = $5
		WHERE user_id = $1 AND track_id = $2 AND energy IS NULL
	`, userID, trackID, features.Energy, features.Danceability, features.Valence)
	return err
}

// GetDistinctTrackIDsWithoutFeatures returns track IDs needing audio feature enrichment.
func GetDistinctTrackIDsWithoutFeatures(userID int, limit int) ([]string, error) {
	rows, err := db.Query(`
		SELECT DISTINCT track_id FROM music_events
		WHERE user_id = $1 AND energy IS NULL
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// CountNightEvents returns events between 10pm and 4am for a hex.
func CountNightEvents(userID int, h3Index string) (int, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM music_events
		WHERE user_id = $1 AND h3_index = $2
		AND (EXTRACT(HOUR FROM timestamp AT TIME ZONE 'UTC') >= 22
		  OR EXTRACT(HOUR FROM timestamp AT TIME ZONE 'UTC') < 4)
	`, userID, h3Index).Scan(&count)
	return count, err
}

// GetTopTrackIDForHex returns the most played track ID in a hex cell.
func GetTopTrackIDForHex(userID int, h3Index string) (string, error) {
	var trackID string
	err := db.QueryRow(`
		SELECT track_id FROM music_events
		WHERE user_id = $1 AND h3_index = $2
		GROUP BY track_id
		ORDER BY COUNT(*) DESC
		LIMIT 1
	`, userID, h3Index).Scan(&trackID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return trackID, err
}

// MusicMapNeedsAggregation reports whether hex profiles are missing or older than latest events.
func MusicMapNeedsAggregation(userID int) (bool, error) {
	var eventCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM music_events WHERE user_id = $1`, userID).Scan(&eventCount); err != nil {
		return false, err
	}
	if eventCount == 0 {
		return false, nil
	}

	var profileCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM music_hex_profiles WHERE user_id = $1`, userID).Scan(&profileCount); err != nil {
		return false, err
	}
	if profileCount == 0 {
		return true, nil
	}

	latestEvent, err := LatestEventTimestamp(userID)
	if err != nil || latestEvent == nil {
		return false, err
	}

	var latestProfileUpdate sql.NullTime
	err = db.QueryRow(`
		SELECT MAX(last_updated) FROM music_hex_profiles WHERE user_id = $1
	`, userID).Scan(&latestProfileUpdate)
	if err != nil {
		return false, err
	}
	if !latestProfileUpdate.Valid {
		return true, nil
	}

	return latestEvent.After(latestProfileUpdate.Time), nil
}

// LatestEventTimestamp returns the most recent event time for a user.
func LatestEventTimestamp(userID int) (*time.Time, error) {
	var ts time.Time
	err := db.QueryRow(`
		SELECT MAX(timestamp) FROM music_events WHERE user_id = $1
	`, userID).Scan(&ts)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ts, nil
}
