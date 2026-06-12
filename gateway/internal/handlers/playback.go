package handlers

import (
	"log"
	"time"

	"braindance-gateway/internal/database"
	"braindance-gateway/internal/models"
	"braindance-gateway/internal/spotify"
)

const songTTL = 5 * time.Second

// SyncCurrentlyPlaying polls Spotify for the user's active track and keeps Redis in sync.
// Returns the track when playing, or nil when nothing is playing.
func SyncCurrentlyPlaying(spotifyID string) (*models.Track, error) {
	user, err := database.GetUserBySpotifyID(spotifyID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, nil
	}

	token, err := database.GetValidToken(user.ID)
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, nil
	}

	cp, err := spotify.FetchCurrentlyPlaying(token.AccessToken)
	if err != nil {
		return nil, err
	}

	if cp.IsPlaying && cp.Track != nil {
		if err := database.SetCurrentSong(spotifyID, cp.Track, songTTL); err != nil {
			log.Printf("Failed to cache song for %s: %v", spotifyID, err)
		}
		return cp.Track, nil
	}

	cached, _ := database.GetCurrentSong(spotifyID)
	if cached != nil {
		database.ClearCurrentSong(spotifyID)
	}

	return nil, nil
}
