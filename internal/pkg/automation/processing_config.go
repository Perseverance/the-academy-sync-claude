package automation

import (
	"fmt"
	"time"
)

// ProcessingConfig contains all configuration required for processing a user's automation job
// This represents the consolidated configuration that the automation engine needs to 
// process data for a specific user during a scheduled or manual run.
type ProcessingConfig struct {
	// User identification
	UserID int `json:"user_id"`
	Email  string `json:"email"`
	
	// Google OAuth credentials (decrypted)
	GoogleAccessToken  string     `json:"-"` // Never serialize sensitive tokens
	GoogleRefreshToken string     `json:"-"` // Never serialize sensitive tokens
	GoogleTokenExpiry  *time.Time `json:"-"` // Never serialize sensitive tokens
	
	// Strava OAuth credentials (decrypted)
	StravaAccessToken  string     `json:"-"` // Never serialize sensitive tokens
	StravaRefreshToken string     `json:"-"` // Never serialize sensitive tokens
	StravaTokenExpiry  *time.Time `json:"-"` // Never serialize sensitive tokens
	StravaAthleteID    *int64     `json:"strava_athlete_id"`
	
	// Target configuration
	SpreadsheetID string `json:"spreadsheet_id"`
	Timezone      string `json:"timezone"`
	
	// User preferences
	EmailNotificationsEnabled bool `json:"email_notifications_enabled"`
	AutomationEnabled         bool `json:"automation_enabled"`
}

// ValidationError represents a configuration validation failure
type ValidationError struct {
	Field   string
	Message string
	Cause   error
}

func (e *ValidationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("configuration validation failed for %s: %s (caused by: %v)", e.Field, e.Message, e.Cause)
	}
	return fmt.Sprintf("configuration validation failed for %s: %s", e.Field, e.Message)
}

// Validate ensures that all essential configuration is present and valid for processing
// This implements the validation requirements from US022 acceptance criteria.
func (c *ProcessingConfig) Validate() error {
	// Validate user identification
	if c.UserID <= 0 {
		return &ValidationError{
			Field:   "user_id",
			Message: "must be a positive integer",
		}
	}
	
	if c.Email == "" {
		return &ValidationError{
			Field:   "email",
			Message: "is required for notifications",
		}
	}
	
	// Validate Google OAuth tokens - both are required for Sheets access
	if c.GoogleRefreshToken == "" {
		return &ValidationError{
			Field:   "google_refresh_token",
			Message: "is required for Google Sheets API access",
		}
	}
	
	// Note: Google access token may be empty (will be refreshed if needed)
	// but we validate that token expiry is present if access token exists
	if c.GoogleAccessToken != "" && c.GoogleTokenExpiry == nil {
		return &ValidationError{
			Field:   "google_token_expiry",
			Message: "is required when access token is present",
		}
	}
	
	// Note: We don't validate Google token expiry here since tokens 
	// should be refreshed before validation via the TokenRefreshService
	
	// Validate Strava OAuth tokens - refresh token is essential
	if c.StravaRefreshToken == "" {
		return &ValidationError{
			Field:   "strava_refresh_token",
			Message: "is required for Strava API access",
		}
	}
	
	// Validate Strava athlete ID - must be present for API calls
	if c.StravaAthleteID == nil || *c.StravaAthleteID <= 0 {
		return &ValidationError{
			Field:   "strava_athlete_id",
			Message: "is required for Strava API access",
		}
	}
	
	// Note: Strava access token may be empty (will be refreshed if needed)
	// but we validate that token expiry is present if access token exists
	if c.StravaAccessToken != "" && c.StravaTokenExpiry == nil {
		return &ValidationError{
			Field:   "strava_token_expiry",
			Message: "is required when access token is present",
		}
	}
	
	// Note: We don't validate Strava token expiry here since tokens 
	// should be refreshed before validation via the TokenRefreshService
	
	// Validate target configuration
	if c.SpreadsheetID == "" {
		return &ValidationError{
			Field:   "spreadsheet_id",
			Message: "is required for data output destination",
		}
	}
	
	if c.Timezone == "" {
		return &ValidationError{
			Field:   "timezone",
			Message: "is required for proper date/time processing",
		}
	}
	
	// Validate timezone format (basic check)
	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return &ValidationError{
			Field:   "timezone",
			Message: "must be a valid timezone location",
			Cause:   err,
		}
	}
	
	return nil
}

// HasValidGoogleToken checks if the Google access token is present and not expired
func (c *ProcessingConfig) HasValidGoogleToken() bool {
	if c.GoogleAccessToken == "" || c.GoogleTokenExpiry == nil {
		return false
	}
	
	// Add 5-minute buffer to account for clock skew and processing time
	return time.Now().Add(5 * time.Minute).Before(*c.GoogleTokenExpiry)
}

// HasValidStravaToken checks if the Strava access token is present and not expired
func (c *ProcessingConfig) HasValidStravaToken() bool {
	if c.StravaAccessToken == "" || c.StravaTokenExpiry == nil {
		return false
	}
	
	// Add 5-minute buffer to account for clock skew and processing time
	return time.Now().Add(5 * time.Minute).Before(*c.StravaTokenExpiry)
}

// GetLocation returns the parsed timezone location for date/time operations
func (c *ProcessingConfig) GetLocation() (*time.Location, error) {
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return nil, &ValidationError{
			Field:   "timezone",
			Message: "failed to parse timezone location",
			Cause:   err,
		}
	}
	return loc, nil
}

// String returns a safe string representation (without sensitive tokens)
func (c *ProcessingConfig) String() string {
	return fmt.Sprintf("ProcessingConfig{UserID: %d, Email: %s, SpreadsheetID: %s, Timezone: %s, "+
		"HasGoogleTokens: %t, HasStravaTokens: %t, StravaAthleteID: %v, AutomationEnabled: %t}",
		c.UserID, c.Email, c.SpreadsheetID, c.Timezone,
		c.GoogleRefreshToken != "", c.StravaRefreshToken != "", c.StravaAthleteID, c.AutomationEnabled)
}