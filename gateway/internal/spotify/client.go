package spotify

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"braindance-gateway/internal/models"
)

const spotifyAPI = "https://api.spotify.com/v1"

// FetchCurrentlyPlaying retrieves the user's currently playing track.
func FetchCurrentlyPlaying(accessToken string) (*models.CurrentlyPlaying, error) {
	req, err := http.NewRequest("GET", spotifyAPI+"/me/player/currently-playing", nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// 204 means no track is currently playing
	if resp.StatusCode == http.StatusNoContent {
		return &models.CurrentlyPlaying{IsPlaying: false}, nil
	}

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify returned %d: %s", resp.StatusCode, string(body))
	}

	var cp models.CurrentlyPlaying
	if err := json.Unmarshal(body, &cp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return &cp, nil
}
