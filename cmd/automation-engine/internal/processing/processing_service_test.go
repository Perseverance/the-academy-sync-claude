package processing

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/automation"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/google"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/strava"
)

// MockStravaClient implements a mock Strava client for testing
type MockStravaClient struct {
	activities []strava.Activity
	err        error
}

func (m *MockStravaClient) GetActivities(ctx context.Context, after time.Time) ([]strava.Activity, error) {
	if m.err != nil {
		return nil, m.err
	}

	// Filter activities based on after parameter
	var filtered []strava.Activity
	for _, activity := range m.activities {
		if activity.StartDate.After(after) || activity.StartDate.Equal(after) {
			filtered = append(filtered, activity)
		}
	}

	return filtered, nil
}

func (m *MockStravaClient) GetActivityLaps(ctx context.Context, activityID int64) ([]strava.Lap, error) {
	// Return empty laps for testing - in real tests, this could be mocked with actual data
	return []strava.Lap{}, nil
}

// MockSheetsClient implements a mock Google Sheets client for testing
type MockSheetsClient struct {
	readRangeData     [][]interface{}
	readRangeErr      error
	readRangeCalls    int
	lastReadRange     string
	validateAccessErr error
	batchUpdateCalls  int
	lastBatchUpdates  []*google.SpreadsheetUpdate
}

func (m *MockSheetsClient) ReadRange(ctx context.Context, spreadsheetID, rangeSpec string) ([][]interface{}, error) {
	m.readRangeCalls++
	m.lastReadRange = rangeSpec

	if m.readRangeErr != nil {
		return nil, m.readRangeErr
	}

	return m.readRangeData, nil
}

func (m *MockSheetsClient) ValidateAccess(ctx context.Context, spreadsheetID string) error {
	return m.validateAccessErr
}

func (m *MockSheetsClient) GetSpreadsheetInfo(ctx context.Context, spreadsheetID string) (*google.SpreadsheetInfo, error) {
	return &google.SpreadsheetInfo{
		ID:    spreadsheetID,
		Title: "Test Spreadsheet",
	}, nil
}

func (m *MockSheetsClient) WriteActivities(ctx context.Context, spreadsheetID string, activities []strava.Activity) error {
	return nil
}

func (m *MockSheetsClient) BatchUpdateTrainingPlan(ctx context.Context, spreadsheetID string, updates []*google.SpreadsheetUpdate) error {
	m.batchUpdateCalls++
	// Convert internal updates to google.SpreadsheetUpdate format for tracking
	var convertedUpdates []*google.SpreadsheetUpdate
	for _, u := range updates {
		convertedUpdates = append(convertedUpdates, &google.SpreadsheetUpdate{
			Row:              u.Row,
			DistanceValue:    u.DistanceValue,
			TimeValue:        u.TimeValue,
			RPEValue:         u.RPEValue,
			DescriptionValue: u.DescriptionValue,
			DescriptionBold:  u.DescriptionBold,
		})
	}
	m.lastBatchUpdates = convertedUpdates
	return nil
}

// Helper function to create test logger
func createTestLogger() *logger.Logger {
	return logger.New("test")
}

// Helper function to create test config
func createTestConfig(userID int, timezone string) *automation.ProcessingConfig {
	return &automation.ProcessingConfig{
		UserID:        userID,
		Timezone:      timezone,
		SpreadsheetID: "test-spreadsheet-id",
	}
}

// Test fetchAllTrainingPlanEntries with successful data
func TestFetchAllTrainingPlanEntries_Success(t *testing.T) {
	mockStrava := &MockStravaClient{}
	mockSheets := &MockSheetsClient{
		readRangeData: [][]interface{}{
			{"1.5.2025", "", "", "Бягане", "10", "01:00:00", "", "", "5", "Easy run"},
			{"2.5.2025", "", "", "Почивка", "", "", "", "", "1", "Rest day"},
			{"3.5.2025", "", "", "Бягане", "15", "01:30:00", "", "", "7", "Tempo run"},
		},
	}

	service := NewProcessingService(mockStrava, mockSheets, nil, createTestLogger())
	config := createTestConfig(1, "Europe/Sofia")

	startDate, _ := time.Parse("2006-01-02", "2025-05-01")
	endDate, _ := time.Parse("2006-01-02", "2025-05-03")

	cache, err := service.FetchAllTrainingPlanEntries(context.Background(), config, startDate, endDate)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(cache) != 3 {
		t.Errorf("Expected 3 entries in cache, got %d", len(cache))
	}

	// Verify cache entries
	if entry, ok := cache["2025-05-01"]; !ok {
		t.Error("Expected entry for 2025-05-01")
	} else {
		if entry.ActivityType != "Бягане" {
			t.Errorf("Expected activity type 'Бягане', got '%s'", entry.ActivityType)
		}
		if entry.RPE != 5 {
			t.Errorf("Expected RPE 5, got %.1f", entry.RPE)
		}
	}

	// Verify only one API call was made
	if mockSheets.readRangeCalls != 1 {
		t.Errorf("Expected 1 API call, got %d", mockSheets.readRangeCalls)
	}
}

