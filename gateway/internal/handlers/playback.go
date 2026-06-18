package handlers

import (
	"errors"
	"log"
	"time"

	"braindance-gateway/internal/auth"
	"braindance-gateway/internal/database"
	"braindance-gateway/internal/models"
	"braindance-gateway/internal/spotify"
)

const songTTL = 30 * time.Second

// ErrUserNotFound means the spotify_id is not registered in this gateway's database.
var ErrUserNotFound = errors.New("user not found")

// SyncCurrentlyPlaying polls Spotify for the user's active track and keeps Redis in sync.
// When the track changes and location is available, records a music map event.
func SyncCurrentlyPlaying(spotifyID string) (*models.Track, error) {
	user, err := database.GetUserBySpotifyID(spotifyID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	token, err := auth.EnsureValidToken(user.ID)
	if err != nil {
		if errors.Is(err, auth.ErrReauthRequired) {
			return nil, auth.ErrReauthRequired
		}
		return nil, err
	}

	cp, err := spotify.FetchCurrentlyPlaying(token.AccessToken)
	if err != nil {
		return nil, err
	}

	if cp.IsPlaying && cp.Track != nil {
		if err := database.SetCurrentSong(spotifyID, cp.Track, songTTL); err != nil {
			log.Printf("Failed to cache song for %s: %v", spotifyID, err)
		}

		recordPlaybackEvent(user.ID, spotifyID, cp.Track)
		return cp.Track, nil
	}

	cached, _ := database.GetCurrentSong(spotifyID)
	if cached != nil {
		database.ClearCurrentSong(spotifyID)
		database.ClearMusicSession(user.ID)
	}

	return nil, nil
}

func recordPlaybackEvent(userID int, spotifyID string, track *models.Track) {
	loc, err := database.GetLocation(spotifyID)
	if err != nil || loc == nil {
		return
	}

	artistName := ""
	if len(track.Artists) > 0 {
		artistName = track.Artists[0].Name
	}

	if err := RecordMusicEvent(userID, track.ID, track.Name, artistName, loc.Y, loc.X, time.Now().UTC(), nil); err != nil {
		log.Printf("Failed to record music event for %s: %v", spotifyID, err)
	}
}

// TryRecordPlaybackForLocation saves a listening event when location arrives
// after playback was already detected (common right after WebSocket connect).
func TryRecordPlaybackForLocation(spotifyID string) {
	user, err := database.GetUserBySpotifyID(spotifyID)
	if err != nil || user == nil {
		return
	}

	track, err := database.GetCurrentSong(spotifyID)
	if err != nil || track == nil {
		return
	}

	recordPlaybackEvent(user.ID, spotifyID, track)
}
