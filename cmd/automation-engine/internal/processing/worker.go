package processing

import (
	"context"
	"fmt"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/auth"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/automation"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/database"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/google"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/queue"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/strava"
)

// QueueClient interface for distributed locking and job queueing operations
type QueueClient interface {
	AcquireUserProcessingLock(ctx context.Context, userID int, ttl time.Duration) (bool, error)
	ReleaseUserProcessingLock(ctx context.Context, userID int) error
	EnqueueJob(ctx context.Context, jobType queue.JobType, userID int, data map[string]interface{}) (*queue.Job, error)
}

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
	configService     *automation.ConfigService
	tokenPersister    auth.TokenPersister
	activityLogRepo   ActivityLogRepository
	jobsQueueClient          QueueClient // Queue client for jobs and distributed locking
	notificationQueueClient  QueueClient // Queue client for notifications
	logger            *logger.Logger

	// OAuth credentials for API clients
	stravaClientID     string
	stravaClientSecret string
	googleClientID     string
	googleClientSecret string
	googleRedirectURL  string
}

// NewWorker creates a new processing worker with required dependencies
func NewWorker(
	configService *automation.ConfigService,
	tokenPersister auth.TokenPersister,
	activityLogRepo ActivityLogRepository,
	jobsQueueClient QueueClient,
	notificationQueueClient QueueClient,
	stravaClientID, stravaClientSecret string,
	googleClientID, googleClientSecret, googleRedirectURL string,
	logger *logger.Logger,
) *Worker {
	return &Worker{
		configService:           configService,
		tokenPersister:          tokenPersister,
		activityLogRepo:         activityLogRepo,
		jobsQueueClient:         jobsQueueClient,
		notificationQueueClient: notificationQueueClient,
		stravaClientID:          stravaClientID,
		stravaClientSecret:      stravaClientSecret,
		googleClientID:          googleClientID,
		googleClientSecret:      googleClientSecret,
		googleRedirectURL:       googleRedirectURL,
		logger:                  logger.WithContext("component", "automation_worker"),
	}
}

// WorkerProcessingResult represents the outcome of processing a user's automation job
type WorkerProcessingResult struct {
	UserID                int           `json:"user_id"`
	Success               bool          `json:"success"`
	ActivitiesCount       int           `json:"activities_count"`      // Newly processed activities
	TotalActivitiesFound  int           `json:"total_activities_found"` // Total activities found
	ProcessingTime        time.Duration `json:"processing_time"`
	Error                 string        `json:"error,omitempty"`
	ErrorType             string        `json:"error_type,omitempty"`
	RequiresReauth        bool          `json:"requires_reauth"`
}

// ProcessUser processes automation for a single user
// This method implements the core processing flow:
// 1. Retrieve user configuration (US022)
// 2. Create API clients with token management (US023, US024)
// 3. Process data based on job type (manual or scheduled sync)
// 4. Handle errors gracefully with proper logging
func (w *Worker) ProcessUser(ctx context.Context, userID int, jobType string) *WorkerProcessingResult {
	return w.ProcessUserWithData(ctx, userID, jobType, nil)
}

