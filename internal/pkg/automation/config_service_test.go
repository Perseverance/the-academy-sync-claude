package automation

import (
	"context"
	"testing"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/database"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

// MockUserRepository implements a mock user repository for testing
type MockUserRepository struct {
	users  map[int]*database.User
	tokens map[int]*database.ProcessingTokens
}

func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		users:  make(map[int]*database.User),
		tokens: make(map[int]*database.ProcessingTokens),
	}
}

func (m *MockUserRepository) GetUserByID(ctx context.Context, userID int) (*database.User, error) {
	user, exists := m.users[userID]
	if !exists {
		return nil, nil
	}
	return user, nil
}

func (m *MockUserRepository) GetProcessingConfigForUser(ctx context.Context, userID int) (*database.ProcessingTokens, error) {
	tokens, exists := m.tokens[userID]
	if !exists {
		return nil, nil
	}
	return tokens, nil
}

func (m *MockUserRepository) DecryptToken(encryptedToken []byte) (string, error) {
	// For testing, just return the token as string
	return string(encryptedToken), nil
}

func (m *MockUserRepository) AddUser(userID int, user *database.User) {
	m.users[userID] = user
}

func (m *MockUserRepository) AddProcessingTokens(userID int, tokens *database.ProcessingTokens) {
	m.tokens[userID] = tokens
}

func TestConfigService_GetProcessingConfigForUser(t *testing.T) {
	log := logger.New("test")
	mockRepo := NewMockUserRepository()
	configService := NewConfigService(mockRepo, nil, log) // nil token refresh for tests

	// Test case 1: User not found
	t.Run("UserNotFound", func(t *testing.T) {
		config, err := configService.GetProcessingConfigForUser(context.Background(), 999)
		if err == nil {
			t.Error("Expected error for non-existent user")
		}
		if config != nil {
			t.Error("Expected nil config for non-existent user")
		}
	})

	// Test case 2: User with complete configuration
	t.Run("CompleteConfiguration", func(t *testing.T) {
		now := time.Now()
		futureExpiry := now.Add(time.Hour)
		spreadsheetID := "test-spreadsheet-id"
		athleteID := int64(12345)

		user := &database.User{
			ID:                        1,
			EmailNotificationsEnabled: true,
			AutomationEnabled:         true,
		}

		tokens := &database.ProcessingTokens{
			GoogleAccessToken:  "google-access-token",
			GoogleRefreshToken: "google-refresh-token",
			GoogleTokenExpiry:  &futureExpiry,
			StravaAccessToken:  "strava-access-token",
			StravaRefreshToken: "strava-refresh-token",
			StravaTokenExpiry:  &futureExpiry,
			StravaAthleteID:    &athleteID,
			SpreadsheetID:      &spreadsheetID,
			Timezone:           "America/New_York",
			Email:              "test@example.com",
		}

		mockRepo.AddUser(1, user)
		mockRepo.AddProcessingTokens(1, tokens)

		config, err := configService.GetProcessingConfigForUser(context.Background(), 1)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if config == nil {
			t.Fatal("Expected config to be non-nil")
		}

		// Validate configuration
		if config.UserID != 1 {
			t.Errorf("Expected UserID 1, got %d", config.UserID)
		}
		if config.Email != "test@example.com" {
			t.Errorf("Expected email test@example.com, got %s", config.Email)
		}
		if config.GoogleRefreshToken != "google-refresh-token" {
			t.Errorf("Expected Google refresh token, got %s", config.GoogleRefreshToken)
		}
		if config.StravaRefreshToken != "strava-refresh-token" {
			t.Errorf("Expected Strava refresh token, got %s", config.StravaRefreshToken)
		}
		if config.SpreadsheetID != "test-spreadsheet-id" {
			t.Errorf("Expected spreadsheet ID, got %s", config.SpreadsheetID)
		}
		if !config.HasValidGoogleToken() {
			t.Error("Expected valid Google token")
		}
		if !config.HasValidStravaToken() {
			t.Error("Expected valid Strava token")
		}
	})

	// Test case 3: User with missing Strava configuration
	t.Run("MissingStravaConfig", func(t *testing.T) {
		now := time.Now()
		futureExpiry := now.Add(time.Hour)
		spreadsheetID := "test-spreadsheet-id"

		user := &database.User{
			ID:                        2,
			EmailNotificationsEnabled: true,
			AutomationEnabled:         true,
		}

		tokens := &database.ProcessingTokens{
			GoogleAccessToken:  "google-access-token",
			GoogleRefreshToken: "google-refresh-token",
			GoogleTokenExpiry:  &futureExpiry,
			StravaAccessToken:  "", // Empty
			StravaRefreshToken: "", // Empty
			StravaTokenExpiry:  nil,
			StravaAthleteID:    nil, // Missing
			SpreadsheetID:      &spreadsheetID,
			Timezone:           "America/New_York",
			Email:              "test2@example.com",
		}

		mockRepo.AddUser(2, user)
		mockRepo.AddProcessingTokens(2, tokens)

		config, err := configService.GetProcessingConfigForUser(context.Background(), 2)
		if err == nil {
			t.Error("Expected validation error for missing Strava configuration")
		}
		if config != nil {
			t.Error("Expected nil config for invalid configuration")
		}
	})
}