// Test smart range calculation
func TestFetchAllTrainingPlanEntries_SmartRangeCalculation(t *testing.T) {
	mockStrava := &MockStravaClient{}
	mockSheets := &MockSheetsClient{
		readRangeData: [][]interface{}{},
	}

	service := NewProcessingService(mockStrava, mockSheets, nil, createTestLogger())
	config := createTestConfig(1, "Europe/Sofia")

	// Test mid-year dates (June 22-25)
	startDate, _ := time.Parse("2006-01-02", "2025-06-22")
	endDate, _ := time.Parse("2006-01-02", "2025-06-25")

	_, _ = service.FetchAllTrainingPlanEntries(context.Background(), config, startDate, endDate)

	// Expected range should be around rows 163-186 (day 173 ± 10)
	expectedRange := "Тренировъчен План!A163:J186"
	if mockSheets.lastReadRange != expectedRange {
		t.Errorf("Expected range %s, got %s", expectedRange, mockSheets.lastReadRange)
	}
}

// Test date parsing with standard format
func TestParseTrainingPlanRow_StandardDateFormat(t *testing.T) {
	service := NewProcessingService(nil, nil, nil, createTestLogger())

	testCases := []struct {
		dateStr      string
		expectedDate string
	}{
		{"1.5.2025", "2025-05-01"},
		{"22.6.2025", "2025-06-22"},
		{"31.12.2025", "2025-12-31"},
		{"1.1.2025", "2025-01-01"},
	}

	for _, tc := range testCases {
		row := []interface{}{tc.dateStr, "", "", "Бягане", "10", "01:00:00", "", "", "5", "Test"}
		entry := service.parseTrainingPlanRow(row, 2)

		if entry.Date.Format("2006-01-02") != tc.expectedDate {
			t.Errorf("For date string '%s', expected %s, got %s",
				tc.dateStr, tc.expectedDate, entry.Date.Format("2006-01-02"))
		}
	}
}

// Test invalid date handling
func TestParseTrainingPlanRow_InvalidDate(t *testing.T) {
	service := NewProcessingService(nil, nil, nil, createTestLogger())

	// Invalid date format
	row := []interface{}{"32.13.2025", "", "", "Бягане", "10", "01:00:00", "", "", "5", "Test"}
	entry := service.parseTrainingPlanRow(row, 2)

	if !entry.Date.IsZero() {
		t.Error("Expected zero date for invalid date string")
	}
}

// Test processSingleDay with no training plan
func TestProcessSingleDay_NoTrainingPlan(t *testing.T) {
	mockStrava := &MockStravaClient{
		activities: []strava.Activity{},
	}
	mockSheets := &MockSheetsClient{}

	service := NewProcessingService(mockStrava, mockSheets, nil, createTestLogger())
	config := createTestConfig(1, "Europe/Sofia")

	date, _ := time.Parse("2006-01-02", "2025-05-01")
	cache := make(TrainingPlanCache) // Empty cache

	activitiesCache := make(StravaActivitiesCache)
	result, err := service.processSingleDay(context.Background(), config, date, cache, activitiesCache)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.Processed {
		t.Error("Expected not processed")
	}

	if result.SkippedReason != "No training plan entry for this day" {
		t.Errorf("Expected skip reason for no plan, got: %s", result.SkippedReason)
	}
}

// Test processSingleDay with already processed entry
func TestProcessSingleDay_AlreadyProcessed(t *testing.T) {
	mockStrava := &MockStravaClient{}
	mockSheets := &MockSheetsClient{}

	service := NewProcessingService(mockStrava, mockSheets, nil, createTestLogger())
	config := createTestConfig(1, "Europe/Sofia")

	date, _ := time.Parse("2006-01-02", "2025-05-01")
	cache := TrainingPlanCache{
		"2025-05-01": &TrainingPlanEntry{
			Date:         date,
			ActivityType: "Бягане",
			IsProcessed:  true,
			Row:          2,
		},
	}

	activitiesCache := make(StravaActivitiesCache)
	result, err := service.processSingleDay(context.Background(), config, date, cache, activitiesCache)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.Processed {
		t.Error("Expected not processed")
	}

	if result.SkippedReason != "Day already processed (bold text found)" {
		t.Errorf("Expected skip reason for processed, got: %s", result.SkippedReason)
	}
}

// Test rest day with activity (RPE = 2 special case)
func TestProcessSingleDay_RestDayWithActivity(t *testing.T) {
	date, _ := time.Parse("2006-01-02", "2025-05-01")
	dateKey := date.Format("2006-01-02")

	mockStrava := &MockStravaClient{
		activities: []strava.Activity{
			{
				ID:         123,
				Name:       "Morning Run",
				Type:       "Run",
				Distance:   5000,
				MovingTime: 1800,
				StartDate:  date.Add(8 * time.Hour),
			},
		},
	}
	mockSheets := &MockSheetsClient{}

	service := NewProcessingService(mockStrava, mockSheets, nil, createTestLogger())
	config := createTestConfig(1, "Europe/Sofia")

	cache := TrainingPlanCache{
		dateKey: &TrainingPlanEntry{
			Date:         date,
			ActivityType: "Почивка", // Rest day
			RPE:          1,
			Row:          2,
		},
	}

	// Create activities cache with the activity
	activitiesCache := StravaActivitiesCache{
		dateKey: mockStrava.activities,
	}

	result, err := service.processSingleDay(context.Background(), config, date, cache, activitiesCache)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !result.Processed {
		t.Error("Expected processed")
	}

	if result.SpreadsheetUpdate.RPEValue != 2 {
		t.Errorf("Expected RPE 2 for rest day with activity, got %.1f", result.SpreadsheetUpdate.RPEValue)
	}
}

