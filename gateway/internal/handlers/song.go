package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"braindance-gateway/internal/database"
	"braindance-gateway/internal/spotify"
)

const songTTL = 5 * time.Second

// HandleCurrentlyPlaying polls Spotify for the user's currently playing track,
// updates Redis, and returns the result to the frontend.
//
//	GET /currently-playing?spotify_id=<id>
//
// Returns the track JSON if playing, null body if not.
func HandleCurrentlyPlaying(w http.ResponseWriter, r *http.Request) {
	spotifyID := r.URL.Query().Get("spotify_id")
	if spotifyID == "" {
		http.Error(w, "Missing spotify_id parameter", http.StatusBadRequest)
		return
	}

	// Look up user in Postgres
	user, err := database.GetUserBySpotifyID(spotifyID)
	if err != nil {
		log.Printf("Failed to find user %s: %v", spotifyID, err)
		http.Error(w, "Failed to find user", http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Get a valid Spotify access token
	token, err := database.GetValidToken(user.ID)
	if err != nil {
		log.Printf("Failed to get token for user %d: %v", user.ID, err)
		http.Error(w, "Failed to retrieve token", http.StatusInternalServerError)
		return
	}
	if token == nil {
		http.Error(w, "No valid Spotify token — re-authenticate", http.StatusUnauthorized)
		return
	}

	// Poll Spotify
	cp, err := spotify.FetchCurrentlyPlaying(token.AccessToken)
	if err != nil {
		log.Printf("Spotify API error for %s: %v", spotifyID, err)
		http.Error(w, "Failed to fetch from Spotify", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Playing → store track in Redis, return it
	if cp.IsPlaying && cp.Track != nil {
		if err := database.SetCurrentSong(spotifyID, cp.Track, songTTL); err != nil {
			log.Printf("Failed to cache song for %s: %v", spotifyID, err)
		}
		json.NewEncoder(w).Encode(cp.Track)
		return
	}

	// Not playing → only clear Redis if it had a song before (avoid unnecessary writes)
	cached, _ := database.GetCurrentSong(spotifyID)
	if cached != nil {
		database.ClearCurrentSong(spotifyID)
	}

	// Return null — frontend knows user isn't listening
	w.Write([]byte("null"))
}
