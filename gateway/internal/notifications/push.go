package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"braindance-gateway/internal/database"
)

// Provider sends push notifications to devices.
type Provider interface {
	Send(userID int, title, body string, data map[string]string) error
}

// FCMProvider sends push notifications via Firebase Cloud Messaging.
type FCMProvider struct {
	serverKey string
}

// NewFCMProvider creates an FCM provider using FCM_LEGACY_KEY.
func NewFCMProvider() *FCMProvider {
	return &FCMProvider{
		serverKey: os.Getenv("FCM_SERVER_KEY"),
	}
}

type fcmMessage struct {
	To           string            `json:"to"`
	Notification *fcmNotification  `json:"notification,omitempty"`
	Data         map[string]string `json:"data,omitempty"`
}

type fcmNotification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Sound string `json:"sound"`
}

// Send dispatches a push notification to all registered devices for a user.
// Silently skips if FCM_SERVER_KEY is not configured.
func (p *FCMProvider) Send(userID int, title, body string, data map[string]string) error {
	if p.serverKey == "" {
		log.Printf("FCM_SERVER_KEY not set — skipping push for user %d", userID)
		return nil
	}

	tokens, err := database.GetDeviceTokens(userID)
	if err != nil {
		return fmt.Errorf("fetching device tokens: %w", err)
	}
	if len(tokens) == 0 {
		return nil
	}

	msg := fcmMessage{
		Notification: &fcmNotification{
			Title: title,
			Body:  body,
			Sound: "default",
		},
		Data: data,
	}

	for _, token := range tokens {
		msg.To = token
		payload, _ := json.Marshal(msg)

		req, err := http.NewRequest("POST", "https://fcm.googleapis.com/fcm/send", bytes.NewReader(payload))
		if err != nil {
			log.Printf("FCM request build failed: %v", err)
			continue
		}
		req.Header.Set("Authorization", "key="+p.serverKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("FCM send failed for user %d: %v", userID, err)
			continue
		}
		resp.Body.Close()
	}

	return nil
}
