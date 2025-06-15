package processing

import (
	"context"
	"fmt"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/automation"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/google"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/strava"
)

// Worker handles processing automation jobs for individual users
// This implements the core processing logic that coordinates user configuration retrieval
// and API client operations as specified in US022, US023, and US024
type Worker struct {
	configService       *automation.ConfigService
	logger              *logger.Logger
	
	// OAuth credentials for API clients
	stravaClientID      string
	stravaClientSecret  string
	googleClientID      string
	googleClientSecret  string
	googleRedirectURL   string
}

// NewWorker creates a new processing worker with required dependencies
func NewWorker(
	configService *automation.ConfigService,
	stravaClientID, stravaClientSecret string,
	googleClientID, googleClientSecret, googleRedirectURL string,
	logger *logger.Logger,
) *Worker {
	return &Worker{
		configService:       configService,
		stravaClientID:      stravaClientID,
		stravaClientSecret:  stravaClientSecret,
		googleClientID:      googleClientID,
		googleClientSecret:  googleClientSecret,
		googleRedirectURL:   googleRedirectURL,
		logger:              logger.WithContext("component", "automation_worker"),
	}
}

// ProcessingResult represents the outcome of processing a user's automation job
type ProcessingResult struct {
	UserID           int           `json:"user_id"`
	Success          bool          `json:"success"`
	ActivitiesCount  int           `json:"activities_count"`
	ProcessingTime   time.Duration `json:"processing_time"`
	Error            string        `json:"error,omitempty"`
	ErrorType        string        `json:"error_type,omitempty"`
	RequiresReauth   bool          `json:"requires_reauth"`
}

