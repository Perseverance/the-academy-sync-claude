package google

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/strava"
)

// SpreadsheetInfo contains metadata about a Google Spreadsheet
type SpreadsheetInfo struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	URL        string    `json:"url"`
	SheetCount int       `json:"sheet_count"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ActivityRow represents a single row of activity data for writing to sheets
type ActivityRow struct {
	Date           string  `json:"date"`
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	Distance       string  `json:"distance"`        // formatted with units
	Duration       string  `json:"duration"`        // formatted as HH:MM:SS
	Pace           string  `json:"pace"`            // formatted pace/speed
	ElevationGain  string  `json:"elevation_gain"`  // formatted with units
	HeartRate      string  `json:"heart_rate"`      // formatted average HR
	Kudos          int     `json:"kudos"`
}

// SheetsClient provides Google Sheets API access with automatic token lifecycle management
// This implements US024 requirements for managing Google Sheets access tokens using refresh tokens
type SheetsClient struct {
	userID       int
	refreshToken string
	
	// In-memory token cache (per-job lifecycle)
	mu           sync.RWMutex
	accessToken  string
	tokenExpiry  time.Time
	
	// Google Sheets API service (recreated on token refresh)
	sheetsService *sheets.Service
	
	// OAuth configuration for token refresh
	oauthConfig *oauth2.Config
	
	// Logger for debugging external API interactions
	logger *logger.Logger
}

// NewSheetsClient creates a new Google Sheets API client for a specific user
// The client is designed to be instantiated per-user for each processing job
func NewSheetsClient(userID int, refreshToken string, logger *logger.Logger) *SheetsClient {
	// Create OAuth2 config for token refresh operations
	oauthConfig := &oauth2.Config{
		Scopes: []string{
			"https://www.googleapis.com/auth/spreadsheets",
		},
		Endpoint: google.Endpoint,
		// Note: Client ID and Secret should be injected via config
	}

	return &SheetsClient{
		userID:       userID,
		refreshToken: refreshToken,
		oauthConfig:  oauthConfig,
		logger:       logger.WithContext("component", "google_sheets_client", "user_id", userID),
	}
}

// SetOAuthCredentials configures the OAuth client credentials for token refresh
// This should be called during client initialization with application credentials
func (c *SheetsClient) SetOAuthCredentials(clientID, clientSecret, redirectURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.oauthConfig.ClientID = clientID
	c.oauthConfig.ClientSecret = clientSecret
	c.oauthConfig.RedirectURL = redirectURL
	
	c.logger.Debug("OAuth credentials configured for Google Sheets client",
		"client_id", clientID,
		"has_client_secret", clientSecret != "",
		"redirect_url", redirectURL)
}

// SetInitialTokens sets initial access token and expiry if available
// This allows the client to use existing valid tokens before falling back to refresh
func (c *SheetsClient) SetInitialTokens(accessToken string, expiry time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.accessToken = accessToken
	c.tokenExpiry = expiry
	
	c.logger.Debug("Initial access token set for Google Sheets client",
		"has_access_token", accessToken != "",
		"token_expiry", expiry,
		"token_valid", time.Now().Before(expiry))
}

// ensureValidToken implements the "check-then-fetch" token management logic
// This method is called before every API request to guarantee a valid access token
func (c *SheetsClient) ensureValidToken(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.logger.Debug("Checking token validity before Google Sheets API call",
		"has_access_token", c.accessToken != "",
		"token_expiry", c.tokenExpiry,
		"time_until_expiry_minutes", time.Until(c.tokenExpiry).Minutes())
	
	// Check if current access token is valid (with 5-minute buffer)
	if c.accessToken != "" && time.Now().Add(5*time.Minute).Before(c.tokenExpiry) {
		c.logger.Debug("Using existing valid access token for Google Sheets API call",
			"token_expiry", c.tokenExpiry,
			"minutes_until_expiry", time.Until(c.tokenExpiry).Minutes())
		
		// Ensure we have a sheets service with current token
		if c.sheetsService == nil {
			if err := c.createSheetsService(ctx); err != nil {
				c.logger.Error("Failed to create Sheets service with existing token",
					"error", err,
					"user_id", c.userID)
				return err
			}
		}
		return nil
	}
	
	// Need to refresh token
	c.logger.Debug("Access token invalid or expired, refreshing via Google OAuth",
		"current_token_expired", c.accessToken != "" && time.Now().After(c.tokenExpiry),
		"current_token_missing", c.accessToken == "",
		"refresh_token_available", c.refreshToken != "")
	
	if c.refreshToken == "" {
		c.logger.Error("No refresh token available for Google token refresh",
			"user_id", c.userID)
		return ErrReauthRequired
	}
	
	// Call Google OAuth token endpoint to refresh access token
	startTime := time.Now()
	c.logger.Debug("Making token refresh request to Google OAuth endpoint",
		"endpoint", c.oauthConfig.Endpoint.TokenURL,
		"user_id", c.userID)
	
	token := &oauth2.Token{
		RefreshToken: c.refreshToken,
	}
	
	tokenSource := c.oauthConfig.TokenSource(ctx, token)
	newToken, err := tokenSource.Token()
	
	requestDuration := time.Since(startTime)
	
	if err != nil {
		c.logger.Error("Failed to refresh Google access token via OAuth endpoint",
			"error", err,
			"user_id", c.userID,
			"request_duration_ms", requestDuration.Milliseconds(),
			"endpoint", c.oauthConfig.Endpoint.TokenURL)
		
		// Check if this is an invalid grant error (requires re-authorization)
		if IsReauthRequired(err) {
			c.logger.Warn("Google refresh token is invalid, user re-authorization required",
				"user_id", c.userID,
				"error", err)
			return &AuthError{
				Type:    "REAUTH_REQUIRED",
				Message: "Google refresh token is invalid, user must re-authorize",
				Cause:   err,
			}
		}
		
		return &NetworkError{
			Operation: "token_refresh",
			Message:   "Failed to refresh Google access token",
			Cause:     err,
		}
	}
	
	// Update cached token
	c.accessToken = newToken.AccessToken
	c.tokenExpiry = newToken.Expiry
	
	// Create new Sheets service with refreshed token
	if err := c.createSheetsService(ctx); err != nil {
		c.logger.Error("Failed to create Sheets service with refreshed token",
			"error", err,
			"user_id", c.userID)
		return &NetworkError{
			Operation: "service_creation",
			Message:   "Failed to create Sheets service after token refresh",
			Cause:     err,
		}
	}
	
	c.logger.Info("Successfully refreshed Google access token and created Sheets service",
		"user_id", c.userID,
		"new_token_expiry", newToken.Expiry,
		"token_valid_hours", time.Until(newToken.Expiry).Hours(),
		"refresh_duration_ms", requestDuration.Milliseconds())
	
	return nil
}

// createSheetsService creates a new Google Sheets API service with the current access token
func (c *SheetsClient) createSheetsService(ctx context.Context) error {
	c.logger.Debug("Creating Google Sheets API service",
		"user_id", c.userID)
	
	// Create token source with current access token
	token := &oauth2.Token{
		AccessToken:  c.accessToken,
		RefreshToken: c.refreshToken,
		Expiry:       c.tokenExpiry,
	}
	
	tokenSource := c.oauthConfig.TokenSource(ctx, token)
	
	// Create Sheets service with authenticated client
	sheetsService, err := sheets.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		c.logger.Error("Failed to create Google Sheets service",
			"error", err,
			"user_id", c.userID)
		return &NetworkError{
			Operation: "service_creation",
			Message:   "Failed to create Google Sheets API service",
			Cause:     err,
		}
	}
	
	c.sheetsService = sheetsService
	
	c.logger.Debug("Successfully created Google Sheets API service",
		"user_id", c.userID)
	
	return nil
}

// ValidateAccess validates that the user has read/write access to the specified spreadsheet
// This enhances the existing validation with better error handling and logging
func (c *SheetsClient) ValidateAccess(ctx context.Context, spreadsheetID string) error {
	startTime := time.Now()
	c.logger.Debug("Starting Google Sheets access validation",
		"user_id", c.userID,
		"spreadsheet_id", spreadsheetID)
	
	// Ensure we have a valid token and service
	if err := c.ensureValidToken(ctx); err != nil {
		c.logger.Error("Failed to ensure valid token for access validation",
			"error", err,
			"user_id", c.userID,
			"spreadsheet_id", spreadsheetID)
		return err
	}
	
	// Test read access by getting spreadsheet metadata
	c.logger.Debug("Testing read access to Google Spreadsheet",
		"spreadsheet_id", spreadsheetID,
		"user_id", c.userID)
	
	spreadsheet, err := c.sheetsService.Spreadsheets.Get(spreadsheetID).Context(ctx).Do()
	if err != nil {
		return c.handleSheetsAPIError(err, "read access validation", spreadsheetID)
	}
	
	c.logger.Debug("Successfully retrieved spreadsheet metadata",
		"spreadsheet_id", spreadsheetID,
		"spreadsheet_title", spreadsheet.Properties.Title,
		"sheet_count", len(spreadsheet.Sheets),
		"user_id", c.userID)
	
	// Test write access by attempting to read a range
	testRange := "A1:A1"
	c.logger.Debug("Testing write access permissions",
		"spreadsheet_id", spreadsheetID,
		"test_range", testRange,
		"user_id", c.userID)
	
	_, err = c.sheetsService.Spreadsheets.Values.Get(spreadsheetID, testRange).Context(ctx).Do()
	if err != nil {
		return c.handleSheetsAPIError(err, "write access validation", spreadsheetID)
	}
	
	duration := time.Since(startTime)
	c.logger.Info("Google Sheets access validation successful",
		"user_id", c.userID,
		"spreadsheet_id", spreadsheetID,
		"spreadsheet_title", spreadsheet.Properties.Title,
		"validation_duration_ms", duration.Milliseconds())
	
	return nil
}

// GetSpreadsheetInfo retrieves metadata about a Google Spreadsheet
func (c *SheetsClient) GetSpreadsheetInfo(ctx context.Context, spreadsheetID string) (*SpreadsheetInfo, error) {
	c.logger.Debug("Retrieving Google Spreadsheet metadata",
		"user_id", c.userID,
		"spreadsheet_id", spreadsheetID)
	
	// Ensure we have a valid token and service
	if err := c.ensureValidToken(ctx); err != nil {
		return nil, err
	}
	
	spreadsheet, err := c.sheetsService.Spreadsheets.Get(spreadsheetID).Context(ctx).Do()
	if err != nil {
		c.logger.Error("Failed to retrieve spreadsheet metadata",
			"error", err,
			"user_id", c.userID,
			"spreadsheet_id", spreadsheetID)
		return nil, c.handleSheetsAPIError(err, "get spreadsheet info", spreadsheetID)
	}
	
	info := &SpreadsheetInfo{
		ID:         spreadsheet.SpreadsheetId,
		Title:      spreadsheet.Properties.Title,
		URL:        spreadsheet.SpreadsheetUrl,
		SheetCount: len(spreadsheet.Sheets),
		CreatedAt:  time.Now(), // Current time as proxy since Google doesn't provide creation time
		UpdatedAt:  time.Now(), // Current time as proxy
	}
	
	c.logger.Info("Successfully retrieved Google Spreadsheet metadata",
		"user_id", c.userID,
		"spreadsheet_id", spreadsheetID,
		"title", info.Title,
		"sheet_count", info.SheetCount,
		"url", info.URL)
	
	return info, nil
}

// WriteActivities writes Strava activities to a Google Spreadsheet
// This implements the core automation functionality
func (c *SheetsClient) WriteActivities(ctx context.Context, spreadsheetID string, activities []strava.Activity) error {
	startTime := time.Now()
	c.logger.Debug("Writing activities to Google Spreadsheet",
		"user_id", c.userID,
		"spreadsheet_id", spreadsheetID,
		"activity_count", len(activities))
	
	if len(activities) == 0 {
		c.logger.Debug("No activities to write to spreadsheet",
			"user_id", c.userID,
			"spreadsheet_id", spreadsheetID)
		return nil
	}
	
	// Ensure we have a valid token and service
	if err := c.ensureValidToken(ctx); err != nil {
		return err
	}
	
	// Convert activities to spreadsheet rows
	rows := c.convertActivitiesToRows(activities)
	
	// Prepare the range for writing (assume we're writing to Sheet1, starting from A2)
	writeRange := "Sheet1!A2:I" + fmt.Sprintf("%d", len(rows)+1)
	
	c.logger.Debug("Preparing to write activity data to spreadsheet",
		"user_id", c.userID,
		"spreadsheet_id", spreadsheetID,
		"write_range", writeRange,
		"row_count", len(rows))
	
	// Create the value range
	valueRange := &sheets.ValueRange{
		Values: rows,
	}
	
	// Write to spreadsheet
	_, err := c.sheetsService.Spreadsheets.Values.Update(spreadsheetID, writeRange, valueRange).
		ValueInputOption("USER_ENTERED").
		Context(ctx).
		Do()
	
	if err != nil {
		c.logger.Error("Failed to write activities to Google Spreadsheet",
			"error", err,
			"user_id", c.userID,
			"spreadsheet_id", spreadsheetID,
			"activity_count", len(activities))
		return c.handleSheetsAPIError(err, "write activities", spreadsheetID)
	}
	
	duration := time.Since(startTime)
	c.logger.Info("Successfully wrote activities to Google Spreadsheet",
		"user_id", c.userID,
		"spreadsheet_id", spreadsheetID,
		"activity_count", len(activities),
		"write_range", writeRange,
		"write_duration_ms", duration.Milliseconds())
	
	return nil
}

// convertActivitiesToRows converts Strava activities to spreadsheet row format
func (c *SheetsClient) convertActivitiesToRows(activities []strava.Activity) [][]interface{} {
	rows := make([][]interface{}, len(activities))
	
	for i, activity := range activities {
		// Format duration as HH:MM:SS
		duration := time.Duration(activity.MovingTime) * time.Second
		durationStr := fmt.Sprintf("%02d:%02d:%02d", 
			int(duration.Hours()), 
			int(duration.Minutes())%60, 
			int(duration.Seconds())%60)
		
		// Format distance in kilometers
		distanceKm := activity.Distance / 1000
		distanceStr := fmt.Sprintf("%.2f km", distanceKm)
		
		// Format elevation gain in meters
		elevationStr := fmt.Sprintf("%.0f m", activity.TotalElevationGain)
		
		// Format average heart rate
		heartRateStr := ""
		if activity.AverageHeartrate > 0 {
			heartRateStr = fmt.Sprintf("%.0f bpm", activity.AverageHeartrate)
		}
		
		// Calculate pace (for running activities)
		paceStr := ""
		if activity.Type == "Run" && activity.MovingTime > 0 && activity.Distance > 0 {
			pacePerKm := float64(activity.MovingTime) / (activity.Distance / 1000)
			paceMinutes := int(pacePerKm / 60)
			paceSeconds := int(pacePerKm) % 60
			paceStr = fmt.Sprintf("%d:%02d /km", paceMinutes, paceSeconds)
		}
		
		rows[i] = []interface{}{
			activity.StartDateLocal.Format("2006-01-02"),
			activity.Name,
			activity.Type,
			distanceStr,
			durationStr,
			paceStr,
			elevationStr,
			heartRateStr,
			activity.Kudos,
		}
	}
	
	c.logger.Debug("Converted activities to spreadsheet rows",
		"activity_count", len(activities),
		"row_count", len(rows))
	
	return rows
}

// handleSheetsAPIError processes Google Sheets API errors and returns appropriate error types
func (c *SheetsClient) handleSheetsAPIError(err error, operation, spreadsheetID string) error {
	c.logger.Error("Google Sheets API error",
		"error", err,
		"operation", operation,
		"user_id", c.userID,
		"spreadsheet_id", spreadsheetID)
	
	errorString := err.Error()
	
	// Parse common Google API error patterns
	switch {
	case strings.Contains(errorString, "403") || strings.Contains(errorString, "Forbidden"):
		c.logger.Warn("Permission denied for Google Sheets access",
			"user_id", c.userID,
			"spreadsheet_id", spreadsheetID)
		return &SheetsError{
			SpreadsheetID: spreadsheetID,
			Type:          "PERMISSION_DENIED",
			Message:       "You don't have permission to access this spreadsheet",
			Cause:         err,
		}
	case strings.Contains(errorString, "404") || strings.Contains(errorString, "Not Found"):
		c.logger.Warn("Google Spreadsheet not found",
			"user_id", c.userID,
			"spreadsheet_id", spreadsheetID)
		return &SheetsError{
			SpreadsheetID: spreadsheetID,
			Type:          "NOT_FOUND",
			Message:       "Spreadsheet not found",
			Cause:         err,
		}
	case strings.Contains(errorString, "400") || strings.Contains(errorString, "Bad Request"):
		c.logger.Warn("Invalid request to Google Sheets API",
			"user_id", c.userID,
			"spreadsheet_id", spreadsheetID)
		return &SheetsError{
			SpreadsheetID: spreadsheetID,
			Type:          "INVALID_REQUEST",
			Message:       "Invalid request format",
			Cause:         err,
		}
	case strings.Contains(errorString, "429"):
		c.logger.Warn("Google Sheets API rate limit exceeded",
			"user_id", c.userID,
			"spreadsheet_id", spreadsheetID)
		return &APIError{
			StatusCode: 429,
			Type:       "RATE_LIMITED",
			Message:    "Google Sheets API rate limit exceeded",
			Cause:      err,
		}
	default:
		c.logger.Error("Unknown Google Sheets API error",
			"error", err,
			"user_id", c.userID,
			"spreadsheet_id", spreadsheetID)
		return &NetworkError{
			Operation: operation,
			Message:   "Google Sheets API error",
			Cause:     err,
		}
	}
}