// ProcessUserWithData processes automation for a single user with optional job data
// This method allows passing additional job data that may contain trigger_type for scheduled runs
func (w *Worker) ProcessUserWithData(ctx context.Context, userID int, jobType string, jobData map[string]interface{}) (result *WorkerProcessingResult) {
	startTime := time.Now()

	// Set up panic recovery to ensure one user's panic doesn't affect others
	defer func() {
		if r := recover(); r != nil {
			processingDuration := time.Since(startTime)
			w.logger.Critical("PANIC: Unexpected panic during user processing",
				"user_id", userID,
				"job_type", jobType,
				"panic_value", r,
				"processing_duration_ms", processingDuration.Milliseconds(),
				"recovery_action", "User processing aborted, continuing with other users")
			
			// Return error result for this user
			result = &WorkerProcessingResult{
				UserID:         userID,
				Success:        false,
				ProcessingTime: processingDuration,
				Error:          fmt.Sprintf("Processing panic: %v", r),
				ErrorType:      "PANIC_ERROR",
			}
			
			// Persist panic to activity log
			w.persistProcessingResult(ctx, userID, jobType, result, nil, 0)
		}
	}()

	// Acquire distributed lock for user processing
	if w.jobsQueueClient != nil {
		// Use a 10-minute TTL for the lock to match the job processing timeout
		lockAcquired, err := w.jobsQueueClient.AcquireUserProcessingLock(ctx, userID, 10*time.Minute)
		if err != nil {
			w.logger.Error("Failed to acquire user processing lock",
				"error", err,
				"user_id", userID,
				"job_type", jobType)
			return &WorkerProcessingResult{
				UserID:         userID,
				Success:        false,
				ProcessingTime: time.Since(startTime),
				Error:          fmt.Sprintf("Failed to acquire processing lock: %v", err),
				ErrorType:      "LOCK_ERROR",
			}
		}

		if !lockAcquired {
			w.logger.Info("User is already being processed, skipping",
				"user_id", userID,
				"job_type", jobType)
			return &WorkerProcessingResult{
				UserID:         userID,
				Success:        false,
				ProcessingTime: time.Since(startTime),
				Error:          "User is already being processed",
				ErrorType:      "ALREADY_PROCESSING",
			}
		}

		// Ensure lock is released when processing completes
		defer func() {
			if err := w.jobsQueueClient.ReleaseUserProcessingLock(ctx, userID); err != nil {
				w.logger.Error("Failed to release user processing lock",
					"error", err,
					"user_id", userID)
			}
		}()
	}

	w.logger.Info("🚀 Starting automation processing for user",
		"user_id", userID,
		"job_type", jobType,
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

	result = &WorkerProcessingResult{
		UserID:         userID,
		Success:        false,
		ProcessingTime: 0,
	}

	// Step 1: Retrieve user configuration (US022)
	w.logger.Debug("📋 Step 1: Retrieving user configuration for processing",
		"user_id", userID,
		"step", "config_retrieval")

	config, err := w.configService.GetProcessingConfigForUser(ctx, userID)
	if err != nil {
		processingDuration := time.Since(startTime)
		w.logger.Error("❌ FATAL: Failed to retrieve user configuration, skipping user processing",
			"error", err,
			"error_details", map[string]interface{}{
				"error_type":   fmt.Sprintf("%T", err),
				"error_string": err.Error(),
			},
			"user_id", userID,
			"step", "config_retrieval",
			"processing_duration_ms", processingDuration.Milliseconds(),
			"failure_reason", "Cannot proceed without valid user configuration")

		result.ProcessingTime = processingDuration
		result.Error = fmt.Sprintf("Configuration retrieval failed: %v", err)
		result.ErrorType = "CONFIG_ERROR"
		// Persist failure to activity log
		w.persistProcessingResult(ctx, userID, jobType, result, nil, 0)
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
		// Persist skip to activity log
		w.persistProcessingResult(ctx, userID, jobType, result, nil, 0)
		return result
	}

	w.logger.Info("✅ Step 1: Successfully retrieved user configuration",
		"user_id", userID,
		"step", "config_retrieval",
		"config_details", map[string]interface{}{
			"email":                  config.Email,
			"spreadsheet_id":         config.SpreadsheetID,
			"timezone":               config.Timezone,
			"automation_enabled":     config.AutomationEnabled,
			"email_notifications":    config.EmailNotificationsEnabled,
			"has_valid_google_token": config.HasValidGoogleToken(),
			"has_valid_strava_token": config.HasValidStravaToken(),
			"has_strava_athlete_id":  config.StravaAthleteID != nil,
			"google_token_expiry":    config.GoogleTokenExpiry,
			"strava_token_expiry":    config.StravaTokenExpiry,
		})

	// Step 2: Create Strava API client with token management (US023)
	w.logger.Debug("🏃 Step 2: Creating Strava API client with token management",
		"user_id", userID,
		"step", "strava_client_creation",
		"strava_config", map[string]interface{}{
			"has_refresh_token":  config.StravaRefreshToken != "",
			"has_access_token":   config.StravaAccessToken != "",
			"token_valid":        config.HasValidStravaToken(),
			"athlete_id":         config.StravaAthleteID,
			"client_credentials": w.stravaClientID != "" && w.stravaClientSecret != "",
		})

	stravaClient := strava.NewClient(userID, config.StravaRefreshToken, w.logger)
	stravaClient.SetOAuthCredentials(w.stravaClientID, w.stravaClientSecret)

	// Set token persister if available
	if w.tokenPersister != nil {
		stravaClient.SetTokenPersister(w.tokenPersister)
	}

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
	w.logger.Debug("📊 Step 3: Creating Google Sheets API client with token management",
		"user_id", userID,
		"step", "google_client_creation",
		"google_config", map[string]interface{}{
			"has_refresh_token":  config.GoogleRefreshToken != "",
			"has_access_token":   config.GoogleAccessToken != "",
			"token_valid":        config.HasValidGoogleToken(),
			"spreadsheet_id":     config.SpreadsheetID,
			"client_credentials": w.googleClientID != "" && w.googleClientSecret != "",
		})

	sheetsClient := google.NewSheetsClient(userID, config.GoogleRefreshToken, w.logger)
	sheetsClient.SetOAuthCredentials(w.googleClientID, w.googleClientSecret, w.googleRedirectURL)

	// Set token persister if available
	if w.tokenPersister != nil {
		sheetsClient.SetTokenPersister(w.tokenPersister)
	}

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

	// Step 4: Validate spreadsheet access
	w.logger.Debug("🔐 Step 4: Validating Google Sheets access",
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
			
			// Set Google reauth required flag in database
			if err := w.configService.SetGoogleReauthRequired(ctx, userID, true); err != nil {
				w.logger.Error("Failed to set Google reauth required flag",
					"error", err,
					"user_id", userID)
			}
			
			// Persist reauth requirement to activity log
			w.persistProcessingResult(ctx, userID, jobType, result, nil, 0)
			return result
		}

		w.logger.Error("❌ Failed to validate Google Sheets access",
			"error", err,
			"user_id", userID,
			"step", "sheets_access_validation",
			"error_details", map[string]interface{}{
				"error_type":      fmt.Sprintf("%T", err),
				"error_string":    err.Error(),
				"spreadsheet_id":  config.SpreadsheetID,
				"has_valid_token": config.HasValidGoogleToken(),
				"token_expiry":    config.GoogleTokenExpiry,
			},
			"processing_duration_ms", processingDuration.Milliseconds())

		result.ProcessingTime = processingDuration
		result.Error = fmt.Sprintf("Sheets access validation failed: %v", err)
		result.ErrorType = "SHEETS_ACCESS_ERROR"
		// Persist sheets access error to activity log
		w.persistProcessingResult(ctx, userID, jobType, result, nil, 0)
		return result
	}

	// Step 5: Create processing service
	processingService := NewProcessingService(stravaClient, sheetsClient, w.activityLogRepo, w.logger)

	// Step 6: Calculate date range and fetch training plan cache
	w.logger.Info("📊 Step 6: Fetching training plan data to identify unprocessed days",
		"user_id", userID,
		"job_type", jobType)

	// Load user's timezone to calculate date range
	location, err := time.LoadLocation(config.Timezone)
	if err != nil {
		w.logger.Error("Invalid timezone",
			"timezone", config.Timezone,
			"error", err)
		result.ProcessingTime = time.Since(startTime)
		result.Error = fmt.Sprintf("Invalid timezone: %v", err)
		result.ErrorType = "TIMEZONE_ERROR"
		// Persist timezone error to activity log
		w.persistProcessingResult(ctx, userID, jobType, result, nil, 0)
		return result
	}

	// Calculate date range for fetching training plan
	now := time.Now().In(location)
	// Set to beginning of day for consistent date comparisons
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	endDate := today                     // Today at 00:00:00
	startDate := today.AddDate(0, 0, -8) // 8 days ago at 00:00:00

	// Fetch all training plan entries once
	trainingPlanCache, err := processingService.FetchAllTrainingPlanEntries(ctx, config, startDate, endDate)
	if err != nil {
		w.logger.Error("Failed to fetch training plan cache",
			"error", err,
			"user_id", userID,
			"date_range", fmt.Sprintf("%s to %s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02")))
		result.ProcessingTime = time.Since(startTime)
		result.Error = fmt.Sprintf("Failed to fetch training plan: %v", err)
		result.ErrorType = "TRAINING_PLAN_ERROR"
		// Persist training plan error to activity log
		w.persistProcessingResult(ctx, userID, jobType, result, location, 0)
		return result
	}

	// Step 7: Identify unprocessed days
	w.logger.Info("🔍 Step 7: Identifying unprocessed days",
		"user_id", userID,
		"total_training_plan_entries", len(trainingPlanCache))

	// Get unprocessed workout days (scheduled runs)
	unprocessedWorkoutDays := processingService.GetUnprocessedWorkoutDays(trainingPlanCache)
	
	// Also get ALL unprocessed days to catch rest days with activities
	allUnprocessedDays := processingService.GetAllUnprocessedDays(trainingPlanCache)
	
	w.logger.Info("Identified unprocessed days",
		"user_id", userID,
		"unprocessed_workout_days", len(unprocessedWorkoutDays),
		"all_unprocessed_days", len(allUnprocessedDays),
		"workout_dates", func() []string {
			dates := make([]string, len(unprocessedWorkoutDays))
			for i, date := range unprocessedWorkoutDays {
				dates[i] = date.Format("2006-01-02")
			}
			return dates
		}())

	// Step 8: Fetch Strava activities
	w.logger.Info("🏃 Step 8: Fetching Strava activities for unprocessed days",
		"user_id", userID,
		"job_type", jobType,
		"primary_days_to_fetch", len(unprocessedWorkoutDays),
		"all_unprocessed_days", len(allUnprocessedDays))

	var stravaActivitiesCache StravaActivitiesCache
	if len(allUnprocessedDays) > 0 {
		// Fetch Strava activities for the date range covering all unprocessed days
		// but focus on workout days for optimization
		stravaActivitiesCache, err = processingService.FetchStravaActivitiesForDates(ctx, location, unprocessedWorkoutDays, allUnprocessedDays)
		if err != nil {
			// Check if this requires Strava re-authorization
			if strava.IsReauthRequired(err) {
				w.logger.Warn("🔐 Strava access requires user re-authorization",
					"user_id", userID,
					"error", err,
					"error_analysis", map[string]interface{}{
						"error_type":           fmt.Sprintf("%T", err),
						"requires_reauth":      true,
						"strava_token_expired": !config.HasValidStravaToken(),
					},
					"action_required", "User must re-authorize Strava access")
				
				result.ProcessingTime = time.Since(startTime)
				result.Error = "Strava access requires re-authorization"
				result.ErrorType = "STRAVA_REAUTH_REQUIRED"
				result.RequiresReauth = true
				
				// Set Strava reauth required flag in database
				if err := w.configService.SetStravaReauthRequired(ctx, userID, true); err != nil {
					w.logger.Error("Failed to set Strava reauth required flag",
						"error", err,
						"user_id", userID)
				}
				
				// Persist reauth requirement to activity log
				w.persistProcessingResult(ctx, userID, jobType, result, location, 0)
				return result
			}
			
			w.logger.Error("Failed to fetch Strava activities",
				"error", err,
				"user_id", userID,
				"unprocessed_workout_days", len(unprocessedWorkoutDays),
				"all_unprocessed_days", len(allUnprocessedDays))
			result.ProcessingTime = time.Since(startTime)
			result.Error = fmt.Sprintf("Failed to fetch Strava activities: %v", err)
			result.ErrorType = "STRAVA_API_ERROR"
			// Persist Strava API error to activity log
			w.persistProcessingResult(ctx, userID, jobType, result, location, 0)
			return result
		}
	} else {
		// No unprocessed days at all, create empty cache
		stravaActivitiesCache = make(StravaActivitiesCache)
		w.logger.Info("No unprocessed days found, skipping Strava API call",
			"user_id", userID)
	}

	// Step 9: Process based on job type and trigger type
	// Check if this is a scheduled run by looking at trigger_type in job data
	var isScheduledRun bool
	if jobData != nil {
		if triggerType, ok := jobData["trigger_type"].(string); ok && triggerType == "scheduled" {
			isScheduledRun = true
		}
	}

	w.logger.Info("📊 Step 9: Processing data based on job type",
		"user_id", userID,
		"job_type", jobType,
		"is_scheduled_run", isScheduledRun,
		"training_plan_entries", len(trainingPlanCache),
		"strava_activity_days", len(stravaActivitiesCache))

	var totalActivities int        // Newly processed activities
	var totalActivitiesFound int   // Total activities found (including already processed)
	var processingErrors []error
	var spreadsheetUpdates []*google.SpreadsheetUpdate
	var scheduledResult *ProcessingResult      // For scheduled runs
	var todayResult *DayProcessingResult       // For manual runs - today
	var prevDayResult *DayProcessingResult     // For manual runs - yesterday  
	var lookbackResults []*DayProcessingResult // For manual runs - lookback

	// Handle scheduled runs using RunScheduledCycle
	if isScheduledRun {
		w.logger.Debug("Processing scheduled run - will process yesterday and 7-day lookback",
			"user_id", userID,
			"timezone", config.Timezone)

		// Use RunScheduledCycle for scheduled runs (US035)
		var err error
		scheduledResult, err = processingService.RunScheduledCycle(ctx, config, trainingPlanCache, stravaActivitiesCache)
		if err != nil {
			result.ProcessingTime = time.Since(startTime)
			result.Error = fmt.Sprintf("Scheduled cycle processing failed: %v", err)
			result.ErrorType = "PROCESSING_ERROR"
			// Persist error to activity log
			w.persistProcessingResult(ctx, userID, jobType, result, location, 0)
			return result
		}

		// Update result from scheduled cycle
		totalActivities = scheduledResult.ActivitiesCount
		if scheduledResult.Error != "" {
			result.ProcessingTime = time.Since(startTime)
			result.Error = scheduledResult.Error
			result.ErrorType = "PROCESSING_ERROR"
			// Persist error to activity log
			w.persistProcessingResult(ctx, userID, jobType, result, location, scheduledResult.RowsUpdated)
			return result
		}

		// Convert detailed results to spreadsheet updates and track total activities
		for _, dayResult := range scheduledResult.DetailedResults {
			// Track total activities found
			totalActivitiesFound += dayResult.ActivitiesFound
			if dayResult.Processed && dayResult.SpreadsheetUpdate != nil {
				update := &google.SpreadsheetUpdate{
					Row:              dayResult.SpreadsheetUpdate.Row,
					DistanceValue:    dayResult.SpreadsheetUpdate.DistanceValue,
					TimeValue:        dayResult.SpreadsheetUpdate.TimeValue,
					RPEValue:         dayResult.SpreadsheetUpdate.RPEValue,
					DescriptionValue: dayResult.SpreadsheetUpdate.DescriptionValue,
					DescriptionBold:  dayResult.SpreadsheetUpdate.DescriptionBold,
				}
				spreadsheetUpdates = append(spreadsheetUpdates, update)
			}
		}
	} else if jobType == "manual_sync" {
		// For manual sync, also process today's data
		w.logger.Debug("Processing manual sync - will process today, yesterday, and 7-day lookback",
			"user_id", userID,
			"timezone", config.Timezone)

		// Process today's data (US028)
		var err error
		todayResult, err = processingService.ProcessTodaySoFar(ctx, config, trainingPlanCache, stravaActivitiesCache)
		if err != nil {
			processingErrors = append(processingErrors, fmt.Errorf("today processing failed: %w", err))
		} else if todayResult.Error != nil {
			processingErrors = append(processingErrors, fmt.Errorf("today processing error: %w", todayResult.Error))
		} else {
			// Track total activities found
			totalActivitiesFound += todayResult.ActivitiesFound
			// Only count activities for newly processed days
			if todayResult.IsNewlyProcessed {
				totalActivities += todayResult.ActivitiesFound
			}
			if todayResult.Processed && todayResult.SpreadsheetUpdate != nil {
				// Convert to google.SpreadsheetUpdate
				update := &google.SpreadsheetUpdate{
					Row:              todayResult.SpreadsheetUpdate.Row,
					DistanceValue:    todayResult.SpreadsheetUpdate.DistanceValue,
					TimeValue:        todayResult.SpreadsheetUpdate.TimeValue,
					RPEValue:         todayResult.SpreadsheetUpdate.RPEValue,
					DescriptionValue: todayResult.SpreadsheetUpdate.DescriptionValue,
					DescriptionBold:  todayResult.SpreadsheetUpdate.DescriptionBold,
				}
				spreadsheetUpdates = append(spreadsheetUpdates, update)
			}
		}
	} else {
		w.logger.Debug("Processing scheduled sync - will process yesterday and 7-day lookback",
			"user_id", userID,
			"timezone", config.Timezone)
	}

	// Only process previous day and lookback for non-scheduled runs
	// (scheduled runs already processed these in RunScheduledCycle)
	if !isScheduledRun {
		// Process previous day (US025) - for manual and regular scheduled sync
		prevDayResult, err = processingService.ProcessPreviousDay(ctx, config, trainingPlanCache, stravaActivitiesCache)
	if err != nil {
		processingErrors = append(processingErrors, fmt.Errorf("previous day processing failed: %w", err))
	} else if prevDayResult.Error != nil {
		processingErrors = append(processingErrors, fmt.Errorf("previous day processing error: %w", prevDayResult.Error))
	} else {
		// Track total activities found
		totalActivitiesFound += prevDayResult.ActivitiesFound
		// Only count activities for newly processed days
		if prevDayResult.IsNewlyProcessed {
			totalActivities += prevDayResult.ActivitiesFound
		}
		if prevDayResult.Processed && prevDayResult.SpreadsheetUpdate != nil {
			// Convert to google.SpreadsheetUpdate
			update := &google.SpreadsheetUpdate{
				Row:              prevDayResult.SpreadsheetUpdate.Row,
				DistanceValue:    prevDayResult.SpreadsheetUpdate.DistanceValue,
				TimeValue:        prevDayResult.SpreadsheetUpdate.TimeValue,
				RPEValue:         prevDayResult.SpreadsheetUpdate.RPEValue,
				DescriptionValue: prevDayResult.SpreadsheetUpdate.DescriptionValue,
				DescriptionBold:  prevDayResult.SpreadsheetUpdate.DescriptionBold,
			}
			spreadsheetUpdates = append(spreadsheetUpdates, update)
		}
	}

	// Process 7-day lookback (US026 & US027) - common for both sync types
	lookbackResults, err = processingService.ProcessLookbackPeriod(ctx, config, trainingPlanCache, stravaActivitiesCache)
	if err != nil {
		processingErrors = append(processingErrors, fmt.Errorf("lookback processing failed: %w", err))
	} else {
		for _, lr := range lookbackResults {
			if lr.Error == nil && lr.Processed {
				// Track total activities found
				totalActivitiesFound += lr.ActivitiesFound
				// Only count activities for newly processed days
				if lr.IsNewlyProcessed {
					totalActivities += lr.ActivitiesFound
				}
				if lr.SpreadsheetUpdate != nil {
					// Convert to google.SpreadsheetUpdate
					update := &google.SpreadsheetUpdate{
						Row:              lr.SpreadsheetUpdate.Row,
						DistanceValue:    lr.SpreadsheetUpdate.DistanceValue,
						TimeValue:        lr.SpreadsheetUpdate.TimeValue,
						RPEValue:         lr.SpreadsheetUpdate.RPEValue,
						DescriptionValue: lr.SpreadsheetUpdate.DescriptionValue,
						DescriptionBold:  lr.SpreadsheetUpdate.DescriptionBold,
					}
					spreadsheetUpdates = append(spreadsheetUpdates, update)
				}
			}
		}
	}
	} // End of !isScheduledRun block

	// Step 10: Batch update spreadsheet if we have any updates
	if len(spreadsheetUpdates) > 0 {
		w.logger.Info("📝 Step 10: Writing updates to Google Sheets",
			"user_id", userID,
			"update_count", len(spreadsheetUpdates))

		err := sheetsClient.BatchUpdateTrainingPlan(ctx, config.SpreadsheetID, spreadsheetUpdates)
		if err != nil {
			processingErrors = append(processingErrors, fmt.Errorf("spreadsheet batch update failed: %w", err))
			w.logger.Error("Failed to update spreadsheet",
				"error", err,
				"user_id", userID,
				"spreadsheet_id", config.SpreadsheetID,
				"update_count", len(spreadsheetUpdates))
		} else {
			w.logger.Info("Successfully updated training plan spreadsheet",
				"user_id", userID,
				"rows_updated", len(spreadsheetUpdates))
		}
	} else {
		w.logger.Info("No spreadsheet updates needed",
			"user_id", userID,
			"update_count", 0)
	}

	// Step 11: Handle results
	processingDuration := time.Since(startTime)

	if len(processingErrors) > 0 {
		w.logger.Error("❌ Processing completed with errors",
			"user_id", userID,
			"job_type", jobType,
			"error_count", len(processingErrors),
			"errors", processingErrors,
			"processing_duration_ms", processingDuration.Milliseconds())

		result.ProcessingTime = processingDuration
		result.Error = fmt.Sprintf("Processing failed with %d errors", len(processingErrors))
		result.ErrorType = "PROCESSING_ERROR"
		result.ActivitiesCount = totalActivities
		result.TotalActivitiesFound = totalActivitiesFound
		// Persist processing errors to activity log
		w.persistProcessingResult(ctx, userID, jobType, result, location, len(spreadsheetUpdates))
		return result
	}

	// Step 12: Queue notification (if enabled)
	if config.EmailNotificationsEnabled {
		w.logger.Debug("📧 Step 12: Queueing notification",
			"user_id", userID,
			"email", config.Email,
			"activity_count", totalActivities)
		
		// Collect all processing results
		var allResults []*DayProcessingResult
		if isScheduledRun && scheduledResult != nil {
			// For scheduled runs, use the detailed results from RunScheduledCycle
			allResults = scheduledResult.DetailedResults
		} else {
			// For manual runs, collect individual results
			if todayResult != nil {
				allResults = append(allResults, todayResult)
			}
			if prevDayResult != nil {
				allResults = append(allResults, prevDayResult)
			}
			allResults = append(allResults, lookbackResults...)
		}
		
		// Prepare notification job data
		notificationData := w.prepareNotificationData(config, allResults, location)
		
		// Enqueue notification if there's data to send
		if notificationData != nil {
			if err := w.enqueueNotificationJob(ctx, userID, notificationData); err != nil {
				w.logger.Error("Failed to enqueue notification job",
					"error", err,
					"user_id", userID)
				// Don't fail the whole process for notification failures
			}
		}
	}

	// Complete processing successfully
	result.Success = true
	result.ActivitiesCount = totalActivities
	result.TotalActivitiesFound = totalActivitiesFound
	result.ProcessingTime = processingDuration

	w.logger.Info("🎉 Successfully completed automation processing for user",
		"user_id", userID,
		"job_type", jobType,
		"processing_summary", map[string]interface{}{
			"activity_count":         totalActivities,
			"total_activities_found": totalActivitiesFound,
			"processing_duration_ms": processingDuration.Milliseconds(),
			"email":                  config.Email,
			"spreadsheet_id":         config.SpreadsheetID,
			"final_status":           "SUCCESS",
		})

	// Persist to activity log
	w.persistProcessingResult(ctx, userID, jobType, result, location, len(spreadsheetUpdates))

	return result
}

