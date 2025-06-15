package services

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/database"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

// ConfigService handles configuration operations for user settings
type ConfigService struct {
	userRepository *database.UserRepository
	sheetsService  *SheetsService
	logger         *logger.Logger
}

// NewConfigService creates a new configuration service
func NewConfigService(userRepository *database.UserRepository, sheetsService *SheetsService, logger *logger.Logger) *ConfigService {
	return &ConfigService{
		userRepository: userRepository,
		sheetsService:  sheetsService,
		logger:         logger.WithContext("component", "config_service"),
	}
}

// ConfigError represents configuration-related errors
type ConfigError struct {
	Type    string
	Message string
	Cause   error
}

func (e *ConfigError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Configuration error types
const (
	ConfigErrorInvalidURL     = "INVALID_URL"
	ConfigErrorPermission     = "PERMISSION_ERROR"
	ConfigErrorNotFound       = "NOT_FOUND"
	ConfigErrorDatabase       = "DATABASE_ERROR"
	ConfigErrorValidation     = "VALIDATION_ERROR"
	ConfigErrorNetwork        = "NETWORK_ERROR"
)

// SetSpreadsheetURL validates and sets a user's Google Spreadsheet configuration
func (c *ConfigService) SetSpreadsheetURL(ctx context.Context, userID int, spreadsheetURL string) error {
	startTime := time.Now()
	c.logger.Info("Starting spreadsheet URL configuration",
		"user_id", userID,
		"url_length", len(spreadsheetURL))

	// Step 1: Validate and extract spreadsheet ID from URL
	spreadsheetID, err := c.extractSpreadsheetID(spreadsheetURL)
	if err != nil {
		c.logger.Warn("Invalid spreadsheet URL format",
			"user_id", userID,
			"url", c.sanitizeURL(spreadsheetURL),
			"error", err)
		return &ConfigError{
			Type:    ConfigErrorInvalidURL,
			Message: "Invalid Google Spreadsheet URL format. Please ensure you're using a valid Google Sheets URL.",
			Cause:   err,
		}
	}

	c.logger.Debug("Successfully extracted spreadsheet ID",
		"user_id", userID,
		"spreadsheet_id", spreadsheetID)

	// Step 2: Validate user has access to the spreadsheet
	c.logger.Debug("Validating spreadsheet access",
		"user_id", userID,
		"spreadsheet_id", spreadsheetID)

	err = c.sheetsService.ValidateSpreadsheetAccess(ctx, userID, spreadsheetID)
	if err != nil {
		// Convert SheetsService errors to ConfigService errors
		if sheetsErr, ok := err.(*SpreadsheetValidationError); ok {
			switch sheetsErr.Type {
			case ErrorTypePermissionDenied:
				return &ConfigError{
					Type:    ConfigErrorPermission,
					Message: sheetsErr.Message,
					Cause:   sheetsErr.Cause,
				}
			case ErrorTypeNotFound:
				return &ConfigError{
					Type:    ConfigErrorNotFound,
					Message: sheetsErr.Message,
					Cause:   sheetsErr.Cause,
				}
			case ErrorTypeInvalidFormat:
				return &ConfigError{
					Type:    ConfigErrorInvalidURL,
					Message: sheetsErr.Message,
					Cause:   sheetsErr.Cause,
				}
			case ErrorTypeNetworkError:
				return &ConfigError{
					Type:    ConfigErrorNetwork,
					Message: sheetsErr.Message,
					Cause:   sheetsErr.Cause,
				}
			default:
				return &ConfigError{
					Type:    ConfigErrorValidation,
					Message: "Failed to validate spreadsheet access: " + sheetsErr.Message,
					Cause:   sheetsErr.Cause,
				}
			}
		}

		c.logger.Error("Unexpected error during spreadsheet validation",
			"error", err,
			"user_id", userID,
			"spreadsheet_id", spreadsheetID)
		return &ConfigError{
			Type:    ConfigErrorValidation,
			Message: "Failed to validate spreadsheet access. Please try again.",
			Cause:   err,
		}
	}

	c.logger.Debug("Spreadsheet access validation successful",
		"user_id", userID,
		"spreadsheet_id", spreadsheetID)

	// Step 3: Save spreadsheet ID to database
	c.logger.Debug("Saving spreadsheet ID to database",
		"user_id", userID,
		"spreadsheet_id", spreadsheetID)

	err = c.userRepository.UpdateSpreadsheetID(ctx, userID, spreadsheetID)
	if err != nil {
		c.logger.Error("Failed to save spreadsheet ID to database",
			"error", err,
			"user_id", userID,
			"spreadsheet_id", spreadsheetID)
		return &ConfigError{
			Type:    ConfigErrorDatabase,
			Message: "Failed to save spreadsheet configuration. Please try again.",
			Cause:   err,
		}
	}

	duration := time.Since(startTime)
	c.logger.Info("Spreadsheet configuration completed successfully",
		"user_id", userID,
		"spreadsheet_id", spreadsheetID,
		"configuration_duration_ms", duration.Milliseconds())

	return nil
}

// ClearSpreadsheetURL removes a user's spreadsheet configuration
func (c *ConfigService) ClearSpreadsheetURL(ctx context.Context, userID int) error {
	c.logger.Info("Clearing spreadsheet configuration",
		"user_id", userID)

	err := c.userRepository.ClearSpreadsheetID(ctx, userID)
	if err != nil {
		c.logger.Error("Failed to clear spreadsheet configuration",
			"error", err,
			"user_id", userID)
		return &ConfigError{
			Type:    ConfigErrorDatabase,
			Message: "Failed to clear spreadsheet configuration. Please try again.",
			Cause:   err,
		}
	}

	c.logger.Info("Spreadsheet configuration cleared successfully",
		"user_id", userID)

	return nil
}

// extractSpreadsheetID extracts the spreadsheet ID from a Google Sheets URL
func (c *ConfigService) extractSpreadsheetID(url string) (string, error) {
	c.logger.Debug("Extracting spreadsheet ID from URL",
		"url_length", len(url))

	// Trim whitespace
	url = strings.TrimSpace(url)

	if url == "" {
		return "", fmt.Errorf("URL cannot be empty")
	}

	// Google Sheets URL patterns:
	// https://docs.google.com/spreadsheets/d/{SPREADSHEET_ID}/edit#gid=0
	// https://docs.google.com/spreadsheets/d/{SPREADSHEET_ID}/edit
	// https://docs.google.com/spreadsheets/d/{SPREADSHEET_ID}
	
	// Regular expression to extract spreadsheet ID
	// The spreadsheet ID is a string of alphanumeric characters, hyphens, and underscores
	patterns := []string{
		`https://docs\.google\.com/spreadsheets/d/([a-zA-Z0-9-_]+)`,
		`docs\.google\.com/spreadsheets/d/([a-zA-Z0-9-_]+)`,
	}

	for _, pattern := range patterns {
		c.logger.Debug("Testing URL pattern",
			"pattern", pattern)

		regex, err := regexp.Compile(pattern)
		if err != nil {
			c.logger.Error("Failed to compile regex pattern",
				"pattern", pattern,
				"error", err)
			continue
		}

		matches := regex.FindStringSubmatch(url)
		if len(matches) >= 2 {
			spreadsheetID := matches[1]
			c.logger.Debug("Successfully extracted spreadsheet ID",
				"spreadsheet_id", spreadsheetID,
				"pattern", pattern)
			
			// Validate spreadsheet ID format
			if len(spreadsheetID) < 10 || len(spreadsheetID) > 100 {
				c.logger.Warn("Spreadsheet ID has unusual length",
					"spreadsheet_id", spreadsheetID,
					"length", len(spreadsheetID))
			}

			return spreadsheetID, nil
		}
	}

	c.logger.Warn("No matching pattern found for URL",
		"url", c.sanitizeURL(url))

	return "", fmt.Errorf("invalid Google Spreadsheet URL format. Expected format: https://docs.google.com/spreadsheets/d/{SPREADSHEET_ID}")
}

// sanitizeURL removes sensitive parts of URL for logging
func (c *ConfigService) sanitizeURL(url string) string {
	// For privacy, only log the domain and structure, not the full URL
	if strings.Contains(url, "docs.google.com") {
		if strings.Contains(url, "/spreadsheets/d/") {
			return "https://docs.google.com/spreadsheets/d/[ID]..."
		}
		return "https://docs.google.com/..."
	}
	
	// For non-Google URLs, just show the domain
	if len(url) > 50 {
		return url[:30] + "..."
	}
	return url
}