package social

import "braindance-gateway/internal/models"

// MatchType classifies the strength of a musical connection.
type MatchType string

const (
	MatchNone   MatchType = ""
	MatchTrack  MatchType = "track"
	MatchArtist MatchType = "artist"
)

// MatchResult describes a match between two listeners.
type MatchResult struct {
	Type       MatchType `json:"type"`
	TrackID    string    `json:"trackId,omitempty"`
	TrackName  string    `json:"trackName,omitempty"`
	ArtistID   string    `json:"artistId,omitempty"`
	ArtistName string    `json:"artistName,omitempty"`
}

// DetectMatch checks for Tier 1 (same track) and Tier 2 (same artist) matches.
// Returns nil if neither user is playing anything or no match is found.
func DetectMatch(myTrack, theirTrack *models.Track) *MatchResult {
	if myTrack == nil || theirTrack == nil {
		return nil
	}

	// Tier 1: same track.
	if myTrack.ID == theirTrack.ID {
		artistName := ""
		artistID := ""
		if len(myTrack.Artists) > 0 {
			artistName = myTrack.Artists[0].Name
			artistID = myTrack.Artists[0].ID
		}
		return &MatchResult{
			Type:       MatchTrack,
			TrackID:    myTrack.ID,
			TrackName:  myTrack.Name,
			ArtistID:   artistID,
			ArtistName: artistName,
		}
	}

	// Tier 2: shared artist (checks all artists on each track for overlap).
	if sharedArtist := findSharedArtist(myTrack.Artists, theirTrack.Artists); sharedArtist != nil {
		return &MatchResult{
			Type:       MatchArtist,
			ArtistID:   sharedArtist.ID,
			ArtistName: sharedArtist.Name,
		}
	}

	return nil
}

func findSharedArtist(a, b []models.Artist) *models.Artist {
	for i := range a {
		for j := range b {
			if a[i].ID == b[j].ID {
				return &a[i]
			}
		}
	}
	return nil
}
