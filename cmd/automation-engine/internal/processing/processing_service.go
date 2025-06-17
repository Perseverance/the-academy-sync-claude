package processing

import (
	"context"
	"fmt"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/automation"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/google"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/strava"
)

// ProcessingService handles the core processing logic for automation engine
// This service implements the business logic for processing user data across different time scopes
type ProcessingService struct {
	stravaClient *strava.Client
	sheetsClient *google.SheetsClient
	logger       *logger.Logger
}

// NewProcessingService creates a new processing service instance
func NewProcessingService(stravaClient *strava.Client, sheetsClient *google.SheetsClient, logger *logger.Logger) *ProcessingService {
	return &ProcessingService{
		stravaClient: stravaClient,
		sheetsClient: sheetsClient,
		logger:       logger.WithContext("component", "processing_service"),
	}
}

// DayProcessingResult represents the outcome of processing a single day
type DayProcessingResult struct {
	Date            time.Time
	Processed       bool
	SkippedReason   string
	ActivitiesFound int
	TotalDistance   float64 // in meters
	TotalTime       int     // in seconds
	Error           error
}

// ProcessPreviousDay processes the immediately preceding calendar day for a user (US025)
// This function determines "yesterday" based on the user's timezone and processes all activities for that day
func (s *ProcessingService) ProcessPreviousDay(ctx context.Context, config *automation.ProcessingConfig) (*DayProcessingResult, error) {
	s.logger.Info("Processing previous day data",
		"user_id", config.UserID,
		"timezone", config.Timezone)

	// Load user's timezone
	location, err := time.LoadLocation(config.Timezone)
	if err != nil {
		s.logger.Error("Invalid timezone",
			"timezone", config.Timezone,
			"error", err)
		return nil, fmt.Errorf("invalid timezone %s: %w", config.Timezone, err)
	}

	// Calculate yesterday in user's timezone
	now := time.Now().In(location)
	yesterday := now.AddDate(0, 0, -1)
	yesterdayStart := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, location)

	s.logger.Debug("Calculated previous day range",
		"user_id", config.UserID,
		"date", yesterdayStart.Format("2006-01-02"),
		"timezone", config.Timezone)

	return s.processSingleDay(ctx, config, yesterdayStart)
}

// ProcessTodaySoFar processes the current calendar day up to the present moment (US028)
// This function is used for manual sync triggers to get immediate updates
func (s *ProcessingService) ProcessTodaySoFar(ctx context.Context, config *automation.ProcessingConfig) (*DayProcessingResult, error) {
	s.logger.Info("Processing today's data so far",
		"user_id", config.UserID,
		"timezone", config.Timezone)

	// Load user's timezone
	location, err := time.LoadLocation(config.Timezone)
	if err != nil {
		s.logger.Error("Invalid timezone",
			"timezone", config.Timezone,
			"error", err)
		return nil, fmt.Errorf("invalid timezone %s: %w", config.Timezone, err)
	}

	// Calculate today's start in user's timezone
	now := time.Now().In(location)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)

	s.logger.Debug("Processing current day",
		"user_id", config.UserID,
		"date", todayStart.Format("2006-01-02"),
		"current_time", now.Format("15:04:05"),
		"timezone", config.Timezone)

	return s.processSingleDay(ctx, config, todayStart)
}

// ProcessLookbackPeriod processes the 7-day lookback window (US026 & US027)
// This function checks days 2-8 in the past and processes any unprocessed scheduled entries
func (s *ProcessingService) ProcessLookbackPeriod(ctx context.Context, config *automation.ProcessingConfig) ([]*DayProcessingResult, error) {
	s.logger.Info("Processing 7-day lookback period",
		"user_id", config.UserID,
		"timezone", config.Timezone)

	// Load user's timezone
	location, err := time.LoadLocation(config.Timezone)
	if err != nil {
		s.logger.Error("Invalid timezone",
			"timezone", config.Timezone,
			"error", err)
		return nil, fmt.Errorf("invalid timezone %s: %w", config.Timezone, err)
	}

	now := time.Now().In(location)
	results := make([]*DayProcessingResult, 0, 7)

	// Process days 2-8 in the past
	for daysAgo := 2; daysAgo <= 8; daysAgo++ {
		date := now.AddDate(0, 0, -daysAgo)
		dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, location)

		s.logger.Debug("Processing lookback day",
			"user_id", config.UserID,
			"date", dayStart.Format("2006-01-02"),
			"days_ago", daysAgo)

		result, err := s.processSingleDay(ctx, config, dayStart)
		if err != nil {
			s.logger.Error("Failed to process lookback day",
				"user_id", config.UserID,
				"date", dayStart.Format("2006-01-02"),
				"error", err)
			// Continue processing other days even if one fails
			result = &DayProcessingResult{
				Date:  dayStart,
				Error: err,
			}
		}

		results = append(results, result)
	}

	// Log summary
	processedCount := 0
	skippedCount := 0
	errorCount := 0
	for _, r := range results {
		if r.Error != nil {
			errorCount++
		} else if r.Processed {
			processedCount++
		} else {
			skippedCount++
		}
	}

	s.logger.Info("Lookback period processing completed",
		"user_id", config.UserID,
		"total_days", len(results),
		"processed", processedCount,
		"skipped", skippedCount,
		"errors", errorCount)

	return results, nil
}