// Test scheduled run with no activity
func TestProcessSingleDay_ScheduledRunNoActivity(t *testing.T) {
	date, _ := time.Parse("2006-01-02", "2025-05-01")
	dateKey := date.Format("2006-01-02")

	mockStrava := &MockStravaClient{
		activities: []strava.Activity{}, // No activities
	}
	mockSheets := &MockSheetsClient{}

	service := NewProcessingService(mockStrava, mockSheets, nil, createTestLogger())
	config := createTestConfig(1, "Europe/Sofia")

	cache := TrainingPlanCache{
		dateKey: &TrainingPlanEntry{
			Date:         date,
			ActivityType: "Бягане", // Scheduled run
			RPE:          5,
			Row:          2,
		},
	}

	// Create activities cache with empty activities for this date
	activitiesCache := StravaActivitiesCache{
		dateKey: []strava.Activity{},
	}

	result, err := service.processSingleDay(context.Background(), config, date, cache, activitiesCache)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !result.Processed {
		t.Error("Expected processed")
	}

	// Should have zero values
	if result.SpreadsheetUpdate.DistanceValue != "0" {
		t.Errorf("Expected distance '0', got %s", result.SpreadsheetUpdate.DistanceValue)
	}

	if result.SpreadsheetUpdate.TimeValue != "00:00:00" {
		t.Errorf("Expected time '00:00:00', got %s", result.SpreadsheetUpdate.TimeValue)
	}
}

// Test that processing uses cache and makes only one API call
func TestProcessing_SingleAPICall(t *testing.T) {
	mockStrava := &MockStravaClient{
		activities: []strava.Activity{},
	}
	mockSheets := &MockSheetsClient{
		readRangeData: [][]interface{}{
			{"1.5.2025", "", "", "Бягане", "10", "01:00:00", "", "", "5", "Easy run"},
			{"2.5.2025", "", "", "Бягане", "12", "01:15:00", "", "", "6", "Steady run"},
			{"3.5.2025", "", "", "Почивка", "", "", "", "", "1", "Rest day"},
		},
	}

	service := NewProcessingService(mockStrava, mockSheets, nil, createTestLogger())
	config := createTestConfig(1, "Europe/Sofia")

	// Test processing multiple days with cache
	startDate, _ := time.Parse("2006-01-02", "2025-05-01")
	endDate, _ := time.Parse("2006-01-02", "2025-05-03")

	// Fetch cache once
	cache, err := service.FetchAllTrainingPlanEntries(context.Background(), config, startDate, endDate)
	if err != nil {
		t.Fatalf("Failed to fetch cache: %v", err)
	}

	// Reset call counter
	initialCalls := mockSheets.readRangeCalls

	// Create empty activities cache for testing
	activitiesCache := make(StravaActivitiesCache)

	// Process multiple days using the cache
	for i := 0; i < 3; i++ {
		date := startDate.AddDate(0, 0, i)
		_, _ = service.processSingleDay(context.Background(), config, date, cache, activitiesCache)
	}

	// Verify no additional API calls were made
	if mockSheets.readRangeCalls != initialCalls {
		t.Errorf("Expected no additional API calls, but %d calls were made",
			mockSheets.readRangeCalls-initialCalls)
	}
}

// Test empty spreadsheet handling
func TestFetchAllTrainingPlanEntries_EmptySpreadsheet(t *testing.T) {
	mockStrava := &MockStravaClient{}
	mockSheets := &MockSheetsClient{
		readRangeData: [][]interface{}{}, // Empty data
	}

	service := NewProcessingService(mockStrava, mockSheets, nil, createTestLogger())
	config := createTestConfig(1, "Europe/Sofia")

	startDate, _ := time.Parse("2006-01-02", "2025-05-01")
	endDate, _ := time.Parse("2006-01-02", "2025-05-03")

	cache, err := service.FetchAllTrainingPlanEntries(context.Background(), config, startDate, endDate)

	if err != nil {
		t.Fatalf("Expected no error for empty data, got %v", err)
	}

	if len(cache) != 0 {
		t.Errorf("Expected empty cache, got %d entries", len(cache))
	}
}

// Test distance and time formatting
func TestFormatting(t *testing.T) {
	service := NewProcessingService(nil, nil, nil, createTestLogger())

	// Test distance formatting
	testDistances := []struct {
		meters   float64
		expected string
	}{
		{5000, "5,00"},
		{10250, "10,25"},
		{10260, "10,25"}, // Should round to nearest 0.05
		{10270, "10,25"}, // Should round to nearest 0.05
		{10280, "10,30"}, // Should round to nearest 0.05
	}

	for _, td := range testDistances {
		result := service.formatDistance(td.meters)
		if result != td.expected {
			t.Errorf("For %f meters, expected %s, got %s", td.meters, td.expected, result)
		}
	}

	// Test duration formatting with rounding to nearest 5 seconds
	testDurations := []struct {
		seconds  int
		expected string
	}{
		{3600, "01:00:00"}, // Exactly on 00
		{3601, "01:00:00"}, // 01 rounds down to 00
		{3602, "01:00:00"}, // 02 rounds down to 00
		{3603, "01:00:05"}, // 03 rounds up to 05
		{3604, "01:00:05"}, // 04 rounds up to 05
		{3605, "01:00:05"}, // 05 stays at 05
		{3606, "01:00:05"}, // 06 rounds down to 05
		{3607, "01:00:05"}, // 07 rounds down to 05
		{3608, "01:00:10"}, // 08 rounds up to 10
		{3658, "01:01:00"}, // 58 rounds up to 60, carries over
		{3659, "01:01:00"}, // 59 rounds up to 60, carries over
		{7199, "02:00:00"}, // 01:59:59 rounds to 02:00:00
		{0, "00:00:00"},    // Zero stays zero
	}

	for _, td := range testDurations {
		result := service.formatDuration(td.seconds)
		if result != td.expected {
			t.Errorf("For %d seconds (%02d:%02d:%02d), expected %s, got %s",
				td.seconds,
				td.seconds/3600,
				(td.seconds%3600)/60,
				td.seconds%60,
				td.expected,
				result)
		}
	}
}

