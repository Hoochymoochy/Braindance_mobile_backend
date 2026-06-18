package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"braindance-gateway/internal/database"
)

// HandleNotificationRegister stores a device token for push notifications.
//
//	POST /api/v1/notifications/register
//	Body: {"spotify_id":"...", "token":"fcm-or-apns-token", "platform":"ios"}
func HandleNotificationRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SpotifyID string `json:"spotify_id"`
		Token     string `json:"token"`
		Platform  string `json:"platform"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.SpotifyID == "" || req.Token == "" {
		http.Error(w, "spotify_id and token are required", http.StatusBadRequest)
		return
	}
	if req.Platform == "" {
		req.Platform = "ios"
	}

	user, err := database.GetUserBySpotifyID(req.SpotifyID)
	if err != nil {
		log.Printf("Notification register: user lookup failed: %v", err)
		http.Error(w, "Failed to find user", http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if err := database.RegisterDeviceToken(user.ID, req.Token, req.Platform); err != nil {
		log.Printf("Notification register: store failed: %v", err)
		http.Error(w, "Failed to store token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandleNotificationUnregister removes a device token.
//
//	POST /api/v1/notifications/unregister
//	Body: {"spotify_id":"...", "token":"fcm-or-apns-token"}
func HandleNotificationUnregister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SpotifyID string `json:"spotify_id"`
		Token     string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.SpotifyID == "" || req.Token == "" {
		http.Error(w, "spotify_id and token are required", http.StatusBadRequest)
		return
	}

	user, err := database.GetUserBySpotifyID(req.SpotifyID)
	if err != nil {
		log.Printf("Notification unregister: user lookup failed: %v", err)
		http.Error(w, "Failed to find user", http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if err := database.UnregisterDeviceToken(user.ID, req.Token); err != nil {
		log.Printf("Notification unregister: delete failed: %v", err)
		http.Error(w, "Failed to remove token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
