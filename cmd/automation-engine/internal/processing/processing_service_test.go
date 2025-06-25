package processing

import (
	"context"
	"fmt"
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

// MockSheetsClient implements a mock Google Sheets client for testing
type MockSheetsClient struct {
	readRangeData     [][]interface{}
	readRangeErr      error
	readRangeCalls    int
	lastReadRange     string
	validateAccessErr error
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
	
	service := NewProcessingService(mockStrava, mockSheets, createTestLogger())
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
			t.Errorf("Expected RPE 5, got %d", entry.RPE)
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
	
	service := NewProcessingService(mockStrava, mockSheets, createTestLogger())
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
	service := NewProcessingService(nil, nil, createTestLogger())
	
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
	service := NewProcessingService(nil, nil, createTestLogger())
	
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
	
	service := NewProcessingService(mockStrava, mockSheets, createTestLogger())
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
	
	service := NewProcessingService(mockStrava, mockSheets, createTestLogger())
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
	
	service := NewProcessingService(mockStrava, mockSheets, createTestLogger())
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
		t.Errorf("Expected RPE 2 for rest day with activity, got %d", result.SpreadsheetUpdate.RPEValue)
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
	
	service := NewProcessingService(mockStrava, mockSheets, createTestLogger())
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
	
	service := NewProcessingService(mockStrava, mockSheets, createTestLogger())
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
			mockSheets.readRangeCalls - initialCalls)
	}
}

// Test empty spreadsheet handling
func TestFetchAllTrainingPlanEntries_EmptySpreadsheet(t *testing.T) {
	mockStrava := &MockStravaClient{}
	mockSheets := &MockSheetsClient{
		readRangeData: [][]interface{}{}, // Empty data
	}
	
	service := NewProcessingService(mockStrava, mockSheets, createTestLogger())
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
	service := NewProcessingService(nil, nil, createTestLogger())
	
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
		{3600, "01:00:00"},     // Exactly on 00
		{3601, "01:00:00"},     // 01 rounds down to 00
		{3602, "01:00:00"},     // 02 rounds down to 00
		{3603, "01:00:05"},     // 03 rounds up to 05
		{3604, "01:00:05"},     // 04 rounds up to 05
		{3605, "01:00:05"},     // 05 stays at 05
		{3606, "01:00:05"},     // 06 rounds down to 05
		{3607, "01:00:05"},     // 07 rounds down to 05
		{3608, "01:00:10"},     // 08 rounds up to 10
		{3658, "01:01:00"},     // 58 rounds up to 60, carries over
		{3659, "01:01:00"},     // 59 rounds up to 60, carries over
		{7199, "02:00:00"},     // 01:59:59 rounds to 02:00:00
		{0, "00:00:00"},        // Zero stays zero
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
	service := NewProcessingService(nil, nil, createTestLogger())
	
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
	
	service := NewProcessingService(mockStrava, mockSheets, createTestLogger())
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
	
	service := NewProcessingService(mockStrava, mockSheets, createTestLogger())
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
	
	service := NewProcessingService(mockStrava, mockSheets, createTestLogger())
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