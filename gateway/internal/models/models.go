package models

import "time"

// ── Spotify OAuth ────────────────────────────────────

type SpotifyUser struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

// ── Database ─────────────────────────────────────────

type User struct {
	ID          int       `json:"id"`
	SpotifyID   string    `json:"spotify_id"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
	CreatedAt   time.Time `json:"created_at"`
}

type Token struct {
	ID           int       `json:"id"`
	UserID       int       `json:"user_id"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// ── Spotify API ──────────────────────────────────────

type CurrentlyPlaying struct {
	IsPlaying bool   `json:"is_playing"`
	Track     *Track `json:"item"`
}

type Track struct {
	ID      string       `json:"id"`
	Name    string       `json:"name"`
	Artists []Artist     `json:"artists"`
	Album   Album        `json:"album"`
	URI     string       `json:"uri"`
}

type Artist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Album struct {
	ID     string       `json:"id"`
	Name   string       `json:"name"`
	Images []AlbumImage `json:"images"`
}

type AlbumImage struct {
	URL    string `json:"url"`
	Height int    `json:"height"`
	Width  int    `json:"width"`
}

// ── Location ────────────────────────────────────────

type LocationUpdate struct {
	SpotifyID string  `json:"spotify_id"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Z         float64 `json:"z"`
	Timestamp string  `json:"timestamp"`
}

// ── API Responses ────────────────────────────────────

type AuthResponse struct {
	User         *SpotifyUser `json:"user"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresIn    int          `json:"expires_in"`
}

// ── WebSocket Messages ───────────────────────────────

// WsIncoming is what the phone sends: position data.
type WsIncoming struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// WsOutgoing is what the server sends back: the stored location.
type WsOutgoing struct {
	Type      string  `json:"type"` // "location_update" | "error" | "connected"
	SpotifyID string  `json:"spotify_id,omitempty"`
	X         float64 `json:"x,omitempty"`
	Y         float64 `json:"y,omitempty"`
	Z         float64 `json:"z,omitempty"`
	Timestamp string  `json:"timestamp,omitempty"`
	Message   string  `json:"message,omitempty"`
}
