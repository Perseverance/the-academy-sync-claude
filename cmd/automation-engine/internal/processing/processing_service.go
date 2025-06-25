package processing

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/automation"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/strava"
)

// ProcessingService handles the core processing logic for automation engine
// This service implements the business logic for processing user data across different time scopes
type ProcessingService struct {
	stravaClient StravaClient
	sheetsClient SheetsClient
	logger       *logger.Logger
}

// NewProcessingService creates a new processing service instance
func NewProcessingService(stravaClient StravaClient, sheetsClient SheetsClient, logger *logger.Logger) *ProcessingService {
	return &ProcessingService{
		stravaClient: stravaClient,
		sheetsClient: sheetsClient,
		logger:       logger.WithContext("component", "processing_service"),
	}
}

// TrainingPlanEntry represents a single day's training plan from the spreadsheet
type TrainingPlanEntry struct {
	Date         time.Time
	ActivityType string   // Column D: "Почивка" (Rest) or "Бягане" (Run)
	Description  string   // Column J: "Описание на тренировката"
	RPE          int      // Column I: RPE value
	IsProcessed  bool     // Based on bold formatting in Column J
	Distance     *float64 // Column E: Current distance (may be nil/empty)
	Time         *string  // Column F: Current time (may be empty)
	Row          int      // Spreadsheet row number for updates
}

// ProcessedActivity represents a Strava activity with processed data
type ProcessedActivity struct {
	StravaActivity strava.Activity
	ProcessedData  struct {
		Distance float64 // Rounded to 0.05km
		Duration string  // Formatted "HH:MM:SS"
	}
}

// SpreadsheetUpdate represents the changes to be made to a spreadsheet row
type SpreadsheetUpdate struct {
	Row              int
	DistanceValue    string // Column E: "0" or formatted distance
	TimeValue        string // Column F: "00:00:00" or formatted time
	RPEValue         int    // Column I: Updated RPE (e.g., 2 for rest day)
	DescriptionValue string // Column J: Updated description
	DescriptionBold  bool   // Column J: Make bold to mark processed
}

// DayProcessingResult represents the outcome of processing a single day
type DayProcessingResult struct {
	Date               time.Time
	Processed          bool
	SkippedReason      string
	ActivitiesFound    int
	TotalDistance      float64 // in meters
	TotalTime          int     // in seconds
	Activities         []ProcessedActivity
	PlanEntry          *TrainingPlanEntry
	SpreadsheetUpdate  *SpreadsheetUpdate
	Error              error
}

// TrainingPlanCache maps date strings (YYYY-MM-DD) to training plan entries
type TrainingPlanCache map[string]*TrainingPlanEntry

// StravaActivitiesCache maps date strings (YYYY-MM-DD) to activities for that day
type StravaActivitiesCache map[string][]strava.Activity

// ProcessPreviousDay processes the immediately preceding calendar day for a user (US025)
// This function determines "yesterday" based on the user's timezone and processes all activities for that day
func (s *ProcessingService) ProcessPreviousDay(ctx context.Context, config *automation.ProcessingConfig, trainingCache TrainingPlanCache, activitiesCache StravaActivitiesCache) (*DayProcessingResult, error) {
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

	return s.processSingleDay(ctx, config, yesterdayStart, trainingCache, activitiesCache)
}

// ProcessTodaySoFar processes the current calendar day up to the present moment (US028)
// This function is used for manual sync triggers to get immediate updates
func (s *ProcessingService) ProcessTodaySoFar(ctx context.Context, config *automation.ProcessingConfig, trainingCache TrainingPlanCache, activitiesCache StravaActivitiesCache) (*DayProcessingResult, error) {
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

	return s.processSingleDay(ctx, config, todayStart, trainingCache, activitiesCache)
}

