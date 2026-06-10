package database

import (
	"fmt"

	"spotify-app/models"
)

func Connect() {
	fmt.Println("Connecting to PostgreSQL database...")
}

func SaveUser(user *models.SpotifyUser) {
	fmt.Printf("Saving user %s to database...\n", user.DisplayName)
}

func GetUser(userID string) *models.SpotifyUser {
	fmt.Printf("Retrieving user with ID %s from database...\n", userID)

	return &models.SpotifyUser{
		ID:          userID,
		DisplayName: "Mock User",
		Email:       "f0B0w@example.com",
	}
}

func UpdateRefreshToken(userID, refreshToken string) {
	fmt.Printf("Updating refresh token for user %s...\n", userID)
}