// ProcessUser processes automation for a single user
// This method implements the core processing flow:
// 1. Retrieve user configuration (US022)
// 2. Create API clients with token management (US023, US024)
// 3. Fetch activities and write to spreadsheet
// 4. Handle errors gracefully with proper logging
func (w *Worker) ProcessUser(ctx context.Context, userID int) *ProcessingResult {
	startTime := time.Now()
	
	w.logger.Info("Starting automation processing for user",
		"user_id", userID)
	
	result := &ProcessingResult{
		UserID:     userID,
		Success:    false,
		ProcessingTime: 0,
	}
	
	// Step 1: Retrieve user configuration (US022)
	w.logger.Debug("Retrieving user configuration for processing",
		"user_id", userID)
	
	config, err := w.configService.GetProcessingConfigForUser(ctx, userID)
	if err != nil {
		processingDuration := time.Since(startTime)
		w.logger.Error("Failed to retrieve user configuration, skipping user processing",
			"error", err,
			"user_id", userID,
			"processing_duration_ms", processingDuration.Milliseconds())
		
		result.ProcessingTime = processingDuration
		result.Error = fmt.Sprintf("Configuration retrieval failed: %v", err)
		result.ErrorType = "CONFIG_ERROR"
		return result
	}
	
	// Validate that automation is enabled for this user
	if !config.AutomationEnabled {
		processingDuration := time.Since(startTime)
		w.logger.Info("Automation disabled for user, skipping processing",
			"user_id", userID,
			"processing_duration_ms", processingDuration.Milliseconds())
		
		result.ProcessingTime = processingDuration
		result.Error = "Automation is disabled for this user"
		result.ErrorType = "AUTOMATION_DISABLED"
		return result
	}
	
	w.logger.Info("Successfully retrieved user configuration",
		"user_id", userID,
		"email", config.Email,
		"spreadsheet_id", config.SpreadsheetID,
		"timezone", config.Timezone,
		"has_valid_google_token", config.HasValidGoogleToken(),
		"has_valid_strava_token", config.HasValidStravaToken())
	
	// Step 2: Create Strava API client with token management (US023)
	w.logger.Debug("Creating Strava API client with token management",
		"user_id", userID)
	
	stravaClient := strava.NewClient(userID, config.StravaRefreshToken, w.logger)
	stravaClient.SetOAuthCredentials(w.stravaClientID, w.stravaClientSecret)
	
	// Set initial tokens if available
	if config.HasValidStravaToken() {
		stravaClient.SetInitialTokens(config.StravaAccessToken, *config.StravaTokenExpiry)
		w.logger.Debug("Set initial Strava tokens for client",
			"user_id", userID,
			"token_expiry", config.StravaTokenExpiry)
	}
	
	// Step 3: Create Google Sheets API client with token management (US024)
	w.logger.Debug("Creating Google Sheets API client with token management",
		"user_id", userID)
	
	sheetsClient := google.NewSheetsClient(userID, config.GoogleRefreshToken, w.logger)
	sheetsClient.SetOAuthCredentials(w.googleClientID, w.googleClientSecret, w.googleRedirectURL)
	
	// Set initial tokens if available
	if config.HasValidGoogleToken() {
		sheetsClient.SetInitialTokens(config.GoogleAccessToken, *config.GoogleTokenExpiry)
		w.logger.Debug("Set initial Google tokens for client",
			"user_id", userID,
			"token_expiry", config.GoogleTokenExpiry)
	}
	
	// Step 4: Validate spreadsheet access
	w.logger.Debug("Validating Google Sheets access",
		"user_id", userID,
		"spreadsheet_id", config.SpreadsheetID)
	
	if err := sheetsClient.ValidateAccess(ctx, config.SpreadsheetID); err != nil {
		processingDuration := time.Since(startTime)
		
		// Check if this requires re-authorization
		if google.IsReauthRequired(err) {
			w.logger.Warn("Google Sheets access requires user re-authorization",
				"user_id", userID,
				"error", err,
				"processing_duration_ms", processingDuration.Milliseconds())
			
			result.ProcessingTime = processingDuration
			result.Error = "Google Sheets access requires re-authorization"
			result.ErrorType = "GOOGLE_REAUTH_REQUIRED"
			result.RequiresReauth = true
			return result
		}
		
		w.logger.Error("Failed to validate Google Sheets access",
			"error", err,
			"user_id", userID,
			"spreadsheet_id", config.SpreadsheetID,
			"processing_duration_ms", processingDuration.Milliseconds())
		
		result.ProcessingTime = processingDuration
		result.Error = fmt.Sprintf("Sheets access validation failed: %v", err)
		result.ErrorType = "SHEETS_ACCESS_ERROR"
		return result
	}
	
	// Step 5: Fetch activities from Strava
	// Get activities from the last 7 days (configurable in the future)
	since := time.Now().AddDate(0, 0, -7)
	
	w.logger.Debug("Fetching activities from Strava",
		"user_id", userID,
		"since", since.Format(time.RFC3339),
		"athlete_id", config.StravaAthleteID)
	
	activities, err := stravaClient.GetActivities(ctx, since)
	if err != nil {
		processingDuration := time.Since(startTime)
		
		// Check if this requires re-authorization
		if strava.IsReauthRequired(err) {
			w.logger.Warn("Strava access requires user re-authorization",
				"user_id", userID,
				"error", err,
				"processing_duration_ms", processingDuration.Milliseconds())
			
			result.ProcessingTime = processingDuration
			result.Error = "Strava access requires re-authorization"
			result.ErrorType = "STRAVA_REAUTH_REQUIRED"
			result.RequiresReauth = true
			return result
		}
		
		w.logger.Error("Failed to fetch activities from Strava",
			"error", err,
			"user_id", userID,
			"since", since.Format(time.RFC3339),
			"processing_duration_ms", processingDuration.Milliseconds())
		
		result.ProcessingTime = processingDuration
		result.Error = fmt.Sprintf("Strava activity fetch failed: %v", err)
		result.ErrorType = "STRAVA_FETCH_ERROR"
		return result
	}
	
	w.logger.Info("Successfully fetched activities from Strava",
		"user_id", userID,
		"activity_count", len(activities),
		"since", since.Format(time.RFC3339))
	
	// Step 6: Write activities to Google Sheets
	if len(activities) > 0 {
		w.logger.Debug("Writing activities to Google Sheets",
			"user_id", userID,
			"activity_count", len(activities),
			"spreadsheet_id", config.SpreadsheetID)
		
		if err := sheetsClient.WriteActivities(ctx, config.SpreadsheetID, activities); err != nil {
			processingDuration := time.Since(startTime)
			
			// Check if this requires re-authorization
			if google.IsReauthRequired(err) {
				w.logger.Warn("Google Sheets write requires user re-authorization",
					"user_id", userID,
					"error", err,
					"processing_duration_ms", processingDuration.Milliseconds())
				
				result.ProcessingTime = processingDuration
				result.Error = "Google Sheets write requires re-authorization"
				result.ErrorType = "GOOGLE_REAUTH_REQUIRED"
				result.RequiresReauth = true
				return result
			}
			
			w.logger.Error("Failed to write activities to Google Sheets",
				"error", err,
				"user_id", userID,
				"activity_count", len(activities),
				"spreadsheet_id", config.SpreadsheetID,
				"processing_duration_ms", processingDuration.Milliseconds())
			
			result.ProcessingTime = processingDuration
			result.Error = fmt.Sprintf("Sheets write failed: %v", err)
			result.ErrorType = "SHEETS_WRITE_ERROR"
			return result
		}
		
		w.logger.Info("Successfully wrote activities to Google Sheets",
			"user_id", userID,
			"activity_count", len(activities),
			"spreadsheet_id", config.SpreadsheetID)
	} else {
		w.logger.Info("No new activities to write to Google Sheets",
			"user_id", userID,
			"since", since.Format(time.RFC3339))
	}
	
	// Step 7: Complete processing successfully
	processingDuration := time.Since(startTime)
	
	result.Success = true
	result.ActivitiesCount = len(activities)
	result.ProcessingTime = processingDuration
	
	w.logger.Info("Successfully completed automation processing for user",
		"user_id", userID,
		"activity_count", len(activities),
		"processing_duration_ms", processingDuration.Milliseconds(),
		"email", config.Email,
		"spreadsheet_id", config.SpreadsheetID)
	
	return result
}