// ProcessLookbackPeriod processes the 7-day lookback window (US026 & US027)
// This function checks days 2-8 in the past and processes any unprocessed scheduled entries
func (s *ProcessingService) ProcessLookbackPeriod(ctx context.Context, config *automation.ProcessingConfig, trainingCache TrainingPlanCache, activitiesCache StravaActivitiesCache) ([]*DayProcessingResult, error) {
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

		result, err := s.processSingleDay(ctx, config, dayStart, trainingCache, activitiesCache)
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
func (s *ProcessingService) processSingleDay(ctx context.Context, config *automation.ProcessingConfig, dayStart time.Time, trainingCache TrainingPlanCache, activitiesCache StravaActivitiesCache) (*DayProcessingResult, error) {
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

	// Step 1: Fetch training plan for this day from cache
	planEntry, err := s.fetchTrainingPlanEntry(ctx, config, dayStart, trainingCache)
	if err != nil {
		s.logger.Error("Failed to fetch training plan",
			"user_id", config.UserID,
			"date", dayStart.Format("2006-01-02"),
			"error", err)
		result.Error = fmt.Errorf("failed to fetch training plan: %w", err)
		return result, result.Error
	}

	if planEntry == nil {
		result.Processed = false
		result.SkippedReason = "No training plan entry for this day"
		s.logger.Debug("No training plan entry found for day, skipping",
			"user_id", config.UserID,
			"date", dayStart.Format("2006-01-02"))
		return result, nil
	}

	// Store plan entry in result
	result.PlanEntry = planEntry

	// Log fetched training plan
	s.logger.Debug("Fetched training plan entry",
		"user_id", config.UserID,
		"date", dayStart.Format("2006-01-02"),
		"row", planEntry.Row,
		"activity_type", planEntry.ActivityType,
		"description", planEntry.Description,
		"rpe", planEntry.RPE,
		"is_processed", planEntry.IsProcessed,
		"has_distance", planEntry.Distance != nil,
		"has_time", planEntry.Time != nil)

	// Check if already processed (based on bold text)
	if planEntry.IsProcessed {
		result.Processed = false
		result.SkippedReason = "Day already processed (bold text found)"
		s.logger.Info("Skipping day - already processed",
			"user_id", config.UserID,
			"date", dayStart.Format("2006-01-02"),
			"row", planEntry.Row,
			"reason", "bold_text_found")
		return result, nil
	}

	// Step 2: Fetch Strava activities for this day from cache
	activities, err := s.fetchActivitiesForDay(ctx, dayStart, activitiesCache)
	if err != nil {
		s.logger.Error("Failed to fetch Strava activities",
			"user_id", config.UserID,
			"date", dayStart.Format("2006-01-02"),
			"error", err)
		result.Error = err
		return result, err
	}

	result.ActivitiesFound = len(activities)

	// Step 3: Check if there's a scheduled run for this day
	hasScheduledRun := planEntry.ActivityType == "Бягане"
	
	// Step 4: Determine if we should process this day
	// Process if either:
	// - There are activities to record
	// - There's a scheduled run (even with no activities, we need to mark it as '0')
	shouldProcess := result.ActivitiesFound > 0 || hasScheduledRun
	
	if !shouldProcess {
		result.Processed = false
		result.SkippedReason = "No activities and no scheduled run"
		s.logger.Debug("No activities found and no scheduled run, skipping",
			"user_id", config.UserID,
			"date", dayStart.Format("2006-01-02"))
		return result, nil
	}

	// Step 5: Process activities and prepare spreadsheet update
	processedActivities, totalDistance, totalTime := s.processActivities(activities)
	result.Activities = processedActivities
	result.TotalDistance = totalDistance
	result.TotalTime = totalTime

	// Step 6: Prepare spreadsheet update
	spreadsheetUpdate := s.prepareSpreadsheetUpdate(planEntry, processedActivities, totalDistance, totalTime)
	result.SpreadsheetUpdate = spreadsheetUpdate

	// Log the prepared update
	s.logger.Info("Prepared spreadsheet update",
		"user_id", config.UserID,
		"date", dayStart.Format("2006-01-02"),
		"row", spreadsheetUpdate.Row,
		"updates", map[string]interface{}{
			"distance":         spreadsheetUpdate.DistanceValue,
			"time":            spreadsheetUpdate.TimeValue,
			"rpe":             spreadsheetUpdate.RPEValue,
			"description":     spreadsheetUpdate.DescriptionValue,
			"description_bold": spreadsheetUpdate.DescriptionBold,
		},
		"special_case", func() string {
			if planEntry.ActivityType == "Почивка" && len(activities) > 0 {
				return "rest_day_with_activity"
			}
			return "none"
		}(),
		"activities_found", result.ActivitiesFound,
		"total_distance_km", totalDistance/1000,
		"total_time_minutes", totalTime/60)

	result.Processed = true
	return result, nil
}

// FetchAllStravaActivities retrieves all Strava activities for a date range and returns them as a cache
func (s *ProcessingService) FetchAllStravaActivities(ctx context.Context, location *time.Location, startDate, endDate time.Time) (StravaActivitiesCache, error) {
	s.logger.Info("Fetching all Strava activities for date range",
		"start_date", startDate.Format("2006-01-02"),
		"end_date", endDate.Format("2006-01-02"),
		"timezone", location.String())

	// Make a single API call to fetch all activities from the start date
	// Note: Strava API returns activities after the given timestamp
	activities, err := s.stravaClient.GetActivities(ctx, startDate)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch activities: %w", err)
	}

	// Count activities by type for logging
	activityTypeCounts := make(map[string]int)
	for _, activity := range activities {
		activityTypeCounts[activity.Type]++
	}

	// Build cache by organizing activities by date
	cache := make(StravaActivitiesCache)
	
	// Pre-populate cache with empty slices for all dates in range
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		dateKey := d.Format("2006-01-02")
		cache[dateKey] = []strava.Activity{}
	}

	// Organize activities by their start date - only include runs
	for _, activity := range activities {
		// Skip non-running activities
		if activity.Type != "Run" {
			continue
		}
		
		// Convert to user's timezone for proper day assignment
		activityLocalTime := activity.StartDate.In(location)
		activityDayStart := time.Date(activityLocalTime.Year(), activityLocalTime.Month(), activityLocalTime.Day(), 0, 0, 0, 0, location)
		dateKey := activityDayStart.Format("2006-01-02")
		
		
		// Only include if within our date range
		// Note: We use >= startDate and <= endDate (inclusive on both ends)
		if !activityDayStart.Before(startDate) && !activityDayStart.After(endDate) {
			cache[dateKey] = append(cache[dateKey], activity)
		}
	}


	s.logger.Info("Organized Strava running activities into cache",
		"total_days", len(cache),
		"total_runs_in_range", func() int {
			count := 0
			for _, activities := range cache {
				count += len(activities)
			}
			return count
		}(),
		"filtered_out_non_runs", len(activities) - activityTypeCounts["Run"])

	return cache, nil
}

