package handlers

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"

	"braindance-gateway/internal/database"
	"braindance-gateway/internal/social"
)

const defaultNearbyRadius = 100 // meters

// Bubble represents an anonymized nearby listener for the AR view.
type Bubble struct {
	SessionID      string  `json:"sessionId"`
	Distance       float64 `json:"distance"`       // meters, rounded
	Direction      string  `json:"direction"`      // N, NE, E, SE, S, SW, W, NW
	BearingRadians float64 `json:"bearingRadians"` // exact radians clockwise from north
	TrackName      string  `json:"trackName"`
	ArtistName     string  `json:"artistName"`
	AlbumArtURL    string  `json:"albumArtUrl,omitempty"`
	MatchType      string  `json:"matchType,omitempty"`      // "track" | "artist" | ""
	MatchTrackName string  `json:"matchTrackName,omitempty"` // shared track/artist name
}

// nearbyResponse is the JSON body for GET /api/v1/social/nearby.
type nearbyResponse struct {
	Bubbles []Bubble `json:"bubbles"`
}

// HandleSocialNearby returns anonymized listener bubbles for the AR view.
//
//	GET /api/v1/social/nearby?spotify_id=<id>&lat=<lat>&lng=<lng>&radius=<m>
func HandleSocialNearby(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	spotifyID := r.URL.Query().Get("spotify_id")
	if spotifyID == "" {
		spotifyID = r.URL.Query().Get("userId")
	}
	if spotifyID == "" {
		http.Error(w, "Missing spotify_id parameter", http.StatusBadRequest)
		return
	}

	latStr := r.URL.Query().Get("lat")
	lngStr := r.URL.Query().Get("lng")
	if latStr == "" || lngStr == "" {
		http.Error(w, "Missing lat/lng parameters", http.StatusBadRequest)
		return
	}

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		http.Error(w, "Invalid lat parameter", http.StatusBadRequest)
		return
	}
	lng, err := strconv.ParseFloat(lngStr, 64)
	if err != nil {
		http.Error(w, "Invalid lng parameter", http.StatusBadRequest)
		return
	}

	radius := defaultNearbyRadius
	if rStr := r.URL.Query().Get("radius"); rStr != "" {
		if r, err := strconv.Atoi(rStr); err == nil && r > 0 && r <= 500 {
			radius = r
		}
	}

	// Get my currently-playing track for match detection.
	// User lookup is best-effort — still return nearby results even if the
	// requesting user isn't registered in PostgreSQL (e.g., mock/test users).
	myTrack, _ := database.GetCurrentSong(spotifyID)

	// Find nearby active users via Redis GEO index.
	nearby, err := database.NearbyUsers(spotifyID, lng, lat, float64(radius))
	if err != nil {
		log.Printf("Social nearby: georadius failed: %v", err)
		http.Error(w, "Failed to query nearby users", http.StatusInternalServerError)
		return
	}

	bubbles := make([]Bubble, 0, len(nearby))
	for _, n := range nearby {
		if !database.IsVisible(n.SpotifyID) {
			continue
		}

		theirTrack, err := database.GetCurrentSong(n.SpotifyID)
		if err != nil {
			log.Printf("Social nearby: failed to get song for %s: %v", n.SpotifyID, err)
			continue
		}
		if theirTrack == nil {
			// Not currently playing — skip.
			continue
		}

		match := social.DetectMatch(myTrack, theirTrack)

		artistName := ""
		if len(theirTrack.Artists) > 0 {
			artistName = theirTrack.Artists[0].Name
		}

		albumArtURL := ""
		if len(theirTrack.Album.Images) > 0 {
			albumArtURL = theirTrack.Album.Images[0].URL
		}

		dir, bearing := bearingToCardinal(lng, lat, n.Longitude, n.Latitude)

		b := Bubble{
			SessionID:      ephemeralID(n.SpotifyID),
			Distance:       math.Round(n.Distance/5) * 5, // round to nearest 5m
			Direction:      dir,
			BearingRadians: bearing,
			TrackName:      theirTrack.Name,
			ArtistName:     artistName,
			AlbumArtURL:    albumArtURL,
		}

		if match != nil {
			b.MatchType = string(match.Type)
			switch match.Type {
			case social.MatchTrack:
				b.MatchTrackName = match.TrackName
			case social.MatchArtist:
				b.MatchTrackName = match.ArtistName
			}
		}

		bubbles = append(bubbles, b)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nearbyResponse{Bubbles: bubbles})
}

