package musicmap

import (
	"errors"
	"log"
	"sync"
	"time"

	"braindance-gateway/internal/auth"
	"braindance-gateway/internal/database"
	"braindance-gateway/internal/models"
	"braindance-gateway/internal/spotify"
)

var audioFeaturesWarnOnce sync.Once

var (
	aggMu     sync.Mutex
	aggTimers = map[int]*time.Timer{}
)

// StartAggregationJob runs hex profile aggregation periodically.
func StartAggregationJob() {
	go func() {
		time.Sleep(10 * time.Second)
		runAggregation()
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			runAggregation()
		}
	}()
}

// EnsureAggregated updates hex profiles when raw events are newer than the last aggregation.
func EnsureAggregated(userID int) {
	needs, err := database.MusicMapNeedsAggregation(userID)
	if err != nil {
		log.Printf("Aggregation check failed for user %d: %v", userID, err)
		return
	}
	if !needs {
		return
	}
	if err := aggregateUser(userID); err != nil {
		log.Printf("Aggregation: user %d failed: %v", userID, err)
	}
}

// ScheduleAggregation debounces per-user aggregation shortly after new events arrive.
func ScheduleAggregation(userID int) {
	aggMu.Lock()
	defer aggMu.Unlock()

	if timer, ok := aggTimers[userID]; ok {
		timer.Stop()
	}

	aggTimers[userID] = time.AfterFunc(1*time.Second, func() {
		log.Printf("Music map aggregating user %d after new listening event", userID)
		if err := aggregateUser(userID); err != nil {
			log.Printf("Scheduled aggregation failed for user %d: %v", userID, err)
		}
		aggMu.Lock()
		delete(aggTimers, userID)
		aggMu.Unlock()
	})
}

func runAggregation() {
	log.Println("Music map aggregation starting")

	userIDs, err := database.ListUserIDsWithMusicEvents()
	if err != nil {
		log.Printf("Aggregation: failed to list users: %v", err)
		return
	}

	for _, userID := range userIDs {
		if err := aggregateUser(userID); err != nil {
			log.Printf("Aggregation: user %d failed: %v", userID, err)
		}
	}

	log.Printf("Music map aggregation complete for %d users", len(userIDs))
}

func aggregateUser(userID int) error {
	token, _ := auth.EnsureValidToken(userID)
	enrichAudioFeatures(userID)

	rows, err := database.AggregateMusicEventsForUser(userID)
	if err != nil {
		return err
	}

	nightRatios := make(map[string]float64)
	var profiles []models.MusicHexProfile

	for _, row := range rows {
		enrichment, err := enrichHexFromSpotify(userID, row.H3Index, token)
		if err == nil {
			if len(enrichment.TopGenres) > 0 {
				row.TopGenres = enrichment.TopGenres
			}
			row.TopTrackID = enrichment.TopTrackID
			row.TopTrackName = enrichment.TopTrackName
			row.AlbumArtURL = enrichment.AlbumArtURL
			row.ArtistImageURL = enrichment.ArtistImageURL
		}

		nightCount, err := database.CountNightEvents(userID, row.H3Index)
		if err != nil {
			return err
		}
		nightRatio := 0.0
		if row.EventCount > 0 {
			nightRatio = float64(nightCount) / float64(row.EventCount)
		}
		nightRatios[row.H3Index] = nightRatio

		profile := models.MusicHexProfile{
			UserID:           userID,
			H3Index:          row.H3Index,
			EventCount:       row.EventCount,
			ListeningMinutes: row.ListeningMinutes,
			TopGenres:        row.TopGenres,
			TopArtists:       row.TopArtists,
			AvgEnergy:        row.AvgEnergy,
			AvgDanceability:  row.AvgDanceability,
			DiscoveryScore:   &row.DiscoveryScore,
			RepeatScore:      &row.RepeatScore,
			TopTrackID:       row.TopTrackID,
			TopTrackName:     row.TopTrackName,
			AlbumArtURL:      row.AlbumArtURL,
			ArtistImageURL:   row.ArtistImageURL,
		}
		profile.TerritoryName = TerritoryName(profile, nightRatio)

		if err := database.UpsertMusicHexProfile(userID, row, profile.TerritoryName); err != nil {
			return err
		}
		profiles = append(profiles, profile)
	}

	insights := GenerateInsights(profiles, nightRatios)
	return database.ReplaceMusicInsights(userID, insights)
}

type hexSpotifyEnrichment struct {
	TopGenres      []models.GenreCount
	TopTrackID     string
	TopTrackName   string
	AlbumArtURL    string
	ArtistImageURL string
}

func enrichHexFromSpotify(userID int, h3Index string, token *models.Token) (hexSpotifyEnrichment, error) {
	if token == nil {
		return hexSpotifyEnrichment{}, nil
	}

	trackID, err := database.GetTopTrackIDForHex(userID, h3Index)
	if err != nil || trackID == "" {
		return hexSpotifyEnrichment{}, err
	}

	track, err := spotify.FetchTrack(token.AccessToken, trackID)
	if err != nil {
		return hexSpotifyEnrichment{}, err
	}

	enrichment := hexSpotifyEnrichment{
		TopTrackID:   track.ID,
		TopTrackName: track.Name,
		AlbumArtURL:  spotify.BestImageURL(track.Album.Images),
	}

	if len(track.Artists) == 0 {
		return enrichment, nil
	}

	artist, err := spotify.FetchArtist(token.AccessToken, track.Artists[0].ID)
	if err != nil {
		return enrichment, err
	}

	enrichment.ArtistImageURL = spotify.BestImageURL(artist.Images)
	if len(artist.Genres) > 0 {
		genres := make([]models.GenreCount, 0, len(artist.Genres))
		for i, g := range artist.Genres {
			if i >= 3 {
				break
			}
			genres = append(genres, models.GenreCount{Genre: g, Count: 1})
		}
		enrichment.TopGenres = genres
	}

	return enrichment, nil
}

func enrichAudioFeatures(userID int) {
	token, err := auth.EnsureValidToken(userID)
	if err != nil {
		return
	}

	trackIDs, err := database.GetDistinctTrackIDsWithoutFeatures(userID, 50)
	if err != nil || len(trackIDs) == 0 {
		return
	}

	features, err := spotify.FetchAudioFeatures(token.AccessToken, trackIDs)
	if errors.Is(err, spotify.ErrAudioFeaturesUnavailable) {
		audioFeaturesWarnOnce.Do(func() {
			log.Println("Spotify audio-features endpoint unavailable for this app (deprecated by Spotify for new apps) — energy scores will be omitted")
		})
		return
	}
	if err != nil {
		log.Printf("Audio features fetch failed for user %d: %v", userID, err)
		return
	}

	for trackID, f := range features {
		if err := database.UpdateEventAudioFeatures(userID, trackID, &f); err != nil {
			log.Printf("Failed to update audio features for track %s: %v", trackID, err)
		}
	}
}
