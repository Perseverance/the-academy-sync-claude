package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/automation"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/queue"
)

// SyncErrorType represents different types of sync errors
type SyncErrorType string

const (
	SyncErrorValidation         SyncErrorType = "VALIDATION_ERROR"
	SyncErrorUserNotConfigured  SyncErrorType = "USER_NOT_CONFIGURED"
	SyncErrorServiceUnavailable SyncErrorType = "SERVICE_UNAVAILABLE"
	SyncErrorInternal           SyncErrorType = "INTERNAL_ERROR"
)

// SyncError represents an error that occurred during sync operations
type SyncError struct {
	Type    SyncErrorType `json:"type"`
	Code    string        `json:"code"`
	Message string        `json:"message"`
	Cause   error         `json:"-"` // Internal cause, not exposed in JSON
}

func (e *SyncError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s (%s): %s - caused by: %v", e.Type, e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s (%s): %s", e.Type, e.Code, e.Message)
}

// SyncResponse represents the response from a sync trigger request
type SyncResponse struct {
	Success                    bool   `json:"success"`
	Message                    string `json:"message"`
	JobID                      string `json:"job_id,omitempty"`
	EstimatedCompletionSeconds int    `json:"estimated_completion_seconds"`
}

// SyncService handles manual sync operations
type SyncService struct {
	configService automation.ConfigServiceInterface
	queueClient   *queue.Client
	logger        *logger.Logger
}

// NewSyncService creates a new sync service
func NewSyncService(configService automation.ConfigServiceInterface, queueClient *queue.Client, logger *logger.Logger) *SyncService {
	return &SyncService{
		configService: configService,
		queueClient:   queueClient,
		logger:        logger,
	}
}

// TriggerManualSync triggers a manual sync for the specified user
func (s *SyncService) TriggerManualSync(ctx context.Context, userID int, traceID string) (*SyncResponse, error) {
	s.logger.Info("Triggering manual sync for user",
		"user_id", userID,
		"trace_id", traceID)

	// Check if manual sync is available (Redis configured)
	if s.queueClient == nil {
		s.logger.Warn("Manual sync unavailable: Redis queue client not configured",
			"user_id", userID,
			"trace_id", traceID)
		return nil, &SyncError{
			Type:    SyncErrorServiceUnavailable,
			Code:    "SERVICE_UNAVAILABLE",
			Message: "Manual sync is temporarily unavailable. The sync queue service is not configured.",
		}
	}

	// Validate that the user is fully configured for automation
	config, err := s.configService.GetProcessingConfigForUser(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get user configuration for manual sync",
			"user_id", userID,
			"error", err,
			"trace_id", traceID)

		// Check if this is a validation error vs other errors
		if validationErr, ok := err.(*automation.ValidationError); ok {
			return nil, &SyncError{
				Type:    SyncErrorValidation,
				Code:    "USER_NOT_CONFIGURED",
				Message: fmt.Sprintf("User configuration validation failed: %s", validationErr.Message),
				Cause:   err,
			}
		}

		// For other errors (user not found, database errors, etc.)
		return nil, &SyncError{
			Type:    SyncErrorValidation,
			Message: "User validation failed. Please check your configuration and try again.",
			Cause:   err,
		}
	}

	// Ensure user has automation enabled
	if !config.AutomationEnabled {
		s.logger.Warn("Manual sync attempted for user with automation disabled",
			"user_id", userID,
			"trace_id", traceID)
		return nil, &SyncError{
			Type:    SyncErrorUserNotConfigured,
			Code:    "AUTOMATION_DISABLED",
			Message: "Automation is disabled for this user. Please enable automation in your settings.",
		}
	}

	// Create and enqueue the manual sync job
	jobID := uuid.New().String()
	job := &queue.Job{
		ID:      jobID,
		Type:    queue.JobTypeManualSync,
		UserID:  userID,
		TraceID: traceID,
		Data: map[string]interface{}{
			"triggered_by": "user_request",
			"sync_type":    "manual",
		},
		CreatedAt: time.Now(),
	}

	if err := s.queueClient.EnqueueJob(ctx, job); err != nil {
		s.logger.Error("Failed to enqueue manual sync job",
			"user_id", userID,
			"job_id", jobID,
			"error", err,
			"trace_id", traceID)
		return nil, &SyncError{
			Type:    SyncErrorInternal,
			Code:    "QUEUE_ERROR",
			Message: "Failed to queue sync job. Please try again.",
			Cause:   err,
		}
	}

	s.logger.Info("Manual sync job enqueued successfully",
		"user_id", userID,
		"job_id", jobID,
		"trace_id", traceID)

	return &SyncResponse{
		Success:                    true,
		Message:                    "Manual sync triggered successfully",
		JobID:                      jobID,
		EstimatedCompletionSeconds: 60, // Estimated completion time
	}, nil
}

// GetQueueStatus returns the current status of the sync queue
func (s *SyncService) GetQueueStatus(ctx context.Context) (*queue.QueueStatus, error) {
	if s.queueClient == nil {
		return nil, &SyncError{
			Type:    SyncErrorServiceUnavailable,
			Code:    "SERVICE_UNAVAILABLE",
			Message: "Queue status unavailable: Redis queue client not configured.",
		}
	}

	status, err := s.queueClient.GetQueueStatus(ctx)
	if err != nil {
		s.logger.Error("Failed to get queue status", "error", err)
		return nil, &SyncError{
			Type:    SyncErrorInternal,
			Code:    "QUEUE_STATUS_ERROR",
			Message: "Failed to retrieve queue status.",
			Cause:   err,
		}
	}

	return status, nil
}

// IsManualSyncAvailable checks if manual sync functionality is available
func (s *SyncService) IsManualSyncAvailable(ctx context.Context) bool {
	if s.queueClient == nil {
		return false
	}
	return s.queueClient.IsHealthy(ctx)
}