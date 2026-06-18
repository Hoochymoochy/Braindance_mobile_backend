package main

import (
	"log"
	"net/http"
	"os"

	"braindance-gateway/internal/database"
	"braindance-gateway/internal/handlers"
	"braindance-gateway/internal/musicmap"
	"braindance-gateway/internal/notifications"
	"braindance-gateway/internal/social"

	"github.com/joho/godotenv"
)

func loadEnv() {
	for _, path := range []string{"../.env", "../../../.env", ".env"} {
		if err := godotenv.Load(path); err == nil {
			log.Printf("Loaded environment from %s", path)
			return
		}
	}
	log.Println("No .env file found, using system environment")
}

func main() {
	loadEnv()

	// Connect to PostgreSQL
	if err := database.Connect(); err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer database.Close()

	// Connect to Redis
	if err := database.ConnectRedis(); err != nil {
		log.Fatalf("Redis connection failed: %v", err)
	}
	defer database.CloseRedis()

	// Register routes
	http.HandleFunc("/login", handlers.HandleLogin)
	http.HandleFunc("/callback", handlers.HandleCallback)
	http.HandleFunc("/me", handlers.HandleMe)
	http.HandleFunc("/ws", handlers.HandleLocationWS)              // WebSocket: real-time location
	http.HandleFunc("/location", handlers.HandleLocationGet)      // REST: get last-known location
	http.HandleFunc("/currently-playing", handlers.HandleCurrentlyPlaying) // Poll Spotify → Redis

	// Personal Music Map
	http.HandleFunc("/api/v1/music/events", handlers.HandleMusicEvent)
	http.HandleFunc("/api/v1/music/map", handlers.HandleMusicMap)
	http.HandleFunc("/api/v1/music/insights", handlers.HandleMusicInsights)

	// Social / AR
	http.HandleFunc("/api/v1/social/nearby", handlers.HandleSocialNearby)
	http.HandleFunc("/api/v1/social/react", handlers.HandleSocialReact)
	http.HandleFunc("/api/v1/social/reactions", handlers.HandleSocialReactions)
	http.HandleFunc("/api/v1/social/visible", handlers.HandleSocialVisible)

	// Notifications
	http.HandleFunc("/api/v1/notifications/register", handlers.HandleNotificationRegister)
	http.HandleFunc("/api/v1/notifications/unregister", handlers.HandleNotificationUnregister)

	musicmap.StartAggregationJob()

	// Start background match notifier for push notifications.
	fcm := notifications.NewFCMProvider()
	social.StartMatchNotifier(fcm)

	port := os.Getenv("GATEWAY_PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Gateway running on http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
