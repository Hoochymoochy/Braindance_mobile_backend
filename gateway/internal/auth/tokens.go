package auth

import (
	"errors"
	"fmt"
	"log"

	"braindance-gateway/internal/database"
	"braindance-gateway/internal/models"
)

var ErrReauthRequired = errors.New("spotify re-authentication required")

// EnsureValidToken returns a usable access token for the user, refreshing it
// with Spotify when the stored access token has expired.
func EnsureValidToken(userID int) (*models.Token, error) {
	token, err := database.GetValidToken(userID)
	if err != nil {
		return nil, err
	}
	if token != nil {
		return token, nil
	}

	latest, err := database.GetLatestToken(userID)
	if err != nil {
		return nil, err
	}
	if latest == nil || latest.RefreshToken == "" {
		return nil, ErrReauthRequired
	}

	refreshed, err := RefreshAccessToken(latest.RefreshToken)
	if err != nil {
		log.Printf("Token refresh failed for user %d: %v", userID, err)
		return nil, ErrReauthRequired
	}

	if err := database.SaveRefreshedToken(userID, latest.RefreshToken, refreshed); err != nil {
		return nil, fmt.Errorf("saving refreshed token: %w", err)
	}

	token, err = database.GetValidToken(userID)
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, ErrReauthRequired
	}

	log.Printf("Refreshed Spotify token for user %d", userID)
	return token, nil
}
