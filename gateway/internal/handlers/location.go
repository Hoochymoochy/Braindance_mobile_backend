package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"braindance-gateway/internal/auth"
	"braindance-gateway/internal/database"
	"braindance-gateway/internal/models"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true }, // allow mobile clients
}

const (
	wsReadTimeout  = 90 * time.Second
	wsPingInterval = 30 * time.Second
	wsPollInterval = 5 * time.Second
)

// HandleLocationWS is the unified WebSocket for location tracking and playback state.
// Clients connect with ?spotify_id=<id> and can send:
//
//	{"x": 1.0, "y": 2.0, "z": 3.0}           — location (lng, lat, altitude)
//	{"type": "location", "x": ..., "y": ..., "z": ...}
//	{"type": "ping"}                          — keepalive
//
// The server pushes:
//   - connected, location_update, pong, error
//   - currently_playing (with track or null)
func HandleLocationWS(w http.ResponseWriter, r *http.Request) {
	spotifyID := r.URL.Query().Get("spotify_id")
	if spotifyID == "" {
		http.Error(w, "Missing spotify_id query parameter", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed for %s: %v", spotifyID, err)
		return
	}
	defer conn.Close()

	log.Printf("WebSocket connected: %s", spotifyID)

	var writeMu sync.Mutex
	writeJSON := func(v any) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteJSON(v)
	}

	conn.SetReadDeadline(time.Now().Add(wsReadTimeout))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsReadTimeout))
	})

	handshake := models.WsOutgoing{
		Type:      "connected",
		SpotifyID: spotifyID,
		Message:   "Session active",
	}
	if err := writeJSON(handshake); err != nil {
		log.Printf("Handshake write failed for %s: %v", spotifyID, err)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	defer func() {
		cancel()
		if err := database.DeleteLocation(spotifyID); err != nil {
			log.Printf("Failed to remove location for %s: %v", spotifyID, err)
		} else {
			log.Printf("Location removed for %s (disconnected)", spotifyID)
		}
	}()

	go runWebSocketPingLoop(ctx, cancel, conn, &writeMu)
	go runCurrentlyPlayingLoop(ctx, spotifyID, writeJSON)

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("WebSocket read error for %s: %v", spotifyID, err)
			}
			break
		}

		if err := conn.SetReadDeadline(time.Now().Add(wsReadTimeout)); err != nil {
			log.Printf("Failed to extend read deadline for %s: %v", spotifyID, err)
			break
		}

		var msg models.WsIncoming
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "ping":
			if err := writeJSON(models.WsOutgoing{Type: "pong"}); err != nil {
				log.Printf("Pong write failed for %s: %v", spotifyID, err)
				return
			}
			continue
		case "location", "":
			// Fall through to location handling.
		default:
			if err := writeJSON(models.WsOutgoing{
				Type:    "error",
				Message: "Unknown message type",
			}); err != nil {
				return
			}
			continue
		}

		loc, err := database.UpdateLocation(spotifyID, msg.X, msg.Y, msg.Z)
		if err != nil {
			log.Printf("Failed to update location for %s: %v", spotifyID, err)
			if err := writeJSON(models.WsOutgoing{
				Type:    "error",
				Message: "Failed to store location",
			}); err != nil {
				return
			}
			continue
		}

		if err := writeJSON(models.WsOutgoing{
			Type:      "location_update",
			SpotifyID: loc.SpotifyID,
			X:         loc.X,
			Y:         loc.Y,
			Z:         loc.Z,
			Timestamp: loc.Timestamp,
		}); err != nil {
			log.Printf("Location write failed for %s: %v", spotifyID, err)
			return
		}
	}
}

func runWebSocketPingLoop(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn, writeMu *sync.Mutex) {
	ticker := time.NewTicker(wsPingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			writeMu.Lock()
			err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second))
			writeMu.Unlock()
			if err != nil {
				cancel()
				return
			}
		}
	}
}

func runCurrentlyPlayingLoop(ctx context.Context, spotifyID string, writeJSON func(any) error) {
	var lastTrackID string
	var authErrorSent bool
	var userErrorSent bool

	push := func() {
		track, err := SyncCurrentlyPlaying(spotifyID)
		if errors.Is(err, ErrUserNotFound) {
			if !userErrorSent {
				log.Printf("User %s not in local database — re-login required", spotifyID)
				if writeErr := writeJSON(models.WsOutgoing{
					Type:    "error",
					Message: "Not logged in on this server — open Settings, log out, and sign in again.",
				}); writeErr != nil {
					log.Printf("User error write failed for %s: %v", spotifyID, writeErr)
				}
				userErrorSent = true
			}
			return
		}
		if errors.Is(err, auth.ErrReauthRequired) {
			if !authErrorSent {
				log.Printf("Spotify re-auth required for %s", spotifyID)
				if writeErr := writeJSON(models.WsOutgoing{
					Type:    "error",
					Message: "Spotify session expired — log in again.",
				}); writeErr != nil {
					log.Printf("Auth error write failed for %s: %v", spotifyID, writeErr)
				}
				authErrorSent = true
			}
			return
		}
		if err != nil {
			log.Printf("Currently playing sync failed for %s: %v", spotifyID, err)
			return
		}

		authErrorSent = false
		userErrorSent = false

		trackID := ""
		if track != nil {
			trackID = track.ID
		}
		lastTrackID = trackID

		if err := writeJSON(map[string]any{
			"type":       "currently_playing",
			"spotify_id": spotifyID,
			"track":      track,
		}); err != nil {
			log.Printf("Currently playing write failed for %s: %v", spotifyID, err)
		}
	}

	push()

	ticker := time.NewTicker(wsPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			track, err := SyncCurrentlyPlaying(spotifyID)
			if errors.Is(err, ErrUserNotFound) || errors.Is(err, auth.ErrReauthRequired) {
				push()
				continue
			}
			if err != nil {
				log.Printf("Currently playing sync failed for %s: %v", spotifyID, err)
				continue
			}

			trackID := ""
			if track != nil {
				trackID = track.ID
			}
			if trackID == lastTrackID {
				continue
			}

			push()
		}
	}
}

// HandleLocationGet is a REST fallback to fetch a user's last-known location.
// GET /location?spotify_id=<id>
func HandleLocationGet(w http.ResponseWriter, r *http.Request) {
	spotifyID := r.URL.Query().Get("spotify_id")
	if spotifyID == "" {
		http.Error(w, "Missing spotify_id parameter", http.StatusBadRequest)
		return
	}

	loc, err := database.GetLocation(spotifyID)
	if err != nil {
		log.Printf("Failed to get location for %s: %v", spotifyID, err)
		http.Error(w, "Failed to retrieve location", http.StatusInternalServerError)
		return
	}
	if loc == nil {
		http.Error(w, "No location found (expired or never set)", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(loc)
}