// Test processActivities filtering
func TestProcessActivities(t *testing.T) {
	service := NewProcessingService(nil, nil, nil, createTestLogger())

	activities := []strava.Activity{
		{
			ID:         1,
			Type:       "Run",
			Distance:   5000,
			MovingTime: 1802, // 30:02 should round to 30:00
		},
		{
			ID:         2,
			Type:       "Ride", // Not a run
			Distance:   20000,
			MovingTime: 3600,
		},
		{
			ID:         3,
			Type:       "Run",
			Distance:   10000,
			MovingTime: 3724, // 01:02:04 should round to 01:02:05
		},
	}

	processed, totalDistance, totalTime := service.processActivities(activities)

	// Should only process runs
	if len(processed) != 2 {
		t.Errorf("Expected 2 processed activities, got %d", len(processed))
	}

	if totalDistance != 15000 {
		t.Errorf("Expected total distance 15000, got %f", totalDistance)
	}

	if totalTime != 5526 { // 1802 + 3724
		t.Errorf("Expected total time 5526, got %d", totalTime)
	}

	// Check that durations are formatted with proper rounding
	if processed[0].ProcessedData.Duration != "00:30:00" {
		t.Errorf("Expected first activity duration '00:30:00', got %s", processed[0].ProcessedData.Duration)
	}

	if processed[1].ProcessedData.Duration != "01:02:05" {
		t.Errorf("Expected second activity duration '01:02:05', got %s", processed[1].ProcessedData.Duration)
	}
}

// Test year boundary handling
func TestFetchAllTrainingPlanEntries_YearBoundary(t *testing.T) {
	mockStrava := &MockStravaClient{}
	mockSheets := &MockSheetsClient{
		readRangeData: [][]interface{}{
			{"30.12.2024", "", "", "Бягане", "10", "01:00:00", "", "", "5", "End of year"},
			{"31.12.2024", "", "", "Бягане", "12", "01:15:00", "", "", "6", "Last day"},
			{"1.1.2025", "", "", "Бягане", "8", "00:45:00", "", "", "4", "New year"},
		},
	}

	service := NewProcessingService(mockStrava, mockSheets, nil, createTestLogger())
	config := createTestConfig(1, "Europe/Sofia")

	// Test crossing year boundary
	startDate, _ := time.Parse("2006-01-02", "2024-12-30")
	endDate, _ := time.Parse("2006-01-02", "2025-01-01")

	cache, err := service.FetchAllTrainingPlanEntries(context.Background(), config, startDate, endDate)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should handle year boundary correctly
	if len(cache) != 3 {
		t.Errorf("Expected 3 entries across year boundary, got %d", len(cache))
	}

	// Verify entries exist for both years
	if _, ok := cache["2024-12-31"]; !ok {
		t.Error("Missing entry for 2024-12-31")
	}
	if _, ok := cache["2025-01-01"]; !ok {
		t.Error("Missing entry for 2025-01-01")
	}
}

// Integration test for ProcessPreviousDay with cache
func TestProcessPreviousDay_WithCache(t *testing.T) {
	// Use yesterday based on current time
	location, _ := time.LoadLocation("Europe/Sofia")
	now := time.Now().In(location)
	yesterday := now.AddDate(0, 0, -1)
	yesterdayStart := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, location)
	yesterdayKey := yesterdayStart.Format("2006-01-02")

	mockStrava := &MockStravaClient{
		activities: []strava.Activity{
			{
				ID:         123,
				Name:       "Morning Run",
				Type:       "Run",
				Distance:   10000,
				MovingTime: 3600,
				StartDate:  yesterdayStart.Add(8 * time.Hour),
			},
		},
	}
	mockSheets := &MockSheetsClient{}

	service := NewProcessingService(mockStrava, mockSheets, nil, createTestLogger())
	config := createTestConfig(1, "Europe/Sofia")

	// Create cache with yesterday's plan
	cache := TrainingPlanCache{
		yesterdayKey: &TrainingPlanEntry{
			Date:         yesterdayStart,
			ActivityType: "Бягане",
			RPE:          5,
			Row:          2,
		},
	}

	// Create activities cache with yesterday's activities
	activitiesCache := StravaActivitiesCache{
		yesterdayKey: mockStrava.activities,
	}

	result, err := service.ProcessPreviousDay(context.Background(), config, cache, activitiesCache)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !result.Processed {
		t.Error("Expected day to be processed")
	}

	if result.ActivitiesFound != 1 {
		t.Errorf("Expected 1 activity found, got %d", result.ActivitiesFound)
	}
}

