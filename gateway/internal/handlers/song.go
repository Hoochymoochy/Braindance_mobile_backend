package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"braindance-gateway/internal/database"
)

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

	track, err := SyncCurrentlyPlaying(spotifyID)
	if err != nil {
		log.Printf("Spotify API error for %s: %v", spotifyID, err)
		http.Error(w, "Failed to fetch from Spotify", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if track == nil {
		w.Write([]byte("null"))
		return
	}

	json.NewEncoder(w).Encode(track)
}
