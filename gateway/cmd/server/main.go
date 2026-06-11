package main

import (
	"log"
	"net/http"
	"os"

	"braindance-gateway/internal/database"
	"braindance-gateway/internal/handlers"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment")
	}

	// Connect to PostgreSQL
	if err := database.Connect(); err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer database.Close()

	// Register routes
	http.HandleFunc("/", handlers.HandleHome)
	http.HandleFunc("/login", handlers.HandleLogin)
	http.HandleFunc("/callback", handlers.HandleCallback)
	http.HandleFunc("/me", handlers.HandleMe)

	port := os.Getenv("GATEWAY_PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Gateway running on http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