// Test API error handling
func TestFetchAllTrainingPlanEntries_APIError(t *testing.T) {
	mockStrava := &MockStravaClient{}
	mockSheets := &MockSheetsClient{
		readRangeErr: fmt.Errorf("API quota exceeded"),
	}

	service := NewProcessingService(mockStrava, mockSheets, nil, createTestLogger())
	config := createTestConfig(1, "Europe/Sofia")

	startDate, _ := time.Parse("2006-01-02", "2025-05-01")
	endDate, _ := time.Parse("2006-01-02", "2025-05-03")

	_, err := service.FetchAllTrainingPlanEntries(context.Background(), config, startDate, endDate)

	if err == nil {
		t.Error("Expected error from API failure")
	}

	if err.Error() != "failed to read training plan: API quota exceeded" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

// Test that spreadsheet updates are properly prepared
func TestSpreadsheetUpdatePreparation(t *testing.T) {
	date, _ := time.Parse("2006-01-02", "2025-05-01")
	dateKey := date.Format("2006-01-02")

	mockStrava := &MockStravaClient{
		activities: []strava.Activity{
			{
				ID:         123,
				Name:       "Morning Run",
				Type:       "Run",
				Distance:   10274, // Should round to 10.25km
				MovingTime: 3723,  // 01:02:03 should round to 01:02:05
				StartDate:  date.Add(8 * time.Hour),
			},
		},
	}
	mockSheets := &MockSheetsClient{}

	service := NewProcessingService(mockStrava, mockSheets, nil, createTestLogger())
	config := createTestConfig(1, "Europe/Sofia")

	cache := TrainingPlanCache{
		dateKey: &TrainingPlanEntry{
			Date:         date,
			ActivityType: "Бягане",
			RPE:          4.5,
			Row:          123,
			Description:  "Прогресивно бягане",
		},
	}

	// Create activities cache with the activity
	activitiesCache := StravaActivitiesCache{
		dateKey: mockStrava.activities,
	}

	result, err := service.processSingleDay(context.Background(), config, date, cache, activitiesCache)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !result.Processed {
		t.Error("Expected processed")
	}

	if result.SpreadsheetUpdate == nil {
		t.Fatal("Expected spreadsheet update to be prepared")
	}

	// Verify the prepared update
	update := result.SpreadsheetUpdate
	if update.Row != 123 {
		t.Errorf("Expected row 123, got %d", update.Row)
	}

	if update.DistanceValue != "10,25" {
		t.Errorf("Expected distance '10,25', got %s", update.DistanceValue)
	}

	if update.TimeValue != "01:02:05" {
		t.Errorf("Expected time '01:02:05', got %s", update.TimeValue)
	}

	if update.RPEValue != 4.5 {
		t.Errorf("Expected RPE 4.5, got %.1f", update.RPEValue)
	}

	// Description should be generated based on RPE and activities
	if !strings.Contains(update.DescriptionValue, "Прогресивно бягане") {
		t.Errorf("Expected progressive run description, got %s", update.DescriptionValue)
	}

	if !update.DescriptionBold {
		t.Error("Expected description to be marked for bold formatting")
	}
}

// Test RunScheduledCycle - processes previous day and lookback
func TestRunScheduledCycle_Success(t *testing.T) {
	// Setup time references
	location, _ := time.LoadLocation("Europe/Sofia")
	now := time.Now().In(location)
	yesterday := now.AddDate(0, 0, -1)
	yesterdayStart := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, location)
	
	// Create activities for testing
	mockStrava := &MockStravaClient{
		activities: []strava.Activity{
			{
				ID:         123,
				Name:       "Yesterday Run",
				Type:       "Run",
				Distance:   10000,
				MovingTime: 3600,
				StartDate:  yesterdayStart.Add(8 * time.Hour),
			},
			{
				ID:         124,
				Name:       "5 days ago Run",
				Type:       "Run",
				Distance:   8000,
				MovingTime: 3000,
				StartDate:  yesterdayStart.AddDate(0, 0, -4).Add(7 * time.Hour),
			},
		},
	}
	
	// Create mock sheets with training plan data
	mockSheets := &MockSheetsClient{
		readRangeData: [][]interface{}{
			// Include multiple days of training plan
			{yesterday.Format("2.1.2006"), "", "", "Бягане", "10", "01:00:00", "", "", "5", "Easy run"},
			{yesterdayStart.AddDate(0, 0, -1).Format("2.1.2006"), "", "", "Почивка", "", "", "", "", "1", "Rest day"},
			{yesterdayStart.AddDate(0, 0, -2).Format("2.1.2006"), "", "", "Бягане", "12", "01:15:00", "", "", "6", "Tempo run"},
			{yesterdayStart.AddDate(0, 0, -3).Format("2.1.2006"), "", "", "Бягане", "8", "00:45:00", "", "", "4", "Recovery run"},
			{yesterdayStart.AddDate(0, 0, -4).Format("2.1.2006"), "", "", "Бягане", "8", "00:50:00", "", "", "5", "Easy run"},
			{yesterdayStart.AddDate(0, 0, -5).Format("2.1.2006"), "", "", "Почивка", "", "", "", "", "1", "Rest day"},
			{yesterdayStart.AddDate(0, 0, -6).Format("2.1.2006"), "", "", "Бягане", "15", "01:30:00", "", "", "7", "Long run"},
		},
	}
	
	service := NewProcessingService(mockStrava, mockSheets, nil, createTestLogger())
	config := createTestConfig(1, "Europe/Sofia")
	
	// Pre-populate training cache with entries
	trainingCache := make(TrainingPlanCache)
	// Add yesterday's entry
	trainingCache[yesterday.Format("2006-01-02")] = &TrainingPlanEntry{
		Date:         yesterdayStart,
		ActivityType: "Бягане",
		Description:  "Easy run",
		RPE:          5,
		IsProcessed:  false,
		Row:          2,
		Distance:     nil,
		Time:         nil,
	}
	// Add lookback entries (days 2-8 in the past)
	for i := 2; i <= 8; i++ {
		date := yesterdayStart.AddDate(0, 0, -i+1)
		dateKey := date.Format("2006-01-02")
		
		// Map the test data to cache entries
		if i == 2 { // Rest day
			trainingCache[dateKey] = &TrainingPlanEntry{
				Date:         date,
				ActivityType: "Почивка",
				Description:  "Rest day",
				RPE:          1,
				IsProcessed:  false,
				Row:          2 + i - 1,
			}
		} else if i == 5 { // Day with activity
			trainingCache[dateKey] = &TrainingPlanEntry{
				Date:         date,
				ActivityType: "Бягане",
				Description:  "Easy run",
				RPE:          5,
				IsProcessed:  false,
				Row:          2 + i - 1,
			}
		} else if i <= 7 { // Other run days
			trainingCache[dateKey] = &TrainingPlanEntry{
				Date:         date,
				ActivityType: "Бягане",
				Description:  "Run",
				RPE:          float64(4 + (i % 3)),
				IsProcessed:  false,
				Row:          2 + i - 1,
			}
		}
	}
	
	// Pre-populate activities cache
	activitiesCache := make(StravaActivitiesCache)
	// Add yesterday's activity
	activitiesCache[yesterday.Format("2006-01-02")] = []strava.Activity{
		{
			ID:         123,
			Name:       "Yesterday Run",
			Type:       "Run",
			Distance:   10000,
			MovingTime: 3600,
			StartDate:  yesterdayStart.Add(8 * time.Hour),
		},
	}
	// Add activity for 5 days ago
	fiveDaysAgo := yesterdayStart.AddDate(0, 0, -4)
	activitiesCache[fiveDaysAgo.Format("2006-01-02")] = []strava.Activity{
		{
			ID:         124,
			Name:       "5 days ago Run",
			Type:       "Run",
			Distance:   8000,
			MovingTime: 3000,
			StartDate:  fiveDaysAgo.Add(7 * time.Hour),
		},
	}
	// Initialize empty arrays for other days
	for i := 2; i <= 8; i++ {
		date := yesterdayStart.AddDate(0, 0, -i+1)
		dateKey := date.Format("2006-01-02")
		if _, exists := activitiesCache[dateKey]; !exists {
			activitiesCache[dateKey] = []strava.Activity{}
		}
	}
	
	// Run the scheduled cycle
	result, err := service.RunScheduledCycle(context.Background(), config, trainingCache, activitiesCache)
	
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	
	if result == nil {
		t.Fatal("Expected result, got nil")
	}
	
	// Verify that both previous day and lookback were processed
	if result.ProcessedDays == 0 {
		t.Error("Expected some days to be processed")
	}
	
	// Verify that detailed results contain updates
	if len(result.DetailedResults) == 0 {
		t.Error("Expected detailed results with updates")
	}
	
	// Count updates that were prepared (not actual batch update calls)
	updateCount := 0
	for _, dr := range result.DetailedResults {
		if dr.SpreadsheetUpdate != nil {
			updateCount++
		}
	}
	
	if updateCount == 0 {
		t.Error("Expected some spreadsheet updates to be prepared")
	}
}

