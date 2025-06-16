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
//
// Enhanced Debug Logging Features:
// - Step-by-step processing with emojis and clear identifiers
// - Comprehensive error analysis with troubleshooting information
// - Token validity status and expiry time tracking
// - OAuth credential verification and diagnostics
// - Performance timing for each processing step
// - Structured logging with searchable fields for monitoring
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
	
	w.logger.Info("🚀 Starting automation processing for user",
		"user_id", userID,
		"context_deadline", func() string {
			if deadline, ok := ctx.Deadline(); ok {
				return deadline.Format(time.RFC3339)
			}
			return "no_deadline"
		}(),
		"worker_oauth_config", map[string]bool{
			"has_strava_client_id":     w.stravaClientID != "",
			"has_strava_client_secret": w.stravaClientSecret != "",
			"has_google_client_id":     w.googleClientID != "",
			"has_google_client_secret": w.googleClientSecret != "",
		})
	
	result := &ProcessingResult{
		UserID:     userID,
		Success:    false,
		ProcessingTime: 0,
	}
	
	// Step 1: Retrieve user configuration (US022)
	w.logger.Debug("📋 Step 1/8: Retrieving user configuration for processing",
		"user_id", userID,
		"step", "config_retrieval")
	
	config, err := w.configService.GetProcessingConfigForUser(ctx, userID)
	if err != nil {
		processingDuration := time.Since(startTime)
		w.logger.Error("❌ FATAL: Failed to retrieve user configuration, skipping user processing",
			"error", err,
			"error_details", map[string]interface{}{
				"error_type": fmt.Sprintf("%T", err),
				"error_string": err.Error(),
			},
			"user_id", userID,
			"step", "config_retrieval",
			"processing_duration_ms", processingDuration.Milliseconds(),
			"failure_reason", "Cannot proceed without valid user configuration")
		
		result.ProcessingTime = processingDuration
		result.Error = fmt.Sprintf("Configuration retrieval failed: %v", err)
		result.ErrorType = "CONFIG_ERROR"
		return result
	}
	
	// Validate that automation is enabled for this user
	if !config.AutomationEnabled {
		processingDuration := time.Since(startTime)
		w.logger.Info("⏸️ Automation disabled for user, skipping processing",
			"user_id", userID,
			"step", "automation_check",
			"automation_enabled", false,
			"processing_duration_ms", processingDuration.Milliseconds(),
			"skip_reason", "User has disabled automation in their settings")
		
		result.ProcessingTime = processingDuration
		result.Error = "Automation is disabled for this user"
		result.ErrorType = "AUTOMATION_DISABLED"
		return result
	}
	
	w.logger.Info("✅ Step 1/8: Successfully retrieved user configuration",
		"user_id", userID,
		"step", "config_retrieval",
		"config_details", map[string]interface{}{
			"email":                    config.Email,
			"spreadsheet_id":           config.SpreadsheetID,
			"timezone":                 config.Timezone,
			"automation_enabled":       config.AutomationEnabled,
			"email_notifications":      config.EmailNotificationsEnabled,
			"has_valid_google_token":   config.HasValidGoogleToken(),
			"has_valid_strava_token":   config.HasValidStravaToken(),
			"has_strava_athlete_id":    config.StravaAthleteID != nil,
			"google_token_expiry":      config.GoogleTokenExpiry,
			"strava_token_expiry":      config.StravaTokenExpiry,
		})
	
	// Step 2: Create Strava API client with token management (US023)
	w.logger.Debug("🏃 Step 2/8: Creating Strava API client with token management",
		"user_id", userID,
		"step", "strava_client_creation",
		"strava_config", map[string]interface{}{
			"has_refresh_token":    config.StravaRefreshToken != "",
			"has_access_token":     config.StravaAccessToken != "",
			"token_valid":          config.HasValidStravaToken(),
			"athlete_id":           config.StravaAthleteID,
			"client_credentials":   w.stravaClientID != "" && w.stravaClientSecret != "",
		})
	
	stravaClient := strava.NewClient(userID, config.StravaRefreshToken, w.logger)
	stravaClient.SetOAuthCredentials(w.stravaClientID, w.stravaClientSecret)
	
	// Set initial tokens if available
	if config.HasValidStravaToken() {
		stravaClient.SetInitialTokens(config.StravaAccessToken, *config.StravaTokenExpiry)
		w.logger.Debug("✅ Set initial Strava tokens for client",
			"user_id", userID,
			"step", "strava_token_init",
			"token_expiry", config.StravaTokenExpiry,
			"minutes_until_expiry", time.Until(*config.StravaTokenExpiry).Minutes())
	} else {
		w.logger.Debug("⚠️ No valid Strava access token, will use refresh token",
			"user_id", userID,
			"step", "strava_token_init",
			"has_refresh_token", config.StravaRefreshToken != "",
			"token_expired", config.StravaTokenExpiry != nil && time.Now().After(*config.StravaTokenExpiry))
	}
	
	// Step 3: Create Google Sheets API client with token management (US024)
	w.logger.Debug("📊 Step 3/8: Creating Google Sheets API client with token management",
		"user_id", userID,
		"step", "google_client_creation",
		"google_config", map[string]interface{}{
			"has_refresh_token":    config.GoogleRefreshToken != "",
			"has_access_token":     config.GoogleAccessToken != "",
			"token_valid":          config.HasValidGoogleToken(),
			"spreadsheet_id":       config.SpreadsheetID,
			"client_credentials":   w.googleClientID != "" && w.googleClientSecret != "",
		})
	
	sheetsClient := google.NewSheetsClient(userID, config.GoogleRefreshToken, w.logger)
	sheetsClient.SetOAuthCredentials(w.googleClientID, w.googleClientSecret, w.googleRedirectURL)
	
	// Set initial tokens if available
	if config.HasValidGoogleToken() {
		sheetsClient.SetInitialTokens(config.GoogleAccessToken, *config.GoogleTokenExpiry)
		w.logger.Debug("✅ Set initial Google tokens for client",
			"user_id", userID,
			"step", "google_token_init",
			"token_expiry", config.GoogleTokenExpiry,
			"minutes_until_expiry", time.Until(*config.GoogleTokenExpiry).Minutes())
	} else {
		w.logger.Debug("⚠️ No valid Google access token, will use refresh token",
			"user_id", userID,
			"step", "google_token_init",
			"has_refresh_token", config.GoogleRefreshToken != "",
			"token_expired", config.GoogleTokenExpiry != nil && time.Now().After(*config.GoogleTokenExpiry))
	}
	
	// Step 4: Refresh OAuth tokens if needed
	w.logger.Debug("🔄 Step 4/8: Refreshing OAuth tokens if needed",
		"user_id", userID,
		"step", "token_refresh",
		"refresh_reason", "Ensuring tokens are valid before API operations")
	
	// Refresh Strava token if needed
	if !config.HasValidStravaToken() {
		w.logger.Debug("🔄 Refreshing Strava token",
			"user_id", userID,
			"step", "strava_token_refresh",
			"token_expired", config.StravaTokenExpiry != nil && time.Now().After(*config.StravaTokenExpiry))
		
		// Token refresh will happen automatically during first API call
		// The strava client handles this internally
	}
	
	// Refresh Google token if needed
	if !config.HasValidGoogleToken() {
		w.logger.Debug("🔄 Refreshing Google token",
			"user_id", userID,
			"step", "google_token_refresh", 
			"token_expired", config.GoogleTokenExpiry != nil && time.Now().After(*config.GoogleTokenExpiry))
		
		// Token refresh will happen automatically during first API call
		// The sheets client handles this internally
	}
	
	w.logger.Info("✅ Step 4/8: Token refresh preparation completed",
		"user_id", userID,
		"step", "token_refresh",
		"strava_token_valid", config.HasValidStravaToken(),
		"google_token_valid", config.HasValidGoogleToken())
	
	// Step 5: Validate spreadsheet access
	w.logger.Debug("🔐 Step 5/8: Validating Google Sheets access",
		"user_id", userID,
		"step", "sheets_access_validation",
		"spreadsheet_id", config.SpreadsheetID,
		"validation_reason", "Ensuring user has read/write permissions before processing")
	
	if err := sheetsClient.ValidateAccess(ctx, config.SpreadsheetID); err != nil {
		processingDuration := time.Since(startTime)
		
		// Check if this requires re-authorization
		if google.IsReauthRequired(err) {
			w.logger.Warn("🔐 Google Sheets access requires user re-authorization",
				"user_id", userID,
				"step", "sheets_access_validation",
				"error", err,
				"error_analysis", map[string]interface{}{
					"error_type":           fmt.Sprintf("%T", err),
					"requires_reauth":      true,
					"spreadsheet_id":       config.SpreadsheetID,
					"google_token_expired": !config.HasValidGoogleToken(),
				},
				"processing_duration_ms", processingDuration.Milliseconds(),
				"action_required", "User must re-authorize Google Sheets access")
			
			result.ProcessingTime = processingDuration
			result.Error = "Google Sheets access requires re-authorization"
			result.ErrorType = "GOOGLE_REAUTH_REQUIRED"
			result.RequiresReauth = true
			return result
		}
		
		w.logger.Error("❌ Failed to validate Google Sheets access",
			"error", err,
			"user_id", userID,
			"step", "sheets_access_validation",
			"error_details", map[string]interface{}{
				"error_type":       fmt.Sprintf("%T", err),
				"error_string":     err.Error(),
				"spreadsheet_id":   config.SpreadsheetID,
				"has_valid_token":  config.HasValidGoogleToken(),
				"token_expiry":     config.GoogleTokenExpiry,
			},
			"processing_duration_ms", processingDuration.Milliseconds())
		
		result.ProcessingTime = processingDuration
		result.Error = fmt.Sprintf("Sheets access validation failed: %v", err)
		result.ErrorType = "SHEETS_ACCESS_ERROR"
		return result
	}
	
	// Step 6: Fetch activities from Strava
	// TODO: Implement actual Strava activity fetching functionality
	// This is subject to a separate story and should not be implemented yet
	since := time.Now().AddDate(0, 0, -7)
	
	w.logger.Debug("🏃 Step 6/8: Simulating Strava activity fetch (TODO: implement actual fetching)",
		"user_id", userID,
		"step", "strava_activity_fetch",
		"fetch_parameters", map[string]interface{}{
			"since":            since.Format(time.RFC3339),
			"days_back":        7,
			"athlete_id":       config.StravaAthleteID,
			"current_time":     time.Now().Format(time.RFC3339),
			"timezone":         config.Timezone,
		})
	
	// TODO: Replace this simulation with actual Strava API call
	// The implementation should:
	// 1. Use the stravaClient.GetActivities() method
	// 2. Handle re-authorization errors properly
	// 3. Fetch activities since the specified date
	// 4. Apply proper rate limiting and error handling
	
	time.Sleep(500 * time.Millisecond) // Simulate API call time
	
	// Simulate some activities for testing purposes
	activities := []strava.Activity{
		{
			ID:         12345,
			Name:       "Morning Run",
			Type:       "Run",
			Distance:   5000,    // 5km in meters
			MovingTime: 1800,    // 30 minutes in seconds
			StartDate:  time.Now().Add(-24 * time.Hour),
			StartDateLocal: time.Now().Add(-24 * time.Hour),
			TotalElevationGain: 50,
			AverageHeartrate: 150,
			Kudos: 5,
		},
		{
			ID:         12346,
			Name:       "Evening Bike Ride",
			Type:       "Ride",
			Distance:   15000,   // 15km in meters
			MovingTime: 2700,    // 45 minutes in seconds
			StartDate:  time.Now().Add(-48 * time.Hour),
			StartDateLocal: time.Now().Add(-48 * time.Hour),
			TotalElevationGain: 200,
			AverageHeartrate: 130,
			Kudos: 8,
		},
	}
	
	// No error for simulation - actual error handling would be here in real implementation
	w.logger.Info("✅ Step 6/8: Simulated Strava activity fetch completed (TODO: implement actual fetching)",
		"user_id", userID,
		"step", "strava_activity_fetch",
		"fetch_results", map[string]interface{}{
			"activity_count":   len(activities),
			"since":            since.Format(time.RFC3339),
			"fetch_simulated":  true,
			"first_activity":   func() string {
				if len(activities) > 0 {
					return activities[0].StartDate.Format(time.RFC3339)
				}
				return "none"
			}(),
			"last_activity":    func() string {
				if len(activities) > 0 {
					return activities[len(activities)-1].StartDate.Format(time.RFC3339)
				}
				return "none"
			}(),
		})
	
	// Step 7: Write activities to Google Sheets
	// TODO: Implement actual Google Sheets writing functionality
	// This is subject to a separate story and should not be implemented yet
	if len(activities) > 0 {
		w.logger.Debug("📝 Step 7/8: Simulating Google Sheets write (TODO: implement actual writing)",
			"user_id", userID,
			"step", "sheets_activity_write",
			"write_parameters", map[string]interface{}{
				"activity_count":   len(activities),
				"spreadsheet_id":   config.SpreadsheetID,
				"target_sheet":     "Тренировъчен План",
				"write_range":      fmt.Sprintf("A2:I%d", len(activities)+1),
			})
		
		// TODO: Replace this simulation with actual sheets writing
		// The implementation should:
		// 1. Use the corrected Google Sheets client
		// 2. Write to the "Тренировъчен План" sheet
		// 3. Handle re-authorization errors
		// 4. Provide proper error logging and recovery
		
		time.Sleep(300 * time.Millisecond) // Simulate processing time
		
		w.logger.Info("✅ Step 7/8: Simulated Google Sheets write completed (TODO: implement actual writing)",
			"user_id", userID,
			"step", "sheets_activity_write",
			"write_results", map[string]interface{}{
				"activity_count":   len(activities),
				"spreadsheet_id":   config.SpreadsheetID,
				"write_simulated":  true,
			})
	} else {
		w.logger.Info("ℹ️ Step 7/8: No new activities to write to Google Sheets",
			"user_id", userID,
			"step", "sheets_activity_write",
			"skip_details", map[string]interface{}{
				"activity_count":   0,
				"since":            since.Format(time.RFC3339),
				"skip_reason":      "No activities found in the specified time range",
			})
	}
	
	// Step 8: Queue notification to notification service
	w.logger.Debug("📧 Step 8/8: Queueing notification to notification service (TODO: implement)",
		"user_id", userID,
		"step", "notification_queueing",
		"notification_details", map[string]interface{}{
			"target_queue":           "notifications_queue",
			"email":                  config.Email,
			"activity_count":         len(activities),
			"email_notifications":    config.EmailNotificationsEnabled,
			"notification_required":  config.EmailNotificationsEnabled && len(activities) > 0,
		})
	
	// TODO: Implement actual notification queueing
	// if config.EmailNotificationsEnabled && len(activities) > 0 {
	//     notificationData := map[string]interface{}{
	//         "email": config.Email,
	//         "activity_count": len(activities),
	//         "processing_time": processingDuration.Milliseconds(),
	//     }
	//     _, err := notificationQueueClient.EnqueueJob(ctx, queue.JobTypeNotification, userID, notificationData)
	//     if err != nil {
	//         w.logger.Warn("Failed to queue notification", "error", err, "user_id", userID)
	//     }
	// }
	
	w.logger.Info("✅ Step 8/8: Notification queueing completed (TODO: implement actual queueing)",
		"user_id", userID,
		"step", "notification_queueing",
		"notification_status", "simulated")
	
	// Complete processing successfully
	processingDuration := time.Since(startTime)
	
	result.Success = true
	result.ActivitiesCount = len(activities)
	result.ProcessingTime = processingDuration
	
	w.logger.Info("🎉 Successfully completed automation processing for user",
		"user_id", userID,
		"step", "processing_complete",
		"processing_summary", map[string]interface{}{
			"activity_count":        len(activities),
			"processing_duration_ms": processingDuration.Milliseconds(),
			"email":                 config.Email,
			"spreadsheet_id":        config.SpreadsheetID,
			"all_steps_successful": true,
			"final_status":         "SUCCESS",
		})
	
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