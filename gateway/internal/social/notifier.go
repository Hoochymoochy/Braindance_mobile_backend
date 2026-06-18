package social

import (
	"crypto/sha256"
	"fmt"
	"log"
	"time"

	"braindance-gateway/internal/database"
	"braindance-gateway/internal/notifications"
)

const (
	notifyInterval   = 30 * time.Second
	notifyRadius     = 100.0 // meters
	notifiedTTL      = 10 * time.Minute
	userRateLimitTTL = 5 * time.Minute
)

// StartMatchNotifier runs a background loop that detects nearby music matches
// and sends push notifications. Call once from main after Redis is connected.
func StartMatchNotifier(push notifications.Provider) {
	go func() {
		time.Sleep(15 * time.Second)
		log.Println("Match notifier started")
		runMatchCycle(push)
		ticker := time.NewTicker(notifyInterval)
		defer ticker.Stop()
		for range ticker.C {
			runMatchCycle(push)
		}
	}()
}

func runMatchCycle(push notifications.Provider) {
	members, err := database.GeoIndexMembers()
	if err != nil {
		return
	}
	if len(members) < 2 {
		return
	}

	for _, spotifyID := range members {
		// Rate limit: max 1 notification per user per cycle window.
		if userRateLimited(spotifyID) {
			continue
		}

		loc, _ := database.GetLocation(spotifyID)
		if loc == nil {
			continue
		}

		myTrack, _ := database.GetCurrentSong(spotifyID)
		if myTrack == nil {
			continue
		}

		nearby, _ := database.NearbyUsers(spotifyID, loc.X, loc.Y, notifyRadius)

		for _, n := range nearby {
			if pairNotified(spotifyID, n.SpotifyID) {
				continue
			}
			if !database.IsVisible(n.SpotifyID) {
				continue
			}

			theirTrack, _ := database.GetCurrentSong(n.SpotifyID)
			if theirTrack == nil {
				continue
			}

			match := DetectMatch(myTrack, theirTrack)
			if match == nil {
				continue
			}

			user, err := database.GetUserBySpotifyID(spotifyID)
			if err != nil || user == nil {
				continue
			}

			title, body := formatMatchNotification(match)
			data := map[string]string{
				"type":      string(match.Type),
				"sessionId": ephemeralID(n.SpotifyID),
			}
			if err := push.Send(user.ID, title, body, data); err != nil {
				log.Printf("Match notifier: push failed for user %d: %v", user.ID, err)
			}

			markPairNotified(spotifyID, n.SpotifyID)
			markUserRateLimited(spotifyID)
			break // one notification per user per cycle
		}
	}
}

func formatMatchNotification(match *MatchResult) (title, body string) {
	switch match.Type {
	case MatchTrack:
		return "🎵 Same song nearby!",
			fmt.Sprintf("Someone is also listening to %s by %s", match.TrackName, match.ArtistName)
	case MatchArtist:
		return "🎤 Shared artist nearby",
			fmt.Sprintf("Someone nearby is playing %s", match.ArtistName)
	}
	return "🎧 Music match", "Someone nearby shares your taste"
}

// ── Rate limiting ─────────────────────────────────────

func pairNotified(a, b string) bool {
	return database.RedisExists(notifiedKey(a, b))
}

func markPairNotified(a, b string) {
	database.RedisSetTTL(notifiedKey(a, b), "1", notifiedTTL)
}

func userRateLimited(spotifyID string) bool {
	return database.RedisExists(fmt.Sprintf("rate:notify:%s", spotifyID))
}

func markUserRateLimited(spotifyID string) {
	database.RedisSetTTL(fmt.Sprintf("rate:notify:%s", spotifyID), "1", userRateLimitTTL)
}

func notifiedKey(a, b string) string {
	if a < b {
		return fmt.Sprintf("notified:%s:%s", a, b)
	}
	return fmt.Sprintf("notified:%s:%s", b, a)
}

// ephemeralID mirrors handlers.ephemeralID for use in notification data.
func ephemeralID(spotifyID string) string {
	h := sha256.Sum256([]byte(spotifyID))
	return fmt.Sprintf("%x", h[:4])
}