// Test RunScheduledCycle with empty training plan
func TestRunScheduledCycle_EmptyTrainingPlan(t *testing.T) {
	mockStrava := &MockStravaClient{
		activities: []strava.Activity{},
	}
	mockSheets := &MockSheetsClient{
		readRangeData: [][]interface{}{}, // Empty training plan
	}
	
	service := NewProcessingService(mockStrava, mockSheets, nil, createTestLogger())
	config := createTestConfig(1, "Europe/Sofia")
	
	trainingCache := make(TrainingPlanCache)
	activitiesCache := make(StravaActivitiesCache)
	
	result, err := service.RunScheduledCycle(context.Background(), config, trainingCache, activitiesCache)
	
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	
	if result.ProcessedDays != 0 {
		t.Errorf("Expected 0 processed days for empty plan, got %d", result.ProcessedDays)
	}
	
	// Should not have any detailed results with updates
	updateCount := 0
	for _, dr := range result.DetailedResults {
		if dr.SpreadsheetUpdate != nil {
			updateCount++
		}
	}
	
	if updateCount != 0 {
		t.Errorf("Expected no spreadsheet updates for empty plan, got %d", updateCount)
	}
}

// Test RunScheduledCycle with empty caches (no training plan entries)
func TestRunScheduledCycle_EmptyCaches(t *testing.T) {
	mockStrava := &MockStravaClient{}
	mockSheets := &MockSheetsClient{}
	
	service := NewProcessingService(mockStrava, mockSheets, nil, createTestLogger())
	config := createTestConfig(1, "Europe/Sofia")
	
	// Empty caches simulate no training plan entries or activities
	trainingCache := make(TrainingPlanCache)
	activitiesCache := make(StravaActivitiesCache)
	
	// RunScheduledCycle should complete successfully but process nothing
	result, err := service.RunScheduledCycle(context.Background(), config, trainingCache, activitiesCache)
	
	// Should not error with empty caches
	if err != nil {
		t.Errorf("Expected no error with empty caches, got: %v", err)
	}
	
	if result == nil {
		t.Fatal("Expected result, got nil")
	}
	
	// Verify nothing was processed
	if result.ProcessedDays != 0 {
		t.Errorf("Expected 0 processed days with empty caches, got %d", result.ProcessedDays)
	}
	
	if result.ActivitiesCount != 0 {
		t.Errorf("Expected 0 activities with empty caches, got %d", result.ActivitiesCount)
	}
	
	if result.RowsUpdated != 0 {
		t.Errorf("Expected 0 rows updated with empty caches, got %d", result.RowsUpdated)
	}
	
	if result.Error != "" {
		t.Errorf("Expected no error message with empty caches, got: %s", result.Error)
	}
}

