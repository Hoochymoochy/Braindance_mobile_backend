package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"braindance-gateway/internal/database"
	"braindance-gateway/internal/models"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true }, // allow mobile clients
}

// HandleLocationWS is the WebSocket endpoint for real-time location tracking.
// Clients connect with ?spotify_id=<id> and send JSON position updates:
//
//	{"x": 1.0, "y": 2.0, "z": 3.0}
//
// The server stores each update in Redis and echoes back the confirmed position.
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

	log.Printf("Location WebSocket connected: %s", spotifyID)

	// Send handshake so the client knows it's connected
	handshake := models.WsOutgoing{
		Type:      "connected",
		SpotifyID: spotifyID,
		Message:   "Location tracking active",
	}
	if err := conn.WriteJSON(handshake); err != nil {
		log.Printf("Handshake write failed for %s: %v", spotifyID, err)
		return
	}

	// Clean up when the client disconnects
	defer func() {
		if err := database.DeleteLocation(spotifyID); err != nil {
			log.Printf("Failed to remove location for %s: %v", spotifyID, err)
		} else {
			log.Printf("Location removed for %s (disconnected)", spotifyID)
		}
	}()

	for {
		var msg models.WsIncoming
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("WebSocket read error for %s: %v", spotifyID, err)
			}
			break
		}

		// Store the position in Redis
		loc, err := database.UpdateLocation(spotifyID, msg.X, msg.Y, msg.Z)
		if err != nil {
			log.Printf("Failed to update location for %s: %v", spotifyID, err)
			conn.WriteJSON(models.WsOutgoing{
				Type:    "error",
				Message: "Failed to store location",
			})
			continue
		}

		// Echo back the confirmed, timestamped location
		if err := conn.WriteJSON(models.WsOutgoing{
			Type:      "location_update",
			SpotifyID: loc.SpotifyID,
			X:         loc.X,
			Y:         loc.Y,
			Z:         loc.Z,
			Timestamp: loc.Timestamp,
		}); err != nil {
			log.Printf("Write failed for %s: %v", spotifyID, err)
			break
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
