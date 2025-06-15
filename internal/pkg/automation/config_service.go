package automation

import (
	"context"
	"fmt"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/database"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

// UserRepository interface for dependency injection and testing
type UserRepository interface {
	GetUserByID(ctx context.Context, userID int) (*database.User, error)
	DecryptToken(encryptedToken []byte) (string, error)
}

// ConfigService coordinates user configuration retrieval for automation processing
// This service implements US022 requirements for securely retrieving all necessary
// operational configurations from the database for a specific user's processing run.
type ConfigService struct {
	userRepository UserRepository
	logger         *logger.Logger
}

// NewConfigService creates a new configuration service
func NewConfigService(userRepository UserRepository, logger *logger.Logger) *ConfigService {
	return &ConfigService{
		userRepository: userRepository,
		logger:         logger.WithContext("component", "config_service"),
	}
}

// GetProcessingConfigForUser retrieves and validates all necessary configuration for a user's processing run
// This method implements the core requirements from US022:
// - Retrieves all essential operational configurations from database
// - Includes OAuth refresh tokens, spreadsheet ID, email, timezone, and preferences
// - Handles sensitive tokens securely in memory only
// - Fails gracefully with detailed logging if any essential configuration is missing
func (s *ConfigService) GetProcessingConfigForUser(ctx context.Context, userID int) (*ProcessingConfig, error) {
	startTime := time.Now()
	s.logger.Debug("Starting user configuration retrieval for processing run",
		"user_id", userID)

	// Retrieve complete user record from database
	user, err := s.userRepository.GetUserByID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to retrieve user record from database",
			"error", err,
			"user_id", userID,
			"operation_duration_ms", time.Since(startTime).Milliseconds())
		return nil, fmt.Errorf("failed to retrieve user record: %w", err)
	}

	if user == nil {
		s.logger.Warn("User not found in database",
			"user_id", userID,
			"operation_duration_ms", time.Since(startTime).Milliseconds())
		return nil, fmt.Errorf("user not found: %d", userID)
	}

	s.logger.Debug("Retrieved user record from database",
		"user_id", userID,
		"email", user.Email,
		"has_strava_athlete_id", user.StravaAthleteID != nil,
		"has_spreadsheet_id", user.SpreadsheetID != nil,
		"automation_enabled", user.AutomationEnabled)

	// Decrypt Google OAuth tokens
	s.logger.Debug("Decrypting Google OAuth tokens",
		"user_id", userID)
	
	googleAccessToken, err := s.userRepository.DecryptToken(user.GoogleAccessToken)
	if err != nil {
		s.logger.Error("Failed to decrypt Google access token",
			"error", err,
			"user_id", userID)
		return nil, fmt.Errorf("failed to decrypt Google access token: %w", err)
	}

	googleRefreshToken, err := s.userRepository.DecryptToken(user.GoogleRefreshToken)
	if err != nil {
		s.logger.Error("Failed to decrypt Google refresh token",
			"error", err,
			"user_id", userID)
		return nil, fmt.Errorf("failed to decrypt Google refresh token: %w", err)
	}

	s.logger.Debug("Successfully decrypted Google OAuth tokens",
		"user_id", userID,
		"has_google_access_token", googleAccessToken != "",
		"has_google_refresh_token", googleRefreshToken != "",
		"google_token_expiry", user.GoogleTokenExpiry)

	// Decrypt Strava OAuth tokens
	s.logger.Debug("Decrypting Strava OAuth tokens",
		"user_id", userID)

	stravaAccessToken, err := s.userRepository.DecryptToken(user.StravaAccessToken)
	if err != nil {
		s.logger.Error("Failed to decrypt Strava access token",
			"error", err,
			"user_id", userID)
		return nil, fmt.Errorf("failed to decrypt Strava access token: %w", err)
	}

	stravaRefreshToken, err := s.userRepository.DecryptToken(user.StravaRefreshToken)
	if err != nil {
		s.logger.Error("Failed to decrypt Strava refresh token",
			"error", err,
			"user_id", userID)
		return nil, fmt.Errorf("failed to decrypt Strava refresh token: %w", err)
	}

	s.logger.Debug("Successfully decrypted Strava OAuth tokens",
		"user_id", userID,
		"has_strava_access_token", stravaAccessToken != "",
		"has_strava_refresh_token", stravaRefreshToken != "",
		"strava_token_expiry", user.StravaTokenExpiry,
		"strava_athlete_id", user.StravaAthleteID)

	// Build processing configuration
	config := &ProcessingConfig{
		UserID: user.ID,
		Email:  user.Email,

		// Google OAuth tokens (decrypted)
		GoogleAccessToken:  googleAccessToken,
		GoogleRefreshToken: googleRefreshToken,
		GoogleTokenExpiry:  user.GoogleTokenExpiry,

		// Strava OAuth tokens (decrypted)
		StravaAccessToken:  stravaAccessToken,
		StravaRefreshToken: stravaRefreshToken,
		StravaTokenExpiry:  user.StravaTokenExpiry,
		StravaAthleteID:    user.StravaAthleteID,

		// Target configuration
		SpreadsheetID: "",
		Timezone:      user.Timezone,

		// User preferences
		EmailNotificationsEnabled: user.EmailNotificationsEnabled,
		AutomationEnabled:         user.AutomationEnabled,
	}

	// Handle spreadsheet ID (can be nil)
	if user.SpreadsheetID != nil {
		config.SpreadsheetID = *user.SpreadsheetID
	}

	s.logger.Debug("Built processing configuration from user data",
		"user_id", userID,
		"config_summary", config.String())

	// Validate configuration completeness
	s.logger.Debug("Validating processing configuration completeness",
		"user_id", userID)

	if err := config.Validate(); err != nil {
		s.logger.Error("Processing configuration validation failed",
			"error", err,
			"user_id", userID,
			"config_summary", config.String(),
			"operation_duration_ms", time.Since(startTime).Milliseconds())
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	// Log token validity status for debugging
	s.logger.Debug("Token validity status",
		"user_id", userID,
		"google_token_valid", config.HasValidGoogleToken(),
		"strava_token_valid", config.HasValidStravaToken(),
		"google_token_expiry", config.GoogleTokenExpiry,
		"strava_token_expiry", config.StravaTokenExpiry)

	operationDuration := time.Since(startTime)
	s.logger.Info("Successfully retrieved and validated processing configuration",
		"user_id", userID,
		"email", config.Email,
		"spreadsheet_id", config.SpreadsheetID,
		"timezone", config.Timezone,
		"automation_enabled", config.AutomationEnabled,
		"has_valid_google_token", config.HasValidGoogleToken(),
		"has_valid_strava_token", config.HasValidStravaToken(),
		"operation_duration_ms", operationDuration.Milliseconds())

	return config, nil
}

