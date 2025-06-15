package services

import (
	"context"
	"fmt"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/queue"
)

// ConfigValidator interface for user configuration validation
type ConfigValidator interface {
	ValidateUserCanBeProcessed(ctx context.Context, userID int) error
}

// QueueClient interface for job queue operations
type QueueClient interface {
	EnqueueJob(ctx context.Context, userID int, triggerType string) (*queue.Job, error)
	GetQueueLength(ctx context.Context) (int64, error)
	HealthCheck(ctx context.Context) error
}

// SyncService handles manual sync operations
type SyncService struct {
	configService ConfigValidator
	queueClient   QueueClient
	logger        *logger.Logger
}

// NewSyncService creates a new sync service
func NewSyncService(configService ConfigValidator, queueClient QueueClient, logger *logger.Logger) *SyncService {
	return &SyncService{
		configService: configService,
		queueClient:   queueClient,
		logger:        logger.WithContext("component", "sync_service"),
	}
}

// SyncRequest represents a manual sync request
type SyncRequest struct {
	UserID int `json:"user_id"`
}

// SyncResponse represents a successful sync response
type SyncResponse struct {
	Success                   bool   `json:"success"`
	Message                   string `json:"message"`
	TraceID                   string `json:"trace_id"`
	EstimatedCompletionSeconds int    `json:"estimated_completion_seconds"`
}

// SyncError represents sync-specific errors
type SyncError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Cause   error  `json:"-"`
}

func (e *SyncError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("sync error (%s): %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("sync error (%s): %s", e.Type, e.Message)
}

func (e *SyncError) Unwrap() error {
	return e.Cause
}

// Error types for sync operations
const (
	SyncErrorUserNotFound      = "USER_NOT_FOUND"
	SyncErrorUserNotConfigured = "USER_NOT_CONFIGURED"
	SyncErrorQueueUnavailable  = "QUEUE_UNAVAILABLE"
	SyncErrorInternal          = "INTERNAL_ERROR"
	SyncErrorValidation        = "VALIDATION_ERROR"
)

// TriggerManualSync validates user configuration and enqueues a manual sync job
func (s *SyncService) TriggerManualSync(ctx context.Context, userID int) (*SyncResponse, error) {
	startTime := time.Now()
	
	s.logger.Info("Manual sync trigger requested",
		"user_id", userID,
		"trigger_type", "manual")

	// Validate user can be processed
	s.logger.Debug("Validating user can be processed",
		"user_id", userID)
	
	err := s.configService.ValidateUserCanBeProcessed(ctx, userID)
	if err != nil {
		s.logger.Warn("User validation failed for manual sync",
			"error", err,
			"user_id", userID,
			"validation_duration_ms", time.Since(startTime).Milliseconds())
		
		// Determine error type based on the validation error
		if err.Error() == fmt.Sprintf("user not found: %d", userID) {
			return nil, &SyncError{
				Type:    SyncErrorUserNotFound,
				Message: fmt.Sprintf("User %d not found", userID),
				Cause:   err,
			}
		}
		
		// Check if it's a configuration issue
		return nil, &SyncError{
			Type:    SyncErrorUserNotConfigured,
			Message: "User is not fully configured for automation. Please complete OAuth connections and spreadsheet setup.",
			Cause:   err,
		}
	}

	s.logger.Debug("User validation passed, enqueuing manual sync job",
		"user_id", userID,
		"validation_duration_ms", time.Since(startTime).Milliseconds())

	// Enqueue job to Redis
	job, err := s.queueClient.EnqueueJob(ctx, userID, "manual")
	if err != nil {
		s.logger.Error("Failed to enqueue manual sync job",
			"error", err,
			"user_id", userID,
			"total_duration_ms", time.Since(startTime).Milliseconds())
		
		return nil, &SyncError{
			Type:    SyncErrorQueueUnavailable,
			Message: "Job queue is temporarily unavailable. Please try again later.",
			Cause:   err,
		}
	}

	operationDuration := time.Since(startTime)
	
	s.logger.Info("Manual sync job enqueued successfully",
		"user_id", userID,
		"trace_id", job.TraceID,
		"trigger_type", "manual",
		"job_created_at", job.CreatedAt.Format(time.RFC3339),
		"operation_duration_ms", operationDuration.Milliseconds(),
		"estimated_completion_seconds", 60)

	return &SyncResponse{
		Success:                   true,
		Message:                   "Manual sync triggered successfully",
		TraceID:                   job.TraceID,
		EstimatedCompletionSeconds: 60, // Estimated completion time
	}, nil
}

// GetQueueStatus returns current queue information for debugging
func (s *SyncService) GetQueueStatus(ctx context.Context) (map[string]interface{}, error) {
	length, err := s.queueClient.GetQueueLength(ctx)
	if err != nil {
		s.logger.Error("Failed to get queue status",
			"error", err)
		return nil, &SyncError{
			Type:    SyncErrorQueueUnavailable,
			Message: "Unable to retrieve queue status",
			Cause:   err,
		}
	}

	status := map[string]interface{}{
		"queue_length":    length,
		"queue_name":      queue.JobsQueueName,
		"status_time":     time.Now().Format(time.RFC3339),
		"health_status":   "healthy",
	}

	// Test Redis connectivity
	if err := s.queueClient.HealthCheck(ctx); err != nil {
		status["health_status"] = "unhealthy"
		status["health_error"] = err.Error()
	}

	return status, nil
}