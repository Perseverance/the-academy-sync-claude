package transformation

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/strava"
)

// UpdateProgressiveRunDescription updates the description for progressive runs (RPE 4.5)
// It finds and replaces pace placeholders [X:XX-X:XX] with actual lap paces
func UpdateProgressiveRunDescription(desc string, laps []strava.Lap) string {
	// If we don't have at least 2 laps, return original
	if len(laps) < 2 {
		return desc
	}

	// Find pace placeholders - can be [X:XX-X:XX] or 4:20 - 4:30 format
	placeholderRegex := regexp.MustCompile(`(\[[0-9X]:[0-9X]{2}[-\s]*[0-9X]:[0-9X]{2}\]|[0-9]:[0-9]{2}\s*[-–]\s*[0-9]:[0-9]{2})`)
	placeholders := placeholderRegex.FindAllStringIndex(desc, -1)

	// If we don't have at least 2 placeholders, return original
	if len(placeholders) < 2 {
		return desc
	}

	// Calculate pace for first two laps
	pace1 := calculatePacePerKm(laps[0].MovingTime, laps[0].Distance/1000)
	pace2 := calculatePacePerKm(laps[1].MovingTime, laps[1].Distance/1000)

	// Replace placeholders in reverse order to maintain string indices
	result := desc
	if len(placeholders) >= 2 {
		// Replace second placeholder first
		result = result[:placeholders[1][0]] + pace2 + result[placeholders[1][1]:]
		// Replace first placeholder
		result = result[:placeholders[0][0]] + pace1 + result[placeholders[0][1]:]
	}

	return result
}

// UpdateSteadyStateRunDescription updates the description for steady state runs (RPE 6)
// It finds "изграждащо бягане (" and replaces the pace placeholder within parentheses
func UpdateSteadyStateRunDescription(desc string, laps []strava.Lap) string {
	// For steady state runs, we need at least one lap to calculate pace
	// If there's only 1 lap, use that lap's pace
	// If there are 2+ laps, use the second lap (middle portion)
	if len(laps) < 1 {
		return desc
	}

	// Find the pattern "изграждащо бягане ("
	pattern := "изграждащо бягане ("
	index := strings.Index(desc, pattern)
	if index == -1 {
		return desc
	}

	// Find the closing parenthesis
	startIdx := index + len(pattern)
	endIdx := strings.Index(desc[startIdx:], ")")
	if endIdx == -1 {
		return desc
	}
	endIdx += startIdx

	// Extract the content within parentheses
	content := desc[startIdx:endIdx]

	// Find and replace pace placeholder within the content
	// Match patterns like "4:20 - 4:30 мин/км" (with the unit)
	placeholderRegex := regexp.MustCompile(`[0-9]:[0-9]{2}\s*[-–]\s*[0-9]:[0-9]{2}\s*мин/км`)
	if !placeholderRegex.MatchString(content) {
		return desc
	}

	// Calculate pace from appropriate lap
	// Use second lap if available (middle portion), otherwise use first lap
	lapIndex := 0
	if len(laps) >= 2 {
		lapIndex = 1
	}
	pace := calculatePacePerKm(laps[lapIndex].MovingTime, laps[lapIndex].Distance/1000)

	// Replace placeholder in content, keeping the unit
	newContent := placeholderRegex.ReplaceAllString(content, pace + " мин/км")

	// Reconstruct the description
	return desc[:startIdx] + newContent + desc[endIdx:]
}