// ephemeralID returns a short, consistent but non-reversible ID derived from spotify_id.
func ephemeralID(spotifyID string) string {
	h := sha256.Sum256([]byte(spotifyID))
	return fmt.Sprintf("%x", h[:4]) // 8 hex chars
}

var allowedEmojis = map[string]bool{
	"🔥": true,
	"💜": true,
	"👀": true,
	"🎵": true,
	"💯": true,
}

// reactRequest is the JSON body for POST /api/v1/social/react.
type reactRequest struct {
	SessionID string `json:"sessionId"`
	Emoji     string `json:"emoji"`
}

// HandleSocialVisible opts the user in or out of appearing in nearby results.
//
//	POST /api/v1/social/visible
//	Body: {"spotify_id":"...", "visible":true}
func HandleSocialVisible(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SpotifyID string `json:"spotify_id"`
		Visible   bool   `json:"visible"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.SpotifyID == "" {
		http.Error(w, "Missing spotify_id", http.StatusBadRequest)
		return
	}

	if err := database.SetVisibility(req.SpotifyID, req.Visible); err != nil {
		log.Printf("Social visible: store failed: %v", err)
		http.Error(w, "Failed to update visibility", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":  "ok",
		"visible": req.Visible,
	})
}

// HandleSocialReact stores an emoji reaction for a nearby listener.
//
//	POST /api/v1/social/react
func HandleSocialReact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req reactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.SessionID == "" || req.Emoji == "" {
		http.Error(w, "sessionId and emoji are required", http.StatusBadRequest)
		return
	}
	if !allowedEmojis[req.Emoji] {
		http.Error(w, "Invalid emoji", http.StatusBadRequest)
		return
	}

	if err := database.AddReaction(req.SessionID, req.Emoji); err != nil {
		log.Printf("Social react: store failed: %v", err)
		http.Error(w, "Failed to store reaction", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// reactionsResponse is the JSON body for GET /api/v1/social/reactions.
type reactionsResponse struct {
	Reactions []string `json:"reactions"`
}

// HandleSocialReactions returns and clears pending reactions for the caller.
//
//	GET /api/v1/social/reactions?spotify_id=<id>
func HandleSocialReactions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	spotifyID := r.URL.Query().Get("spotify_id")
	if spotifyID == "" {
		spotifyID = r.URL.Query().Get("userId")
	}
	if spotifyID == "" {
		http.Error(w, "Missing spotify_id parameter", http.StatusBadRequest)
		return
	}

	sessionID := ephemeralID(spotifyID)
	reactions, err := database.DrainReactions(sessionID)
	if err != nil {
		log.Printf("Social reactions: fetch failed: %v", err)
		http.Error(w, "Failed to fetch reactions", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reactionsResponse{Reactions: reactions})
}

// bearingToCardinal returns the cardinal/intercardinal direction AND the exact
// bearing in radians (clockwise from north) from point A to point B.
// Longitude difference is scaled by cos(latitude) so the bearing is accurate
// even at higher latitudes — critical for AR bubble placement.
func bearingToCardinal(fromLng, fromLat, toLng, toLat float64) (cardinal string, radians float64) {
	// Convert to radians for trig
	fromLatRad := fromLat * math.Pi / 180
	toLatRad := toLat * math.Pi / 180
	dLngRad := (toLng - fromLng) * math.Pi / 180
	dLatRad := (toLat - fromLat) * math.Pi / 180

	// Scale east/west difference by cosine of mean latitude so that
	// atan2(east, north) gives a geometrically correct bearing.
	meanLat := (fromLatRad + toLatRad) / 2
	east := dLngRad * math.Cos(meanLat)
	north := dLatRad

	radians = math.Atan2(east, north) // 0=north, positive=clockwise
	if radians < 0 {
		radians += 2 * math.Pi
	}

	degrees := radians * 180 / math.Pi

	switch {
	case degrees >= 337.5 || degrees < 22.5:
		cardinal = "N"
	case degrees >= 22.5 && degrees < 67.5:
		cardinal = "NE"
	case degrees >= 67.5 && degrees < 112.5:
		cardinal = "E"
	case degrees >= 112.5 && degrees < 157.5:
		cardinal = "SE"
	case degrees >= 157.5 && degrees < 202.5:
		cardinal = "S"
	case degrees >= 202.5 && degrees < 247.5:
		cardinal = "SW"
	case degrees >= 247.5 && degrees < 292.5:
		cardinal = "W"
	default:
		cardinal = "NW"
	}
	return cardinal, radians
}