// ProcessUsers processes automation for multiple users
// This method handles batch processing with individual error isolation
func (w *Worker) ProcessUsers(ctx context.Context, userIDs []int) []*ProcessingResult {
	w.logger.Info("Starting batch automation processing",
		"user_count", len(userIDs))
	
	results := make([]*ProcessingResult, len(userIDs))
	
	for i, userID := range userIDs {
		w.logger.Debug("Processing user in batch",
			"user_id", userID,
			"user_index", i+1,
			"total_users", len(userIDs))
		
		results[i] = w.ProcessUser(ctx, userID)
		
		// Log batch progress
		if results[i].Success {
			w.logger.Debug("User processing completed successfully in batch",
				"user_id", userID,
				"user_index", i+1,
				"total_users", len(userIDs),
				"activities_processed", results[i].ActivitiesCount)
		} else {
			w.logger.Warn("User processing failed in batch",
				"user_id", userID,
				"user_index", i+1,
				"total_users", len(userIDs),
				"error", results[i].Error,
				"error_type", results[i].ErrorType,
				"requires_reauth", results[i].RequiresReauth)
		}
	}
	
	// Calculate batch summary
	successful := 0
	reauthRequired := 0
	totalActivities := 0
	
	for _, result := range results {
		if result.Success {
			successful++
			totalActivities += result.ActivitiesCount
		}
		if result.RequiresReauth {
			reauthRequired++
		}
	}
	
	w.logger.Info("Completed batch automation processing",
		"total_users", len(userIDs),
		"successful_users", successful,
		"failed_users", len(userIDs)-successful,
		"reauth_required_users", reauthRequired,
		"total_activities_processed", totalActivities)
	
	return results
}