// UpdateTempoRunDescription updates the description for tempo runs (RPE 7-9)
// It finds "темпово бягане (" and replaces the pace placeholder within parentheses
func UpdateTempoRunDescription(desc string, laps []strava.Lap) string {
	// For tempo runs, we need at least one lap to calculate pace
	// If there's only 1 lap, use that lap's pace
	// If there are 2+ laps, use the second lap (tempo portion)
	if len(laps) < 1 {
		return desc
	}

	// Find the pattern "темпово бягане ("
	pattern := "темпово бягане ("
	index := strings.Index(desc, pattern)
	if index == -1 {
		return desc
	}

	// Find the closing parenthesis
	startIdx := index + len(pattern)
	endIdx := strings.Index(desc[startIdx:], ")")
	if endIdx == -1 {
		return desc
	}
	endIdx += startIdx

	// Extract the content within parentheses
	content := desc[startIdx:endIdx]

	// Find and replace pace placeholder within the content
	// Match patterns like "4:20 - 4:30 мин/км" (with the unit)
	placeholderRegex := regexp.MustCompile(`[0-9]:[0-9]{2}\s*[-–]\s*[0-9]:[0-9]{2}\s*мин/км`)
	if !placeholderRegex.MatchString(content) {
		return desc
	}

	// Calculate pace from appropriate lap
	// Use second lap if available (tempo portion), otherwise use first lap
	lapIndex := 0
	if len(laps) >= 2 {
		lapIndex = 1
	}
	pace := calculatePacePerKm(laps[lapIndex].MovingTime, laps[lapIndex].Distance/1000)

	// Replace placeholder in content, keeping the unit
	newContent := placeholderRegex.ReplaceAllString(content, pace + " мин/км")

	// Reconstruct the description
	return desc[:startIdx] + newContent + desc[endIdx:]
}

// UpdateIntervalWorkoutDescription updates the description for interval workouts (RPE 7-9)
// It finds interval patterns like "3 x 1000м" and replaces the duration/distance with averages
func UpdateIntervalWorkoutDescription(desc string, laps []strava.Lap) string {
	// Match interval pattern like "3 x 1000м" or "5 x 4мин"
	intervalRegex := regexp.MustCompile(`(\d+)\s*x\s*([\d\w]+)`)
	matches := intervalRegex.FindStringSubmatch(desc)
	
	if len(matches) < 3 {
		return desc
	}

	// Extract number of repetitions
	var reps int
	fmt.Sscanf(matches[1], "%d", &reps)
	
	if reps <= 0 {
		return desc
	}

	// Calculate work lap indices (2, 4, 6, ...)
	workLapIndices := make([]int, 0, reps)
	for i := 1; i <= reps; i++ {
		lapIdx := i * 2 - 1 // 1->1, 2->3, 3->5 (0-based indices)
		if lapIdx < len(laps) {
			workLapIndices = append(workLapIndices, lapIdx)
		}
	}

	if len(workLapIndices) == 0 {
		return desc
	}

	// Calculate average duration of work laps
	totalDuration := 0
	totalDistance := 0.0
	for _, idx := range workLapIndices {
		totalDuration += laps[idx].MovingTime
		totalDistance += laps[idx].Distance
	}
	avgDuration := totalDuration / len(workLapIndices)
	avgDistance := totalDistance / float64(len(workLapIndices))

	// Determine if original was time or distance based
	originalValue := matches[2]
	var replacement string
	
	if strings.Contains(originalValue, "мин") || strings.Contains(originalValue, "сек") {
		// Time-based interval
		minutes := avgDuration / 60
		seconds := avgDuration % 60
		if minutes > 0 {
			replacement = fmt.Sprintf("%dмин %dсек", minutes, seconds)
		} else {
			replacement = fmt.Sprintf("%dсек", seconds)
		}
	} else if strings.Contains(originalValue, "м") || strings.Contains(originalValue, "км") {
		// Distance-based interval
		if avgDistance >= 1000 {
			replacement = fmt.Sprintf("%.2fкм", avgDistance/1000)
		} else {
			replacement = fmt.Sprintf("%.0fм", avgDistance)
		}
	} else {
		// Unknown format, return original
		return desc
	}

	// Replace only the duration/distance part
	return strings.Replace(desc, matches[0], fmt.Sprintf("%s x %s", matches[1], replacement), 1)
}

// calculatePacePerKm calculates pace in min:sec format per kilometer
func calculatePacePerKm(seconds int, distanceKm float64) string {
	if distanceKm == 0 {
		return "0:00"
	}

	paceSeconds := float64(seconds) / distanceKm
	minutes := int(paceSeconds / 60)
	secs := int(paceSeconds) % 60

	return fmt.Sprintf("%d:%02d", minutes, secs)
}