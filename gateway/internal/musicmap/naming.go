package musicmap

import (
	"fmt"
	"sort"
	"time"

	"braindance-gateway/internal/models"
)

// TerritoryName generates a human-readable label for a hex profile.
func TerritoryName(profile models.MusicHexProfile, nightEventRatio float64) string {
	energy := 0.0
	if profile.AvgEnergy != nil {
		energy = *profile.AvgEnergy
	}
	discovery := 0.0
	if profile.DiscoveryScore != nil {
		discovery = *profile.DiscoveryScore
	}
	repeat := 0.0
	if profile.RepeatScore != nil {
		repeat = *profile.RepeatScore
	}

	switch {
	case nightEventRatio >= 0.5:
		return "Night Shift"
	case discovery >= 0.7:
		return "Discovery Zone"
	case repeat >= 0.7:
		return "Comfort Zone"
	case energy >= 0.75:
		return "Momentum Zone"
	case energy <= 0.35:
		return "Focus Territory"
	default:
		return "Listening Ground"
	}
}

// GenerateInsights produces rule-based insights from hex profiles.
func GenerateInsights(profiles []models.MusicHexProfile, nightRatios map[string]float64) []models.MusicInsight {
	if len(profiles) == 0 {
		return nil
	}

	sorted := make([]models.MusicHexProfile, len(profiles))
	copy(sorted, profiles)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].EventCount > sorted[j].EventCount
	})

	var insights []models.MusicInsight

	// Most listening territory
	top := sorted[0]
	name := top.TerritoryName
	if name == "" {
		name = "this territory"
	}
	insights = append(insights, models.MusicInsight{
		InsightType: "most_listening",
		H3Index:     top.H3Index,
		Message:     fmt.Sprintf("Most listening occurs in %s.", name),
	})

	// Discovery territory
	discoverySorted := make([]models.MusicHexProfile, len(profiles))
	copy(discoverySorted, profiles)
	sort.Slice(discoverySorted, func(i, j int) bool {
		di, dj := 0.0, 0.0
		if discoverySorted[i].DiscoveryScore != nil {
			di = *discoverySorted[i].DiscoveryScore
		}
		if discoverySorted[j].DiscoveryScore != nil {
			dj = *discoverySorted[j].DiscoveryScore
		}
		return di > dj
	})
	if discoverySorted[0].DiscoveryScore != nil && *discoverySorted[0].DiscoveryScore >= 0.5 {
		dName := discoverySorted[0].TerritoryName
		if dName == "" {
			dName = "this territory"
		}
		insights = append(insights, models.MusicInsight{
			InsightType: "discovery",
			H3Index:     discoverySorted[0].H3Index,
			Message:     fmt.Sprintf("You discover the most music in %s.", dName),
		})
	}

	// Highest energy territory
	energySorted := make([]models.MusicHexProfile, len(profiles))
	copy(energySorted, profiles)
	sort.Slice(energySorted, func(i, j int) bool {
		ei, ej := 0.0, 0.0
		if energySorted[i].AvgEnergy != nil {
			ei = *energySorted[i].AvgEnergy
		}
		if energySorted[j].AvgEnergy != nil {
			ej = *energySorted[j].AvgEnergy
		}
		return ei > ej
	})
	if energySorted[0].AvgEnergy != nil && *energySorted[0].AvgEnergy >= 0.6 {
		eName := energySorted[0].TerritoryName
		if eName == "" {
			eName = "this territory"
		}
		insights = append(insights, models.MusicInsight{
			InsightType: "high_energy",
			H3Index:     energySorted[0].H3Index,
			Message:     fmt.Sprintf("This is your highest-energy territory — %s.", eName),
		})
	}

	// Night shift insight
	for _, p := range profiles {
		if ratio, ok := nightRatios[p.H3Index]; ok && ratio >= 0.5 {
			nName := p.TerritoryName
			if nName == "" {
				nName = "Night Shift"
			}
			insights = append(insights, models.MusicInsight{
				InsightType: "night_shift",
				H3Index:     p.H3Index,
				Message:     fmt.Sprintf("%s comes alive after dark.", nName),
			})
			break
		}
	}

	now := time.Now()
	for i := range insights {
		insights[i].CreatedAt = now
		insights[i].UpdatedAt = now
	}

	return insights
}
