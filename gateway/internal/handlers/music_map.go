package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"braindance-gateway/internal/database"
	"braindance-gateway/internal/geo"
	"braindance-gateway/internal/models"
	"braindance-gateway/internal/musicmap"
)

// RecordMusicEvent stores a listening event, using Redis to deduplicate active sessions.
func RecordMusicEvent(userID int, trackID, trackName, artistName string, lat, lng float64, ts time.Time, features *models.AudioFeatures) error {
	session, err := database.GetMusicSession(userID)
	if err != nil {
		return err
	}

	// Same track still playing — refresh session TTL, skip DB write.
	if session != nil && session.TrackID == trackID {
		session.Latitude = lat
		session.Longitude = lng
		return database.SetMusicSession(userID, session)
	}

	h3Index := geo.LatLngToH3(lat, lng)
	if h3Index == "" {
		return nil
	}

	event := &models.MusicEvent{
		UserID:     userID,
		TrackID:    trackID,
		TrackName:  trackName,
		ArtistName: artistName,
		Timestamp:  ts,
		Latitude:   lat,
		Longitude:  lng,
		H3Index:    h3Index,
	}
	if features != nil {
		event.Energy = &features.Energy
		event.Danceability = &features.Danceability
		event.Valence = &features.Valence
	}

	if err := database.SaveMusicEvent(event); err != nil {
		return err
	}

	musicmap.ScheduleAggregation(event.UserID)

	return database.SetMusicSession(userID, &models.MusicSession{
		TrackID:   trackID,
		StartedAt: ts.UTC().Format(time.RFC3339),
		Latitude:  lat,
		Longitude: lng,
	})
}

// HandleMusicEvent records a listening event from the client.
//
//	POST /api/v1/music/events
func HandleMusicEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.MusicEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == "" || req.TrackID == "" {
		http.Error(w, "userId and trackId are required", http.StatusBadRequest)
		return
	}

	user, err := database.GetUserBySpotifyID(req.UserID)
	if err != nil {
		log.Printf("Music event: user lookup failed: %v", err)
		http.Error(w, "Failed to find user", http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	ts, err := time.Parse(time.RFC3339, req.Timestamp)
	if err != nil {
		ts = time.Now().UTC()
	}

	if err := RecordMusicEvent(user.ID, req.TrackID, req.TrackName, req.ArtistName, req.Latitude, req.Longitude, ts, nil); err != nil {
		log.Printf("Music event: save failed: %v", err)
		http.Error(w, "Failed to save event", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandleMusicMap returns aggregated hex profiles for a user.
//
//	GET /api/v1/music/map?spotify_id=<id>
func HandleMusicMap(w http.ResponseWriter, r *http.Request) {
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

	user, err := database.GetUserBySpotifyID(spotifyID)
	if err != nil {
		http.Error(w, "Failed to find user", http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	musicmap.EnsureAggregated(user.ID)

	profiles, err := database.GetMusicHexProfiles(user.ID)
	if err != nil {
		log.Printf("Music map: fetch failed: %v", err)
		http.Error(w, "Failed to fetch map", http.StatusInternalServerError)
		return
	}

	hexagons := make([]models.HexagonResponse, 0, len(profiles))
	for _, p := range profiles {
		hex := models.HexagonResponse{
			H3Index:          p.H3Index,
			Boundary:         geo.H3Boundary(p.H3Index),
			EventCount:       p.EventCount,
			ListeningMinutes: p.ListeningMinutes,
			TerritoryName:    p.TerritoryName,
		}
		if len(p.TopGenres) > 0 {
			hex.TopGenre = p.TopGenres[0].Genre
		}
		if len(p.TopArtists) > 0 {
			hex.TopArtist = p.TopArtists[0].Artist
		}
		if p.AvgEnergy != nil {
			hex.Energy = *p.AvgEnergy
		}
		if p.DiscoveryScore != nil {
			hex.DiscoveryScore = *p.DiscoveryScore
		}
		if p.RepeatScore != nil {
			hex.RepeatScore = *p.RepeatScore
		}
		hexagons = append(hexagons, hex)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.MusicMapResponse{Hexagons: hexagons})
}

// HandleMusicInsights returns generated insights for a user.
//
//	GET /api/v1/music/insights?spotify_id=<id>
func HandleMusicInsights(w http.ResponseWriter, r *http.Request) {
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

	user, err := database.GetUserBySpotifyID(spotifyID)
	if err != nil {
		http.Error(w, "Failed to find user", http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	insights, err := database.GetMusicInsights(user.ID)
	if err != nil {
		log.Printf("Music insights: fetch failed: %v", err)
		http.Error(w, "Failed to fetch insights", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.MusicInsightsResponse{Insights: insights})
}
