package services

import (
	"context"
	"fmt"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/queue"
)

// SchedulingService handles scheduled job processing
type SchedulingService struct {
	userRepo    UserRepository
	queueClient QueueClient
	logger      *logger.Logger
}

// NewSchedulingService creates a new scheduling service
func NewSchedulingService(userRepo UserRepository, queueClient QueueClient, logger *logger.Logger) *SchedulingService {
	return &SchedulingService{
		userRepo:    userRepo,
		queueClient: queueClient,
		logger:      logger,
	}
}

// ProcessScheduledRun finds users in their processing window and enqueues jobs for them
func (s *SchedulingService) ProcessScheduledRun(ctx context.Context) (int, error) {
	s.logger.Info("Starting scheduled run processing")

	// Get current UTC time for logging
	currentTime := time.Now().UTC()
	s.logger.Info("Current UTC time", "time", currentTime.Format("2006-01-02 15:04:05 MST"))

	// Get users whose local time is in the processing window (3:00-5:00 AM)
	userIDs, err := s.userRepo.GetUsersInProcessingWindow(ctx)
	if err != nil {
		s.logger.Error("Failed to get users in processing window", "error", err)
		return 0, fmt.Errorf("failed to get users in processing window: %w", err)
	}

	if len(userIDs) == 0 {
		s.logger.Info("No users found in processing window")
		return 0, nil
	}

	s.logger.Info("Found users in processing window", "count", len(userIDs), "user_ids", userIDs)

	// Enqueue a job for each user
	enqueuedCount := 0
	for _, userID := range userIDs {
		// Create job data for scheduled sync
		jobData := map[string]interface{}{
			"trigger_type": "scheduled",
			"scheduled_at": currentTime.Format(time.RFC3339),
		}

		// Enqueue the job
		job, err := s.queueClient.EnqueueJob(ctx, queue.JobTypeScheduledSync, userID, jobData)
		if err != nil {
			s.logger.Error("Failed to enqueue job for user",
				"user_id", userID,
				"error", err)
			// Continue with other users even if one fails
			continue
		}

		enqueuedCount++
		s.logger.Info("Enqueued scheduled job for user",
			"user_id", userID,
			"job_id", job.ID)
	}

	s.logger.Info("Completed scheduled run processing",
		"users_found", len(userIDs),
		"jobs_enqueued", enqueuedCount)

	return enqueuedCount, nil
}