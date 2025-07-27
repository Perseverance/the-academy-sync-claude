package services

import (
	"context"
	"fmt"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/database"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

// Note: ErrStravaConnectionRequired and ErrSpreadsheetRequired are already defined in sync_service.go
// We reuse them here for consistency

// ErrInvalidLanguagePreference is returned when an unsupported language code is provided
var ErrInvalidLanguagePreference = fmt.Errorf("invalid language preference: must be 'bg' or 'en'")

// supportedLanguages defines the list of supported language codes
var supportedLanguages = map[string]bool{
	"bg": true, // Bulgarian
	"en": true, // English
}

// SettingsService handles user settings operations
type SettingsService struct {
	userRepo *database.UserRepository
	logger   *logger.Logger
}

// NewSettingsService creates a new settings service
func NewSettingsService(userRepo *database.UserRepository, logger *logger.Logger) *SettingsService {
	return &SettingsService{
		userRepo: userRepo,
		logger:   logger,
	}
}

// UpdateUserSettings updates the user's settings with validation
func (s *SettingsService) UpdateUserSettings(ctx context.Context, userID int, automationEnabled, emailNotificationsEnabled bool, languagePreference string) error {
	// Validate language preference if provided
	if languagePreference != "" && !supportedLanguages[languagePreference] {
		s.logger.Warn("Invalid language preference provided", 
			"user_id", userID, 
			"language_preference", languagePreference,
			"supported_languages", []string{"bg", "en"})
		return ErrInvalidLanguagePreference
	}

	// If trying to enable automation, validate requirements
	if automationEnabled {
		// Fetch user to check requirements
		user, err := s.userRepo.GetUserByID(ctx, userID)
		if err != nil {
			s.logger.Error("Failed to fetch user for validation", "error", err, "user_id", userID)
			return fmt.Errorf("failed to fetch user: %w", err)
		}

		// Check if user has Strava connection
		if len(user.StravaRefreshToken) == 0 {
			s.logger.Info("User tried to enable automation without Strava connection", "user_id", userID)
			return ErrStravaConnectionRequired
		}

		// Check if user has spreadsheet configured
		if user.SpreadsheetID == nil || *user.SpreadsheetID == "" {
			s.logger.Info("User tried to enable automation without spreadsheet", "user_id", userID)
			return ErrSpreadsheetRequired
		}

		// Timezone is already enforced as NOT NULL with default 'UTC' in database
		s.logger.Info("User meets all requirements for automation", 
			"user_id", userID,
			"has_strava", true,
			"has_spreadsheet", true,
			"timezone", user.Timezone)
	}

	// Update the settings
	err := s.userRepo.UpdateUserSettings(ctx, userID, automationEnabled, emailNotificationsEnabled, languagePreference)
	if err != nil {
		s.logger.Error("Failed to update user settings", 
			"error", err, 
			"user_id", userID,
			"automation_enabled", automationEnabled,
			"email_notifications_enabled", emailNotificationsEnabled,
			"language_preference", languagePreference)
		return fmt.Errorf("failed to update settings: %w", err)
	}

	s.logger.Info("Successfully updated user settings",
		"user_id", userID,
		"automation_enabled", automationEnabled,
		"email_notifications_enabled", emailNotificationsEnabled,
		"language_preference", languagePreference)

	return nil
}