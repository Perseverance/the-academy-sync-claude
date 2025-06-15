package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/database"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
)

// SheetsService handles Google Sheets API operations
type SheetsService struct {
	userRepository *database.UserRepository
	logger         *logger.Logger
}

// NewSheetsService creates a new Google Sheets service
func NewSheetsService(userRepository *database.UserRepository, logger *logger.Logger) *SheetsService {
	return &SheetsService{
		userRepository: userRepository,
		logger:         logger.WithContext("component", "sheets_service"),
	}
}

// SpreadsheetValidationError represents different types of validation failures
type SpreadsheetValidationError struct {
	Type    string
	Message string
	Cause   error
}

func (e *SpreadsheetValidationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Error types for different validation failures
const (
	ErrorTypePermissionDenied = "PERMISSION_DENIED"
	ErrorTypeNotFound         = "NOT_FOUND"
	ErrorTypeInvalidFormat    = "INVALID_FORMAT"
	ErrorTypeNetworkError     = "NETWORK_ERROR"
	ErrorTypeUnknown          = "UNKNOWN"
)

// ValidateSpreadsheetAccess validates that the user has read/write access to the specified spreadsheet
func (s *SheetsService) ValidateSpreadsheetAccess(ctx context.Context, userID int, spreadsheetID string) error {
	startTime := time.Now()
	s.logger.Debug("Starting spreadsheet access validation",
		"user_id", userID,
		"spreadsheet_id", spreadsheetID)

	// Get user's Google OAuth tokens
	accessToken, refreshToken, expiry, err := s.userRepository.GetDecryptedGoogleTokens(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get user's Google tokens",
			"error", err,
			"user_id", userID)
		return &SpreadsheetValidationError{
			Type:    ErrorTypeUnknown,
			Message: "Failed to retrieve authentication tokens",
			Cause:   err,
		}
	}

	if accessToken == "" {
		s.logger.Warn("User has no Google access token",
			"user_id", userID)
		return &SpreadsheetValidationError{
			Type:    ErrorTypePermissionDenied,
			Message: "No Google authentication found. Please reconnect your Google account.",
		}
	}

	// Create OAuth2 token
	token := &oauth2.Token{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Expiry:       *expiry,
	}

	s.logger.Debug("Retrieved user tokens",
		"user_id", userID,
		"token_expires_at", expiry,
		"token_valid", time.Now().Before(*expiry))

	// Create authenticated Sheets API client
	sheetsService, err := s.createSheetsClient(ctx, token)
	if err != nil {
		s.logger.Error("Failed to create Sheets API client",
			"error", err,
			"user_id", userID)
		return &SpreadsheetValidationError{
			Type:    ErrorTypeNetworkError,
			Message: "Failed to initialize Google Sheets API client",
			Cause:   err,
		}
	}

	// Test read access by getting spreadsheet metadata
	s.logger.Debug("Testing read access to spreadsheet",
		"spreadsheet_id", spreadsheetID,
		"user_id", userID)

	spreadsheet, err := sheetsService.Spreadsheets.Get(spreadsheetID).Context(ctx).Do()
	if err != nil {
		return s.handleSheetsAPIError(err, "read access validation", userID, spreadsheetID)
	}

	s.logger.Debug("Successfully retrieved spreadsheet metadata",
		"spreadsheet_id", spreadsheetID,
		"spreadsheet_title", spreadsheet.Properties.Title,
		"user_id", userID)

	// Test write access by attempting to read a range (this requires write scope)
	// We use a minimal operation that doesn't modify data but validates permissions
	testRange := "A1:A1" // Read just one cell
	s.logger.Debug("Testing write access permissions",
		"spreadsheet_id", spreadsheetID,
		"test_range", testRange,
		"user_id", userID)

	_, err = sheetsService.Spreadsheets.Values.Get(spreadsheetID, testRange).Context(ctx).Do()
	if err != nil {
		return s.handleSheetsAPIError(err, "write access validation", userID, spreadsheetID)
	}

	duration := time.Since(startTime)
	s.logger.Info("Spreadsheet access validation successful",
		"user_id", userID,
		"spreadsheet_id", spreadsheetID,
		"spreadsheet_title", spreadsheet.Properties.Title,
		"validation_duration_ms", duration.Milliseconds())

	return nil
}

// createSheetsClient creates an authenticated Google Sheets API client
func (s *SheetsService) createSheetsClient(ctx context.Context, token *oauth2.Token) (*sheets.Service, error) {
	s.logger.Debug("Creating Google Sheets API client")

	// Create OAuth2 config for token source (we don't need the full config, just for token refresh)
	tokenSource := oauth2.StaticTokenSource(token)

	// Create Sheets service with authenticated client
	sheetsService, err := sheets.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		s.logger.Error("Failed to create Sheets service", "error", err)
		return nil, err
	}

	s.logger.Debug("Successfully created Google Sheets API client")
	return sheetsService, nil
}

// handleSheetsAPIError processes Google Sheets API errors and returns appropriate validation errors
func (s *SheetsService) handleSheetsAPIError(err error, operation string, userID int, spreadsheetID string) *SpreadsheetValidationError {
	s.logger.Error("Google Sheets API error",
		"error", err,
		"operation", operation,
		"user_id", userID,
		"spreadsheet_id", spreadsheetID)

	errorString := err.Error()

	// Parse common Google API error patterns
	switch {
	case contains(errorString, "403") || contains(errorString, "Forbidden") || contains(errorString, "permission"):
		s.logger.Warn("Permission denied for spreadsheet access",
			"user_id", userID,
			"spreadsheet_id", spreadsheetID)
		return &SpreadsheetValidationError{
			Type:    ErrorTypePermissionDenied,
			Message: "You don't have permission to access this spreadsheet. Please check that the spreadsheet is shared with your Google account with edit permissions.",
			Cause:   err,
		}
	case contains(errorString, "404") || contains(errorString, "Not Found"):
		s.logger.Warn("Spreadsheet not found",
			"user_id", userID,
			"spreadsheet_id", spreadsheetID)
		return &SpreadsheetValidationError{
			Type:    ErrorTypeNotFound,
			Message: "Spreadsheet not found. Please check that the URL is correct and the spreadsheet exists.",
			Cause:   err,
		}
	case contains(errorString, "400") || contains(errorString, "Bad Request"):
		s.logger.Warn("Invalid spreadsheet ID format",
			"user_id", userID,
			"spreadsheet_id", spreadsheetID)
		return &SpreadsheetValidationError{
			Type:    ErrorTypeInvalidFormat,
			Message: "Invalid spreadsheet ID format. Please check the URL and try again.",
			Cause:   err,
		}
	default:
		s.logger.Error("Unknown Google Sheets API error",
			"error", err,
			"user_id", userID,
			"spreadsheet_id", spreadsheetID)
		return &SpreadsheetValidationError{
			Type:    ErrorTypeNetworkError,
			Message: "Unable to access Google Sheets. Please try again later.",
			Cause:   err,
		}
	}
}

// contains checks if a string contains a substring (case-insensitive helper)
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}