// ValidateUserCanBeProcessed performs a quick validation check to determine if a user
// has the minimum required configuration for processing without fully retrieving all data.
// This can be used for job queue filtering before expensive operations.
func (s *ConfigService) ValidateUserCanBeProcessed(ctx context.Context, userID int) error {
	s.logger.Debug("Performing quick user processing validation",
		"user_id", userID)

	// Get basic user information
	user, err := s.userRepository.GetUserByID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to retrieve user for validation",
			"error", err,
			"user_id", userID)
		return fmt.Errorf("failed to retrieve user: %w", err)
	}

	if user == nil {
		s.logger.Warn("User not found for validation",
			"user_id", userID)
		return fmt.Errorf("user not found: %d", userID)
	}

	// Check essential fields without decryption
	var missingFields []string

	if len(user.GoogleRefreshToken) == 0 {
		missingFields = append(missingFields, "google_refresh_token")
	}

	if len(user.StravaRefreshToken) == 0 {
		missingFields = append(missingFields, "strava_refresh_token")
	}

	if user.StravaAthleteID == nil {
		missingFields = append(missingFields, "strava_athlete_id")
	}

	if user.SpreadsheetID == nil || *user.SpreadsheetID == "" {
		missingFields = append(missingFields, "spreadsheet_id")
	}

	if user.Timezone == "" {
		missingFields = append(missingFields, "timezone")
	}

	if len(missingFields) > 0 {
		s.logger.Warn("User missing essential configuration for processing",
			"user_id", userID,
			"missing_fields", missingFields)
		return fmt.Errorf("user missing essential configuration: %v", missingFields)
	}

	s.logger.Debug("User passed quick processing validation",
		"user_id", userID,
		"automation_enabled", user.AutomationEnabled)

	return nil
}