func TestProcessingConfig_Validate(t *testing.T) {
	now := time.Now()
	futureExpiry := now.Add(time.Hour)
	athleteID := int64(12345)

	// Test case 1: Valid configuration
	t.Run("ValidConfig", func(t *testing.T) {
		config := &ProcessingConfig{
			UserID:             1,
			Email:              "test@example.com",
			GoogleRefreshToken: "google-refresh-token",
			GoogleTokenExpiry:  &futureExpiry,
			StravaRefreshToken: "strava-refresh-token",
			StravaTokenExpiry:  &futureExpiry,
			StravaAthleteID:    &athleteID,
			SpreadsheetID:      "test-spreadsheet-id",
			Timezone:           "America/New_York",
		}

		err := config.Validate()
		if err != nil {
			t.Errorf("Unexpected validation error: %v", err)
		}
	})

	// Test case 2: Missing Google refresh token
	t.Run("MissingGoogleRefreshToken", func(t *testing.T) {
		config := &ProcessingConfig{
			UserID:             1,
			Email:              "test@example.com",
			GoogleRefreshToken: "", // Missing
			StravaRefreshToken: "strava-refresh-token",
			StravaAthleteID:    &athleteID,
			SpreadsheetID:      "test-spreadsheet-id",
			Timezone:           "America/New_York",
		}

		err := config.Validate()
		if err == nil {
			t.Error("Expected validation error for missing Google refresh token")
		}
		if validationErr, ok := err.(*ValidationError); ok {
			if validationErr.Field != "google_refresh_token" {
				t.Errorf("Expected error for google_refresh_token, got %s", validationErr.Field)
			}
		} else {
			t.Error("Expected ValidationError type")
		}
	})

	// Test case 3: Invalid timezone
	t.Run("InvalidTimezone", func(t *testing.T) {
		config := &ProcessingConfig{
			UserID:             1,
			Email:              "test@example.com",
			GoogleRefreshToken: "google-refresh-token",
			StravaRefreshToken: "strava-refresh-token",
			StravaAthleteID:    &athleteID,
			SpreadsheetID:      "test-spreadsheet-id",
			Timezone:           "Invalid/Timezone",
		}

		err := config.Validate()
		if err == nil {
			t.Error("Expected validation error for invalid timezone")
		}
		if validationErr, ok := err.(*ValidationError); ok {
			if validationErr.Field != "timezone" {
				t.Errorf("Expected error for timezone, got %s", validationErr.Field)
			}
		}
	})
}

func TestProcessingConfig_TokenValidation(t *testing.T) {
	now := time.Now()
	pastExpiry := now.Add(-time.Hour)
	futureExpiry := now.Add(time.Hour)
	soonExpiry := now.Add(2 * time.Minute) // Within 5-minute buffer

	// Test case 1: Valid Google token
	t.Run("ValidGoogleToken", func(t *testing.T) {
		config := &ProcessingConfig{
			GoogleAccessToken: "valid-token",
			GoogleTokenExpiry: &futureExpiry,
		}

		if !config.HasValidGoogleToken() {
			t.Error("Expected valid Google token")
		}
	})

	// Test case 2: Expired Google token
	t.Run("ExpiredGoogleToken", func(t *testing.T) {
		config := &ProcessingConfig{
			GoogleAccessToken: "expired-token",
			GoogleTokenExpiry: &pastExpiry,
		}

		if config.HasValidGoogleToken() {
			t.Error("Expected invalid Google token for expired token")
		}
	})

	// Test case 3: Soon-to-expire Google token (within buffer)
	t.Run("SoonExpiredGoogleToken", func(t *testing.T) {
		config := &ProcessingConfig{
			GoogleAccessToken: "soon-expired-token",
			GoogleTokenExpiry: &soonExpiry,
		}

		if config.HasValidGoogleToken() {
			t.Error("Expected invalid Google token for soon-to-expire token")
		}
	})

	// Test case 4: Missing Google token
	t.Run("MissingGoogleToken", func(t *testing.T) {
		config := &ProcessingConfig{
			GoogleAccessToken: "",
			GoogleTokenExpiry: &futureExpiry,
		}

		if config.HasValidGoogleToken() {
			t.Error("Expected invalid Google token for missing token")
		}
	})
}