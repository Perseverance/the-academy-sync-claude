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
	GetProcessingConfigForUser(ctx context.Context, userID int) (*database.ProcessingTokens, error)
	DecryptToken(encryptedToken []byte) (string, error)
}

// ConfigService coordinates user configuration retrieval for automation processing
// This service implements US022 requirements for securely retrieving all necessary
// operational configurations from the database for a specific user's processing run.
type ConfigService struct {
	userRepository      UserRepository
	tokenRefreshService *TokenRefreshService
	logger              *logger.Logger
}

// NewConfigService creates a new configuration service
func NewConfigService(userRepository UserRepository, tokenRefreshService *TokenRefreshService, logger *logger.Logger) *ConfigService {
	return &ConfigService{
		userRepository:      userRepository,
		tokenRefreshService: tokenRefreshService,
		logger:              logger.WithContext("component", "config_service"),
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

	// Retrieve complete user record for automation preferences
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

	// Retrieve decrypted processing tokens
	s.logger.Debug("Retrieving decrypted processing tokens",
		"user_id", userID)
	
	tokens, err := s.userRepository.GetProcessingConfigForUser(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to retrieve processing tokens from database",
			"error", err,
			"user_id", userID,
			"operation_duration_ms", time.Since(startTime).Milliseconds())
		return nil, fmt.Errorf("failed to retrieve processing tokens: %w", err)
	}

	s.logger.Debug("Successfully retrieved decrypted processing tokens",
		"user_id", userID,
		"has_google_access_token", tokens.GoogleAccessToken != "",
		"has_google_refresh_token", tokens.GoogleRefreshToken != "",
		"has_strava_access_token", tokens.StravaAccessToken != "",
		"has_strava_refresh_token", tokens.StravaRefreshToken != "",
		"has_strava_athlete_id", tokens.StravaAthleteID != nil,
		"has_spreadsheet_id", tokens.SpreadsheetID != nil,
		"google_token_expiry", tokens.GoogleTokenExpiry,
		"strava_token_expiry", tokens.StravaTokenExpiry)

	// Build processing configuration
	config := &ProcessingConfig{
		UserID: user.ID,
		Email:  tokens.Email,

		// Google OAuth tokens (decrypted)
		GoogleAccessToken:  tokens.GoogleAccessToken,
		GoogleRefreshToken: tokens.GoogleRefreshToken,
		GoogleTokenExpiry:  tokens.GoogleTokenExpiry,

		// Strava OAuth tokens (decrypted)
		StravaAccessToken:  tokens.StravaAccessToken,
		StravaRefreshToken: tokens.StravaRefreshToken,
		StravaTokenExpiry:  tokens.StravaTokenExpiry,
		StravaAthleteID:    tokens.StravaAthleteID,

		// Target configuration
		SpreadsheetID: "",
		Timezone:      tokens.Timezone,

		// User preferences
		EmailNotificationsEnabled: user.EmailNotificationsEnabled,
		AutomationEnabled:         user.AutomationEnabled,
	}

	// Handle spreadsheet ID (can be nil)
	if tokens.SpreadsheetID != nil {
		config.SpreadsheetID = *tokens.SpreadsheetID
	}

	s.logger.Debug("Built processing configuration from user data",
		"user_id", userID,
		"config_summary", config.String())

	// Refresh tokens if needed and token refresh service is available
	if s.tokenRefreshService != nil {
		s.logger.Debug("Checking if token refresh is needed", "user_id", userID)
		
		refreshedConfig, err := s.tokenRefreshService.RefreshTokensIfNeeded(ctx, config)
		if err != nil {
			s.logger.Error("Failed to refresh tokens",
				"error", err,
				"user_id", userID,
				"operation_duration_ms", time.Since(startTime).Milliseconds())
			return nil, fmt.Errorf("failed to refresh tokens: %w", err)
		}
		
		// Use the refreshed config
		config = refreshedConfig
		
		s.logger.Debug("Token refresh check completed",
			"user_id", userID,
			"has_valid_google_token", config.HasValidGoogleToken(),
			"has_valid_strava_token", config.HasValidStravaToken())
	} else {
		s.logger.Debug("Token refresh service not available, skipping token refresh", "user_id", userID)
	}

	// Validate configuration completeness
	s.logger.Debug("Validating processing configuration completeness",
		"user_id", userID)

	if err := config.Validate(); err != nil {
		s.logger.Error("❌ Processing configuration validation failed",
			"error", err,
			"user_id", userID,
			"step", "config_validation",
			"validation_failure", map[string]interface{}{
				"error_type":     fmt.Sprintf("%T", err),
				"error_message":  err.Error(),
				"config_summary": config.String(),
				"validation_checklist": map[string]interface{}{
					"has_google_refresh":   config.GoogleRefreshToken != "",
					"has_strava_refresh":   config.StravaRefreshToken != "",
					"has_athlete_id":       config.StravaAthleteID != nil,
					"has_spreadsheet_id":   config.SpreadsheetID != "",
					"has_timezone":         config.Timezone != "",
					"automation_enabled":   config.AutomationEnabled,
					"valid_timezone":       func() bool {
						if config.Timezone == "" {
							return false
						}
						_, err := time.LoadLocation(config.Timezone)
						return err == nil
					}(),
				},
			},
			"operation_duration_ms", time.Since(startTime).Milliseconds(),
			"troubleshooting_tips", []string{
				"Check that user has completed OAuth flow for both Google and Strava",
				"Verify user has configured a Google Spreadsheet ID",
				"Ensure user has set a valid timezone in their profile",
				"Confirm that automation is enabled in user settings",
			})
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	// Log comprehensive token validity status for debugging
	s.logger.Debug("🔍 Token validity analysis for user",
		"user_id", userID,
		"token_analysis", map[string]interface{}{
			"google": map[string]interface{}{
				"has_access_token":    config.GoogleAccessToken != "",
				"has_refresh_token":   config.GoogleRefreshToken != "",
				"token_valid":         config.HasValidGoogleToken(),
				"token_expiry":        config.GoogleTokenExpiry,
				"minutes_until_expiry": func() float64 {
					if config.GoogleTokenExpiry != nil {
						return time.Until(*config.GoogleTokenExpiry).Minutes()
					}
					return -1
				}(),
			},
			"strava": map[string]interface{}{
				"has_access_token":     config.StravaAccessToken != "",
				"has_refresh_token":    config.StravaRefreshToken != "",
				"token_valid":          config.HasValidStravaToken(),
				"token_expiry":         config.StravaTokenExpiry,
				"athlete_id":           config.StravaAthleteID,
				"minutes_until_expiry": func() float64 {
					if config.StravaTokenExpiry != nil {
						return time.Until(*config.StravaTokenExpiry).Minutes()
					}
					return -1
				}(),
			},
		})

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

	// Check if automation is enabled
	if !user.AutomationEnabled {
		missingFields = append(missingFields, "automation_enabled")
	}

	// Validate timezone if present
	if user.Timezone != "" {
		if _, err := time.LoadLocation(user.Timezone); err != nil {
			missingFields = append(missingFields, "valid_timezone")
		}
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