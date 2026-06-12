package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"braindance-gateway/internal/models"
)

// BuildAuthURL constructs the Spotify authorization URL.
// state is passed through OAuth and returned on callback (e.g. "ios" for mobile).
func BuildAuthURL(state string) string {
	scope := "user-read-email user-read-currently-playing user-top-read"

	authURL := fmt.Sprintf(
		"https://accounts.spotify.com/authorize?client_id=%s&response_type=code&redirect_uri=%s&scope=%s",
		os.Getenv("SPOTIFY_CLIENT_ID"),
		url.QueryEscape(os.Getenv("REDIRECT_URI")),
		url.QueryEscape(scope),
	)

	if state != "" {
		authURL += "&state=" + url.QueryEscape(state)
	}

	return authURL
}

// ExchangeCodeForToken swaps the authorization code for an access/refresh token pair.
func ExchangeCodeForToken(code string) (*models.TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", os.Getenv("REDIRECT_URI"))

	req, err := http.NewRequest(
		"POST",
		"https://accounts.spotify.com/api/token",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("building token request: %w", err)
	}

	req.SetBasicAuth(
		os.Getenv("SPOTIFY_CLIENT_ID"),
		os.Getenv("SPOTIFY_CLIENT_SECRET"),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify returned %d: %s", resp.StatusCode, string(body))
	}

	var token models.TokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("parsing token response: %w", err)
	}

	return &token, nil
}

// GetSpotifyUser fetches the authenticated user's Spotify profile.
func GetSpotifyUser(accessToken string) (*models.SpotifyUser, error) {
	req, err := http.NewRequest("GET", "https://api.spotify.com/v1/me", nil)
	if err != nil {
		return nil, fmt.Errorf("building user request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("user request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify returned %d: %s", resp.StatusCode, string(body))
	}

	var user models.SpotifyUser
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("parsing user response: %w", err)
	}

	return &user, nil
}
