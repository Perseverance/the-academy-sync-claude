package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/database"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/queue"
)

// Sentinel errors for sync service
var (
	// ErrUserNotFound is returned when the user does not exist
	ErrUserNotFound = errors.New("user not found")

	// ErrStravaConnectionRequired is returned when manual sync is attempted without Strava connection
	ErrStravaConnectionRequired = errors.New("strava connection required for manual sync")

	// ErrSpreadsheetRequired is returned when manual sync is attempted without spreadsheet configuration
	ErrSpreadsheetRequired = errors.New("spreadsheet configuration required for manual sync")

	// ErrSyncAlreadyInProgress is returned when a sync is already processing for the user
	ErrSyncAlreadyInProgress = errors.New("sync already in progress for this user")
)

// UserRepository interface for testability
type UserRepository interface {
	GetUserByID(ctx context.Context, userID int) (*database.User, error)
}

// SyncService handles manual sync operations
type SyncService struct {
	userRepo    UserRepository
	queueClient *queue.Client
	logger      *logger.Logger
}

// NewSyncService creates a new sync service
func NewSyncService(userRepo UserRepository, queueClient *queue.Client, logger *logger.Logger) *SyncService {
	if queueClient == nil {
		panic("queue client is required for sync service")
	}
	if userRepo == nil {
		panic("user repository is required for sync service")
	}
	if logger == nil {
		panic("logger is required for sync service")
	}

	return &SyncService{
		userRepo:    userRepo,
		queueClient: queueClient,
		logger:      logger.WithContext("component", "sync_service"),
	}
}

// TriggerManualSync validates user configuration and enqueues a manual sync job
func (s *SyncService) TriggerManualSync(ctx context.Context, userID int) error {
	s.logger.Debug("Starting manual sync trigger",
		"user_id", userID)

	// Step 1: Fetch user to validate configuration
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to fetch user for sync validation",
			"error", err,
			"user_id", userID)
		return fmt.Errorf("failed to fetch user: %w", err)
	}

	if user == nil {
		s.logger.Warn("User not found for manual sync",
			"user_id", userID)
		return ErrUserNotFound
	}

	s.logger.Debug("User fetched for sync validation",
		"user_id", userID,
		"email", user.Email,
		"has_strava_connection", user.StravaRefreshToken != nil && len(user.StravaRefreshToken) > 0,
		"has_spreadsheet", user.SpreadsheetID != nil && *user.SpreadsheetID != "",
		"automation_enabled", user.AutomationEnabled)

	// Step 2: Validate user has Strava connection
	if user.StravaRefreshToken == nil || len(user.StravaRefreshToken) == 0 {
		s.logger.Warn("Manual sync attempted without Strava connection",
			"user_id", userID,
			"email", user.Email)
		return ErrStravaConnectionRequired
	}

	// Step 3: Validate user has configured spreadsheet
	if user.SpreadsheetID == nil || *user.SpreadsheetID == "" {
		s.logger.Warn("Manual sync attempted without spreadsheet configuration",
			"user_id", userID,
			"email", user.Email,
			"has_strava", true)
		return ErrSpreadsheetRequired
	}

	// Step 4: Check if user is already being processed
	// Try to acquire a processing lock with 10-minute TTL
	lockAcquired, err := s.queueClient.AcquireUserProcessingLock(ctx, userID, 10*time.Minute)
	if err != nil {
		s.logger.Error("Failed to check processing lock",
			"error", err,
			"user_id", userID)
		return fmt.Errorf("failed to check processing status: %w", err)
	}

	if !lockAcquired {
		s.logger.Warn("Manual sync attempted while sync already in progress",
			"user_id", userID,
			"email", user.Email)
		return ErrSyncAlreadyInProgress
	}

	// Step 5: Create job data
	jobData := map[string]interface{}{
		"trigger_type":   "manual",
		"email":          user.Email,
		"spreadsheet_id": *user.SpreadsheetID,
	}

	s.logger.Debug("Enqueueing manual sync job",
		"user_id", userID,
		"spreadsheet_id", *user.SpreadsheetID)

	// Step 6: Enqueue the job
	job, err := s.queueClient.EnqueueJob(ctx, queue.JobTypeManualSync, userID, jobData)
	if err != nil {
		// Release the lock if enqueue fails
		if releaseErr := s.queueClient.ReleaseUserProcessingLock(ctx, userID); releaseErr != nil {
			s.logger.Error("Failed to release processing lock after enqueue failure",
				"error", releaseErr,
				"user_id", userID)
		}

		s.logger.Error("Failed to enqueue manual sync job",
			"error", err,
			"user_id", userID)
		return fmt.Errorf("failed to enqueue sync job: %w", err)
	}

	s.logger.Info("Manual sync job enqueued successfully",
		"user_id", userID,
		"job_id", job.ID,
		"trace_id", job.TraceID,
		"email", user.Email)

	return nil
}

// GetUserSyncStatus checks if a user is eligible for sync
func (s *SyncService) GetUserSyncStatus(ctx context.Context, userID int) (eligible bool, reason string, err error) {
	s.logger.Debug("Checking user sync status",
		"user_id", userID)

	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to fetch user for status check",
			"error", err,
			"user_id", userID)
		return false, "", fmt.Errorf("failed to fetch user: %w", err)
	}

	if user == nil {
		return false, "user not found", nil
	}

	// Check Strava connection
	if user.StravaRefreshToken == nil || len(user.StravaRefreshToken) == 0 {
		s.logger.Debug("User not eligible: no Strava connection",
			"user_id", userID)
		return false, "strava connection required", nil
	}

	// Check spreadsheet configuration
	if user.SpreadsheetID == nil || *user.SpreadsheetID == "" {
		s.logger.Debug("User not eligible: no spreadsheet configured",
			"user_id", userID)
		return false, "spreadsheet configuration required", nil
	}

	s.logger.Debug("User is eligible for sync",
		"user_id", userID,
		"email", user.Email)

	return true, "", nil
}
