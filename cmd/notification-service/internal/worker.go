package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/logger"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/queue"
)

// NotificationJob represents the structure of a notification job
type NotificationJob struct {
	UserID             int             `json:"user_id"`
	UserEmail          string          `json:"user_email"`
	UserName           string          `json:"user_name"`
	RunDate            string          `json:"run_date"`
	Logs               []ProcessingLog `json:"logs"`
	SpreadsheetID      string          `json:"spreadsheet_id"`
	LanguagePreference string          `json:"language_preference"`
}

// ProcessingLog represents a single day's processing result
type ProcessingLog struct {
	Date           string `json:"date"`           // YYYY-MM-DD format
	Status         string `json:"status"`         // success/partial/failed/skipped
	SummaryMessage string `json:"summary_message"`
	ActivitiesFound int   `json:"activities_found"`
	Error          string `json:"error,omitempty"`
}

// NotificationServiceInterface defines the interface for notification service
type NotificationServiceInterface interface {
	ProcessNotification(ctx context.Context, notification *NotificationJob) error
}

// QueueClientInterface defines the interface for queue operations
type QueueClientInterface interface {
	DequeueJob(ctx context.Context) (*queue.Job, error)
	EnqueueJob(ctx context.Context, jobType queue.JobType, userID int, data map[string]interface{}) (*queue.Job, error)
	Close() error
	HealthCheck(ctx context.Context) error
	GetQueueLength(ctx context.Context) (int64, error)
	AcquireUserProcessingLock(ctx context.Context, userID int, ttl time.Duration) (bool, error)
	ReleaseUserProcessingLock(ctx context.Context, userID int) error
	IsUserProcessingLocked(ctx context.Context, userID int) (bool, error)
}

// Worker handles consuming and processing notification jobs from the queue
type Worker struct {
	queueClient QueueClientInterface
	service     NotificationServiceInterface
	logger      *logger.Logger
}

// NewWorker creates a new notification worker
func NewWorker(queueClient QueueClientInterface, service NotificationServiceInterface, logger *logger.Logger) *Worker {
	return &Worker{
		queueClient: queueClient,
		service:     service,
		logger:      logger.WithContext("component", "notification_worker"),
	}
}

// ProcessJobs continuously processes jobs from the notification queue
func (w *Worker) ProcessJobs(ctx context.Context) {
	w.logger.Info("Starting notification worker")

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Notification worker stopping due to context cancellation")
			return
		default:
			// Try to dequeue a job with timeout
			dequeueCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			job, err := w.queueClient.DequeueJob(dequeueCtx)
			cancel()

			if err != nil {
				if err == context.DeadlineExceeded {
					// Timeout - no job available, continue
					continue
				}
				w.logger.Error("Failed to dequeue job", "error", err)
				time.Sleep(5 * time.Second)
				continue
			}

			if job == nil {
				// No job available
				continue
			}

			// Process the job
			w.processJob(ctx, job)
		}
	}
}

// processJob processes a single notification job
func (w *Worker) processJob(ctx context.Context, job *queue.Job) {
	startTime := time.Now()
	
	w.logger.Info("Processing notification job",
		"job_id", job.ID,
		"user_id", job.UserID,
		"job_type", job.Type,
		"created_at", job.CreatedAt)

	// Decode job data
	var notificationData NotificationJob
	if err := decodeJobData(job.Data, &notificationData); err != nil {
		w.logger.Error("Failed to decode notification job data",
			"error", err,
			"job_id", job.ID,
			"user_id", job.UserID)
		// Job is already removed from queue by BRPOP
		return
	}

	// Validate notification data
	if notificationData.UserEmail == "" {
		w.logger.Error("Invalid notification data: missing user email",
			"job_id", job.ID,
			"user_id", job.UserID)
		// Job is already removed from queue by BRPOP
		return
	}

	// Process notification through the service
	err := w.service.ProcessNotification(ctx, &notificationData)
	if err != nil {
		w.logger.Error("Failed to process notification",
			"error", err,
			"job_id", job.ID,
			"user_id", job.UserID,
			"email", notificationData.UserEmail,
			"processing_time_ms", time.Since(startTime).Milliseconds())
		// TODO: Consider retry logic for transient errors
	} else {
		w.logger.Info("Successfully processed notification",
			"job_id", job.ID,
			"user_id", job.UserID,
			"email", notificationData.UserEmail,
			"processing_time_ms", time.Since(startTime).Milliseconds())
	}

	// Job is already removed from queue by BRPOP, no acknowledgment needed
}

// decodeJobData decodes the job data map into the target structure
func decodeJobData(data map[string]interface{}, target interface{}) error {
	// Convert map to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal job data: %w", err)
	}

	// Decode JSON into target structure
	if err := json.Unmarshal(jsonData, target); err != nil {
		return fmt.Errorf("failed to unmarshal job data: %w", err)
	}

	return nil
}