package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"braindance-gateway/internal/auth"
	"braindance-gateway/internal/database"
)

// HandleHome renders the landing page with a Spotify login button.
func HandleHome(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`
		<h1>Braindance Nearby</h1>
		<p>Real-time music discovery powered by Spotify</p>
		<a href="/login">
			<button>Login with Spotify</button>
		</a>
	`))
}

// HandleLogin redirects the user to Spotify's authorization page.
func HandleLogin(w http.ResponseWriter, r *http.Request) {
	authURL := auth.BuildAuthURL()
	http.Redirect(w, r, authURL, http.StatusFound)
}

// HandleCallback processes the OAuth callback from Spotify.
func HandleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "No authorization code received", http.StatusBadRequest)
		return
	}

	// Exchange code for token
	token, err := auth.ExchangeCodeForToken(code)
	if err != nil {
		log.Printf("Token exchange failed: %v", err)
		http.Error(w, "Failed to exchange authorization code", http.StatusInternalServerError)
		return
	}

	// Fetch Spotify profile
	spotifyUser, err := auth.GetSpotifyUser(token.AccessToken)
	if err != nil {
		log.Printf("Fetching Spotify user failed: %v", err)
		http.Error(w, "Failed to fetch Spotify profile", http.StatusInternalServerError)
		return
	}

	// Persist user in PostgreSQL
	user, err := database.UpsertUser(spotifyUser)
	if err != nil {
		log.Printf("Saving user failed: %v", err)
		http.Error(w, "Failed to save user", http.StatusInternalServerError)
		return
	}

	// Store OAuth tokens
	if err := database.SaveToken(user.ID, token); err != nil {
		log.Printf("Saving token failed: %v", err)
		// Non-fatal — user is still authenticated for this session
	}

	log.Printf("User authenticated: %s (%s)", user.DisplayName, user.Email)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user":          spotifyUser,
		"access_token":  token.AccessToken,
		"refresh_token": token.RefreshToken,
		"expires_in":    token.ExpiresIn,
	})
}
