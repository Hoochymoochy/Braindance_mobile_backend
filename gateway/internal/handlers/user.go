package handlers

import (
	"encoding/json"
	"net/http"

	"braindance-gateway/internal/database"
)

// HandleMe returns the authenticated user's profile.
// Expects ?spotify_id=<id> query parameter (in production, use JWT/session).
func HandleMe(w http.ResponseWriter, r *http.Request) {
	spotifyID := r.URL.Query().Get("spotify_id")
	if spotifyID == "" {
		http.Error(w, "Missing spotify_id parameter", http.StatusBadRequest)
		return
	}

	user, err := database.GetUserBySpotifyID(spotifyID)
	if err != nil {
		http.Error(w, "Failed to retrieve user", http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}