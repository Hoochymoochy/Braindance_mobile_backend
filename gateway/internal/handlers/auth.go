package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"braindance-gateway/internal/auth"
	"braindance-gateway/internal/database"
)

// HandleLogin redirects the user to Spotify's authorization page.
// Pass ?platform=ios to receive a braindance:// redirect after authentication.
func HandleLogin(w http.ResponseWriter, r *http.Request) {
	state := ""
	if r.URL.Query().Get("platform") == "ios" {
		state = "ios"
	}

	authURL := auth.BuildAuthURL(state)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// HandleCallback processes the OAuth callback from Spotify.
func HandleCallback(w http.ResponseWriter, r *http.Request) {
	if oauthErr := r.URL.Query().Get("error"); oauthErr != "" {
		state := r.URL.Query().Get("state")
		if state == "ios" {
			redirectURL := fmt.Sprintf(
				"braindance://callback?error=%s",
				url.QueryEscape(oauthErr),
			)
			http.Redirect(w, r, redirectURL, http.StatusFound)
			return
		}
		http.Error(w, "Spotify authorization denied", http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "No authorization code received", http.StatusBadRequest)
		return
	}

	state := r.URL.Query().Get("state")

	redirectIOS := func(query string) {
		http.Redirect(w, r, "braindance://callback?"+query, http.StatusFound)
	}

	// Exchange code for token
	token, err := auth.ExchangeCodeForToken(code)
	if err != nil {
		log.Printf("Token exchange failed: %v", err)
		if state == "ios" {
			redirectIOS("error=" + url.QueryEscape("token_exchange_failed"))
			return
		}
		http.Error(w, "Failed to exchange authorization code", http.StatusInternalServerError)
		return
	}

	// Fetch Spotify profile
	spotifyUser, err := auth.GetSpotifyUser(token.AccessToken)
	if err != nil {
		log.Printf("Fetching Spotify user failed: %v", err)
		if state == "ios" {
			redirectIOS("error=" + url.QueryEscape("profile_fetch_failed"))
			return
		}
		http.Error(w, "Failed to fetch Spotify profile", http.StatusInternalServerError)
		return
	}

	// Persist user in PostgreSQL
	user, err := database.UpsertUser(spotifyUser)
	if err != nil {
		log.Printf("Saving user failed: %v", err)
		if state == "ios" {
			redirectIOS("error=" + url.QueryEscape("save_user_failed"))
			return
		}
		http.Error(w, "Failed to save user", http.StatusInternalServerError)
		return
	}

	// Store OAuth tokens
	if err := database.SaveToken(user.ID, token); err != nil {
		log.Printf("Saving token failed: %v", err)
		// Non-fatal — user is still authenticated for this session
	}

	log.Printf("User authenticated: %s (%s)", user.DisplayName, user.Email)

	if state == "ios" {
		redirectURL := fmt.Sprintf(
			"braindance://callback?spotify_id=%s&display_name=%s",
			url.QueryEscape(spotifyUser.ID),
			url.QueryEscape(spotifyUser.DisplayName),
		)
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user":          spotifyUser,
		"access_token":  token.AccessToken,
		"refresh_token": token.RefreshToken,
		"expires_in":    token.ExpiresIn,
	})
}
