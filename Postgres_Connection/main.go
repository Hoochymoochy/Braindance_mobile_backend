package main

import (
	"log"
	"net/http"

	"spotify-app/database"
)

func main() {
	database.Connect()

	log.Println("Server running on http://localhost:8081")

	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatal(err)
	}
}