// fetchActivitiesForDay retrieves activities for a specific day from the cache
func (s *ProcessingService) fetchActivitiesForDay(ctx context.Context, dayStart time.Time, cache StravaActivitiesCache) ([]strava.Activity, error) {
	dateKey := dayStart.Format("2006-01-02")
	
	if activities, ok := cache[dateKey]; ok {
		return activities, nil
	}
	
	// Return empty slice if not in cache
	return []strava.Activity{}, nil
}

// FetchAllTrainingPlanEntries retrieves all training plan entries from the spreadsheet
// and returns them as a cache mapped by date string (YYYY-MM-DD)
func (s *ProcessingService) FetchAllTrainingPlanEntries(ctx context.Context, config *automation.ProcessingConfig, startDate, endDate time.Time) (TrainingPlanCache, error) {
	s.logger.Debug("Fetching all training plan entries",
		"user_id", config.UserID,
		"start_date", startDate.Format("2006-01-02"),
		"end_date", endDate.Format("2006-01-02"),
		"spreadsheet_id", config.SpreadsheetID)

	// Calculate the range more intelligently based on the year
	// Since this is a yearly plan, we can estimate the row range
	startDayOfYear := startDate.YearDay()
	endDayOfYear := endDate.YearDay()
	
	// Add some buffer for safety (in case plan doesn't start on Jan 1)
	startRow := max(2, startDayOfYear - 10) // Start from at least row 2
	endRow := min(endDayOfYear + 10, 367)   // Max 365 days + buffer
	
	rangeSpec := fmt.Sprintf("Тренировъчен План!A%d:J%d", startRow, endRow)
	
	s.logger.Debug("Calculated spreadsheet range",
		"user_id", config.UserID,
		"start_row", startRow,
		"end_row", endRow,
		"range", rangeSpec)
	
	rows, err := s.sheetsClient.ReadRange(ctx, config.SpreadsheetID, rangeSpec)
	if err != nil {
		s.logger.Error("Failed to read training plan from spreadsheet",
			"error", err,
			"user_id", config.UserID,
			"spreadsheet_id", config.SpreadsheetID,
			"range", rangeSpec)
		return nil, fmt.Errorf("failed to read training plan: %w", err)
	}

	// Build the cache
	cache := make(TrainingPlanCache)
	
	for rowIndex, row := range rows {
		if len(row) == 0 {
			continue
		}
		
		entry := s.parseTrainingPlanRow(row, startRow+rowIndex)
		if entry != nil && !entry.Date.IsZero() {
			// Store in cache with normalized date key (YYYY-MM-DD)
			dateKey := entry.Date.Format("2006-01-02")
			cache[dateKey] = entry
		}
	}

	s.logger.Info("Fetched training plan entries",
		"user_id", config.UserID,
		"entries_found", len(cache),
		"date_range", fmt.Sprintf("%s to %s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02")))
	
	return cache, nil
}

