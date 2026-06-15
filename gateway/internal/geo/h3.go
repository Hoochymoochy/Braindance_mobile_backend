package geo

import (
	"os"
	"strconv"

	"github.com/uber/h3-go/v4"
)

// DefaultResolution is neighborhood-sized (~174m edge length).
const DefaultResolution = 9

// Resolution returns the configured H3 resolution (env H3_RESOLUTION, default 9).
func Resolution() int {
	if v := os.Getenv("H3_RESOLUTION"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 15 {
			return n
		}
	}
	return DefaultResolution
}

// LatLngToH3 converts coordinates to an H3 cell index string at the configured resolution.
func LatLngToH3(lat, lng float64) string {
	cell, err := h3.LatLngToCell(h3.LatLng{Lat: lat, Lng: lng}, Resolution())
	if err != nil {
		return ""
	}
	return cell.String()
}

// H3Boundary returns polygon vertices for an H3 cell as [lat, lng] pairs.
func H3Boundary(h3Index string) [][2]float64 {
	cell := h3.Cell(h3.IndexFromString(h3Index))
	boundary, err := cell.Boundary()
	if err != nil {
		return nil
	}
	points := make([][2]float64, len(boundary))
	for i, p := range boundary {
		points[i] = [2]float64{p.Lat, p.Lng}
	}
	return points
}
