package models

import "time"

// ── Music Events ─────────────────────────────────────

type MusicEventRequest struct {
	UserID     string  `json:"userId"`
	TrackID    string  `json:"trackId"`
	TrackName  string  `json:"trackName"`
	ArtistName string  `json:"artistName"`
	Timestamp  string  `json:"timestamp"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
}

type MusicEvent struct {
	ID             int       `json:"id"`
	UserID         int       `json:"user_id"`
	TrackID        string    `json:"track_id"`
	TrackName      string    `json:"track_name"`
	ArtistName     string    `json:"artist_name"`
	Timestamp      time.Time `json:"timestamp"`
	Latitude       float64   `json:"latitude"`
	Longitude      float64   `json:"longitude"`
	H3Index        string    `json:"h3_index"`
	Energy         *float64  `json:"energy,omitempty"`
	Danceability   *float64  `json:"danceability,omitempty"`
	Valence        *float64  `json:"valence,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// ── Redis Session ────────────────────────────────────

type MusicSession struct {
	TrackID   string  `json:"trackId"`
	StartedAt string  `json:"startedAt"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// ── Hex Profiles ─────────────────────────────────────

type GenreCount struct {
	Genre string `json:"genre"`
	Count int    `json:"count"`
}

type ArtistCount struct {
	Artist string `json:"artist"`
	Count  int    `json:"count"`
}

type TrackCount struct {
	Track string `json:"track"`
	Count int    `json:"count"`
}

type MusicHexProfile struct {
	ID               int           `json:"id"`
	UserID           int           `json:"user_id"`
	H3Index          string        `json:"h3_index"`
	EventCount       int           `json:"event_count"`
	ListeningMinutes float64       `json:"listening_minutes"`
	TopGenres        []GenreCount  `json:"top_genres"`
	TopArtists       []ArtistCount `json:"top_artists"`
	TopSongs         []TrackCount  `json:"top_songs"`
	AvgEnergy        *float64      `json:"avg_energy,omitempty"`
	AvgDanceability  *float64      `json:"avg_danceability,omitempty"`
	DiscoveryScore   *float64      `json:"discovery_score,omitempty"`
	RepeatScore      *float64      `json:"repeat_score,omitempty"`
	TerritoryName    string        `json:"territory_name,omitempty"`
	TopTrackID       string        `json:"top_track_id,omitempty"`
	TopTrackName     string        `json:"top_track_name,omitempty"`
	AlbumArtURL      string        `json:"album_art_url,omitempty"`
	ArtistImageURL   string        `json:"artist_image_url,omitempty"`
	LastUpdated      time.Time     `json:"last_updated"`
}

// ── API Responses ────────────────────────────────────

type HexagonResponse struct {
	H3Index          string       `json:"h3Index"`
	Boundary         [][2]float64 `json:"boundary"`
	EventCount       int          `json:"eventCount"`
	ListeningMinutes float64      `json:"listeningMinutes"`
	TopGenre         string        `json:"topGenre,omitempty"`
	TopArtist        string        `json:"topArtist,omitempty"`
	TopArtists       []ArtistCount `json:"topArtists,omitempty"`
	TopSongs         []TrackCount  `json:"topSongs,omitempty"`
	TopTrackName     string        `json:"topTrackName,omitempty"`
	AlbumArtURL      string       `json:"albumArtUrl,omitempty"`
	ArtistImageURL   string       `json:"artistImageUrl,omitempty"`
	Energy           float64      `json:"energy,omitempty"`
	TerritoryName    string       `json:"territoryName,omitempty"`
	DiscoveryScore   float64      `json:"discoveryScore,omitempty"`
	RepeatScore      float64      `json:"repeatScore,omitempty"`
}

type MusicMapResponse struct {
	Hexagons []HexagonResponse `json:"hexagons"`
}

type MusicInsight struct {
	ID          int       `json:"id"`
	UserID      int       `json:"user_id"`
	InsightType string    `json:"insight_type"`
	H3Index     string    `json:"h3_index,omitempty"`
	Message     string    `json:"message"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type MusicInsightsResponse struct {
	Insights []MusicInsight `json:"insights"`
}

// ── Spotify Audio Features ───────────────────────────

type AudioFeatures struct {
	ID           string  `json:"id"`
	Energy       float64 `json:"energy"`
	Danceability float64 `json:"danceability"`
	Valence      float64 `json:"valence"`
}

type SpotifyArtist struct {
	ID     string       `json:"id"`
	Name   string       `json:"name"`
	Genres []string     `json:"genres"`
	Images []AlbumImage `json:"images"`
}
