package spotify

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"braindance-gateway/internal/models"
)

// ErrAudioFeaturesUnavailable means Spotify blocked audio-features for this app
// (deprecated for new apps without a pre-Nov-2024 quota extension).
var ErrAudioFeaturesUnavailable = errors.New("spotify audio features unavailable")

// FetchAudioFeatures retrieves audio features for up to 100 track IDs.
func FetchAudioFeatures(accessToken string, trackIDs []string) (map[string]models.AudioFeatures, error) {
	if len(trackIDs) == 0 {
		return map[string]models.AudioFeatures{}, nil
	}

	ids := strings.Join(trackIDs, ",")
	url := fmt.Sprintf("%s/audio-features?ids=%s", spotifyAPI, ids)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusForbidden {
		return nil, ErrAudioFeaturesUnavailable
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AudioFeatures []models.AudioFeatures `json:"audio_features"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing audio features: %w", err)
	}

	out := make(map[string]models.AudioFeatures, len(result.AudioFeatures))
	for _, f := range result.AudioFeatures {
		if f.ID != "" {
			out[f.ID] = f
		}
	}
	return out, nil
}

// FetchTrack retrieves track metadata including artist IDs.
func FetchTrack(accessToken, trackID string) (*models.Track, error) {
	req, err := http.NewRequest("GET", spotifyAPI+"/tracks/"+trackID, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify returned %d: %s", resp.StatusCode, string(body))
	}

	var track models.Track
	if err := json.Unmarshal(body, &track); err != nil {
		return nil, fmt.Errorf("parsing track: %w", err)
	}
	return &track, nil
}

// FetchArtist retrieves artist details including genres.
func FetchArtist(accessToken, artistID string) (*models.SpotifyArtist, error) {
	req, err := http.NewRequest("GET", spotifyAPI+"/artists/"+artistID, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify returned %d: %s", resp.StatusCode, string(body))
	}

	var artist models.SpotifyArtist
	if err := json.Unmarshal(body, &artist); err != nil {
		return nil, fmt.Errorf("parsing artist: %w", err)
	}
	return &artist, nil
}
