package spotify

import "braindance-gateway/internal/models"

// BestImageURL returns the largest Spotify image URL, or the first if sizes are unknown.
func BestImageURL(images []models.AlbumImage) string {
	if len(images) == 0 {
		return ""
	}

	best := images[0]
	bestArea := best.Width * best.Height
	for _, img := range images[1:] {
		area := img.Width * img.Height
		if area > bestArea {
			best = img
			bestArea = area
		}
	}
	return best.URL
}
