package database

import "fmt"

// RegisterDeviceToken stores a push notification token for a user.
func RegisterDeviceToken(userID int, token, platform string) error {
	query := `
		INSERT INTO device_tokens (user_id, token, platform)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, token) DO NOTHING
	`
	_, err := db.Exec(query, userID, token, platform)
	if err != nil {
		return fmt.Errorf("registering device token: %w", err)
	}
	return nil
}

// UnregisterDeviceToken removes a push notification token.
func UnregisterDeviceToken(userID int, token string) error {
	_, err := db.Exec(`DELETE FROM device_tokens WHERE user_id = $1 AND token = $2`, userID, token)
	if err != nil {
		return fmt.Errorf("unregistering device token: %w", err)
	}
	return nil
}

// GetDeviceTokens returns all push tokens for a user.
func GetDeviceTokens(userID int) ([]string, error) {
	rows, err := db.Query(`SELECT token FROM device_tokens WHERE user_id = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("querying device tokens: %w", err)
	}
	defer rows.Close()

	var tokens []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, fmt.Errorf("scanning device token: %w", err)
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}