// fetchTrainingPlanEntry retrieves the training plan for a specific day from the cache
func (s *ProcessingService) fetchTrainingPlanEntry(ctx context.Context, config *automation.ProcessingConfig, date time.Time, cache TrainingPlanCache) (*TrainingPlanEntry, error) {
	dateKey := date.Format("2006-01-02")
	
	s.logger.Debug("Looking up training plan entry in cache",
		"user_id", config.UserID,
		"date", dateKey)
	
	if entry, ok := cache[dateKey]; ok {
		return entry, nil
	}
	
	s.logger.Debug("No training plan entry found in cache for date",
		"user_id", config.UserID,
		"date", dateKey)
	
	return nil, nil
}

// parseTrainingPlanRow parses a spreadsheet row into a TrainingPlanEntry
func (s *ProcessingService) parseTrainingPlanRow(row []interface{}, rowNumber int) *TrainingPlanEntry {
	entry := &TrainingPlanEntry{
		Row: rowNumber,
	}

	// Helper function to safely get string value from row
	getString := func(index int) string {
		if index < len(row) && row[index] != nil {
			if str, ok := row[index].(string); ok {
				return str
			}
		}
		return ""
	}

	// Helper function to safely get float value
	getFloat := func(index int) *float64 {
		if index < len(row) && row[index] != nil {
			switch v := row[index].(type) {
			case float64:
				return &v
			case string:
				if v != "" {
					// Try to parse string as float
					if f, err := strconv.ParseFloat(strings.Replace(v, ",", ".", 1), 64); err == nil {
						return &f
					}
				}
			}
		}
		return nil
	}

	// Parse date (Column A - index 0)
	// Date format is always non-zero-padded: "1.5.2025", "22.6.2025", etc.
	if dateStr := getString(0); dateStr != "" {
		parsedDate, err := time.Parse("2.1.2006", dateStr)
		if err != nil {
			s.logger.Warn("Failed to parse date from training plan",
				"date_string", dateStr,
				"row", rowNumber,
				"error", err)
		} else {
			entry.Date = parsedDate
		}
	}

	// Activity Type (Column D - index 3)
	entry.ActivityType = getString(3)

	// Distance (Column E - index 4)
	entry.Distance = getFloat(4)

	// Time (Column F - index 5)
	if timeStr := getString(5); timeStr != "" {
		entry.Time = &timeStr
	}

	// RPE (Column I - index 8)
	if index := 8; index < len(row) && row[index] != nil {
		switch v := row[index].(type) {
		case float64:
			entry.RPE = int(v)
		case string:
			if rpe, err := strconv.Atoi(v); err == nil {
				entry.RPE = rpe
			}
		}
	}

	// Description (Column J - index 9)
	entry.Description = getString(9)

	// TODO: Check if description is bold to determine IsProcessed
	// This requires an additional API call to get cell formatting
	// For now, we'll assume not processed to allow testing
	entry.IsProcessed = false

	return entry
}

