package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"braindance-gateway/internal/models"

	_ "github.com/lib/pq"
)

var db *sql.DB

// Connect opens a connection pool to PostgreSQL and verifies connectivity.
func Connect() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = fmt.Sprintf(
			"postgres://%s:%s@localhost:5432/%s?sslmode=disable",
			os.Getenv("POSTGRES_USER"),
			os.Getenv("POSTGRES_PASSWORD"),
			os.Getenv("POSTGRES_DB"),
		)
	}

	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return fmt.Errorf("pinging database: %w", err)
	}

	log.Println("Connected to PostgreSQL")
	return nil
}

// Close shuts down the database connection pool.
func Close() {
	if db != nil {
		db.Close()
	}
}

// ── User Operations ──────────────────────────────────

// UpsertUser inserts a user or updates their profile on conflict (spotify_id).
func UpsertUser(spotifyUser *models.SpotifyUser) (*models.User, error) {
	query := `
		INSERT INTO users (spotify_id, display_name, email)
		VALUES ($1, $2, $3)
		ON CONFLICT (spotify_id)
		DO UPDATE SET display_name = $2, email = $3
		RETURNING id, spotify_id, display_name, email, created_at
	`

	var user models.User
	err := db.QueryRow(query, spotifyUser.ID, spotifyUser.DisplayName, spotifyUser.Email).
		Scan(&user.ID, &user.SpotifyID, &user.DisplayName, &user.Email, &user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("upserting user: %w", err)
	}

	return &user, nil
}

// GetUserBySpotifyID retrieves a user by their Spotify ID.
func GetUserBySpotifyID(spotifyID string) (*models.User, error) {
	query := `SELECT id, spotify_id, display_name, email, created_at FROM users WHERE spotify_id = $1`

	var user models.User
	err := db.QueryRow(query, spotifyID).
		Scan(&user.ID, &user.SpotifyID, &user.DisplayName, &user.Email, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying user: %w", err)
	}

	return &user, nil
}

// ── Token Operations ─────────────────────────────────

// SaveToken stores or updates OAuth tokens for a user.
func SaveToken(userID int, token *models.TokenResponse) error {
	expiresAt := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)

	query := `
		INSERT INTO tokens (user_id, access_token, refresh_token, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id)
		DO UPDATE SET access_token = $2, refresh_token = $3, expires_at = $4
	`

	_, err := db.Exec(query, userID, token.AccessToken, token.RefreshToken, expiresAt)
	if err != nil {
		return fmt.Errorf("saving token: %w", err)
	}

	return nil
}

// GetLatestToken retrieves the most recent stored token for a user,
// even if it has already expired.
func GetLatestToken(userID int) (*models.Token, error) {
	query := `
		SELECT id, user_id, access_token, refresh_token, expires_at
		FROM tokens
		WHERE user_id = $1
		ORDER BY expires_at DESC
		LIMIT 1
	`

	var token models.Token
	err := db.QueryRow(query, userID).
		Scan(&token.ID, &token.UserID, &token.AccessToken, &token.RefreshToken, &token.ExpiresAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying latest token: %w", err)
	}

	return &token, nil
}

// SaveRefreshedToken updates a user's access token after a Spotify refresh.
// Spotify may omit a new refresh token, so the existing one is preserved.
func SaveRefreshedToken(userID int, existingRefreshToken string, refreshed *models.TokenResponse) error {
	refreshToken := existingRefreshToken
	if refreshed.RefreshToken != "" {
		refreshToken = refreshed.RefreshToken
	}

	expiresAt := time.Now().Add(time.Duration(refreshed.ExpiresIn) * time.Second)

	query := `
		INSERT INTO tokens (user_id, access_token, refresh_token, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id)
		DO UPDATE SET access_token = $2, refresh_token = $3, expires_at = $4
	`

	_, err := db.Exec(query, userID, refreshed.AccessToken, refreshToken, expiresAt)
	if err != nil {
		return fmt.Errorf("saving refreshed token: %w", err)
	}

	return nil
}

// GetValidToken retrieves a non-expired token for a user.
func GetValidToken(userID int) (*models.Token, error) {
	query := `
		SELECT id, user_id, access_token, refresh_token, expires_at
		FROM tokens
		WHERE user_id = $1 AND expires_at > NOW()
		ORDER BY expires_at DESC
		LIMIT 1
	`

	var token models.Token
	err := db.QueryRow(query, userID).
		Scan(&token.ID, &token.UserID, &token.AccessToken, &token.RefreshToken, &token.ExpiresAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying token: %w", err)
	}

	return &token, nil
}

// ── Listening History ────────────────────────────────

// SaveListeningEvent records a track play.
func SaveListeningEvent(userID int, track *models.Track) error {
	artistName := ""
	if len(track.Artists) > 0 {
		artistName = track.Artists[0].Name
	}

	albumArtURL := ""
	if len(track.Album.Images) > 0 {
		albumArtURL = track.Album.Images[0].URL
	}

	query := `
		INSERT INTO listening_history (user_id, track_id, track_name, artist_name, album_name, album_art_url)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := db.Exec(query, userID, track.ID, track.Name, artistName, track.Album.Name, albumArtURL)
	return err
}