// ProcessUsers processes automation for multiple users
// This method handles batch processing with individual error isolation
func (w *Worker) ProcessUsers(ctx context.Context, userIDs []int, jobType string) []*WorkerProcessingResult {
	w.logger.Info("Starting batch automation processing",
		"user_count", len(userIDs),
		"job_type", jobType)

	results := make([]*WorkerProcessingResult, len(userIDs))

	for i, userID := range userIDs {
		// Wrap each user processing in a function with panic recovery
		func(index int, uid int) {
			defer func() {
				if r := recover(); r != nil {
					w.logger.Critical("PANIC in batch processing: Recovered from panic for user",
						"user_id", uid,
						"user_index", index+1,
						"total_users", len(userIDs),
						"panic_value", r,
						"recovery_action", "Continuing with remaining users")
					
					// Set error result for this user
					results[index] = &WorkerProcessingResult{
						UserID:         uid,
						Success:        false,
						ProcessingTime: time.Duration(0),
						Error:          fmt.Sprintf("Processing panic: %v", r),
						ErrorType:      "PANIC_ERROR",
					}
				}
			}()
			
			w.logger.Debug("Processing user in batch",
				"user_id", uid,
				"user_index", index+1,
				"total_users", len(userIDs))

			results[index] = w.ProcessUser(ctx, uid, jobType)

			// Log batch progress
			if results[index].Success {
				w.logger.Debug("User processing completed successfully in batch",
					"user_id", uid,
					"user_index", index+1,
					"total_users", len(userIDs),
					"activities_processed", results[index].ActivitiesCount)
			} else {
				w.logger.Warn("User processing failed in batch",
					"user_id", uid,
					"user_index", index+1,
					"total_users", len(userIDs),
					"error", results[index].Error,
					"error_type", results[index].ErrorType,
					"requires_reauth", results[index].RequiresReauth)
			}
		}(i, userID)
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

// persistProcessingResult saves the processing outcome to the activity log
func (w *Worker) persistProcessingResult(ctx context.Context, userID int, jobType string, result *WorkerProcessingResult, location *time.Location, rowsUpdated int) {

	// Skip if no activity log repository configured
	if w.activityLogRepo == nil {
		w.logger.Debug("Activity log repository not configured, skipping persistence",
			"user_id", userID)
		return
	}

	// Determine processing date (today in user's timezone)
	processingDate := time.Now()
	if location != nil {
		processingDate = processingDate.In(location)
	}
	// Truncate to date only
	processingDateOnly := time.Date(processingDate.Year(), processingDate.Month(), processingDate.Day(), 0, 0, 0, 0, processingDate.Location())

	// Determine status
	status := "success"
	if !result.Success {
		if result.RequiresReauth {
			status = "failed"
		} else if result.ActivitiesCount > 0 {
			status = "partial"
		} else {
			status = "failed"
		}
	}

	// Determine processing scope - both manual and scheduled process multiple days
	processingScope := "date_range"

	// Prepare error message if needed
	var errorMessage *string
	if result.Error != "" {
		errorMessage = &result.Error
	}

	// Calculate processing duration
	processingDurationMs := int(result.ProcessingTime.Milliseconds())

	// Map job type to processing type enum value
	processingType := "daily" // default
	switch jobType {
	case "manual_sync":
		processingType = "manual"
	case "scheduled_sync":
		processingType = "daily"
	}

	// Create activity log entry with full schema
	logEntry := &database.ActivityLog{
		UserID:               userID,
		ProcessingDate:       processingDateOnly,
		ProcessingType:       processingType,
		ProcessingScope:      processingScope,
		Status:               status,
		ActivitiesFound:      result.TotalActivitiesFound,  // Total activities found
		ActivitiesProcessed:  result.ActivitiesCount,        // Only newly processed activities
		SpreadsheetUpdated:   rowsUpdated > 0,
		ErrorMessage:         errorMessage,
		ProcessingStartedAt:  processingDate.Add(-result.ProcessingTime),
		ProcessingCompletedAt: &processingDate,
		ProcessingDurationMs: &processingDurationMs,
	}

	// Use a separate context with timeout for database operation
	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := w.activityLogRepo.CreateActivityLog(dbCtx, logEntry); err != nil {
		w.logger.Error("Failed to persist activity log",
			"error", err,
			"user_id", userID,
			"job_type", jobType,
			"status", status)
		// Don't return error - activity log persistence is non-critical
	} else {
		w.logger.Debug("Activity log persisted successfully",
			"user_id", userID,
			"job_type", jobType,
			"status", status,
			"activities_processed", result.ActivitiesCount,
			"rows_updated", rowsUpdated)
	}
}

// prepareNotificationData prepares the notification job data from processing results
func (w *Worker) prepareNotificationData(config *automation.ProcessingConfig, dayResults []*DayProcessingResult, location *time.Location) map[string]interface{} {
	if len(dayResults) == 0 {
		return nil
	}

	// Create processing logs from day results - only include newly processed days
	var logs []map[string]interface{}
	hasNewlyProcessedDays := false
	
	for _, dayResult := range dayResults {
		if dayResult == nil {
			continue
		}

		// Debug: Log the day result being processed
		w.logger.Debug("Processing day result for notification",
			"date", dayResult.Date.Format("2006-01-02"),
			"processed", dayResult.Processed,
			"skipped_reason", dayResult.SkippedReason,
			"activities_found", dayResult.ActivitiesFound)

		// Skip days that were already processed (marked as bold in spreadsheet)
		if dayResult.SkippedReason == SkipReasonAlreadyProcessed {
			w.logger.Debug("Skipping already processed day",
				"date", dayResult.Date.Format("2006-01-02"))
			continue
		}

		// Skip days marked as "No activities and no scheduled run"
		if dayResult.SkippedReason == SkipReasonNoActivities {
			w.logger.Debug("Skipping day with no activities and no scheduled run",
				"date", dayResult.Date.Format("2006-01-02"))
			continue
		}

		// Skip rest days with no activities (successful rest)
		if dayResult.SkippedReason == SkipReasonRestDayNoActivity {
			w.logger.Debug("Skipping successful rest day",
				"date", dayResult.Date.Format("2006-01-02"))
			continue
		}

		// Track if we have any newly processed days
		if dayResult.Processed || dayResult.Error != nil {
			hasNewlyProcessedDays = true
		}

		// Determine status
		status := "success"
		if dayResult.Error != nil {
			status = "failed"
		} else if !dayResult.Processed {
			if dayResult.SkippedReason != SkipReasonNone {
				// Check if it's a rest day with no activity
				if dayResult.SkippedReason == SkipReasonRestDayNoActivity {
					status = "success" // Rest days are successful
				} else {
					status = "skipped"
				}
			}
		}

		// Create summary message
		summaryMessage := w.createSummaryMessage(dayResult)

		// Add to logs
		log := map[string]interface{}{
			"date":             dayResult.Date.Format("2006-01-02"),
			"status":           status,
			"summary_message":  summaryMessage,
			"activities_found": dayResult.ActivitiesFound,
		}

		if dayResult.Error != nil {
			log["error"] = dayResult.Error.Error()
		}

		logs = append(logs, log)
	}

	// Don't send notification if no newly processed days
	if !hasNewlyProcessedDays || len(logs) == 0 {
		w.logger.Debug("No newly processed days, skipping notification",
			"user_id", config.UserID)
		return nil
	}

	// Don't send notification if all logs are uneventful rest days (US038)
	allUneventfulRestDays := true
	for _, log := range logs {
		if msg, ok := log["summary_message"].(string); ok {
			if msg != SkipReasonRestDayNoActivity.String() {
				allUneventfulRestDays = false
				break
			}
		}
	}

	if allUneventfulRestDays {
		w.logger.Debug("All days are uneventful rest days, skipping notification",
			"user_id", config.UserID)
		return nil
	}

	// Determine run date (today in user's timezone)
	runDate := time.Now()
	if location != nil {
		runDate = runDate.In(location)
	}

	// Create notification job data
	notificationData := map[string]interface{}{
		"user_id":       config.UserID,
		"user_email":    config.Email,
		"user_name":     config.Email, // TODO: Get user's full name from database
		"run_date":      runDate.Format(time.RFC3339),
		"logs":          logs,
		"spreadsheet_id": config.SpreadsheetID,
	}

	// Debug: Log the notification data
	w.logger.Debug("Prepared notification data",
		"user_id", config.UserID,
		"log_count", len(logs),
		"logs", logs)

	return notificationData
}

// createSummaryMessage creates a summary message for a day's processing result
func (w *Worker) createSummaryMessage(dayResult *DayProcessingResult) string {
	if dayResult.Error != nil {
		return fmt.Sprintf("Failed to process: %v", dayResult.Error)
	}

	if !dayResult.Processed {
		if dayResult.SkippedReason != SkipReasonNone {
			return dayResult.SkippedReason.String()
		}
		return "No scheduled training for this day"
	}

	// Check if this was a rest day
	if dayResult.PlanEntry != nil && dayResult.PlanEntry.RPE == 1 && dayResult.ActivitiesFound == 0 {
		// This is a special case - we need to set the skip reason for consistency
		dayResult.SkippedReason = SkipReasonRestDayNoActivity
		return dayResult.SkippedReason.String()
	}

	// Create activity summary
	if dayResult.ActivitiesFound == 0 {
		return "No activities found"
	} else if dayResult.ActivitiesFound == 1 {
		// Format: "1 activity logged (X.Xkm in HH:MM:SS)"
		distance := dayResult.TotalDistance / 1000.0 // Convert to km
		duration := time.Duration(dayResult.TotalTime) * time.Second
		return fmt.Sprintf("1 activity logged (%.1fkm in %s)", distance, formatDuration(duration))
	} else {
		// Format: "X activities logged (total: X.Xkm in HH:MM:SS)"
		distance := dayResult.TotalDistance / 1000.0 // Convert to km
		duration := time.Duration(dayResult.TotalTime) * time.Second
		return fmt.Sprintf("%d activities logged (total: %.1fkm in %s)", 
			dayResult.ActivitiesFound, distance, formatDuration(duration))
	}
}

// formatDuration formats a duration as HH:MM:SS
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

// enqueueNotificationJob enqueues a notification job to the notification queue
func (w *Worker) enqueueNotificationJob(ctx context.Context, userID int, jobData map[string]interface{}) error {
	if w.notificationQueueClient == nil {
		return fmt.Errorf("notification queue client not configured")
	}

	// Enqueue to notification queue
	job, err := w.notificationQueueClient.EnqueueJob(ctx, queue.JobType("notification"), userID, jobData)
	if err != nil {
		return fmt.Errorf("failed to enqueue notification job: %w", err)
	}

	w.logger.Info("Successfully enqueued notification job",
		"user_id", userID,
		"job_id", job.ID,
		"email", jobData["user_email"])

	return nil
}