// processActivities processes raw Strava activities and returns processed data
// Note: Activities are already filtered to only include runs at the cache level
func (s *ProcessingService) processActivities(activities []strava.Activity) ([]ProcessedActivity, float64, int) {
	var processedActivities []ProcessedActivity
	var totalDistance float64
	var totalTime int

	for _, activity := range activities {
		// Activities should already be filtered to runs only, but double-check
		if activity.Type != "Run" {
			s.logger.Warn("Non-run activity found in processing",
				"activity_id", activity.ID,
				"activity_type", activity.Type,
				"activity_name", activity.Name)
			continue
		}
		
		processed := ProcessedActivity{
			StravaActivity: activity,
		}
		
		// Round distance to nearest 0.05km
		distanceKm := activity.Distance / 1000
		processed.ProcessedData.Distance = math.Round(distanceKm*20) / 20 // Round to nearest 0.05
		
		// Format duration as HH:MM:SS with rounding to nearest 5 seconds
		processed.ProcessedData.Duration = s.formatDuration(activity.MovingTime)
		
		processedActivities = append(processedActivities, processed)
		
		// Add to totals
		totalDistance += activity.Distance
		totalTime += activity.MovingTime
	}

	return processedActivities, totalDistance, totalTime
}

// prepareSpreadsheetUpdate prepares the update data for the spreadsheet
func (s *ProcessingService) prepareSpreadsheetUpdate(planEntry *TrainingPlanEntry, activities []ProcessedActivity, totalDistance float64, totalTime int) *SpreadsheetUpdate {
	update := &SpreadsheetUpdate{
		Row:             planEntry.Row,
		DescriptionBold: true, // Always mark as processed
	}

	// Handle special case: Rest day with activities -> RPE = 2
	if planEntry.ActivityType == "Почивка" && len(activities) > 0 {
		update.RPEValue = 2
	} else {
		update.RPEValue = planEntry.RPE
	}

	// Format distance and time
	if len(activities) > 0 {
		update.DistanceValue = s.formatDistance(totalDistance)
		update.TimeValue = s.formatDuration(totalTime)
	} else {
		// No activities - use zero values
		update.DistanceValue = "0"
		update.TimeValue = "00:00:00"
	}

	// Prepare description update
	update.DescriptionValue = s.prepareDescriptionUpdate(planEntry.Description, activities, update.RPEValue)

	return update
}

// formatDistance formats distance in kilometers with comma decimal separator
func (s *ProcessingService) formatDistance(distanceMeters float64) string {
	distanceKm := distanceMeters / 1000
	// Round to nearest 0.05km
	rounded := math.Round(distanceKm*20) / 20
	// Format with 2 decimal places and replace . with ,
	formatted := fmt.Sprintf("%.2f", rounded)
	return strings.Replace(formatted, ".", ",", 1)
}

// formatDuration formats seconds into HH:MM:SS format with rounding to nearest 5 seconds
func (s *ProcessingService) formatDuration(seconds int) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60
	
	// Round to nearest 5 seconds
	roundedSecs := int(math.Round(float64(secs)/5.0) * 5)
	
	// Handle carry-over if rounding caused 60 seconds
	if roundedSecs == 60 {
		minutes++
		roundedSecs = 0
		
		// Handle carry-over to hours if needed
		if minutes == 60 {
			hours++
			minutes = 0
		}
	}
	
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, roundedSecs)
}

// prepareDescriptionUpdate prepares the description text based on RPE and activities
func (s *ProcessingService) prepareDescriptionUpdate(originalDescription string, activities []ProcessedActivity, rpe int) string {
	// TODO: Implement RPE-based description updates
	// - RPE 4-5: Progressive run logic
	// - RPE 6: Steady state logic
	// - RPE 7-9: Tempo/interval logic with manual splits
	// For now, return original description
	return originalDescription
}
