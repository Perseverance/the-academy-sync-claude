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
// It finds interval patterns like "6 x 1000 / 600 (3:45 - 3:50 мин)" and updates the duration/pace in parentheses
func UpdateIntervalWorkoutDescription(desc string, laps []strava.Lap) string {
	// Match interval pattern like "6 x 1000 / 600" or "5 х 1.6 км / 400 м"
	// Supports both Latin 'x' and Cyrillic 'х'
	intervalRegex := regexp.MustCompile(`(\d+)\s*[xх]\s*([\d.,]+\s*(?:км|м)?)\s*/\s*[\d.,]+\s*(?:км|м)?`)
	matches := intervalRegex.FindStringSubmatch(desc)
	
	if len(matches) < 2 {
		return desc
	}

	// Extract number of repetitions
	var reps int
	fmt.Sscanf(matches[1], "%d", &reps)
	
	if reps <= 0 {
		return desc
	}

	// Check if we have sufficient laps (at least 2*N laps)
	requiredLaps := 2 * reps
	if len(laps) < requiredLaps {
		return desc
	}

	// Calculate work lap indices (2, 4, 6, ... which are indices 1, 3, 5, ... in 0-based array)
	workLapIndices := make([]int, 0, reps)
	for i := 1; i <= reps; i++ {
		lapIdx := i * 2 - 1 // 1->1, 2->3, 3->5 (0-based indices)
		workLapIndices = append(workLapIndices, lapIdx)
	}

	// Calculate average duration and distance of work laps
	totalDuration := 0
	totalDistance := 0.0
	for _, idx := range workLapIndices {
		totalDuration += laps[idx].MovingTime
		totalDistance += laps[idx].Distance
	}
	avgDuration := totalDuration / len(workLapIndices)
	avgDistanceKm := totalDistance / float64(len(workLapIndices)) / 1000.0

	// Format average duration
	durationMinutes := avgDuration / 60
	durationSeconds := avgDuration % 60
	avgDurationStr := fmt.Sprintf("%d:%02d мин", durationMinutes, durationSeconds)

	// Calculate and format average pace per km
	avgPace := calculatePacePerKm(avgDuration, avgDistanceKm)
	avgPaceStr := fmt.Sprintf("%s мин/км", avgPace)

	// Find the parentheses content after the interval pattern
	intervalEnd := matches[0]
	intervalEndIdx := strings.Index(desc, intervalEnd)
	if intervalEndIdx == -1 {
		return desc
	}

	// Look for parentheses after the interval pattern
	afterInterval := desc[intervalEndIdx+len(intervalEnd):]
	parenStart := strings.Index(afterInterval, "(")
	if parenStart == -1 {
		return desc
	}
	parenEnd := strings.Index(afterInterval[parenStart:], ")")
	if parenEnd == -1 {
		return desc
	}
	parenEnd += parenStart

	// Extract content within parentheses
	parenContent := afterInterval[parenStart+1 : parenEnd]

	// Check what's in the parentheses and build replacement
	hasDuration := regexp.MustCompile(`\d+:\d+\s*[-–]\s*\d+:\d+\s*мин(?:\s*[,/]|$)`).MatchString(parenContent)
	hasPace := regexp.MustCompile(`[~]?\d+:\d+(?:\s*[-–]\s*\d+:\d+)?\s*мин/км`).MatchString(parenContent)

	var newParenContent string
	if hasDuration && hasPace {
		// Both duration and pace present - replace both
		newParenContent = fmt.Sprintf("%s, %s", avgDurationStr, avgPaceStr)
	} else if hasDuration {
		// Only duration present - replace it
		newParenContent = avgDurationStr
	} else if hasPace {
		// Only pace present - replace it
		newParenContent = avgPaceStr
	} else {
		// Neither found, return original
		return desc
	}

	// Reconstruct the description
	beforeParen := desc[:intervalEndIdx+len(intervalEnd)+parenStart+1]
	afterParen := desc[intervalEndIdx+len(intervalEnd)+parenEnd:]
	
	return beforeParen + newParenContent + afterParen
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