// Test RunScheduledCycle with lookback period
func TestRunScheduledCycle_LookbackPeriod(t *testing.T) {
	location, _ := time.LoadLocation("Europe/Sofia")
	now := time.Now().In(location)
	
	// Create activities spread across the lookback period
	var activities []strava.Activity
	for i := 0; i < 7; i++ {
		dayOffset := -i - 1 // -1 to -7 days
		activityDate := now.AddDate(0, 0, dayOffset)
		activityStart := time.Date(activityDate.Year(), activityDate.Month(), activityDate.Day(), 8, 0, 0, 0, location)
		
		activities = append(activities, strava.Activity{
			ID:         int64(100 + i),
			Name:       fmt.Sprintf("Day %d Run", dayOffset),
			Type:       "Run",
			Distance:   float64(8000 + i*1000),
			MovingTime: 3000 + i*300,
			StartDate:  activityStart,
		})
	}
	
	mockStrava := &MockStravaClient{
		activities: activities,
	}
	
	// Create training plan entries for the lookback period
	var trainingPlanRows [][]interface{}
	for i := 0; i < 8; i++ { // Include one extra day
		dayOffset := -i
		date := now.AddDate(0, 0, dayOffset)
		
		if i%3 == 2 { // Every third day is rest
			trainingPlanRows = append(trainingPlanRows, []interface{}{
				date.Format("2.1.2006"), "", "", "Почивка", "", "", "", "", "1", "Rest day",
			})
		} else {
			trainingPlanRows = append(trainingPlanRows, []interface{}{
				date.Format("2.1.2006"), "", "", "Бягане", fmt.Sprintf("%d", 10+i), "01:00:00", "", "", fmt.Sprintf("%d", 4+i%3), "Run",
			})
		}
	}
	
	mockSheets := &MockSheetsClient{
		readRangeData: trainingPlanRows,
	}
	
	service := NewProcessingService(mockStrava, mockSheets, nil, createTestLogger())
	config := createTestConfig(1, "Europe/Sofia")
	
	// Pre-populate training cache
	trainingCache := make(TrainingPlanCache)
	for i := 0; i < 8; i++ {
		dayOffset := -i
		date := now.AddDate(0, 0, dayOffset)
		dateKey := date.Format("2006-01-02")
		
		if i%3 == 2 { // Every third day is rest
			trainingCache[dateKey] = &TrainingPlanEntry{
				Date:         date,
				ActivityType: "Почивка",
				Description:  "Rest day",
				RPE:          1,
				IsProcessed:  false,
				Row:          i + 2,
			}
		} else {
			distance := float64(10 + i)
			timeStr := "01:00:00"
			trainingCache[dateKey] = &TrainingPlanEntry{
				Date:         date,
				ActivityType: "Бягане",
				Description:  "Run",
				RPE:          float64(4 + i%3),
				IsProcessed:  false,
				Row:          i + 2,
				Distance:     &distance,
				Time:         &timeStr,
			}
		}
	}
	
	// Pre-populate activities cache
	activitiesCache := make(StravaActivitiesCache)
	for i := 0; i < 7; i++ {
		dayOffset := -i - 1 // -1 to -7 days
		date := now.AddDate(0, 0, dayOffset)
		dateKey := date.Format("2006-01-02")
		
		if i < len(activities) {
			activitiesCache[dateKey] = []strava.Activity{activities[i]}
		} else {
			activitiesCache[dateKey] = []strava.Activity{}
		}
	}
	// Also add today with empty activities
	activitiesCache[now.Format("2006-01-02")] = []strava.Activity{}
	
	result, err := service.RunScheduledCycle(context.Background(), config, trainingCache, activitiesCache)
	
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	
	// Should process multiple days (not today, but previous day + lookback)
	if result.ProcessedDays < 2 {
		t.Errorf("Expected at least 2 processed days, got %d", result.ProcessedDays)
	}
	
	// Verify updates were prepared in detailed results
	updateCount := 0
	for _, dr := range result.DetailedResults {
		if dr.SpreadsheetUpdate != nil {
			updateCount++
		}
	}
	
	if updateCount < 2 {
		t.Errorf("Expected at least 2 spreadsheet updates, got %d", updateCount)
	}
}