// processSingleDay contains the core logic for processing one calendar day
// This is the heart of the automation engine that implements the BRD flowchart logic
func (s *ProcessingService) processSingleDay(ctx context.Context, config *automation.ProcessingConfig, dayStart time.Time) (*DayProcessingResult, error) {
	result := &DayProcessingResult{
		Date: dayStart,
	}

	// Calculate day end (start of next day)
	dayEnd := dayStart.AddDate(0, 0, 1)

	s.logger.Debug("Processing single day",
		"user_id", config.UserID,
		"date", dayStart.Format("2006-01-02"),
		"start_utc", dayStart.UTC().Format(time.RFC3339),
		"end_utc", dayEnd.UTC().Format(time.RFC3339))

	// Step 1: Check if day is already processed
	// TODO: Implement Google Sheets check when sheet updates are added
	// For now, we'll skip this check since we're not updating sheets yet
	isProcessed := false
	if isProcessed {
		result.Processed = false
		result.SkippedReason = "Day already processed"
		s.logger.Debug("Day already processed, skipping",
			"user_id", config.UserID,
			"date", dayStart.Format("2006-01-02"))
		return result, nil
	}

	// Step 2: Fetch Strava activities for this day
	activities, err := s.fetchActivitiesForDay(ctx, dayStart, dayEnd)
	if err != nil {
		s.logger.Error("Failed to fetch Strava activities",
			"user_id", config.UserID,
			"date", dayStart.Format("2006-01-02"),
			"error", err)
		result.Error = err
		return result, err
	}

	result.ActivitiesFound = len(activities)

	// Step 3: Aggregate activity data
	var totalDistance float64
	var totalTime int
	for _, activity := range activities {
		// Only count running activities
		if activity.Type == "Run" {
			totalDistance += activity.Distance
			totalTime += activity.MovingTime
		}
	}

	result.TotalDistance = totalDistance
	result.TotalTime = totalTime

	s.logger.Info("Day processing completed",
		"user_id", config.UserID,
		"date", dayStart.Format("2006-01-02"),
		"activities_found", result.ActivitiesFound,
		"total_distance_km", totalDistance/1000,
		"total_time_minutes", totalTime/60)

	// Step 4: Apply conditional logic and update spreadsheet
	// TODO: Implement spreadsheet updates when that functionality is added
	// For now, we'll just log what would happen
	if result.ActivitiesFound > 0 {
		s.logger.Debug("Would update spreadsheet with activity data",
			"user_id", config.UserID,
			"date", dayStart.Format("2006-01-02"),
			"distance_km", totalDistance/1000,
			"time_minutes", totalTime/60)
	} else {
		s.logger.Debug("Would update spreadsheet with zero values (no activities)",
			"user_id", config.UserID,
			"date", dayStart.Format("2006-01-02"))
	}

	result.Processed = true
	return result, nil
}

// fetchActivitiesForDay retrieves all Strava activities for a specific day
func (s *ProcessingService) fetchActivitiesForDay(ctx context.Context, dayStart, dayEnd time.Time) ([]strava.Activity, error) {
	s.logger.Debug("Fetching Strava activities for day",
		"day_start", dayStart.Format(time.RFC3339),
		"day_end", dayEnd.Format(time.RFC3339))

	// Fetch activities from Strava starting from the beginning of the day
	// Note: The Strava client will handle token refresh automatically if needed
	// We need to fetch from dayStart and then filter for activities within the day
	activities, err := s.stravaClient.GetActivities(ctx, dayStart)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch activities: %w", err)
	}

	// Filter activities to only include those that started on the specific day
	var dayActivities []strava.Activity
	for _, activity := range activities {
		// Check if activity started within our day range (inclusive of start, exclusive of end)
		if (activity.StartDate.Equal(dayStart) || activity.StartDate.After(dayStart)) && activity.StartDate.Before(dayEnd) {
			dayActivities = append(dayActivities, activity)
		}
	}

	s.logger.Debug("Filtered activities for specific day",
		"total_fetched", len(activities),
		"day_activities", len(dayActivities),
		"day", dayStart.Format("2006-01-02"))

	return dayActivities, nil
}