// Test that RunScheduledCycle does NOT process today
func TestRunScheduledCycle_DoesNotProcessToday(t *testing.T) {
	location, _ := time.LoadLocation("Europe/Sofia")
	now := time.Now().In(location)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	
	// Add an activity for today
	mockStrava := &MockStravaClient{
		activities: []strava.Activity{
			{
				ID:         999,
				Name:       "Today's Run",
				Type:       "Run",
				Distance:   5000,
				MovingTime: 1800,
				StartDate:  today.Add(6 * time.Hour), // Morning run today
			},
		},
	}
	
	// Include today in the training plan
	mockSheets := &MockSheetsClient{
		readRangeData: [][]interface{}{
			{today.Format("2.1.2006"), "", "", "Бягане", "10", "01:00:00", "", "", "5", "Today's planned run"},
			{today.AddDate(0, 0, -1).Format("2.1.2006"), "", "", "Бягане", "8", "00:45:00", "", "", "4", "Yesterday's run"},
		},
	}
	
	service := NewProcessingService(mockStrava, mockSheets, nil, createTestLogger())
	config := createTestConfig(1, "Europe/Sofia")
	
	// Track batch updates
	
	// Override the batch update to track updates
	mockSheets.batchUpdateCalls = 0
	mockSheets.lastBatchUpdates = nil
	
	// Pre-populate caches
	trainingCache := make(TrainingPlanCache)
	// Add today's entry
	trainingCache[today.Format("2006-01-02")] = &TrainingPlanEntry{
		Date:         today,
		ActivityType: "Бягане",
		Description:  "Today's planned run",
		RPE:          5,
		IsProcessed:  false,
		Row:          2,
	}
	// Add yesterday's entry
	yesterday := today.AddDate(0, 0, -1)
	trainingCache[yesterday.Format("2006-01-02")] = &TrainingPlanEntry{
		Date:         yesterday,
		ActivityType: "Бягане",
		Description:  "Yesterday's run",
		RPE:          4,
		IsProcessed:  false,
		Row:          3,
	}
	
	// Pre-populate activities cache
	activitiesCache := make(StravaActivitiesCache)
	activitiesCache[today.Format("2006-01-02")] = []strava.Activity{
		{
			ID:         999,
			Name:       "Today's Run",
			Type:       "Run",
			Distance:   5000,
			MovingTime: 1800,
			StartDate:  today.Add(6 * time.Hour),
		},
	}
	activitiesCache[yesterday.Format("2006-01-02")] = []strava.Activity{}
	// Initialize empty arrays for lookback days
	for i := 2; i <= 8; i++ {
		date := today.AddDate(0, 0, -i)
		activitiesCache[date.Format("2006-01-02")] = []strava.Activity{}
	}
	
	result, err := service.RunScheduledCycle(context.Background(), config, trainingCache, activitiesCache)
	
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	
	// Verify today was NOT processed by checking the detailed results
	for _, dr := range result.DetailedResults {
		if dr.Date.Format("2006-01-02") == today.Format("2006-01-02") {
			t.Error("Today should not be processed in scheduled cycle")
		}
		if dr.SpreadsheetUpdate != nil && strings.Contains(dr.SpreadsheetUpdate.DescriptionValue, "Today's planned run") {
			t.Error("Today's planned run should not be in updates")
		}
	}
	
	// Should still process some days (yesterday/lookback)
	if result.ProcessedDays == 0 {
		t.Error("Expected some days to be processed (not today)")
	}
}

// Test RunScheduledCycle with mixed processed/unprocessed days
func TestRunScheduledCycle_MixedProcessedDays(t *testing.T) {
	location, _ := time.LoadLocation("Europe/Sofia")
	now := time.Now().In(location)
	yesterday := now.AddDate(0, 0, -1)
	
	mockStrava := &MockStravaClient{
		activities: []strava.Activity{},
	}
	
	// Create training plan with some already processed entries (marked with bold text)
	mockSheets := &MockSheetsClient{
		readRangeData: [][]interface{}{
			{yesterday.Format("2.1.2006"), "", "", "Бягане", "10", "01:00:00", "10,00", "01:00:00", "5", "Easy run"},
			{yesterday.AddDate(0, 0, -1).Format("2.1.2006"), "**bold**", "", "Бягане", "12", "01:15:00", "12,00", "01:15:00", "6", "Already processed"},
			{yesterday.AddDate(0, 0, -2).Format("2.1.2006"), "", "", "Бягане", "8", "00:45:00", "", "", "4", "Not processed"},
			{yesterday.AddDate(0, 0, -3).Format("2.1.2006"), "**bold**", "", "Почивка", "", "", "", "", "1", "Already processed rest"},
		},
	}
	
	service := NewProcessingService(mockStrava, mockSheets, nil, createTestLogger())
	config := createTestConfig(1, "Europe/Sofia")
	
	trainingCache := make(TrainingPlanCache)
	activitiesCache := make(StravaActivitiesCache)
	
	result, err := service.RunScheduledCycle(context.Background(), config, trainingCache, activitiesCache)
	
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	
	// Should only process unprocessed days
	if result.ProcessedDays > 2 {
		t.Errorf("Expected at most 2 processed days (skipping already processed), got %d", result.ProcessedDays)
	}
	
	// Verify batch update was called if there were unprocessed days
	if result.ProcessedDays > 0 && mockSheets.batchUpdateCalls == 0 {
		t.Error("Expected batch update to be called for unprocessed days")
	}
}
